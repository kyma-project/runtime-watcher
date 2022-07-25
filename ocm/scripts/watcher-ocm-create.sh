#!/usr/bin/env bash

#Creates OCM package using watcher helm chart

REPO_ROOT=../..
MODULE_NAME="kyma-project.io/module/kyma-watcher/operator"
MODULE_VERSION="0.1.0"
OCM_PACKAGE_PATH="${REPO_ROOT}/ocm/ocm-descriptors"
RESOURCES_DIR="${REPO_ROOT}/ocm/resources"
COMPONENT_ARCHIVE="${OCM_PACKAGE_PATH}"
WATCHER_CHART_RESOURCES="${RESOURCES_DIR}/watcher-ocm-resources.yaml"
LOCALBIN="${REPO_ROOT}/ocm/bin"
COMPONENT_CLI="${LOCALBIN}/component-cli"

#Install gardner component-cli in working directory
if [ ! -f "$COMPONENT_CLI" ]; then
    GOBIN="${LOCALBIN}" go install github.com/gardener/component-cli/cmd/component-cli@latest
fi

oldCompDescFile="${OCM_PACKAGE_PATH}/component-descriptor.yaml"
if [ -f "$oldCompDescFile" ]; then
    rm $oldCompDescFile
fi

"$COMPONENT_CLI" archive create ${OCM_PACKAGE_PATH} \
    --component-name ${MODULE_NAME} \
    --component-version ${MODULE_VERSION}

"$COMPONENT_CLI" archive resources add ${OCM_PACKAGE_PATH} ${WATCHER_CHART_RESOURCES}
