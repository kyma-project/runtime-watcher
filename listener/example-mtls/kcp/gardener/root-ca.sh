cfssl gencert -initca ./cert_config/ca-csr.json | cfssljson -bare ca
cfssl gencert \
  -ca=ca.pem \
  -ca-key=ca-key.pem \
  -config=./cert_config/ca-config.json \
  -hostname="j2fmn4e1n7.jellyfish.shoot.canary.k8s-hana.ondemand.com,localhost,127.0.0.1" \
  -profile=default \
  ./cert_config/ca-csr.json | cfssljson -bare signed-cert


kubectl -n istio-system create secret tls issuer-ca-secret \
--cert=ca.pem --key=ca-key.pem -oyaml \
--dry-run=client > secret.yaml