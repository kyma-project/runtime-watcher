package event_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	listenerEvent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
)

const hostname = "http://localhost:8082"

type unmarshalTestCase struct {
	name               string
	urlPath            string
	expectedEvent      *types.WatchEvent
	expectedErrMsg     string
	expectedHTTPStatus int
}

func newWatcherEventRequest(t *testing.T, method, url string, watcherEvent *types.WatchEvent) *http.Request {
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

func TestUnmarshalSKREvent(t *testing.T) {
	t.Parallel()

	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "kyma", Namespace: v1.NamespaceDefault},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: v1.NamespaceDefault},
		WatchedGvk: v1.GroupVersionKind{Kind: "kyma", Group: "operator.kyma-project.io", Version: "v1alpha1"},
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
			req := newWatcherEventRequest(t, http.MethodPost, url, testWatcherEvt)
			// WHEN
			currentWatcherEvent, err := listenerEvent.UnmarshalSKREvent(req)
			// THEN
			if err != nil {
				require.Equal(t, testCase.expectedErrMsg, err.Message)
				require.Equal(t, testCase.expectedHTTPStatus, err.HTTPErrorCode)

				return
			}

			require.Equal(t, testCase.expectedErrMsg, "")
			require.Equal(t, testCase.expectedHTTPStatus, http.StatusOK)
			require.Equal(t, testCase.expectedEvent, currentWatcherEvent)
		})
	}
}

