# Watcher CR

The [Watcher CR](https://github.com/kyma-project/lifecycle-manager/blob/main/api/v1beta2/watcher_types.go) configures the Kyma Control Plane (KCP) setup and Runtime Watcher on Kyma runtimes. Read the following document to see the the parameters the CR consists of.

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

### Resources to Watch

The **spec.resourceToWatch** field specifies the GVK of the resources Runtime Watcher watches. Note that **spec.resourceToWatch.resource** must be the API resource name, not the kind of the resource. For example, it must be "configmaps" instead of "ConfigMap". It is possible to specify the wildcard `*` for **spec.resourceToWatch.version**.

### Labels to Watch

Optionally, the **spec.labelsToWatch** field allows to filter the resources by a specific label value.

> [!NOTE]
> Runtime Watcher does not provide a mechanism to add this label to the resources. You must ensure that the resources you want to watch have this label.

### Field

The **spec.field** field specifies what parts of the watched object trigger events. Allowed values are `spec` and `status`.

If `status` is specified, watch events are only emitted if the `.status` subresource of the watched object changes. The ValidatingWebhookConfiguration is configured to only watch the status subresource accordingly. For example, "pods/status" instead of "pods".

If `spec` is specified, watch events are only emitted if the `.spec` field of the watched object changes. If the object doesn't contain a `.spec` field, it falls back to emit a watch event on **any** change to the object, including changes to metadata or status.

### Manager

The **spec.manager** field defines the URL path the Runtime Watcher sends the events to. The entire path follows `/v2/<spec.manager>/event`. Accordingly, a VirtualService is created matching the prefix `/v2/<spec.manager>/event` and routing received requests to the Service defined in **spec.serviceInfo**.

> [!NOTE]
> In the Kyma Runtime, this setting configures the ValidatingWebhookConfiguration to call `/validate/<spec.manager>` of the Runtime Watcher deployment.

### Service Info

The **spec.serviceInfo** specifies the name, namespace, and port to which events received from the Runtime Watcher are routed.

### Gateway

The **spec.gateway** field defines the label selector of the Istio Gateway in KCP. Don't change the default value.
