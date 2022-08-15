package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"sync"

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
	allowed bool
	message string
	status  string
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
	admissionError      = "admission error"
	errorSeparator      = ":"
	invalidationMessage = "invalidated from webhook"
	validationMessage   = "validated from webhook"
	defaultBufferSize   = 2048
	requestStorePath    = "/tmp/request"
)

//nolint:gochecknoglobals
var UniversalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()

func (h *Handler) Handle(writer http.ResponseWriter, req *http.Request) {
	ctx := context.TODO()
	kyma, resourceList := h.getKymaAndResourceListFromConfigMap(ctx)

	// read incoming request to bytes
	body, err := io.ReadAll(req.Body)
	if err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			h.Logger.Error(fmt.Errorf("%s%s %w", admissionError, errorSeparator, err), "")
		}
		return
	}

	// create admission review from bytes
	admissionReview := h.getAdmissionRequestFromBytes(body)
	if admissionReview == nil {
		return
	}

	resourceValidation := h.validateResources(admissionReview, resourceList, kyma.GetName())

	h.Logger.Info(
		fmt.Sprintf("incoming admission review for: %s", admissionReview.Request.Kind.Kind),
	)

	// store incoming request
	sideCarEnabled, err := strconv.ParseBool(os.Getenv("WEBHOOK_SIDE_CAR"))
	if err != nil {
		h.Logger.Error(fmt.Errorf("cannot parse sidecar enable env variable %w", err), "")
		return
	}

	if sideCarEnabled {
		storeRequest := &storeRequest{
			logger: h.Logger,
			path:   requestStorePath,
			mu:     sync.Mutex{},
		}
		go storeRequest.save(body)
	}

	// prepare response
	responseBytes := h.prepareResponse(admissionReview, resourceValidation)
	if responseBytes == nil {
		return
	}
	if _, err = writer.Write(responseBytes); err != nil {
		h.Logger.Error(err, "")
		return
	}
}

func (h *Handler) prepareResponse(admissionReview *admissionv1.AdmissionReview,
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
				string(admissionReview.Request.Operation), invalidationMessage),
		)
	} else {
		h.Logger.Info(
			fmt.Sprintf("%s %s %s", admissionReview.Request.Kind.Kind,
				string(admissionReview.Request.Operation), validationMessage),
		)
	}

	admissionReviewBytes, err := json.Marshal(&finalizedAdmissionReview)
	if err != nil {
		h.Logger.Error(fmt.Errorf("%s%s %w", admissionError, errorSeparator, err), "")
		return nil
	}
	return admissionReviewBytes
}

func (h *Handler) validateResources(admissionReview *admissionv1.AdmissionReview,
	resourcesList []Resource, kymaName string,
) validateResource {
	if admissionReview.Request.Operation != admissionv1.Update {
		return h.validAdmissionReviewObj(
			fmt.Sprintf("%s operation not supported", admissionReview.Request.Operation), metav1.StatusFailure,
		)
	}

	var resource Resource
	for _, resourceItem := range resourcesList {
		if admissionReview.Request.Kind.String() != resourceItem.GroupVersionKind.String() {
			continue
		}
		resource = resourceItem
	}

	// Note: OldObject is nil for "CONNECT" and "CREATE" operations
	// old object
	oldObjectWatched := ObjectWatched{}
	validatedResource := h.unmarshalRawObj(admissionReview.Request.OldObject.Raw,
		&oldObjectWatched)
	if !validatedResource.allowed {
		return validatedResource
	}

	// new object
	objectWatched := ObjectWatched{}
	validatedResource = h.unmarshalRawObj(admissionReview.Request.Object.Raw,
		&objectWatched)
	if !validatedResource.allowed {
		return validatedResource
	}

	// send valid admission response - under all circumstances!
	return h.validAdmissionReviewObj(h.evaluateRequestForKcp(resource, oldObjectWatched, objectWatched, kymaName))
}

func (h *Handler) evaluateRequestForKcp(resource Resource, oldObjectWatched ObjectWatched,
	objectWatched ObjectWatched, kymaName string,
) (string, string) {
	var message, status string
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
	return message, status
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
		return metav1.StatusFailure
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

func (h *Handler) unmarshalRawObj(rawBytes []byte, response responseInterface,
) validateResource {
	if err := json.Unmarshal(rawBytes, response); err != nil || response.isEmpty() {
		h.Logger.Error(fmt.Errorf("admission review resource object could not be unmarshaled %s%s %w",
			admissionError, errorSeparator, err), "")
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
		return nil, nil
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
	}
	if len(kymasList.Items) > 1 {
		h.Logger.Error(fmt.Errorf("more than one Kyma exists in SKR"), "abort")
		return nil, nil
	} else if len(kymasList.Items) > 1 {
		h.Logger.Error(fmt.Errorf("no Kyma exists in SKR"), "abort")
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

// Uncomment lines below to throw http errors
// func (h *Handler) httpError(writer http.ResponseWriter, code int, err error) {
//	h.Logger.Error(err, "")
//	http.Error(writer, err.Error(), code)
//}

func (h *Handler) getAdmissionRequestFromBytes(body []byte) *admissionv1.AdmissionReview {
	admissionReview := admissionv1.AdmissionReview{}
	if _, _, err := UniversalDeserializer.Decode(body, nil, &admissionReview); err != nil {
		h.Logger.Error(fmt.Errorf("admission request could not be retreived, %s%s %w", admissionError,
			errorSeparator, err), "")
		return nil
	} else if admissionReview.Request == nil {
		h.Logger.Error(fmt.Errorf("admission request was empty, %s%s %w", admissionError, errorSeparator, err),
			"")
		return nil
	}
	return &admissionReview
}

func (h *Handler) validAdmissionReviewObj(message string, status string) validateResource {
	return validateResource{
		allowed: true,
		message: message,
		status:  status,
	}
}
