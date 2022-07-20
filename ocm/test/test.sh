#!/usr/bin/env bash

#TODO: check if registry exists
#k3d registry create registry.localhost --port=4000 --verbose
"$COMPONENT_CLI" archive remote push ${COMPONENT_ARCHIVE} --repo-ctx ${K3D_REGISTRY_SOCKET}

#ocm image path: "k3d-registry.localhost:4000/component-descriptors/example.org/sample-module:v0.1.0"
TEST_IMAGE=k3d-registry.localhost:4000/component-descriptors/example.org/sample-module:v0.1.0 &&\
./component-cli oci pull $TEST_IMAGE

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
