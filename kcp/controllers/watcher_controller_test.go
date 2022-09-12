package controllers_test

import (
	"math/rand"
	"time"

	kymaapi "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherapiv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/controllers"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultBufferSize  = 2048
	servicePathTpl     = "/validate/%s"
	specSubresources   = "*"
	statusSubresources = "*/status"
)

var _ = Describe("Watcher CR scenarios", Ordered, func() {

	kymaSample := &kymaapi.Kyma{}
	kymaSecret := &v1.Secret{}
	BeforeAll(func() {
		kymaName := "kyma-sample"
		kymaSample = createKymaCR(kymaName)
		Expect(k8sClient.Create(ctx, kymaSample)).To(Succeed())
		kymaSecret, err := createInClusterConfigSecret(kymaName)
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Create(ctx, kymaSecret)).To(Succeed())
	})

	AfterAll(func() {
		// clean up kyma CR and secret
		Expect(k8sClient.Delete(ctx, kymaSample)).To(Succeed())
		Expect(k8sClient.Delete(ctx, kymaSecret)).To(Succeed())

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
			returns, err := util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, watcherCR,
				controllers.IstioGatewayResourceName, controllers.IstioGatewayNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(returns).To(BeFalse())

			//verify webhook config
			webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
			// webhookCfgList := &admissionv1.ValidatingWebhookConfigurationList{}
			// Expect(k8sClient.List(ctx, webhookCfgList)).To(Succeed())
			// Expect(webhookCfgList.Items).NotTo(BeNil())
			// Expect(len(webhookCfgList.Items)).To(Equal(1))
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
			Expect(err).ShouldNot(HaveOccurred())
			correct := verifyWebhookConfig(webhookConfig, watcherCR)
			Expect(correct).To(BeTrue())

			// update watcher CR spec
			currentWatcherCR := &watcherapiv1alpha1.Watcher{}
			Expect(k8sClient.Get(ctx, crObjectKey, currentWatcherCR)).To(Succeed())
			currentWatcherCR.Spec.ServiceInfo.ServicePort = 9090
			currentWatcherCR.Spec.Field = watcherapiv1alpha1.StatusField
			Expect(k8sClient.Update(ctx, currentWatcherCR)).Should(Succeed())

			Eventually(watcherCRState(crObjectKey)).
				WithTimeout(20 * time.Second).
				WithPolling(250 * time.Millisecond).
				Should(Equal(watcherapiv1alpha1.WatcherStateReady))

			returns, err = util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, currentWatcherCR,
				controllers.IstioGatewayResourceName, controllers.IstioGatewayNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(returns).To(BeFalse())

			//verify webhook config
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
			Expect(err).ShouldNot(HaveOccurred())
			correct = verifyWebhookConfig(webhookConfig, watcherCR)
			Expect(correct).To(BeTrue())

		}, watcherCREntries)

	It("should delete all resources on SKR when all CRs are deleted", func() {
		idx := rand.Intn(len(watcherCRNames))
		firstToBeRemovedObjKey := client.ObjectKey{Name: watcherCRNames[idx], Namespace: metav1.NamespaceDefault}
		firstToBeRemoved := &watcherapiv1alpha1.Watcher{}
		err := k8sClient.Get(ctx, firstToBeRemovedObjKey, firstToBeRemoved)
		Expect(err).ToNot(HaveOccurred())
		err = k8sClient.Delete(ctx, firstToBeRemoved)
		Expect(err).ToNot(HaveOccurred())
		Eventually(isCRDeletetionFinished(firstToBeRemovedObjKey)).WithTimeout(2 * time.Second).
			WithPolling(20 * time.Microsecond).Should(BeTrue())
		//TODO: verify that istio resources and skr webhooks are deleted
	})

	It("should delete all resources on SKR when all CRs are deleted", func() {
		k8sClient.DeleteAllOf(ctx, &watcherapiv1alpha1.Watcher{})
		Eventually(isCRDeletetionFinished()).WithTimeout(2 * time.Second).
			WithPolling(20 * time.Microsecond).Should(BeTrue())
		//TODO: verify that istio resources and skr webhooks are deleted
	})
})
