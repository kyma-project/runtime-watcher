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

func newCustomRouter() *CustomRouter {
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
	kcpTestHandler := newCustomRouter()
	handleFnPattern := fmt.Sprintf("/v1/%s/event", moduleName)
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
	return kcpTestHandler
}

func getAdmissionHTTPRequest(operation admissionv1.Operation, watchedName, moduleName string,
	labels map[string]string, subResource bool,
) (*http.Request, error) {
	admissionReview, err := createAdmissionRequest(operation, watchedName, labels, subResource)
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

func createAdmissionRequest(operation admissionv1.Operation, watchedName string,
	labels map[string]string, subResource bool) (*admissionv1.AdmissionReview, error) {
	admissionReview := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Name:      watchedName,
			Operation: operation,
			UID:       types.UID(uuid.NewString()),
		},
	}
	if subResource {
		admissionReview.Request.SubResource = "status"
	}
	if operation == admissionv1.Delete || operation == admissionv1.Update {
		oldRawObject, err := generateAdmissionRequestRawObject(watchedName, labels, "oldValue", subResource)
		if err != nil {
			return nil, err
		}
		admissionReview.Request.OldObject.Raw = oldRawObject
	}
	if operation != admissionv1.Delete {
		rawObject, err := generateAdmissionRequestRawObject(watchedName, labels, "newValue", subResource)
		if err != nil {
			return nil, err
		}
		admissionReview.Request.Object.Raw = rawObject
	}

	return admissionReview, nil
}

func generateAdmissionRequestRawObject(objectName string, labels map[string]string, value string, subResource bool,
) ([]byte, error) {
	objectWatched := &internal.ObjectWatched{
		Metadata: internal.Metadata{
			Name:      objectName,
			Namespace: metav1.NamespaceDefault,
			Labels:    labels,
		},
		Spec:       map[string]interface{}{},
		Status:     map[string]interface{}{},
		Kind:       watchedResourceKind,
		APIVersion: watchedResourceAPIVersion,
	}
	if subResource {
		objectWatched.Status["someKey"] = value
	} else {
		objectWatched.Spec["someKey"] = value
	}
	rawObject, err := json.Marshal(objectWatched)
	if err != nil {
		return nil, err
	}

	return rawObject, nil
}
