package listener

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/kyma-project/kyma-watcher/pkg/contract"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func newTestListener(addr, component string, log logr.Logger) *SKREventsListener {
	return &SKREventsListener{
		addr:           addr,
		logger:         log,
		componentName:  component,
		receivedEvents: make(chan event.GenericEvent),
	}
}

func setupLogger() logr.Logger {
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize zap logger: %v?", err))
	}
	return zapr.NewLogger(zapLog)
}

func newListenerRequest(t *testing.T, method, url string, watcherEvent *contract.WatcherEvent) *http.Request {

	var body io.Reader

	if watcherEvent != nil {
		json, err := json.Marshal(watcherEvent)

		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewBuffer(json)
	}

	r, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

type GenericTestEvt struct {
	evt event.GenericEvent
	sync.Mutex
}

func TestHandler(t *testing.T) {

	//SETUP
	log := setupLogger()
	skrEventsListener := newTestListener(":8082", "kyma", log)

	handlerUnderTest := skrEventsListener.handleSKREvent()
	respRec := httptest.NewRecorder()

	//GIVEN
	testWatcherEvt := &contract.WatcherEvent{
		KymaCr:    "kyma",
		Name:      "kyma-sample",
		Namespace: "kyma-operator",
	}
	req := newListenerRequest(t, http.MethodPost, "http://localhost:8082/v1/kyma/event", testWatcherEvt)
	testEvt := GenericTestEvt{}
	go func() {
		testEvt.Lock()
		defer testEvt.Unlock()
		testEvt.evt = <-skrEventsListener.ReceivedEvents()
	}()

	//WHEN
	handlerUnderTest(respRec, req)

	//THEN
	resp := respRec.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"mismatching status code: expected %d, got %d", http.StatusOK, resp.StatusCode)
	testEvt.Lock()
	defer testEvt.Unlock()
	assert.NotEqual(t, nil, testEvt.evt,
		"error reading event from channel: expected non nil event, got %v", testEvt.evt)
	assert.Equal(t, testWatcherEvt.Name, testEvt.evt.Object.GetName(),
		"mismatching event object name: expected %s, got %s", testWatcherEvt.Name, testEvt.evt.Object.GetName())
	assert.Equal(t, testWatcherEvt.Namespace, testEvt.evt.Object.GetNamespace(),
		"mismatching event object namespace: expected %s, got %s", testWatcherEvt.Namespace, testEvt.evt.Object.GetNamespace())

}
