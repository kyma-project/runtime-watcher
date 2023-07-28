
# Runtime Watcher

## Overview

Runtime Watcher is a validation webhook deployed by [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager) on a Kyma cluster. It watches changes on the resources configured in Watcher custom resources (CRs) in Kyma Control Plane (KCP).

The main Kyma use case for the Runtime Watcher is to reduce Lifecycle Manager's workload which results in a longer success-requeue-interval. With Runtime Watcher enabled and a Watcher CR properly configured, Kyma CRs should get requeued and reconciled only when a Kyma CR spec changes on a Kyma cluster.

Runtime Watcher is able to watch any kind of resoucres and subresources. It can also watch on status amd spec changes of those different resources.

## Components

The workflow of Runtime Watcher includes the following main components:

- Watcher custom resources (CRs)
- Runtime Watcher
- Listener package

## Read more

For further details on Runtime Watcher's architecture, see the [Architecture](./docs/01-architecture.md) document.
