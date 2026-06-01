# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

runtime-watcher is a **two-component system** that reduces Lifecycle Manager's reconciliation load by forwarding only meaningful resource changes from SKR clusters back to KCP. Instead of periodic polling, LKM reacts to real events.

```
SKR cluster                             KCP cluster
┌───────────────────────────┐           ┌────────────────────────────────────┐
│  runtime-watcher          │  mTLS     │  listener (library)                │
│  (ValidatingWebhook)      │──────────►│  SKREventListener HTTP server      │
│  watches configured       │           │  emits GenericEvent on channel     │
│  resources, detects       │           │  → controller-runtime              │
│  spec/status changes      │           │    reconcile queue                 │
│  always allows admission  │           │                                    │
└───────────────────────────┘           └────────────────────────────────────┘
```

The webhook **always allows** admission requests (`Allowed: true`) — it uses the admission flow purely as a change-detection mechanism, not for enforcement.

## Repo structure — three Go modules

| Directory | Module | Role |
|---|---|---|
| `listener/` | `github.com/kyma-project/runtime-watcher/listener` | **Library** consumed by KCP operators (Lifecycle Manager) |
| `runtime-watcher/` | `github.com/kyma-project/runtime-watcher/skr` | **Binary** deployed as a ValidatingWebhook on each SKR |
| `runtime-watcher/tests/` | `github.com/kyma-project/runtime-watcher/tests` | Shared e2e test suite (requires real KCP + SKR clusters) |

Run `go` and `make` commands from inside the module directory, not from the repo root. The root `Makefile` only orchestrates linting across all modules. Tool versions (golangci-lint, envtest, k8s) are centralized in `versions.yaml`.

Each module has its own release cycle with distinct tag formats: `v1.2.3` for the SKR webhook, `listener/v1.2.3` for the listener library.

## Make targets

### Root `Makefile`

| Target | What it does |
|---|---|
| `make lint-all` | Lint both modules |
| `make lint-runtime-watcher` | Lint only `runtime-watcher/` |
| `make lint-listener` | Lint only `listener/` |
| `make bump-go-version GO_VERSION=x.y.z` | Bump Go version across all modules |

### `runtime-watcher/`

| Target | What it does |
|---|---|
| `make build` | Compile webhook binary with version ldflags (`GOFIPS140=v1.0.0`) |
| `make test` | `fmt` + `vet` + envtest unit tests (excludes `tests/` dir) |
| `make lint` | golangci-lint |
| `make run` | Run webhook locally |
| `make docker-build` / `make docker-push` | Container image lifecycle |

### `listener/`

| Target | What it does |
|---|---|
| `make test` | `fmt` + `vet` + unit tests with race detector (`GOFIPS140=v1.0.0`) |
| `make lint` | golangci-lint |
| `make resolve` | `go mod tidy` |
| `make build-verbose` | Build with verbose output |

### Running a single test

```sh
# listener
cd listener && GOFIPS140=v1.0.0 go test -run TestFoo ./pkg/...

# runtime-watcher
cd runtime-watcher && KUBEBUILDER_ASSETS=$(../bin/setup-envtest use 1.32.0 -p path) \
  GOFIPS140=v1.0.0 go test -run TestFoo `go list ./... | grep -v /tests/`
```

## How it works

1. KLM deploys the webhook as a `ValidatingWebhookConfiguration` + Deployment on each SKR (failure policy: `Ignore`).
2. When a watched resource changes on SKR, Kubernetes calls the webhook via admission.
3. For UPDATE operations, the webhook compares old vs new `.spec` or `.status` and only fires if there's an actual change. CREATE and DELETE are always forwarded.
4. The webhook sends an HTTP POST to KCP (`/<KCP_CONTRACT>/<moduleName>/event`) over mTLS using per-SKR client certificates.
5. On KCP, the Istio Gateway terminates TLS, VirtualServices route to the listener, and the listener puts events on a Go channel consumed by the Kyma controller.
6. Module name is extracted from the incoming webhook URL path: `/validate/<moduleName>`.

## Watcher CR

