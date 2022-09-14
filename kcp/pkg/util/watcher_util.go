package util

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	componentv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	istioapiv1beta1 "istio.io/api/networking/v1beta1"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclient "istio.io/client-go/pkg/clientset/versioned"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	gwPortVarName                  = "LISTENER_GW_PORT"
	requeueIntervalVarName         = "REQUEUE_INTERVAL"
	DefaultIstioGatewayPort        = 80
	DefaultRequeueInterval         = 500
	firstElementIdx                = 0
	IstioVirtualServiceGVR         = "virtualservices.networking.istio.io/v1beta1"
	ManagedBylabel                 = "operator.kyma-project.io/managed-by"
	contractVersion                = "v1"
	DefaultWebhookChartPath        = "./skr-webhook"
	DefaultVirtualServiceName      = "kcp-events"
	DefaultVirtualServiceNamespace = metav1.NamespaceDefault
	DefaultWebhookChartReleaseName = "watcher"
)

type WatcherConfig struct {
	// VirtualServiceName represents the label of the virtual service resource to be updated
	VirtualServiceName string
	// VirtualServiceNamespace represents the namespace of the virtual service resource to be updated
	VirtualServiceNamespace string
	// RequeueInterval represents requeue interval in seconds
	RequeueInterval int
	// WebhookChartPath represents the path of the webhook chart
	// to be installed on SKR clusters upon reconciling watcher CRs
	WebhookChartPath string
	// WebhookChartReleaseName represents the helm release name of the webhook chart
	// to be installed on SKR clusters upon reconciling watcher CRs
	WebhookChartReleaseName string
}

func IstioResourcesErrorCheck(err error) error {
	if err != nil && !k8sapierrors.IsNotFound(err) {
		return err
	}
	if err != nil {
		installed, err := isCrdInstalled(err)
		if err != nil {
			return err
		}
		if !installed {
			return fmt.Errorf("API server does not recognize %s CRD", IstioVirtualServiceGVR)
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
	config.RequeueInterval = DefaultRequeueInterval
	return &WatcherConfig{
		RequeueInterval:    DefaultRequeueInterval,
		WebhookChartPath:   DefaultWebhookChartPath,
		VirtualServiceName: DefaultVirtualServiceName,
	}
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
	obj *componentv1alpha1.Watcher,
) bool {
	if len(virtualService.Spec.Http) == 0 {
		return true
	}

	for idx, route := range virtualService.Spec.Http {
		if route.Name == obj.Labels[ManagedBylabel] {
			istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
			return !isRouteConfigEqual(virtualService.Spec.Http[idx], istioHTTPRoute)
		}
	}

	return true
}

func UpdateVirtualServiceConfig(virtualService *istioclientapiv1beta1.VirtualService, obj *componentv1alpha1.Watcher) {
	if virtualService == nil {
		return
	}
	istioHTTPRoute := prepareIstioHTTPRouteForCR(obj)
	virtualService.Spec.Http = append(virtualService.Spec.Http, istioHTTPRoute)
}

func prepareIstioHTTPRouteForCR(obj *componentv1alpha1.Watcher) *istioapiv1beta1.HTTPRoute {
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

	if len(errCauses) > 0 && reflect.DeepEqual(
		errCauses[0], metav1.StatusCause{
			Type:    metav1.CauseTypeUnexpectedServerResponse,
			Message: "404 page not found",
		},
	) {
		return false, nil
	}
	return true, nil
}

func PerformIstioVirtualServiceCheck(ctx context.Context, istioClientSet *istioclient.Clientset,
	obj *componentv1alpha1.Watcher, vsName, vsNamespace string,
) error {
	virtualService, apiErr := istioClientSet.NetworkingV1beta1().
		VirtualServices(vsNamespace).Get(ctx, vsName, metav1.GetOptions{})
	err := IstioResourcesErrorCheck(apiErr)
	if err != nil {
		return err
	}
	if k8sapierrors.IsNotFound(apiErr) {
		return apiErr
	}
	if IsVirtualServiceConfigChanged(virtualService, obj) {
		return fmt.Errorf("virtual service config not ready")
	}
	return nil
}
