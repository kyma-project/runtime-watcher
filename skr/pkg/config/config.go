package config

import "k8s.io/apimachinery/pkg/runtime/schema"

const ComponentLabel = "operator.kyma-project.io/controller-name"
const KymaCrLabel = "operator.kyma-project.io/kyma-name"

const KcpIp = "http://localhost"
const KcpPort = "8082"
const ContractVersion = "v1"
const EventEndpoint = "event"
const SkrClusterId = "skr-1"

// Gvs which will be watched
var Gvs = []schema.GroupVersion{
	{
		Group:   "operator.kyma-project.io",
		Version: "v1alpha1",
	},
	{
		Group:   "component.kyma-project.io",
		Version: "v1alpha1",
	},
}
