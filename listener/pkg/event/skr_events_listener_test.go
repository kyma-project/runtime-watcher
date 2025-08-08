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

	// Test that the listener can be stopped
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
	assert.Contains(t, unstructuredContent, "watchedGvk")
	assert.Equal(t, testWatcherEvt.Owner, unstructuredContent["owner"])
	assert.Equal(t, testWatcherEvt.Watched, unstructuredContent["watched"])
	assert.Equal(t, testWatcherEvt.WatchedGvk, unstructuredContent["watchedGvk"])

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

func TestSKREventListener_EndToEndTypeCompatibility(t *testing.T) {
	t.Parallel()

	// GIVEN - Simulate the exact data flow from runtime-watcher to KLM
	testWatcherEvt := &types.WatchEvent{
		Owner:      types.ObjectKey{Name: "test-kyma", Namespace: "kyma-system"},
		Watched:    types.ObjectKey{Name: "watched-resource", Namespace: "test-namespace"},
		WatchedGvk: v1.GroupVersionKind{Kind: "Deployment", Group: "apps", Version: "v1"},
	}

	// WHEN - Runtime watcher converts to unstructured (what gets sent to KLM)
	unstructuredEvent := listenerEvent.GenericEvent(testWatcherEvt)

	// THEN - Verify KLM can extract the data correctly (simulating controller logic)
	assert.NotNil(t, unstructuredEvent)
	assert.Equal(t, testWatcherEvt.Owner.Name, unstructuredEvent.GetName())
	assert.Equal(t, testWatcherEvt.Owner.Namespace, unstructuredEvent.GetNamespace())

	// Verify the unstructured content contains all expected fields for KLM extraction
	expectedFields := []string{"owner", "watched", "watchedGvk"}
	for _, field := range expectedFields {
		assert.Contains(t, unstructuredEvent.Object, field,
			"KLM expects field %s to be present for extraction", field)
	}

	// Verify owner can be extracted (what KLM controllers do)
	ownerData, found := unstructuredEvent.Object["owner"]
	assert.True(t, found, "owner field must be present for KLM extraction")

	// The owner field is a types.ObjectKey struct, not a map
	owner, ok := ownerData.(types.ObjectKey)
	assert.True(t, ok, "owner must be types.ObjectKey for KLM extraction")

	assert.Equal(t, testWatcherEvt.Owner.Name, owner.Name)
	assert.Equal(t, testWatcherEvt.Owner.Namespace, owner.Namespace)

	// Also verify that the watched field is correctly structured
	watchedData, found := unstructuredEvent.Object["watched"]
	assert.True(t, found, "watched field must be present")

	watched, ok := watchedData.(types.ObjectKey)
	assert.True(t, ok, "watched must be types.ObjectKey")

	assert.Equal(t, testWatcherEvt.Watched.Name, watched.Name)
	assert.Equal(t, testWatcherEvt.Watched.Namespace, watched.Namespace)

	// Verify watchedGvk field
	watchedGvkData, found := unstructuredEvent.Object["watchedGvk"]
	assert.True(t, found, "watchedGvk field must be present")

	watchedGvk, ok := watchedGvkData.(v1.GroupVersionKind)
	assert.True(t, ok, "watchedGvk must be GroupVersionKind")

	assert.Equal(t, testWatcherEvt.WatchedGvk.Kind, watchedGvk.Kind)
	assert.Equal(t, testWatcherEvt.WatchedGvk.Group, watchedGvk.Group)
	assert.Equal(t, testWatcherEvt.WatchedGvk.Version, watchedGvk.Version)
}
