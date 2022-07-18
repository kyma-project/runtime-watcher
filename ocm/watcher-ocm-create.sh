#!/usr/bin/env bash

#1st step: package watcher helm chart
MAKEFILE_PATH=../
HELM_PACKAGE_PATH=$(pwd)/chart
HELM_CHART_PATH="${HELM_PACKAGE_PATH}/kyma-watcher"

# render kustomize manifests and copy them ->
# to templates folder for helm package
make -C "${MAKEFILE_PATH}" helm-prepare

#2nd step: create OCM package using watcher helm chart

# this script uses a local k3d registry to push/pull images to/from.
K3D_REGISTRY_NAME="registry.localhost"
K3D_REGISTRY_PORT="4000"
K3D_REGISTRY_SOCKET="k3d-${K3D_REGISTRY_NAME}:${K3D_REGISTRY_PORT}"

MODULE_NAME="kyma-project.io/module/kyma-watcher/operator"
MODULE_VERSION="0.1.0"
OCM_PACKAGE_PATH=$(pwd)
DATA_DIR="${HELM_CHART_PATH}"
COMPONENT_ARCHIVE="${OCM_PACKAGE_PATH}"
COMPONENT_RESOURCES="./resources.yaml"
LOCALBIN=$(pwd)/bin
COMPONENT_CLI="${LOCALBIN}/component-cli"

#3rd step: install gardner component-cli in working directory
if [ ! -f "$COMPONENT_CLI" ]; then
    GOBIN="${LOCALBIN}" go install github.com/gardener/component-cli/cmd/component-cli@latest
fi

#TODO: validate module name against regex
#moduleName=sample-module
# if [[ "$moduleName" =~ ^[a-z0-9.\-]+[.][a-z]{2,4}/[-a-z0-9/_.]*$ ]]; then
#    echo "Valid name"
#else
#    echo "Invalid name"
#fi

"$COMPONENT_CLI" archive create ${COMPONENT_ARCHIVE} \
    --component-name ${MODULE_NAME} \
    --component-version ${MODULE_VERSION}
#TODO: check if templates dir of chart is empty
"$COMPONENT_CLI" archive resources add ${COMPONENT_ARCHIVE} ${COMPONENT_RESOURCES}
