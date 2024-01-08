package utils

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"strings"

	"errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
)

func NewKymaWithSyncLabel(name, namespace, channel, syncStrategy string) *v1beta2.Kyma {
	return NewKymaBuilder().
		WithNamePrefix(name).
		WithNamespace(namespace).
		WithAnnotation("skr-domain", "example.domain.com").
		WithAnnotation(shared.SyncStrategyAnnotation, syncStrategy).
		WithLabel(shared.InstanceIDLabel, "test-instance").
		WithLabel(shared.SyncLabel, shared.EnableLabelValue).
		WithChannel(channel).
		Build()
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if machineryruntime.IsNotRegisteredError(err) ||
		meta.IsNoMatchError(err) ||
		apierrors.IsNotFound(err) {
		return true
	}
	groupErr := &discovery.ErrGroupDiscoveryFailed{}
	if errors.As(err, &groupErr) {
		for _, err := range groupErr.Groups {
			if apierrors.IsNotFound(err) {
				return true
			}
		}
	}
	for _, msg := range []string{
		"failed to get restmapping",
		"could not find the requested resource",
	} {
		if strings.Contains(err.Error(), msg) {
			return true
		}
	}
	return false
}
