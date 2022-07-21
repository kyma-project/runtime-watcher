package config

import (
	"context"
	"encoding/json"
	"errors"
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

// JSON struct for labels to watch
type LabelsToWatch struct {
	LabelValueList []LabelValuePair `json:"labelValueList"`
}

type LabelValuePair struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// JSON struct for GroupVersions to watch
type gvToWatch struct {
	GvrList []struct {
		Group    string `json:"group"`
		Version  string `json:"version"`
		Resource string `json:"resource"`
	} `json:"gvrList"`
}

func GetGvs(ctx context.Context, namespace, name string, client client.Client, log logr.Logger) []schema.GroupVersion {
	// Fetch config secret
	configSecret, err := getConfigSecret(ctx, namespace, name, client, log)
	if err != nil {
		log.Info(fmt.Sprintf("Error fetching config secret: %s", err.Error()))
		return nil
	}

	// Get data from secret
	data := gvToWatch{}
	err = json.Unmarshal(configSecret.Data["gvToWatch"], &data)
	if err != nil {
		log.Info(fmt.Sprintf("Error unmarshalling data from secret: %s", err.Error()))
		return nil
	}

	// GetGvs which will be watched
	var gvs = []schema.GroupVersion{}
	for _, gv := range data.GvrList {
		gvs = append(gvs, schema.GroupVersion{Group: gv.Group, Version: gv.Version})
	}
	return gvs
}

func GetLabelsToWatch(ctx context.Context, namespace, name string, client client.Client, log logr.Logger) []LabelValuePair {
	// Fetch config secret
	configSecret, err := getConfigSecret(ctx, namespace, name, client, log)
	if err != nil {
		log.Info(fmt.Sprintf("Error fetching config secret: %s", err.Error()))
		return nil
	}

	// Get data from secret
	data := LabelsToWatch{}
	err = json.Unmarshal(configSecret.Data["labelsToWatch"], &data)
	if err != nil {
		log.Info(fmt.Sprintf("Error unmarshalling data from secret: %s", err.Error()))
		return nil
	}

	return data.LabelValueList
}

//TODO: In next iteration: mount secret instead of using kubeconfig
func getConfigSecret(ctx context.Context, namespace, name string, client client.Client, log logr.Logger) (*v1.Secret, error) {
	// Get config secret
	var configSecret = &v1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name},
		configSecret)
	cacheNotStartedError := cache.ErrCacheNotStarted{}
	if err.Error() == cacheNotStartedError.Error() {
		// cache has not been started, create temporary in-cluster config
		log.Info("Cluster cache not started, will create a temporary in-cluster client")
		cl, err := config.GetConfig()
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Unable to get kube-config %s", err))
		}

		clientset, err := kubernetes.NewForConfig(cl)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Unable to create our clientset: %s", err))
		}
		configSecret, err = clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, errors.New(fmt.Sprintf("No Secret for label reference found: %s", err.Error()))
		}
	}
	return configSecret, nil
}
