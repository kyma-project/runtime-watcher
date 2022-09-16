package util

import (
	"context"
	"fmt"

	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	firstElementIdx                = 0
	IstioVirtualServiceGVR         = "virtualservices.networking.istio.io/v1beta1"
	ManagedBylabel                 = "operator.kyma-project.io/managed-by"
	contractVersion                = "1"
	DefaultVirtualServiceName      = "kcp-events"
	DefaultVirtualServiceNamespace = metav1.NamespaceDefault
	DefaultWebhookChartReleaseName = "watcher"
)

func isRouteConfigEqual(route1 *istioapiv1beta1.HTTPRoute, route2 *istioapiv1beta1.HTTPRoute) bool {
	if route1.Match[firstElementIdx].Uri.MatchType.(*istioapiv1beta1.StringMatch_Prefix).Prefix != //nolint:nosnakecase
		route2.Match[firstElementIdx].Uri.MatchType.(*istioapiv1beta1.StringMatch_Prefix).Prefix { //nolint:nosnakecase
		return false
	}

	if route1.Route[firstElementIdx].Destination.Host !=
		route2.Route[firstElementIdx].Destination.Host {
		return false
	}

	if route1.Route[firstElementIdx].Destination.Port.Number !=
		route2.Route[firstElementIdx].Destination.Port.Number {
		return false
	}

	return true
}

func IsListenerHTTPRouteConfigured(virtualService *istioclientapiv1beta1.VirtualService,
	obj *watcherv1alpha1.Watcher,
) bool {
	if len(virtualService.Spec.Http) == 0 {
		return false
	}

	for idx, route := range virtualService.Spec.Http {
		if route.Name == obj.Labels[ManagedBylabel] {
			istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
			return isRouteConfigEqual(virtualService.Spec.Http[idx], istioHTTPRoute)
		}
	}

	return false
}

func UpdateVirtualServiceConfig(virtualService *istioclientapiv1beta1.VirtualService, obj *watcherv1alpha1.Watcher) {
	if virtualService == nil {
		return
	}
	//lookup cr config
	routeIdx := lookupHTTPRouteByName(virtualService.Spec.Http, obj.Labels[ManagedBylabel])
	if routeIdx != -1 {
		virtualService.Spec.Http[routeIdx] = prepareIstioHTTPRouteForCR(obj)
		return
	}
	//if route doesn't exist already append it to the route list
	istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
	virtualService.Spec.Http = append(virtualService.Spec.Http, istioHTTPRoute)
}
func RemoveVirtualServiceConfigForCR(virtualService *istioclientapiv1beta1.VirtualService, obj *watcherv1alpha1.Watcher) {
	if virtualService == nil {
		return
	}
	if len(virtualService.Spec.Http) == 0 {
		return
	}

	routeIdx := lookupHTTPRouteByName(virtualService.Spec.Http, obj.Labels[ManagedBylabel])
	if routeIdx != -1 {
		l := len(virtualService.Spec.Http)
		copy(virtualService.Spec.Http[routeIdx:], virtualService.Spec.Http[routeIdx+1:])
		virtualService.Spec.Http[l-1] = nil
		virtualService.Spec.Http = virtualService.Spec.Http[:l-1]
	}

}

func lookupHTTPRouteByName(routes []*istioapiv1beta1.HTTPRoute, name string) int {
	if len(routes) == 0 {
		return -1
	}
	for idx, route := range routes {
		if route.Name == name {
			return idx
		}
	}
	return -1
}

func prepareIstioHTTPRouteForCR(obj *watcherv1alpha1.Watcher) *istioapiv1beta1.HTTPRoute {
	return &istioapiv1beta1.HTTPRoute{
		Name: obj.Labels[ManagedBylabel],
		Match: []*istioapiv1beta1.HTTPMatchRequest{
			{
				Uri: &istioapiv1beta1.StringMatch{
					MatchType: &istioapiv1beta1.StringMatch_Prefix{ //nolint:nosnakecase
						Prefix: fmt.Sprintf("/v%s/%s/event", contractVersion, obj.Labels[ManagedBylabel]),
					},
				},
			},
		},
		Route: []*istioapiv1beta1.HTTPRouteDestination{
			{
				Destination: &istioapiv1beta1.Destination{
					Host: destinationHost(obj.Spec.ServiceInfo.Name, obj.Spec.ServiceInfo.Namespace),
					Port: &istioapiv1beta1.PortSelector{
						Number: uint32(obj.Spec.ServiceInfo.Port),
					},
				},
			},
		},
	}
}

func destinationHost(serviceName, serviceNamespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, serviceNamespace)
}

func PerformIstioVirtualServiceCheck(ctx context.Context, istioClientSet *istioclient.Clientset,
	obj *watcherv1alpha1.Watcher, vsName, vsNamespace string,
) error {
	virtualService, err := istioClientSet.NetworkingV1beta1().
		VirtualServices(vsNamespace).Get(ctx, vsName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if !IsListenerHTTPRouteConfigured(virtualService, obj) {
		return fmt.Errorf("virtual service config not ready")
	}
	return nil
}
