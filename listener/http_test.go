package listener_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/kyma-project/runtime-watcher/listener"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/runtime-watcher/kcp/pkg/types"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func newTestListener(addr, component string, log logr.Logger) *listener.SKREventListener {
	return &listener.SKREventListener{
		Addr:          addr,
		Logger:        log,
		ComponentName: component,
	}
}

func setupLogger() logr.Logger {
	option := zap.AddStacktrace(zapcore.DebugLevel)
	zapLog := zap.NewExample(option)
	return zapr.NewLogger(zapLog)
}

func newListenerRequest(t *testing.T, method, url string, watcherEvent *types.WatcherEvent) *http.Request {
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
	skrEventsListener := newTestListener(":8082", "kyma", log)

	handlerUnderTest := skrEventsListener.HandleSKREvent()
	respRec := httptest.NewRecorder()

	// GIVEN
	testWatcherEvt := &types.WatcherEvent{
		KymaCr:    "kyma",
		Name:      "kyma-sample",
		Namespace: "lifecycle-manager",
	}
	req := newListenerRequest(t, http.MethodPost, "http://localhost:8082/v1/kyma/event", testWatcherEvt)
	testEvt := GenericTestEvt{}
	go func() {
		testEvt.mu.Lock()
		defer testEvt.mu.Unlock()
		testEvt.evt = <-skrEventsListener.GetReceivedEvents()
	}()

	// WHEN
	handlerUnderTest(respRec, req)

	// THEN
	resp := respRec.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"mismatching status code: expected %d, got %d", http.StatusOK, resp.StatusCode)
	testEvt.mu.Lock()
	defer testEvt.mu.Unlock()
	assert.NotEqual(t, nil, testEvt.evt,
		"error reading event from channel: expected non nil event, got %v", testEvt.evt)
	testWatcherEvtContents := listener.UnstructuredContent(testWatcherEvt)
	assert.Equal(t, testWatcherEvtContents, testEvt.evt.Object.(*unstructured.Unstructured).Object,
		"mismatching event object contents: expected %v, got %v",
		testWatcherEvtContents, testEvt.evt.Object.(*unstructured.Unstructured).Object)
}
