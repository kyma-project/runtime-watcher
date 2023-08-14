[![REUSE status](https://api.reuse.software/badge/github.com/kyma-project/runtime-watcher)](https://api.reuse.software/info/github.com/kyma-project/runtime-watcher)

# Runtime-Watcher (PoC)

## Components

* [kcp-watcher](https://github.com/kyma-project/lifecycle-manager/blob/main/operator/api/v1alpha1/watcher_types.go)
  
    Responsible for reconciling _WatcherCRs_. Based on this reconciliation resources of [skr-watcher](./skr) are configured for all available SKRs.


* [skr-watcher](./skr)
  
    Watches configured resources by labels mentioned as part of _ValidatingWebhookConfiguration_. There is one configuration for each _WatcherCR_ present in KCP.


* [listener](./listener)

    Utility for KCP operators to start a listener on the provided port and listen on the returned channel.
    

<img src="./docs/assets/runtime-watcher.svg" width="1000">

