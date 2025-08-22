package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ObjectKey identifies a Kubernetes Object.
type ObjectKey = types.NamespacedName

type WatchEvent struct {
	Owner      ObjectKey               `json:"owner"`
	Watched    ObjectKey               `json:"watched"`
	WatchedGvk metav1.GroupVersionKind `json:"watchedGvk"`
}
