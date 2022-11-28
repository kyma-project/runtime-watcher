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
          cert.gardener.cloud/secretname: acme-tls
          dns.gardener.cloud/class: garden
          dns.gardener.cloud/ttl: "600"
          dns.gardener.cloud/dnsnames: "*.j2fmn4e1n7.jellyfish.shoot.canary.k8s-hana.ondemand.com"
EOF


# cert.gardener.cloud/commonname: "*.j2fmn4e1n7.jellyfish.shoot.canary.k8s-hana.ondemand.com"
# cert.gardener.cloud/dnsnames: "j2fmn4e1n7.jellyfish.shoot.canary.k8s-hana.ondemand.com"
# cert.gardener.cloud/issuer: issuer-ca