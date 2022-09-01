package listener

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kyma-project/runtime-watcher/kcp/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const contentMapCapacity = 3

type UnmarshalError struct {
	Message       string
	HTTPErrorCode int
}

func UnmarshalSKREvent(req *http.Request) (*unstructured.Unstructured, *UnmarshalError) {
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

	watcherEvent := &types.WatcherEvent{}
	err = json.Unmarshal(body, watcherEvent)
	if err != nil {
		return nil, &UnmarshalError{"could not unmarshal watcher event", http.StatusInternalServerError}
	}

	genericEvtObject := &unstructured.Unstructured{}
	content := UnstructuredContent(watcherEvent)
	genericEvtObject.SetUnstructuredContent(content)

	return genericEvtObject, nil
}

func UnstructuredContent(watcherEvt *types.WatcherEvent) map[string]interface{} {
	content := make(map[string]interface{}, contentMapCapacity)
	content["name"] = watcherEvt.Name
	content["namespace"] = watcherEvt.Namespace
	content["kyma-name"] = watcherEvt.KymaCr
	return content
}
