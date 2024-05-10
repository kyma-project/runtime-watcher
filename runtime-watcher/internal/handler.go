package internal

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/sethgrid/pester"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	listenerTypes "github.com/kyma-project/runtime-watcher/listener/pkg/types"

	"github.com/kyma-project/runtime-watcher/skr/internal/cacertificatehandler"
	"github.com/kyma-project/runtime-watcher/skr/internal/requestparser"
	"github.com/kyma-project/runtime-watcher/skr/internal/serverconfig"
	"github.com/kyma-project/runtime-watcher/skr/internal/watchermetrics"
)

const (
	HTTPTimeout              = time.Minute * 3
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
	h.logger.Info("Handle request - START")
	h.metrics.UpdateAdmissionRequestsTotal()
	start := time.Now()
	admissionReview, err := h.requestParser.ParseAdmissionReview(request)
	if err != nil {
		h.logger.Error(errors.Join(errAdmission, err), "failed to parse AdmissionReview")
		h.metrics.UpdateAdmissionRequestsErrorTotal()
		return
	}

	h.logger.Info("Incoming admission review for: " + admissionReview.Request.Kind.String())

	moduleName, err := getModuleName(request.URL.Path)
	if err != nil {
		h.logger.Error(err, "failed to get module name")
		return
	}

	validationMsg := h.validateResources(admissionReview.Request, moduleName)
	h.logger.Info(validationMsg)

	responseBytes := h.prepareResponse(admissionReview, validationMsg)
	if responseBytes == nil {
		h.logger.Info("Empty response from incoming admission review")
		return
	}
	if _, err = writer.Write(responseBytes); err != nil {
		h.logger.Error(err, admissionError)
		return
	}

	duration := time.Since(start)
	h.metrics.UpdateRequestDuration(duration)
	h.logger.Info("Handle request - END")
}

func getModuleName(urlPath string) (string, error) {
	var moduleName string
	_, err := fmt.Sscanf(urlPath, urlPathPattern, &moduleName)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", errors.New("could not parse url path")
	}

	if err != nil && errors.Is(err, io.EOF) || moduleName == "" {
		return "", errors.New("module name must not be empty")
	}

	return moduleName, nil
}

func (h *Handler) prepareResponse(admissionReview *admissionv1.AdmissionReview,
	validationMessage string,
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
				Message: validationMessage,
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

func (h *Handler) validateResources(request *admissionv1.AdmissionRequest, moduleName string,
) string {
	object, oldObject := WatchedObject{}, WatchedObject{}

	switch request.Operation {
	case admissionv1.Update:
		h.unmarshalWatchedObject(request.OldObject.Raw, &oldObject)
		h.unmarshalWatchedObject(request.Object.Raw, &object)
		resource := &Resource{
			GroupVersionKind: request.Kind,
			SubResource:      request.SubResource,
		}
		changed, err := h.checkForChange(resource, oldObject, object)
		if err != nil {
			h.metrics.UpdateFailedKCPTotal(watchermetrics.ReasonSubresource)
			return err.Error()
		}
		if !changed {
			return fmt.Sprintf("no change detected on watched resource %s/%s",
				object.Namespace, object.Name)
		}
		err = h.sendRequestToKcp(moduleName, object)
		if err != nil {
			return err.Error()
		}
	case admissionv1.Delete:
		h.unmarshalWatchedObject(request.OldObject.Raw, &oldObject)
		err := h.sendRequestToKcp(moduleName, oldObject)
		if err != nil {
			return err.Error()
		}
	case admissionv1.Create:
		h.unmarshalWatchedObject(request.Object.Raw, &object)
		err := h.sendRequestToKcp(moduleName, object)
		if err != nil {
			return err.Error()
		}
	case admissionv1.Connect:
		return fmt.Sprintf("operation %s not supported for %s", admissionv1.Connect, request.Kind.String())
	}
	return kcpReqSucceededMsg
}

var (
	errAdmission  = errors.New(admissionError)
	errKcpRequest = errors.New(kcpReqFailedMsg)
)

func (h *Handler) unmarshalWatchedObject(rawBytes []byte, response responseInterface) {
	if err := json.Unmarshal(rawBytes, response); err != nil {
		h.logger.Error(errors.Join(errAdmission, err), "failed to unmarshal admission review resource object")
	}
	if response.IsEmpty() {
		h.logger.Error(errAdmission, "admission review resource object is empty")
	}
}

func (h *Handler) checkForChange(resource *Resource, oldObj, obj WatchedObject) (bool, error) {
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
		return false, fmt.Errorf("invalid subresource for watched resource %s/%s",
			obj.Namespace, obj.Name)
	}

	return registerChange, nil
}

