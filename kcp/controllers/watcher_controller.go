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

	componentv1alpha1 "github.com/kyma-project/kyma-watcher/kcp/api/v1alpha1"
	"github.com/kyma-project/kyma-watcher/kcp/pkg/util"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	istioGatewayResourceName = "kcp-listener-gw"
	// defaultOperatorWatcherCRLabel is a label indicating that watcher CR applies to all Kymas
	defaultOperatorWatcherCRLabel = "operator.kyma-project.io/default"
	watcherFinalizer              = "component.kyma-project.io/watcher"
	istioGatewayGVR               = "gateways.networking.istio.io/v1beta1"
	istioVirtualServiceGVR        = "virtualservices.networking.istio.io/v1beta1"
)

// WatcherReconciler reconciles a Watcher object
type WatcherReconciler struct {
	client.Client
	RestConfig *rest.Config
	Scheme     *runtime.Scheme
	Config     *util.WatcherConfig
}

//nolint:lll
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=watchers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=watchers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=watchers/finalizers,verbs=update
func (r *WatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName(req.NamespacedName.String())
	logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())

	watcherObj := &componentv1alpha1.Watcher{}
	err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, watcherObj)
	if err != nil {

		logger.Info(fmt.Sprintf("%s got deleted", req.NamespacedName.String()))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	watcherObj = watcherObj.DeepCopy()

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !watcherObj.DeletionTimestamp.IsZero() && watcherObj.Status.State != componentv1alpha1.WatcherStateDeleting {
		// if the status is not yet set to deleting, also update the status
		return ctrl.Result{}, r.updateWatcherCRStatus(ctx, watcherObj, componentv1alpha1.WatcherStateDeleting, "deletion timestamp set")
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
		return ctrl.Result{}, r.HandleInitialState(ctx, logger, watcherObj)
	case componentv1alpha1.WatcherStateProcessing:
		return ctrl.Result{RequeueAfter: requeueInterval},
			r.HandleProcessingState(ctx, logger, watcherObj)
	case componentv1alpha1.WatcherStateDeleting:
		return ctrl.Result{}, r.HandleDeletingState(ctx, logger, watcherObj)
	case componentv1alpha1.WatcherStateError:
		return ctrl.Result{RequeueAfter: requeueInterval},
			r.HandleErrorState(ctx, logger, watcherObj)
	case componentv1alpha1.WatcherStateReady:
		return ctrl.Result{RequeueAfter: requeueInterval},
			r.HandleReadyState(ctx, logger, watcherObj)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.RestConfig = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		For(&componentv1alpha1.Watcher{}).
		Complete(r)
}

func (r *WatcherReconciler) HandleInitialState(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	return r.updateWatcherCRStatus(ctx, obj, componentv1alpha1.WatcherStateProcessing, "watcher cr created")
}

func (r *WatcherReconciler) HandleProcessingState(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	err := r.createConfigMapForCR(ctx, logger, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to create config map for watcher cr")
	}
	err = r.createOrUpdateServiceMeshConfigForCR(ctx, logger, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to create or update service mesh config for watcher cr")
	}
	err = r.updateSKRWatcherConfigForCR(ctx, logger, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to update SKR config for watcher cr")
	}
	err = r.updateWatcherCRStatus(ctx, obj, componentv1alpha1.WatcherStateReady, "successfully reconciled watcher cr")
	if err != nil {
		logger.Error(err, "failed to update watcher cr to ready status")
	}
	logger.Info("watcher cr is Ready!")
	return nil
}

func (r *WatcherReconciler) updateWatcherCRErrStatus(ctx context.Context, logger logr.Logger, err error, obj *componentv1alpha1.Watcher, errMsg string) error {
	logger.Error(err, errMsg)
	apiErr := r.updateWatcherCRStatus(ctx, obj, componentv1alpha1.WatcherStateError, errMsg)
	if apiErr != nil {
		logger.Error(apiErr, "update request to API server failed")
		return apiErr
	}
	return err
}

func (r *WatcherReconciler) updateSKRWatcherConfigForCR(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	//this will be implemented as part of another step: see https://github.com/kyma-project/kyma-watcher/issues/33
	return nil
}

func (r *WatcherReconciler) createOrUpdateServiceMeshConfigForCR(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	namespace := obj.GetNamespace()
	istioClientSet, err := istioclient.NewForConfig(r.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create istio client set from rest config(%s): %w", r.RestConfig.String(), err)
	}
	err = r.createOrUpdateIstioGwForCR(ctx, istioClientSet, namespace)
	if err != nil {
		return fmt.Errorf("failed to create and configure Istio Gateway resource: %w", err)
	}
	err = r.createOrUpdateIstioVirtualServiceForCR(ctx, istioClientSet, obj)
	if err != nil {
		return fmt.Errorf("failed to create and configure Istio VirtualService resource: %w", err)
	}
	return nil
}

func (r *WatcherReconciler) createOrUpdateIstioGwForCR(ctx context.Context, istioClientSet *istioclient.Clientset, namespace string) error {
	listenerGateway, apiErr := istioClientSet.NetworkingV1beta1().Gateways(namespace).Get(ctx, istioGatewayResourceName, metav1.GetOptions{})
	ready, err := util.IstioReourcesErrorCheck(istioGatewayGVR, apiErr)
	if !ready {
		return err
	}

	if errors.IsNotFound(apiErr) {
		//create gateway with config from CR
		gw := &istioclientapiv1beta1.Gateway{}
		gw.SetName(istioGatewayResourceName)
		gw.SetNamespace(namespace)
		util.UpdateIstioGWConfig(gw, r.Config.ListenerIstioGatewayPort)
		_, apiErr = istioClientSet.NetworkingV1beta1().Gateways(namespace).Create(ctx, gw, metav1.CreateOptions{})
		if apiErr != nil {
			return apiErr
		}
		return nil
	}

	if util.IsGWConfigChanged(listenerGateway, r.Config.ListenerIstioGatewayPort) {
		util.UpdateIstioGWConfig(listenerGateway, r.Config.ListenerIstioGatewayPort)
		_, apiErr = istioClientSet.NetworkingV1beta1().Gateways(namespace).Update(ctx, listenerGateway, metav1.UpdateOptions{})
		if apiErr != nil {
			return apiErr
		}
	}

	return nil
}

func (r *WatcherReconciler) createOrUpdateIstioVirtualServiceForCR(ctx context.Context, istioClientSet *istioclient.Clientset, obj *componentv1alpha1.Watcher) error {
	namespace := obj.GetNamespace()
	vsName := obj.GetName()
	listenerVirtualService, apiErr := istioClientSet.NetworkingV1beta1().VirtualServices(namespace).Get(ctx, vsName, metav1.GetOptions{})
	ready, err := util.IstioReourcesErrorCheck(istioGatewayGVR, apiErr)
	if !ready {
		return err
	}
	if errors.IsNotFound(apiErr) {
		//create virtual service with config from CR
		vs := &istioclientapiv1beta1.VirtualService{}
		vs.SetName(vsName)
		vs.SetNamespace(namespace)
		util.UpdateVirtualServiceConfig(vs, obj, istioGatewayResourceName)
		_, err := istioClientSet.NetworkingV1beta1().VirtualServices(namespace).Create(ctx, vs, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		return nil
	}
	//check if config already exists
	if util.IsVirtualServiceConfigChanged(listenerVirtualService, obj, istioGatewayResourceName) {
		util.UpdateVirtualServiceConfig(listenerVirtualService, obj, istioGatewayResourceName)
		_, err = istioClientSet.NetworkingV1beta1().VirtualServices(namespace).Update(ctx, listenerVirtualService, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *WatcherReconciler) createConfigMapForCR(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	watcherCRLabels := obj.GetLabels()
	_, ok := watcherCRLabels[defaultOperatorWatcherCRLabel]
	if ok {
		//watcher CR belongs to a KCP operator which applies on all Kymas (e.g. kyma-operator, manifest operator...). So no need to create configMap for it!
		return nil
	}
	//create empty config map for CR
	watcherObjKey := client.ObjectKeyFromObject(obj)
	cm := &v1.ConfigMap{}
	err := r.Get(ctx, watcherObjKey, cm)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to send get config map request to API server: %w", err)
	}
	if errors.IsNotFound(err) {
		cm.SetName(watcherObjKey.Name)
		cm.SetNamespace(watcherObjKey.Namespace)
		err = r.Create(ctx, cm)
		if err != nil {
			return fmt.Errorf("failed to send create config map request to API server: %w", err)
		}
	}
	return nil
}

func (r *WatcherReconciler) updateWatcherCRStatus(ctx context.Context, obj *componentv1alpha1.Watcher, state componentv1alpha1.WatcherState, msg string) error {
	obj.Status.State = state
	switch state {
	case componentv1alpha1.WatcherStateReady:
		util.AddReadyCondition(obj, componentv1alpha1.ConditionStatusTrue, msg)
	case "":
		util.AddReadyCondition(obj, componentv1alpha1.ConditionStatusUnknown, msg)
	default:
		util.AddReadyCondition(obj, componentv1alpha1.ConditionStatusFalse, msg)
	}
	return r.Status().Update(ctx, obj.SetObservedGeneration())
}

func (r *WatcherReconciler) HandleDeletingState(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	err := r.deleteConfigMapForCR(ctx, logger, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to create config map for watcher cr")
	}
	err = r.deleteServiceMeshConfigForCR(ctx, logger, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to create or update service mesh config for watcher cr")
	}
	err = r.deleteSKRWatcherConfigForCR(ctx, logger, obj)
	if err != nil {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to update SKR config for watcher cr")
	}
	updated := controllerutil.RemoveFinalizer(obj, watcherFinalizer)
	if !updated {
		return r.updateWatcherCRErrStatus(ctx, logger, err, obj, "failed to remove finalizer from watcher cr")
	}
	err = r.Update(ctx, obj)
	if err != nil {
		logger.Error(err, "failed to update watcher cr")
	}
	logger.Info("deletion state handling was successful")
	return nil
}

func (r *WatcherReconciler) deleteSKRWatcherConfigForCR(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	//this will be implemented as part of another step: see https://github.com/kyma-project/kyma-watcher/issues/33
	return nil
}

func (r *WatcherReconciler) deleteServiceMeshConfigForCR(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	namespace := obj.GetNamespace()
	vsName := obj.GetName()
	ic, err := istioclient.NewForConfig(r.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create istio client set from rest config(%s): %w", r.RestConfig.String(), err)
	}
	_, err = ic.NetworkingV1beta1().VirtualServices(namespace).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get istio virtual service: %w", err)
	}
	if errors.IsNotFound(err) {
		//nothing to do
		return nil
	}
	err = ic.NetworkingV1beta1().VirtualServices(namespace).Delete(ctx, vsName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete istio virtual service: %w", err)
	}
	return nil

}

func (r *WatcherReconciler) deleteConfigMapForCR(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	watcherCRLabels := obj.GetLabels()
	_, ok := watcherCRLabels[defaultOperatorWatcherCRLabel]
	if ok {
		//watcher CR belongs to a KCP operator which applies on all Kymas (e.g. kyma-operator, manifest operator...). So no need to create configMap for it!
		return nil
	}
	//delete config map for CR
	watcherObjKey := client.ObjectKeyFromObject(obj)
	cm := &v1.ConfigMap{}
	err := r.Get(ctx, watcherObjKey, cm)
	if err != nil {
		return fmt.Errorf("failed to send get config map request to API server: %w", err)
	}
	err = r.Delete(ctx, cm)
	if err != nil {
		return fmt.Errorf("failed to delete config map: %w", err)
	}
	return nil
}

func (r *WatcherReconciler) HandleErrorState(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	return r.updateWatcherCRStatus(ctx, obj, componentv1alpha1.WatcherStateProcessing, "observed generation change")
}

func (r *WatcherReconciler) HandleReadyState(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) error {
	if obj.Generation != obj.Status.ObservedGeneration {
		logger.Info("observed generation change for watcher cr")
		return r.updateWatcherCRStatus(ctx, obj, componentv1alpha1.WatcherStateProcessing, "observed generation change")
	}

	logger.Info("checking consistent state for watcher cr")
	ready, err := r.checkConsistentStateForCR(ctx, logger, obj)
	if err != nil {
		logger.Info("failed while checking resources for watcher cr")
		return r.updateWatcherCRStatus(ctx, obj, componentv1alpha1.WatcherStateError, "failed while checking resources")
	}
	if !ready {
		logger.Info("resources not yet ready for watcher cr")
		return r.updateWatcherCRStatus(ctx, obj, componentv1alpha1.WatcherStateProcessing, "resources not yet ready")
	}
	logger.Info("watcher cr resources are Ready!")
	return nil
}

func (r *WatcherReconciler) checkConsistentStateForCR(ctx context.Context, logger logr.Logger, obj *componentv1alpha1.Watcher) (bool, error) {
	//1.step: config map check
	watcherCRLabels := obj.GetLabels()
	_, ok := watcherCRLabels[defaultOperatorWatcherCRLabel]
	if !ok {
		watcherObjKey := client.ObjectKeyFromObject(obj)
		cm := &v1.ConfigMap{}
		err := r.Get(ctx, watcherObjKey, cm)
		if err != nil && !errors.IsNotFound(err) {
			return false, fmt.Errorf("failed to send get config map request to API server: %w", err)
		}
		if errors.IsNotFound(err) {
			return false, nil
		}
	}
	//2.step: istio GW check
	namespace := obj.GetNamespace()
	ic, err := istioclient.NewForConfig(r.RestConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create istio client set from rest config(%s): %w", r.RestConfig.String(), err)
	}
	gw, apiErr := ic.NetworkingV1beta1().Gateways(namespace).Get(ctx, istioGatewayResourceName, metav1.GetOptions{})
	ready, err := util.IstioReourcesErrorCheck(istioGatewayGVR, apiErr)
	if !ready {
		return false, err
	}
	if errors.IsNotFound(apiErr) {
		return false, nil
	}
	if util.IsGWConfigChanged(gw, r.Config.ListenerIstioGatewayPort) {
		//CR config changed, resources not ready!
		return false, nil
	}
	//3.step: istio VirtualService check
	vsName := obj.GetName()
	virtualService, apiErr := ic.NetworkingV1beta1().VirtualServices(namespace).Get(ctx, vsName, metav1.GetOptions{})
	ready, err = util.IstioReourcesErrorCheck(istioVirtualServiceGVR, apiErr)
	if !ready {
		return false, err
	}
	if errors.IsNotFound(apiErr) {
		return false, nil
	}
	if util.IsVirtualServiceConfigChanged(virtualService, obj, istioGatewayResourceName) {
		//CR config changed, resources not ready!
		return false, nil
	}
	//4.step: SKR watcher config check
	//this will be implemented as part of another step: see https://github.com/kyma-project/kyma-watcher/issues/33
	return true, nil
}
