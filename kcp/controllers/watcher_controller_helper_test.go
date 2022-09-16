package controllers_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"

	kymaapi "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherapiv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

const defaultBufferSize = 2048

func deserializeIstioResources(filePath string) ([]*unstructured.Unstructured, error) {
	var istioResourcesList []*unstructured.Unstructured
	//create istio resources
	file, err := os.Open(istioResourcesFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := yaml.NewYAMLOrJSONDecoder(file, defaultBufferSize)
	for {
		istioResource := &unstructured.Unstructured{}
		err = decoder.Decode(istioResource)
		if err == nil {
			istioResourcesList = append(istioResourcesList, istioResource)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return istioResourcesList, nil
}

func prepareRequiredCRDs(testCrdURLs []string) ([]*apiextv1.CustomResourceDefinition, error) {
	var crds []*apiextv1.CustomResourceDefinition
	for _, testCrdURL := range testCrdURLs {
		_, err := url.Parse(testCrdURL)
		if err != nil {
			return nil, err
		}
		resp, err := http.Get(testCrdURL)
		if err != nil {
			return nil, fmt.Errorf("failed pulling content for URL (%s) :%w", testCrdURL, err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed pulling content for URL (%s) with status code: %d", testCrdURL, resp.StatusCode)
		}
		defer resp.Body.Close()
		decoder := yaml.NewYAMLOrJSONDecoder(resp.Body, defaultBufferSize)
		for {
			crd := &apiextv1.CustomResourceDefinition{}
			err = decoder.Decode(crd)
			if err == nil {
				crds = append(crds, crd)
			}
			if errors.Is(err, io.EOF) {
				break
			}
		}
	}
	return crds, nil
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

//nolint:gochecknoglobals
var watcherCRNames = []string{"lifecycle-manager", "module-manager", "compass"}

//nolint:gochecknoglobals
var watcherCREntries = createTableEntries(watcherCRNames)

func createTableEntries(watcherCRNames []string) []TableEntry {
	tableEntries := []TableEntry{}
	for idx, watcherCRName := range watcherCRNames {
		entry := Entry(fmt.Sprintf("%s-CR-scenario", watcherCRName),
			createWatcherCR(watcherCRName, isEven(idx)),
		)
		tableEntries = append(tableEntries, entry)
	}
	return tableEntries
}

func isEven(idx int) bool {
	return idx%2 == 0
}

func isCrDeletetionFinished(watcherObjKeys ...client.ObjectKey) func(g Gomega) bool {
	if len(watcherObjKeys) > 1 {
		return nil
	}
	if len(watcherObjKeys) == 0 {
		return func(g Gomega) bool {
			watchers := &watcherapiv1alpha1.WatcherList{}
			err := k8sClient.List(ctx, watchers)
			return err == nil && len(watchers.Items) == 0
		}
	}
	return func(g Gomega) bool {
		err := k8sClient.Get(ctx, watcherObjKeys[0], &watcherapiv1alpha1.Watcher{})
		return kerrors.IsNotFound(err)
	}
}

func watcherCRState(watcherObjKey client.ObjectKey) func(g Gomega) watcherapiv1alpha1.WatcherState {
	return func(g Gomega) watcherapiv1alpha1.WatcherState {
		watcherCR := &watcherapiv1alpha1.Watcher{}
		err := k8sClient.Get(ctx, watcherObjKey, watcherCR)
		g.Expect(err).NotTo(HaveOccurred())
		return watcherCR.Status.State
	}
}

// func isWebhookDeployed(webhookName string) func(g Gomega) bool {
// 	return func(g Gomega) bool {
// 		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
// 		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: webhookName}, webhookConfig)
// 		return err == nil
// 	}
// }

func createWatcherCR(moduleName string, statusOnly bool) *watcherapiv1alpha1.Watcher {
	field := watcherapiv1alpha1.SpecField
	if statusOnly {
		field = watcherapiv1alpha1.StatusField
	}
	return &watcherapiv1alpha1.Watcher{
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
			Field: field,
		},
	}
}

func lookupWebhook(webhookCfg *admissionv1.ValidatingWebhookConfiguration,
	watcherCR *watcherapiv1alpha1.Watcher,
) int {
	cfgIdx := -1
	for idx, webhook := range webhookCfg.Webhooks {
		webhookNameParts := strings.Split(webhook.Name, ".")
		if len(webhookNameParts) == 0 {
			continue
		}
		moduleName := webhookNameParts[0]
		objModuleName, exists := watcherCR.Labels[util.ManagedBylabel]
		if !exists {
			return cfgIdx
		}
		if moduleName == objModuleName {
			return idx
		}
	}
	return cfgIdx
}

func verifyWebhookConfig(
	webhook admissionv1.ValidatingWebhook,
	watcherCR *watcherapiv1alpha1.Watcher,
) bool {
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

	return true
}
