| Title  | Description  |  Keywords | Owner  |
| ------------ | ------------ | ------------ | ------------ |
| Listener mTLS setup  | Enable mTLS between KCP and SKR using gardner cert management extention.   |  runtime-watcher,istio,kcp-listener | kyma-project.io/jellyfish   |

## Description
This document outlines the basic steps required to get an end-to-end setup running using mTLS, involving one KCP and one SKR cluster.
In the example below a Gardener shoot is used as the KCP cluster and a cluster of your choice can be used as the SKR cluster (e.g. k3d).
There are two types of listener endpoints mentioned: `http-bin` (expects `GET` calls) and `example-listener` (expects `POST` calls).


`example-listener` can be used to simulate behavior of a real SKR watcher.

---
## Steps for KCP

1. Ensure that you have enabled the Gardner `CertConfig` extension on your `Shoot` cluster by adding the following lines to its yaml Manifest under `spec.extensions`:
    ```yaml
    kind: Shoot
    apiVersion: core.gardener.cloud/v1beta1
    spec:
      extensions:
      - type: shoot-cert-service
        providerConfig:
          apiVersion: service.cert.extensions.gardener.cloud/v1alpha1
          kind: CertConfig
          shootIssuers:
            enabled: true # if true, allows to specify issuers in the shoot cluster
    ```
    You can find the Gardner reference docs for this step [here](https://github.com/gardener/gardener-extension-shoot-cert-service/blob/47292b079fc3ea3e4014781a661957b067c0a007/docs/usage/custom_shoot_issuer.md?plain=1#L53-L71)

2. Install `istio` using `istioctl` in your cluster. Replace the host names with your cluster (shoot) domain.
   ```sh
   brew install istioctl
   sh ./kcp/istio/istio-installation.sh
   ```

3. Create root CA (and certificates optionally). This will generate a CA secret for the issuer. Replace the host names with your cluster (shoot) domain.
   ```sh
   cfssl gencert -initca ./kcp/gardener/cert_config/ca-csr.json | cfssljson -bare ca
   ```   
   
   Optionally: generate certificate. Not required you create a `Certificate` CR in step 4 below.
   ```sh
   cfssl gencert \
   -ca=ca.pem \
   -ca-key=ca-key.pem \
   -config=./kcp/gardener/cert_config/ca-config.json \
   -hostname="j2fmn4e1n7.jellyfish.shoot.canary.k8s-hana.ondemand.com,localhost,127.0.0.1" \
   -profile=default \
   ./kcp/gardener/cert_config/ca-csr.json | cfssljson -bare signed-cert
   ```
   
4. Install `Certificate` CR for KCP and SKR. The subsequent secrets will be generated.
   Uncomment `DNSEntry` CR, if you want to create your own DNS records. 
   States can be tracked for both types of CRs to make sure they're configured correctly.
   ```sh
   kubectl apply -f ./kcp/gardener/certificate.yaml
   ```

5. Install Istio resources: `Gateway`, `VirtualService`, `PeerAuthentication`. Replace the host names with your cluster (shoot) domain.
   ```sh
   kubectl apply -f ./kcp/istio/istio-resources.yaml
   ```

6. Create a Docker image for the example-listener to be deployed on KCP. Alternatively, use the already mentioned image in Step 7.
   ```sh
   docker build -t <docker-hub-repo>/listener:0.0.1 ../
   ```
   
7. Deploy the test listener resources. Please verify, the `Service` resources are being referenced by the `VirtualService` created in Step 5.
   ```sh
   kubectl apply -f ./kcp/listener/
   ```

8. Extract secret created for SKR
   ```sh
   kubectl get secret skr-tls -n istio-system -oyaml > skr-tls.yaml
   ```

## Steps for SKR 

1. Install the secret from the last step in the [section](#steps-for-kcp) above.
   ```sh
   kubectl apply -f skr-tls.yaml
   ```

2. Checkout the [SKR Watcher chart](https://github.com/kyma-project/lifecycle-manager/tree/main/skr-webhook).
3. Modify `Values.yaml` to accommodate the example-listener endpoints.
   ```
   modules: |-
     example-listener:
       statusOnly: false
       labels:
         app: "watched-by-example-listener"
   ```

4. Add KCP endpoints, based on your shoot domain to `Values.yaml`

5. Make sure Helm certificate generation is not used for `Secret` generation.
   Instead, reference the `Secret` generated from [Steps for KCP](#steps-for-kcp) Step 7. Also, copy this `CACert` over to the `caBundle` of `ValidationWebhookConfiguration`.
    
6. Install SKR watcher
   ```sh
   helm upgrade --install --namespace=default watcher ./skr-webhook
   ```  

7. Install a custom resource with the labels mentioned in Step 2.
   ```
   kubectl apply -f ./skr/
   ```
   
8. Based on the `ValidationWebhookConfiguration` SKR will send events to KCP

## Steps for debugging SKR certificate and request generation

This step can be used to debug requests to both http-bin and example-listener endpoints specified in [Steps for KCP](#steps-for-kcp).

1. Adjust `./skr/main.go` to accommodate your KCP domain name

2. Start in debug mode.

3. Request will be sent sequentially to `http-bin` and `example-listener` endpoints.

## Result

1. Check logs of SKR webhook pod that the request was sent correctly to the desired endpoint.
2. Check logs of `http-bin` and `example-listener` pods to check if requests were received correctly.

## References
https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#configure-a-mutual-tls-ingress-gateway
https://github.com/gardener/cert-management
https://gardener.cloud/docs/extensions/others/gardener-extension-shoot-cert-service/