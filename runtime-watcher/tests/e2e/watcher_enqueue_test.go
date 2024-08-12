package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"

	apiappsv1 "k8s.io/api/apps/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/runtime-watcher/skr/tests/e2e/utils"
)

const (
	controlPlaneNamespace = "kcp-system"
	watcherCrName         = "klm-watcher"
	remoteNamespace       = "kyma-system"
	localHostname         = "0.0.0.0"
	k3dHostname           = "host.k3d.internal"
	kymaName              = "kyma-sample"
	kymaChannel           = "regular"
	watcherSecretName     = "skr-webhook-tls" //nolint:gosec
	watcherName           = "skr-webhook"
)

type ResourceName = types.NamespacedName

var errWatcherDeploymentNotReady = errors.New("watcher Deployment is not ready")

var _ = Describe("Enqueue Event from Watcher", Ordered, func() {
	kyma := NewKyma(kymaName, controlPlaneNamespace, kymaChannel)
	GinkgoWriter.Printf("kyma before create %v\n", kyma)
	incomingRequestMsg := fmt.Sprintf("event received from SKR, adding %s/%s to queue",
		kyma.GetNamespace(), kyma.GetName())

	initEmptyKymaBeforeAll(kyma)
	cleanupKymaAfterAll(kyma)

	skrSecret := ResourceName{
		Name:      watcherSecretName,
		Namespace: remoteNamespace,
	}
	watcher := ResourceName{
		Namespace: remoteNamespace,
		Name:      watcherName,
	}

	Context("Given SKR Cluster with TLS Secret", func() {
		It("When Runtime Watcher deployment is ready", func() {
			Eventually(deploymentReady).
				WithContext(ctx).
				WithArguments(runtimeClient, watcher).
				Should(Succeed())

			By("And Runtime Watcher deployment is deleted")
			Eventually(deleteDeployment).
				WithContext(ctx).
				WithArguments(runtimeClient, watcher).
				Should(Succeed())
		})

		It("Then Runtime Watcher deployment is ready again", func() {
			Eventually(deploymentReady).
				WithContext(ctx).
				WithArguments(runtimeClient, watcher).
				Should(Succeed())
		})

		It("When TLS Secret is deleted on SKR Cluster", func() {
			Eventually(secretExists).
				WithContext(ctx).
				WithArguments(runtimeClient, skrSecret).
				Should(Succeed())
			Eventually(deleteSecret).
				WithContext(ctx).
				WithArguments(runtimeClient, skrSecret).
				Should(Succeed())
		})

		It("Then TLS Secret is recreated", func() {
			Eventually(secretExists).
				WithContext(ctx).
				WithArguments(runtimeClient, skrSecret).
				Should(Succeed())
		})

		timeNow := &apimetav1.Time{Time: time.Now()}
		GinkgoWriter.Println(fmt.Sprintf("Spec watching logs since %s: ", timeNow))
		It("When spec of SKR Kyma CR is changed", func() {
			Eventually(changeRemoteKymaChannel).
				WithContext(ctx).
				WithArguments(runtimeClient, "fast").
				Should(Succeed())
		})
		It("Then new reconciliation gets triggered for KCP Kyma CR", func() {
			logAssert := NewLogAsserter(controlPlaneRESTConfig, runtimeRESTConfig, controlPlaneClient,
				runtimeClient)
			Eventually(logAssert.ContainsKLMLogMessage).
				WithContext(ctx).
				WithArguments(incomingRequestMsg, timeNow).
				Should(Succeed())
			Eventually(logAssert.ContainsWatcherLogs).
				WithContext(ctx).
				WithArguments(timeNow).
				Should(Succeed())
		})

		time.Sleep(1 * time.Second)
		patchingTimestamp := &apimetav1.Time{Time: time.Now()}
		GinkgoWriter.Println(fmt.Sprintf("Status subresource watching logs since %s: ", patchingTimestamp))
		It("When Runtime Watcher spec field is changed to status", func() {
			Expect(updateWatcherSpecField(ctx, controlPlaneClient)).
				Should(Succeed())

			By("And SKR Kyma CR Status is updated")
			Eventually(updateRemoteKymaStatus).
				WithContext(ctx).
				WithArguments(runtimeClient).
				Should(Succeed())
		})

		It("Then new reconciliation gets triggered for KCP Kyma CR", func() {
			logAssert := NewLogAsserter(controlPlaneRESTConfig, runtimeRESTConfig, controlPlaneClient,
				runtimeClient)
			Eventually(logAssert.ContainsKLMLogMessage).
				WithContext(ctx).
				WithArguments(incomingRequestMsg, patchingTimestamp).
				Should(Succeed())
			Eventually(logAssert.ContainsWatcherLogs).
				WithContext(ctx).
				WithArguments(patchingTimestamp).
				Should(Succeed())
		})
	})
})

