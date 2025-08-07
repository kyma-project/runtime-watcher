package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectKey represents a Kubernetes object key.
type ObjectKey struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// WatchEvent represents an event from Runtime Watcher.
type WatchEvent struct {
	Owner      ObjectKey               `json:"owner"`
	Watched    ObjectKey               `json:"watched"`
	WatchedGvk metav1.GroupVersionKind `json:"watchedGvk"`
}

func (k ObjectKey) String() string {
	if k.Namespace == "" {
		return k.Name
	}

	return k.Namespace + "/" + k.Name
}
