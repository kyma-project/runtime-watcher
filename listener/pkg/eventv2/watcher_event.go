package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	typesv2 "github.com/kyma-project/runtime-watcher/listener/pkg/typesv2"
)

const (
	contentMapCapacity = 3
)

type UnmarshalError struct {
	Message       string
	HTTPErrorCode int
}

func UnmarshalSKREvent(req *http.Request) (*typesv2.WatchEvent, *UnmarshalError) {
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
	defer req.Body.Close()

	watcherEvent := &typesv2.WatchEvent{}
	err = json.Unmarshal(body, watcherEvent)
	if err != nil {
		return nil, &UnmarshalError{fmt.Sprintf("could not unmarshal watcher event: Body{%s}",
			string(body)), http.StatusInternalServerError}
	}

	return watcherEvent, nil
}

func GenericEvent(watcherEvent *typesv2.WatchEvent) *unstructured.Unstructured {
	genericEvtObject := &unstructured.Unstructured{}
	content := UnstructuredContent(watcherEvent)
	genericEvtObject.SetUnstructuredContent(content)
	genericEvtObject.SetName(watcherEvent.Owner.Name)
	genericEvtObject.SetNamespace(watcherEvent.Owner.Namespace)
	return genericEvtObject
}

func UnstructuredContent(watcherEvt *typesv2.WatchEvent) map[string]interface{} {
	content := make(map[string]interface{}, contentMapCapacity)
	content["owner"] = watcherEvt.Owner
	content["watched"] = watcherEvt.Watched
	content["watched-gvk"] = watcherEvt.WatchedGvk
	return content
}
