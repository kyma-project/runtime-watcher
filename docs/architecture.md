# Runtime Watcher Architecture

## Overview

The workflow of Runtime Watcher uses the Watcher CR, Runtime Watcher, and the listener module.
The following diagram presents Runtime Watcher's workflow:

![Runtime Watcher architecture](./assets/runtime_watcher_architecture_simplified.svg)

### Watcher CR

The Watcher CR's main purposes:

- configure the KCP landscape
- configure Runtime Watcher in Kyma clusters

To configure Virtual Services in KCP, Lifecycle Manager uses the `spec.gateway` and `spec.serviceinfo` fields, and the `operator.kyma-project.io/managed-by` label from each Watcher CR. Each Watcher CR owns one Virtual Service. If Runtime Watcher's configuration changes or the Watcher CR is deleted, Lifecycle Manager reconfigures the corresponding Virtual Service or deletes it as well. This mechanism is implemented in its own [reconcile loop](https://github.com/kyma-project/lifecycle-manager/blob/main/internal/controller/watcher/controller.go).

Runtime Watcher is configured during the reconciliation of a Kyma CR. This means, during each reconciliation of a Kyma CR, all Watcher CRs are fetched, and one `ValidationWebhookConfiguration` using the `spec.LabelsToWatch`, `spec.ResourceToWatch`, and `spec.Field` from all Watcher CRs is created. The configuration includes one webhook per Watcher CR. The validation webhook configuration is applied to the Kyma cluster as a part of Runtime Watcher.

### Runtime Watcher

Runtime Watcher consists of `ValidationWebhookConfiguration` configured by Watcher CRs, an attached deployment, and a Secret holding TLS certificates. The `ValidationWebhookConfiguration` watches configured resources and forwards admission review requests to the attached deployment. Instead of validating the received requests, the deployment converts them into [WatchEvents](https://github.com/kyma-project/runtime-watcher/blob/main/listener/pkg/v2/types/event.go), which carry the name and namespace of the changed resource along with its GVK, and sends them to KCP using a secured mTLS connection. To establish the mTLS connection from a Kyma cluster to KCP, the deployment uses the TLS certificate stored inside a Secret. For more information on how this Secret is created, see [certificates](#certificates).

Runtime Watcher is configured and deployed in a Kyma cluster in the Kyma reconciliation loop.

### Listener module

The Listener module (`runtime-watcher/listener`) defines the HTTP endpoint in KCP that receives WatchEvents transmitted from Runtime Watcher. Call `NewSKREventListener(addr, componentName string)` to get an `SKREventListener`, which implements the `Runnable` interface and can be added directly to a controller-runtime Manager. Incoming events are then read from the channel returned by `runnableListener.ReceivedEvents()` and adapted into controller-runtime generic events to requeue the corresponding resource. See this [example of how the listener module is used in Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager/blob/main/internal/controller/kyma/setup.go).

The listener authenticates each incoming request by extracting the client certificate from the `X-Forwarded-Client-Cert` (XFCC) header that the Istio gateway injects. The Common Name of that certificate is the runtime ID of the originating SKR, which the listener uses to identify and route the event.

For more information on how to set up a listener, see [Kyma Listener Module](https://github.com/kyma-project/runtime-watcher/blob/main/docs/listener.md).

### Certificates

Because you must have an mTLS connection from the Kyma cluster to the KCP Gateway, you need signed TLS certificates for both the KCP Gateway and each Kyma cluster. To be independent from third parties in the infrastructure, Kyma built the `Public Key Infrastructure` (PKI) by [bootstrapping a self-signed CA issuer](https://cert-manager.io/docs/configuration/selfsigned/#bootstrapping-ca-issuers). The PKI uses [Cert-Manager](https://cert-manager.io/) in standard landscapes or [Gardener Certificate Management](https://github.com/gardener/cert-management) in restricted markets.

The PKI uses only two certificate levels: a self-signed CA certificate and client certificates signed by that CA. The CA certificate also acts as the server certificate for the KCP Istio Gateway.

Two secrets on KCP hold the gateway certificates:
- `klm-watcher` (in `istio-system`): stores the current CA certificate, managed automatically by Cert-Manager.
- `klm-istio-gateway` (in `istio-system`): stores the server certificate in use by the Istio Gateway, together with a CA bundle of all currently unexpired CA certificates. Lifecycle Manager maintains this secret through its dedicated [Istio Gateway Secret controller](https://github.com/kyma-project/lifecycle-manager/blob/main/internal/controller/istiogatewaysecret/controller.go).

In each Kyma reconciliation loop, Lifecycle Manager creates or updates a [Certificate CR](https://cert-manager.io/docs/concepts/certificate/) for the Kyma CR. The Certificate CR is signed by a deployed [Issuer](https://cert-manager.io/docs/concepts/issuer/#supported-issuers), which requests Cert-Manager to create a signed client certificate. This certificate, together with the CA bundle from `klm-istio-gateway`, is stored in a Secret in KCP and synced to the corresponding Kyma cluster when Runtime Watcher is deployed. The Secret includes the CA bundle, a TLS certificate, and a TLS key.

#### Zero-Downtime CA Certificate Rotation

When the CA certificate is rotated, Lifecycle Manager follows a six-step process to maintain uninterrupted mTLS connectivity:

1. Cert-Manager automatically issues a new CA certificate into the `klm-watcher` secret.
2. Lifecycle Manager detects the change and adds the new CA certificate to the CA bundle in `klm-istio-gateway`, recording the timestamp in the `caAddedToBundleAt` annotation. Expired CA certificates are removed from the bundle.
3. During Kyma CR reconciliation, Lifecycle Manager compares the `caAddedToBundleAt` timestamp against the `NotBefore` date of the SKR's client certificate. If the client certificate pre-dates the new CA, Lifecycle Manager triggers re-issuance by setting the `Issuing` condition on the Certificate CR (Cert-Manager) or `renew: true` on the spec (Gardener cert-management).
4. Cert-Manager re-issues the client certificate, now signed by the new CA.
5. On the next reconciliation, Lifecycle Manager syncs the renewed client certificate and the updated CA bundle to the SKR.
6. After a configurable grace period following CA rotation, Lifecycle Manager switches the server certificate in `klm-istio-gateway` to the latest CA certificate from `klm-watcher`.

This ordering guarantees that the gateway always trusts all in-flight client certificates, and SKR deployments always trust the current server certificate, with no connection downtime. For the full design rationale see [ADR 007 - PKI Certificates and Zero-Downtime Rotation](https://github.com/kyma-project/lifecycle-manager/blob/main/docs/contributor/adr/007-pki-certs-and-rotation.md).

> ### Note:
> Lifecycle Manager updates the `operator.kyma-project.io/pod-restart-trigger` label with the value of the current resource version of the Certificate Secret CR in the Runtime Watcher deployment. This label triggers a rolling update of the Runtime Watcher deployment when the Certificate Secret is updated.
