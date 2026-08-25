# Upgrade Toolchain to Go 1.27

## Why

Go 1.27 (released August 2026) ships language changes (generic methods, struct-selector literal keys), a json/v2-backed `encoding/json`, and size-specialized small-object allocation that directly affect invowk's hot paths and goplint's analyzed corpus. The entire tool ecosystem caught up within days of the release (x/tools v0.49.0, golangci-lint v2.13.1, go-mutesting v2.8.2), so the upgrade window is open now. Staying on 1.26.5 leaves performance on the table and, more importantly, leaves goplint's behavior on new Go 1.27 method shapes untested — a latent soundness gap for a value-type analyzer whose core subject is method/constructor shapes.

## What Changes

- Bump `go 1.26.5` → `go 1.27.x` in both Go modules (root and `tools/goplint`) in lockstep.
- Bump `build/bencher/Dockerfile` base image `golang:1.26-bookworm` → `golang:1.27-bookworm` in the same commit (the image sets `GOTOOLCHAIN=local`, so a go.mod-only bump hard-fails the benchmark image build).
- Upgrade toolchain-coupled tools: golangci-lint v2.12.2 → v2.13.1 (tool directive, `scripts/golangci-lint.sh` pin, test fixtures, version-pinning rule), go-mutesting v2.7.5 → v2.8.2, `golang.org/x/tools` v0.47.0 → v0.49.0 (both modules).
- Regenerate toolchain-bound goplint artifacts: race-repeat timing census (`spec/goplint-test-timings.v1.json` — exact-match on `runtime.Version()`), benchmark threshold manifests (`bench/*.toml` × 3, prefix-checked `go1.26` → `go1.27`), clean-tree evidence record, and the six `_test.go` fixture files carrying `go1.26.x` literals.
- Regenerate `default.pgo` and refresh Bencher-facing benchmark expectations (Go 1.27's size-specialized malloc changes allocation costs on the CUE-parse/discovery hot path).
- Resolve new `modernize`/lint findings surfaced by the lang-version retarget (both `.golangci.toml` files defer to go.mod via `go = ""`).
- Add goplint fixture coverage for Go 1.27 generic methods (methods with type-parameter lists) so analyzer behavior on the new method shape is pinned by test, not by accident.
- Documentation sweep: README (×2 spots), website installation page + pt-BR i18n parity, `AGENTS.md`/`.claude/CLAUDE.md`, `.agents/rules/commands.md`, `.agents/rules/version-pinning.md`, skills mentioning Go 1.26 or `golang:1.26` exemplars, issue template placeholder, website Snippet `dependencies.ts` version examples (including the numeric `-ge 26` threshold), `invowkfile_schema.cue` example comment + paired behavioral sync fixture.
- Adopt `encoding/json/v2` explicitly for integrity-sensitive JSON decode paths — goplint evidence records, execution plans, timing censuses, and baselines — gaining duplicate-key and invalid-UTF-8 rejection as tamper-resistance (the module lock file is CUE, so CUE unification already covers it); evaluate v2 for the interactive-protocol decode hot path with its case-sensitive field-matching difference handled explicitly.
- Add goroutine-leak assertions (Go 1.27 GA `goroutineleak` profile) to serverbase/sshserver/tuiserver lifecycle tests, asserting zero leaked goroutines after `Stop()`.
- Apply `strings.CutLast`/`bytes.CutLast` at the ~11 production `LastIndex` slicing sites where it simplifies (with the modernize sweep).
- Replace `github.com/arnodel/golua v0.2.0` with the patched fork `github.com/invowk/golua v0.3.0`: Go 1.27 removed `runtime.memhash64`/`nilinterhash`/`noescape` from the `//go:linkname` allowlist, breaking linking for every golua-dependent binary (including `go install` users); the fork rewrites its hash functions onto `hash/maphash.Comparable`.
- **Evaluated and dropped**: stdlib `uuid` adoption (`google/uuid` is only an indirect dependency; no production usage) and `testing/synctest.Sleep` (no synctest usage exists in the repo).

## Capabilities

### New Capabilities

- `go-toolchain-upgrades`: Requirements for upgrading the repository's Go toolchain safely and adopting its new capabilities — lockstep module bumps, toolchain-pinned image pairing, regeneration (not hand-editing) of toolchain-bound performance/timing/evidence artifacts, analyzer fixture coverage for new language constructs, documentation/example parity, strict JSON decoding for integrity-sensitive records, and goroutine-leak assertions for server lifecycles.

### Modified Capabilities

- `virtual-lua-interpreter`: the "Built-in Lua runtime" requirement now names `github.com/invowk/golua` (patched fork) instead of `github.com/arnodel/golua`, forced by Go 1.27's linkname allowlist removal.

<!-- lint-tooling-quality-gates already requires exact-version golangci-lint normalization and documentation sync; this change complies with that spec rather than modifying it. goplint-analysis-soundness requirements are unchanged; the generic-method fixture obligation is captured in the new capability. -->

## Impact

- **Build**: `go.mod` (root + `tools/goplint`), `build/bencher/Dockerfile`, tool directives.
- **Lint**: `scripts/golangci-lint.sh`, `scripts/test_golangci_lint.sh` fixtures, `.agents/rules/version-pinning.md`; expect new modernize findings across `cmd/`, `internal/`, `pkg/`, `tools/goplint/`.
- **Goplint gates**: `tools/goplint/bench/*.toml`, `tools/goplint/spec/goplint-test-timings.v1.json`, `tools/goplint/testdata/gates/clean-tree-run.v4.json`, six test files with `go1.26.x` literals; execution-plan digests change (they embed `runtime.Version()`). Soundness routing: this diff touches harness- and analyzer-semantics-owned files, so the authoritative CI tier is `semantic`; a completion claim requires the full evidence regeneration sequence.
- **Performance**: `default.pgo` regeneration; benchmark numbers shift due to size-specialized malloc; Bencher thresholds may need review.
- **Behavioral risk**: `encoding/json` v1 is now backed by json/v2 — marshal output is at parity but decoder error strings may differ; tests asserting exact JSON error text must be audited (`GOEXPERIMENT=nojsonv2` is the temporary escape hatch). Explicit json/v2 adoption changes field matching to case-sensitive and rejects duplicate keys/invalid UTF-8 — previously-accepted malformed inputs will now fail on converted paths (intended for evidence records; must be deliberate for protocol paths). `compress/flate` output bytes differ from 1.26, so release archive checksums will not match 1.26-built artifacts.
- **Servers/tests**: serverbase/sshserver/tuiserver lifecycle tests gain leak assertions; latent goroutine leaks they surface must be fixed or triaged, not suppressed.
- **Docs**: ~20 documentation-class touches with i18n and snippet-parity gates (`scripts/validate-docs-parity.mjs`, `make check-agent-docs`).
