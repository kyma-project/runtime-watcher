package util

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	componentv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	gwPortVarName           = "LISTENER_GW_PORT"
	requeueIntervalVarName  = "REQUEUE_INTERVAL"
	DefaultIstioGatewayPort = 80
	DefaultRequeueInterval  = 500
	httpProtocol            = "HTTP"
	istioGWSelectorMapKey   = "istio"
	istioGWSelectorMapValue = "ingressgateway"
	istioHostsWildcard      = "*"
	firstElementIdx         = 0
	// defaultOperatorWatcherCRLabel is a label indicating that watcher CR applies to all Kymas.
	defaultOperatorWatcherCRLabel = "operator.kyma-project.io/default"
	ConfigMapResourceName         = "kcp-watcher-modules"
	// TODO: add ConfigMapNamespace as a parameter in WatcherConfig.
	ConfigMapNamespace      = metav1.NamespaceDefault
	IstioGatewayGVR         = "gateways.networking.istio.io/v1beta1"
	IstioVirtualServiceGVR  = "virtualservices.networking.istio.io/v1beta1"
	ManagedBylabel          = "operator.kyma-project.io/managed-by"
	contractVersion         = "v1"
	DefaultWebhookChartPath = "webhook-chart"
)

type WatcherConfig struct {
	// ListenerIstioGatewayHost represents hostname or IP address
	// to which the watcher on SKR will send events
	ListenerIstioGatewayHost string
	// ListenerIstioGatewayPort represents port on which KCP listeners will be reachable for SKR watchers
	ListenerIstioGatewayPort uint32
	// RequeueInterval represents requeue interval in seconds
	RequeueInterval int
}

func IstioResourcesErrorCheck(gvr string, err error) error {
	if err != nil && !k8sapierrors.IsNotFound(err) {
		return err
	}
	if err != nil {
		installed, err := isCrdInstalled(err)
		if err != nil {
			return err
		}
		if !installed {
			return fmt.Errorf("API server does not recognize %s CRD", gvr)
		}
	}
	return nil
}

func GetConfigValuesFromEnv(logger logr.Logger) *WatcherConfig {
	// TODO: remove before pushing the changes
	fileInfo, err := os.Stat(DefaultWebhookChartPath)
	if err != nil || !fileInfo.IsDir() {
		logger.V(1).Error(err, "failed to read local skr chart")
	}
	// TODO: refactor, default values are set for now
	config := &WatcherConfig{}
	_, isSet := os.LookupEnv(gwPortVarName)
	if !isSet {
		logger.V(1).Error(nil, fmt.Sprintf("%s env var is not set", gwPortVarName))
	}
	config.ListenerIstioGatewayPort = DefaultIstioGatewayPort
	_, isSet = os.LookupEnv(requeueIntervalVarName)
	if !isSet {
		logger.V(1).Error(nil, fmt.Sprintf("%s env var is not set", requeueIntervalVarName))
	}
	config.RequeueInterval = DefaultRequeueInterval
	return config
}

func AddReadyCondition(obj *componentv1alpha1.Watcher, state componentv1alpha1.WatcherConditionStatus, msg string) {
	obj.Status.Conditions = append(obj.Status.Conditions, componentv1alpha1.WatcherCondition{
		Type:               componentv1alpha1.ConditionTypeReady,
		Status:             state,
		Message:            msg,
		LastTransitionTime: &metav1.Time{Time: time.Now()},
	})
}

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

func IsVirtualServiceConfigChanged(virtualService *istioclientapiv1beta1.VirtualService,
	obj *componentv1alpha1.Watcher, gwName, gwNamespace string,
) bool {
	if len(virtualService.Spec.Gateways) != 1 {
		return true
	}
	if virtualService.Spec.Gateways[firstElementIdx] != gateway(gwName, gwNamespace) {
		return true
	}
	if len(virtualService.Spec.Hosts) != 1 {
		return true
	}
	if virtualService.Spec.Hosts[firstElementIdx] != istioHostsWildcard {
		return true
	}
	if len(virtualService.Spec.Http) != 1 {
		return true
	}
	istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
	return !isRouteConfigEqual(virtualService.Spec.Http[firstElementIdx], istioHTTPRoute)
}

