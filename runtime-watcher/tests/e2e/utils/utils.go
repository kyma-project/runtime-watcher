package utils

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
)

const (
	skrDomainAnnotationKey   = "skr-domain"
	skrDomainAnnotationValue = "example.domain.com"
	instanceIDLabelValue     = "test-instance"
)

func NewKyma(name, namespace, channel string) *v1beta2.Kyma {
	return &v1beta2.Kyma{
		TypeMeta: apimetav1.TypeMeta{
			APIVersion: v1beta2.GroupVersion.String(),
			Kind:       string(shared.KymaKind),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				skrDomainAnnotationKey: skrDomainAnnotationValue,
			},
			Labels: map[string]string{
				shared.InstanceIDLabel: instanceIDLabelValue,
				shared.SyncLabel:       shared.EnableLabelValue,
			},
		},
		Spec: v1beta2.KymaSpec{
			Channel: channel,
		},
		Status: v1beta2.KymaStatus{},
	}
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

func AddSkipReconciliationLabelToKyma(ctx context.Context, clnt client.Client, kymaName, kymaNamespace string) error {
	kyma := &v1beta2.Kyma{}
	err := clnt.Get(ctx, client.ObjectKey{Namespace: kymaNamespace, Name: kymaName}, kyma)
	if err != nil {
		return fmt.Errorf("failed to get kyma %w", err)
	}

	kyma.Labels[shared.SkipReconcileLabel] = "true"
	if err := clnt.Update(ctx, kyma); err != nil {
		return fmt.Errorf("failed to update kyma, %w", err)
	}

	return nil
}

func RemoveKymaAnnotations(ctx context.Context, clnt client.Client,
	kymaName, kymaNamespace string,
) error {
	kyma := &v1beta2.Kyma{}
	err := clnt.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma)
	if err != nil {
		return fmt.Errorf("failed to get Kyma %w", err)
	}

	kyma.Annotations = nil
	if err := clnt.Update(ctx, kyma); err != nil {
		return fmt.Errorf("failed to update kyma, %w", err)
	}
	return nil
}
