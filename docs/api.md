# Watcher CR

The [Watcher CR](https://github.com/kyma-project/lifecycle-manager/blob/main/api/v1beta2/watcher_types.go#L121) configures the Kyma Control Plane (KCP) setup and Runtime Watcher on Kyma runtimes. Read the following document to see the the parameters the CR consists of.

## Runtime-Watcher configuration

### **spec.resourceToWatch**

Defines which resources the Runtime Watcher watches. By default, the **version** value is a wildcard (`*`), so an update is not required after the API version changes.

```yaml
spec:
  resourceToWatch:
    group: operator.kyma-project.io
    version: "*"
    resource: kymas
```

### **spec.labelsToWatch**

> **NOTE:** The resources you're watching must have the `operator.kyma-project.io/watched-by` label. The label also informs where to route requests to.

With the **spec.labelsToWatch** attribute, you can filter the specified Group/Version/Kind (GVK) in **spec.resourceToWatch**. For example, if the specified GVK is `secrets`, then it is useful to filter them by a specific label, otherwise, the Runtime Watcher would send an event to the KCP for every Create/Read/Update/Delete (CRUD) event of any Secret in the Kyma cluster.

```yaml
spec:
  labelsToWatch:
    "operator.kyma-project.io/watched-by": "lifecycle-manager"
    "example.label.to.watch": "true"
  resourceToWatch:
    group: ""
    version: "v1"
    resource: secrets
```


### **spec.field**

It uses either the `spec` or `status` value to define if Runtime Watcher sends an event to KCP if the spec of the specified GVK or the status changes.

## KCP configuration

### **spec.serviceInfo**

Specifies to which name, Namespace, and port the incoming events are routed.

```yaml
spec:
    name: klm-event-service
    namespace: kcp-system
    port: 8082
```

### **spec.gateway**

Defines to which label selector of a Gateway the VirtualService binds.

```yaml
spec:
  gateway:
    selector:
      matchLabels:
        "operator.kyma-project.io/watcher-gateway": "default"
```

## Labels

- **`operator.kyma-project.io/managed-by`:** This label specifies the module that manages and listens the Watcher CR's corresponding webhook. The value of this label is used to generate the path in KCP for the Watcher webhook's requests - `/validate/<label-value>`.