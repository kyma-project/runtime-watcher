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
	kyma "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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
	kyma := &kyma.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if errors.IsNotFound(err) {
			logger.Info(req.NamespacedName.String() + " got deleted!")
			// TODO: Delete Kyma from the corresponding ConfigMaps
			return ctrl.Result{}, nil //nolint:wrapcheck
		}
		return ctrl.Result{}, err //nolint:wrapcheck
	}

	// Kyma resource was created or updated

	kyma.Status.Conditions

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
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)

	return controllerBuilder.Complete(r)
}
