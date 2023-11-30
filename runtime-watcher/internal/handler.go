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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	listenerTypes "github.com/kyma-project/runtime-watcher/listener/pkg/types"
	"github.com/kyma-project/runtime-watcher/skr/internal/requestparser"
	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"
	"github.com/kyma-project/runtime-watcher/skr/internal/watchermetrics"
)

const (
	HTTPSClientTimeout       = time.Minute * 3
	eventEndpoint            = "event"
	admissionError           = "admission error"
	kcpReqFailedMsg          = "kcp request failed"
	kcpReqSucceededMsg       = "kcp request succeeded"
	urlPathPattern           = "/validate/%s"
	ownedBy                  = "operator.kyma-project.io/owned-by"
	statusSubResource        = "status"
	namespaceNameEntityCount = 2
)

func NewHandler(client client.Client,
	logger logr.Logger,
	config serverconfig.ServerConfig,
	parser requestparser.RequestParser,
	metrics watchermetrics.WatcherMetrics,
) *Handler {
	return &Handler{
		client:        client,
		logger:        logger,
		config:        config,
		requestParser: parser,
		metrics:       metrics,
	}
}

type Handler struct {
	client        client.Client
	logger        logr.Logger
	config        serverconfig.ServerConfig
	requestParser requestparser.RequestParser
	metrics       watchermetrics.WatcherMetrics
}

type responseInterface interface {
	IsEmpty() bool
}

func (h *Handler) Handle(writer http.ResponseWriter, request *http.Request) {
	admissionReview, err := h.requestParser.ParseAdmissionReview(request)
	if err != nil {
		h.logger.Error(errors.Join(errAdmission, err), "failed to parse AdmissionReview")
		return
	}
	someValue := 20
	h.metrics.UpdateSomething("handle_entry", float64(someValue))

	h.logger.Info(fmt.Sprintf("incoming admission review for: %s", admissionReview.Request.Kind.String()))

	moduleName, err := getModuleName(request.URL.Path)
	if err != nil {
		h.logger.Error(err, "failed to get module name")
		return
	}

	validationMsg := h.validateResources(admissionReview.Request, moduleName)

	h.logger.Info(string(validationMsg))

	responseBytes := h.prepareResponse(admissionReview, validationMsg)
	if responseBytes == nil {
		h.logger.Info("empty response from incoming admission review")
		return
	}
	if _, err = writer.Write(responseBytes); err != nil {
		h.logger.Error(err, admissionError)
		return
	}
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
	validationMessage admissionMessage,
) []byte {
	h.logger.Info(fmt.Sprintf("Preparing response for AdmissionReview: %s %s %s",
		admissionReview.Request.Kind.Kind,
		string(admissionReview.Request.Operation),
		validationMessage))

	finalizedAdmissionReview := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       admissionReview.Kind,
			APIVersion: admissionReview.APIVersion,
		},
		Response: &admissionv1.AdmissionResponse{
			UID:     admissionReview.Request.UID,
			Allowed: true,
			Result: &metav1.Status{
				Message: string(validationMessage),
				Status:  metav1.StatusSuccess,
			},
		},
	}

	admissionReviewBytes, err := json.Marshal(&finalizedAdmissionReview)
	if err != nil {
		h.logger.Error(err, admissionError)
		return nil
	}
	return admissionReviewBytes
}

type admissionMessage string

func (h *Handler) validateResources(request *admissionv1.AdmissionRequest, moduleName string,
) admissionMessage {
	object, oldObject := WatchedObject{}, WatchedObject{}

	switch request.Operation {
	case admissionv1.Update:
		h.unmarshalWatchedObject(request.OldObject.Raw, &oldObject)
		h.unmarshalWatchedObject(request.Object.Raw, &object)
		resource := &Resource{
			GroupVersionKind: request.Kind,
			SubResource:      request.SubResource,
		}
		msg := h.sendRequestToKcpOnUpdate(resource, oldObject, object, moduleName)
		return admissionMessage(msg)
	case admissionv1.Delete:
		h.unmarshalWatchedObject(request.OldObject.Raw, &oldObject)
		msg := h.sendRequestToKcp(moduleName, oldObject)
		return admissionMessage(msg)
	case admissionv1.Create:
		h.unmarshalWatchedObject(request.Object.Raw, &object)
		msg := h.sendRequestToKcp(moduleName, object)
		return admissionMessage(msg)
	case admissionv1.Connect:
		msg := fmt.Sprintf("operation %s not supported for %s", admissionv1.Connect, request.Kind.String())
		return admissionMessage(msg)
	}
	return ""
}

var errAdmission = errors.New(admissionError)

func (h *Handler) unmarshalWatchedObject(rawBytes []byte, response responseInterface) {
	if err := json.Unmarshal(rawBytes, response); err != nil {
		h.logger.Error(errors.Join(errAdmission, err), "failed to unmarshal admission review resource object")
	}
	if response.IsEmpty() {
		h.logger.Error(errAdmission, "admission review resource object is empty")
	}
}

