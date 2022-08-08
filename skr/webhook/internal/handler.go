package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	"github.com/kyma-project/kyma-watcher/skr/pkg/config"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"net/http"
	"os"
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
}

type responseInterface interface {
	isEmpty() bool
}

type Resource struct {
	schema.GroupVersionKind
	Fields ResourceFields `json:"fields"`
}

type ResourceFields struct {
	Status []FieldKey `json:"status"`
	Spec   []FieldKey `json:"spec"`
}

type FieldKey struct {
	Field string `json:"field"`
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
}

const (
	LabelTenantType     = "sme.sap.com/tenant-type"
	SideCarEnv          = "WEBHOOK_SIDE_CAR"
	AdmissionError      = "admission error:"
	InvalidResource     = "invalid resource"
	InvalidationMessage = "invalidated from webhook"
	ValidationMessage   = "validated from webhook"
	RequestPath         = "/request"
)

var (
	UniversalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

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

	h.validateResources(writer, admissionReview, resourceList)

	h.Logger.Info(fmt.Sprintf("incoming admission review for: %s", admissionReview.Request.Kind.Kind))

	// store incoming request
	err = os.WriteFile("/tmp/request", body, 0644)
	if err != nil {
		panic(err.Error())
	}

	// send request to kcp
	watcherEvent := &types.WatcherEvent{
		KymaCr:    kyma.GetName(),
		Namespace: "default",
		Name:      "manifestkyma-sample",
	}
	postBody, _ := json.Marshal(watcherEvent)

	responseBody := bytes.NewBuffer(postBody)

	kcpIp := os.Getenv("KCP_IP")
	kcpPort := os.Getenv("KCP_PORT")
	contract := os.Getenv("KCP_CONTRACT")
	component := "manifest"

	url := fmt.Sprintf("http://%s:%s/%s/%s/%s", kcpIp, kcpPort, contract, component, config.EventEndpoint)
	fmt.Println("url" + url)
	resp, err := http.Post(url, "application/json", responseBody)

	if err != nil {
		fmt.Println("error" + err.Error())
	}
	fmt.Println(resp)
}

func (h *Handler) validateResources(writer http.ResponseWriter, admissionReview *admissionv1.AdmissionReview,
	resourcesList []Resource) validateResource {
	for _, resource := range resourcesList {
		objectWatched := ObjectWatched{}
		// Note: OldObject is nil for "CONNECT" and "CREATE" operations
		if admissionReview.Request.Operation == admissionv1.Update {
			if validatedResource := h.unmarshalRawObj(writer, admissionReview.Request.OldObject.Raw,
				&objectWatched, resource.GroupVersionKind.String()); !validatedResource.allowed {
				return validatedResource
			}
		}
	}
	return h.validAdmissionReviewObj()
}

func (h *Handler) unmarshalRawObj(writer http.ResponseWriter, rawBytes []byte, response responseInterface, resourceKind string) validateResource {
	if err := json.Unmarshal(rawBytes, response); err != nil || response.isEmpty() {
		return h.invalidAdmissionReviewObj(writer, resourceKind, err)
	}
	return h.validAdmissionReviewObj()
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

func (h *Handler) httpError(w http.ResponseWriter, code int, err error) {
	h.Logger.Error(err, "")
	http.Error(w, err.Error(), code)
}

func (h *Handler) getAdmissionRequestFromBytes(w http.ResponseWriter, body []byte) *admissionv1.AdmissionReview {
	admissionReview := admissionv1.AdmissionReview{}
	if _, _, err := UniversalDeserializer.Decode(body, nil, &admissionReview); err != nil {
		h.httpError(w, http.StatusBadRequest, fmt.Errorf("%s %w", AdmissionError, err))
		return nil
	} else if admissionReview.Request == nil {
		h.httpError(w, http.StatusBadRequest, fmt.Errorf("%s empty request", AdmissionError))
		return nil
	}
	return &admissionReview
}

func (h *Handler) invalidAdmissionReviewObj(writer http.ResponseWriter, kind string, sourceErr error) validateResource {
	h.httpError(writer, http.StatusInternalServerError,
		fmt.Errorf("%s %s %s %w", InvalidResource, kind, AdmissionError, sourceErr))
	return validateResource{errorOccurred: true}
}

func (h *Handler) validAdmissionReviewObj() validateResource {
	return validateResource{allowed: true}
}
