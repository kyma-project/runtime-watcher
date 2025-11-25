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

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/certificate"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/certificate/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	listenerEvent "github.com/kyma-project/runtime-watcher/listener/pkg/v2/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/v2/types"
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

func newListenerRequest(t *testing.T,
	method, url string,
	watcherEvent *types.WatchEvent,
	encodedCertificate string,
) *http.Request {
	t.Helper()

	var body io.Reader
	if watcherEvent != nil {
		jsonBody, err := json.Marshal(watcherEvent)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewBuffer(jsonBody)
	}

	httpRequest, err := http.NewRequestWithContext(t.Context(), method, url, body)
	httpRequest.Header.Set(certificate.XFCCHeader, certificate.CertificateKey+encodedCertificate)

	if err != nil {
		t.Fatal(err)
	}
	return httpRequest
}

type GenericTestEvt struct {
	evt types.GenericEvent
	mu  sync.Mutex
}

func TestHandler(t *testing.T) {
	t.Parallel()
	// SETUP
	log := setupLogger()
	skrEventsListener := newTestListener(":8082", "kyma", log,
		func(_ *http.Request, _ *types.WatchEvent) error {
			return nil
		})

	handlerUnderTest := skrEventsListener.HandleSKREvent()
	responseRecorder := httptest.NewRecorder()

	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "kyma", Namespace: v1.NamespaceDefault},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: v1.NamespaceDefault},
		WatchedGvk: v1.GroupVersionKind{Kind: "kyma", Group: "operator.kyma-project.io", Version: "v1alpha1"},
		SkrMeta:    types.SkrMeta{RuntimeId: "test-cert"},
	}
	pemCert, err := utils.NewPemCertificateBuilder().Build()
	require.NoError(t, err)
	httpRequest := newListenerRequest(t, http.MethodPost, "http://localhost:8082/v1/kyma/event", testWatcherEvt,
		pemCert)
	testEvt := GenericTestEvt{}
	go func() {
		testEvt.mu.Lock()
		defer testEvt.mu.Unlock()
		testEvt.evt = <-skrEventsListener.ReceivedEvents()
	}()

	// WHEN
	handlerUnderTest(responseRecorder, httpRequest)

	// THEN
	resp := responseRecorder.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"mismatching status code: expected %d, got %d", http.StatusOK, resp.StatusCode)
	testEvt.mu.Lock()
	defer testEvt.mu.Unlock()
	assert.NotNil(t, testEvt.evt,
		"error reading event from channel: expected non nil event, got %v", testEvt.evt)
	testWatcherEvtContents := listenerEvent.UnstructuredContent(testWatcherEvt)
	for key, value := range testWatcherEvtContents {
		assert.Contains(t, testEvt.evt.Object.Object, key)
		assert.Equal(t, value, testEvt.evt.Object.Object[key])
	}
}

func TestMiddleware(t *testing.T) {
	t.Parallel()
	// SETUP
	log := setupLogger()
	skrEventsListener := newTestListener(":8082", "kyma", log,
		func(_ *http.Request, _ *types.WatchEvent) error {
			return nil
		})

	const successfulResponseString = "SUCCESS"
	const requestSizeLimitInBytes = 16384 // 16KB
	handlerUnderTest := http.MaxBytesHandler(skrEventsListener.RequestSizeLimitingMiddleware(
		func(writer http.ResponseWriter, _ *http.Request) {
			_, err := writer.Write([]byte(successfulResponseString))
			if err != nil {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
				return
			}
		}), requestSizeLimitInBytes)
	goodResponseRecorder := httptest.NewRecorder()
	badResponseRecorder := httptest.NewRecorder()

	// GIVEN
	// 200 bytes
	smallJSONFile, err := os.ReadFile("test_resources/small_size.json")
	if err != nil {
		t.Error(err)
	}
	// 32 KBs
	largeJSONFile, err := os.ReadFile("test_resources/large_size.json")
	if err != nil {
		t.Error(err)
	}

	goodHTTPRequest, _ := http.NewRequestWithContext(t.Context(),
		http.MethodPost, "http://test.url", bytes.NewBuffer(smallJSONFile))
	badHTTPRequest, _ := http.NewRequestWithContext(t.Context(),
		http.MethodPost, "http://test.url", bytes.NewBuffer(largeJSONFile))

	// WHEN
	handlerUnderTest.ServeHTTP(goodResponseRecorder, goodHTTPRequest)
	handlerUnderTest.ServeHTTP(badResponseRecorder, badHTTPRequest)

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
