# Design: Upgrade Toolchain to Go 1.27

## Context

Both Go modules pin `go 1.26.5`. There is no `toolchain` directive, no `GOTOOLCHAIN` in CI, and every `actions/setup-go@v6` step uses `go-version-file:`, so CI follows go.mod automatically. The only toolchain pin outside go.mod is `build/bencher/Dockerfile` (`golang:1.26-bookworm` + `GOTOOLCHAIN=local`), which Dependabot does not watch (no docker ecosystem configured).

The heavyweight part is goplint's soundness machinery, which deliberately binds artifacts to the exact toolchain:

- `tools/goplint/cmd/race-repeat/main.go` requires exact equality between `spec/goplint-test-timings.v1.json`'s `"toolchain"` field and `runtime.Version()`.
- `tools/goplint/internal/benchmarkpolicy/policy.go` and `scripts/check-cfg-bench-thresholds.sh` prefix-check `go_toolchain = "go1.26"` in the three `bench/*.toml` manifests.
- Execution-plan digests and repository-audit output embed `runtime.Version()`; the retained clean-tree evidence (v4) records the `go version` and golangci-lint banners.

Ecosystem readiness (verified August 2026): x/tools v0.49.0 (the Go 1.27-synchronized release), golangci-lint v2.13.1, go-mutesting v2.8.2 are all published.

Go 1.27 runtime/stdlib changes with repo-visible effects: size-specialized small-object allocation (up to 30% cheaper <80-byte allocs — moves every benchmark number), `encoding/json` v1 now backed by json/v2 (error-string text may differ), `compress/flate` output bytes differ (release archive checksums diverge from 1.26 builds), `stdversion` vet check on by default in `go test`, `go mod tidy` merges require blocks for `go 1.27+` modules, tracebacks include pprof labels.

## Goals / Non-Goals

**Goals:**

- Move both modules, the bencher image, and all toolchain-coupled tool pins to Go 1.27 in a single coherent change.
- Regenerate (never hand-edit) every toolchain-bound performance/timing/evidence artifact so certified numbers are re-certified on 1.27.
- Pin goplint's behavior on Go 1.27 generic methods via fixture coverage.
- Keep all documentation, examples, and snippet-parity surfaces synchronized.

**Goals (feature adoption):**

- Adopt `encoding/json/v2` for integrity-sensitive decode paths (goplint evidence/plan/timing/baseline records) to gain duplicate-key and invalid-UTF-8 rejection.
- Add goroutine-leak assertions to serverbase/sshserver/tuiserver lifecycle tests using the GA `goroutineleak` profile.
- Apply `CutLast` and other 1.27 idioms where modernize or manual review shows a simplification.

**Non-Goals:**

- Using generic methods or other Go 1.27 language features in production code.
- Broadening mutation profiles or benchmark scopes beyond what the bump requires.
- Wholesale json/v2 conversion of every marshal/unmarshal site — only integrity-sensitive decodes and clearly-winning hot paths convert; plain marshal-only sites stay on v1 (already v2-backed).
- stdlib `uuid` adoption: `google/uuid` is only an indirect dependency with no production usage — nothing to convert.
- `testing/synctest.Sleep` adoption: the repo has no synctest usage — nothing to simplify.

## Decisions

### D1: Single change, three commit-ready stages (bump → regenerate → docs)

Stage the work as: (1) mechanical version bumps + tool upgrades + lint-finding fixes, (2) artifact regeneration wave, (3) docs sweep. One OpenSpec change rather than three because the stages are not independently shippable: go.mod at 1.27 with 1.26 timing manifests fails CI hard.

*Alternative considered*: separate "bump" and "regenerate" PRs — rejected because the exact-match timing gate makes the intermediate state red.

### D2: `go 1.27.0` (or latest patch at implementation time) with no `toolchain` directive

Keep the existing convention: version via the `go` directive only, `go-version-file:` in CI. Adding a `toolchain` directive would be a new pinning mechanism the repo has deliberately avoided.

### D3: Tool upgrades ride along, pinned exactly

golangci-lint → v2.13.1 via `go get -tool ...@v2.13.1`; update `scripts/golangci-lint.sh` (`GOLANGCI_LINT_VERSION`), the five fixtures in `scripts/test_golangci_lint.sh` (including the cosmetic `go1.26.4` stub string), and `.agents/rules/version-pinning.md`. go-mutesting → v2.8.2 the same way. x/tools → v0.49.0 in both modules. Rationale: these are the releases built against Go 1.27's `go/types`; staying on the old pins risks analysis-time failures on 1.27 syntax/export data.

*Alternative considered*: bump Go only, defer tool upgrades — rejected; x/tools v0.47.0 predates Go 1.27 and cannot be trusted to typecheck it.

### D4: Regenerate toolchain-bound artifacts with their existing Make targets

