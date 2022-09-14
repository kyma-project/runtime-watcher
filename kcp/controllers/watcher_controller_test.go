package controllers_test

import (
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

	kymaSample := &kymaapi.Kyma{}
	kymaSecret := &v1.Secret{}
	var istioResources []*unstructured.Unstructured
	BeforeAll(func() {
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

			crObjectKey := client.ObjectKeyFromObject(watcherCR)
			Eventually(watcherCRState(crObjectKey)).
				WithTimeout(20 * time.Second).
				WithPolling(250 * time.Millisecond).
				Should(Equal(watcherapiv1alpha1.WatcherStateReady))

			// verify istio config
			istioClientSet, err := istioclient.NewForConfig(cfg)
			Expect(err).ToNot(HaveOccurred())
			err = util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, watcherCR,
				util.DefaultVirtualServiceName, util.DefaultVirtualServiceNamespace)
			Expect(err).ToNot(HaveOccurred())

			Eventually(isWebhookDeployed("skr-webhook")).
				WithTimeout(20 * time.Second).
				WithPolling(10 * time.Millisecond).
				Should(BeTrue())

			//verify webhook config
			webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
			Expect(err).ShouldNot(HaveOccurred())
			correct := verifyWebhookConfig(webhookConfig, watcherCR)
			Expect(correct).To(BeTrue())

			// update watcher CR spec
			currentWatcherCR := &watcherapiv1alpha1.Watcher{}
			Expect(k8sClient.Get(ctx, crObjectKey, currentWatcherCR)).To(Succeed())
			currentWatcherCR.Spec.ServiceInfo.Port = 9090
			currentWatcherCR.Spec.Field = watcherapiv1alpha1.StatusField
			Expect(k8sClient.Update(ctx, currentWatcherCR)).Should(Succeed())

			Eventually(watcherCRState(crObjectKey)).
				WithTimeout(20 * time.Second).
				WithPolling(250 * time.Millisecond).
				Should(Equal(watcherapiv1alpha1.WatcherStateReady))

			err = util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, currentWatcherCR,
				util.DefaultVirtualServiceName, util.DefaultVirtualServiceNamespace)
			Expect(err).ToNot(HaveOccurred())

			//verify webhook config
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
			Expect(err).ShouldNot(HaveOccurred())
			correct = verifyWebhookConfig(webhookConfig, watcherCR)
			Expect(correct).To(BeTrue())

		}, watcherCREntries)

	It("should delete service mesh routes and SKR config when one CR is deleted", func() {
		idx := rand.Intn(len(watcherCRNames))
		firstToBeRemovedObjKey := client.ObjectKey{Name: watcherCRNames[idx], Namespace: metav1.NamespaceDefault}
		firstToBeRemoved := &watcherapiv1alpha1.Watcher{}
		err := k8sClient.Get(ctx, firstToBeRemovedObjKey, firstToBeRemoved)
		Expect(err).ToNot(HaveOccurred())
		err = k8sClient.Delete(ctx, firstToBeRemoved)
		Expect(err).ToNot(HaveOccurred())
		Eventually(isCRDeletetionFinished(firstToBeRemovedObjKey)).
			WithTimeout(20 * time.Second).
			WithPolling(250 * time.Millisecond).
			Should(BeTrue())
		//TODO: verify that istio virtual service http route (which contains specs corresponding to the deleted CR) are deleted
		//verify webhook config
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(err).ShouldNot(HaveOccurred())
		correct := verifyWebhookConfig(webhookConfig, firstToBeRemoved)
		Expect(correct).To(BeFalse())
	})

	It("should delete all resources on SKR when all CRs are deleted", func() {
		k8sClient.DeleteAllOf(ctx, &watcherapiv1alpha1.Watcher{})
		Eventually(isCRDeletetionFinished()).
			WithTimeout(20 * time.Second).
			WithPolling(250 * time.Millisecond).
			Should(BeTrue())
		//TODO: verify that all istio virtual service http routes are deleted
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(k8sapierrors.IsNotFound(err)).To(BeTrue())
	})
})