func (h *Handler) sendRequestToKcp(moduleName string, watched WatchedObject) error {
	h.metrics.UpdateKCPTotal()

	owner, err := extractOwner(watched)
	if err != nil {
		return h.logAndReturnKCPErr(err, watchermetrics.ReasonOwner)
	}

	watcherEvent := &listenerTypes.WatchEvent{
		Owner:      client.ObjectKey{Namespace: owner.Namespace, Name: owner.Name},
		Watched:    client.ObjectKey{Namespace: watched.Namespace, Name: watched.Name},
		WatchedGvk: metav1.GroupVersionKind(schema.FromAPIVersionAndKind(watched.APIVersion, watched.Kind)),
	}

	if h.config.KCPAddress == "" || h.config.KCPContract == "" {
		return h.logAndReturnKCPErr(errors.New("KCPAddress or KCPContract empty"), watchermetrics.ReasonKcpAddress)
	}

	url := fmt.Sprintf("https://%s/%s/%s/%s", h.config.KCPAddress, h.config.KCPContract, moduleName, eventEndpoint)
	httpsClient, err := h.getHTTPSClient()
	if err != nil {
		return h.logAndReturnKCPErr(err, watchermetrics.ReasonRequest)
	}

	resilientClient := pester.NewExtendedClient(httpsClient)
	resilientClient.Backoff = pester.ExponentialBackoff
	resilientClient.MaxRetries = 3
	resilientClient.KeepLog = true

	postBody, err := json.Marshal(watcherEvent)
	if err != nil {
		return h.logAndReturnKCPErr(err, watchermetrics.ReasonRequest)
	}
	resp, err := resilientClient.Post(url, "application/json", bytes.NewBuffer(postBody))
	if err != nil {
		err = errors.Join(errKcpRequest, err)
		h.logger.Error(err, resilientClient.LogString(), "postBody", watcherEvent)
		h.metrics.UpdateFailedKCPTotal(watchermetrics.ReasonResponse)
		return err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		err = errors.Join(errKcpRequest, err)
		h.logger.Error(err, err.Error(), "postBody", watcherEvent)
		h.metrics.UpdateFailedKCPTotal(watchermetrics.ReasonResponse)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("%w: responseBody: %s with StatusCode: %d", errKcpRequest, responseBody, resp.StatusCode)
		h.logger.Error(err, err.Error(), "postBody", watcherEvent)
		h.metrics.UpdateFailedKCPTotal(watchermetrics.ReasonResponse)
		return err
	}

	h.logger.Info(fmt.Sprintf("sent request to KCP successfully for resource %s/%s",
		watched.Namespace, watched.Name), "postBody", watcherEvent)
	return nil
}

func (h *Handler) logAndReturnKCPErr(err error, reason watchermetrics.KcpErrReason) error {
	err = errors.Join(errKcpRequest, err)
	h.logger.Error(err, err.Error())
	h.metrics.UpdateFailedKCPTotal(reason)
	return err
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

	rootCertPool, err := cacertificatehandler.GetCertificatePool(h.config.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate pool:%w", err)
	}

	httpsClient.Timeout = HTTPTimeout
	//nolint:gosec
	httpsClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
			RootCAs:      rootCertPool,
		},
	}

	return &httpsClient, nil
}
