package deploy

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kymav1alpha1 "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	customChartConfigName      = "custom-modules-config"
	customChartConfigNamespace = metav1.NamespaceDefault
	customConfigKey            = "modules"
	kubeconfigKey              = "config"
	servicePathTpl             = "/validate/%s"
	webhookNameTpl             = "%s.operator.kyma-project.io"
	specSubresources           = "*"
	statusSubresources         = "*/status"
)

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
}

func UpdateWebhookConfig(ctx context.Context, chartPath, releaseName string,
	obj *watcherv1alpha1.Watcher, inClusterCfg *rest.Config, k8sClient client.Client,
) error {
	restCfgs, err := getSKRRestConfigs(ctx, k8sClient, inClusterCfg)
	if err != nil {
		return err
	}
	for _, restCfg := range restCfgs {
		err = updateWebhookConfigOrInstallSKRChart(ctx, chartPath, releaseName, obj, restCfg, k8sClient)
		if err != nil {
			continue
		}
	}
	// return err so that if err!=nil for at least one SKR, reconciliation will be retriggered after requeue interval
	return err
}

func updateWebhookConfigOrInstallSKRChart(ctx context.Context, chartPath, releaseName string,
	obj *watcherv1alpha1.Watcher, restConfig *rest.Config, k8sClient client.Client,
) error {
	remoteClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	// TODO: make webhook config name a chart value
	err = remoteClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if k8sapierrors.IsNotFound(err) {
		// install chart
		return InstallSKRWebhook(ctx, chartPath, releaseName, obj, restConfig)
	}
	//generate webhook config from CR and update webhook config resource
	if len(webhookConfig.Webhooks) < 1 {
		return fmt.Errorf("failed to get base webhook config")
	}
	updatedWebhookCfg := generateWebhookConfigForCR(webhookConfig.Webhooks[0], obj)
	webhookConfig.Webhooks = append(webhookConfig.Webhooks, updatedWebhookCfg)
	return remoteClient.Update(ctx, webhookConfig)
}

func generateWebhookConfigForCR(baseCfg admissionv1.ValidatingWebhook, obj *watcherv1alpha1.Watcher) admissionv1.ValidatingWebhook {
	watcherCrWebhookCfg := baseCfg.DeepCopy()
	moduleName := obj.Labels[util.ManagedBylabel]
	watcherCrWebhookCfg.Name = fmt.Sprintf(webhookNameTpl, moduleName)
	if obj.Spec.LabelsToWatch != nil {
		watcherCrWebhookCfg.ObjectSelector.MatchLabels = obj.Spec.LabelsToWatch
	}
	servicePath := fmt.Sprintf(servicePathTpl, moduleName)
	watcherCrWebhookCfg.ClientConfig.Service.Path = &servicePath
	if obj.Spec.Field == watcherv1alpha1.StatusField {
		watcherCrWebhookCfg.Rules[0].Resources[0] = statusSubresources
	}
	watcherCrWebhookCfg.Rules[0].Resources[0] = specSubresources
	return *watcherCrWebhookCfg
}

func RemoveWebhookConfig(ctx context.Context, chartPath, releaseName string,
	obj *watcherv1alpha1.Watcher, inClusterCfg *rest.Config, k8sClient client.Client,
) error {
	restCfgs, err := getSKRRestConfigs(ctx, k8sClient, inClusterCfg)
	if err != nil {
		return err
	}
	for _, restCfg := range restCfgs {
		err = removeWebhookConfig(ctx, chartPath, releaseName, obj, restCfg, k8sClient)
		if err != nil {
			continue
		}
	}
	// return err so that if err!=nil for at least one SKR, reconciliation will be retriggered after requeue interval
	return err
}

func removeWebhookConfig(ctx context.Context, chartPath, releaseName string,
	obj *watcherv1alpha1.Watcher, restConfig *rest.Config, k8sClient client.Client,
) error {
	remoteClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	// TODO: make webhook config name a chart value
	err = remoteClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "skr-webhook"}, webhookConfig)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if k8sapierrors.IsNotFound(err) {
		return nil
	}
	l := len(webhookConfig.Webhooks)
	if l < 2 {
		// this watcher CR is the latest CR configured on the SKR webhook
		// remove the webhook configuration
		return remoteClient.Delete(ctx, webhookConfig)
	}
	cfgIdx := -1
	for idx, webhook := range webhookConfig.Webhooks {
		webhookNameParts := strings.Split(webhook.Name, ".")
		if len(webhookNameParts) == 0 {
			continue
		}
		moduleName := webhookNameParts[0]
		objModuleName, exists := obj.Labels[util.ManagedBylabel]
		if !exists {
			break
		}
		if moduleName == objModuleName {
			cfgIdx = idx
		}
	}
	if cfgIdx != -1 {
		// remove corresponding config from webhook config resource
		copy(webhookConfig.Webhooks[cfgIdx:], webhookConfig.Webhooks[cfgIdx+1:])
		webhookConfig.Webhooks[l-1] = admissionv1.ValidatingWebhook{}
		webhookConfig.Webhooks = webhookConfig.Webhooks[:len(webhookConfig.Webhooks)-1]
		return remoteClient.Update(ctx, webhookConfig)
	}
	return nil
}

func getSKRRestConfigs(ctx context.Context, reader client.Reader, inClusterCfg *rest.Config) (map[string]*rest.Config, error) {
	kymas := &kymav1alpha1.KymaList{}
	err := reader.List(ctx, kymas)
	if err != nil {
		return nil, err
	}
	restCfgMap := make(map[string]*rest.Config, len(kymas.Items))
	for _, kyma := range kymas.Items {
		if kyma.Spec.Sync.Strategy == kymav1alpha1.SyncStrategyLocalClient || !kyma.Spec.Sync.Enabled {
			restCfgMap[kyma.Name] = inClusterCfg
			continue
		}
		secret := &v1.Secret{}
		//nolint:gosec
		err = reader.Get(ctx, client.ObjectKeyFromObject(&kyma), secret)
		if err != nil {
			return nil, err
		}
		restCfg, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[kubeconfigKey])
		if err == nil {
			restCfgMap[kyma.Name] = restCfg
		}
	}

	return restCfgMap, nil
}
