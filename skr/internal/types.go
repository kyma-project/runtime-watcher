package internal

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO: remove after moving to runtime-watcher/listener.
type WatchEvent struct {
	Owner      client.ObjectKey        `json:"owner"`
	Watched    client.ObjectKey        `json:"watched"`
	WatchedGvk metav1.GroupVersionKind `json:"watchedGvk"`
}
