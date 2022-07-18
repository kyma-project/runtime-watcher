#!/usr/bin/env bash

#TODO: check if registry exists
#k3d registry create registry.localhost --port=4000 --verbose
"$COMPONENT_CLI" archive remote push ${COMPONENT_ARCHIVE} --repo-ctx ${K3D_REGISTRY_SOCKET}

#ocm image path: "k3d-registry.localhost:4000/component-descriptors/example.org/sample-module:v0.1.0"
TEST_IMAGE=k3d-registry.localhost:4000/component-descriptors/example.org/sample-module:v0.1.0 &&\
./component-cli oci pull $TEST_IMAGE

# #{
#   "schemaVersion": 2,
#   "config": {
#     "mediaType": "application/vnd.gardener.cloud.cnudie.component.config.v1+json",
#     "digest": "sha256:413dd44468f786d6b7cec5ad7102452b87a30b8094345b93f5dbb87ba82ea6ea", => source.ref in the manifest
#     "size": 210
#   },
#   "layers": [
#     {
#       "mediaType": "application/vnd.gardener.cloud.cnudie.component-descriptor.v2+yaml+tar",
#       "digest": "sha256:e6525641c517ea0921f38cb140a86157eee94de33e9de1af712d490d81de0d59",
#       "size": 2048
#     },
#     {
#       "mediaType": "application/gzip",
#       "digest": "sha256:13b06b769b2c04ca9c00127ce5c1872c0ab707c5e9da431633653ed1269fc94e",
#       "size": 563
#     }
#   ]
# }%   

k3d cluster create kyma --registry-use k3d-registry.localhost:4000

manifestOpPath=/Users/I553979/khlifi411/GitHub/cloud-native/manifest-operator

#install manifest CRDs
make -C "${manifestOpPath}/api" install

# disable webhooks as we are testing in local
export ENABLE_WEBHOOKS=false

#run manifest operator
make -C "${manifestOpPath}/operator" run

#create kubeconfig secret
./gen-kyma-sample-secret.sh

kubectl apply -f example-manifest.yaml
