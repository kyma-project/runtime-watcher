package controllers_test

import (
	"errors"
	"io"
	"os"
	"time"

	kymav1alpha1 "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherapiv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/controllers"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultBufferSize = 2048
	gatewayPortNumber = uint32(80)
)

//nolint:gochecknoglobals
var watcherCREntries = []TableEntry{
	Entry("lifecycle manager Watcher CR", &watcherapiv1alpha1.Watcher{
		TypeMeta: metav1.TypeMeta{
			Kind:       watcherapiv1alpha1.WatcherKind,
			APIVersion: watcherapiv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{Name: "watcher-sample", Namespace: metav1.NamespaceDefault},
		Spec: watcherapiv1alpha1.WatcherSpec{
			ContractVersion: "v1",
			ComponentName:   "lifecycle-manager",
			ServiceInfo: watcherapiv1alpha1.ServiceInfo{
				ServicePort:      8082,
				ServiceName:      "lifecycle-manager-svc",
				ServiceNamespace: metav1.NamespaceDefault,
			},
			GvrsToWatch: []watcherapiv1alpha1.WatchableGvr{
				{
					Gvr: watcherapiv1alpha1.Gvr{
						Group:    kymav1alpha1.GroupVersion.Group,
						Version:  kymav1alpha1.GroupVersion.Version,
						Resource: "kymas",
					},
				},
			},
		},
	}),
}

var _ = Context("Watcher CR scenarios", Ordered, func() {
	cmObjectKey := client.ObjectKey{
		Name:      util.ConfigMapResourceName,
		Namespace: util.ConfigMapNamespace,
	}
	istioCrdList := &apiextensionsv1.CustomResourceDefinitionList{}
	BeforeAll(func() {
		istioCrds, err := os.Open("assets/istio.networking.crds.yaml")
		Expect(err).NotTo(HaveOccurred())
		defer istioCrds.Close()
		decoder := yaml.NewYAMLOrJSONDecoder(istioCrds, 2048)
		for {
			crd := apiextensionsv1.CustomResourceDefinition{}
			err = decoder.Decode(&crd)
			if err == nil {
				istioCrdList.Items = append(istioCrdList.Items, crd)
				// create istio CRD
				Expect(k8sClient.Create(ctx, &crd)).To(Succeed())
			}
			if errors.Is(err, io.EOF) {
				break
			}
		}
	})

	AfterAll(func() {
		// clean up config map
		cm := &v1.ConfigMap{}
		Expect(k8sClient.Get(ctx, cmObjectKey, cm)).To(Succeed())
		Expect(k8sClient.Delete(ctx, cm)).To(Succeed())
		//nolint:gosec
		for _, crd := range istioCrdList.Items {
			Expect(k8sClient.Delete(ctx, &crd)).To(Succeed())
		}
	})

	DescribeTable("should reconcile istio service mesh resources according to watcher CR config",
		func(watcherCR *watcherapiv1alpha1.Watcher) {
			// create watcher CR
			Expect(k8sClient.Create(ctx, watcherCR)).Should(Succeed())

			watcherObjKey := client.ObjectKeyFromObject(watcherCR)
			Eventually(watcherCRState(watcherObjKey)).WithTimeout(3 * time.Second).
				WithPolling(30 * time.Microsecond).
				Should(Equal(watcherapiv1alpha1.WatcherStateReady))

			Expect(k8sClient.Get(ctx, cmObjectKey, &v1.ConfigMap{})).To(Succeed())
			// verify istio config
			istioClientSet, err := istioclient.NewForConfig(cfg)
			Expect(err).ToNot(HaveOccurred())
			returns, err := util.PerformIstioGWCheck(ctx, istioClientSet, gatewayPortNumber,
				controllers.IstioGatewayResourceName, controllers.IstioGatewayNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(returns).To(BeFalse())
			returns, err = util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, watcherCR,
				controllers.IstioGatewayResourceName, controllers.IstioGatewayNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(returns).To(BeFalse())

			// update watcher CR
			currentWatcherCR := &watcherapiv1alpha1.Watcher{}
			Expect(k8sClient.Get(ctx, watcherObjKey, currentWatcherCR)).To(Succeed())
			currentWatcherCR.SetLabels(map[string]string{"label-name": "label-value"})
			Expect(k8sClient.Update(ctx, currentWatcherCR)).Should(Succeed())

			Eventually(watcherCRState(watcherObjKey)).WithTimeout(2 * time.Second).
				WithPolling(20 * time.Microsecond).
				Should(Equal(watcherapiv1alpha1.WatcherStateReady))

			Expect(k8sClient.Get(ctx, cmObjectKey, &v1.ConfigMap{})).To(Succeed())
			returns, err = util.PerformIstioGWCheck(ctx, istioClientSet, gatewayPortNumber,
				controllers.IstioGatewayResourceName, controllers.IstioGatewayNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(returns).To(BeFalse())
			returns, err = util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, watcherCR,
				controllers.IstioGatewayResourceName, controllers.IstioGatewayNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(returns).To(BeFalse())

			Expect(k8sClient.Get(ctx, watcherObjKey, currentWatcherCR)).To(Succeed())
			Expect(k8sClient.Delete(ctx, currentWatcherCR)).To(Succeed())
			Eventually(isCRDeletetionSuccessful(watcherObjKey)).WithTimeout(2 * time.Second).
				WithPolling(20 * time.Microsecond).Should(BeTrue())
		}, watcherCREntries)
})

func isCRDeletetionSuccessful(watcherObjKey client.ObjectKey) func(g Gomega) bool {
	return func(g Gomega) bool {
		// watcherCR := &watcherapiv1alpha1.Watcher{}
		err := k8sClient.Get(ctx, watcherObjKey, &watcherapiv1alpha1.Watcher{})
		if err == nil || !k8sapierrors.IsNotFound(err) {
			return false
		}
		return true
	}
}

func watcherCRState(watcherObjKey client.ObjectKey) func(g Gomega) watcherapiv1alpha1.WatcherState {
	return func(g Gomega) watcherapiv1alpha1.WatcherState {
		watcherCR := &watcherapiv1alpha1.Watcher{}
		err := k8sClient.Get(ctx, watcherObjKey, watcherCR)
		g.Expect(err).NotTo(HaveOccurred())
		return watcherCR.Status.State
	}
}