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
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	listenerTypes "github.com/kyma-project/runtime-watcher/listener/pkg/types"
	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"
)

const (
	HTTPClientTimeout        = time.Minute * 3
	eventEndpoint            = "event"
	admissionError           = "admission error"
	errorSeparator           = ":"
	invalidationMessage      = "invalidated from webhook"
	validationMessage        = "validated from webhook"
	kcpReqFailedMsg          = "kcp request failed"
	kcpReqSucceededMsg       = "kcp request succeeded"
	urlPathPattern           = "/validate/%s"
	ownedBy                  = "operator.kyma-project.io/owned-by"
	statusSubResource        = "status"
	namespaceNameEntityCount = 2
)

type Handler struct {
	Client       client.Client
	Logger       logr.Logger
	Config       serverconfig.ServerConfig
	Deserializer runtime.Decoder
}

type admissionResponseInfo struct {
	allowed bool
	message string
	status  string
}

type responseInterface interface {
	IsEmpty() bool
}

func (h *Handler) Handle(writer http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		h.Logger.Error(err, admissionError)
		return
	}

	admissionReview := h.parseAdmissionReview(body)
	if admissionReview == nil {
		return
	}

	h.Logger.Info(
		fmt.Sprintf("incoming admission review for: %s", admissionReview.Request.Kind.String()),
	)

	moduleName, err := getModuleName(req.URL.Path)
	if err != nil {
		h.Logger.Error(err, "failed to get module name")
		return
	}

	validation := h.validateResources(admissionReview, moduleName)

	h.Logger.Info(validation.message)

	responseBytes := h.prepareResponse(admissionReview, validation)
	if responseBytes == nil {
		h.Logger.Info("empty response from incoming admission review")
		return
	}
	if _, err = writer.Write(responseBytes); err != nil {
		h.Logger.Error(err, admissionError)
		return
	}
}

func (h *Handler) parseAdmissionReview(body []byte) *admissionv1.AdmissionReview {
	admissionReview := admissionv1.AdmissionReview{}
	if _, _, err := h.Deserializer.Decode(body, nil, &admissionReview); err != nil {
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

func getModuleName(urlPath string) (string, error) {
	var moduleName string
	_, err := fmt.Sscanf(urlPath, urlPathPattern, &moduleName)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("could not parse url path")
	}

	if err != nil && errors.Is(err, io.EOF) || moduleName == "" {
		return "", fmt.Errorf("module name must not be empty")
	}

	return moduleName, nil
}

func (h *Handler) prepareResponse(admissionReview *admissionv1.AdmissionReview,
	validation admissionResponseInfo,
) []byte {
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
		h.Logger.Error(err, admissionError)
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
		if oldObjectWatched.Spec != nil && objectWatched.Spec != nil {
			registerChange = !reflect.DeepEqual(oldObjectWatched.Spec, objectWatched.Spec)
		} else {
			// object watched doesn't have spec field
			// send request to kcp for all UPDATE events
			registerChange = true
		}
	case statusSubResource:
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
	ownerNamespace, ownerName, err := extractOwner(watched)
	if err != nil {
		h.Logger.Error(err, "resource owner name could not be determined")
		return "resource owner name could not be determined"
	}

	watcherEvent := &listenerTypes.WatchEvent{
		Owner:      client.ObjectKey{Namespace: ownerNamespace, Name: ownerName},
		Watched:    client.ObjectKey{Namespace: watched.Namespace, Name: watched.Name},
		WatchedGvk: metav1.GroupVersionKind(schema.FromAPIVersionAndKind(watched.APIVersion, watched.Kind)),
	}
	postBody, err := json.Marshal(watcherEvent)
	if err != nil {
		h.Logger.Error(err, kcpReqFailedMsg)
		return kcpReqFailedMsg
	}

	requestPayload := bytes.NewBuffer(postBody)

	if h.Config.KCPAddress == "" || h.Config.KCPContract == "" {
		return kcpReqFailedMsg
	}

	uri := fmt.Sprintf("%s/%s/%s/%s", h.Config.KCPAddress, h.Config.KCPContract, moduleName, eventEndpoint)
	httpClient, url, err := h.getHTTPClientAndURL(uri)
	if err != nil {
		h.Logger.Error(err, kcpReqFailedMsg)
		return err.Error()
	}

	resp, err := httpClient.Post(url, "application/json", requestPayload)
	if err != nil {
		h.Logger.Error(err, kcpReqFailedMsg, "postBody", watcherEvent)
		return kcpReqFailedMsg
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		h.Logger.Error(err, kcpReqFailedMsg, "postBody", watcherEvent)
		h.Logger.Error(err, fmt.Sprintf("responseBody: %s with StatusCode: %d", responseBody, resp.StatusCode))
		return kcpReqFailedMsg
	}

	h.Logger.Info(fmt.Sprintf("sent request to KCP successfully for resource %s/%s",
		watched.Namespace, watched.Name), "postBody", watcherEvent)
	return kcpReqSucceededMsg
}

func extractOwner(watched ObjectWatched) (namespace, name string, err error) {
	if watched.Annotations == nil || watched.Annotations[ownedBy] == "" {
		return namespace, name, fmt.Errorf("no '%s' annotation found for watched resource %s",
			ownedBy, watched.NamespacedName())
	}
	ownerKey := watched.Annotations[ownedBy]
	ownerParts := strings.Split(ownerKey, "/")
	if len(ownerParts) != namespaceNameEntityCount {
		return namespace, name, fmt.Errorf("annotation %s not set correctly on resource %s: %s",
			ownedBy, watched.NamespacedName(), ownerKey)
	}

	namespace = ownerParts[0]
	name = ownerParts[1]
	return namespace, name, nil
}

func (h *Handler) unmarshalRawObj(rawBytes []byte, response responseInterface,
) admissionResponseInfo {
	if err := json.Unmarshal(rawBytes, response); err != nil || response.IsEmpty() {
		h.Logger.Error(fmt.Errorf("admission review resource object could not be unmarshaled %s%s %w",
			admissionError, errorSeparator, err), "")
	}
	return h.validAdmissionReviewObj("")
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
