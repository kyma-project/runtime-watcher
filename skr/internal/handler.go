package internal

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"
	"io"
	"k8s.io/apimachinery/pkg/runtime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	HTTPClientTimeout = time.Minute * 3
	eventEndpoint     = "event"
)

type Handler struct {
	Client       client.Client
	Logger       logr.Logger
	Config       serverconfig.ServerConfig
	Deserializer runtime.Decoder
}

type responseInterface interface {
	isEmpty() bool
}

type Resource struct {
	metav1.GroupVersionKind `json:"groupVersionKind"`
	SubResource             string `json:"subResource"`
}

type Metadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Annotations map[string]string `json:"annotations"`
	Labels      map[string]string `json:"labels"`
}

func (m Metadata) isEmpty() bool {
	return m.Name == ""
}

func (m Metadata) NamespacedName() string {
	return fmt.Sprintf("%s/%s", m.Namespace, m.Name)
}

type ObjectWatched struct {
	Metadata   `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Status     map[string]interface{} `json:"status"`
}

const (
	admissionError      = "admission error"
	errorSeparator      = ":"
	invalidationMessage = "invalidated from webhook"
	validationMessage   = "validated from webhook"
	kcpReqFailedMsg     = "kcp request failed"
	kcpReqSucceededMsg  = "kcp request succeeded"
	requestStorePath    = "/tmp/request"
	urlPathPattern      = "/validate/%s"
	ownedBy             = "operator.kyma-project.io/owned-by"
	delimiter           = "/"
	StatusSubResource   = "status"
	keyLen              = 2
)

func (h *Handler) Handle(writer http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		h.Logger.Error(err, admissionError)
		return
	}
	admissionReview, err := h.getAdmissionRequestFromBytes(body)
	if err != nil {
		h.Logger.Error(err, admissionError)
		return
	}

	h.Logger.Info(fmt.Sprintf("incoming admission review for: %s", admissionReview.Request.Kind.String()))

	moduleName, err := getModuleName(req.URL.Path)
	if err != nil {
		h.Logger.Error(err, "failed to get module name")
		return
	}

	//validation := h.validateResources(admissionReview, moduleName)

	var msg string
	switch admissionReview.Request.Operation {
	case admissionv1.Update:
		oldObj := ObjectWatched{}
		h.unmarshalRawObj(admissionReview.Request.OldObject.Raw, &oldObj)

		newObj := ObjectWatched{}
		h.unmarshalRawObj(admissionReview.Request.Object.Raw, &newObj)

		msg = h.sendRequestToKcpOnUpdate(&Resource{
			GroupVersionKind: admissionReview.Request.Kind,
			SubResource:      admissionReview.Request.SubResource,
		}, oldObj, newObj, moduleName)

		return validAdmissionReview(msg)
	case admissionv1.Delete:
		oldObjectWatched := ObjectWatched{}
		validatedResource := v.unmarshalRawObj(review.Request.OldObject.Raw,
			&oldObjectWatched)
		if !validatedResource.Allowed {
			return validatedResource
		}

		// send notification to kcp
		msg = h.sendRequestToKcp(moduleName, oldObjectWatched)

		// return valid admission response - under all circumstances!
		return validAdmissionReview(msg)
	case admissionv1.Create:
		objectWatched := ObjectWatched{}
		validatedResource := h.unmarshalRawObj(review.Request.Object.Raw,
			&objectWatched)
		if !validatedResource.allowed {
			return validatedResource
		}

		// send notification to kcp
		msg = h.sendRequestToKcp(moduleName, objectWatched)

		// return valid admission response - under all circumstances!
	case admissionv1.Connect:
		msg = fmt.Sprintf("operation %s not supported for resource %s",
			admissionv1.Connect, review.Request.Kind.String())
	}

	responseInfo := validAdmissionReview(msg)

	h.Logger.Info(responseInfo.Message)

	h.storeIncomingRequest(body)

	responseBytes, err := h.prepareResponse(admissionReview, responseInfo)
	if err != nil {
		h.Logger.Error(err, admissionError)
		return
	}

	if _, err = writer.Write(responseBytes); err != nil {
		h.Logger.Error(err, admissionError)
		return
	}
}

func (h *Handler) getAdmissionRequestFromBytes(body []byte) (*admissionv1.AdmissionReview, error) {
	admissionReview := admissionv1.AdmissionReview{}
	_, _, err := h.Deserializer.Decode(body, nil, &admissionReview)
	if err != nil {
		return nil, fmt.Errorf("admission request could not be decoded: %w", err)
	}

	if admissionReview.Request == nil {
		return &admissionReview, errors.New("decoded admission request is empty")
	}
	return &admissionReview, nil
}

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

func (h *Handler) unmarshalRawObj(rawBytes []byte, response responseInterface) {
	if err := json.Unmarshal(rawBytes, response); err != nil || response.isEmpty() {
		h.Logger.Error(fmt.Errorf("admission review resource object could not be unmarshaled %s%s %w",
			admissionError, errorSeparator, err), "")
	}
	return validAdmissionReview("")
}

func (h *Handler) storeIncomingRequest(body []byte) {
	// store incoming request
	enableSideCarStr := os.Getenv("WEBHOOK_SIDE_CAR")
	sideCarEnabled := false
	var err error
	if enableSideCarStr != "" {
		sideCarEnabled, err = strconv.ParseBool(enableSideCarStr)
		if err != nil {
			h.Logger.Error(err, "cannot parse sidecar enable env variable ")
			return
		}
	}

	if sideCarEnabled {
		storeRequest := &storeRequest{
			logger: h.Logger,
			path:   requestStorePath,
			mu:     sync.Mutex{},
		}
		go storeRequest.save(body)
	}
}

func (h *Handler) prepareResponse(admissionReview *admissionv1.AdmissionReview, validation AdmissionResponseInfo) ([]byte, error) {
	// prepare response object
	finalizedReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{},
		Request:  nil,
		Response: &admissionv1.AdmissionResponse{
			UID:     admissionReview.Request.UID,
			Allowed: validation.allowed,
			Result: &metav1.Status{
				Message: validation.message,
				Status:  validation.status,
			},
		},
	}
	finalizedReview.Kind = admissionReview.Kind
	finalizedReview.APIVersion = admissionReview.APIVersion

	info := fmt.Sprintf("%s %s", admissionReview.Request.Kind.Kind, string(admissionReview.Request.Operation))
	if validation.allowed {
		h.Logger.Info(fmt.Sprintf("%s %s", info, validationMessage))
	} else {
		h.Logger.Info(fmt.Sprintf("%s %s", info, invalidationMessage))
	}

	finalizedBytes, err := json.Marshal(&finalizedReview)
	if err != nil {
		return finalizedBytes, err
	}
	if finalizedBytes == nil {
		return nil, errors.New("empty response from incoming admission review")
	}

	return finalizedBytes, nil
}

func extractOwner(watched ObjectWatched) (namespace, name string, err error) {
	if watched.Annotations == nil || watched.Annotations[ownedBy] == "" {
		return namespace, name, fmt.Errorf("no '%s' annotation found for watched resource %s",
			ownedBy, watched.NamespacedName())
	}
	ownerKey := watched.Annotations[ownedBy]
	ownerParts := strings.Split(ownerKey, delimiter)
	if len(ownerParts) != keyLen {
		return namespace, name, fmt.Errorf("annotation %s not set correctly on resource %s: %s",
			ownedBy, watched.NamespacedName(), ownerKey)
	}

	namespace = ownerParts[0]
	name = ownerParts[1]
	return namespace, name, nil
}

func (h *Handler) getHTTPClientAndURL(uri string) (http.Client, string, error) {
	httpClient := http.Client{}
	protocol := "http"

	if h.Config.TLSCallbackEnabled {
		h.Logger.Info("will attempt to send an https request")
		protocol = "https"

		certificate, err := tls.LoadX509KeyPair(h.Config.TLSCert, h.Config.TLSKey)
		if err != nil {
			msg := "could not load tls certificate"
			return httpClient, msg, fmt.Errorf("%s :%w", msg, err)
		}

		caCertBytes, err := os.ReadFile(h.Config.CACert)
		if err != nil {
			msg := "could not load CA certificate"
			return httpClient, msg, fmt.Errorf("%s :%w", msg, err)
		}
		publicPemBlock, _ := pem.Decode(caCertBytes)
		rootPubCrt, errParse := x509.ParseCertificate(publicPemBlock.Bytes)
		if errParse != nil {
			msg := "failed to parse public key"
			return httpClient, msg, fmt.Errorf("%s :%w", msg, err)
		}
		rootCertPool := x509.NewCertPool()
		rootCertPool.AddCert(rootPubCrt)

		httpClient.Timeout = HTTPClientTimeout
		//nolint:gosec
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      rootCertPool,
				Certificates: []tls.Certificate{certificate},
			},
		}
	}

	url := fmt.Sprintf("%s://%s", protocol, uri)
	h.Logger.Info("KCP Address", "url", url)
	return httpClient, url, nil
}
