# Architecture

## Overview

The workflow of Runtime Watcher consists of the following elements:

- Watcher custom resource (CR)
- Runtime Watcher
- Listener package

## Watcher CR

The Watcher CR configures Kyma Control Plane (KCP) setup and Runtime Watcher on Kyma runtimes.

## Runtime Watcher

Runtime Watcher consists of a validation webhook, configured by the Watcher CR, and deployment. The validation webhook is a resource that watches configured resources and sends validation requests to the attached deployment. The deployment sends requests to KCP.

Runtime Watcher generation will be installed in Kyma Reconciliation.

### Listener package

The listener package registers an endpoint to the received events, such as Functions, example usage, and provides an event channel to listen to. The listener package is also able to provide its own validation function.

> **NOTE:** The listener package is temporarily part of the Runtime Watcher repository and will be moved to Lifecycle Manager. 


The diagram presents Runtime Watcher's workflow.

![Runtime Watcher architecture](./assets/runtime_watcher_architecture_simplified.svg)