# KLM Watcher Setup Guide

This document will guide you through the process of configuring the Watcher to watch a resource in the SKR and receive events in your components when the watched resource changes.

The Watcher is deployed to the SKR as webhook and watches specified resources for changes. When a change occurs, the Watcher sends an event to the KCP. The event is then forwarded to the component that registered a listener in the KCP.

## Watcher CR
To be able to setup a watch on a resource, you need to design a Watcher CR for it. The Watcher CR defines which resources the Watcher watches and where the events are forwarded in the KCP.

Here is the definition of the Watcher CR, in succeeding sections we will explain each field in detail:

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: Watcher
metadata:
  name: <name>
  namespace: kcp-system
  labels:
    "operator.kyma-project.io/managed-by": "<operator-name>"
spec:
  resourceToWatch:
    group: <api-group>
    version: <version> # wildcard "*" is allowed
    resource: <kind>
  labelsToWatch:
    "operator.kyma-project.io/watched-by": "<label>" # needs to be on the resource to watch
  field: "" # possible values: "spec", "status"
  serviceInfo:
    name: <service-name>
    port: <port>
    namespace: <namespace>
  gateway:
    selector:
      matchLabels:
        "operator.kyma-project.io/watcher-gateway": "default" # don't change
```
To find out more about the Watcher CR, see the [Watcher API definition](./api.md).

## Watching Resources
The `spec.resourceToWatch` field specifies the GVK of the resources the Watcher watches.

These resources must also have the `operator.kyma-project.io/watched-by` label. The `spec.labelsToWatch` field allows you to filter the resources by a specific label value.
**NOTE:** The Watcher does not provide a mechanism to add this label to the resources. You need to ensure that the resources you want to watch have this label.

Lastly, the `spec.field` field specifies if the Watcher sends an event to the KCP when the `spec` or `status` of the watched resource changes.

Here is an example of a Watcher CR that watches Secrets on the SKR:
```yaml
spec:
  resourceToWatch:
    group: ""
    version: v1
    resource: secrets
  labelsToWatch:
    "operator.kyma-project.io/watched-by": "my-operator"
  field: "spec"
```

## Consuming Events
The `spec.serviceInfo` field specifies the name, namespace, and port to which the events are routed. 

The `spec.gateway` field defines the label selector of the Istio Gateway in the KCP; it should not be changed.

Here is an example of a Watcher CR that forwards events to the `my-operator-service` service in the `my-system` namespace on port `8080`:
```yaml
spec:
  serviceInfo:
    name: my-operator-service
    namespace: my-system
    port: 8080
```

The service receiving the events can be any arbitrary service that is listening on the specified port, or it can be a k8s controller using the [Listener package](./Listener.md).

This is the request body of the event that is sent to the service:
```json
{
  "owner": {
    "name": "my-kyma",
    "namespace": "kcp-system"
  },
  "watched": {
    "name": "my-secret",
    "namespace": "kyma-system"
  },
  "watchedGvk": {
    "group": "",
    "version": "v1",
    "kind": "secrets"
  }
}
```

The `owner` field contains namespaced name of the resource that owns the watched resource. It is the reference to the resource on KCP that should be reconciled when the event is received. It is parsed from the `operator.kyma-project.io/owned-by` label on the watched resource in the format `<namespace>/<name>`.

### 1. Arbitrary Service Example

It is possible to consume events in any arbitrary service that listens on the specified port. It will receive the events as POST requests on the route `/v1/<operator-name>/event`. This <operator-name> is the value of the `operator.kyma-project.io/managed-by` label in the Watcher CR.

Here is a Golang example of a server that listens on port `8080` and prints the received events:
```go
package main

import (
    "fmt"
    "io"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/v1/my-operator/event", func(w http.ResponseWriter, r *http.Request) {
        body, err := io.ReadAll(r.Body)
        if err != nil {
            http.Error(w, "can't read body", http.StatusBadRequest)
            return
        }
        fmt.Println(string(body))
    })

    log.Fatal(http.ListenAndServe(":8080", nil))
}
```
### 2. Listener Package Example
The listener pkg simplifies setting up an endpoint for an operator residing in KCP, which receives events sent by the Watcher to KCP. It provides a simple API to register a handler for the event type and start the server in a controller.

Here is an example of setting up a listener in a controller and reconciling the owner resource:

```go
import (
    "context"
    "net/http"

    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/source"
    "sigs.k8s.io/controller-runtime/pkg/event"
    "sigs.k8s.io/controller-runtime/pkg/handler"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/client-go/util/workqueue"
    watcherevent "github.com/kyma-project/runtime-watcher/listener/pkg/event"
    "github.com/kyma-project/runtime-watcher/listener/pkg/types"
)


func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, ...) error {
    verifyFunc = func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
        return nil // If needed, implement your verification logic here
    }
    runnableListener := watcherevent.NewSKREventListener(
        ":8080", // The port on which the listener listens
        "operator-name", // The value of the `operator.kyma-project.io/managed-by` label in the Watcher CR
        verifyFunc,
    )
    if err := mgr.Add(runnableListener); err != nil {
        // Handle error
    }
    ...
    if err := ctrl.NewControllerManagedBy(mgr).For(...).
        ... // Add your controller setup here
        WatchesRawSource(source.Channel(runnableListener.ReceivedEvents, r.skrEventHandler())).
        Complete(r); err != nil {
            // Handle error
        }
}

func (r *Reconciler) skrEventHandler() *handler.Funcs {
    return &handler.Funcs{
        GenericFunc: func(ctx context.Context, evnt event.GenericEvent,
            queue workqueue.TypedRateLimitingInterface[ctrl.Request],
        ) {
            unstructWatcherEvt, conversionOk := evnt.Object.(*unstructured.Unstructured)
            if !conversionOk {
                // Handle error
            }
            
            unstructuredOwner, ok := unstructWatcherEvt.Object["owner"]
            if !ok {
                // Handle error
            }
            
            ownerObjectKey, conversionOk := unstructuredOwner.(client.ObjectKey)
            if !conversionOk {
                // Handle error
            }
            
            queue.Add(ctrl.Request{
                NamespacedName: ownerObjectKey,
            })
        },
    }
}
```

The package provides a `SKREventListener` struct that implements the `Runnable` interface. The `SKREventListener` struct listens on the specified port and verifies the incoming events. The `ReceivedEvents` channel is used to receive the events. The `SKREventListener` struct is added to the manager, and the controller watches the `ReceivedEvents` channel to reconcile the owner resource through WatcherRawSource.