- Timing census: `make update-goplint-race-repeat-timings` (three-sample weighted).
- Threshold manifests: update `go_toolchain` to `"go1.27"` and re-derive numbers per the manifests' documented refresh process; do not carry 1.26 numbers forward (allocation costs changed).
- PGO: `make pgo-profile-parse-discovery` (or full `make pgo-profile`) with `-pgo=off` training as already implemented.
- Clean-tree evidence: full `make generate-goplint-clean-tree-evidence` — **not** `rebind`, because the toolchain change is semantic-content drift by definition (recorded tool banners change).
- Test fixtures carrying `go1.26.x` literals are hand-updated (they are inputs to unit tests, not certified evidence); keep the deliberate-mismatch case in `plan_model_test.go` a mismatch.

### D5: Generic-method fixture is part of this change

Add goplint testdata exercising a value type with a generic method (type-parameter list on a method declaration) and assert current analyzer classification (including, if applicable, an inconclusive/unsupported outcome — what matters is that behavior is pinned and fail-closed, not that generic methods are fully analyzed). Rationale: goplint's subject matter is method/constructor shapes; a new legal method shape entering the language without fixture coverage is a silent soundness gap. Full semantic support for generic methods, if needed, is future work scoped by what the fixture reveals.

### D6: Audit JSON error-string assertions instead of pinning `nojsonv2`

Grep test suites for assertions on `encoding/json` error text; fix any that break under the v2-backed decoder. Do not set `GOEXPERIMENT=nojsonv2` anywhere (it is a temporary opt-out slated for removal); treat it only as a local triage tool.

### D7: json/v2 adoption targets integrity-sensitive decodes, not the lock file

The module lock file is CUE (`invowkmod.lock.cue`), and CUE unification already rejects conflicting duplicate fields — the originally-floated "lock-file duplicate-key hardening" does not apply. The surfaces where v2 strictness is a genuine integrity win are goplint's JSON record readers (clean-tree evidence, execution plans, timing censuses, baselines, subgate censuses, mutation contracts): under v1 a doctored record with duplicate keys parses last-wins; under v2 it is rejected. Convert those readers to `encoding/json/v2` with strict defaults. For the root module, the interactive-protocol decode path may convert for performance, but only with the v1→v2 semantic differences handled explicitly — v2 field matching is **case-sensitive** by default and rejects duplicate keys/invalid UTF-8, so each converted path needs a deliberate decision (strict is desired for evidence; protocol messages are produced by invowk itself, so strictness is safe but must be verified against the actual message shapes). LLM-response parsing in `internal/auditllm`/`internal/agentcmd` stays on v1 semantics: LLM output is inherently messy and lenient parsing there is a feature.

*Alternative considered*: `GOEXPERIMENT`-level or global v2 opt-in — rejected; per-call-site adoption with explicit options keeps behavior reviewable and avoids surprise strictness on paths that legitimately tolerate loose input.

### D8: Goroutine-leak assertions live in lifecycle tests, using the GA goroutineleak profile

Go 1.27 graduates goroutine-leak profiling (`runtime/pprof` `goroutineleak` profile) to GA. Add assertions to the serverbase state-machine tests and the sshserver/tuiserver lifecycle tests: after `Stop()` reaches a terminal state, capture the goroutineleak profile and assert zero leaked goroutines attributable to the server under test. This turns the serverbase contract ("idempotent stop, race-safe shutdown") into a machine-checked property instead of a convention. Any latent leak the assertions surface is fixed or explicitly triaged with the user — never suppressed with an exclusion list.

*Alternative considered*: goroutine-count diffing (before/after snapshots) — rejected; count diffs are flaky under test parallelism, while the leak profile identifies goroutines blocked on unreachable primitives via GC reachability, which is precise.

### D9: CutLast and 1.27 idiom adoption rides the modernize sweep

The 11 production `strings.LastIndex`/`bytes.LastIndex` slicing sites are reviewed once: where `CutLast` simplifies, rewrite; where the index itself is needed, leave alone. Whatever the retargeted modernize analyzer rewrites automatically is accepted; the manual pass only covers what it misses. No new lint rule is added for this.

### D10: golua is forked to remove disallowed runtime linknames (discovered during implementation)

`github.com/arnodel/golua v0.2.0` obtains Lua-table hash functions via `//go:linkname` to `runtime.memhash64`/`nilinterhash`/`noescape`; Go 1.27 removed those from the linkname allowlist, so every binary depending on golua fails to link. Neither `-ldflags=-checklinkname=0` nor a `replace` directive propagates through `go install github.com/invowk/invowk@version` (a documented install channel), so the fix must live in the dependency graph itself. Resolution (user-approved): fork as `github.com/invowk/golua` (v0.3.0, from upstream v0.2.0) with `runtime/hash.go` rewritten to `hash/maphash.Comparable` (Go 1.24+ supported API for the same runtime hashing machinery; panics on unhashable values like `nilinterhash`; per-process random seed adds hash-flooding resistance). The fork also drops `cmd/golua-repl`, whose `arnodel/edit` dependency pulls an old arnodel/golua pseudo-version that reintroduces the linkname failure. Upstream fix candidate: the same patch can be offered to arnodel/golua; if accepted and released, invowk can return to upstream.

