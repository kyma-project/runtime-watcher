package deploy

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/go-logr/logr"
	"github.com/kyma-project/module-manager/operator/pkg/custom"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"

	lifecycleLib "github.com/kyma-project/module-manager/operator/pkg/manifest"
	"helm.sh/helm/v3/pkg/cli"

	componentv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	customChartConfigPath = "pkg/deploy/assets/custom-modules-config.yaml"
	customConfigKey       = "modules"
	FileWritePermissions  = 0o644
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
	obj *componentv1alpha1.Watcher, restConfig *rest.Config,
) error {
	err := updateChartConfigFileForCR(obj)
	if err != nil {
		return err
	}
	argsVals, err := generateHelmChartArgs()
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

func updateChartConfigFileForCR(obj *componentv1alpha1.Watcher) error {
	currDir, err := os.Getwd()
	if err != nil {
		return err
	}
	absoluteFilePath := path.Join(currDir, customChartConfigPath)
	_, err = os.Stat(absoluteFilePath)
	if os.IsNotExist(err) {

		chartCfg := generateWatchableConfigForCR(obj)
		bytes, err := k8syaml.Marshal(map[string]map[string]WatchableConfig{
			customConfigKey: chartCfg,
		})
		if err != nil {
			return err
		}
		err = os.WriteFile(absoluteFilePath, bytes, FileWritePermissions)
		if err != nil {
			return err
		}
		return nil
	}

	currentConfig, err := getCurrentConfig(absoluteFilePath)
	if err != nil {
		return err
	}
	moduleName := obj.Labels[util.ManagedBylabel]
	_, ok := currentConfig[moduleName]
	if ok {

		return nil
	}
	updatedConfig := make(map[string]WatchableConfig, len(currentConfig)+1)

	for k, v := range currentConfig {
		updatedConfig[k] = v
	}
	statusOnly := obj.Spec.SubresourceToWatch == componentv1alpha1.SubresourceTypeStatus
	updatedConfig[moduleName] = WatchableConfig{
		Labels:     obj.Spec.LabelsToWatch,
		StatusOnly: statusOnly,
	}
	bytes, err := k8syaml.Marshal(map[string]map[string]WatchableConfig{
		customConfigKey: updatedConfig,
	})
	if err != nil {
		return err
	}
	err = os.WriteFile(customChartConfigPath, bytes, FileWritePermissions)
	if err != nil {
		return err
	}
	return nil
}

func getCurrentConfig(absoluteFilePath string) (map[string]WatchableConfig, error) {
	customChartConfig := map[string]map[string]WatchableConfig{}
	bytes, err := os.ReadFile(absoluteFilePath)
	if err != nil {
		return nil, err
	}
	err = k8syaml.Unmarshal(bytes, &customChartConfig)
	if err != nil {
		return nil, err
	}
	currentConfig, ok := customChartConfig[customConfigKey]
	if !ok {
		return nil, fmt.Errorf("error getting modules config")
	}
	return currentConfig, nil
}

func RemoveSKRWebhook(ctx context.Context, webhookChartPath, releaseName string,
	watchableConfigs map[string]WatchableConfig, restConfig *rest.Config,
) error {
	argsVals, err := generateHelmChartArgs()
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

func generateHelmChartArgs() (map[string]interface{}, error) {
	currDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	absoluteFilePath := path.Join(currDir, customChartConfigPath)
	currentConfig, err := getCurrentConfig(absoluteFilePath)
	if err != nil {
		return nil, fmt.Errorf("error getting modules config")
	}
	bytes, err := k8syaml.Marshal(currentConfig)
	if err != nil {
		return nil, err
	}
	helmChartArgs := make(map[string]interface{}, 1)
	helmChartArgs[customConfigKey] = string(bytes)
	return helmChartArgs, nil
}

func generateWatchableConfigForCR(obj *componentv1alpha1.Watcher) map[string]WatchableConfig {
	statusOnly := obj.Spec.SubresourceToWatch == componentv1alpha1.SubresourceTypeStatus
	return map[string]WatchableConfig{
		obj.Labels[util.ManagedBylabel]: {
			Labels:     obj.Spec.LabelsToWatch,
			StatusOnly: statusOnly,
		},
	}
}
