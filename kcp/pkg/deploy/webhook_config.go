package deploy

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/kyma-project/kyma-watcher/kcp/api/v1alpha1"
	manifestLib "github.com/kyma-project/manifest-operator/operator/pkg/manifest"
	"helm.sh/helm/v3/pkg/cli"
	admissionv1 "k8s.io/api/admissionregistration/v1"

	// k8sapiyaml "k8s.io/apimachinery/pkg/util/yaml"
	"github.com/slok/go-helm-template/helm"
	k8sapiyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	webhookHandlerURLPathPattern = "/%s/validate"
	firstElementIdx              = 0
	FileWritePermissions         = 0o644
	DecodeBufferSize             = 2048
	renderedWebhookConfigSuffix  = "rendered"
	kymaProjectDomain            = "webhook.kyma-project.io"
	HelmTemplatesDirName         = "templates"
	webhookAPIVersion            = "admissionregistration.k8s.io/v1"
	webhookConfigKind            = "ValidatingWebhookConfiguration"
)

type WatchableResourcesByModule struct {
	ModuleName  string
	GvrsToWatch []*v1alpha1.WatchableGvr
}

// watchableResources are result of the config merge operation
// config merge will be implemented by https://github.com/kyma-project/kyma-watcher/issues/16
func RedeploySKRWebhook(ctx context.Context, restConfig *rest.Config, watchableResources []*WatchableResourcesByModule,
	helmRepoFile, releaseName, namespace, webhookChartPath, webhookConfigFileName string,
) error {

	// 1.step: update webhook helm chart
	err := updateWebhookChart(ctx, watchableResources,
		releaseName, namespace, webhookChartPath, webhookConfigFileName)
	if err != nil {
		return err
	}
	// 2.step: deploy helm chart
	argsVal := make(map[string]interface{}, 1)
	argsVal["isWebhookConfigRendered"] = true
	skrWatcherDeployInfo := manifestLib.DeployInfo{
		ChartInfo: &manifestLib.ChartInfo{
			ChartPath:   webhookChartPath,
			ReleaseName: releaseName,
		},
	}
	err = deployWatcherHelmChart(ctx, restConfig, helmRepoFile, releaseName, skrWatcherDeployInfo, argsVal)
	if err != nil {
		return err
	}

	return nil
}

func updateWebhookChart(ctx context.Context, watchableResources []*WatchableResourcesByModule,
	releaseName, namespace, webhookChartPath, webhookConfigFileName string,
) error {
	chartFS := os.DirFS(webhookChartPath)
	chart, err := helm.LoadChart(ctx, chartFS)
	if err != nil {
		return err
	}
	// render helm template only for the file that contains the webhook config.
	result, err := helm.Template(ctx, helm.TemplateConfig{
		Chart:       chart,
		ReleaseName: releaseName,
		Namespace:   namespace,
		ShowFiles:   []string{webhookConfigFileName},
	})
	if err != nil {
		return err
	}

	webhookConfig, err := lookupWebhookConfigInYaml(result)
	if err != nil {
		return err
	}
	baseWebhookConfig := getBaseWebhookConfig(webhookConfig)
	webhookConfig.Webhooks = webhooksConfigsFromBaseWebhookAndWatchableResources(baseWebhookConfig, watchableResources)
	webhookConfigYaml, err := k8syaml.Marshal(webhookConfig)
	if err != nil {
		return err
	}
	renderedWebhookConfigFilePath := RenderedConfigFilePath(webhookChartPath, webhookConfigFileName)
	err = os.WriteFile(renderedWebhookConfigFilePath, webhookConfigYaml, FileWritePermissions)
	if err != nil {
		return err
	}
	return nil
}

func lookupWebhookConfigInYaml(yaml string) (*admissionv1.ValidatingWebhookConfiguration, error) {
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	reader := strings.NewReader(yaml)
	k8sYamlDec := k8sapiyaml.NewYAMLOrJSONDecoder(reader, DecodeBufferSize)
	err := k8sYamlDec.Decode(webhookConfig)
	if err != nil {
		return nil, err
	}
	if !isWebhookConfig(webhookConfig.APIVersion, webhookConfig.Kind) {
		return nil, fmt.Errorf("webhook config not found")
	}
	return webhookConfig, nil
}

