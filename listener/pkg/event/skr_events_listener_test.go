package event_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	listenerEvent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
)

func newSKRListener(addr, component string, verify listenerEvent.VerifyFunc) *listenerEvent.SKREventListener {
	return listenerEvent.NewSKREventListener(addr, component, verify)
}

func TestSKREventListener_ConversionLogic(t *testing.T) {
	t.Parallel()

	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "kyma", Namespace: v1.NamespaceDefault},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: v1.NamespaceDefault},
		WatchedGvk: v1.GroupVersionKind{Kind: "kyma", Group: "operator.kyma-project.io", Version: "v1alpha1"},
	}

	// WHEN
	expectedContent := listenerEvent.UnstructuredContent(testWatcherEvt)
	genericEvent := listenerEvent.GenericEvent(testWatcherEvt)

	// THEN - Verify the conversion
	assert.NotNil(t, genericEvent, "expected generic event to be created")
	assert.Equal(t, testWatcherEvt.Owner.Name, genericEvent.GetName())
	assert.Equal(t, testWatcherEvt.Owner.Namespace, genericEvent.GetNamespace())

	// Verify the unstructured content
	for key, value := range expectedContent {
		assert.Contains(t, genericEvent.Object, key)
		assert.Equal(t, value, genericEvent.Object[key])
	}
}

func TestSKREventListener_Lifecycle(t *testing.T) {
	t.Parallel()
	// SETUP
	skrListener := newSKRListener(":0", "kyma",
		func(_ *http.Request, _ *types.WatchEvent) error {
			return nil
		})

	// Start the SKR listener in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = skrListener.Start(ctx)
	}()

	// Wait for listener to start
	time.Sleep(100 * time.Millisecond)

	assert.NotNil(t, skrListener.ReceivedEvents, "expected ReceivedEvents channel to be available")

	// Test that the listener can be stopped gracefully
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestSKREventListener_UnstructuredContentMapping(t *testing.T) {
	t.Parallel()

	// GIVEN
	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "test-owner", Namespace: "test-namespace"},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: "watched-namespace"},
		WatchedGvk: v1.GroupVersionKind{Kind: "TestKind", Group: "test.io", Version: "v1beta1"},
	}

	// WHEN
	unstructuredContent := listenerEvent.UnstructuredContent(testWatcherEvt)
	genericEvent := listenerEvent.GenericEvent(testWatcherEvt)

	// THEN
	assert.Contains(t, unstructuredContent, "owner")
	assert.Contains(t, unstructuredContent, "watched")
	assert.Contains(t, unstructuredContent, "watched-gvk")
	assert.Equal(t, testWatcherEvt.Owner, unstructuredContent["owner"])
	assert.Equal(t, testWatcherEvt.Watched, unstructuredContent["watched"])
	assert.Equal(t, testWatcherEvt.WatchedGvk, unstructuredContent["watched-gvk"])

	// Verify the generic event object
	assert.NotNil(t, genericEvent)
	assert.Equal(t, testWatcherEvt.Owner.Name, genericEvent.GetName())
	assert.Equal(t, testWatcherEvt.Owner.Namespace, genericEvent.GetNamespace())

	// Verify the unstructured content in the generic event
	for key, value := range unstructuredContent {
		assert.Contains(t, genericEvent.Object, key)
		assert.Equal(t, value, genericEvent.Object[key])
	}
}
