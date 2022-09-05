package deploy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyma-project/module-manager/operator/pkg/custom"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"

	lifecycleLib "github.com/kyma-project/module-manager/operator/pkg/manifest"
	"helm.sh/helm/v3/pkg/cli"

	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	WebhookHandlerURLPathPattern = "/%s/validate"
	firstElementIdx              = 0
	FileWritePermissions         = 0o644
	DecodeBufferSize             = 2048
	renderedWebhookConfigSuffix  = "rendered"
	KymaProjectWebhookFQDN       = "webhook.kyma-project.io"
	HelmTemplatesDirName         = "templates"
	webhookAPIVersion            = "admissionregistration.k8s.io/v1"
	webhookConfigKind            = "ValidatingWebhookConfiguration"
)

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
}

func InstallSKRWebhook(ctx context.Context, webhookChartPath, releaseName string,
	watchableConfigs map[string]WatchableConfig, restConfig *rest.Config,
) error {
	argsVals, err := generateHelmChartArgs(watchableConfigs)
	if err != nil {
		return err
	}
	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	skrWatcherDeployInfo := lifecycleLib.InstallInfo{
		ChartInfo: &lifecycleLib.ChartInfo{
			ChartPath:   webhookChartPath,
			ReleaseName: releaseName,
		},
		RemoteInfo: custom.RemoteInfo{
			RemoteClient: &restClient,
			RemoteConfig: restConfig,
		},
		CheckFn: func(ctx context.Context, u *unstructured.Unstructured, logger *logr.Logger, info custom.RemoteInfo,
		) (bool, error) {
			return true, nil
		},
	}
	logger := logf.FromContext(ctx)
	args := make(map[string]map[string]interface{}, 1)
	args["set"] = argsVals
	ops, err := lifecycleLib.NewOperations(&logger, restConfig, releaseName,
		&cli.EnvSettings{}, args, nil)
	if err != nil {
		return err
	}
	installed, err := ops.Install(skrWatcherDeployInfo)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("failed to install webhook config")
	}
	return nil
}

func generateHelmChartArgs(watchableConfigs map[string]WatchableConfig) (map[string]interface{}, error) {
	helmChartArgs := make(map[string]interface{}, 1)
	bytes, err := k8syaml.Marshal(watchableConfigs)
	if err != nil {
		return nil, err
	}

	helmChartArgs["modules"] = string(bytes)
	return helmChartArgs, nil
}
