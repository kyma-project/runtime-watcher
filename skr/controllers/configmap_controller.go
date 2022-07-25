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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/kyma-project/kyma-watcher/kcp/pkg/types"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyma-project/kyma-watcher/skr/pkg/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type EventType string

// ConfigMapReconciler reconciles a ConfigMap object
type ConfigMapReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Logger  logr.Logger
	KcpIp   string
	KcpPort string
}

//+kubebuilder:rbac:groups="*",resources="*",verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the ConfigMap object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// logger := log.FromContext(ctx).WithName(req.NamespacedName.String())
	// Should do nothing
	return ctrl.Result{}, nil
}

func (r *ConfigMapReconciler) CreateFunc(e event.CreateEvent, q workqueue.RateLimitingInterface) {
	r.Logger.Info(fmt.Sprintf("Create Event: %s", e.Object.GetName()))
	_, err := r.sendRequest(e)
	if err != nil {
		r.Logger.Error(err, "Error occured while sending request")
		return
	}

}

func (r *ConfigMapReconciler) UpdateFunc(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	r.Logger.Info(fmt.Sprintf("Update Event: %s", e.ObjectNew.GetName()))
	_, err := r.sendRequest(e)
	if err != nil {
		r.Logger.Error(err, "Error occured while sending request")
		return
	}
}

func (r *ConfigMapReconciler) DeleteFunc(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	r.Logger.Info(fmt.Sprintf("Delete Event: %s", e.Object.GetName()))
	_, err := r.sendRequest(e)
	if err != nil {
		r.Logger.Error(err, "Error occured while sending request")
		return
	}
}

func (r *ConfigMapReconciler) GenericFunc(e event.GenericEvent, q workqueue.RateLimitingInterface) {
	r.Logger.Info(fmt.Sprintf("Generic Event: %s", e.Object.GetName()))
	_, err := r.sendRequest(e)
	if err != nil {
		r.Logger.Error(err, "Error occured while sending request")
		return
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Create ControllerBuilder
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).For(&v1.ConfigMap{})

	// Create Dynamic InformerFactory
	c, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	informers := dynamicinformer.NewFilteredDynamicSharedInformerFactory(c, time.Minute*30, "", func(options *metav1.ListOptions) {
		labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{"operator.kyma-project.io/managed-by": "Kyma"}}
		options.LabelSelector = labels.Set(labelSelector.MatchLabels).String() // TODO: Check if it is possible to select from a given set (with OR condition)

	})
	err = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		informers.Start(ctx.Done())
		return nil
	}))
	if err != nil {
		return err
	}

	// Create K8s-Client
	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	for _, gv := range config.Gvs {
		resources, err := cs.ServerResourcesForGroupVersion(gv.String())
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		// resources found
		if err == nil {
			dynamicInformerSet := make(map[string]*source.Informer)
			for _, resource := range resources.APIResources {
				if strings.Contains(resource.Name, "/") || !strings.Contains(resource.Verbs.String(), "watch") {
					// Skip no listable resources, i.e. nodes/proxy
					continue
				}
				r.Logger.Info(fmt.Sprintf("Resource `%s` from GroupVersion `%s` will be watched", resource.Name, gv))
				gvr := gv.WithResource(resource.Name)
				dynamicInformerSet[gvr.String()] = &source.Informer{Informer: informers.ForResource(gvr).Informer()}
			}

			for gvr, informer := range dynamicInformerSet {
				controllerBuilder = controllerBuilder.
					Watches(informer, &handler.Funcs{
						CreateFunc:  r.CreateFunc,
						UpdateFunc:  r.UpdateFunc,
						DeleteFunc:  r.DeleteFunc,
						GenericFunc: r.GenericFunc,
					},
						builder.WithPredicates(predicate.GenerationChangedPredicate{}))
				r.Logger.Info("initialized dynamic watching", "source", gvr)
			}
		}
	}

	return controllerBuilder.Complete(r)
}

func (r *ConfigMapReconciler) sendRequest(newEvent interface{}) (string, error) {
	var component string
	var kymaCr string
	var namespace string
	var name string
	var object client.Object

	switch newEvent.(type) {
	case event.CreateEvent:
		object = newEvent.(event.CreateEvent).Object
	case event.UpdateEvent:
		object = newEvent.(event.UpdateEvent).ObjectNew
	case event.DeleteEvent:
		object = newEvent.(event.DeleteEvent).Object
	case event.GenericEvent:
		object = newEvent.(event.GenericEvent).Object
	default:
		r.Logger.Info(fmt.Sprintf("Undefined eventType: %#v", newEvent))
	}

	component = r.getValueFromLabel(config.ComponentLabel, object)
	kymaCr = r.getValueFromLabel(config.KymaCrLabel, object)
	namespace = object.GetNamespace()
	name = object.GetName()

	watcherEvent := &types.WatcherEvent{
		KymaCr:    kymaCr,
		Namespace: namespace,
		Name:      name,
	}
	postBody, _ := json.Marshal(watcherEvent)

	responseBody := bytes.NewBuffer(postBody)
	url := fmt.Sprintf("%s:%s/%s/%s/%s", r.KcpIp, r.KcpPort, config.ContractVersion, component, config.EventEndpoint)
	resp, err := http.Post(url, "application/json", responseBody)
	//Handle Error
	if err != nil {
		r.Logger.Info(fmt.Sprintf("Error POST %#v", err))
		return "", err
	}
	defer resp.Body.Close()
	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	sb := string(body)
	r.Logger.Info("Request to KCP successfully!")
	return sb, nil
}

func (r *ConfigMapReconciler) getValueFromLabel(label string, object client.Object) string {
	labels := object.GetLabels()
	value, ok := labels[label]
	if ok {
		r.Logger.Info(fmt.Sprintf("Value of `%s` new Event: %s", label, value))
		return value
	}
	r.Logger.Info(fmt.Sprintf("Label `%s` not found", label))
	return ""
}
