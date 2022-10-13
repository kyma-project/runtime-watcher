| Title  | Description  |  Keywords | Owner  |
| ------------ | ------------ | ------------ | ------------ |
| Listener mTLS setup  | Enable mTLS between KCP and SKR using gardner cert management extention.   |  runtime-watcher,istio,kcp-listener | kyma-project.io/jellyfish   |

---
## Before you begin

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

2. Install `istio` using `istioctl` in your cluster
```sh
brew install istioctl
istioctl install
```

## Generate Root CA's PKI and Create the Gardner Issuer CR

For this task you can use your favorite tool to generate certificates and keys. The command below uses [cfssl](https://github.com/cloudflare/cfssl)

1. Generate the root CA public and private key pair:
```sh
cat <<EOF | cfssl gencert -initca - | cfssljson -bare ca
{
  "CN": "jellyfish.shoot.canary.k8s-hana.ondemand.com",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "DE",
      "L": "Munich",
      "O": "BTP",
      "OU": "Kyma",
      "ST": "Bayern"
    }
  ]
}
EOF
```
2. create a tls secret that holds the root CA's PKI:
```sh
kubectl create secret tls ca-secret --cert=ca.pem --key=ca-key.pem
```
3. create an Issuer CR that uses `ca-secret` to sign the generated certificates:
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
    - "skr-watcher-webhook.default.svc"
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
## Deploy httpbin and sample-listener and expose them
1. Create `watcher` namespace and enable istio-injection:
```sh
```
2. Deploy httpbin :
```sh
kubectl create ns watcher && \
kubectl label --overwrite ns watcher istio-injection=enabled && \
kubectl create -n watcher deployment httpbin --image=docker.io/kennethreitz/httpbin --port=80 && \
kubectl expose -n watcher deployment httpbin --port=8089 --target-port=80
```
3. Build and push sample-listener docker image:
```sh
make -C listener docker-build IMG=khlifi411/sample-listener:0.0.1
make -C listener docker-push IMG=khlifi411/sample-listener:0.0.1
```
4. Deploy sample listener:
```sh
kubectl create -n watcher deployment sample-listener --image=khlifi411/sample-listener:0.0.1 --port=8089 && \
kubectl expose -n watcher deployment sample-listener --port=10019 --target-port=8089
```
5. (optional) check listener logs:
```sh
kubectl logs -n watcher -l app=sample-listener
# sample output
1.665476404040741e+09   INFO    Listener is starting up...      {"Module": "Listener", "Addr": ":8089", "ApiPath": "/v1/example-listener/event"}
```

## Create istio gateway and virtual service resources

1. Create an istio gateway exposing a HTTPS endpoint with tls mode set to `MUTUAL` and `credentialName` referencing the same `secretRef` for the server secret created in the previous step:
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
  - match:
    - uri:
        prefix: /v1/example-listener/event
    route:
    - destination:
        port:
          number: 10019
        host: sample-listener.watcher.svc.cluster.local
EOF
```
3. Determine the ingress IP and port:
```sh
export INGRESS_HOST=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}') && \
export SECURE_INGRESS_PORT=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.spec.ports[?(@.name=="https")].port}')
```

## Test httpbin using curl

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
## Test sample-listener using curl:
```sh
k get -n istio-system secret skr-mtls -o json | jq -r '.data["tls.crt"]' | base64 --decode > skr-tls-crt.pem && \
k get -n istio-system secret skr-mtls -o json | jq -r '.data["tls.key"]' | base64 --decode > skr-tls-key.pem && \
k get -n istio-system secret skr-mtls -o json | jq -r '.data["ca.crt"]' | base64 --decode > skr-ca-crt.pem
curl -v -X POST \
-HHost:zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com \
-H "Content-Type: application/json" \
-d '{"owner":{"Name":"kyma","Namespace":"default"},"watched":{"Name":"watched-resource","Namespace":"default"},"watchedGvk":{"group":"operator.kyma-project.io","version":"v1alpha1","kind":"kyma"}}' \
--resolve "zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com:$SECURE_INGRESS_PORT:$INGRESS_HOST" \
--cacert skr-ca-crt.pem --cert skr-tls-crt.pem --key skr-tls-key.pem \
"https://zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com:$SECURE_INGRESS_PORT/v1/example-listener/event"
# curl output
* Added zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com:443:34.140.62.50 to DNS cache
* Hostname zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com was found in DNS cache
*   Trying 34.140.62.50:443...
* Connected to zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com (34.140.62.50) port 443 (#0)
* ALPN, offering h2
* ALPN, offering http/1.1
* successfully set certificate verify locations:
*  CAfile: skr-ca-crt.pem
*  CApath: none
* (304) (OUT), TLS handshake, Client hello (1):
* (304) (IN), TLS handshake, Server hello (2):
* (304) (IN), TLS handshake, Unknown (8):
* (304) (IN), TLS handshake, Request CERT (13):
* (304) (IN), TLS handshake, Certificate (11):
* (304) (IN), TLS handshake, CERT verify (15):
* (304) (IN), TLS handshake, Finished (20):
* (304) (OUT), TLS handshake, Certificate (11):
* (304) (OUT), TLS handshake, CERT verify (15):
* (304) (OUT), TLS handshake, Finished (20):
* SSL connection using TLSv1.3 / AEAD-AES256-GCM-SHA384
* ALPN, server accepted to use h2
* Server certificate:
*  subject: CN=*.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com
*  start date: Oct  5 17:03:12 2022 GMT
*  expire date: Jan  3 17:03:12 2023 GMT
*  subjectAltName: host "zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com" matched cert's "zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com"
*  issuer: CN=jellyfish.shoot.canary.k8s-hana.ondemand.com
*  SSL certificate verify ok.
* Using HTTP2, server supports multiplexing
* Connection state changed (HTTP/2 confirmed)
* Copying HTTP/2 data in stream buffer to connection buffer after upgrade: len=0
* Using Stream ID: 1 (easy handle 0x7f9c7400da00)
> POST /v1/example-listener/event HTTP/2
> Host:zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com
> user-agent: curl/7.79.1
> accept: */*
> content-type: application/json
> content-length: 192
> 
* We are completely uploaded and fine
* Connection state changed (MAX_CONCURRENT_STREAMS == 2147483647)!
< HTTP/2 200 
< date: Tue, 11 Oct 2022 08:42:04 GMT
< content-length: 0
< x-envoy-upstream-service-time: 28
< server: istio-envoy
< 
* Connection #0 to host zeta.ak-neferpitu.jellyfish.shoot.canary.k8s-hana.ondemand.com left intact
```

## Verify sample-listener logs
```log
❯ kubectl logs -n watcher -l app=sample-listener
1.665476404040741e+09   INFO    Listener is starting up...      {"Module": "Listener", "Addr": ":8089", "ApiPath": "/v1/example-listener/event"}
1.6654777241444569e+09  DEBUG   received event from SKR {"Module": "Listener"}
1.665477724144772e+09   INFO    dispatched event object into channel    {"Module": "Listener", "resource-name": "kyma"}
1.6654777241449695e+09  INFO    example-listener        watcher event received....
1.6654777241450973e+09  INFO    example-listener        &{map[metadata:map[name:kyma namespace:default] owner:default/kyma watched:default/watched-resource watched-gvk:operator.kyma-project.io/v1alpha1, Kind=kyma]}
```

<!-- TODO: add cleanup steps -->

## References
https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#configure-a-mutual-tls-ingress-gateway