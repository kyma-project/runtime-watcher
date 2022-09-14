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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/deploy"
	"github.com/kyma-project/runtime-watcher/kcp/pkg/util"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	watcherFinalizer = "operator.kyma-project.io/watcher"
)

// WatcherReconciler reconciles a Watcher object.
type WatcherReconciler struct {
	client.Client
	RestConfig *rest.Config
	Scheme     *runtime.Scheme
	Config     *util.WatcherConfig
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
	watcherObj = watcherObj.DeepCopy()

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
	err := r.createOrUpdateServiceMeshConfigForCR(ctx, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to create or update service mesh config")
	}
	err = r.updateSKRWatcherConfigForCR(ctx, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to update SKR config")
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
	err := r.deleteServiceMeshConfigForCR(ctx, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to delete service mesh config")
	}
	err = r.deleteSKRWatcherConfigForCR(ctx, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to update SKR config")
	}
	updated := controllerutil.RemoveFinalizer(obj, watcherFinalizer)
	if !updated {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to remove finalizer")
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

	logger.Info("checking consistent state for watcher cr")
	err := r.checkConsistentStateForCR(ctx, obj)
	if err != nil {
		logger.Info("resources not yet ready for watcher cr")
		return r.updateWatcherCRStatus(ctx, obj,
			watcherv1alpha1.WatcherStateProcessing, "resources not yet ready")
	}
	logger.Info("watcher cr resources are Ready!")
	return nil
}

func (r *WatcherReconciler) createOrUpdateServiceMeshConfigForCR(ctx context.Context,
	obj *watcherv1alpha1.Watcher,
) error {
	istioClientSet, err := istioclient.NewForConfig(r.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create istio client set from rest config(%s): %w", r.RestConfig.String(), err)
	}
	err = r.updateIstioVirtualServiceForCR(ctx, istioClientSet, obj)
	if err != nil {
		return fmt.Errorf("failed to create and configure Istio VirtualService resource: %w", err)
	}
	return nil
}

func (r *WatcherReconciler) updateIstioVirtualServiceForCR(ctx context.Context,
	istioClientSet *istioclient.Clientset, obj *watcherv1alpha1.Watcher,
) error {
	virtualService, apiErr := istioClientSet.NetworkingV1beta1().
		VirtualServices(r.Config.VirtualServiceNamespace).
		Get(ctx, r.Config.VirtualServiceName, metav1.GetOptions{})
	err := util.IstioResourcesErrorCheck(apiErr)
	if err != nil {
		return err
	}
	if util.IsVirtualServiceConfigChanged(virtualService, obj) {
		util.UpdateVirtualServiceConfig(virtualService, obj)
		_, err = istioClientSet.NetworkingV1beta1().
			VirtualServices(r.Config.VirtualServiceNamespace).
			Update(ctx, virtualService, metav1.UpdateOptions{})
		return err
	}
	return nil
}

func (r *WatcherReconciler) updateSKRWatcherConfigForCR(ctx context.Context, obj *watcherv1alpha1.Watcher) error {
	return deploy.UpdateWebhookConfig(ctx, r.Config.WebhookChartPath, r.Config.WebhookChartReleaseName, obj, r.RestConfig, r.Client)
}

func (r *WatcherReconciler) deleteSKRWatcherConfigForCR(ctx context.Context, obj *watcherv1alpha1.Watcher) error {
	return deploy.RemoveWebhookConfig(ctx, r.Config.WebhookChartPath, r.Config.WebhookChartReleaseName, obj, r.RestConfig, r.Client)
}

func (r *WatcherReconciler) deleteServiceMeshConfigForCR(ctx context.Context, obj *watcherv1alpha1.Watcher) error {
	namespace := obj.GetNamespace()
	vsName := obj.GetName()
	istioClientSet, err := istioclient.NewForConfig(r.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create istio client set from rest config(%s): %w", r.RestConfig.String(), err)
	}
	_, err = istioClientSet.NetworkingV1beta1().VirtualServices(namespace).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get istio virtual service: %w", err)
	}
	if errors.IsNotFound(err) {
		// nothing to do
		return nil
	}
	err = istioClientSet.NetworkingV1beta1().VirtualServices(namespace).Delete(ctx, vsName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete istio virtual service: %w", err)
	}
	return nil
}

func (r *WatcherReconciler) updateWatcherCRStatus(ctx context.Context, obj *watcherv1alpha1.Watcher,
	state watcherv1alpha1.WatcherState, msg string,
) error {
	obj.Status.State = state
	switch state { //nolint:exhaustive
	case watcherv1alpha1.WatcherStateReady:
		util.AddReadyCondition(obj, watcherv1alpha1.ConditionStatusTrue, msg)
	case "":
		util.AddReadyCondition(obj, watcherv1alpha1.ConditionStatusUnknown, msg)
	default:
		util.AddReadyCondition(obj, watcherv1alpha1.ConditionStatusFalse, msg)
	}
	return r.Status().Update(ctx, obj.SetObservedGeneration())
}

func (r *WatcherReconciler) updateWatcherCRErrStatus(ctx context.Context, logger logr.Logger, err error,
	obj *watcherv1alpha1.Watcher, errMsg string,
) error {
	logger.Error(err, errMsg)
	apiErr := r.updateWatcherCRStatus(ctx, obj, watcherv1alpha1.WatcherStateError, errMsg)
	if apiErr != nil {
		logger.Error(apiErr, "update request to API server failed")
		return apiErr
	}
	return err
}

func (r *WatcherReconciler) checkConsistentStateForCR(ctx context.Context,
	obj *watcherv1alpha1.Watcher,
) error {
	istioClientSet, err := istioclient.NewForConfig(r.RestConfig)
	if err != nil {
		return err
	}
	return util.PerformIstioVirtualServiceCheck(ctx, istioClientSet, obj,
		r.Config.VirtualServiceName, r.Config.VirtualServiceNamespace)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.RestConfig = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		For(&watcherv1alpha1.Watcher{}).
		Complete(r)
}
