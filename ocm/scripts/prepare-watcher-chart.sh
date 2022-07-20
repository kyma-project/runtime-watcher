#!/usr/bin/env bash

REPO_ROOT=../..
MAKEFILE_PATH="${REPO_ROOT}/operator"
HELM_CHARTS_PATH="${REPO_ROOT}/ocm/charts"
WATCHER_CHART_PATH="${HELM_CHARTS_PATH}/kyma-watcher"
HELM_CHART_TEMPLATES_DIR="${WATCHER_CHART_PATH}/templates"

# render kustomize manifests and copy them to templates folder of the helm chart
mkdir -p "${HELM_CHART_TEMPLATES_DIR}" && make -C "${MAKEFILE_PATH}" helm-prepare
