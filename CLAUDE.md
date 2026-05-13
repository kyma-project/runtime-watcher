# CLAUDE.md

Runtime Watcher is a validation webhook deployed by [Lifecycle Manager](https://github.com/kyma-project/lifecycle-manager) (KLM) onto SAP Kyma Runtime (SKR) clusters.
It detects changes to watched Kubernetes resources on the SKR and notifies the Kyma Control Plane (KCP) so that Lifecycle Manager can reconcile immediately instead of polling.
The webhook always allows admission requests (`Allowed: true`) — it uses the admission flow purely as a change-detection mechanism, not for enforcement.

The repository contains two components:
- **runtime-watcher** — the SKR webhook binary: a standalone HTTP server (not a controller-runtime operator) that receives `AdmissionReview` requests, detects spec/status changes, and sends `WatchEvent` notifications to KCP over mTLS.
- **listener** — a Go library consumed by KCP-side operators (like Lifecycle Manager) to receive and route `WatchEvent` payloads. Implements controller-runtime's `Runnable` interface.

## How it works

1. KLM deploys the webhook as a `ValidatingWebhookConfiguration` + Deployment on each SKR (failure policy: `Ignore`).
2. When a watched resource changes on SKR, Kubernetes calls the webhook via admission.
3. For UPDATE operations, the webhook compares old vs new `.spec` or `.status` and only fires if there's an actual change.
4. The webhook sends an HTTP POST to KCP (`/v2/<manager>/event`) over mTLS using per-SKR client certificates.
5. On KCP, the Istio Gateway terminates TLS, VirtualServices route to the listener, and the listener puts events on a Go channel consumed by the Kyma controller.

## Multi-Module Structure

Two independent Go modules (no local `replace` directives — they reference each other via versioned imports):

- **`runtime-watcher/`** (`github.com/kyma-project/runtime-watcher/skr`) — the webhook binary
- **`listener/`** (`github.com/kyma-project/runtime-watcher/listener`) — the KCP-side library

Each has its own `go.mod`, `Makefile`, and release cycle (different tag formats: `v1.2.3` vs `listener/v1.2.3`). Tool versions are centralized in `versions.yaml`.

## Build and Test Commands

The project uses `GOFIPS140=v1.0.0 go` for all Go commands (FIPS-enabled builds).

**Root Makefile:**
- `make lint-all` — lint both modules
- `make lint-runtime-watcher` / `make lint-listener` — lint individually

**runtime-watcher/Makefile:**
- `make build` — FIPS-enabled build to `bin/webhook`
- `make test` — fmt + vet + unit/integration tests (excludes `tests/` dir)
- `make lint` — golangci-lint

**listener/Makefile:**
- `make test` — fmt + vet + go test with race detector
- `make lint` — golangci-lint

**E2E tests** (`runtime-watcher/tests/e2e/`):
- Require real KCP + SKR clusters (`KCP_KUBECONFIG`, `SKR_KUBECONFIG`)
- `make watcher-enqueue` / `make watcher-metrics`

## Key Packages

**runtime-watcher:**
| Package | Purpose |
|---------|---------|
| `pkg/admissionreview` | HTTP handler: receives admission reviews, detects changes, sends WatchEvents to KCP |
| `pkg/serverconfig` | Environment-variable-based configuration (`KCP_ADDR`, `KCP_CONTRACT`, certs, ports) |
| `pkg/cacertificatehandler` | Loads CA cert into `x509.CertPool` for mTLS |
| `pkg/requestparser` | Deserializes `AdmissionReview` from HTTP body |
| `pkg/watchermetrics` | Prometheus metrics (request duration, KCP requests, FIPS mode) |

**listener:**
| Package | Purpose |
|---------|---------|
| `pkg/v2/event` | `SKREventListener`, WatchEvent unmarshaling, GenericEvent conversion |
| `pkg/v2/types` | Core types: `WatchEvent`, `GenericEvent`, `SkrMeta`, `ObjectKey` |
| `pkg/v2/certificate` | Extracts x509 certs from XFCC header (injected by Istio) |
| `pkg/metrics` | Listener-side Prometheus metrics |

## Code Conventions

Follow the [Google Go Style Guide](https://google.github.io/styleguide/go/) as a baseline.

Project-specific rules enforced by `golangci-lint` (see `.golangci.yaml`):
- **Import ordering** (gci): standard → third-party → project (`github.com/kyma-project`)
- **Line length**: 120 chars | **Function length**: 80 lines / 60 statements | **Cyclomatic complexity**: 20
- **All linters enabled by default** — check `.golangci.yaml` before adding `//nolint`
- Configuration is via **environment variables**, not CLI flags (only `--version` and `--development` flags exist)

## Commits and Pull Requests

- PRs are merged with squash merge, so the PR title and description form the commit message.
- Follow [conventional commits](https://www.conventionalcommits.org/) — enforced by `.github/workflows/lint-conventional-prs.yml`.
- PR Title format: `<type>: <title>`, where title is one sentence explaining the reason for the changeset.
- Types: `deps`, `chore`, `docs`, `feat`, `fix`, `refactor`, `test`.
- Never mention Claude or any AI agent in commits or PRs (no author attribution, no Co-Authored-By, no references in commit messages).
