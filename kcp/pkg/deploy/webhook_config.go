package deploy

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyma-project/module-manager/operator/pkg/custom"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lifecycleLib "github.com/kyma-project/module-manager/operator/pkg/manifest"
	"helm.sh/helm/v3/pkg/cli"

	kymav1alpha1 "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	componentv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	customChartConfigName      = "custom-modules-config"
	customChartConfigNamespace = metav1.NamespaceDefault
	customChartConfigPath      = "pkg/deploy/assets/custom-modules-config.yaml"
	customConfigKey            = "modules"
	FileWritePermissions       = 0o644
	kubeconfigKey              = "config"
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

func getSKRRestConfigs(ctx context.Context, r client.Reader) ([]*rest.Config, error) {
	kymas := &kymav1alpha1.KymaList{}
	err := r.List(ctx, kymas)
	if err != nil {
		return nil, err
	}
	restCfgs := []*rest.Config{}
	for _, kyma := range kymas.Items {
		secret := &v1.Secret{}
		err = r.Get(ctx, client.ObjectKeyFromObject(&kyma), secret)
		if err != nil {
			return nil, err
		}
		restCfg, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[kubeconfigKey])
		if err == nil {
			restCfgs = append(restCfgs, restCfg)
		}
	}

	return restCfgs, nil
}

func InstallWebhookOnAllSKRs(ctx context.Context, releaseName string,
	obj *componentv1alpha1.Watcher, r client.Client,
) error {
	restCfgs, err := getSKRRestConfigs(ctx, r)
	if err != nil {
		return err
	}
	for _, restCfg := range restCfgs {
		err = InstallSKRWebhook(ctx, releaseName, obj, restCfg, r)
		if err != nil {
			continue
		}
	}
	// return err so that if err!=nil, reconciliation will be retriggered after requeue interval
	return err
}

func InstallSKRWebhook(ctx context.Context, releaseName string,
	obj *componentv1alpha1.Watcher, restConfig *rest.Config, r client.Client,
) error {
	err := updateChartConfigMapForCR(ctx, r, obj)
	if err != nil {
		return err
	}
	argsVals, err := generateHelmChartArgs(ctx, r)
	if err != nil {
		return err
	}
	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	skrWatcherDeployInfo := lifecycleLib.InstallInfo{
		ChartInfo: &lifecycleLib.ChartInfo{
			ChartPath:   util.DefaultWebhookChartPath,
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

func updateChartConfigMapForCR(ctx context.Context, r client.Client, obj *componentv1alpha1.Watcher) error {
	configMap := &v1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      customChartConfigName,
		Namespace: customChartConfigNamespace,
	}, configMap)
	if k8sapierrors.IsNotFound(err) {

		chartCfg := generateWatchableConfigForCR(obj)
		bytes, err := k8syaml.Marshal(chartCfg)
		if err != nil {
			return err
		}
		configMap.SetName(customChartConfigName)
		configMap.SetNamespace(customChartConfigNamespace)
		configMapData := map[string]string{
			customConfigKey: string(bytes),
		}
		configMap.Data = configMapData
		err = r.Create(ctx, configMap)
		if err != nil {
			return err
		}
		return nil
	}

	rawConfig, ok := configMap.Data[customConfigKey]
	if !ok {
		return fmt.Errorf("error getting modules config")
	}
	currentConfig := map[string]WatchableConfig{}
	err = k8syaml.Unmarshal([]byte(rawConfig), &currentConfig)
	if err != nil {
		return err
	}
	moduleName := obj.Labels[util.ManagedBylabel]
	_, ok = currentConfig[moduleName]
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
	bytes, err := k8syaml.Marshal(updatedConfig)
	if err != nil {
		return err
	}
	configMap.Data[customConfigKey] = string(bytes)
	err = r.Update(ctx, configMap)
	if err != nil {
		return err
	}
	return nil
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

func generateHelmChartArgs(ctx context.Context, r client.Reader) (map[string]interface{}, error) {
	configMap := &v1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      customChartConfigName,
		Namespace: customChartConfigNamespace,
	}, configMap)
	if err != nil {
		return nil, err
	}
	rawConfig, ok := configMap.Data[customConfigKey]
	if !ok {
		return nil, fmt.Errorf("error getting modules config")
	}

	helmChartArgs := make(map[string]interface{}, 1)
	helmChartArgs[customConfigKey] = rawConfig
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
