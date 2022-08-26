package internal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/kyma-project/kyma-watcher/webhook/internal"

	kcptypes "github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const kcpListenerUrlPattern = "/v1/%s/event"

type CustomRouter struct {
	*http.ServeMux
	Recorder *httptest.ResponseRecorder
}

func NewCustomRouter() *CustomRouter {
	return &CustomRouter{
		ServeMux: http.NewServeMux(),
		Recorder: httptest.NewRecorder(),
	}
}

func (cr *CustomRouter) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	w := cr.Recorder
	if r.RequestURI == "*" {
		if r.ProtoAtLeast(1, 1) {
			w.Header().Set("Connection", "close")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h, _ := cr.Handler(r)
	h.ServeHTTP(w, r)
}

func BootStrapKcpMockHandlers() *CustomRouter {
	kcpTestHandler := NewCustomRouter()
	for _, kcpModule := range kcpModulesList {
		handleFnPattern := fmt.Sprintf(kcpListenerUrlPattern, kcpModule)
		kcpTestHandler.HandleFunc(handleFnPattern, func(w http.ResponseWriter, r *http.Request) {
			reqBytes, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			watcherEvt := &kcptypes.WatcherEvent{}
			err = json.Unmarshal(reqBytes, watcherEvt)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
			}
			_, err = w.Write(reqBytes)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}
	return kcpTestHandler
}

func MockApiServerHTTPRequest(operation admissionv1.Operation, crName, moduleName string, crGVK metav1.GroupVersionKind) (*http.Request, error) {
	admissionReview, err := createAdmissionRequest(operation, crName, crGVK)
	if err != nil {
		return nil, err
	}
	bytesRequest, err := json.Marshal(admissionReview)
	if err != nil {
		return nil, err
	}
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/validate/%s", moduleName), bytes.NewBuffer(bytesRequest))
	return req, nil
}

func createAdmissionRequest(operation admissionv1.Operation, crName string, crGVK metav1.GroupVersionKind) (*admissionv1.AdmissionReview, error) {
	admissionReview := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Name:      crName,
			Kind:      crGVK,
			Operation: operation,
			UID:       types.UID(uuid.NewString()),
		},
	}
	if operation == admissionv1.Delete {
		oldRawObject, err := generateAdmissionRequestRawObject(crName, crGVK.Kind, "oldSpecValue")
		if err != nil {
			return nil, err
		}
		admissionReview.Request.OldObject.Raw = oldRawObject
		return admissionReview, nil
	}
	if operation == admissionv1.Create || operation == admissionv1.Connect {
		rawObject, err := generateAdmissionRequestRawObject(crName, crGVK.Kind, "specValue")
		if err != nil {
			return nil, err
		}
		admissionReview.Request.Object.Raw = rawObject
		return admissionReview, nil
	}

	rawObject, err := generateAdmissionRequestRawObject(crName, crGVK.Kind, "specValue")
	if err != nil {
		return nil, err
	}
	admissionReview.Request.Object.Raw = rawObject

	oldRawObject, err := generateAdmissionRequestRawObject(crName, crGVK.Kind, "oldSpecValue")
	if err != nil {
		return nil, err
	}
	admissionReview.Request.OldObject.Raw = oldRawObject

	return admissionReview, nil
}

func generateAdmissionRequestRawObject(objectName, objectKind, specValue string) ([]byte, error) {
	objectWatched := &internal.ObjectWatched{
		Metadata: internal.Metadata{
			Name:      objectName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: map[string]interface{}{
			"specField": specValue,
		},
		Kind: objectKind,
	}
	rawObject, err := json.Marshal(objectWatched)
	if err != nil {
		return nil, err
	}

	return rawObject, nil
}
