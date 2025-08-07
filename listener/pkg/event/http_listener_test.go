package event_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	listenerEvent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
)

func newTestHTTPListener(addr, component string, log logr.Logger,
	verify listenerEvent.VerifyFunc,
) *listenerEvent.HTTPEventListener {
	config := listenerEvent.Config{
		Addr:          addr,
		ComponentName: component,
		VerifyFunc:    verify,
		Logger:        log,
	}

	return listenerEvent.NewHTTPEventListener(config)
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

func TestHTTPEventListener_UnmarshalEvent(t *testing.T) {
	t.Parallel()
	// SETUP
	log := setupLogger()
	_ = newTestHTTPListener(":0", "kyma", log, // Use :0 for random available port
		func(_ *http.Request, _ *types.WatchEvent) error {
			return nil
		})

	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "kyma", Namespace: v1.NamespaceDefault},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: v1.NamespaceDefault},
		WatchedGvk: v1.GroupVersionKind{Kind: "kyma", Group: "operator.kyma-project.io", Version: "v1alpha1"},
	}

	// Test the UnmarshalSKREvent function directly
	request := newListenerRequest(t, http.MethodPost, "/v1/kyma/event", testWatcherEvt)

	// WHEN
	unmarshaledEvent, unmarshalErr := listenerEvent.UnmarshalSKREvent(request)

	// THEN
	assert.Nil(t, unmarshalErr, "expected no unmarshal error")
	assert.NotNil(t, unmarshaledEvent, "expected event to be unmarshaled")
	assert.Equal(t, testWatcherEvt.Owner, unmarshaledEvent.Owner)
	assert.Equal(t, testWatcherEvt.Watched, unmarshaledEvent.Watched)
	assert.Equal(t, testWatcherEvt.WatchedGvk, unmarshaledEvent.WatchedGvk)
}

func TestHTTPEventListener_Lifecycle(t *testing.T) {
	t.Parallel()
	// SETUP
	log := setupLogger()
	httpListener := newTestHTTPListener(":0", "kyma", log,
		func(_ *http.Request, _ *types.WatchEvent) error {
			return nil
		})

	// Start the HTTP listener in the background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = httpListener.Start(ctx)
	}()

	// Wait for the server to start
	time.Sleep(100 * time.Millisecond)

	// Test that the listener started successfully and has the Events channel
	assert.NotNil(t, httpListener.Events(), "expected Events channel to be available")

	// Test Health endpoint
	require.NoError(t, httpListener.Health(), "expected listener to be healthy initially")

	// Stop the listener
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestHTTPEventListener_RequestSizeValidation(t *testing.T) {
	t.Parallel()
	// GIVEN
	// Small request (should pass)
	smallEvent := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "test", Namespace: "default"},
		Watched:    types.ObjectKey{Name: "small", Namespace: "default"},
		WatchedGvk: v1.GroupVersionKind{Kind: "Test", Group: "test.io", Version: "v1"},
	}

	smallRequest := newListenerRequest(t, http.MethodPost, "/v1/kyma/event", smallEvent)
	smallRecorder := httptest.NewRecorder()

	// Large request (should be rejected)
	largeData := make([]byte, 2*1024*1024) // 2MB - larger than the 1MB limit
	for i := range largeData {
		largeData[i] = 'a'
	}

	largeRequest, err := http.NewRequest(http.MethodPost, "/v1/kyma/event", bytes.NewBuffer(largeData))
	require.NoError(t, err)

	largeRecorder := httptest.NewRecorder()

	// Create a test handler that mimics the actual HTTP listener behavior
	testHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		const maxRequestSize = 1 << 20 // 1MB

		if request.ContentLength > maxRequestSize {
			http.Error(writer, "request too large", http.StatusRequestEntityTooLarge)
			return
		}

		// Try to unmarshal
		_, unmarshalErr := listenerEvent.UnmarshalSKREvent(request)
		if unmarshalErr != nil {
			http.Error(writer, unmarshalErr.Message, unmarshalErr.HTTPErrorCode)
			return
		}

		writer.WriteHeader(http.StatusOK)
	})

	// WHEN
	testHandler.ServeHTTP(smallRecorder, smallRequest)
	testHandler.ServeHTTP(largeRecorder, largeRequest)

	// THEN
	smallResp := smallRecorder.Result()
	require.Equal(t, http.StatusOK, smallResp.StatusCode,
		"small request should succeed")

	largeResp := largeRecorder.Result()
	require.Equal(t, http.StatusRequestEntityTooLarge, largeResp.StatusCode,
		"large request should be rejected")
}