func isWebhookConfig(apiVersion, kind string) bool {
	return apiVersion == webhookAPIVersion && kind == webhookConfigKind
}

func RenderedConfigFilePath(webhookChartPath, webhookConfigFileName string) string {
	fileNameArray := strings.Split(webhookConfigFileName, ".")
	renderedWebhookConfigFileName := fmt.Sprintf("%s-%s.%s", fileNameArray[0],
		renderedWebhookConfigSuffix, fileNameArray[1])
	return path.Join(webhookChartPath, HelmTemplatesDirName, renderedWebhookConfigFileName)
}

func getBaseWebhookConfig(webhookConfig *admissionv1.ValidatingWebhookConfiguration) *admissionv1.ValidatingWebhook {
	return &webhookConfig.Webhooks[firstElementIdx]
}

func webhooksConfigsFromBaseWebhookAndWatchableResources(baseWebhook *admissionv1.ValidatingWebhook,
	watchableResources []*WatchableResourcesByModule,
) []admissionv1.ValidatingWebhook {
	webhooks := make([]admissionv1.ValidatingWebhook, 0, len(watchableResources))
	for _, watchableResource := range watchableResources {
		webhook := *baseWebhook.DeepCopy()
		moduleName := watchableResource.ModuleName
		configName := fmt.Sprintf("%s.%s", moduleName, kymaProjectDomain)
		rules, labels := rulesAndLabelsFromGvrsToWatch(watchableResource.GvrsToWatch)
		servicePath := fmt.Sprintf(webhookHandlerURLPathPattern, watchableResource.ModuleName)
		webhook.Name = configName
		webhook.ObjectSelector.MatchLabels = labels
		webhook.Rules = rules
		webhook.ClientConfig.Service.Path = &servicePath
		webhooks = append(webhooks, webhook)
	}
	return webhooks
}

func rulesAndLabelsFromGvrsToWatch(gvrsToWatch []*v1alpha1.WatchableGvr) (
	[]admissionv1.RuleWithOperations, map[string]string,
) {
	l := len(gvrsToWatch)
	labelsArray := make([]map[string]string, 0, l)
	labelsMapCapacity := 0
	rules := make([]admissionv1.RuleWithOperations, 0, l)
	for _, gvr := range gvrsToWatch {
		rule := admissionv1.RuleWithOperations{
			Operations: []admissionv1.OperationType{admissionv1.OperationAll},
			Rule: admissionv1.Rule{
				APIGroups:   []string{gvr.Gvr.Group},
				APIVersions: []string{gvr.Gvr.Version},
				Resources:   []string{gvr.Gvr.Resource},
			},
		}
		rules = append(rules, rule)
		labelsArray = append(labelsArray, gvr.LabelsToWatch)
		labelsMapCapacity += len(gvr.LabelsToWatch)
	}

	return rules, aggregateLabels(labelsArray, labelsMapCapacity)
}

func aggregateLabels(maps []map[string]string, cap int) map[string]string {
	result := make(map[string]string, cap)
	for _, mx := range maps {
		for k, v := range mx {
			result[k] = v
		}
	}
	return result
}

func deployWatcherHelmChart(ctx context.Context, restConfig *rest.Config, helmRepoFile, releaseName string,
	deployInfo manifestLib.DeployInfo, argsVals map[string]interface{},
) error {
	logger := logf.FromContext(ctx)
	args := make(map[string]map[string]interface{}, 1)
	args["set"] = argsVals
	ops, err := manifestLib.NewOperations(&logger, restConfig, releaseName,
		&cli.EnvSettings{
			RepositoryConfig: helmRepoFile,
		}, args)
	if err != nil {
		return err
	}
	uninstalled, err := ops.Uninstall(deployInfo)
	if err != nil {
		return err
	}
	if !uninstalled {
		return fmt.Errorf("failed to uninstall webhook config")
	}
	installed, err := ops.Install(deployInfo)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("failed to install webhook config")
	}

	ready, err := ops.VerifyResources(deployInfo)
	if err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("skr webhook resources are not ready")
	}
	return nil
}