*Alternatives considered*: global `-checklinkname=0` (breaks `go install` users; weakens enforcement), vendoring golua in-repo (large foreign code mass under repo gates), waiting for upstream (stalls the change indefinitely).

### D11: Soundness routing expectation

This diff spans documentation, consumer, harness (timing/threshold manifests, executor-adjacent scripts), and analyzer-adjacent test fixtures. Highest-class-wins routing means the authoritative CI tier is at least `harness` and, with the generic-method fixture touching analyzer testdata, `semantic`. Plan for the full semantic profile in CI and the four-command completion sequence at the end; do not attempt to carve the diff to dodge the tier.

## Risks / Trade-offs

- [Benchmark thresholds mis-set on first try under new malloc behavior] → Derive from fresh 1.27 runs on the reference runner; treat first CI benchmark pass as calibration, adjust manifests from measured data rather than guessing offsets from 1.26 numbers.
- [golangci-lint v2.13.x introduces new/changed linters producing a large finding wave] → Fix findings rather than suppressing; if a rule change is disputed, config changes go through `.golangci.toml` with rationale, consistent with existing lint-tooling spec.
- [modernize retargets to 1.27 idioms (e.g., `strings.CutLast`, new fixers) across many files] → Accept the rewrites; they are mechanical and gofmt-clean, and the lint gate requires zero findings anyway.
- [json/v2-backed decoder changes error text asserted somewhere in tests or golden files] → Targeted grep + full `make test`; fix assertions to be less brittle (error kind, not exact text) where touched.
- [Release archive checksums differ from 1.26 builds due to compress/flate changes] → Expected and harmless (each release is checksummed at build time); note it in the change so nobody mistakes it for tampering.
- [go-mutesting v2.8.x behavior drift affects baselines] → Run `make mutation-dry-run` after the bump; the targeted soundness mutation gate (`make check-goplint-targeted-mutation`) is the blocking signal and permits no baseline, so drift surfaces immediately.
- [`go mod tidy` reformats require blocks under go 1.27 semantics, inflating the diff] → Accept once in the bump commit; it is deterministic and reviewable.
- [pt-BR i18n or snippet parity drifts during docs sweep] → `scripts/validate-docs-parity.mjs` and `make check-agent-docs` gate this; run both before completion.
- [json/v2 case-sensitive field matching silently drops fields a v1 decode accepted case-insensitively] → For each converted decode path, verify field-name casing against the actual producers (goplint writers, protocol emitters) and add a round-trip test; goplint records are self-produced so casing is controlled, but the check must be explicit per site.
- [v2 strictness rejects existing on-disk goplint records written before the change] → Toolchain-bound records are regenerated in this change anyway (D4), so strict readers only ever face freshly written records; verify by running the full gate suite after conversion.
- [Leak assertions surface pre-existing goroutine leaks in servers] → That is the point; fix or triage with the user, never baseline. Budget for this in review — a surfaced leak is in-scope work, not noise.
- [goroutineleak profile interacts with test parallelism (other tests' goroutines visible)] → Assert only on goroutines attributable to the server under test (stack/label matching), or run leak-asserting tests without `t.Parallel()` where attribution is ambiguous.

## Migration Plan

1. Bump commit: go.mods, bencher Dockerfile, tool pins, x/tools, `go mod tidy` both modules, fix lint/test fallout.
2. Feature-adoption commit(s): json/v2 conversion of goplint record readers and selected root-module decode paths; goroutine-leak assertions + any leak fixes; CutLast/idiom pass. Before regeneration, so regenerated evidence is produced and re-read by the strict readers.
3. Regeneration commit(s): timings, thresholds, PGO profile, clean-tree evidence (evidence last — it binds the finished tree).
4. Docs commit: prose/example sweep + parity gates.
5. Completion sequence: `make check-goplint-soundness-semantic` → `make generate-goplint-clean-tree-evidence` → `make check-goplint-clean-tree-evidence` → `make check-goplint-soundness-complete`, plus `make test`, `make lint`, `make check-baseline`.

Rollback: revert the squash-merged commit; all regenerated artifacts revert with it (they are tree-bound, so partial rollback is never valid).

## Open Questions

- Whether the generic-method fixture reveals that goplint hard-errors (rather than classifying or reporting inconclusive) on generic methods — if so, the fix scope (fail-closed handling in the analyzer) may grow and should be triaged with the user per the checklist's goplint-triage rule.
- Exact Go patch version at implementation time (use latest 1.27.x available).
