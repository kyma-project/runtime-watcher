package deploy_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/kyma-project/runtime-watcher/kcp/pkg/deploy"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kymaapi "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherapiv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	servicePathTpl     = "/validate/%s"
	specSubresources   = "*"
	statusSubresources = "*/status"
	webhookChartPath   = "assets/sample-chart"
	releaseName        = "watcher"
)

var _ = Describe("deploy watcher", Ordered, func() {
	ctx := context.TODO()
	moduleName := "lifecyle-manager"
	watcherCR := &watcherapiv1alpha1.Watcher{
		TypeMeta: metav1.TypeMeta{
			Kind:       watcherapiv1alpha1.WatcherKind,
			APIVersion: watcherapiv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sample", moduleName),
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				util.ManagedBylabel: moduleName,
			}},
		Spec: watcherapiv1alpha1.WatcherSpec{
			ServiceInfo: watcherapiv1alpha1.Service{
				Port:      8082,
				Name:      fmt.Sprintf("%s-svc", moduleName),
				Namespace: metav1.NamespaceDefault,
			},
			LabelsToWatch: map[string]string{
				fmt.Sprintf("%s-watchable", moduleName): "true",
			},
			Field: watcherapiv1alpha1.StatusField,
		},
	}
	kymaSample := &kymaapi.Kyma{}
	BeforeAll(func() {
		kymaName := "kyma-sample"
		kymaSample = createKymaCR(kymaName)
		Expect(k8sClient.Create(ctx, kymaSample)).To(Succeed())
	})

	AfterAll(func() {
		// clean up kyma CR
		Expect(k8sClient.Delete(ctx, kymaSample)).To(Succeed())
	})

	It("deploys watcher helm chart with correct webhook config", func() {
		err := deploy.UpdateWebhookConfig(ctx, webhookChartPath, releaseName, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(verifyWebhookConfig(webhookConfig, watcherCR)).To(BeTrue())
	})

	It("updates webhook config when helm chart is already installed", func() {
		watcherCR.Spec.Field = watcherapiv1alpha1.SpecField
		err := deploy.UpdateWebhookConfig(ctx, webhookChartPath, releaseName, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(verifyWebhookConfig(webhookConfig, watcherCR)).To(BeTrue())
	})

	It("removes webhook config resource from SKR cluster when last cr is deleted", func() {
		err := deploy.RemoveWebhookConfig(ctx, webhookChartPath, releaseName, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(k8sapierrors.IsNotFound(err)).To(BeTrue())
	})
})

func verifyWebhookConfig(
	webhookCfg *admissionv1.ValidatingWebhookConfiguration,
	watcherCR *watcherapiv1alpha1.Watcher,
) bool {
	for _, webhook := range webhookCfg.Webhooks {
		webhookNameParts := strings.Split(webhook.Name, ".")
		if len(webhookNameParts) < 2 {
			return false
		}
		moduleName := webhookNameParts[0]
		expectedModuleName, exists := watcherCR.Labels[util.ManagedBylabel]
		if !exists {
			return false
		}
		if moduleName != expectedModuleName {
			return false
		}
		if *webhook.ClientConfig.Service.Path != fmt.Sprintf(servicePathTpl, moduleName) {
			return false
		}

		if !reflect.DeepEqual(webhook.ObjectSelector.MatchLabels, watcherCR.Spec.LabelsToWatch) {
			return false
		}
		if watcherCR.Spec.Field == watcherapiv1alpha1.StatusField && webhook.Rules[0].Resources[0] != statusSubresources {
			return false
		}
		if watcherCR.Spec.Field == watcherapiv1alpha1.SpecField && webhook.Rules[0].Resources[0] != specSubresources {
			return false
		}
	}

	return true
}

func createKymaCR(kymaName string) *kymaapi.Kyma {
	return &kymaapi.Kyma{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(kymaapi.KymaKind),
			APIVersion: kymaapi.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kymaName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: kymaapi.KymaSpec{
			Channel: kymaapi.ChannelStable,
			Modules: []kymaapi.Module{
				{
					Name: "sample-skr-module",
				},
				{
					Name: "sample-kcp-module",
				},
			},
			Sync: kymaapi.Sync{
				Enabled:  false,
				Strategy: kymaapi.SyncStrategyLocalClient,
			},
		},
	}
}
