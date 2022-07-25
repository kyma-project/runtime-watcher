#!/usr/bin/env bash

#Pushes OCM package to k3d registry for local testing
REPO_ROOT=../..
LOCALBIN="${REPO_ROOT}/ocm/bin"
COMPONENT_CLI="${LOCALBIN}/component-cli"
OCM_PACKAGE_PATH="${REPO_ROOT}/ocm/ocm-descriptors"

# this script uses a local k3d registry to push/pull images to/from.
K3D_REGISTRY_NAME="k3d-registry.localhost"
K3D_REGISTRY_PORT="4000"
K3D_REGISTRY_SOCKET="${K3D_REGISTRY_NAME}:${K3D_REGISTRY_PORT}"

"$COMPONENT_CLI" archive remote push ${OCM_PACKAGE_PATH} --repo-ctx ${K3D_REGISTRY_SOCKET}