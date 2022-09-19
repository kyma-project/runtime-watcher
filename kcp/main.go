/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"os"

	"github.com/go-logr/logr"
	kyma "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()        //nolint:gochecknoglobals
	setupLog = ctrl.Log.WithName("setup") //nolint:gochecknoglobals
)

const (
	port = 9443
)

func init() { //nolint:gochecknoinits
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(watcherv1alpha1.AddToScheme(scheme))
	utilruntime.Must(kyma.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var skrWatcherPath string
	var skrWatcherRelName string
	var vsName string
	var vsNamepace string
	var requeueInterval int
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&skrWatcherPath, "skr-watcher-path", "../skr/chart/skr-webhook", "The path to the skr watcher chart.")
	flag.StringVar(&skrWatcherRelName, "skr-watcher-release", "watcher", "The Helm release name for the skr watcher chart.")
	flag.StringVar(&vsName, "virtual-svc-name", "kcp-events", "The name of the Istio virtual service to be updated.")
	flag.StringVar(&vsNamepace, "virtual-svc-namespace", metav1.NamespaceDefault, "The namespace of the Istio virtual service to be updated.")
	flag.IntVar(&requeueInterval, "requeue-interval", 300, "The reconciliation requeue interval in seconds.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   port,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "38af9e76.kyma-project.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	watcherReconciler := &controllers.WatcherReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		RestConfig: mgr.GetConfig(),
		Config:     getConfigValues(setupLog, skrWatcherPath, skrWatcherRelName, vsName, vsNamepace, requeueInterval),
	}
	watcherReconciler.SetIstioClient()
	if err = watcherReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Watcher")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getConfigValues(logger logr.Logger, skrWatcherPath, skrWatcherRelName, vsName, vsNamespace string, requeueInterval int) *controllers.WatcherConfig {
	fileInfo, err := os.Stat(skrWatcherPath)
	if err != nil || !fileInfo.IsDir() {
		logger.V(1).Error(err, "failed to read local skr chart")
	}
	return &controllers.WatcherConfig{
		VirtualServiceObjKey: client.ObjectKey{
			Name:      vsName,
			Namespace: vsNamespace,
		},
		RequeueInterval:         requeueInterval,
		WebhookChartPath:        skrWatcherPath,
		WebhookChartReleaseName: skrWatcherRelName,
	}
}
