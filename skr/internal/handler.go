package internal

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-logr/logr"

	listenerTypes "github.com/kyma-project/runtime-watcher/listener/pkg/types"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	HTTPClientTimeout = time.Minute * 3
	EventEndpoint     = "event"
)

type Handler struct {
	Client     client.Client
	Logger     logr.Logger
	Parameters ServerParameters
}

type ServerParameters struct {
	Port        int    // webhook server port
	CACert      string // CA key used to sign the certificate
	TLSCert     string // path to TLS certificate for https
	TLSKey      string // path to TLS key matching for certificate
	TLSServer   bool   // indicates if an HTTPS server should be created
	TLSCallback bool   // indicates if KCP accepts HTTP or HTTPS requests
}

type admissionResponseInfo struct {
	allowed bool
	message string
	status  string
}

type responseInterface interface {
	isEmpty() bool
}

type Resource struct {
	metav1.GroupVersionKind `json:"groupVersionKind"`
	SubResource             string `json:"subResource"`
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
	Metadata   `json:"metadata"`
	Spec       map[string]interface{} `json:"spec"`
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Status     map[string]interface{} `json:"status"`
}

const (
	admissionError           = "admission error"
	errorSeparator           = ":"
	invalidationMessage      = "invalidated from webhook"
	validationMessage        = "validated from webhook"
	requestStorePath         = "/tmp/request"
	urlPathPattern           = "/validate/%s"
	KcpReqFailedMsg          = "kcp request failed"
	KcpReqSucceededMsg       = "kcp request succeeded"
	ManagedByLabel           = "operator.kyma-project.io/managed-by"
	OwnedByLabel             = "operator.kyma-project.io/owned-by"
	StatusSubResource        = "status"
	namespaceNameEntityCount = 2
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

	h.Logger.Info(
		fmt.Sprintf("incoming admission review for: %s", admissionReview.Request.Kind.String()),
	)

	validation := h.validateResources(admissionReview, moduleName)

	// log admission response message
	h.Logger.Info(validation.message)

	// store incoming request
	h.storeIncomingRequest(body)

	// prepare response
	responseBytes := h.prepareResponse(admissionReview, validation)
	if responseBytes == nil {
		return
	}
	if _, err = writer.Write(responseBytes); err != nil {
		h.Logger.Error(err, "")
		return
	}
}

