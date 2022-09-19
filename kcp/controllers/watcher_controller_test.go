package controllers_test

import (
	"fmt"
	"math/rand"
	"time"

	kyma "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/custom"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/deploy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	istioResourcesFilePath = "assets/istio-test-resources.yaml"
)

var _ = Describe("Watcher CR scenarios", Ordered, func() {

	var customIstioClient *custom.IstioClient
	var err error
	kymaSample := &kyma.Kyma{}
	var istioResources []*unstructured.Unstructured
	BeforeAll(func() {
		customIstioClient, err = custom.NewIstioClient(cfg)
		Expect(err).ToNot(HaveOccurred())
		kymaName := "kyma-sample"
		kymaSample = createKymaCR(kymaName)
		Expect(k8sClient.Create(ctx, kymaSample)).To(Succeed())
		istioResources, err = deserializeIstioResources(istioResourcesFilePath)
		Expect(err).NotTo(HaveOccurred())
		for _, istioResource := range istioResources {
			Expect(k8sClient.Create(ctx, istioResource)).To(Succeed())
		}

	})

	AfterAll(func() {
		// clean up kyma CR
		Expect(k8sClient.Delete(ctx, kymaSample)).To(Succeed())
		// clean up istio resources
		for _, istioResource := range istioResources {
			Expect(k8sClient.Delete(ctx, istioResource)).To(Succeed())
		}

	})

	DescribeTable("should reconcile istio service mesh resources according to watcher CR config",
		func(watcherCR *watcherv1alpha1.Watcher) {
			// create watcher CR
			Expect(k8sClient.Create(ctx, watcherCR)).To(Succeed())

			time.Sleep(250 * time.Millisecond)
			crObjectKey := client.ObjectKeyFromObject(watcherCR)

			Eventually(watcherCRState(crObjectKey)).
				WithTimeout(20 * time.Second).
				WithPolling(250 * time.Millisecond).
				Should(Equal(watcherv1alpha1.WatcherStateReady))

			// verify istio config
			Expect(customIstioClient.IsListenerHTTPRouteConfigured(ctx, client.ObjectKey{
				Name:      vsName,
				Namespace: vsNamespace,
			}, watcherCR)).To(BeTrue())

			// verify webhook config
			Expect(deploy.IsWebhookDeployed(ctx, cfg)).To(BeTrue())
			Expect(deploy.IsWebhookConfigured(ctx, watcherCR, cfg)).To(BeTrue())

			// update watcher CR spec
			currentWatcherCR := &watcherv1alpha1.Watcher{}
			Expect(k8sClient.Get(ctx, crObjectKey, currentWatcherCR)).To(Succeed())
			currentWatcherCR.Spec.ServiceInfo.Port = 9090
			currentWatcherCR.Spec.Field = watcherv1alpha1.StatusField
			Expect(k8sClient.Update(ctx, currentWatcherCR)).Should(Succeed())

			time.Sleep(250 * time.Millisecond)

			Eventually(watcherCRState(crObjectKey)).
				WithTimeout(20 * time.Second).
				WithPolling(250 * time.Millisecond).
				Should(Equal(watcherv1alpha1.WatcherStateReady))

			Expect(customIstioClient.IsListenerHTTPRouteConfigured(ctx, client.ObjectKey{
				Name:      vsName,
				Namespace: vsNamespace,
			}, currentWatcherCR)).To(BeTrue())

			//verify webhook config
			Expect(deploy.IsWebhookDeployed(ctx, cfg)).To(BeTrue())
			Expect(deploy.IsWebhookConfigured(ctx, currentWatcherCR, cfg)).To(BeTrue())

		}, watcherCREntries)

	It("should delete service mesh routes and SKR config when one CR is deleted", func() {
		idx := rand.Intn(len(watcherCRNames)) //nolint:gosec
		firstToBeRemovedObjKey := client.ObjectKey{
			Name:      fmt.Sprintf("%s-sample", watcherCRNames[idx]),
			Namespace: metav1.NamespaceDefault,
		}
		firstToBeRemoved := &watcherv1alpha1.Watcher{}
		Expect(k8sClient.Get(ctx, firstToBeRemovedObjKey, firstToBeRemoved)).To(Succeed())
		Expect(k8sClient.Delete(ctx, firstToBeRemoved)).To(Succeed())

		time.Sleep(250 * time.Millisecond)

		Eventually(isCrDeletetionFinished(firstToBeRemovedObjKey)).
			WithTimeout(20 * time.Second).
			WithPolling(250 * time.Millisecond).
			Should(BeTrue())

		Expect(customIstioClient.IsListenerHTTPRouteConfigured(ctx, client.ObjectKey{
			Name:      vsName,
			Namespace: vsNamespace,
		}, firstToBeRemoved)).To(BeFalse())

		// verify webhook config
		Expect(deploy.IsWebhookDeployed(ctx, cfg)).To(BeTrue())
		Expect(deploy.IsWebhookConfigured(ctx, firstToBeRemoved, cfg)).To(BeFalse())
	})

	It("should delete all resources on SKR when all CRs are deleted", func() {
		watchers := &watcherv1alpha1.WatcherList{}
		Expect(k8sClient.List(ctx, watchers)).To(Succeed())
		Expect(len(watchers.Items)).To(Equal(len(watcherCRNames) - 1))
		for _, watcher := range watchers.Items {
			//nolint:gosec
			Expect(k8sClient.Delete(ctx, &watcher)).To(Succeed())
		}

		time.Sleep(250 * time.Millisecond)

		Eventually(isCrDeletetionFinished()).
			WithTimeout(20 * time.Second).
			WithPolling(250 * time.Millisecond).
			Should(BeTrue())
		Expect(customIstioClient.IsListenerHTTPRoutesEmpty(ctx, client.ObjectKey{
			Name:      vsName,
			Namespace: vsNamespace,
		})).To(BeTrue())

		Expect(deploy.IsWebhookDeployed(ctx, cfg)).To(BeFalse())
	})
})
