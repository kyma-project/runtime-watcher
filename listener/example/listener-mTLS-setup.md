| Title  | Description  |  Keywords | Owner  |
| ------------ | ------------ | ------------ | ------------ |
| Listener mTLS setup  | Enable mTLS between KCP and SKR using gardner cert management extention.   |  runtime-watcher,istio,kcp-listener | kyma-project.io/jellyfish   |

---
## Before you begin

1. Ensure that you have enabled the Gardner `CertConfig` extension on your `Shoot` cluster by adding the following lines to its yaml Manifest:
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

2. Install `istio` using `istioctl` in your cluster
```sh
brew install istioctl
istioctl install
```

## Generate Root CA's PKI and Create the Gardner Issuer CR

For this task you can use your favorite tool to generate certificates and keys. The command below use [cfssl](https://github.com/cloudflare/cfssl)

1. Generate the root CA public and private key pair:
```sh
cat <<EOF | cfssl gencert -initca - | cfssljson -bare ca
{
  "CN": "jellyfish.shoot.canary.k8s-hana.ondemand.com",
  "key": {
    "algo": "rsa",
    "size": 2048
  }
}
EOF
```
2. create a tls secret that holds the root CA's PKI:
```sh
kubectl create secret tls ca-secret --cert=ca.pem --key=ca-key.pem
```
3. create an Issuer CR that uses `ca-secret` to sign the generated certificates
```sh
cat <<EOF | kubectl apply -f -
apiVersion: cert.gardener.cloud/v1alpha1
kind: Issuer
metadata:
  name: issuer-ca
spec:
  ca:
    privateKeySecretRef:
      name: ca-secret
      namespace: default
EOF
```

## Create client and server certificates

> Please make sure that the `secretRef` and the `Certificate` resources are on the same namespace.

1. Create the client certificate:

```sh
cat <<EOF | kubectl apply -f -
apiVersion: cert.gardener.cloud/v1alpha1
kind: Certificate
metadata:
  name: skr-cert
  namespace: istio-system
spec:
  commonName: "*.ak-illumi.jellyfish.shoot.canary.k8s-hana.ondemand.com"
  dnsNames:
    - "skr.ak-illumi.jellyfish.shoot.canary.k8s-hana.ondemand.com"
    - "*.svc.cluster.local"
  secretRef:
    name: skr-mtls
    namespace: istio-system
  issuerRef:
    name: issuer-ca
    namespace: default
EOF
```

2. Create the server certificate:

> Please make sure that the `secretRef` of the server certificate points to a secret in the `istio-system` for the istio gateway to find it.

```sh
cat <<EOF | kubectl apply -f -
apiVersion: cert.gardener.cloud/v1alpha1
kind: Certificate
metadata:
  name: kcp-cert
  namespace: istio-system
spec:
  commonName: "*.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com"
  dnsNames:
    - "zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com"
  secretRef:
    name: kcp-mtls
    namespace: istio-system
  issuerRef:
    name: issuer-ca
    namespace: default
EOF
```
<!-- TODO: deploy listener sample -->

## Create istio gateway and virtual service resources

1. Create an istio gateway exposing a HTTPS endpoint with tls mode set to `MUTUAL` and credentialName referencing the same `secretRef` for the server secret created in the previous step:
```sh
cat <<EOF | kubectl apply -f -
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: kcp-gw
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 443
      name: https
      protocol: HTTPS
    tls:
      mode: MUTUAL
      credentialName: kcp-mtls
    hosts:
    - "*.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com"
EOF
```
2. Create a virtual service :
```sh
cat <<EOF | kubectl apply -f -
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: kcp-vs
spec:
  hosts:
  - "*.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com"
  gateways:
  - kcp-gw
  http:
  - match:
    - uri:
        prefix: /status
    - uri:
        prefix: /delay
    route:
    - destination:
        port:
          number: 8089
        host: httpbin.watcher.svc.cluster.local
EOF
```
3. Determine the ingress IP and port:
```sh
export INGRESS_HOST=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}') && \
export SECURE_INGRESS_PORT=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="https")].port}')
```

## Test listener using curl

```sh
k get -n istio-system secret skr-mtls -o json | jq -r '.data["tls.crt"]' | base64 --decode > skr-tls-crt.pem && \
k get -n istio-system secret skr-mtls -o json | jq -r '.data["tls.key"]' | base64 --decode > skr-tls-key.pem && \
k get -n istio-system secret skr-mtls -o json | jq -r '.data["ca.crt"]' | base64 --decode > skr-ca-crt.pem
curl -v -HHost:zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com \
--resolve "zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com:$SECURE_INGRESS_PORT:$INGRESS_HOST" \
--cacert skr-ca-crt.pem --cert skr-tls-crt.pem --key skr-tls-key.pem \
"https://zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com:$SECURE_INGRESS_PORT/status/418"
...
    -=[ teapot ]=-

       _...._
     .'  _ _ `.
    | ."` ^ `". _,
    \_;`"---"`|//
      |       ;/
      \_     _/
        `"""`
```

<!-- TODO: add cleanup steps -->

## References
https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#configure-a-mutual-tls-ingress-gateway