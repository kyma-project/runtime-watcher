---
paths:
  - "**/*.go"
---

# Go code conventions — runtime-watcher

`make lint` is the authoritative check. The full linter config is in `.golangci.yaml` (run from within each module directory: `listener/` or `runtime-watcher/`).

## nolint policy

Every `//nolint` directive **must** include an explanation:

```go
//nolint:funlen // webhook handler — acceptable exception
```

Bare suppressions fail CI. Check `.golangci.yaml` before adding any.

## FIPS

Use `GOFIPS140=v1.0.0 go` for any `go` command run directly (the Makefile sets this automatically). Do not add dependencies that bypass the FIPS-approved stdlib crypto.

## TLS — never downgrade

The TLS config in the webhook server contains compliance requirements. When touching TLS code:

- Keep `MinVersion: tls.VersionTLS13` — do not lower
- Keep `MaxVersion: tls.VersionTLS13` — do not raise to allow negotiation of older versions
- Keep mTLS mutual authentication — `InsecureSkipVerify` is not acceptable outside tests

## Retry logic

KCP requests must use `github.com/sethgrid/pester` (exponential backoff). Do not replace with a manual `for` loop — thundering herd under KCP load.
