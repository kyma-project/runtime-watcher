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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/custom"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/deploy"
)

const (
	watcherFinalizer = "operator.kyma-project.io/watcher"
)

// WatcherReconciler reconciles a Watcher object.
type WatcherReconciler struct {
	client.Client
	*custom.IstioClient
	RestConfig *rest.Config
	Scheme     *runtime.Scheme
	Config     *WatcherConfig
}

type WatcherConfig struct {
	// VirtualServiceObjKey represents the object key (name and namespace) of the virtual service resource to be updated
	VirtualServiceObjKey client.ObjectKey
	// RequeueInterval represents requeue interval in seconds
	RequeueInterval int
	// WebhookChartPath represents the path of the webhook chart
	// to be installed on SKR clusters upon reconciling watcher CRs
	WebhookChartPath string
	// WebhookChartReleaseName represents the helm release name of the webhook chart
	// to be installed on SKR clusters upon reconciling watcher CRs
	WebhookChartReleaseName string
}

//nolint:lll
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=watchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=watchers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=watchers/finalizers,verbs=update
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.istio.io,resources=gateways,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
//nolint:lll
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName(req.NamespacedName.String())
	logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())

	watcherObj := &watcherv1alpha1.Watcher{}
	err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, watcherObj)
	if err != nil {
		logger.Info(fmt.Sprintf("failed to get reconciliation object: %s", req.NamespacedName.String()))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !watcherObj.DeletionTimestamp.IsZero() && watcherObj.Status.State != watcherv1alpha1.WatcherStateDeleting {
		// if the status is not yet set to deleting, also update the status
		return ctrl.Result{}, r.updateWatcherCRStatus(ctx, watcherObj, watcherv1alpha1.WatcherStateDeleting, "deletion timestamp set")
	}

	// check finalizer on native object
	if !controllerutil.ContainsFinalizer(watcherObj, watcherFinalizer) {
		controllerutil.AddFinalizer(watcherObj, watcherFinalizer)
		return ctrl.Result{}, r.Update(ctx, watcherObj)
	}

	requeueInterval := time.Duration(r.Config.RequeueInterval) * time.Second

	// state handling
	switch watcherObj.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, watcherObj)
	case watcherv1alpha1.WatcherStateProcessing:
		return ctrl.Result{RequeueAfter: requeueInterval},
			r.HandleProcessingState(ctx, logger, watcherObj)
	case watcherv1alpha1.WatcherStateDeleting:
		return ctrl.Result{}, r.HandleDeletingState(ctx, logger, watcherObj)
	case watcherv1alpha1.WatcherStateError:
		return ctrl.Result{RequeueAfter: requeueInterval},
			r.HandleErrorState(ctx, watcherObj)
	case watcherv1alpha1.WatcherStateReady:
		return ctrl.Result{RequeueAfter: requeueInterval},
			r.HandleReadyState(ctx, logger, watcherObj)
	}

	return ctrl.Result{}, nil
}

func (r *WatcherReconciler) HandleInitialState(ctx context.Context, obj *watcherv1alpha1.Watcher) error {
	return r.updateWatcherCRStatus(ctx, obj, watcherv1alpha1.WatcherStateProcessing, "watcher cr created")
}

func (r *WatcherReconciler) HandleProcessingState(ctx context.Context,
	logger logr.Logger, obj *watcherv1alpha1.Watcher,
) error {

	err := r.UpdateVirtualServiceConfig(ctx, r.Config.VirtualServiceObjKey, obj)
	if err != nil {
		return r.updateWatcherCRStatus(ctx, obj, watcherv1alpha1.WatcherStateError,
			"failed to create or update service mesh config")
	}
	err = deploy.UpdateWebhookConfig(ctx, r.Config.WebhookChartPath, r.Config.WebhookChartReleaseName, obj,
		r.RestConfig, r.Client)
	if err != nil {
		return r.updateWatcherCRStatus(ctx, obj, watcherv1alpha1.WatcherStateError, "failed to update SKR config")
	}
	err = r.updateWatcherCRStatus(ctx, obj, watcherv1alpha1.WatcherStateReady, "successfully reconciled watcher cr")
	if err != nil {
		logger.Error(err, "failed to update watcher cr to ready status")
	}
	logger.Info("watcher cr is Ready!")
	return nil
}

func (r *WatcherReconciler) HandleDeletingState(ctx context.Context, logger logr.Logger,
	obj *watcherv1alpha1.Watcher,
) error {
	err := r.RemoveVirtualServiceConfigForCR(ctx, r.Config.VirtualServiceObjKey, obj)
	if err != nil {
		return r.updateWatcherCRStatus(ctx, obj, watcherv1alpha1.WatcherStateError,
			"failed to delete service mesh config")
	}
	err = deploy.RemoveWebhookConfig(ctx, r.Config.WebhookChartPath, r.Config.WebhookChartReleaseName, obj,
		r.RestConfig, r.Client)
	if err != nil {
		return r.updateWatcherCRStatus(ctx, obj, watcherv1alpha1.WatcherStateError, "failed to delete SKR config")
	}
	updated := controllerutil.RemoveFinalizer(obj, watcherFinalizer)
	if !updated {
		return r.updateWatcherCRStatus(ctx, obj, watcherv1alpha1.WatcherStateError, "failed to remove finalizer")
	}
	err = r.Update(ctx, obj)
	if err != nil {
		logger.Error(err, "failed to update watcher cr")
	}
	logger.Info("deletion state handling was successful")
	return nil
}

func (r *WatcherReconciler) HandleErrorState(ctx context.Context, obj *watcherv1alpha1.Watcher) error {
	return r.updateWatcherCRStatus(ctx, obj, watcherv1alpha1.WatcherStateProcessing, "observed generation change")
}

func (r *WatcherReconciler) HandleReadyState(ctx context.Context, logger logr.Logger,
	obj *watcherv1alpha1.Watcher,
) error {
	if obj.Generation != obj.Status.ObservedGeneration {
		logger.Info("observed generation change for watcher cr")
		return r.updateWatcherCRStatus(ctx, obj,
			watcherv1alpha1.WatcherStateProcessing, "observed generation change")
	}

	return nil
}

func (r *WatcherReconciler) updateWatcherCRStatus(ctx context.Context, obj *watcherv1alpha1.Watcher,
	state watcherv1alpha1.WatcherState, msg string,
) error {
	obj.Status.State = state
	switch state { //nolint:exhaustive
	case watcherv1alpha1.WatcherStateReady:
		obj.AddOrUpdateReadyCondition(watcherv1alpha1.ConditionStatusTrue, msg)
	case "":
		obj.AddOrUpdateReadyCondition(watcherv1alpha1.ConditionStatusUnknown, msg)
	default:
		obj.AddOrUpdateReadyCondition(watcherv1alpha1.ConditionStatusFalse, msg)
	}
	return r.Status().Update(ctx, obj.SetObservedGeneration())
}

func (r *WatcherReconciler) SetIstioClient() error {
	if r.RestConfig == nil {
		return fmt.Errorf("reconciler rest config is not set")
	}
	customIstioClient, err := custom.NewVersionedIstioClient(r.RestConfig)
	r.IstioClient = customIstioClient
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&watcherv1alpha1.Watcher{}).
		Complete(r)
}