func TestUnmarshalSKREvent_InvalidJsonBody_EmptyBody_MalformedPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedError  string
		expectedStatus int
	}{
		{
			name: "invalid JSON body",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest(http.MethodPost, "/v1/kyma/event", strings.NewReader("{invalid json"))
				return req
			},
			expectedError:  "could not unmarshal watcher event",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "empty body",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest(http.MethodPost, "/v1/kyma/event", strings.NewReader(""))
				return req
			},
			expectedError:  "could not unmarshal watcher event",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "malformed path - not enough segments",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"owner":{"name":"test"}}`))
				return req
			},
			expectedError:  "could not read contract version",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// GIVEN
			req := testCase.setupRequest()

			// WHEN
			event, unmarshalErr := listenerEvent.UnmarshalSKREvent(req)

			// THEN
			assert.NotNil(t, unmarshalErr, "expected unmarshal error")
			assert.Contains(t, unmarshalErr.Message, testCase.expectedError)
			assert.Equal(t, testCase.expectedStatus, unmarshalErr.HTTPErrorCode)
			assert.Nil(t, event, "expected nil event on error")
		})
	}
}

func TestGenericEvent(t *testing.T) {
	t.Parallel()

	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "test-owner", Namespace: "test-namespace"},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: "watched-namespace"},
		WatchedGvk: v1.GroupVersionKind{Kind: "TestKind", Group: "test.io", Version: "v1beta1"},
	}

	// WHEN
	genericEvent := listenerEvent.GenericEvent(testWatcherEvt)

	// THEN
	assert.NotNil(t, genericEvent, "generic event should not be nil")
	assert.Equal(t, testWatcherEvt.Owner.Name, genericEvent.GetName())
	assert.Equal(t, testWatcherEvt.Owner.Namespace, genericEvent.GetNamespace())

	// Verify the unstructured content is properly set
	expectedContent := listenerEvent.UnstructuredContent(testWatcherEvt)
	for key, expectedValue := range expectedContent {
		actualValue, found := genericEvent.Object[key]
		assert.True(t, found, "key %s should be present in generic event", key)
		assert.Equal(t, expectedValue, actualValue, "value for key %s should match", key)
	}
}

func TestGenericEvent_EmptyValues(t *testing.T) {
	t.Parallel()

	// Test with empty/minimal values
	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "", Namespace: ""},
		Watched:    types.ObjectKey{Name: "", Namespace: ""},
		WatchedGvk: v1.GroupVersionKind{},
	}

	// WHEN
	genericEvent := listenerEvent.GenericEvent(testWatcherEvt)

	// THEN
	assert.NotNil(t, genericEvent, "generic event should not be nil even with empty values")
	assert.Empty(t, genericEvent.GetName())
	assert.Empty(t, genericEvent.GetNamespace())

	// Verify content is still properly mapped
	expectedContent := listenerEvent.UnstructuredContent(testWatcherEvt)
	assert.Len(t, expectedContent, 3, "should have exactly 3 fields")
	assert.Contains(t, expectedContent, "owner")
	assert.Contains(t, expectedContent, "watched")
	assert.Contains(t, expectedContent, "watchedGvk")
}

func TestUnstructuredContent(t *testing.T) {
	t.Parallel()

	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner: types.ObjectKey{
			Name:      "test-owner",
			Namespace: "test-namespace",
		},
		Watched: types.ObjectKey{
			Name:      "watched-resource",
			Namespace: "watched-namespace",
		},
		WatchedGvk: v1.GroupVersionKind{
			Kind:    "TestKind",
			Group:   "test.io",
			Version: "v1beta1",
		},
	}

	// WHEN
	content := listenerEvent.UnstructuredContent(testWatcherEvt)

	// THEN
	assert.NotNil(t, content, "content should not be nil")
	assert.Len(t, content, 3, "content should have exactly 3 keys")

	// Verify owner mapping
	assert.Contains(t, content, "owner")
	assert.Equal(t, testWatcherEvt.Owner, content["owner"])

	// Verify watched mapping
	assert.Contains(t, content, "watched")
	assert.Equal(t, testWatcherEvt.Watched, content["watched"])

	// Verify watched-gvk mapping
	assert.Contains(t, content, "watchedGvk")
	assert.Equal(t, testWatcherEvt.WatchedGvk, content["watchedGvk"])
}

func TestUnstructuredContent_ConsistentKeys(t *testing.T) {
	t.Parallel()

	// Test that the content mapping uses consistent key names
	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "test", Namespace: "default"},
		Watched:    types.ObjectKey{Name: "resource", Namespace: "default"},
		WatchedGvk: v1.GroupVersionKind{Kind: "Kind", Group: "group", Version: "v1"},
	}

	// WHEN
	content := listenerEvent.UnstructuredContent(testWatcherEvt)

	// THEN
	expectedKeys := []string{"owner", "watched", "watchedGvk"}

	actualKeys := make([]string, 0, len(content))
	for key := range content {
		actualKeys = append(actualKeys, key)
	}

	assert.ElementsMatch(t, expectedKeys, actualKeys, "content should have exactly the expected keys")
}

func TestUnstructuredContent_TypePreservation(t *testing.T) {
	t.Parallel()

	// Test that the content preserves the correct types
	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner: types.ObjectKey{
			Name:      "test-owner",
			Namespace: "test-namespace",
		},
		Watched: types.ObjectKey{
			Name:      "watched-resource",
			Namespace: "watched-namespace",
		},
		WatchedGvk: v1.GroupVersionKind{
			Kind:    "TestKind",
			Group:   "test.io",
			Version: "v1beta1",
		},
	}

	// WHEN
	content := listenerEvent.UnstructuredContent(testWatcherEvt)

	// THEN
	// Verify types are preserved
	owner, ownerOK := content["owner"].(types.ObjectKey)
	assert.True(t, ownerOK, "owner should be of type ObjectKey")
	assert.Equal(t, testWatcherEvt.Owner, owner)

	watched, watchedOK := content["watched"].(types.ObjectKey)
	assert.True(t, watchedOK, "watched should be of type ObjectKey")
	assert.Equal(t, testWatcherEvt.Watched, watched)

	watchedGvk, gvkOK := content["watchedGvk"].(v1.GroupVersionKind)
	assert.True(t, gvkOK, "watchedGvk should be of type GroupVersionKind")
	assert.Equal(t, testWatcherEvt.WatchedGvk, watchedGvk)
}
