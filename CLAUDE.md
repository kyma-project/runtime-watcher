# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

runtime-watcher is a **two-component system** that reduces Lifecycle Manager's reconciliation load by forwarding only meaningful resource changes from SKR clusters back to KCP. Instead of periodic polling, LKM reacts to real events.

```
SKR cluster                          KCP cluster
┌─────────────────────────┐          ┌────────────────────────────┐
│  runtime-watcher        │  mTLS    │  listener (library)        │
│  (ValidatingWebhook)    │─────────►│  SKREventListener HTTP srv │
│  watches configured     │          │  emits GenericEvent on ch  │
│  resources, detects     │          │  → controller-runtime      │
│  spec/status changes    │          │    reconcile queue         │
└─────────────────────────┘          └────────────────────────────┘
```

## Repo structure — three Go modules

| Directory | Module | Role |
|---|---|---|
| `listener/` | `github.com/kyma-project/runtime-watcher/listener` | **Library** consumed by KCP operators (lifecycle-manager) |
| `runtime-watcher/` | `github.com/kyma-project/runtime-watcher/skr` | **Binary** deployed as a webhook on each SKR |
| `runtime-watcher/tests/` | `github.com/kyma-project/runtime-watcher/tests` | Shared e2e test suite |

Run `go` and `make` commands from inside the module directory, not from repo root. Root `Makefile` only orchestrates linting across all modules.

## Make targets

### `listener/`

| Target | What it does |
|---|---|
| `make test` | Unit tests with race detector |
| `make lint` | golangci-lint |
| `make resolve` | `go mod tidy` |
| `make build-verbose` | Build with verbose output (uses `GOFIPS140=v1.0.0`) |

### `runtime-watcher/`

| Target | What it does |
|---|---|
| `make build` | Compile binary with version ldflags |
| `make test` | Tests with envtest |
| `make lint` | golangci-lint |
| `make run` | Run webhook locally |
| `make docker-build` / `make docker-push` | Container image lifecycle |

### Running a single test

```sh
# listener
cd listener && go test -run TestFoo ./pkg/...

# runtime-watcher
cd runtime-watcher && KUBEBUILDER_ASSETS=$(../bin/setup-envtest use 1.32.0 -p path) \
  go test -run TestFoo ./...
```

## listener — key API surface

The listener package is consumed by lifecycle-manager. Its public API:

```go
// Create and start the listener (implements controller-runtime Runnable)
listener := event.NewSKREventListener(addr, componentName)
mgr.Add(listener)

// Receive events in a controller watch
source.Channel(listener.ReceivedEvents(), &handler.EnqueueRequestForOwner{...})
```

**`types.WatchEvent`** — the event payload forwarded from SKR:
```go
type WatchEvent struct {
    Watched    ObjectKey               // {Namespace, Name} of the watched resource
    WatchedGvk metav1.GroupVersionKind // GVK of the watched resource
    SkrMeta    SkrMeta                 // Runtime ID (from mTLS certificate CN)
}
```

The listener serves HTTP on `/v2/{componentName}/event`. Certificate validation (SAN pinning) is optional and passed as a callback — lifecycle-manager uses it to verify the SKR certificate matches the Kyma CR's domain annotation.

## runtime-watcher (SKR webhook) — key behaviour

- Deployed as a `ValidatingWebhookConfiguration` on each SKR via lifecycle-manager's `SKRWebhookManager`
- Only forwards events when a **spec or status change is detected** (UPDATE operations) — create/delete are always forwarded
- Sends `WatchEvent` to KCP gateway address over **mTLS** (TLS 1.3 minimum — do not downgrade)
- Module name is extracted from the webhook URL path: `/validate/{moduleName}`

**Configuration via environment variables** (set by lifecycle-manager when deploying the webhook):

| Env var | Default | Purpose |
|---|---|---|
| `WEBHOOK_PORT` | `8443` | Webhook HTTPS port |
| `METRICS_PORT` | `2112` | Prometheus metrics port |
| `KCP_ADDR` | — | KCP gateway address |
| `KCP_CONTRACT` | — | API contract path prefix |
| `CA_CERT` / `TLS_CERT` / `TLS_KEY` | — | Certificate paths |

## Code conventions

- `GOFIPS140=v1.0.0` is used in all build commands
- **TLS 1.3 is the minimum** — the `NextProtos: []string{"http/1.1"}` configuration in the TLS setup is a CVE-2023-44487 mitigation and must not be removed
- Metrics are Prometheus-based, registered in `watchermetrics/` and `listener/pkg/metrics/`
- All HTTP retry logic uses `github.com/sethgrid/pester` (exponential backoff) — do not replace with a manual retry loop

## Security guardrails

These constraints are CVE mitigations or compliance requirements — do not remove or weaken them.

- **CVE-2023-44487 (HTTP/2 Rapid Reset)**: `NextProtos: []string{"http/1.1"}` in the TLS config disables HTTP/2 on the webhook server. This is intentional. Do not add `"h2"` to this list.
- **TLS 1.3 minimum**: The mTLS connection from SKR to KCP uses TLS 1.3. Do not add a `MinVersion` below `tls.VersionTLS13`.
- **`GOFIPS140=v1.0.0`**: All builds use the FIPS Go module. Never remove this from the Makefile or Dockerfile.
- **mTLS**: The SKR webhook always authenticates to KCP via mutual TLS. Do not bypass certificate validation in non-test code.
- **Retry loop**: `github.com/sethgrid/pester` provides exponential backoff. Replacing it with a `for` loop risks thundering-herd amplification under KCP load.

## Documentation

Detailed docs in `docs/`:
- `architecture.md` — full system architecture diagram
- `listener.md` — how to integrate the listener package in a KCP operator
- `watcher-setup-guide.md` — configuration and deployment guide
- `api.md` — Watcher CR API reference

## Model usage

Follow the Kyma team's Claude Code workflow:

- **Planning complex tasks** — switch to Opus: `/model claude-opus-4-7`
- **Implementation** — use the default Sonnet: `/model claude-sonnet-4-6`

Use Opus when you need to understand an unfamiliar subsystem, design a non-trivial change, or reason about cross-cutting impacts. Switch back to Sonnet once the approach is clear and you are writing code.
