package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WatchEvent struct {
	Owner      client.ObjectKey        `json:"owner"`
	Watched    client.ObjectKey        `json:"watched"`
	WatchedGvk metav1.GroupVersionKind `json:"watchedGvk"`
	SkrMeta    SkrMeta                 `json:"-"`
}
