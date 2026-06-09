## Why

The dependency audit found a reachable `golang.org/x/net` vulnerability in the nested `tools/goplint` module while the current CI vulnerability job only scans the root module. Several low-risk root direct dependency updates are also available and should be refreshed together with a verification path that keeps tool dependency churn visible.

## What Changes

- Update the nested `tools/goplint` module to resolve the reachable `golang.org/x/net` vulnerability fixed in `v0.55.0`.
- Batch the root direct same-path dependency updates identified in the audit: Bubble Tea, ACP SDK, go-internal, `x/sys`, and `x/term`.
- Extend vulnerability scanning so CI and local audit guidance cover both Go modules: the root module and `tools/goplint`.
- Keep deprecated or noisy transitive modules as audit findings unless the dependency graph can remove them through ordinary upgrades without broad migrations.
- Preserve tidy module files and run focused verification for TUI, ACP, CLI/testscript, cross-platform syscall, and nested goplint surfaces.

## Capabilities

### New Capabilities
- `go-dependency-maintenance`: Defines the contract for Go dependency refreshes, nested-module vulnerability coverage, and dependency-audit verification.

### Modified Capabilities
- None.

## Impact

- Root `go.mod` / `go.sum` and nested `tools/goplint/go.mod` / `tools/goplint/go.sum`.
- CI vulnerability scanning in `.github/workflows/ci.yml`.
- Agent-facing dependency-audit/version-pinning guidance under `.agents/`.
- Verification commands for `make test`, `make lint`, nested `govulncheck`, and targeted TUI/ACP/CLI checks.
