package listener

import (
	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const hostName = "http://localhost:8082"

type unmarshalTestCase struct {
	name       string
	urlPath    string
	payload    *types.WatcherEvent
	errMsg     string
	httpStatus int
}

func TestUnmarshalSKREvent(t *testing.T) {

	testWatcherEvt := &types.WatcherEvent{
		KymaCr:    "kyma",
		Name:      "kyma-sample",
		Namespace: "kyma-control-plane",
	}

	testCases := []unmarshalTestCase{
		{"happy path", "/v1/kyma/event", testWatcherEvt, "", http.StatusOK},
		{"missing contract version", "/r1/kyma/event", testWatcherEvt, "could not read contract version", http.StatusBadRequest},
		{"empty contract version", "/v/kyma/event", testWatcherEvt, "contract version cannot be empty", http.StatusBadRequest},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			//GIVEN
			req := newListenerRequest(t, http.MethodPost, hostName+testCase.urlPath, testWatcherEvt)
			//WHEN
			evtObject, err := unmarshalSKREvent(req)
			//THEN
			if err != nil {
				require.Equal(t, testCase.errMsg, err.Message)
				require.Equal(t, testCase.httpStatus, err.httpErrorCode)
				return
			}
			require.Equal(t, testCase.errMsg, "")
			require.Equal(t, testCase.httpStatus, http.StatusOK)
			require.Equal(t, testCase.payload.Name, evtObject.GetName())
			require.Equal(t, testCase.payload.Namespace, evtObject.GetNamespace())

		})
	}

}
