package listener

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func newTestListener(addr, component string, log logr.Logger) *SKREventListener {
	return &SKREventListener{
		addr:           addr,
		logger:         log,
		componentName:  component,
		receivedEvents: make(chan event.GenericEvent),
	}
}

func setupLogger() logr.Logger {
	option := zap.AddStacktrace(zapcore.DebugLevel)
	zapLog := zap.NewExample(option)
	return zapr.NewLogger(zapLog)
}

func newListenerRequest(t *testing.T, method, url string, watcherEvent *types.WatcherEvent) *http.Request {
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
	// SETUP
	log := setupLogger()
	skrEventsListener := newTestListener(":8082", "kyma", log)

	handlerUnderTest := skrEventsListener.handleSKREvent()
	respRec := httptest.NewRecorder()

	// GIVEN
	testWatcherEvt := &types.WatcherEvent{
		KymaCr:    "kyma",
		Name:      "kyma-sample",
		Namespace: "kyma-operator",
	}
	req := newListenerRequest(t, http.MethodPost, "http://localhost:8082/v1/kyma/event", testWatcherEvt)
	testEvt := GenericTestEvt{}
	go func() {
		testEvt.mu.Lock()
		defer testEvt.mu.Unlock()
		testEvt.evt = <-skrEventsListener.ReceivedEvents()
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
	assert.Equal(t, testWatcherEvt.Name, testEvt.evt.Object.GetName(),
		"mismatching event object name: expected %s, got %s", testWatcherEvt.Name, testEvt.evt.Object.GetName())
	assert.Equal(t, testWatcherEvt.Namespace, testEvt.evt.Object.GetNamespace(),
		"mismatching event object namespace: expected %s, got %s", testWatcherEvt.Namespace, testEvt.evt.Object.GetNamespace())
}
