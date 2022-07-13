#!/usr/bin/env bash

# this script uses a local k3d registry to push/pull images to/from.
K3D_REGISTRY_NAME="registry.localhost"
K3D_REGISTRY_PORT="4000"
K3D_REGISTRY_SOCKET="k3d-${K3D_REGISTRY_NAME}:${K3D_REGISTRY_PORT}"

MODULE_NAME="kyma-watcher"
MODULE_VERSION="0.1.0"
OCM_PACKAGE_PATH=$(pwd)
HELM_CHART_PATH="${OCM_PACKAGE_PATH}"/chart
DATA_DIR="${HELM_CHART_PATH}"
COMPONENT_ARCHIVE="${OCM_PACKAGE_PATH}"
COMPONENT_RESOURCES="./resources.yaml"


component-cli component-archive create ${COMPONENT_ARCHIVE} --component-name ${MODULE_NAME} --component-version ${MODULE_VERSION}
component-cli ca resources add ${COMPONENT_ARCHIVE} ${COMPONENT_RESOURCES}
component-cli ca remote push ${COMPONENT_ARCHIVE} --repo-ctx ${K3D_REGISTRY_SOCKET}