# Configuring Runtime Watcher

This document guides you through the process of configuring Runtime Watcher to watch a resource in the SKR and receive events in your components when the watched resource changes.

The Watcher mechanism is deployed to the SKR as `ValidatingWebhookConfiguration` and a webhook handler that watches specified resources for changes. When a change occurs, the webhook sends an event to KCP. The event is then forwarded to the component that registered a listener in KCP.

## Watcher CR

To set up a watch on a resource, you must define and apply a Watcher CR for it. The Watcher CR defines which resources Runtime Watcher notifies changes for and where to forward the events in KCP.

Here is the definition of the Watcher CR. The detailed field descriptions are provided below in the [Resources to Watch](#resources-to-watch) and [Events to Consume](#events-to-consume) sections.

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: Watcher
metadata:
  name: <name>
  namespace: kcp-system
spec:
  resourceToWatch:
    group: <api-group>
    version: <version>
    resource: <kind>
  labelsToWatch:
    "<some>": "<label>"
  field: <"spec" or "status">
  manager: <manager-name>
  serviceInfo:
    name: <service-name>
    port: <port>
    namespace: <namespace>
  gateway: # don't change
    selector:
      matchLabels:
        "operator.kyma-project.io/watcher-gateway": "default"
```

For more information, see the [Watcher API definition](./api.md).

### Resources to Watch

The **spec.resourceToWatch** field specifies the GVK of the resources Runtime Watcher watches. Note that **spec.resourceToWatch.resource** must be the API resource name, not the kind of the resource. For example, it must be "configmaps" instead of "ConfigMap". It is possible to specify the wildcard `*` for **spec.resourceToWatch.version**.

### Labels to Watch

Optionally, the **spec.labelsToWatch** field allows you to filter the resources by a specific label value.

> [!NOTE]
> Runtime Watcher does not provide a mechanism to add this label to the resources. You must ensure that the resources you want to watch have this label.

### Field

The **spec.field** field specifies what parts of the watched object trigger events. Allowed values are `spec` and `status`.

If `status` is specified, watch events are only emitted if the `.status` subresource of the watched object changes.

If `spec` is specified, watch events are only emitted if the `.spec` field of the watched object changes. If the object doesn't contain a `.spec` field, it falls back to emit a watch event on **any** change to the object, including changes to metadata or status.

### Manager

The **spec.manager** field defines the URL path the Runtime Watcher sends the events to. The entire path follows `/v2/<spec.manager>/event`. Accordingly, a VirtualService is created matching the prefix `/v2/<spec.manager>/event` and routing received requests to the Service defined in **spec.serviceInfo**.

### Service Info

The **spec.serviceInfo** specifies the name, namespace, and port to which events received from the Runtime Watcher are routed.

### Gateway

The **spec.gateway** field defines the label selector of the Istio Gateway in KCP. Don't change the default value.

## Consuming Events

The service receiving the events can be any arbitrary service that is listening on the specified port. Behind the service there needs to be consumer expecting POST requests on `/v2/<spec.manager>/event` with the following content:

```json
{
  "watched": { "Namespace": "<watched object's namespace>", "Name": "<watched object's name>" },
  "watchedGvk": { "group": "<watched object's group>", "version": "<watched object's version>", "kind": "<watched object's kind>" }
}
```

To determine what Kyma Runtime the received event is from, the Runtime Id can be extracted from the Common Name of the certificate attached to the request. The certificate attached to the request is available as HTTP header and the `listener` package provides the [`GetCertificateFromHeader()`](https://github.com/kyma-project/runtime-watcher/blob/de2f534ce7c0c73da817505c9aad0db12f966b27/listener/pkg/v2/certificate/parse_certificate.go#L26-L65) helper function to extract it. It can be used as follows:

```Go
func getRuntimeIdFromRequest(req *http.Request) (string, *UnmarshalError) {
	clientCertificate, err := certificate.GetCertificateFromHeader(req)
	if err != nil {
		return "", &UnmarshalError{
			fmt.Sprintf("could not get client certificate from request: %v", err),
			http.StatusUnauthorized,
		}
	}

	if clientCertificate.Subject.CommonName == "" {
		return "", &UnmarshalError{
			"client certificate common name is empty",
			http.StatusBadRequest,
		}
	}

	return clientCertificate.Subject.CommonName, nil
}
```

For further convenience, the `listener` package also provides a [`SKREventListener`](../listener/pkg/v2/event/skr_events_listener.go#L32-L43) that handles the requests and exposes a channel via [`ReceivedEvents()`](../listener/pkg/v2/event/skr_events_listener.go#L46-L54) providing an unstructured object for every received event. For a usage example, refer to Lifecycle Manager:

- https://github.com/kyma-project/lifecycle-manager/blob/d76d77a2c636b26084a0233b876c41189c556d77/internal/controller/kyma/setup.go#L30-L37
- https://github.com/kyma-project/lifecycle-manager/blob/d76d77a2c636b26084a0233b876c41189c556d77/internal/controller/kyma/setup.go#L50-L51
