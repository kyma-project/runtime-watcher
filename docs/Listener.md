
# Kyma Listener

Listener module that listens to events sent by the Kyma [Watcher](https://github.com/kyma-project/runtime-watcher/tree/main/runtime-watcher) component.

## Overview

Typically, we use the Listener module with operators built using Kubebuilder, but we can use it for other operators as well.

### Use

1. For operators built using the Kubebuilder framework, leverage your `SetupWithManager()` method to initialize the Listener by calling `RegisterListenerComponent()`.

2. Set up your controller to watch for changes sent through the `source.Channel{}` returned by the Listener module, and to react to them by calling the `(blder *Builder) Watches()` method and providing your `handler.EventHandler` implementation.

3. To start the Listener, add it as a runnable to your controller-manager: Call `mgr.Add()` and pass the Listener returned by `RegisterListenerComponent()`.


### Sample code

```golang
//register listener component
runnableListener, eventChannel := listener.RegisterListenerComponent(listenerAddr, strings.ToLower(v1alpha1.KymaKind))

//watch event channel
controllerBuilder.Watches(eventChannel, &handler.EnqueueRequestForObject{})

//start listener as a manager runnable
mgr.Add(runnableListener)
```
