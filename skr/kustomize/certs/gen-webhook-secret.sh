#!/usr/bin/env bash

cat <<EOF > "$KUSTOMIZE_DIR/base/secret.yaml"
apiVersion: v1
kind: Secret
metadata:
  name: skr-mtls-secret
  namespace: kyma-system
type: Opaque
data:
  CA_CERT: $CA_CERT
  TLS_CERT: $TLS_CERT
  TLS_KEY: $TLS_KEY
EOF