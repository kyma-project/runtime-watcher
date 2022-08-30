# Watcher web-hook setup on local k3d 

## Preparing local k3d clusters

To create a local k3d registry run:
```shell
k3d registry delete k3d-<your-registry-name>.localhost && \
k3d registry create <your-registry-name>.localhost --port 0.0.0.0:5111
```
Next, please make sure that your `/etc/hosts` file includes this line:
```sh
127.0.0.1 k3d-<your-registry-name>.localhost
```
Now, create the SKR cluster by running:
```sh
k3d cluster create <your-skr-cluster-name> \
--registry-use k3d-<your-registry-name>.localhost:5111
```
Next, create KCP cluster by running:
```sh
k3d cluster create <your-kcp-cluster-name> \
--registry-use k3d-<your-registry-name>.localhost:5111 \
--port 9080:80@loadbalancer \
--k3s-arg '--no-deploy=traefik@server:0'
```
Here, we are using the `--port` flag to expose port `80` on the loadbalancer to port `9080` on `loacalhost`.
Also, as we are `Istio` service mesh for our cluster, we passed `'--no-deploy=traefik@server:0'` as a `--k3s-arg` to disable `traefik` as the service mesh provider which is installed by default on `k3d` clusters.

Next, install `Istio` on the KCP cluster by running:
```sh
<your-istio-bin-directory>/bin/istioctl install --set profile=default
```
To build and push a docker image for the Kyma operator, run:
```sh
kymaOperDir=<local-dir-for-kyma-operator-code>
export DOCKER_TAG=<your-docker-tag> && \
export DOCKER_PUSH_REPOSITORY=k3d-<your-registry-name>.localhost:5111 && \
export DOCKER_PUSH_DIRECTORY=<your-docker-push-dir>
make -C $kymaOperDir docker-build && make -C $kymaOperDir docker-push
```
To deploy kyma operator on KCP cluster, run:
```sh
make -C $kymaOperDir install && make -C $kymaOperDir deploy
```
After deploying Kyma operator, enable istio sidecar injection in the operator namespace:
```sh
kubectl label namespace operator-system istio-injection=enabled
```
Then, remove all of the kyma operator pods so that they will be created with istio sidecar injected:
```sh
k delete po -n operator-system  -l control-plane=controller-manager
```
Next, apply istio service mesh [configuration](./assets/service-mesh-config.yaml) to enable routing the traffic from the loadbalancer to the kyma-operator service:
```sh
k apply -f service-mesh-config.yaml
```
Next, Build the docker image of the web-hook and push it to the local registry:
```sh
webhookDir=<local-dir-for-web-hook-code>
dockerRegistryName=k3d-<your-registry-name>.localhost:5111 && imageName=<your-image-name> && imageVersion=<your-image-version>
dockerImgTag="$dockerRegistryName/$imageName:$imageVersion"
docker build $webhookDir -t $dockerImgTag && docker push $dockerImgTag
```
Next, deploy the web-hook chart on the SKR cluster:
```sh
helm template skr $webhookDir/chart/skr-webhook \
--set image.name=$dockerImgTag,kcp.loadbalancerIP=host.k3d.internal,kcp.loadbalancerPort=9080 \
| kubectl apply -f -
```
Here, by passing `host.k3d.internal` as the kcp loadbalancer IP and `9080` as the KCP loadbalancer port. We are telling the web-hook to send its requests to `localhost:9080`, which was mapped in a previous step to the KCP cluster's loadbalancer port 80, which the `Istio` gateway will be listening to.
