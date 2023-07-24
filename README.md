
# Runtime Watcher

## Overview

Runtime Watcher is ... deployed by [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager). Runtime Watcher runs on a Kyma clutser and observes changes in the Module Catalog and Component Manager. It uses certificates for communication with...

The main function of the Runtime Watcher is to reduce Lifecycle Manager's workload which results in a longer success-requeue-interval. That means that Kyma custom resources (CRS) should get requeued and reconcilied only when a Kyma CR spec changes in a Kyma runtime.

Runtime Watcher also allows to Teams with operators deployed in KCP can leverage functionality - requeue CRs in KCP corresponding to changes (spec, status, etc e.g. changes to anything in specfied GVK also Kyma CR otehr CRs, config maps with specifics labels, secrets, etc.) of specified GVKs they can watch on - requested feature - how to configure that?

The workflow of Runtime Watcher consists of the following elements:

- Watcher custom resource (CR)
- Runtime Watcher
- Listener package

For further details on Runtime Watcher's architectur, see the [Architecture](./docs/01-architecture.md) document.

