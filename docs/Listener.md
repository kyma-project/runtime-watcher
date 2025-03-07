
## Kyma Listener Package

Module intended to be used with controller-runtime based operators. The Listener offers a package to set up event listening for events sent by the Kyma [Watcher](https://github.com/kyma-project/runtime-watcher/tree/main/runtime-watcher) webhook from SKRs.
It contains the [WatchEvents](https://github.com/kyma-project/runtime-watcher/blob/de040bddeba1a7875e3a0e626db4634134971022/listener/pkg/types/event.go#L8) type (mentioned in the [architecture overview](./architecture.md)) to be received from the configured channel and provides a [SKREventListener](https://github.com/kyma-project/runtime-watcher/blob/812f64dc4021b4f3c5d49aa15d1c45f5ede6ee05/listener/pkg/event/skr_events_listener.go#L30) type that implements the [Runnable](https://github.com/kubernetes-sigs/controller-runtime/blob/de4367fbd92c9d9d3a31e37107ff4fad0208f7a6/pkg/manager/manager.go#L293) interface to be registered and added to controller-runtime [Managers](https://github.com/kubernetes-sigs/controller-runtime/blob/de4367fbd92c9d9d3a31e37107ff4fad0208f7a6/pkg/manager/manager.go#L52).

See the [step-by-step guide](./guide.md) on how to setup and use this package in detail.
