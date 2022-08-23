package controllers_test

import (
	"time"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	componentv1alpha1 "github.com/kyma-project/kyma-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/kyma-watcher/kcp/controllers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

const (
	namespace        = "default"
	contractVersion  = "v1"
	componentName    = "example-module"
	componentChannel = "stable"
	serviceName      = "test-service"
	servicePort      = 8083
	watcherName      = componentName + "-" + componentChannel
	configMapName    = "kcp-watcher-modules"

	interval = time.Millisecond * 250
)

var _ = Describe("Correct WatcherCR Setup", func() {
	testKyma := NewTestKyma("test-kyma")
	testConfigMap := NewTestConfigMap(configMapName)
	testWatcherCR := NewTestWatcherCR(watcherName, map[string]string{controllers.DefaultOperatorWatcherCRLabel: "true"})

	SetupTestEnvironment(testKyma, testConfigMap, testWatcherCR)

	It("should insert testKyma in the ConfigMap of the example-module WatcherCR", func() {
		By("checking the data of the ConfigMap")
		Eventually(GetConfigMapData(testConfigMap), 5*time.Second, interval).
			Should(Equal(map[string]string{
				"example-module-stable": "{\"kymaCrList\":[{\"kymaCr\":\"test-kyma\",\"kymaNamespace\":\"default\"}]}",
			},
			))
	})
})

var _ = Describe("WatcherCR applies to all Kymas - Configmap stores no data", func() {
	testKyma := NewTestKyma("test-kyma")
	testConfigMap := NewTestConfigMap(configMapName)
	testWatcherCR := NewTestWatcherCR(watcherName, map[string]string{})

	SetupTestEnvironment(testKyma, testConfigMap, testWatcherCR)

	It("should not throw an error in reconcile loop an ConfigMap should not be initialised nor updated", func() {
		By("checking the data of the ConfigMap")
		Eventually(GetConfigMapData(testConfigMap), 5*time.Second, interval).Should(BeEmpty())
	})
})

func SetupTestEnvironment(kyma *v1alpha1.Kyma, configmap *v1.ConfigMap, watcher *componentv1alpha1.Watcher) {
	BeforeEach(func() {
		Expect(k8sClient.Create(ctx, configmap)).Should(Succeed())
		Expect(k8sClient.Create(ctx, watcher)).Should(Succeed())
		Expect(k8sClient.Create(ctx, kyma)).Should(Succeed())
	})
	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, configmap)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, watcher)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, kyma)).Should(Succeed())
	})
}

func NewTestKyma(name string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       v1alpha1.KymaKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.KymaSpec{
			Modules: []v1alpha1.Module{
				{
					Name:           componentName,
					ControllerName: "",
					Channel:        componentChannel,
					Settings:       unstructured.Unstructured{},
				},
			},
			Channel: v1alpha1.DefaultChannel,
			Profile: v1alpha1.DefaultProfile,
		},
	}
}

func NewTestConfigMap(name string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{Kind: "ConfigMap", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       make(map[string]string),
	}
}

func NewTestWatcherCR(name string, labels map[string]string) *componentv1alpha1.Watcher {
	return &componentv1alpha1.Watcher{
		TypeMeta: metav1.TypeMeta{
			Kind:       componentv1alpha1.WatcherKind,
			APIVersion: componentv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: componentv1alpha1.WatcherSpec{
			ContractVersion: contractVersion,
			ComponentName:   componentName,
			ServiceInfo: componentv1alpha1.ServiceInfo{
				ServicePort: servicePort,
				ServiceName: serviceName,
			},
			GvrsToWatch: []componentv1alpha1.WatchableGvr{
				{
					Gvr: componentv1alpha1.Gvr{
						Group:    v1alpha1.GroupVersion.Group,
						Version:  v1alpha1.GroupVersion.Version,
						Resource: v1alpha1.KymaPlural,
					},
					LabelsToWatch: map[string]string{},
				},
			},
		},
		Status: componentv1alpha1.WatcherStatus{},
	}
}

func GetConfigMapData(configMap *v1.ConfigMap) func() map[string]string {
	return func() map[string]string {
		createdConfigMap := &v1.ConfigMap{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      configMap.GetName(),
			Namespace: configMap.GetNamespace(),
		}, createdConfigMap)
		if err != nil {
			return map[string]string{}
		}
		return createdConfigMap.Data
	}
}
