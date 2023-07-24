# Architecture

## Overview

The workflow of Runtime Watcher consists of the following elements:

### Watcher CR

The Watcher CR configures Kyma Control Plane (KCP) setup and Runtime Watcher on Kyma runtimes.

### Runtime Watcher

Runtime Watcher consists of a validation webhook, configured by the Watcher CR, and deployment. The validation webhook is a resource that watches configured resources and sends validation requests to the attached deployment. The deployment sends requests to KCP.

Runtime Watcher generation will be installed in Kyma Reconciliation.

### Listener package

The listener package registers an endpoint to the received events, such as Functions or example usage, and provides an event channel to listen to. The listener package is also able to provide its own validation function.

> **NOTE:** The listener package is temporarily part of the Runtime Watcher repository and will be moved to Lifecycle Manager.

## Workflow

The diagram presents Runtime Watcher's workflow.

![Runtime Watcher architecture](./assets/runtime_watcher_architecture_simplified.svg)

KCP environment includes multiple Watcher CRs that are reconciled by Lifecycle Manager. It means that Lifecycle Manager configures VirtualServices on KCP. (TBD: update the diagram to multiply the VirtualService)

Istio Gateway is static and the Lifecycle Manager service is attached to the Lifecycle Manager deployment. VirtualServices are configured dynamically by Watcher CRs.

Runtime Watcher and certificates (TBD: or certificate Secret?) are installed in every Kyma reconciliation in a Kyma runtime.

Certificate Secret includes CA certificate, mTLS certificate, and TLS key saved as a Secret and stored on a Kyma cluster. Lifecycle Manager creates the certificates using [cert-manager](https://github.com/cert-manager/cert-manager) and its self-signed feature. The solution requires Cluster Issuer and Issuer in the `istio-system` Namespace. Cluster Issuer issues a RootCACert. Issuer creates and signs all Kyma certificates using the CA certificate.

## SAN pinning

At least one SAN of the request certificate needs to match the domain specified in the Kyma CR. For that reason, the certificate of an incoming request to Gateway needs to be forwarded to Lifecycle Manager.
