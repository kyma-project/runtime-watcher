package event_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	listenerEvent "github.com/kyma-project/runtime-watcher/listener/pkg/event"

	"github.com/kyma-project/runtime-watcher/listener/pkg/types"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func newTestListener(addr, component string, log logr.Logger,
	verify listenerEvent.Verify,
) *listenerEvent.SKREventListener {
	listener := listenerEvent.NewSKREventListener(addr, component, verify)
	listener.Logger = log
	return listener
}

func setupLogger() logr.Logger {
	option := zap.AddStacktrace(zapcore.DebugLevel)
	zapLog := zap.NewExample(option)
	return zapr.NewLogger(zapLog)
}

func newListenerRequest(t *testing.T, method, url string, watcherEvent *types.WatchEvent) *http.Request {
	t.Helper()

	var body io.Reader
	if watcherEvent != nil {
		jsonBody, err := json.Marshal(watcherEvent)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewBuffer(jsonBody)
	}

	r, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

type GenericTestEvt struct {
	evt event.GenericEvent
	mu  sync.Mutex
}

func TestHandler(t *testing.T) {
	t.Parallel()
	// SETUP
	log := setupLogger()
	skrEventsListener := newTestListener(":8082", "kyma", log,
		func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		})

	handlerUnderTest := skrEventsListener.HandleSKREvent()
	responseRecorder := httptest.NewRecorder()

	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner:      client.ObjectKey{Name: "kyma", Namespace: v1.NamespaceDefault},
		Watched:    client.ObjectKey{Name: "watched-resource", Namespace: v1.NamespaceDefault},
		WatchedGvk: v1.GroupVersionKind{Kind: "kyma", Group: "operator.kyma-project.io", Version: "v1alpha1"},
	}
	httpRequest := newListenerRequest(t, http.MethodPost, "http://localhost:8082/v1/kyma/event", testWatcherEvt)
	testEvt := GenericTestEvt{}
	go func() {
		testEvt.mu.Lock()
		defer testEvt.mu.Unlock()
		testEvt.evt = <-skrEventsListener.ReceivedEvents
	}()

	// WHEN
	handlerUnderTest(responseRecorder, httpRequest)

	// THEN
	resp := responseRecorder.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"mismatching status code: expected %d, got %d", http.StatusOK, resp.StatusCode)
	testEvt.mu.Lock()
	defer testEvt.mu.Unlock()
	assert.NotEqual(t, nil, testEvt.evt,
		"error reading event from channel: expected non nil event, got %v", testEvt.evt)
	testWatcherEvtContents := listenerEvent.UnstructuredContent(testWatcherEvt)
	for key, value := range testWatcherEvtContents {
		assert.Contains(t, testEvt.evt.Object.(*unstructured.Unstructured).Object, key)
		assert.Equal(t, value, testEvt.evt.Object.(*unstructured.Unstructured).Object[key])
	}
}
