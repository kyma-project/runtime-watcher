package config

import (
	"context"
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

// Default values
const ComponentLabel = "operator.kyma-project.io/controller-name"
const KymaCrLabel = "operator.kyma-project.io/kyma-name"

const WatcherSecretLabel = "operator.kyma-project.io/task-name"
const WatcherSecretLabelValue = "label-watching"

const KcpIp = "http://localhost"
const KcpPort = "8082"
const ContractVersion = "v1"
const EventEndpoint = "event"

// TODO: Replace it with a k8s secret
func Gvs(ctx context.Context, namespace, name string, client client.Client, log logr.Logger) []schema.GroupVersion {

	var labelSecret = &v1.Secret{}
	err := client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name},
		labelSecret)
	cacheNotStartedError := cache.ErrCacheNotStarted{}
	if err.Error() == cacheNotStartedError.Error() {
		// cache has not been started, create temporary in-cluster config
		log.Info("Watcher runs in in-cluster mode")
		cl, err := config.GetConfig()
		if err != nil {
			panic("Application not running inside of K8s cluster")
		} else if err != nil {
			panic(fmt.Sprintf("Unable to get our client configuration: %s", err))
		}

		clientset, err := kubernetes.NewForConfig(cl)
		if err != nil {
			panic("Unable to create our clientset")
		}
		labelSecret, err = clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			log.Info("No Secret for label reference found: %s", err.Error())
		}

	}
	// Gvs which will be watched

	var gvs = []schema.GroupVersion{
		{
			Group:   "",
			Version: "v1",
		},
		{
			Group:   "operator.kyma-project.io",
			Version: "v1alpha1",
		},
		{
			Group:   "component.kyma-project.io",
			Version: "v1alpha1",
		},
	}
	return gvs
}

// TODO: Replace it with a k8s secret
func LabelsToWatch() map[string]string {
	var labels = map[string]string{"operator.kyma-project.io/managed-by": "Kyma"}
	return labels
}
