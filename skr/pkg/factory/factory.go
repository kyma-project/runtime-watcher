package factory

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"time"
)

func InformerFactoryWithLabel(client dynamic.Interface, mgr ctrl.Manager, label, value string) (dynamicinformer.DynamicSharedInformerFactory, error) {
	// Create informerFactory for each configured label
	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, time.Minute*30, "", func(options *metav1.ListOptions) {
		labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{label: value}}
		options.LabelSelector = labels.Set(labelSelector.MatchLabels).String()
	})
	err := mgr.Add(manager.RunnableFunc(
		func(ctx context.Context) error {
			informerFactory.Start(ctx.Done())
			return nil
		}))
	return informerFactory, err
}

func BuildInformerSet(gv schema.GroupVersion, resources *metav1.APIResourceList, informerFactory dynamicinformer.DynamicSharedInformerFactory) map[string]*source.Informer {
	dynamicInformerSet := make(map[string]*source.Informer)
	for _, resource := range resources.APIResources {
		if strings.Contains(resource.Name, "/") || !strings.Contains(resource.Verbs.String(), "watch") {
			// Skip not listable resources, i.e. nodes/proxy
			continue
		}
		// TODO have propper logging in place
		//r.Logger.Info(fmt.Sprintf("Resource `%s` from GroupVersion `%s` will be watched", resource.Name, gv.String()))
		gvr := gv.WithResource(resource.Name)
		dynamicInformerSet[gvr.String()] = &source.Informer{Informer: informerFactory.ForResource(gvr).Informer()}
	}
	return dynamicInformerSet
}

func GetResourceList(mgr ctrl.Manager, gvs []schema.GroupVersion) (map[schema.GroupVersion]*metav1.APIResourceList, error) {
	// Create K8s-Client
	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}
	// Get Resources for configured GV
	var resourcesMap = map[schema.GroupVersion]*metav1.APIResourceList{}
	for _, gv := range gvs {
		resources, err := cs.ServerResourcesForGroupVersion(gv.String())
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}
		// Resources found
		if err == nil {
			resourcesMap[gv] = resources
		}
	}
	return resourcesMap, nil
}
