package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/certificate"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
)

const (
	contentMapCapacity = 4
)

type UnmarshalError struct {
	Message       string
	HTTPErrorCode int
}

func UnmarshalSKREvent(req *http.Request) (*types.WatchEvent, *UnmarshalError) {
	pathVariables := strings.Split(req.URL.Path, "/")

	var contractVersion string
	_, err := fmt.Sscanf(pathVariables[1], "v%s", &contractVersion)

	if err != nil && !errors.Is(err, io.EOF) {
		return nil, &UnmarshalError{"could not read contract version", http.StatusBadRequest}
	}

	if err != nil && errors.Is(err, io.EOF) || contractVersion == "" {
		return nil, &UnmarshalError{"contract version cannot be empty", http.StatusBadRequest}
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, &UnmarshalError{"could not read request body", http.StatusInternalServerError}
	}

	watcherEvent := &types.WatchEvent{}
	err = json.Unmarshal(body, watcherEvent)
	if err != nil {
		return nil, &UnmarshalError{
			fmt.Sprintf("could not unmarshal watcher event: Body{%s}",
				string(body)), http.StatusInternalServerError,
		}
	}

	skrMetaFromRequest, unmarshalError := getSkrMetaFromRequest(req)
	if unmarshalError != nil {
		return nil, unmarshalError
	}
	watcherEvent.SkrMeta = skrMetaFromRequest

	return watcherEvent, nil
}

func getSkrMetaFromRequest(req *http.Request) (types.SkrMeta, *UnmarshalError) {
	clientCertificate, err := certificate.GetCertificateFromHeader(req)
	if err != nil {
		return types.SkrMeta{}, &UnmarshalError{
			fmt.Sprintf("could not get client certificate from request: %v", err),
			http.StatusUnauthorized,
		}
	}

	if clientCertificate.Subject.CommonName == "" {
		return types.SkrMeta{}, &UnmarshalError{
			"client certificate common name is empty",
			http.StatusBadRequest,
		}
	}

	return types.SkrMeta{
		RuntimeId: clientCertificate.Subject.CommonName,
		SkrDomain: "", // this cannot be reliably extracted from the certificate.DNSNames slice
	}, nil
}

func GenericEvent(watcherEvent *types.WatchEvent) *unstructured.Unstructured {
	genericEvtObject := &unstructured.Unstructured{}
	content := UnstructuredContent(watcherEvent)
	genericEvtObject.SetUnstructuredContent(content)
	genericEvtObject.SetName(watcherEvent.Owner.Name)
	genericEvtObject.SetNamespace(watcherEvent.Owner.Namespace)
	return genericEvtObject
}

func UnstructuredContent(watcherEvt *types.WatchEvent) map[string]interface{} {
	content := make(map[string]interface{}, contentMapCapacity)
	content["owner"] = watcherEvt.Owner
	content["watched"] = watcherEvt.Watched
	content["watched-gvk"] = watcherEvt.WatchedGvk
	content["runtime-id"] = watcherEvt.SkrMeta.RuntimeId
	return content
}
