
# Runtime-Watcher (PoC)

## Components

* [kcp-watcher](./kcp)
  
    Responsible for reconciling _WatcherCRs_. Based on this reconciliation resources of [skr-watcher](./skr) are configured for all available SKRs.


* [skr-watcher](./skr)
  
    Watches configured resources by labels mentioned as part of _ValidatingWebhookConfiguration_. There is one configuration for each _WatcherCR_ present in KCP.


* [listener](./listener)

    Utility for KCP operators to start a listener on the provided port and listen on the returned channel.
    

<img src="./docs/assets/runtime-watcher.svg" width="1000">

