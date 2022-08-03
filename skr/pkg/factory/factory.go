package factory

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const defaultResync = time.Minute * 30

func InformerFactoryWithLabel(client dynamic.Interface, //nolint:ireturn
	mgr ctrl.Manager,
	label,
	value string,
) (dynamicinformer.DynamicSharedInformerFactory, error) {
	// Create informerFactory for each configured label
	informerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(client,
		defaultResync,
		"",
		func(options *metav1.ListOptions) {
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

func InformerFactory(client dynamic.Interface, //nolint:ireturn
	mgr ctrl.Manager,
) (dynamicinformer.DynamicSharedInformerFactory, error) {
	// Create informerFactory for each configured label
	informerFactory := dynamicinformer.NewDynamicSharedInformerFactory(client, defaultResync)
	err := mgr.Add(manager.RunnableFunc(
		func(ctx context.Context) error {
			informerFactory.Start(ctx.Done())
			return nil
		}))
	return informerFactory, err
}