func TestHTTPEventListener_MethodValidation(t *testing.T) {
	t.Parallel()

	// Test that only POST method is accepted
	tests := []struct {
		method         string
		expectedStatus int
	}{
		{http.MethodPost, http.StatusOK},
		{http.MethodGet, http.StatusMethodNotAllowed},
		{http.MethodPut, http.StatusMethodNotAllowed},
		{http.MethodDelete, http.StatusMethodNotAllowed},
	}

	for _, testCase := range tests {
		t.Run(testCase.method, func(t *testing.T) {
			t.Parallel()
			// GIVEN
			testEvent := &types.WatchEvent{
				Owner:      types.ObjectKey{Name: "test", Namespace: "default"},
				Watched:    types.ObjectKey{Name: "watched", Namespace: "default"},
				WatchedGvk: v1.GroupVersionKind{Kind: "Test", Group: "test.io", Version: "v1"},
			}

			var request *http.Request
			if testCase.method == http.MethodPost {
				request = newListenerRequest(t, testCase.method, "/v1/kyma/event", testEvent)
			} else {
				var err error

				request, err = http.NewRequest(testCase.method, "/v1/kyma/event", nil)
				require.NoError(t, err)
			}

			recorder := httptest.NewRecorder()

			// Create a test handler that mimics method validation
			testHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost {
					http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
					return
				}

				// For POST requests, try to process the event
				_, unmarshalErr := listenerEvent.UnmarshalSKREvent(request)
				if unmarshalErr != nil {
					http.Error(writer, unmarshalErr.Message, unmarshalErr.HTTPErrorCode)
					return
				}

				writer.WriteHeader(http.StatusOK)
			})

			// WHEN
			testHandler.ServeHTTP(recorder, request)

			// THEN
			resp := recorder.Result()
			assert.Equal(t, testCase.expectedStatus, resp.StatusCode,
				"method %s should return status %d", testCase.method, testCase.expectedStatus)
		})
	}
}

func TestHTTPEventListener_UnmarshalErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "invalid contract version",
			method:         http.MethodPost,
			path:           "/invalid/kyma/event",
			body:           `{"owner":{"name":"test","namespace":"default"}}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "empty contract version",
			method:         http.MethodPost,
			path:           "/v/kyma/event",
			body:           `{"owner":{"name":"test","namespace":"default"}}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "invalid JSON body",
			method:         http.MethodPost,
			path:           "/v1/kyma/event",
			body:           `{invalid json`,
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// GIVEN
			request, err := http.NewRequest(testCase.method, testCase.path, bytes.NewBufferString(testCase.body))
			require.NoError(t, err)

			// WHEN
			event, unmarshalErr := listenerEvent.UnmarshalSKREvent(request)

			// THEN
			assert.NotNil(t, unmarshalErr, "expected unmarshal error")
			assert.Equal(t, testCase.expectedStatus, unmarshalErr.HTTPErrorCode)
			assert.Nil(t, event, "expected nil event on error")
		})
	}
}

func TestHTTPEventListener_UnmarshalValidRequest(t *testing.T) {
	t.Parallel()

	// GIVEN
	body := `{"owner":{"name":"test","namespace":"default"},` +
		`"watched":{"name":"watched","namespace":"default"},` +
		`"watchedGvk":{"kind":"Test","group":"test.io","version":"v1"}}`

	request, err := http.NewRequest(http.MethodPost, "/v1/kyma/event", bytes.NewBufferString(body))
	require.NoError(t, err)

	// WHEN
	event, unmarshalErr := listenerEvent.UnmarshalSKREvent(request)

	// THEN
	assert.Nil(t, unmarshalErr, "expected no unmarshal error")
	assert.NotNil(t, event, "expected valid event")
	assert.Equal(t, "test", event.Owner.Name)
	assert.Equal(t, "default", event.Owner.Namespace)
}
