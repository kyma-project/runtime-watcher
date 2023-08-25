package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type WatchEvent struct {
	Owner      types.NamespacedName    `json:"owner"`
	Watched    types.NamespacedName    `json:"watched"`
	WatchedGvk metav1.GroupVersionKind `json:"watchedGvk"`
}
