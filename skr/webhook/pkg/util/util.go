package util

import (
	"flag"
	"fmt"
	"os"
	"path"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func GetConfig(kubeConfig string, explicitPath string) (*rest.Config, error) {
	logger := ctrl.Log.WithName("getRestConfig")
	if kubeConfig != "" {
		// parameter string
		return clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
			logger.Info("Found config from passed kubeconfig")
			return clientcmd.Load([]byte(kubeConfig))
		})
	}
	// in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		logger.Info("Found config in-cluster")
		return config, err
	}

	// kubeconfig flag
	if flag.Lookup("kubeconfig") != nil {
		if kubeconfig := flag.Lookup("kubeconfig").Value.String(); kubeconfig != "" {
			logger.Info("Found config from flags")
			return clientcmd.BuildConfigFromFlags("", kubeconfig)
		}
	}

	// env variable
	if len(os.Getenv("KUBECONFIG")) > 0 {
		logger.Info("Found config from env")
		return clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	}

	// default directory + working directory + explicit path -> merged
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = explicitPath
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error reading current working directory %w", err)
	}
	loadingRules.Precedence = append(loadingRules.Precedence, path.Join(pwd, ".kubeconfig"))
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	logger.Info(fmt.Sprintf("Found config file in: %s", clientConfig.ConfigAccess().GetDefaultFilename()))
	return config, nil
}
