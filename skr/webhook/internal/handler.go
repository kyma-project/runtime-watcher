package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	"github.com/kyma-project/kyma-watcher/skr/pkg/config"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	Client client.Client
	Logger logr.Logger
}

type validateResource struct {
	errorOccurred bool
	allowed       bool
	message       string
	status        string
}

type responseInterface interface {
	isEmpty() bool
}

type Resource struct {
	metav1.GroupVersionKind
	Status bool `json:"status"`
	Spec   bool `json:"spec"`
}

type Metadata struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels"`
}

func (m Metadata) isEmpty() bool {
	return m.Name == ""
}

type ObjectWatched struct {
	Metadata `json:"metadata"`
	Spec     map[string]interface{} `json:"spec"`
	Kind     string                 `json:"kind"`
	Status   map[string]interface{} `json:"status"`
}

const (
	AdmissionError       = "admission error:"
	InvalidResource      = "invalid resource"
	InvalidationMessage  = "invalidated from webhook"
	ValidationMessage    = "validated from webhook"
	writeFilePermissions = 0o600
	defaultBufferSize    = 2048
)

//nolint:gochecknoglobals
var UniversalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()

func (h *Handler) Handle(writer http.ResponseWriter, req *http.Request) {
	ctx := context.TODO()
	kyma, resourceList := h.getKymaAndResourceListFromConfigMap(ctx)

	// read incoming request to bytes
	body, err := io.ReadAll(req.Body)
	if err != nil {
		h.httpError(writer, http.StatusInternalServerError, fmt.Errorf("%s %w",
			AdmissionError, err))
		return
	}

	// create admission review from bytes
	admissionReview := h.getAdmissionRequestFromBytes(writer, body)
	if admissionReview == nil {
		return
	}

	resourceValidation := h.validateResources(writer, admissionReview, resourceList, kyma.GetName())

	h.Logger.Info(fmt.Sprintf("incoming admission review for: %s", admissionReview.Request.Kind.Kind))

	// store incoming request
	//nolint:gosec
	err = os.WriteFile("/tmp/request", body, writeFilePermissions)
	if err != nil {
		h.Logger.Error(err, "")
	}

	// prepare response
	responseBytes := h.prepareResponse(writer, admissionReview, resourceValidation)
	if responseBytes == nil {
		return
	}
	if _, err = writer.Write(responseBytes); err != nil {
		h.Logger.Error(err, "")
	}
}

func (h *Handler) prepareResponse(writer http.ResponseWriter, admissionReview *admissionv1.AdmissionReview,
	validation validateResource,
) []byte {
	// prepare response object
	finalizedAdmissionReview := admissionv1.AdmissionReview{}
	finalizedAdmissionReview.Kind = admissionReview.Kind
	finalizedAdmissionReview.APIVersion = admissionReview.APIVersion
	finalizedAdmissionReview.Response = &admissionv1.AdmissionResponse{
		UID:     admissionReview.Request.UID,
		Allowed: validation.allowed,
	}
	finalizedAdmissionReview.APIVersion = admissionReview.APIVersion

	finalizedAdmissionReview.Response.Result = &metav1.Status{
		Message: validation.message,
		Status:  validation.status,
	}
	if !validation.allowed {
		h.Logger.Info(
			fmt.Sprintf("%s %s %s", admissionReview.Request.Kind.Kind,
				string(admissionReview.Request.Operation), InvalidationMessage),
		)
	} else {
		h.Logger.Info(
			fmt.Sprintf("%s %s %s", admissionReview.Request.Kind.Kind,
				string(admissionReview.Request.Operation), ValidationMessage),
		)
	}

	bytes, err := json.Marshal(&finalizedAdmissionReview)
	if err != nil {
		h.httpError(writer, http.StatusInternalServerError, fmt.Errorf("%s %w", AdmissionError, err))
		return nil
	}
	return bytes
}

func (h *Handler) validateResources(writer http.ResponseWriter, admissionReview *admissionv1.AdmissionReview,
	resourcesList []Resource, kymaName string,
) validateResource {
	if admissionReview.Request.Operation != admissionv1.Update {
		return h.validAdmissionReviewObj(
			fmt.Sprintf("%s operation not supported", admissionReview.Request.Operation), metav1.StatusFailure,
		)
	}
	var message string
	var status string
	for _, resource := range resourcesList {
		if admissionReview.Request.Kind.String() != resource.GroupVersionKind.String() {
			continue
		}
		// Note: OldObject is nil for "CONNECT" and "CREATE" operations
		// old object
		oldObjectWatched := ObjectWatched{}
		validatedResource := h.unmarshalRawObj(writer, admissionReview.Request.OldObject.Raw,
			&oldObjectWatched, resource.GroupVersionKind.String())
		if !validatedResource.allowed {
			return validatedResource
		}

		// new object
		objectWatched := ObjectWatched{}
		validatedResource = h.unmarshalRawObj(writer, admissionReview.Request.Object.Raw,
			&objectWatched, resource.GroupVersionKind.String())
		if !validatedResource.allowed {
			return validatedResource
		}

		if resource.Status {
			if !reflect.DeepEqual(oldObjectWatched.Status, objectWatched.Status) {
				message = "sent requests to KCP for Status"
			}
		}

		if resource.Spec && message == "" {
			if !reflect.DeepEqual(oldObjectWatched.Spec, objectWatched.Spec) {
				message = "sent requests to KCP for Spec"
			}
		}

		if message != "" {
			status = h.sendRequestToKcp(kymaName, objectWatched)
		}

		// since resource was found - exit loop
		break
	}

	// send valid admission response - under all circumstances!
	return h.validAdmissionReviewObj(message, status)
}

