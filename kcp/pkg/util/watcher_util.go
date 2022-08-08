package util

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	componentv1alpha1 "github.com/kyma-project/kyma-watcher/kcp/api/v1alpha1"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	gwPortVarName           = "LISTENER_GW_PORT"
	requeueIntervalVarName  = "REQUEUE_INTERVAL"
	defaultIstioGatewayPort = 80
	defaultRequeueInterval  = 500
	httpProtocol            = "HTTP"
	istioGWSelectorMapKey   = "istio"
	istioGWSelectorMapValue = "ingressgateway"
	istioHostsWildcard      = "*"
	firstElementIdx         = 0
)

type WatcherConfig struct {
	// ListenerIstioGatewayPort represents port on which KCP listeners will be reachable for SKR watchers
	ListenerIstioGatewayPort uint32
	// RequeueInterval represents requeue interval in seconds
	RequeueInterval int
}

func IstioReourcesErrorCheck(gvr string, err error) (bool, error) {
	if err != nil && !errors.IsNotFound(err) {
		return false, err
	}
	if err != nil {
		installed, err := isCrdInstalled(err)
		if err != nil {
			return false, err
		}
		if !installed {
			return false, fmt.Errorf("API server does not recognize %s CRD", gvr)
		}
	}
	return true, nil
}

func GetConfigValuesFromEnv(logger logr.Logger) *WatcherConfig {
	config := &WatcherConfig{}
	gwPortVarValue, isSet := os.LookupEnv(gwPortVarName)
	if !isSet {
		logger.V(1).Error(nil, fmt.Sprintf("%s env var is not set", gwPortVarName))
		config.ListenerIstioGatewayPort = defaultIstioGatewayPort
	}
	requeueIntervalVarValue, isSet := os.LookupEnv(requeueIntervalVarName)
	if !isSet {
		logger.V(1).Error(nil, fmt.Sprintf("%s env var is not set", requeueIntervalVarName))
		config.RequeueInterval = defaultRequeueInterval
		return config
	}
	gwPortIntValue, err := strconv.ParseUint(gwPortVarValue, 10, 0)
	if err != nil {
		logger.V(1).Error(err, "could not get unsigned int value for ", gwPortVarName)
	}
	config.ListenerIstioGatewayPort = uint32(gwPortIntValue)
	requeueIntervalIntValue, err := strconv.Atoi(requeueIntervalVarValue)
	if err != nil {
		logger.V(1).Error(err, "could not get int value for ", requeueIntervalVarName)
	}
	config.RequeueInterval = requeueIntervalIntValue
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

	if route1.Match[firstElementIdx].Uri.MatchType.(*istioapiv1beta1.StringMatch_Prefix).Prefix !=
		route2.Match[firstElementIdx].Uri.MatchType.(*istioapiv1beta1.StringMatch_Prefix).Prefix {
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

func IsVirtualServiceConfigChanged(virtualService *istioclientapiv1beta1.VirtualService, obj *componentv1alpha1.Watcher, gwName string) bool {
	if len(virtualService.Spec.Gateways) != 1 {
		return true
	}
	if virtualService.Spec.Gateways[firstElementIdx] != gwName {
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
	istioHttpRoute := prepareIstioHTTPRouteForCR(obj)
	return !isRouteConfigEqual(virtualService.Spec.Http[firstElementIdx], istioHttpRoute)
}

func UpdateVirtualServiceConfig(virtualService *istioclientapiv1beta1.VirtualService, obj *componentv1alpha1.Watcher, gwName string) {
	if virtualService == nil {
		return
	}
	istioHttpRoute := prepareIstioHTTPRouteForCR(obj)
	virtualService.Spec = istioapiv1beta1.VirtualService{
		Gateways: []string{gwName},
		Hosts:    []string{istioHostsWildcard},
		Http:     []*istioapiv1beta1.HTTPRoute{istioHttpRoute},
	}
}

func prepareIstioHTTPRouteForCR(obj *componentv1alpha1.Watcher) *istioapiv1beta1.HTTPRoute {
	return &istioapiv1beta1.HTTPRoute{
		Match: []*istioapiv1beta1.HTTPMatchRequest{
			{
				Uri: &istioapiv1beta1.StringMatch{
					MatchType: &istioapiv1beta1.StringMatch_Prefix{
						Prefix: fmt.Sprintf("/v%s/%s/event", obj.Spec.ContractVersion, obj.Spec.ComponentName),
					},
				},
			},
		},
		Route: []*istioapiv1beta1.HTTPRouteDestination{
			{
				Destination: &istioapiv1beta1.Destination{
					Host: obj.Spec.ServiceInfo.ServiceName,
					Port: &istioapiv1beta1.PortSelector{
						Number: uint32(obj.Spec.ServiceInfo.ServicePort),
					},
				},
			},
		},
	}
}

func isCrdInstalled(err error) (bool, error) {
	if err == nil || !errors.IsNotFound(err) {
		return false, fmt.Errorf("expected non nil error of NotFound kind")
	}
	errCauses := err.(*errors.StatusError).ErrStatus.Details.Causes
	expectedErrCause := metav1.StatusCause{
		Type:    metav1.CauseTypeUnexpectedServerResponse,
		Message: "404 page not found",
	}

	if len(errCauses) > 0 && reflect.DeepEqual(expectedErrCause, errCauses[0]) {
		return false, nil
	}
	return true, nil
}

func IsGWConfigChanged(gateway *istioclientapiv1beta1.Gateway, gwPortNumber uint32) bool {
	if len(gateway.Spec.Selector) != 1 {
		return true
	}
	istioSelector, ok := gateway.Spec.Selector[istioGWSelectorMapKey]
	if !ok || istioSelector != istioGWSelectorMapValue {
		return true
	}
	if len(gateway.Spec.Servers) != 1 {
		return true
	}
	listenerGwPort := gateway.Spec.Servers[0].Port
	if listenerGwPort.Number != gwPortNumber || listenerGwPort.Name != strings.ToLower(httpProtocol) ||
		listenerGwPort.Protocol != httpProtocol {
		return true
	}

	return false
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
