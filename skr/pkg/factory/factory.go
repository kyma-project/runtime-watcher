package factory

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	ctrl "sigs.k8s.io/controller-runtime"
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

func InformerFactory(client dynamic.Interface, mgr ctrl.Manager) (dynamicinformer.DynamicSharedInformerFactory, error) {
	// Create informerFactory for each configured label
	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(client, time.Minute*30)
	err := mgr.Add(manager.RunnableFunc(
		func(ctx context.Context) error {
			informerFactory.Start(ctx.Done())
			return nil
		}))
	return informerFactory, err
}

func BuildInformerSet(gvrList []schema.GroupVersionResource, informerFactory dynamicinformer.DynamicSharedInformerFactory) map[string]*source.Informer {
	dynamicInformerSet := make(map[string]*source.Informer)
	for _, gvr := range gvrList {
		if strings.Contains(gvr.Resource, "/") {
			// Skip not listable resources, i.e. nodes/proxy
			continue
		}
		dynamicInformerSet[gvr.String()] = &source.Informer{Informer: informerFactory.ForResource(gvr).Informer()}
	}
	return dynamicInformerSet
}
