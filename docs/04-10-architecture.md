# Architecture

## Overview

The workflow of Runtime Watcher uses the Watcher CR, Runtime Watcher and listener package.
The following diagram presents Runtime Watcher's workflow.
![Runtime Watcher architecture](./assets/runtime_watcher_architecture_simplified.svg)

### Watcher CR

The Watcher CR has two main purposes: Configuring the KCP landscape and configuring the Runtime Watcher on Kyma Clusters.

To configure the Virtual Services on KCP, the KLM will use the `spec.gateway` and `spec.serviceinfo` field, and the `operator.kyma-project.io/managed-by` label from each Watcher CR. Each Watcher CR will own exact one Virtual Service. It would also be possible to have one big VirtualService including all Watcher CR configuration, but for simplicity it was decided against. If the Watcher configurations changes, or the Watcher CR has been deleted, the KLM will configure the corresponding Virtual Service or delete it as well. This whole mechanism is implemented in its own reconcile loop, which can be found [here](https://github.com/kyma-project/lifecycle-manager/blob/4cb423780633afe7805d26d624c22a6f51943492/controllers/watcher_controller.go#L74).

The configuration of the Runtime Watcher happens during the reconciliation of a Kyma CR. This means, during each reconciliation of a Kyma CR all Watcher CRs will be fetched and a `ValidationWebhookConfiguration` will be created using the `spec.LabelsToWatch`, `spec.ResourceToWatch` and `spec.Field` from all Watcher CRs. One webhook in the configuration for each Watcher CR. The resulting Validation Webhook Configuration then will be applied to the Kyma cluster as a part of the Runtime Watcher.



### Runtime Watcher

Runtime Watcher consists of a `ValidationWebhookConfiguration`, configured by Watcher CRs, an attached deployment and a secret holding a TLS certificate. The validation webhook is a resource that watches configured resources and sends validation requests to the attached deployment. Instead of validating the received requests, the deployment converts the validation requests into a [WatchEvent](https://github.com/kyma-project/runtime-watcher/blob/de040bddeba1a7875e3a0e626db4634134971022/listener/pkg/types/event.go#L8), which is then send to the KCP using a secured mTLS connection. To establish a secured mTLS connection from the SKR to the KCP, it uses the TLS certificate stored inside a secret. How this secret is being created is [described in this section](###certificates).

Runtime Watcher is configured and deployed on a Kyma cluster in the Kyma reconciliation loop.

### Listener package

The Listener package is designed to streamline the process of establishing an endpoint for an operator (located within KCP). This operator intends to receive Watcher Events that are transmitted from the Runtime Watcher to the KCP. When calling the `RegisterListenerComponent` function, it returns you a runnable Listener, which should be added to your reconile-manager, and a channel. Example use in the KLM can be found [here](https://github.com/kyma-project/lifecycle-manager/blob/24d21bb642ceaf9dadffe7732bf7c3f70c085ffb/controllers/manifest_controller.go#L43-L50). The channel is then the source the operator should mainly deal with, i.e. it can fetch the incoming watch events from the channel and requeue the corresponding resource in the reconcile loop. Furthermore, it is possible to provide a validation function to the `RegisterListenerComponent` which can be used to already filter out not needed request before processing them further. An example for a verify-function is the SAN pinning, described in the next section.

To learn how to setup the listener, have a look [here](./Listener.md)

#### Subject Alternative Name (SAN) pinning

[SAN pinning](https://github.com/kyma-project/lifecycle-manager/blob/c1e06b7b973aca17cc715b6a4660b76f4e7b9e29/pkg/security/san_pinning.go#L55) is an example implementation of a validation function given to the listener package, and is used in the KLM. The given `Verify` function checks if the certificate of the incoming watcher event request has at least one matching SAN with the domain of the corresponding Kyma CR. The domain of a Kyma CR is saved in an annotation which is called `skr-domain`.

### Certificates
Since it is required to have a mTLS connection from the Kyma cluster to the KCP Gateway, signed TLS certificates are needed for the KCP Gateway as well as for each Kyma cluster. Considering the fact that we do not want to rely on third parties in our infrastructure, we are building our own `Public Key Infrastructure`(PKI) by [bootstraping a selfsigned CA issuer](https://cert-manager.io/docs/configuration/selfsigned/#bootstrapping-ca-issuers). To build this, we are going to leverage the function of the [Cert-Manager](https://cert-manager.io/). 

In each Kyma reconciliation loop, the KLM will create or update a [Certificate CR](https://cert-manager.io/docs/concepts/certificate/) for the Kyma CR. This certificate CR will then be signed by a deployed [Issuer](https://cert-manager.io/docs/concepts/issuer/#supported-issuers), which will request the Cert-Manager to create a signed certificate. This certificate will then be stored in a secret on KCP and copied over to the corresponding Kyma cluster when the Runtime Watcher is being deployed. The secret includes the CA certificate, a TLS certificate, and a TLS key.
