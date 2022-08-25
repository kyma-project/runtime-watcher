package internal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	"github.com/kyma-project/kyma-watcher/webhook/internal"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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

	objectSpec := map[string]interface{}{
		"specField": "value",
	}
	oldObjectSpec := map[string]interface{}{
		"specField": "oldValue",
	}
	oldRawObject, rawObject := generateAdmissionRequestObjects(crName, crGVK.Kind, oldObjectSpec, objectSpec)

	if oldRawObject == nil || rawObject == nil {
		return nil, fmt.Errorf("failed to generate request objects")
	}

	admissionReview.Request.Object.Raw = rawObject
	admissionReview.Request.OldObject.Raw = oldRawObject

	return admissionReview, nil
}

func generateAdmissionRequestObjects(objectName, objectKind string, oldObjectSpec, objectSpec map[string]interface{}) (oldRawObject []byte, rawObject []byte) {
	var err error
	rawObject, err = json.Marshal(generateObjectWatched(objectName, objectKind, objectSpec))
	if err != nil {
		return nil, nil
	}
	oldRawObject, err = json.Marshal(generateObjectWatched(objectName, objectKind, oldObjectSpec))
	if err != nil {
		return nil, nil
	}
	return oldRawObject, rawObject
}

func generateObjectWatched(objectName, objectKind string, objectSpec map[string]interface{}) *internal.ObjectWatched {
	return &internal.ObjectWatched{
		Metadata: internal.Metadata{
			Name:      objectName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: objectSpec,
		Kind: objectKind,
	}
}
