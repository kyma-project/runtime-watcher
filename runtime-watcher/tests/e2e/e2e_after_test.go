package e2e_test

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/runtime-watcher/skr/tests/e2e/utils"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func cleanupKymaAfterAll(kyma *v1beta2.Kyma) {
	kymaName := types.NamespacedName{
		Namespace: kyma.Namespace,
		Name:      kyma.Name,
	}
	AfterAll(func() {
		By("When Purge Finalizer is deleted", func() {
			By("And Kyma is deleted", func() {
				Eventually(removePurgeFinalizerAndDeleteKyma).
					WithContext(ctx).
					WithArguments(controlPlaneClient, kyma).
					Should(Succeed())
			})
		})
		By("Then SKR Kyma is deleted", func() {
			Eventually(confirmNoKymaInstance).
				WithContext(ctx).
				WithArguments(runtimeClient, kymaName).
				Should(Succeed())

			By("And KCP Kyma is deleted", func() {
				Eventually(confirmNoKymaInstance).
					WithContext(ctx).
					WithArguments(controlPlaneClient, kymaName).
					Should(Succeed())
			})
		})
	})
}

func removePurgeFinalizerAndDeleteKyma(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma) error {
	if err := syncKyma(ctx, clnt, kyma); err != nil {
		return fmt.Errorf("sync kyma %w", err)
	}
	if !kyma.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(kyma, shared.PurgeFinalizer) {
			controllerutil.RemoveFinalizer(kyma, shared.PurgeFinalizer)
			if err := clnt.Update(ctx, kyma); err != nil {
				return fmt.Errorf("can't remove purge finalizer %w", err)
			}
		}
	}
	return deleteKyma(ctx, clnt, kyma, apimetav1.DeletePropagationBackground)
}

func syncKyma(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma) error {
	err := clnt.Get(ctx, client.ObjectKey{
		Name:      kyma.Name,
		Namespace: kyma.Namespace,
	}, kyma)
	err = client.IgnoreNotFound(err)
	if err != nil {
		return fmt.Errorf("failed to fetch Kyma CR: %w", err)
	}
	return nil
}

func deleteKyma(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma, delProp apimetav1.DeletionPropagation) error {
	err := clnt.Delete(ctx, kyma, &client.DeleteOptions{PropagationPolicy: &delProp})
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("updating kyma failed %w", err)
	}
	return nil
}

func confirmNoKymaInstance(ctx context.Context, clnt client.Client, name types.NamespacedName) error {
	kyma := &v1beta2.Kyma{}
	err := clnt.Get(ctx, name, kyma)
	if utils.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("kyma not deleted: %w", err)
	}
	return nil
}
