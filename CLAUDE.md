# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

runtime-watcher is a **two-component system** that reduces Kyma Lifecycle Manager (KLM) reconciliation load by forwarding only meaningful resource changes from SAP BTP, Kyma Runtime (SKR) clusters back to the central control plane (KCP). Instead of periodic polling, KLM reacts to real events.

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
| `listener/` | `github.com/kyma-project/runtime-watcher/listener` | Go module, a **Library** used by KCP operators (Lifecycle Manager) |
| `runtime-watcher/` | `github.com/kyma-project/runtime-watcher/skr` | Go project for creating a **Binary** deployed as a ValidatingWebhook on each SKR cluster |
| `runtime-watcher/tests/` | `github.com/kyma-project/runtime-watcher/tests` | Shared e2e test suite (requires real KCP + SKR clusters) |

Run `go` and `make` commands from inside the module directory, not from the repo root. The root `Makefile` only orchestrates linting across all modules. Each module has its own release cycle with distinct tag formats: `v1.2.3` for the SKR webhook, `listener/v1.2.3` for the listener library.

## Build and test

From `runtime-watcher/`: `make build`, `make test`, `make lint`
From `listener/`: `make test`, `make lint`
From repo root: `make lint-all`

E2E tests (`runtime-watcher/tests/e2e/`) require real KCP and SKR clusters — set `KCP_KUBECONFIG` and `SKR_KUBECONFIG`. All builds and test runs require `GOFIPS140=v1.0.0`.

## How it works

1. KLM deploys the webhook as a `ValidatingWebhookConfiguration` + Deployment on each SKR cluster (failure policy: `Ignore`).
2. When a watched resource changes on SKR, Kubernetes calls the webhook via admission.
3. For UPDATE operations, the webhook compares old vs new `.spec` or `.status` and only fires if there's an actual change. CREATE and DELETE are always forwarded.
4. The webhook sends an HTTP POST to KCP (`/<KCP_CONTRACT>/<moduleName>/event`) over mTLS using per-SKR client certificates.
5. On KCP, the Istio Gateway terminates TLS, VirtualServices route to the listener, and the listener puts events on a Go channel consumed by the Kyma controller.
6. Module name is extracted from the incoming webhook URL path: `/validate/<moduleName>`.

## Watcher CR

The [Watcher CR](https://github.com/kyma-project/lifecycle-manager/blob/main/api/v1beta2/watcher_types.go) is defined in **Lifecycle Manager** (not this repo). It drives both the `ValidatingWebhookConfiguration` on SKR and the `VirtualService` routing on KCP. Key fields: `spec.resourceToWatch` (GVK), `spec.labelsToWatch` (optional label filter), `spec.field` (`spec` or `status`), `spec.manager` (URL path segment).

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
- **All linters enabled by default** — check `.golangci.yaml` before adding `//nolint`
- **Import ordering** (gci): standard → third-party → project (`github.com/kyma-project`)
- **Line length**: 120 chars | **Function length**: 80 lines / 60 statements | **Cyclomatic complexity**: 20

## Commits and Pull Requests

- PRs are usually created from a **fork branch** against `main`.
- PRs are merged with **squash merge** — the PR title and description form the commit message.
- Follow [conventional commits](https://www.conventionalcommits.org/), enforced by `.github/workflows/lint-conventional-prs.yml`.
- PR title format: `<type>: <title>` where the title is one sentence explaining the reason for the changeset.
- Ask what type to use when creating a PR: `deps`, `chore`, `docs`, `feat`, `fix`, `refactor`, `test`.
- PR description should contain a short summary of the changes and, if applicable, a reference to the issue using the `closes` or `resolves` keyword.
- Never mention Claude or any AI agent in commits or PRs (no author attribution, no `Co-Authored-By`, no references in commit messages).
