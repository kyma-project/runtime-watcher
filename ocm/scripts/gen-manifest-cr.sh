#!/usr/bin/env bash

componentDescriptorYaml="../ocm-descriptors/component-descriptor.yaml"
watcherModuleName="kyma-project.io/module/kyma-watcher/operator"
cat <<EOF > ../samples/watcher-manifest-cr.yaml
apiVersion: component.kyma-project.io/v1alpha1
kind: Manifest
metadata:
  labels:
    operator.kyma-project.io/channel: stable
    operator.kyma-project.io/controller-name: manifest
    operator.kyma-project.io/kyma-name: kyma-sample
  name: manifestkyma-sample
  namespace: default
spec:
  config:
    ref: $(yq '.component.resources[1].access.filename' $componentDescriptorYaml)
    name: $watcherModuleName
    repo: k3d-registry.localhost:4000/component-descriptors
    type: oci-ref
  installs:
    - source:
        name: $watcherModuleName
        repo: k3d-registry.localhost:4000/component-descriptors
        ref: $(yq '.component.resources[0].access.filename' $componentDescriptorYaml)
        type: oci-ref
      name: watcher-chart
EOF
