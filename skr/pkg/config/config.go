package config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// JSON struct for GroupVersionResources to watch
type gvrToWatch struct {
	GvrList []struct {
		Group          string           `json:"group"`
		Version        string           `json:"version"`
		Resource       string           `json:"resource"`
		LabelValueList []LabelValuePair `json:"labelValueList"`
	} `json:"gvrList"`
}

type LabelValuePair struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type WatchItem struct {
	Gvr    schema.GroupVersionResource
	Labels map[string]string
}

func GetGvr(ctx context.Context, namespace, name string, kubeClient client.Client, log logr.Logger) []WatchItem {
	// Fetch config secret
	configSecret, err := getConfigSecret(ctx, namespace, name, kubeClient, log)
	if err != nil {
		log.Info(fmt.Sprintf("Error fetching config secret: %s", err.Error()))
		return nil
	}

	// Get data from secret
	data := gvrToWatch{}
	err = json.Unmarshal(configSecret.Data["gvrToWatch"], &data)
	if err != nil {
		log.Info(fmt.Sprintf("Error unmarshalling data from secret: %s", err.Error()))
		return nil
	}

	// Construct GVR list with labels
	var grvList []WatchItem
	for _, gvr := range data.GvrList {
		grvList = append(grvList,
			WatchItem{
				Gvr: schema.GroupVersionResource{
					Group:    gvr.Group,
					Version:  gvr.Version,
					Resource: gvr.Resource},
				Labels: labelsListToMap(gvr.LabelValueList)})
	}
	return grvList
}

func labelsListToMap(labelList []LabelValuePair) map[string]string {
	lvMap := map[string]string{}
	for _, lv := range labelList {
		lvMap[lv.Label] = lv.Value
	}
	return lvMap
}

//TODO: In next iteration: mount secret in deployment instead of using kubeconfig
func getConfigSecret(ctx context.Context, namespace, name string, kubeClient client.Client, log logr.Logger) (*v1.Secret, error) {
	// Get config secret
	var configSecret = &v1.Secret{}
	err := kubeClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name},
		configSecret)
	cacheNotStartedError := cache.ErrCacheNotStarted{}
	if err.Error() == cacheNotStartedError.Error() {
		// cache has not been started, create temporary in-cluster config
		log.Info("Cluster cache not started, will create a temporary in-cluster kubeClient")
		cl, err := config.GetConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to get kube-config %s", err)
		}

		clientset, err := kubernetes.NewForConfig(cl)
		if err != nil {
			return nil, fmt.Errorf("unable to create our clientset: %s", err)
		}
		configSecret, err = clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("no Secret for label reference found: %s", err)
		}
	}
	return configSecret, nil
}
