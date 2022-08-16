package deploy_test

import (
	"os"

	"github.com/kyma-project/kyma-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/kyma-watcher/kcp/pkg/deploy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

var _ = Describe("deploy watcher", func() {
	watchableRes := createSampleWatchableResourcesForModule()
	webhookConfigFileName := "webhook-config.yaml"
	webhookChartPath := "assets/sample-chart"
	releaseName := "watcher"
	namespace := "kyma-system"
	helmRepoFile := "assets/repositories.yaml"
	renderedTplFilePath := deploy.RenderedConfigFilePath(webhookChartPath, webhookConfigFileName)
	It("deploy watcher helm chart with correct webhook config", func() {
		err := deploy.DeploySKRWebhook(ctx, testEnv.Config, watchableRes, helmRepoFile, releaseName, namespace, webhookChartPath, webhookConfigFileName)
		Expect(err).ShouldNot(HaveOccurred())
		// check rendered configs
		yamlFile, err := os.Open(renderedTplFilePath)
		Expect(err).ShouldNot(HaveOccurred())
		k8sYamlDec := k8syaml.NewYAMLOrJSONDecoder(yamlFile, deploy.DecodeBufferSize)
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		err = k8sYamlDec.Decode(webhookConfig)
		Expect(err).ShouldNot(HaveOccurred())
		checkRes, err := checkRenderedWebhookConfig(webhookConfig.Webhooks)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(checkRes).To(BeTrue())
		//check deployed resources
		checkRes, err = checkInstalledChartResources(k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(checkRes).To(BeTrue())

	})
})

func checkInstalledChartResources(k8sClient client.Client) (bool, error) {
	secret := &v1.Secret{}
	err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "skr-webhook-tls"}, secret)
	if err != nil {
		return false, err
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err = k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "skr-webhook"}, webhookConfig)
	if err != nil {
		return false, err
	}
	return true, nil
}

func checkRenderedWebhookConfig(webhookConfigs []admissionv1.ValidatingWebhook) (bool, error) {
	//TODO: add verification of webhook configs
	return true, nil
}

func createSampleWatchableResourcesForModule() []*deploy.WatchableResourcesByModule {
	labelsToWatch := make(map[string]string, 1)
	labelsToWatch["kyma-label"] = "kyma-label-value"
	return []*deploy.WatchableResourcesByModule{
		{
			ModuleName: "kyma",
			GvrsToWatch: []*v1alpha1.WatchableGvr{
				{
					Gvr: v1alpha1.Gvr{
						Group:    "operator.kyma-project.io",
						Version:  "v1alpha1",
						Resource: "kymas",
					}, LabelsToWatch: labelsToWatch,
				},
			},
		},
		{
			ModuleName: "compass",
			GvrsToWatch: []*v1alpha1.WatchableGvr{
				{
					Gvr: v1alpha1.Gvr{
						Group:    "component.kyma-project.io",
						Version:  "v1alpha1",
						Resource: "compasses",
					},
				},
			},
		},
	}
}
