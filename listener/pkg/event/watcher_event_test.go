package event_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	listenerEvent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
)

const hostname = "http://localhost:8082"

type unmarshalTestCase struct {
	name               string
	urlPath            string
	expectedEvent      *types.WatchEvent
	expectedErrMsg     string
	expectedHTTPStatus int
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
	for idx := range testCases {
		testCase := testCases[idx]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("Testing %q for %q", testCase.name, testCase.urlPath)
			// GIVEN
			url := fmt.Sprintf("%s%s", hostname, testCase.urlPath)
			req := newListenerRequest(t, http.MethodPost, url, testWatcherEvt)
			// WHEN
			currentWatcherEvent, err := listenerEvent.UnmarshalSKREvent(req)
			// THEN
			if err != nil {
				require.Equal(t, testCase.expectedErrMsg, err.Message)
				require.Equal(t, testCase.expectedHTTPStatus, err.HTTPErrorCode)
				return
			}
			require.Equal(t, testCase.expectedErrMsg, "")
			require.Equal(t, testCase.expectedHTTPStatus, http.StatusOK)
			require.Equal(t, testCase.expectedEvent, currentWatcherEvent)
		})
	}
}
