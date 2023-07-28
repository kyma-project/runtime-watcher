
# Runtime Watcher

## Overview

Runtime Watcher is a validation webhook deployed by [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager) on a Kyma cluster. It watches changes on the resources configured in Watcher custom resources (CRs) in Kyma Control Plane (KCP).

The main function of the Runtime Watcher is to reduce Lifecycle Manager's workload which results in a longer success-requeue-interval. With Runtime Watcher enabled, Kyma CRs should get requeued and reconciled only when a Kyma CR spec changes on a Kyma cluster.

Runtime Watcher also allows to requeue other custom resources, config maps with specific labels, or Secrets in KCP. It could watch changes corresponding to **spec**, or **status** of specified Group Version Kind. <!--TBD: Tutorial how to set up that-->

## Components

The workflow of Runtime Watcher includes the following main components:

- Watcher custom resources (CRs)
- Runtime Watcher
- Listener package

## Read more

For further details on Runtime Watcher's architecture, see the [Architecture](./docs/01-architecture.md) document.
