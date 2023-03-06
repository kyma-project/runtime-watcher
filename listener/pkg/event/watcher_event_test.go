package event_test

import (
	"fmt"
	"net/http"
	"testing"

	listenerEvent "github.com/kyma-project/runtime-watcher/listener/pkg/event"

	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/require"
)

const hostname = "http://localhost:8082"

type unmarshalTestCase struct {
	name       string
	urlPath    string
	payload    *types.WatchEvent
	errMsg     string
	httpStatus int
}

func TestUnmarshalSKREvent(t *testing.T) {
	t.Parallel()
	testWatcherEvt := &types.WatchEvent{
		Owner:      client.ObjectKey{Name: "kyma", Namespace: v1.NamespaceDefault},
		Watched:    client.ObjectKey{Name: "watched-resource", Namespace: v1.NamespaceDefault},
		WatchedGvk: v1.GroupVersionKind{Kind: "kyma", Group: "operator.kyma-project.io", Version: "v1alpha1"},
	}

	testCases := []unmarshalTestCase{
		{
			"happy path", "/v1/kyma/event",
			testWatcherEvt, "",
			http.StatusOK,
		},
		{
			"missing contract version", "/r1/kyma/event",
			testWatcherEvt, "could not read contract version",
			http.StatusBadRequest,
		},
		{
			"empty contract version", "/v/kyma/event",
			testWatcherEvt, "contract version cannot be empty",
			http.StatusBadRequest,
		},
	}
	for idx := range testCases { //nolint:paralleltest
		t.Run(testCases[idx].name, func(t *testing.T) {
			t.Parallel()
			testCase := testCases[idx]
			// GIVEN
			url := fmt.Sprintf("%s%s", hostname, testCase.urlPath)
			req := newListenerRequest(t, http.MethodPost, url, testWatcherEvt)
			// WHEN
			watcherEvent, err := listenerEvent.UnmarshalSKREvent(req)
			// THEN
			if err != nil {
				require.Equal(t, testCase.errMsg, err.Message)
				require.Equal(t, testCase.httpStatus, err.HTTPErrorCode)
				return
			}
			genericEvtObject := listenerEvent.GenericEvent(watcherEvent)
			require.Equal(t, testCase.errMsg, "")
			require.Equal(t, testCase.httpStatus, http.StatusOK)
			testcasePayloadContent := listenerEvent.UnstructuredContent(testCase.payload)
			require.Equal(t, testcasePayloadContent, genericEvtObject.Object)
		})
	}
}