The [Watcher CR](https://github.com/kyma-project/lifecycle-manager/blob/main/api/v1beta2/watcher_types.go) is defined in **Lifecycle Manager** (not this repo). It drives both the `ValidatingWebhookConfiguration` on SKR and the `VirtualService` routing on KCP. Key fields: `spec.resourceToWatch` (GVK), `spec.labelsToWatch` (optional label filter), `spec.field` (`spec` or `status`), `spec.manager` (URL path segment).

## listener — key API surface

The listener package is consumed by Lifecycle Manager. Its public API:

```go
// Create and start the listener (implements controller-runtime Runnable)
l := event.NewSKREventListener(addr, componentName)
mgr.Add(l)

// Receive events in a controller watch
source.Channel(l.ReceivedEvents(), &handler.EnqueueRequestForOwner{...})
```

**`types.WatchEvent`** — the event payload forwarded from SKR:

```go
type WatchEvent struct {
    Watched    ObjectKey               // {Namespace, Name} of the watched resource
    WatchedGvk metav1.GroupVersionKind // GVK of the watched resource
    SkrMeta    SkrMeta                 `json:"-"` // populated by listener from XFCC header
}
```

The listener serves HTTP on `/v2/{componentName}/event`. It extracts the SKR's runtime ID from the `X-Forwarded-Client-Cert` (XFCC) header injected by Istio — this is how the KCP side identifies which SKR sent the event.

See [Lifecycle Manager's usage](https://github.com/kyma-project/lifecycle-manager/blob/main/internal/controller/kyma/setup.go) for a concrete integration example.

## runtime-watcher (SKR webhook) — key packages

| Package | Purpose |
|---|---|
| `pkg/admissionreview` | HTTP handler: receives `AdmissionReview`, detects spec/status changes, sends `WatchEvent` to KCP via mTLS |
| `pkg/serverconfig` | Reads all config from environment variables |
| `pkg/cacertificatehandler` | Loads CA cert into `x509.CertPool` for mTLS |
| `pkg/requestparser` | Deserializes `AdmissionReview` from HTTP body |
| `pkg/watchermetrics` | Prometheus metrics (request duration, KCP requests, FIPS mode) |

**Configuration via environment variables** (set by Lifecycle Manager when deploying the webhook):

| Env var | Default | Purpose |
|---|---|---|
| `WEBHOOK_PORT` | `8443` | Webhook HTTPS port |
| `METRICS_PORT` | `2112` | Prometheus metrics port |
| `KCP_ADDR` | — | KCP gateway address |
| `KCP_CONTRACT` | — | API contract path prefix |
| `CA_CERT` / `TLS_CERT` / `TLS_KEY` | — | Certificate paths |

The binary only accepts two CLI flags: `--version` and `--development`. Everything else is via environment variables.

## Security constraints

Do not modify the following without a security review — these are active CVE mitigations or compliance requirements:

- **TLS 1.3 only**: `MinVersion: tls.VersionTLS13` and `MaxVersion: tls.VersionTLS13` in the webhook server TLS config (`runtime-watcher/main.go`). Do not lower `MinVersion` or add protocol negotiation that could downgrade the connection.
- **`GOFIPS140=v1.0.0`**: All builds and test runs use the FIPS Go module. Never remove this from Makefiles or the Dockerfile.
- **mTLS mutual authentication**: The SKR webhook authenticates to KCP via mutual TLS. Do not bypass certificate validation (`InsecureSkipVerify`) in non-test code.
- **Retry via `pester`**: `github.com/sethgrid/pester` provides exponential backoff for KCP requests. Replacing it with a bare `for` loop risks thundering-herd amplification under KCP load.

## Code conventions

Go nolint policy, FIPS, and TLS constraint rules load automatically when editing `.go` files — see [`.claude/rules/go-conventions.md`](.claude/rules/go-conventions.md).

The full linter config is in `.golangci.yaml` — check it before adding any `//nolint` directive. Every `//nolint` **must** include an explanation comment. Bare suppressions fail CI.

Key limits enforced by golangci-lint:
- **Import ordering** (gci): standard → third-party → project (`github.com/kyma-project`)
- **Line length**: 120 chars | **Function length**: 80 lines / 60 statements | **Cyclomatic complexity**: 20

## Commits and Pull Requests

- PRs are merged with **squash merge** — the PR title and description form the commit message.
- Follow [conventional commits](https://www.conventionalcommits.org/), enforced by `.github/workflows/lint-conventional-prs.yml`.
- PR title format: `<type>: <title>` where the title is one sentence explaining the reason for the changeset.
- Types: `deps`, `chore`, `docs`, `feat`, `fix`, `refactor`, `test`.
- Never mention Claude or any AI agent in commits or PRs (no author attribution, no `Co-Authored-By`, no references in commit messages).

## Documentation

Detailed docs in `docs/`:
- `architecture.md` — full system architecture with certificate rotation process
- `listener.md` — how to integrate the listener package in a KCP operator
- `watcher-setup-guide.md` — Watcher CR configuration and event consumption guide
- `api.md` — Watcher CR API reference

## Model usage

Follow the Kyma team's Claude Code workflow:

- **Planning complex tasks** — switch to Opus: `/model claude-opus-4-8`
- **Implementation** — use the default Sonnet: `/model claude-sonnet-4-6`

Use Opus when you need to understand an unfamiliar subsystem, design a non-trivial change, or reason about cross-cutting impacts (e.g. listener API changes that affect Lifecycle Manager). Switch back to Sonnet once the approach is clear.
