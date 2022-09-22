package controllers_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	kyma "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
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
		resp, err := http.Get(testCrdURL) //nolint:gosec
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

func createKymaCR(kymaName string) *kyma.Kyma {
	return &kyma.Kyma{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(kyma.KymaKind),
			APIVersion: kyma.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kymaName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: kyma.KymaSpec{
			Channel: kyma.ChannelStable,
			Modules: []kyma.Module{
				{
					Name: "sample-skr-module",
				},
				{
					Name: "sample-kcp-module",
				},
			},
			Sync: kyma.Sync{
				Enabled:  false,
				Strategy: kyma.SyncStrategyLocalClient,
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
			watchers := &watcherv1alpha1.WatcherList{}
			err := k8sClient.List(ctx, watchers)
			return err == nil && len(watchers.Items) == 0
		}
	}
	return func(g Gomega) bool {
		err := k8sClient.Get(ctx, watcherObjKeys[0], &watcherv1alpha1.Watcher{})
		return kerrors.IsNotFound(err)
	}
}

func watcherCRState(watcherObjKey client.ObjectKey) func(g Gomega) watcherv1alpha1.WatcherState {
	return func(g Gomega) watcherv1alpha1.WatcherState {
		watcherCR := &watcherv1alpha1.Watcher{}
		err := k8sClient.Get(ctx, watcherObjKey, watcherCR)
		g.Expect(err).NotTo(HaveOccurred())
		return watcherCR.Status.State
	}
}

func createWatcherCR(moduleName string, statusOnly bool) *watcherv1alpha1.Watcher {
	field := watcherv1alpha1.SpecField
	if statusOnly {
		field = watcherv1alpha1.StatusField
	}
	return &watcherv1alpha1.Watcher{
		TypeMeta: metav1.TypeMeta{
			Kind:       watcherv1alpha1.WatcherKind,
			APIVersion: watcherv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sample", moduleName),
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				watcherv1alpha1.ManagedBylabel: moduleName,
			},
		},
		Spec: watcherv1alpha1.WatcherSpec{
			ServiceInfo: watcherv1alpha1.Service{
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
