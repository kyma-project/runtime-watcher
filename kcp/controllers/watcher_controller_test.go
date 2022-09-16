package controllers_test

import (
	"fmt"
	"math/rand"
	"time"

	kyma "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	servicePathTpl         = "/validate/%s"
	specSubresources       = "*"
	statusSubresources     = "*/status"
	istioResourcesFilePath = "assets/istio-test-resources.yaml"
)

var _ = Describe("Watcher CR scenarios", Ordered, func() {

	var istioClientSet *istioclient.Clientset
	var err error
	kymaSample := &kyma.Kyma{}
	var istioResources []*unstructured.Unstructured
	BeforeAll(func() {
		istioClientSet, err = istioclient.NewForConfig(cfg)
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
			Expect(util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, watcherCR,
				util.DefaultVirtualServiceName, util.DefaultVirtualServiceNamespace)).To(Succeed())

			//verify webhook config
			webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)).To(Succeed())
			webhookIdx := lookupWebhook(webhookConfig, watcherCR)
			Expect(webhookIdx).NotTo(Equal(-1))
			Expect(verifyWebhookConfig(webhookConfig.Webhooks[webhookIdx], watcherCR)).To(BeTrue())

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

			Expect(util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, currentWatcherCR,
				util.DefaultVirtualServiceName, util.DefaultVirtualServiceNamespace)).To(Succeed())

			//verify webhook config
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)).To(Succeed())
			webhookIdx = lookupWebhook(webhookConfig, watcherCR)
			Expect(webhookIdx).NotTo(Equal(-1))
			Expect(verifyWebhookConfig(webhookConfig.Webhooks[webhookIdx], currentWatcherCR)).To(BeTrue())

		}, watcherCREntries)

	It("should delete service mesh routes and SKR config when one CR is deleted", func() {
		idx := rand.Intn(len(watcherCRNames))
		firstToBeRemovedObjKey := client.ObjectKey{Name: fmt.Sprintf("%s-sample", watcherCRNames[idx]), Namespace: metav1.NamespaceDefault}
		firstToBeRemoved := &watcherv1alpha1.Watcher{}
		Expect(k8sClient.Get(ctx, firstToBeRemovedObjKey, firstToBeRemoved)).To(Succeed())
		Expect(k8sClient.Delete(ctx, firstToBeRemoved)).To(Succeed())

		time.Sleep(250 * time.Millisecond)

		Eventually(isCrDeletetionFinished(firstToBeRemovedObjKey)).
			WithTimeout(20 * time.Second).
			WithPolling(250 * time.Millisecond).
			Should(BeTrue())

		Expect(util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, firstToBeRemoved,
			util.DefaultVirtualServiceName, util.DefaultVirtualServiceNamespace)).ToNot(BeNil())
		//verify webhook config
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(err).ShouldNot(HaveOccurred())
		webhookIdx := lookupWebhook(webhookConfig, firstToBeRemoved)
		Expect(webhookIdx).To(Equal(-1))
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
		//TODO: verify that all istio virtual service http routes are deleted
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(kerrors.IsNotFound(err)).To(BeTrue())
	})
})