func (h *Handler) storeIncomingRequest(body []byte) {
	// store incoming request
	enableSideCarStr := os.Getenv("WEBHOOK_SIDE_CAR")
	sideCarEnabled := false
	var err error
	if enableSideCarStr != "" {
		sideCarEnabled, err = strconv.ParseBool(enableSideCarStr)
		if err != nil {
			h.Logger.Error(fmt.Errorf("cannot parse sidecar enable env variable %w", err), "")
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

func (h *Handler) prepareResponse(admissionReview *admissionv1.AdmissionReview,
	validation admissionResponseInfo,
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

func (h *Handler) validateResources(admissionReview *admissionv1.AdmissionReview, moduleName string,
) admissionResponseInfo {
	var msg string
	switch admissionReview.Request.Operation {
	case admissionv1.Update:
		oldObjectWatched := ObjectWatched{}
		validatedResource := h.unmarshalRawObj(admissionReview.Request.OldObject.Raw,
			&oldObjectWatched)
		if !validatedResource.allowed {
			return validatedResource
		}

		objectWatched := ObjectWatched{}
		validatedResource = h.unmarshalRawObj(admissionReview.Request.Object.Raw, &objectWatched)
		if !validatedResource.allowed {
			return validatedResource
		}

		// send notification to kcp
		msg = h.sendRequestToKcpOnUpdate(&Resource{
			GroupVersionKind: admissionReview.Request.Kind,
			SubResource:      admissionReview.Request.SubResource,
		}, oldObjectWatched, objectWatched, moduleName)

		// send valid admission response - under all circumstances!
		return h.validAdmissionReviewObj(msg)
	case admissionv1.Delete:
		oldObjectWatched := ObjectWatched{}
		validatedResource := h.unmarshalRawObj(admissionReview.Request.OldObject.Raw,
			&oldObjectWatched)
		if !validatedResource.allowed {
			return validatedResource
		}

		// send notification to kcp
		msg = h.sendRequestToKcp(moduleName, oldObjectWatched)

		// return valid admission response - under all circumstances!
		return h.validAdmissionReviewObj(msg)
	case admissionv1.Create:
		objectWatched := ObjectWatched{}
		validatedResource := h.unmarshalRawObj(admissionReview.Request.Object.Raw,
			&objectWatched)
		if !validatedResource.allowed {
			return validatedResource
		}

		// send notification to kcp
		msg = h.sendRequestToKcp(moduleName, objectWatched)

		// return valid admission response - under all circumstances!
	case admissionv1.Connect:
		msg = fmt.Sprintf("operation %s not supported for resource %s",
			admissionv1.Connect, admissionReview.Request.Kind.String())
	}

	return h.validAdmissionReviewObj(msg)
}

func (h *Handler) sendRequestToKcpOnUpdate(resource *Resource, oldObjectWatched, objectWatched ObjectWatched,
	moduleName string,
) string {
	var registerChange bool
	// e.g. slice or status subresource. Only status is supported.
	watchedSubResource := strings.ToLower(resource.SubResource)

	switch watchedSubResource {
	// means watched on spec
	case "":
		registerChange = !reflect.DeepEqual(oldObjectWatched.Spec, objectWatched.Spec)
	case StatusSubResource:
		registerChange = !reflect.DeepEqual(oldObjectWatched.Status, objectWatched.Status)
	default:
		return fmt.Sprintf("invalid subresource for watched resource %s/%s",
			objectWatched.Namespace, objectWatched.Name)
	}

	if !registerChange {
		return fmt.Sprintf("no change detected on watched resource %s/%s",
			objectWatched.Namespace, objectWatched.Name)
	}

	return h.sendRequestToKcp(moduleName, objectWatched)
}

func (h *Handler) sendRequestToKcp(moduleName string, watched ObjectWatched) string {
	ownerKey, err := getKcpResourceName(watched)
	if err != nil {
		h.Logger.Error(err, "resource owner name could not be determined")
		return ""
	}

	ownerParts := strings.Split(ownerKey, "__")
	if len(ownerParts) != namespaceNameEntityCount {
		return fmt.Sprintf("label %s not set correctly on resource %s/%s: %s", OwnedByLabel,
			watched.Namespace, watched.Name, err.Error())
	}
	ownerNs := ownerParts[0]
	ownerName := ownerParts[1]

	// send request to kcp
	watcherEvent := &listenerTypes.WatchEvent{
		Owner:      client.ObjectKey{Namespace: ownerNs, Name: ownerName},
		Watched:    client.ObjectKey{Namespace: watched.Namespace, Name: watched.Name},
		WatchedGvk: metav1.GroupVersionKind(schema.FromAPIVersionAndKind(watched.APIVersion, watched.Kind)),
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

	if kcpIP == "" || contract == "" {
		return KcpReqFailedMsg
	}
	if kcpPort != "" {
		kcpIP = net.JoinHostPort(kcpIP, kcpPort)
	}

	uri := fmt.Sprintf("%s/%s/%s/%s", kcpIP, contract, moduleName, EventEndpoint)
	httpClient, url, err := h.getHTTPClientAndURL(uri)
	if err != nil {
		h.Logger.Error(err, "")
		return err.Error()
	}

	resp, err := httpClient.Post(url, "application/json", responseBody)
	if err != nil {
		h.Logger.Error(err, "")
		return KcpReqFailedMsg
	}
	if resp.StatusCode != http.StatusOK {
		h.Logger.Error(err, "")
		return KcpReqFailedMsg
	}

	h.Logger.Info(fmt.Sprintf("sent request to KCP successfully for resource %s/%s",
		watched.Namespace, watched.Name))
	return KcpReqSucceededMsg
}

func getKcpResourceName(watched ObjectWatched) (string, error) {
	if watched.Labels == nil || watched.Labels[OwnedByLabel] == "" {
		return "", fmt.Errorf("no labels found for watched resource %s/%s", watched.Namespace, watched.Name)
	}
	return watched.Labels[OwnedByLabel], nil
}

func (h *Handler) unmarshalRawObj(rawBytes []byte, response responseInterface,
) admissionResponseInfo {
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

func (h *Handler) validAdmissionReviewObj(message string) admissionResponseInfo {
	return admissionResponseInfo{
		allowed: true,
		message: message,
		status:  metav1.StatusSuccess,
	}
}

func (h *Handler) getHTTPClientAndURL(uri string) (http.Client, string, error) {
	httpClient := http.Client{}
	protocol := "http"

	if h.Parameters.TLSCallback {
		h.Logger.Info("will attempt to send an https request")
		protocol = "https"

		certificate, err := tls.LoadX509KeyPair(h.Parameters.TLSCert, h.Parameters.TLSKey)
		if err != nil {
			msg := "could not load tls certificate"
			return httpClient, msg, fmt.Errorf("%s :%w", msg, err)
		}

		caCertBytes, err := os.ReadFile(h.Parameters.CACert)
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
		rootCertpool := x509.NewCertPool()
		rootCertpool.AddCert(rootPubCrt)

		httpClient.Timeout = HTTPClientTimeout
		//nolint:gosec
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      rootCertpool,
				Certificates: []tls.Certificate{certificate},
			},
		}
	}

	url := fmt.Sprintf("%s://%s", protocol, uri)
	h.Logger.Info("KCP", "url", url)
	return httpClient, url, nil
}