func deleteDeployment(ctx context.Context, k8sClient client.Client, name ResourceName) error {
	deployment := &apiappsv1.Deployment{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}
	return k8sClient.Delete(ctx, deployment)
}

func deploymentReady(ctx context.Context, clnt client.Client, name ResourceName) error {
	deployment := &apiappsv1.Deployment{}
	if err := clnt.Get(ctx, name, deployment); err != nil {
		return err
	}
	if deployment.Status.ReadyReplicas != 1 {
		return fmt.Errorf("%w: %s/%s", errWatcherDeploymentNotReady, name.Namespace, name.Name)
	}
	return nil
}

func secretExists(ctx context.Context, clnt client.Client, name ResourceName) error {
	secret := &apicorev1.Secret{}
	err := clnt.Get(ctx, name, secret)
	if err != nil {
		return fmt.Errorf("failed to get secret %w", err)
	}

	return nil
}

func deleteSecret(ctx context.Context, clnt client.Client, name ResourceName) error {
	certificateSecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}
	err := clnt.Delete(ctx, certificateSecret)
	if err != nil {
		return fmt.Errorf("failed to delete secret %w", err)
	}
	return nil
}

func changeRemoteKymaChannel(ctx context.Context, clnt client.Client, channel string) error {
	kyma := &v1beta2.Kyma{}
	if err := clnt.Get(ctx,
		client.ObjectKey{Name: defaultRemoteKymaName, Namespace: remoteNamespace},
		kyma); err != nil {
		return err
	}

	kyma.Spec.Channel = channel

	return clnt.Update(ctx, kyma)
}

func updateRemoteKymaStatus(clnt client.Client) error {
	kyma := &v1beta2.Kyma{}
	err := clnt.Get(ctx, client.ObjectKey{Name: defaultRemoteKymaName, Namespace: remoteNamespace}, kyma)
	if err != nil {
		return fmt.Errorf("failed to get Kyma %w", err)
	}

	kyma.Status.State = shared.StateWarning
	kyma.Status.LastOperation = shared.LastOperation{
		Operation:      "Updated Kyma Status subresource for test",
		LastUpdateTime: apimetav1.NewTime(time.Now()),
	}
	kyma.ManagedFields = nil
	if err := clnt.Status().Update(ctx, kyma); err != nil {
		return fmt.Errorf("kyma status subresource could not be updated: %w", err)
	}

	return nil
}

func updateWatcherSpecField(ctx context.Context, k8sClient client.Client) error {
	watcherCR := &v1beta2.Watcher{}
	err := k8sClient.Get(ctx,
		client.ObjectKey{Name: watcherCrName, Namespace: controlPlaneNamespace},
		watcherCR)
	if err != nil {
		return fmt.Errorf("failed to get Kyma %w", err)
	}
	watcherCR.Spec.Field = v1beta2.StatusField
	if err = k8sClient.Update(ctx, watcherCR); err != nil {
		return fmt.Errorf("failed to update watcher spec.field: %w", err)
	}
	return nil
}
