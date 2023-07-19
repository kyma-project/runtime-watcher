package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	listenerTypes "github.com/kyma-project/runtime-watcher/listener/pkg/types"
	"io"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"net/http"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Validator struct {
}

type AdmissionResponseInfo struct {
	Allowed bool
	Message string
	Status  string
}

func validAdmissionReview(message string) AdmissionResponseInfo {
	return AdmissionResponseInfo{
		Allowed: true,
		Status:  metav1.StatusSuccess,
		Message: message,
	}
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
	namespace, name, err := extractOwner(watched)
	if err != nil {
		h.Logger.Error(err, "resource owner name could not be determined")
		return "resource owner name could not be determined"
	}

	watcherEvent := &listenerTypes.WatchEvent{
		Owner:      client.ObjectKey{Namespace: namespace, Name: name},
		Watched:    client.ObjectKey{Namespace: watched.Namespace, Name: watched.Name},
		WatchedGvk: metav1.GroupVersionKind(schema.FromAPIVersionAndKind(watched.APIVersion, watched.Kind)),
	}
	postBody, err := json.Marshal(watcherEvent)
	if err != nil {
		h.Logger.Error(err, kcpReqFailedMsg)
		return kcpReqFailedMsg
	}

	requestPayload := bytes.NewBuffer(postBody)

	kcpAddr := os.Getenv("KCP_ADDR")
	contract := os.Getenv("KCP_CONTRACT")

	if kcpAddr == "" || contract == "" {
		return kcpReqFailedMsg
	}

	uri := fmt.Sprintf("%s/%s/%s/%s", kcpAddr, contract, moduleName, eventEndpoint)
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
