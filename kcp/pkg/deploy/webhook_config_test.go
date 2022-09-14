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

var _ = Describe("deploy watcher", func() {
	ctx := context.TODO()
	watcherCR := &watcherapiv1alpha1.Watcher{}

	It("deploys watcher helm chart with correct webhook config", func() {
		err := deploy.UpdateWebhookConfig(ctx, webhookChartPath, releaseName, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(err).ShouldNot(HaveOccurred())
		correct := verifyWebhookConfig(webhookConfig, watcherCR)
		Expect(correct).To(BeTrue())
	})

	It("removes webhook config from SKR cluster", func() {
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
