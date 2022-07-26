package listener

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"net/http"
	"strings"
)

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

	body, err := ioutil.ReadAll(r.Body)
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

	return genericEvtObject, nil
}
