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
	"strconv"
	"sync"

	"github.com/go-logr/logr"

	"github.com/kyma-project/runtime-watcher/kcp/pkg/types"
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

const EventEndpoint = "event"

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
	requestStorePath    = "/tmp/request"
	kymaModuleName      = "kyma"
	urlPathPattern      = "/validate/%s"
	KcpReqFailedMsg     = "kcp request failed"
	KcpReqSucceededMsg  = "kcp request succeeded"
)

//nolint:gochecknoglobals
var UniversalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()

func getModuleName(urlPath string) (string, error) {
	var moduleName string
	_, err := fmt.Sscanf(urlPath, urlPathPattern, &moduleName)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("could not parse url path")
	}

	if err != nil && errors.Is(err, io.EOF) || moduleName == "" {
		return "", fmt.Errorf("module name cannot be empty")
	}

	return moduleName, nil
}

func (h *Handler) Handle(writer http.ResponseWriter, req *http.Request) {
	moduleName, err := getModuleName(req.URL.Path)
	if err != nil {
		h.Logger.Error(err, "failed to get module name")
		return
	}

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

	resourceValidation := h.validateResources(admissionReview, nil, moduleName)

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
	resourcesList []Resource, moduleName string,
) validateResource {
	if admissionReview.Request.Operation == admissionv1.Delete {
		oldObjectWatched := ObjectWatched{}
		validatedResource := h.unmarshalRawObj(admissionReview.Request.OldObject.Raw,
			&oldObjectWatched)
		if !validatedResource.allowed {
			return validatedResource
		}
		msg := h.sendRequestToKcp(moduleName, oldObjectWatched)
		// send valid admission response - under all circumstances!
		return h.validAdmissionReviewObj(msg)
	}

	objectWatched := ObjectWatched{}
	validatedResource := h.unmarshalRawObj(admissionReview.Request.Object.Raw,
		&objectWatched)
	if !validatedResource.allowed {
		return validatedResource
	}

	msg := h.sendRequestToKcp(moduleName, objectWatched)
	// send valid admission response - under all circumstances!
	return h.validAdmissionReviewObj(msg)
}

func getKymaFromConfigMap(reader client.Reader) (*unstructured.UnstructuredList, error) {
	// fetch resource mapping ConfigMap
	ctx := context.TODO()
	configMap := v1.ConfigMap{}
	err := reader.Get(ctx, client.ObjectKey{
		Name:      "skr-webhook-resource-mapping",
		Namespace: "default",
	}, &configMap)
	if err != nil {
		return nil, fmt.Errorf("could not fetch resource mapping ConfigMap: %w", err)
	}

	// parse ConfigMap for kyma GVK
	kymaGvkStringified, kymaExists := configMap.Data["kyma"]
	if !kymaExists {
		return nil, fmt.Errorf("failed to fetch kyma GVK from resource mapping")
	}
	kymaGvr := schema.GroupVersionKind{}
	err = yaml.Unmarshal([]byte(kymaGvkStringified), &kymaGvr)
	if err != nil {
		return nil, fmt.Errorf("kyma GVK could not me unmarshalled: %w", err)
	}

	// get SKR kyma
	kymasList := unstructured.UnstructuredList{}
	kymasList.SetGroupVersionKind(kymaGvr)
	err = reader.List(ctx, &kymasList)
	if err != nil {
		return nil, fmt.Errorf("could not list kyma resources: %w", err)
	}
	return &kymasList, nil
}

func (h *Handler) sendRequestToKcp(moduleName string, watched ObjectWatched) string {
	var kymaName string
	if moduleName == kymaModuleName {
		kymaName = watched.Metadata.Name
	} else {
		// get kyma name
		kymaList, err := getKymaFromConfigMap(h.Client)
		if err != nil {
			h.Logger.Error(err, "failed to get kyma list")
			return KcpReqFailedMsg
		}
		if len(kymaList.Items) != 1 {
			h.Logger.Error(nil, fmt.Sprintf("found %d kyma resources, expected 1", len(kymaList.Items)))
			return KcpReqFailedMsg
		}
		kymaName = kymaList.Items[0].GetName()
	}

	// send request to kcp
	watcherEvent := &types.WatcherEvent{
		KymaCr:    kymaName,
		Namespace: watched.Namespace,
		Name:      watched.Name,
	}
	postBody, err := json.Marshal(watcherEvent)
	if err != nil {
		h.Logger.Error(err, "")
		return KcpReqFailedMsg
	}

	responseBody := bytes.NewBuffer(postBody)

	kcpIP := os.Getenv("KCP_IP")
	kcpPort := os.Getenv("KCP_PORT")
	contract := os.Getenv("KCP_CONTRACT")

	if kcpIP == "" || kcpPort == "" || contract == "" {
		return KcpReqFailedMsg
	}

	url := fmt.Sprintf("http://%s/%s/%s/%s", net.JoinHostPort(kcpIP, kcpPort),
		contract, moduleName, EventEndpoint)

	h.Logger.V(1).Info("KCP", "url", url)
	//nolint:gosec
	resp, err := http.Post(url, "application/json", responseBody)
	if err != nil {
		h.Logger.Error(err, "")
		return KcpReqFailedMsg
	}
	if resp.StatusCode != http.StatusOK {
		h.Logger.Error(err, "")
		return KcpReqFailedMsg
	}

	h.Logger.Info("sent request to KCP successfully")
	return KcpReqSucceededMsg
}

func (h *Handler) unmarshalRawObj(rawBytes []byte, response responseInterface,
) validateResource {
	if err := json.Unmarshal(rawBytes, response); err != nil || response.isEmpty() {
		h.Logger.Error(fmt.Errorf("admission review resource object could not be unmarshaled %s%s %w",
			admissionError, errorSeparator, err), "")
	}
	return h.validAdmissionReviewObj("")
}

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

func (h *Handler) validAdmissionReviewObj(message string) validateResource {
	return validateResource{
		allowed: true,
		message: message,
		status:  metav1.StatusSuccess,
	}
}
