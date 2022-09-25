cat <<EOF | istioctl install -y -f -
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
spec:
  profile: default
  components:
    ingressGateways:
    - name: istio-ingressgateway
      enabled: true
      k8s:
        serviceAnnotations:
          cert.gardener.cloud/secretname: test-tls
          cert.gardener.cloud/commonname: "*.ab-test1.jellyfish.shoot.canary.k8s-hana.ondemand.com"
          dns.gardener.cloud/class: garden
          dns.gardener.cloud/ttl: "600"
          dns.gardener.cloud/dnsnames: "ab-test1.jellyfish.shoot.canary.k8s-hana.ondemand.com"
          cert.gardener.cloud/dnsnames: "ab-test1.jellyfish.shoot.canary.k8s-hana.ondemand.com"
EOF
#          cert.gardener.cloud/issuer: issuer-ca

cfssl gencert -initca ./cert_config/ca-csr.json | cfssljson -bare ca
cfssl gencert \
  -ca=ca.pem \
  -ca-key=ca-key.pem \
  -config=./cert_config/ca-config.json \
  -hostname="ab-test1.jellyfish.shoot.canary.k8s-hana.ondemand.com,localhost,127.0.0.1" \
  -profile=default \
  ./cert_config/ca-csr.json | cfssljson -bare signed-cert


kubectl -n istio-system create secret tls issuer-ca-secret \
--cert=ca.pem --key=ca-key.pem -oyaml \
--dry-run=client > secret.yaml