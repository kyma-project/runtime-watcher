package controllers_test

import (
	"fmt"
	"math/rand"
	"time"

	kymaapi "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherapiv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
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
	kymaSample := &kymaapi.Kyma{}
	kymaSecret := &v1.Secret{}
	var istioResources []*unstructured.Unstructured
	BeforeAll(func() {
		istioClientSet, err = istioclient.NewForConfig(cfg)
		Expect(err).ToNot(HaveOccurred())
		kymaName := "kyma-sample"
		kymaSample = createKymaCR(kymaName)
		Expect(k8sClient.Create(ctx, kymaSample)).To(Succeed())
		kymaSecret, err := createInClusterConfigSecret(kymaName)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Create(ctx, kymaSecret)).To(Succeed())
		istioResources, err = deserializeIstioResources(istioResourcesFilePath)
		Expect(err).NotTo(HaveOccurred())
		for _, istioResource := range istioResources {
			Expect(k8sClient.Create(ctx, istioResource)).To(Succeed())
		}

	})

	AfterAll(func() {
		// clean up kyma CR and secret
		Expect(k8sClient.Delete(ctx, kymaSample)).To(Succeed())
		Expect(k8sClient.Delete(ctx, kymaSecret)).To(Succeed())
		// clean up istio resources
		for _, istioResource := range istioResources {
			Expect(k8sClient.Delete(ctx, istioResource)).To(Succeed())
		}

	})

	DescribeTable("should reconcile istio service mesh resources according to watcher CR config",
		func(watcherCR *watcherapiv1alpha1.Watcher) {
			// create watcher CR
			Expect(k8sClient.Create(ctx, watcherCR)).Should(Succeed())

			time.Sleep(5 * time.Second)
			crObjectKey := client.ObjectKeyFromObject(watcherCR)

			// Expect(currentWatcherCR.Status.State).Should(Equal(watcherapiv1alpha1.WatcherStateReady))
			Eventually(watcherCRState(crObjectKey)).
				WithTimeout(20 * time.Second).
				WithPolling(250 * time.Millisecond).
				Should(Equal(watcherapiv1alpha1.WatcherStateReady))

			// verify istio config

			Expect(util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, watcherCR,
				util.DefaultVirtualServiceName, util.DefaultVirtualServiceNamespace)).To(Succeed())

			// Eventually(isWebhookDeployed("skr-webhook")).
			// 	WithTimeout(20 * time.Second).
			// 	WithPolling(10 * time.Millisecond).
			// 	Should(BeTrue())

			//verify webhook config
			webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)).To(Succeed())
			// Expect(k8sClient.Get(ctx, crObjectKey, watcherCR)).To(Succeed())
			webhookIdx := lookupWebhook(webhookConfig, watcherCR)
			Expect(webhookIdx).NotTo(Equal(-1))
			Expect(verifyWebhookConfig(webhookConfig.Webhooks[webhookIdx], watcherCR)).To(BeTrue())

			// update watcher CR spec
			currentWatcherCR := &watcherapiv1alpha1.Watcher{}
			Expect(k8sClient.Get(ctx, crObjectKey, currentWatcherCR)).To(Succeed())
			currentWatcherCR.Spec.ServiceInfo.Port = 9090
			currentWatcherCR.Spec.Field = watcherapiv1alpha1.StatusField
			Expect(k8sClient.Update(ctx, currentWatcherCR)).Should(Succeed())

			time.Sleep(5 * time.Second)

			Eventually(watcherCRState(crObjectKey)).
				WithTimeout(20 * time.Second).
				WithPolling(250 * time.Millisecond).
				Should(Equal(watcherapiv1alpha1.WatcherStateReady))

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
		firstToBeRemoved := &watcherapiv1alpha1.Watcher{}
		Expect(k8sClient.Get(ctx, firstToBeRemovedObjKey, firstToBeRemoved)).To(Succeed())
		err = k8sClient.Delete(ctx, firstToBeRemoved)
		Expect(k8sClient.Delete(ctx, firstToBeRemoved)).To(Succeed())

		time.Sleep(5 * time.Second)

		Eventually(isCRDeletetionFinished(firstToBeRemovedObjKey)).
			WithTimeout(20 * time.Second).
			WithPolling(250 * time.Millisecond).
			Should(BeTrue())
		//TODO: verify that istio virtual service http route (which contains specs corresponding to the deleted CR) are deleted
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
		Skip("for now")
		k8sClient.DeleteAllOf(ctx, &watcherapiv1alpha1.Watcher{})
		time.Sleep(5 * time.Second)
		Eventually(isCRDeletetionFinished()).
			WithTimeout(30 * time.Second).
			WithPolling(30 * time.Millisecond).
			Should(BeTrue())
		//TODO: verify that all istio virtual service http routes are deleted
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(k8sapierrors.IsNotFound(err)).To(BeTrue())
	})
})
