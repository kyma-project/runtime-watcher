package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	"github.com/kyma-project/kyma-watcher/skr/pkg/config"
	"io/ioutil"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	Client client.Client
	Logger logr.Logger
}

const (
	admissionError = "admission request error"
)

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

func (h *Handler) Handle(writer http.ResponseWriter, req *http.Request) {
	ctx := context.TODO()
	kyma := h.getKymaFromConfigMap(ctx)

	// read incoming request to bytes
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		h.httpError(writer, http.StatusInternalServerError, fmt.Errorf("%s %w",
			admissionError, err))
		return
	}

	// create admission review from bytes
	admissionReview := h.getAdmissionRequestFromBytes(writer, body)
	if admissionReview == nil {
		return
	}

	h.Logger.Info(fmt.Sprintf("incoming admission review for: %s", admissionReview.Request.Kind.Kind))

	// store incoming request
	err = ioutil.WriteFile("/tmp/request", body, 0644)
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

func (h *Handler) getKymaFromConfigMap(ctx context.Context) *unstructured.Unstructured {
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
		return nil
	}
	kymaGvr := schema.GroupVersionKind{}
	err = json.Unmarshal([]byte(kymaGvkStringified), &kymaGvr)
	if err != nil {
		h.Logger.Error(err, "kyma GVK could not me unmarshalled")
		return nil
	}

	// get SKR kyma
	kymasList := unstructured.UnstructuredList{}
	kymasList.SetGroupVersionKind(kymaGvr)
	err = h.Client.List(ctx, &kymasList)
	if err != nil {
		h.Logger.Error(err, "could not list kyma resources")
		return nil
	} else if len(kymasList.Items) != 1 {
		h.Logger.Error(fmt.Errorf("only one Kyma should exist in SKR"), "abort")
		return nil
	}

	return &kymasList.Items[0]
}

func (h *Handler) httpError(w http.ResponseWriter, code int, err error) {
	h.Logger.Error(err, "")
	http.Error(w, err.Error(), code)
}

func (h *Handler) getAdmissionRequestFromBytes(w http.ResponseWriter, body []byte) *admissionv1.AdmissionReview {
	admissionReview := admissionv1.AdmissionReview{}
	if _, _, err := universalDeserializer.Decode(body, nil, &admissionReview); err != nil {
		h.httpError(w, http.StatusBadRequest, fmt.Errorf("%s %w", admissionError, err))
		return nil
	} else if admissionReview.Request == nil {
		h.httpError(w, http.StatusBadRequest, fmt.Errorf("%s empty request", admissionError))
		return nil
	}
	return &admissionReview
}