func (h *Handler) sendRequestToKcp(kymaName string, watched ObjectWatched) string {
	// send request to kcp
	watcherEvent := &types.WatcherEvent{
		KymaCr:    kymaName,
		Namespace: watched.Namespace,
		Name:      watched.Name,
	}
	postBody, err := json.Marshal(watcherEvent)
	if err != nil {
		h.Logger.Error(err, "")
		return metav1.StatusFailure
	}

	responseBody := bytes.NewBuffer(postBody)

	kcpIP := os.Getenv("KCP_IP")
	kcpPort := os.Getenv("KCP_PORT")
	contract := os.Getenv("KCP_CONTRACT")
	component := "manifest"

	if kcpIP == "" || kcpPort == "" || contract == "" {
		return metav1.StatusSuccess
	}

	url := fmt.Sprintf("http://%s/%s/%s/%s", net.JoinHostPort(kcpIP, kcpPort),
		contract, component, config.EventEndpoint)

	h.Logger.Info("KCP", "url", url)
	//nolint:gosec
	resp, err := http.Post(url, "application/json", responseBody)
	if err != nil {
		h.Logger.Error(err, "")
		return metav1.StatusFailure
	}

	var response interface{}
	if err = yaml.NewYAMLOrJSONDecoder(resp.Body, defaultBufferSize).Decode(response); err != nil {
		h.Logger.Error(err, "response from KCP could not be unmarshalled")
		return metav1.StatusFailure
	}
	h.Logger.Info("response", response)
	return metav1.StatusSuccess
}

func (h *Handler) unmarshalRawObj(writer http.ResponseWriter, rawBytes []byte, response responseInterface,
	resourceKind string,
) validateResource {
	if err := json.Unmarshal(rawBytes, response); err != nil || response.isEmpty() {
		return h.invalidAdmissionReviewObj(writer, resourceKind, err)
	}
	return h.validAdmissionReviewObj("", "")
}

func (h *Handler) getKymaAndResourceListFromConfigMap(ctx context.Context) (*unstructured.Unstructured, []Resource) {
	// fetch resource mapping ConfigMap
	configMap := v1.ConfigMap{}
	err := h.Client.Get(ctx, client.ObjectKey{
		Name:      "skr-webhook-resource-mapping",
		Namespace: "default",
	}, &configMap)
	if err != nil {
		h.Logger.Error(err, "could not fetch resource mapping ConfigMap")
	}

	// parse ConfigMap for kyma GVK
	kymaGvkStringified, kymaExists := configMap.Data["kyma"]
	if !kymaExists {
		h.Logger.Error(fmt.Errorf("failed to fetch kyma GVK from resource mapping"), "")
		return nil, nil
	}
	kymaGvr := schema.GroupVersionKind{}
	err = yaml.Unmarshal([]byte(kymaGvkStringified), &kymaGvr)
	if err != nil {
		h.Logger.Error(err, "kyma GVK could not me unmarshalled")
		return nil, nil
	}

	// get SKR kyma
	kymasList := unstructured.UnstructuredList{}
	kymasList.SetGroupVersionKind(kymaGvr)
	err = h.Client.List(ctx, &kymasList)
	if err != nil {
		h.Logger.Error(err, "could not list kyma resources")
		return nil, nil
	} else if len(kymasList.Items) != 1 {
		h.Logger.Error(fmt.Errorf("only one Kyma should exist in SKR"), "abort")
		return nil, nil
	}

	resourceListStringified, listExists := configMap.Data["resources"]
	if !listExists {
		h.Logger.Error(fmt.Errorf("failed to fetch resources list resource mapping"), "")
	}
	var resources []Resource
	err = yaml.Unmarshal([]byte(resourceListStringified), &resources)
	if err != nil {
		h.Logger.Error(err, "resources list could not me unmarshalled")
		return nil, nil
	}

	return &kymasList.Items[0], resources
}

func (h *Handler) httpError(writer http.ResponseWriter, code int, err error) {
	h.Logger.Error(err, "")
	http.Error(writer, err.Error(), code)
}

func (h *Handler) getAdmissionRequestFromBytes(writer http.ResponseWriter, body []byte) *admissionv1.AdmissionReview {
	admissionReview := admissionv1.AdmissionReview{}
	if _, _, err := UniversalDeserializer.Decode(body, nil, &admissionReview); err != nil {
		h.httpError(writer, http.StatusBadRequest, fmt.Errorf("%s %w", AdmissionError, err))
		return nil
	} else if admissionReview.Request == nil {
		h.httpError(writer, http.StatusBadRequest, fmt.Errorf("%s empty request", AdmissionError))
		return nil
	}
	return &admissionReview
}

func (h *Handler) invalidAdmissionReviewObj(writer http.ResponseWriter, kind string, sourceErr error) validateResource {
	h.httpError(writer, http.StatusInternalServerError,
		fmt.Errorf("%s %s %s %w", InvalidResource, kind, AdmissionError, sourceErr))
	return validateResource{errorOccurred: true}
}

func (h *Handler) validAdmissionReviewObj(message string, status string) validateResource {
	return validateResource{
		allowed: true,
		message: message,
		status:  status,
	}
}
