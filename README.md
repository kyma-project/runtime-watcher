
# Runtime Watcher

## Overview

Runtime Watcher is a validation webhook deployed by [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager) on a Kyma cluster. It is configured by Watcher custom resources (CRs) located in Kyma Control Plane. Runtime Watcher runs on a Kyma cluster and observes changes in the Module Catalog and Component Manager. It uses certificates for communication with...

The main function of the Runtime Watcher is to reduce Lifecycle Manager's workload which results in a longer success-requeue-interval. That means that Kyma CRs should get requeued and reconciled only when a Kyma CR spec changes on a Kyma cluster.

Runtime Watcher also allows to requeue other custom resources, config maps with specific labels, or Secrets in KCP. It could watch changes corresponding to **spec**, **status** of specified Group Version Kind. also Kyma CR otehr CRs, config maps with sspecific labels, secrets, etc.) of specified GVKs they can watch on - requested feature - how to configure that?

The workflow of Runtime Watcher consists of the following elements:

- Watcher custom resource (CR)
- Runtime Watcher
- Listener package

For further details on Runtime Watcher's architecture, see the [Architecture](./docs/01-architecture.md) document.
