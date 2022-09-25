/bin/bash

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: kyma-sample #change with your kyma name
  namespace: kyma-system
  labels:
    "operator.kyma-project.io/managed-by": "lifecycle-manager"
    "operator.kyma-project.io/kyma-name": "kyma-sample"
type: Opaque
data:
  config: $(cat <path-to-kubeconfig-file> | sed 's/---//g' | base64)
EOF