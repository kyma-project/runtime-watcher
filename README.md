[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/runtime-watcher)](https://api.reuse.software/info/github.com/kyma-project/runtime-watcher)

# Runtime Watcher

## Overview

Runtime Watcher is mostly a validation webhook deployed by [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager) in a Kyma cluster. It watches changes in the resources, configured by Watcher custom resources (CRs) in Kyma Control Plane (KCP).

The main Kyma use case for the Runtime Watcher is to reduce Lifecycle Manager's workload which results in a longer success-requeue-interval. With Runtime Watcher enabled and a Watcher CR properly configured, Kyma CRs should be requeued and reconciled only when a Kyma CR spec changes on a Kyma cluster.

Runtime Watcher is able to watch any kind of resources and subresources. It can watch on status or spec changes of those different resources. More details can be found in the [Watcher CR definition](https://github.com/kyma-project/lifecycle-manager/blob/main/api/v1beta2/watcher_types.go).


## Components

The workflow of Runtime Watcher includes the following main components:

### Watcher custom resources (CRs)
[Watcher CRs](https://github.com/kyma-project/lifecycle-manager/blob/main/api/v1beta2/watcher_types.go) configure the [Virtual Services](https://istio.io/latest/docs/reference/config/networking/virtual-service/) in KCP, which are used as a reverse proxy to route incoming requests to the correct operator. Watcher CRs are also used to configure the Runtime Watcher deployed in each Kyma cluster. For more details, see the [Watcher CR](./docs/api.md) document.

### Runtime Watcher
The Runtime Watcher consists of multiple parts. First of all, it has a ValidationWebhookConfiguration with one or more webhooks which is re-used. Instead of its original use case, it is re-used to validate CRUD actions (creating, reading, updating, and deleting) on Kubernetes resources, for the general watch mechanism inside the SKR. These webhooks are configured by the Lifecycle Manager using the Watcher CRs. In addition, a deployment is attached to this webhook, which is the receiver for the validation requests. The deployment converts the validation requests into [WatchEvents](https://github.com/kyma-project/runtime-watcher/blob/de040bddeba1a7875e3a0e626db4634134971022/listener/pkg/types/event.go#L8), which are then sent to KCP using a secured mTLS connection. To establish the secured mTLS connection from a Kyma cluster to KCP, Lifecycle Manager deploys a Secret with a TLS certificate in each Kyma cluster.

### Listener package
The [Listener package](https://github.com/kyma-project/runtime-watcher/tree/main/listener) simplifies setting up an endpoint for an operator residing in KCP, which receives WatchEvents sent by Runtime Watcher to KCP. Follow the [guide](./docs/Listener.md) to learn how to use it.

## Read more

The release process is described in the [How To Release](./docs/how_to_release.md) document.
For further details on Runtime Watcher's architecture, see the [Architecture](./docs/architecture.md) document.

## Contributing

See the [Contributing Rules](CONTRIBUTING.md).

## Code of Conduct

See the [Code of Conduct](CODE_OF_CONDUCT.md) document.

## Licensing

See the [license](./LICENSE) file.
