
# Runtime Watcher

## Overview

Runtime Watcher is a validation webhook deployed by [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager) on a Kyma cluster. It watches changes on the resources, configured by Watcher custom resources (CRs) in Kyma Control Plane (KCP).

The main Kyma use case for the Runtime Watcher is to reduce Lifecycle Manager's workload which results in a longer success-requeue-interval. With Runtime Watcher enabled and a Watcher CR properly configured, Kyma CRs should get requeued and reconciled only when a Kyma CR spec changes on a Kyma cluster.

Runtime Watcher is able to watch any kind of resources and subresources. It can watch on status or spec changes of those different resources. More details can be found in the [Watcher CR definition](https://github.com/kyma-project/lifecycle-manager/blob/main/api/v1beta2/watcher_types.go).


## Components

The workflow of Runtime Watcher includes the following main components:

### [Watcher custom resources (CRs)](https://github.com/kyma-project/lifecycle-manager/blob/main/api/v1beta2/watcher_types.go)
The Watcher CRs configures the [Virtual Services](https://istio.io/latest/docs/reference/config/networking/virtual-service/) in the KCP, which are used as a Reverse-Proxy to route incoming request to the correct operator. Furthermore, they are used to configure the [Runtime Watcher](###runtime-watcher) deployed in each Kyma cluster. More details can be found [here](./docs/05-10-api.md).

### [Runtime Watcher](https://github.com/kyma-project/runtime-watcher/tree/main/runtime-watcher)
The Runtime Watcher consists of multiple parts. First of all, it has a ValidationWebhook which is re-used, instead of its original usecase, for the general watch mechanism inside the SKR. This webhook is configured by the Lifecycle Manager using the Watcher CRs. In addition, a deployment is attached to this webhook, which is the receiver for the validation requests. The deployment converts the validation requests into a [WatchEvent](https://github.com/kyma-project/runtime-watcher/blob/de040bddeba1a7875e3a0e626db4634134971022/listener/pkg/types/event.go#L8), which is then send to the KCP using a secured mTLS connection. To establish a secured mTLS connection from the SKR to the KCP, a secret which holds a TLS certificate will be deployed by the Lifecycle Manager on each SKR as well.


### [Listener package](https://github.com/kyma-project/runtime-watcher/tree/main/listener)
The Listener pkg is provided to simplify setting up an endpoint for an operator (residing inside KCP), which wants to receive                                                                                                                                                                                      Watcher Events send by the Runtime Watcher to the KCP.

## Read more

For further details on Runtime Watcher's architecture, see the [Architecture](./docs/01-architecture.md) document.
