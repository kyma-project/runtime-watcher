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

package controllers

import (
	"context"
	"github.com/kyma-project/manifest-operator/operator/pkg/custom"
	manifestLib "github.com/kyma-project/manifest-operator/operator/pkg/manifest"
	"helm.sh/helm/v3/pkg/cli"
	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	componentv1alpha1 "github.com/kyma-project/kyma-watcher/kcp/api/v1alpha1"
)

// WatcherReconciler reconciles a Watcher object
type WatcherReconciler struct {
	client.Client
	RestConfig *rest.Config
	Scheme     *runtime.Scheme
}

//+kubebuilder:rbac:groups=component.kyma-project.io,resources=watchers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=watchers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=watchers/finalizers,verbs=update
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// evaluate rest config
	clusterClient := &custom.ClusterClient{DefaultClient: r.Client}
	restConfig, err := clusterClient.GetRestConfig(ctx, "kyma-sample", "default", r.RestConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	_, err = manifestLib.NewOperations(&logger, restConfig, "release-name",
		cli.New(), map[string]map[string]interface{}{})

	// TODO: add Watcher control loop logic here
	// TODO: pass kyma name and namespace as chart values to SKR
	// TODO: think about merging logic for all Watcher CRs
	// TODO: implement State handling based on Watcher installation on target cluster
	// TODO: how to access skr-watcher helm charts?

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.RestConfig = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		For(&componentv1alpha1.Watcher{}).
		Complete(r)
}
