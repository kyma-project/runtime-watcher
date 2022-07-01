
# Kyma-Watcher (PoC)

> Modify the title and insert the name of your project. Use Heading 1 (H1).

## Overview

Kyma is the opinionated set of Kubernetes based modular building blocks that includes the necessary capabilities to develop and run enterprise-grade cloud-native applications. This repository is the PoC (proof of Concept) for the `Kyma Watcher` (short: `Watcher`). The Watcher is an operator watching for events (`ADDED`, `DELETED`, `MODIFIED`) of ConfigMaps inside a Kyma-Cluster in specific namespaces. The observed events will then be processed and communicated to {...}.
TODO

### GitHub Issues
- [MVP](https://github.com/kyma-project/kyma-operator/issues/33)
- [PoC](https://github.com/kyma-project/kyma-operator/issues/10)
## Prerequisites

- Go 1.18
- Running Kubernetes cluster



## Usage

> Explain how to use the project. You can create multiple subsections (H3). Include the instructions or provide links to the related documentation.
1. Have a running kubernetes cluster and exported the corresponding KubeConfig in the environment-variable: `KUBECONFIG`
2. Insert the IP-Adress of the KCP Gateway in: `operator/config/default/manager_auth_proxy_patch.yaml`
3. Set IMG to a valid image repository path (i.e. DockerHub <username>/kyma-watcher:latest) in: `./operator/Makefile`
4. `cd operator`
5.  `make docker-build`
6.  `make docker-push`
7.  `make deploy`
## Development

> Add instructions on how to develop the project or example. It must be clear what to do and, for example, how to trigger the tests so that other contributors know how to make their pull requests acceptable. Include the instructions or provide links to related documentation.

## Troubleshooting

> List potential issues and provide tips on how to avoid or solve them. To structure the content, use the following sections:
>
> - **Symptom**
> - **Cause**
> - **Remedy**
