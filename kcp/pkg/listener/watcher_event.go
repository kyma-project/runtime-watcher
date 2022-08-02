package listener

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const kymaNameLabel = "operator.kyma-project.io/kyma-name"

type UnmarshalError struct {
	Message       string
	httpErrorCode int
}

func unmarshalSKREvent(r *http.Request) (*unstructured.Unstructured, *UnmarshalError) {
	pathVariables := strings.Split(r.URL.Path, "/")

	var contractVersion string
	_, err := fmt.Sscanf(pathVariables[1], "v%s", &contractVersion)

	if err != nil && !errors.Is(err, io.EOF) {
		return nil, &UnmarshalError{"could not read contract version", http.StatusBadRequest}
	}

	if err != nil && errors.Is(err, io.EOF) || contractVersion == "" {
		return nil, &UnmarshalError{"contract version cannot be empty", http.StatusBadRequest}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, &UnmarshalError{"could not read request body", http.StatusInternalServerError}
	}
	defer r.Body.Close()

	watcherEvent := &types.WatcherEvent{}
	err = json.Unmarshal(body, watcherEvent)
	if err != nil {
		return nil, &UnmarshalError{"could not unmarshal watcher event", http.StatusInternalServerError}
	}

	genericEvtObject := &unstructured.Unstructured{}
	genericEvtObject.SetName(watcherEvent.Name)
	genericEvtObject.SetNamespace(watcherEvent.Namespace)
	labels := make(map[string]string, 1)
	labels[kymaNameLabel] = watcherEvent.KymaCr
	genericEvtObject.SetLabels(labels)

	return genericEvtObject, nil
}
