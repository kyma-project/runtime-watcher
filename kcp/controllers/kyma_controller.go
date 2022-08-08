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
	"errors"
	"fmt"
	kyma "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	componentv1alpha1 "github.com/kyma-project/kyma-watcher/kcp/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// KymaReconciler reconciles a Kyma object.
type KymaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const DefaultOperatorWatcherCRLabel = "operator.kyma-project.io/default"

//+kubebuilder:rbac:groups=kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kyma-project.io,resources=kymas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kyma-project.io,resources=kymas/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())
	// check if kyma resource exists
	kymaCR := &kyma.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kymaCR); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info(req.NamespacedName.String() + " got deleted! Reference in WatcherCR ConfigMap will be removed")
			// TODO: Delete Kyma from the corresponding ConfigMaps - will be implemented in next iteration
			return ctrl.Result{}, nil //nolint:wrapcheck
		}
		return ctrl.Result{}, err //nolint:wrapcheck
	}

	// Kyma resource was created or updated
	// Get installed modules from Kyma CR
	modules := getModulesList(kymaCR)
	if len(modules) == 0 {
		return ctrl.Result{}, errors.New("module list of KymaCR is empty")
	}
	for _, module := range modules {
		watcherCR, err := r.getWatcherCR(ctx, module, kymaCR.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		if _, ok := watcherCR.Labels[DefaultOperatorWatcherCRLabel]; ok {
			watcherConfigMap, err := r.getWatcherCM(ctx, module, kymaCR.Namespace)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.updateConfigMap(ctx, watcherConfigMap, kymaCR)
			if err != nil {
				return ctrl.Result{}, err
			}
			logger.Info("Corresponding ConfigMap of WatcherCR got updated.")
			return ctrl.Result{}, nil
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

func (r *KymaReconciler) getWatcherCR(ctx context.Context,
	module kyma.Module,
	namespace string,
) (*componentv1alpha1.Watcher, error) {
	watcherCRName := fmt.Sprintf("%s-%s", module.Name, module.Channel)
	watcherCR := &componentv1alpha1.Watcher{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: watcherCRName}, watcherCR); err != nil {
		return nil, err
	}
	return watcherCR, nil
}

func (r *KymaReconciler) getWatcherCM(ctx context.Context,
	module kyma.Module,
	namespace string,
) (*v1.ConfigMap, error) {
	watcherConfigMapName := fmt.Sprintf("%s-%s", module.Name, module.Channel)
	watcherConfigMap := &v1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: watcherConfigMapName},
		watcherConfigMap); err != nil {
		return nil, err
	}
	return watcherConfigMap, nil
}

func (r *KymaReconciler) updateConfigMap(ctx context.Context,
	watcherConfigMap *v1.ConfigMap,
	kymaCR *kyma.Kyma,
) error {
	if watcherConfigMap.Data == nil {
		// initialize data map, if map is nil
		watcherConfigMap.Data = make(map[string]string)
	} else {
		for key, value := range watcherConfigMap.Data {
			if key == kymaCR.Name && value == kymaCR.Namespace {
				// Kyma already exists in ConfigMap, nothing has to be done
				return nil
			}
		}
	}
	// Kyma does not exists in ConfigMap
	watcherConfigMap.Data[kymaCR.Name] = kymaCR.Namespace
	err := r.Client.Update(ctx, watcherConfigMap)
	if err != nil {
		return err
	}
	return nil
}
