package deploy_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/kyma-project/runtime-watcher/kcp/pkg/deploy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	componentv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	admissionv1 "k8s.io/api/admissionregistration/v1"
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
	watcherCR := &componentv1alpha1.Watcher{}

	It("deploys watcher helm chart with correct webhook config", func() {
		Skip("skipped for now in favor of local testing due to time constraints")
		err := deploy.InstallSKRWebhook(ctx, webhookChartPath, releaseName, watcherCR, testEnv.Config)
		Expect(err).ShouldNot(HaveOccurred())
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err = k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
		Expect(err).ShouldNot(HaveOccurred())
		correct := verifyWebhookConfig(webhookConfig, nil)
		Expect(correct).To(BeTrue())
	})
})

//nolint:unused
func verifyWebhookConfig(webhookCfg *admissionv1.ValidatingWebhookConfiguration,
	watchableConfigs map[string]deploy.WatchableConfig,
) bool {
	for _, webhook := range webhookCfg.Webhooks {
		webhookNameParts := strings.Split(webhook.Name, ".")
		if len(webhookNameParts) < 2 {
			return false
		}
		moduleName := webhookNameParts[0]
		if *webhook.ClientConfig.Service.Path != fmt.Sprintf(servicePathTpl, moduleName) {
			return false
		}
		watchableConfig, ok := watchableConfigs[moduleName]
		if !ok {
			return false
		}
		if !reflect.DeepEqual(webhook.ObjectSelector.MatchLabels, watchableConfig.Labels) {
			return false
		}
		if watchableConfig.StatusOnly && webhook.Rules[0].Resources[0] != statusSubresources {
			return false
		}
		if !watchableConfig.StatusOnly && webhook.Rules[0].Resources[0] != specSubresources {
			return false
		}
	}

	return true
}