func UpdateVirtualServiceConfig(virtualService *istioclientapiv1beta1.VirtualService,
	obj *componentv1alpha1.Watcher, gwName, gwNamespace string,
) {
	if virtualService == nil {
		return
	}
	istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
	virtualService.Spec = istioapiv1beta1.VirtualService{
		Gateways: []string{gateway(gwName, gwNamespace)},
		Hosts:    []string{istioHostsWildcard},
		Http:     []*istioapiv1beta1.HTTPRoute{istioHTTPRoute},
	}
}

func prepareIstioHTTPRouteForCR(obj *componentv1alpha1.Watcher) *istioapiv1beta1.HTTPRoute {
	return &istioapiv1beta1.HTTPRoute{
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
					Host: destinationHost(obj.Spec.ServiceInfo.ServiceName, obj.Spec.ServiceInfo.ServiceNamespace),
					Port: &istioapiv1beta1.PortSelector{
						Number: uint32(obj.Spec.ServiceInfo.ServicePort),
					},
				},
			},
		},
	}
}

func destinationHost(serviceName, serviceNamespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, serviceNamespace)
}

func isCrdInstalled(err error) (bool, error) {
	if err == nil || !k8sapierrors.IsNotFound(err) {
		return false, fmt.Errorf("expected non nil error of NotFound kind")
	}
	var k8sStatusErr *k8sapierrors.StatusError
	converted := errors.As(err, &k8sStatusErr)
	if !converted {
		return false, fmt.Errorf("expected non nil error of StatusError type")
	}
	errCauses := k8sStatusErr.ErrStatus.Details.Causes
	expectedErrCause := metav1.StatusCause{
		Type:    metav1.CauseTypeUnexpectedServerResponse,
		Message: "404 page not found",
	}

	if len(errCauses) > 0 && reflect.DeepEqual(expectedErrCause, errCauses[0]) {
		return false, nil
	}
	return true, nil
}

func UpdateIstioGWConfig(gateway *istioclientapiv1beta1.Gateway, gwPortNumber uint32) {
	selectorMap := make(map[string]string, 1)
	selectorMap[istioGWSelectorMapKey] = istioGWSelectorMapValue
	gateway.Spec.Selector = selectorMap
	gateway.Spec.Servers = []*istioapiv1beta1.Server{
		{
			Hosts: []string{istioHostsWildcard},
			Port: &istioapiv1beta1.Port{
				Number:   gwPortNumber,
				Name:     strings.ToLower(httpProtocol),
				Protocol: httpProtocol,
			},
		},
	}
}

func PerformConfigMapCheck(ctx context.Context, reader client.Reader,
	namespace string,
) (bool, error) {
	cmObjectKey := client.ObjectKey{
		Name:      ConfigMapResourceName,
		Namespace: namespace,
	}
	configMap := &v1.ConfigMap{}
	err := reader.Get(ctx, cmObjectKey, configMap)
	if err != nil && !k8sapierrors.IsNotFound(err) {
		return true, fmt.Errorf("failed to send get config map request to API server: %w", err)
	}
	if k8sapierrors.IsNotFound(err) {
		return true, nil
	}
	return false, nil
}

func PerformIstioVirtualServiceCheck(ctx context.Context, istioClientSet *istioclient.Clientset,
	obj *componentv1alpha1.Watcher, gwName, gwNamespace string,
) (bool, error) {
	watcherObjKey := client.ObjectKeyFromObject(obj)
	virtualService, apiErr := istioClientSet.NetworkingV1beta1().
		VirtualServices(watcherObjKey.Namespace).Get(ctx, watcherObjKey.Name, metav1.GetOptions{})
	err := IstioResourcesErrorCheck(IstioVirtualServiceGVR, apiErr)
	if err != nil {
		return true, err
	}
	if k8sapierrors.IsNotFound(apiErr) {
		return true, nil
	}
	if IsVirtualServiceConfigChanged(virtualService, obj, gwName, gwNamespace) {
		return true, nil
	}
	return false, nil
}

func gateway(gwName, gwNamespace string) string {
	return fmt.Sprintf("%s/%s", gwNamespace, gwName)
}
