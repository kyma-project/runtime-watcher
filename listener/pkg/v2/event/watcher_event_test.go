package event_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/certificate/utils"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	listenerEvent "github.com/kyma-project/runtime-watcher/listener/pkg/v2/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
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
		Owner:      types.ObjectKey{Name: "kyma", Namespace: v1.NamespaceDefault},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: v1.NamespaceDefault},
		WatchedGvk: v1.GroupVersionKind{Kind: "kyma", Group: "operator.kyma-project.io", Version: "v1alpha1"},
		SkrMeta:    types.SkrMeta{RuntimeId: "test-cert"},
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
			pemCert, err := utils.NewPemCertificateBuilder().Build()
			require.NoError(t, err)
			req := newListenerRequest(t, http.MethodPost, url, testWatcherEvt, pemCert)
			// WHEN
			currentWatcherEvent, unmarshalErr := listenerEvent.UnmarshalSKREvent(req)
			// THEN
			if unmarshalErr != nil {
				require.Equal(t, testCase.expectedErrMsg, unmarshalErr.Message)
				require.Equal(t, testCase.expectedHTTPStatus, unmarshalErr.HTTPErrorCode)
				return
			}
			require.Equal(t, testCase.expectedErrMsg, "")
			require.Equal(t, testCase.expectedHTTPStatus, http.StatusOK)
			require.Equal(t, testCase.expectedEvent, currentWatcherEvent)
		})
	}
}

func TestUnmarshalSKREvent_WhenNoCommonNameInClientCertificate_ReturnsError(t *testing.T) {
	t.Parallel()
	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "kyma", Namespace: v1.NamespaceDefault},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: v1.NamespaceDefault},
		WatchedGvk: v1.GroupVersionKind{Kind: "kyma", Group: "operator.kyma-project.io", Version: "v1alpha1"},
		SkrMeta:    types.SkrMeta{RuntimeId: ""},
	}
	url := hostname + "/v1/kyma/event"
	pemCert, err := utils.NewPemCertificateBuilder().WithCommonName("").Build()
	require.NoError(t, err)
	req := newListenerRequest(t, http.MethodPost, url, testWatcherEvt, pemCert)
	// WHEN
	_, unmarshalErr := listenerEvent.UnmarshalSKREvent(req)
	// THEN
	require.NotNil(t, unmarshalErr)
	require.Equal(t, "client certificate common name is empty", unmarshalErr.Message)
	require.Equal(t, http.StatusBadRequest, unmarshalErr.HTTPErrorCode)
}
