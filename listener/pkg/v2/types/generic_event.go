package types

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GenericEvent contains information for a Generic event.
type GenericEvent struct {
	// Object is the object from the event
	Object *unstructured.Unstructured
}
