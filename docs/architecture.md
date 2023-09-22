# Architecture

## Overview

The workflow of Runtime Watcher uses the Watcher CR, Runtime Watcher and listener package.
The following diagram presents Runtime Watcher's workflow:

![Runtime Watcher architecture](./assets/runtime_watcher_architecture_simplified.svg)

### Watcher CR

The Watcher CR's main purposes:

- configure the KCP landscape
- configure Runtime Watcher in Kyma clusters

To configure Virtual Services in KCP, Lifecycle Manager uses the `spec.gateway` and `spec.serviceinfo` fields, and the `operator.kyma-project.io/managed-by` label from each Watcher CR. Each Watcher CR owns one Virtual Service. If Runtime Watcher's configuration changes or the Watcher CR is deleted, Lifecycle Manager re-configures the corresponding Virtual Service or deletes it as well. This mechanism is implemented in its own [reconcile loop](https://github.com/kyma-project/lifecycle-manager/blob/4cb423780633afe7805d26d624c22a6f51943492/controllers/watcher_controller.go#L74).

Runtime Watcher is configured during the reconciliation of a Kyma CR. This means, during each reconciliation of a Kyma CR all Watcher CRs are fetched, and one `ValidationWebhookConfiguration` using the `spec.LabelsToWatch`, `spec.ResourceToWatch`, and `spec.Field` from all Watcher CRs is created. The configuration includes one webhook per Watcher CR. The validation webhook configuration is applied to the Kyma cluster as a part of Runtime Watcher.

### Runtime Watcher

Runtime Watcher consists of `ValidationWebhookConfiguration` configured by Watcher CRs, an attached deployment, and a Secret holding a TLS certificate. The ValidationWebhookConfiguration is a resource that watches configured resources and sends validation requests to the attached deployment. Instead of validating the received requests, the deployment converts the validation requests into [WatchEvents](https://github.com/kyma-project/runtime-watcher/blob/de040bddeba1a7875e3a0e626db4634134971022/listener/pkg/types/event.go#L8), which are sent to KCP using a secured mTLS connection. To establish a secured mTLS connection from a Kyma cluster to KCP, it uses the TLS certificate stored inside a Secret. To see how this Secret is created, go to [certificates](#certificates).

Runtime Watcher is configured and deployed in a Kyma cluster in the Kyma reconciliation loop.

### Listener package

The Listener package is designed to streamline the process of establishing an endpoint for an operator located in KCP. This operator intends to receive Watcher Events that are transmitted from Runtime Watcher to KCP. When calling the `RegisterListenerComponent` function, it returns you a runnable listener, which is added to your reconile-manager, and a channel. See this [example of how the listener package is used in Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager/blob/24d21bb642ceaf9dadffe7732bf7c3f70c085ffb/controllers/manifest_controller.go#L43-L50). The channel becomes the source for the operator. For example, the operator can fetch the incoming WatchEvents from the channel and requeue the corresponding resource in the reconcile loop. Furthermore, it is possible to provide a validation function to the `RegisterListenerComponent` which can be used to filter out not needed requests before processing them further. An example of the validation function is SAN pinning.

To learn how to set up a listener, read the [Listener document](./Listener.md).

#### Subject Alternative Name (SAN) pinning

[SAN pinning](https://github.com/kyma-project/lifecycle-manager/blob/c1e06b7b973aca17cc715b6a4660b76f4e7b9e29/pkg/security/san_pinning.go#L55) is an example implementation of a validation function given to the listener package. SAN pinning is used in Lifecycle Manager. The validation function checks if the certificate of the incoming WatchEvent request has at least one matching SAN with the domain of the corresponding Kyma CR. The domain of a Kyma CR is saved in an annotation called `skr-domain`.

### Certificates

Because you must have an mTLS connection from the Kyma cluster to the KCP Gateway,  you need signed TLS certificates for both the KCP Gateway and each Kyma cluster. To be independent from third parties in the infrastructure, Kyma built the `Public Key Infrastructure`(PKI) by [bootstraping a self-signed CA issuer](https://cert-manager.io/docs/configuration/selfsigned/#bootstrapping-ca-issuers). The PKI uses the function of the [Cert-Manager](https://cert-manager.io/).

In each Kyma reconciliation loop, Lifecycle Manager creates or updates a [Certificate CR](https://cert-manager.io/docs/concepts/certificate/) for the Kyma CR. The Certificate CR is signed by a deployed [Issuer](https://cert-manager.io/docs/concepts/issuer/#supported-issuers), which requests the Cert-Manager to create a signed certificate. This certificate is stored in a Secret in KCP and copied over to the corresponding Kyma cluster when Runtime Watcher is deployed. The Secret includes the CA certificate, a TLS certificate, and a TLS key.
