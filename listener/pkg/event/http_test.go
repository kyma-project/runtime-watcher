package event_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
	return &listenerEvent.SKREventListener{
		Addr:          addr,
		Logger:        log,
		ComponentName: component,
		VerifyFunc:    verify,
	}
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
		testEvt.evt = <-skrEventsListener.GetReceivedEvents()
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
func TestMiddleware(t *testing.T) {
	t.Parallel()
	// SETUP
	log := setupLogger()
	skrEventsListener := newTestListener(":8082", "kyma", log,
		func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		})

	const successfulResponseString = "SUCCESS"
	handlerUnderTest := skrEventsListener.Middleware(
		func(writer http.ResponseWriter, request *http.Request) {
			writer.Write([]byte(successfulResponseString))
		})
	goodResponseRecorder := httptest.NewRecorder()
	badResponseRecorder := httptest.NewRecorder()

	// GIVEN
	// 200 bytes
	smallJsonFile, err := os.ReadFile("test_resources/small_size.json")
	if err != nil {
		t.Error(err)
	}
	// 32 KBs
	largeJsonFile, err := os.ReadFile("test_resources/large_size.json")
	if err != nil {
		t.Error(err)
	}

	goodHttpRequest, _ := http.NewRequest(http.MethodPost, "http://test.url", bytes.NewBuffer(smallJsonFile))
	badHttpRequest, _ := http.NewRequest(http.MethodPost, "http://test.url", bytes.NewBuffer(largeJsonFile))

	// WHEN
	handlerUnderTest(goodResponseRecorder, goodHttpRequest)
	handlerUnderTest(badResponseRecorder, badHttpRequest)

	// THEN
	resp := goodResponseRecorder.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"mismatching status code: expected %d, got %d", http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, successfulResponseString, string(body),
		"mismatching body: expected %s, got %s", successfulResponseString, string(body))

	resp = badResponseRecorder.Result()
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode,
		"mismatching status code: expected %d, got %d", http.StatusRequestEntityTooLarge, resp.StatusCode)
	body, _ = io.ReadAll(resp.Body)
	assert.NotEqual(t, successfulResponseString, string(body),
		"mismatching body: expected NOT %s, got %s", successfulResponseString, string(body))
}
