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
	"fmt"
	kyma "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	componentv1alpha1 "github.com/kyma-project/kyma-watcher/kcp/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// KymaReconciler reconciles a Kyma object
type KymaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const defaultOperatorWatcherCRLabel = "operator.kyma-project.io/default"

//+kubebuilder:rbac:groups=kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kyma-project.io,resources=kymas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kyma-project.io,resources=kymas/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Kyma object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())

	// check if kyma resource exists
	kymaCR := &kyma.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kymaCR); err != nil {
		if errors.IsNotFound(err) {
			logger.Info(req.NamespacedName.String() + " got deleted!")
			// TODO: Delete Kyma from the corresponding ConfigMaps
			return ctrl.Result{}, nil //nolint:wrapcheck
		}
		return ctrl.Result{}, err //nolint:wrapcheck
	}

	// Kyma resource was created or updated
	// Get installed modules fomr Kyma CR
	modules := getModulesList(kymaCR)
	if len(modules) == 0 {
		// TODO return error
	}
	for _, module := range modules {
		watcherCR, err := r.getWatcherCR(ctx, module, kymaCR.Namespace)
		if err != nil {
			// Corresponding WatcherCR could not be found
			//TODO
		}
		if _, ok := watcherCR.Labels[defaultOperatorWatcherCRLabel]; ok == true {
			watcherConfigMap, err := r.getWatcherCM(module)
			if err != nil {
				// TODO log error that configmap was not found
			}
			err = updateConfigMap(watcherConfigMap, kymaCR)
		} else {
			// Nothing has to be done
		}

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager) error {

	SchemeBuilder := &scheme.Builder{GroupVersion: kyma.GroupVersion}
	SchemeBuilder.Register(&kyma.Kyma{})
	// Create ControllerBuilder
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		For(
			&kyma.Kyma{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		)

	return controllerBuilder.Complete(r)
}

func getModulesList(kymaResource *kyma.Kyma) []kyma.Module {
	modules := kymaResource.Spec.Modules
	return modules
}

func (r *KymaReconciler) getWatcherCR(ctx context.Context, module kyma.Module, namespace string) (*componentv1alpha1.Watcher, error) {
	watcherCRName := fmt.Sprintf("%s-%s", module.Name, module.Channel)
	watcherCR := &componentv1alpha1.Watcher{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: watcherCRName}, watcherCR); err != nil {
		return nil, err
	}
	return watcherCR, nil
}

func (r *KymaReconciler) getWatcherCM(module string) (*v1.ConfigMap, error) {
	// TODO implement
	return nil, nil
}

func updateConfigMap(watcherConfigMap *v1.ConfigMap, kymaCR *kyma.Kyma) error {
	return nil
}
