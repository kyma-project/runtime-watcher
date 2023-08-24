package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/uuid"
	listenerTypes "github.com/kyma-project/runtime-watcher/listener/pkg/types"
	"github.com/kyma-project/runtime-watcher/skr/internal"
)

type ChangeObj string

const (
	WatchedResourceKind                 = "testResourceKind"
	WatchedResourceAPIVersion           = "testGroup/testResourceVersion"
	SpecChange                ChangeObj = "spec"
	StatusChange              ChangeObj = "status subresource"
	NoChange                  ChangeObj = "no"
	NoSpecField               ChangeObj = "spec field is empty"
	specOrStatusKey                     = "key"
	specOrStatusOldValue                = "oldValue"
	specOrStatusNewValue                = "newValue"
)

var (
	OperationsToTest = []admissionv1.Operation{ //nolint:gochecknoglobals
		admissionv1.Connect,
		admissionv1.Update,
		admissionv1.Create, admissionv1.Delete,
	}
	ChangeObjTypes = []ChangeObj{ //nolint:gochecknoglobals
		NoChange,
		SpecChange,
		StatusChange, NoSpecField,
	}
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

func BootStrapKcpMockHandlers(moduleName string) *CustomRouter {
	kcpTestHandler := newCustomRouter()
	handleFnPattern := fmt.Sprintf("/v1/%s/event", moduleName)
	kcpTestHandler.HandleFunc(handleFnPattern, func(response http.ResponseWriter, r *http.Request) {
		reqBytes, err := io.ReadAll(r.Body)
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
		}
		watcherEvt := &listenerTypes.WatchEvent{}
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

func GetAdmissionHTTPRequest(operation admissionv1.Operation, watchedName, moduleName string,
	labels, annotations map[string]string, subResource ChangeObj,
) (*http.Request, error) {
	admissionReview, err := createAdmissionRequest(operation, watchedName, labels, annotations, subResource)
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
	labels, annotations map[string]string, changeObj ChangeObj,
) (*admissionv1.AdmissionReview, error) {
	admissionReview := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Name:      watchedName,
			Operation: operation,
			UID:       types.UID(uuid.NewString()),
			Kind: metav1.GroupVersionKind(schema.FromAPIVersionAndKind(WatchedResourceAPIVersion,
				WatchedResourceKind)),
		},
	}

	if changeObj == StatusChange {
		admissionReview.Request.SubResource = internal.StatusSubResource
	}

	if operation == admissionv1.Delete || operation == admissionv1.Update {
		oldRawObject, err := generateAdmissionRequestRawObject(watchedName, labels, annotations,
			true, changeObj)
		if err != nil {
			return nil, err
		}
		admissionReview.Request.OldObject.Raw = oldRawObject
	}
	if operation != admissionv1.Delete {
		rawObject, err := generateAdmissionRequestRawObject(watchedName, labels, annotations,
			false, changeObj)
		if err != nil {
			return nil, err
		}
		admissionReview.Request.Object.Raw = rawObject
	}

	return admissionReview, nil
}

func generateAdmissionRequestRawObject(objectName string, labels, annotations map[string]string,
	isOldObject bool, changeObj ChangeObj,
) ([]byte, error) {
	objectWatched := &internal.ObjectWatched{
		Metadata: internal.Metadata{
			Name:        objectName,
			Namespace:   metav1.NamespaceDefault,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec:       map[string]interface{}{},
		Status:     map[string]interface{}{},
		Kind:       WatchedResourceKind,
		APIVersion: WatchedResourceAPIVersion,
	}

	configuredObjectWatched := configureObjectWatched(objectWatched, isOldObject, changeObj)

	rawObject, err := json.Marshal(configuredObjectWatched)
	if err != nil {
		return nil, err
	}
	return rawObject, nil
}

func configureObjectWatched(objectWatched *internal.ObjectWatched,
	isOldObject bool, changeObj ChangeObj,
) *internal.ObjectWatched {
	if isOldObject {
		switch changeObj {
		case NoSpecField:
			objectWatched.Spec = nil
		case NoChange, SpecChange, StatusChange:
			fallthrough
		default:
			objectWatched.Status[specOrStatusKey] = specOrStatusOldValue
			objectWatched.Spec[specOrStatusKey] = specOrStatusOldValue
		}
	} else {
		switch changeObj {
		case SpecChange:
			objectWatched.Spec[specOrStatusKey] = specOrStatusNewValue
		case StatusChange:
			objectWatched.Status[specOrStatusKey] = specOrStatusNewValue
		case NoSpecField:
			objectWatched.Spec = nil
		case NoChange:
			fallthrough
		default:
			objectWatched.Status[specOrStatusKey] = specOrStatusOldValue
			objectWatched.Spec[specOrStatusKey] = specOrStatusOldValue
		}
	}
	return objectWatched
}
