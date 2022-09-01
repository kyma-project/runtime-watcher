package internal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/kyma-project/runtime-watcher/skr/internal"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type CustomRouter struct {
	*http.ServeMux
	Recorder *httptest.ResponseRecorder
}

const (
	testResourceKind  = "testResourceKind"
	DefaultBufferSize = 2048
)

var ownerLabels = map[string]string{
	internal.ManagedByLabel: "lifecycle-manager",
	internal.OwnedByLabel:   "ownerNamespace/ownerName",
}

func NewCustomRouter() *CustomRouter {
	return &CustomRouter{
		ServeMux: http.NewServeMux(),
		Recorder: httptest.NewRecorder(),
	}
}

func (cr *CustomRouter) ServeHTTP(_ http.ResponseWriter, request *http.Request) {
	if request.RequestURI == "*" {
		if request.ProtoAtLeast(1, 1) {
			cr.Recorder.Header().Set("Connection", "close")
		}
		cr.Recorder.WriteHeader(http.StatusBadRequest)
		return
	}
	h, _ := cr.Handler(request)
	h.ServeHTTP(cr.Recorder, request)
}

func bootStrapKcpMockHandlers() *CustomRouter {
	kcpTestHandler := NewCustomRouter()
	for _, kcpModule := range kcpModulesList {
		handleFnPattern := fmt.Sprintf("/v1/%s/event", kcpModule)
		kcpTestHandler.HandleFunc(handleFnPattern, func(response http.ResponseWriter, r *http.Request) {
			reqBytes, err := io.ReadAll(r.Body)
			if err != nil {
				response.WriteHeader(http.StatusBadRequest)
			}
			watcherEvt := &internal.WatchEvent{}
			err = json.Unmarshal(reqBytes, watcherEvt)
			if err != nil {
				response.WriteHeader(http.StatusBadRequest)
			}
			_, err = response.Write(reqBytes)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
			}
		})
	}
	return kcpTestHandler
}

func getAdmissionHTTPRequest(operation admissionv1.Operation, crName, moduleName string,
	labels map[string]string,
) (*http.Request, error) {
	admissionReview, err := createAdmissionRequest(operation, crName, labels)
	if err != nil {
		return nil, err
	}
	bytesRequest, err := json.Marshal(admissionReview)
	if err != nil {
		return nil, err
	}
	return httptest.NewRequest(http.MethodGet, fmt.Sprintf("/validate/%s", moduleName),
		bytes.NewBuffer(bytesRequest)), nil
}

func createAdmissionRequest(operation admissionv1.Operation, crName string,
	labels map[string]string) (*admissionv1.AdmissionReview, error) {
	admissionReview := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Name:      crName,
			Operation: operation,
			UID:       types.UID(uuid.NewString()),
		},
	}
	if operation == admissionv1.Delete {
		oldRawObject, err := generateAdmissionRequestRawObject(crName, labels, "oldSpecValue")
		if err != nil {
			return nil, err
		}
		admissionReview.Request.OldObject.Raw = oldRawObject
		return admissionReview, nil
	}
	if operation == admissionv1.Create || operation == admissionv1.Connect {
		rawObject, err := generateAdmissionRequestRawObject(crName, labels, "specValue")
		if err != nil {
			return nil, err
		}
		admissionReview.Request.Object.Raw = rawObject
		return admissionReview, nil
	}

	rawObject, err := generateAdmissionRequestRawObject(crName, labels, "specValue")
	if err != nil {
		return nil, err
	}
	admissionReview.Request.Object.Raw = rawObject

	oldRawObject, err := generateAdmissionRequestRawObject(crName, labels, "oldSpecValue")
	if err != nil {
		return nil, err
	}
	admissionReview.Request.OldObject.Raw = oldRawObject

	return admissionReview, nil
}

func generateAdmissionRequestRawObject(objectName string, labels map[string]string, specValue string,
) ([]byte, error) {
	objectWatched := &internal.ObjectWatched{
		Metadata: internal.Metadata{
			Name:      objectName,
			Namespace: metav1.NamespaceDefault,
			Labels:    labels,
		},
		Spec: map[string]interface{}{
			"specField": specValue,
		},
		Kind: testResourceKind,
	}
	rawObject, err := json.Marshal(objectWatched)
	if err != nil {
		return nil, err
	}

	return rawObject, nil
}
