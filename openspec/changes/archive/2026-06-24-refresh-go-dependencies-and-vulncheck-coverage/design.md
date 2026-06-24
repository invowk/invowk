## Context

Invowk has two Go modules: the root module and the nested `tools/goplint` module. The dependency audit found no root-module vulnerabilities, but `govulncheck ./...` from `tools/goplint` reports a reachable `GO-2026-5026` finding through `golang.org/x/net@v0.54.0`, fixed in `v0.55.0`.

The root module also has five direct same-path updates available: `charm.land/bubbletea/v2`, `github.com/coder/acp-go-sdk`, `github.com/rogpeppe/go-internal`, `golang.org/x/sys`, and `golang.org/x/term`. The broader root graph has many transitive updates because root `go.mod` intentionally pins Go tool dependencies such as golangci-lint and go-mutesting. Those transitive updates should not be treated as a blanket product dependency refresh.

## Goals / Non-Goals

**Goals:**
- Fix the reachable nested-module `golang.org/x/net` vulnerability.
- Refresh the five identified root direct same-path dependency updates.
- Make vulnerability scanning cover every repository Go module in local guidance and CI.
- Keep the module graph tidy and preserve current runtime, TUI, ACP, CLI, and goplint behavior.

**Non-Goals:**
- Migrate OpenAI SDK major module paths.
- Update go-mutesting or mutation-output semantics.
- Sweep all transitive dependencies with `go get -u ./...`.
- Remove all deprecated transitive modules unless ordinary bounded updates remove them naturally.

## Decisions

### Fix the nested security issue before broader cleanup

Update `tools/goplint` so its selected `golang.org/x/net` version is at least `v0.55.0`, then rerun `govulncheck ./...` from the nested module. This directly addresses the only reachable vulnerability found by the audit without depending on unrelated root graph movement.

Alternative considered: run a repository-wide `go get -u all`. This would churn the tool graph, lint dependencies, cloud-provider transitives, and deprecated modules without a clear product benefit.

### Batch only root direct same-path updates

Apply the five root direct updates explicitly. This keeps review scope understandable and ties verification to affected surfaces:
- Bubble Tea: TUI and interactive tests.
- ACP SDK: ACP client tests and fake-agent lifecycle tests.
- go-internal: CLI testscript suite.
- `x/sys` and `x/term`: platform and terminal-sensitive tests.

Alternative considered: update every module reported by `go list -m -u all`. The audit showed 203 root updates, most transitive, including tool dependencies and major upgrades unrelated to Invowk runtime behavior.

### Add a shared vulnerability-scan path for all Go modules

Add a local target or script that discovers Go modules with `git ls-files 'go.mod' '*/go.mod'`, then runs `govulncheck ./...` in each module. CI should invoke that shared path instead of hard-coding only the root module. Failures should identify the module that failed so nested findings are actionable.

Alternative considered: duplicate a second CI step for `tools/goplint`. That would fix the immediate gap but repeat module knowledge in workflow YAML and age poorly if another nested module is added.

### Keep deferred dependency findings visible

Deprecated or noisy transitive modules should remain visible in dependency-audit reporting. Removing them should happen only when bounded direct/tool updates naturally remove them or when a separate proposal owns the migration.

## Risks / Trade-offs

- ACP SDK patch removes unstable `session/set_model` symbols -> Mitigate by confirming Invowk does not use those symbols and running ACP client tests.
- Terminal dependency updates can change platform behavior -> Mitigate with TUI, terminal, and cross-platform relevant tests.
- go-internal testscript changes can alter CLI test semantics -> Mitigate with the CLI testscript suite.
- Adding nested govulncheck to CI can expose existing nested-only issues -> Mitigate by fixing the known `tools/goplint` issue before enabling the broader gate.

## Migration Plan

1. Update `tools/goplint` `golang.org/x/net` to `v0.55.0` or newer and tidy the nested module.
2. Apply the five explicit root direct dependency updates and tidy the root module.
3. Add the shared all-modules vulnerability scan target or script and route CI through it.
4. Update agent-facing dependency-audit or command guidance if command names change.
5. Verify with nested and root `govulncheck`, module tidiness, `make test`, `make lint`, and targeted TUI/ACP/CLI checks.

Rollback is straightforward: revert the module updates and CI scan wiring together if validation exposes an unacceptable compatibility issue.

## Open Questions

None. Implementation should choose the smallest shared command shape that fits existing Makefile and script patterns.
