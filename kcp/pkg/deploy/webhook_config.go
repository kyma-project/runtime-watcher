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

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
}

type Mode string

const (
	ModeInstall   = Mode("install")
	ModeUninstall = Mode("uninstall")
)

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
	return installOrRemoveChartOnSKR(ctx, restConfig, releaseName, argsVals, skrWatcherDeployInfo, ModeInstall)
}

func RemoveSKRWebhook(ctx context.Context, webhookChartPath, releaseName string,
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
	return installOrRemoveChartOnSKR(ctx, restConfig, releaseName, argsVals, skrWatcherDeployInfo, ModeUninstall)

}

func installOrRemoveChartOnSKR(ctx context.Context, restConfig *rest.Config, releaseName string,
	argsVals map[string]interface{}, deployInfo lifecycleLib.InstallInfo, mode Mode,
) error {
	logger := logf.FromContext(ctx)
	args := make(map[string]map[string]interface{}, 1)
	args["set"] = argsVals
	ops, err := lifecycleLib.NewOperations(&logger, restConfig, releaseName,
		&cli.EnvSettings{}, args, nil)
	if err != nil {
		return err
	}
	if mode == ModeUninstall {
		uninstalled, err := ops.Uninstall(deployInfo)
		if err != nil {
			return err
		}
		if !uninstalled {
			return fmt.Errorf("failed to install webhook config")
		}
		return nil
	}
	installed, err := ops.Install(deployInfo)
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