func (h *Handler) sendRequestToKcpOnUpdate(resource *Resource, oldObj, obj WatchedObject,
	moduleName string,
) string {
	var registerChange bool
	// e.g. slice or status subresource. Only status is supported.
	watchedSubResource := strings.ToLower(resource.SubResource)

	switch watchedSubResource {
	// means watched on spec
	case "":
		if oldObj.Spec != nil && obj.Spec != nil {
			registerChange = !reflect.DeepEqual(oldObj.Spec, obj.Spec)
		} else {
			// object watched doesn't have spec field
			// send request to kcp for all UPDATE events
			registerChange = true
		}
	case statusSubResource:
		registerChange = !reflect.DeepEqual(oldObj.Status, obj.Status)
	default:
		return fmt.Sprintf("invalid subresource for watched resource %s/%s",
			obj.Namespace, obj.Name)
	}

	if !registerChange {
		return fmt.Sprintf("no change detected on watched resource %s/%s",
			obj.Namespace, obj.Name)
	}

	return h.sendRequestToKcp(moduleName, obj)
}

func (h *Handler) sendRequestToKcp(moduleName string, watched WatchedObject) string {
	owner, err := extractOwner(watched)
	if err != nil {
		h.logger.Error(err, "resource owner name could not be determined")
		return kcpReqFailedMsg
	}

	watcherEvent := &listenerTypes.WatchEvent{
		Owner:      client.ObjectKey{Namespace: owner.Namespace, Name: owner.Name},
		Watched:    client.ObjectKey{Namespace: watched.Namespace, Name: watched.Name},
		WatchedGvk: metav1.GroupVersionKind(schema.FromAPIVersionAndKind(watched.APIVersion, watched.Kind)),
	}
	postBody, err := json.Marshal(watcherEvent)
	if err != nil {
		h.logger.Error(err, kcpReqFailedMsg)
		return kcpReqFailedMsg
	}

	requestPayload := bytes.NewBuffer(postBody)

	if h.config.KCPAddress == "" || h.config.KCPContract == "" {
		return kcpReqFailedMsg
	}

	url := fmt.Sprintf("https://%s/%s/%s/%s", h.config.KCPAddress, h.config.KCPContract, moduleName, eventEndpoint)
	httpsClient, err := h.getHTTPSClient()
	if err != nil {
		h.logger.Error(err, kcpReqFailedMsg)
		return err.Error()
	}

	resp, err := httpsClient.Post(url, "application/json", requestPayload)
	if err != nil {
		h.logger.Error(err, kcpReqFailedMsg, "postBody", watcherEvent)
		return kcpReqFailedMsg
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Error(err, kcpReqFailedMsg, "postBody", watcherEvent)
		return kcpReqFailedMsg
	}
	if resp.StatusCode != http.StatusOK {
		h.logger.Error(fmt.Errorf("%w: responseBody: %s with StatusCode: %d", err, responseBody, resp.StatusCode),
			kcpReqFailedMsg, "postBody", watcherEvent)
		return kcpReqFailedMsg
	}

	h.logger.Info(fmt.Sprintf("sent request to KCP successfully for resource %s/%s",
		watched.Namespace, watched.Name), "postBody", watcherEvent)
	return kcpReqSucceededMsg
}

func extractOwner(watched WatchedObject) (types.NamespacedName, error) {
	if watched.Annotations == nil || watched.Annotations[ownedBy] == "" {
		return types.NamespacedName{}, fmt.Errorf("no '%s' annotation found for watched resource %s",
			ownedBy, watched.NamespacedName())
	}
	ownerKey := watched.Annotations[ownedBy]
	ownerParts := strings.Split(ownerKey, "/")
	if len(ownerParts) != namespaceNameEntityCount {
		return types.NamespacedName{}, fmt.Errorf("annotation %s not set correctly on resource %s: %s",
			ownedBy, watched.NamespacedName(), ownerKey)
	}

	return types.NamespacedName{Namespace: ownerParts[0], Name: ownerParts[1]}, nil
}

func (h *Handler) getHTTPSClient() (*http.Client, error) {
	httpsClient := http.Client{}

	certificate, err := tls.LoadX509KeyPair(h.config.TLSCertPath, h.config.TLSKeyPath)
	if err != nil {
		msg := "could not load tls certificate"
		return nil, fmt.Errorf("%s :%w", msg, err)
	}
	caCertBytes, err := os.ReadFile(h.config.CACertPath)
	if err != nil {
		msg := "could not load CA certificate"
		return nil, fmt.Errorf("%s :%w", msg, err)
	}
	publicPemBlock, _ := pem.Decode(caCertBytes)
	rootPubCrt, errParse := x509.ParseCertificate(publicPemBlock.Bytes)
	if errParse != nil {
		msg := "failed to parse public key"
		return nil, fmt.Errorf("%s :%w", msg, errParse)
	}
	rootCertPool := x509.NewCertPool()
	rootCertPool.AddCert(rootPubCrt)

	httpsClient.Timeout = HTTPSClientTimeout
	//nolint:gosec
	httpsClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
			RootCAs:      rootCertPool,
		},
	}

	return &httpsClient, nil
}
