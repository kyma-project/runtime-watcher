package listener_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/kyma-project/runtime-watcher/listener"

	"github.com/kyma-project/runtime-watcher/listener"

	"github.com/kyma-project/runtime-watcher/kcp/pkg/types"

	"github.com/stretchr/testify/require"
)

const hostname = "http://localhost:8082"

type unmarshalTestCase struct {
	name       string
	urlPath    string
	payload    *types.WatcherEvent
	errMsg     string
	httpStatus int
}

func TestUnmarshalSKREvent(t *testing.T) {
	t.Parallel()
	testWatcherEvt := &types.WatcherEvent{
		KymaCr:    "kyma",
		Name:      "kyma-sample",
		Namespace: "kyma-control-plane",
		KymaModules: []types.KymaModule{
			{
				Name:    "eventing",
				Channel: "rapid",
			},
			{
				Name:    "istio",
				Channel: "stable",
			},
		},
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

	for _, testCase := range testCases { //nolint:paralleltest
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			// GIVEN
			url := fmt.Sprintf("%s%s", hostname, testCase.urlPath)
			req := newListenerRequest(t, http.MethodPost, url, testWatcherEvt)
			// WHEN
			evtObject, err := listener.UnmarshalSKREvent(req)
			// THEN
			if err != nil {
				require.Equal(t, testCase.errMsg, err.Message)
				require.Equal(t, testCase.httpStatus, err.HTTPErrorCode)
				return
			}
			require.Equal(t, testCase.errMsg, "")
			require.Equal(t, testCase.httpStatus, http.StatusOK)
			testcasePayloadContent := listener.UnstructuredContent(testCase.payload)
			require.Equal(t, testcasePayloadContent, evtObject.Object)
		})
	}
}
