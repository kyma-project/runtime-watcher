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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"

	kyma "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	componentv1alpha1 "github.com/kyma-project/kyma-watcher/kcp/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	logger logr.Logger
}

const (
	DefaultOperatorWatcherCRLabel       = "operator.kyma-project.io/default"
	KcpWatcherModulesConfigMapName      = "kcp-watcher-modules" //nolint:gosec
	KcpWatcherModulesConfigMapNamespace = "default"             //nolint:gosec
)

//+kubebuilder:rbac:groups=kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kyma-project.io,resources=kymas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kyma-project.io,resources=kymas/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = log.FromContext(ctx)
	r.logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())
	// check if kyma resource exists
	kymaCR := &kyma.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kymaCR); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info(req.NamespacedName.String() + " got deleted! Reference in WatcherCR ConfigMap will be removed")
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
	// Sync ConfigMap of given modules
	return r.SyncConfigMap(ctx, modules, kymaCR)
}

func (r *KymaReconciler) SyncConfigMap(ctx context.Context, modules []kyma.Module, kymaCR *kyma.Kyma) (ctrl.Result, error) { //nolint:lll
	watcherConfigMap, err := r.getWatcherCM(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, module := range modules {
		watcherCR, err := r.getWatcherCR(ctx, module, kymaCR.Namespace)
		if apierrors.IsNotFound(err) {
			r.logger.Info("No WatcherCR has been found", "module", module)
			continue
		} else if err != nil {
			return ctrl.Result{}, err
		}
		if value, ok := watcherCR.Labels[DefaultOperatorWatcherCRLabel]; ok && strings.ToLower(value) == "true" {
			err = r.updateConfigMap(ctx, watcherConfigMap, module, kymaCR)
			if err != nil {
				return ctrl.Result{}, err
			}
			r.logger.Info("Corresponding ConfigMap of WatcherCR got updated.")
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

func (r *KymaReconciler) getWatcherCM(ctx context.Context) (*v1.ConfigMap, error) {
	watcherConfigMap := &v1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: KcpWatcherModulesConfigMapNamespace,
		Name:      KcpWatcherModulesConfigMapName,
	},
		watcherConfigMap); err != nil {
		return nil, err
	}
	return watcherConfigMap, nil
}

func (r *KymaReconciler) updateConfigMap(ctx context.Context,
	watcherConfigMap *v1.ConfigMap,
	module kyma.Module,
	kymaCR *kyma.Kyma,
) error {
	moduleKey := fmt.Sprintf("%s-%s", module.Name, module.Channel)
	if watcherConfigMap.Data == nil {
		// initialize data map, if map is nil
		watcherConfigMap.Data = make(map[string]string)
	} else {
		// ConfigMap is not empty, check if Module exists in data
		if data, ok := watcherConfigMap.Data[moduleKey]; ok {
			// insert KymaCR if it does not exist
			err := r.insertIntoModule(ctx, moduleKey, data, kymaCR, watcherConfigMap)
			if err != nil {
				return err
			}
			return nil
		}
	}
	// module does not exist in ConfigMap
	err := r.createModuleAndInsertKyma(ctx, moduleKey, kymaCR, watcherConfigMap)
	if err != nil {
		return err
	}
	return nil
}

type WatcherJSONData struct {
	KymaCRList []KymaCREntry `json:"kymaCrList"`
}

type KymaCREntry struct {
	KymaCR        string `json:"kymaCr"`
	KymaNamespace string `json:"kymaNamespace"`
}

func (r *KymaReconciler) insertIntoModule(
	ctx context.Context,
	module, data string,
	kymaCR *kyma.Kyma,
	watcherConfigMap *v1.ConfigMap,
) error {
	// unmarshall json into go struct
	var configMapdata WatcherJSONData
	err := json.Unmarshal([]byte(data), &configMapdata)
	if err != nil {
		return err
	}
	// check if KymaCR already exists in ConfigMap
	for _, kyma := range configMapdata.KymaCRList {
		if kyma.KymaCR == kymaCR.Name && kyma.KymaNamespace == kymaCR.Namespace {
			r.logger.Info(
				fmt.Sprintf(
					"KymaCR `%s` already exists in Watcher-KCP-ConfigMap - Nothing has to be done",
					kymaCR.Name),
			)
			return nil
		}
	}
	// KymaCR does not exist, insert it into ConfigMap
	configMapdata.KymaCRList = append(configMapdata.KymaCRList, KymaCREntry{
		KymaCR:        kymaCR.Name,
		KymaNamespace: kymaCR.Namespace,
	})
	byteString, err := json.Marshal(configMapdata)
	if err != nil {
		return err
	}
	watcherConfigMap.Data[module] = string(byteString)
	// update the ConfigMap on the cluster
	err = r.Update(ctx, watcherConfigMap)
	return err
}

func (r *KymaReconciler) createModuleAndInsertKyma(
	ctx context.Context,
	module string,
	kymaCR *kyma.Kyma,
	watcherConfigMap *v1.ConfigMap,
) error {
	// create KymaCR entry
	configMapdata := WatcherJSONData{
		KymaCRList: []KymaCREntry{
			{KymaCR: kymaCR.Name, KymaNamespace: kymaCR.Namespace},
		},
	}
	byteString, err := json.Marshal(configMapdata)
	if err != nil {
		return err
	}
	// insert entry into ConfigMap data
	watcherConfigMap.Data[module] = string(byteString)
	// update the ConfigMap on the cluster
	err = r.Update(ctx, watcherConfigMap)
	return err
}
