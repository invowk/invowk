# Design: Extract goplint into invowk/goplint

## Context

`tools/goplint` is a nested Go module (`github.com/invowk/invowk/tools/goplint`) with zero Go import edges to the root module. All coupling is process invocation, file paths, and manifests. goplint fuses two roles:

1. **Analyzer applied to invowk's code** — the consumer gates (`check-types*`, baseline, exceptions, full-scan, performance smoke).
2. **Soundness-assurance harness whose workspace under proof is the invowk repository** — workspace digests over the whole invowk tree, clean-tree v4 evidence bound to invowk git objects, ownership routing over invowk paths, docs-guard anchored to invowk's root Makefile.

Post-split, role (1) stays in invowk against a pinned goplint; role (2) moves wholesale and re-roots on the goplint repository's own tree.

Reviewed decisions: preserve history via `git filter-repo`; consume via root `go.mod` tool directive; performance gating on both sides; public repository; required status checks added to both repositories; goplint specs governed by docs-guard only; SonarCloud onboarded at bootstrap.

## Goals / Non-Goals

- **Goal**: zero invowk gate loss at every commit on either `main`.
- **Goal**: goplint becomes independently versioned, releasable, and consumable.
- **Non-goal**: changing any analyzer diagnostic semantics, baseline policy, exception policy, or semantic population.
- **Non-goal (deferred)**: consolidating the consumer flows into a single `goplint audit` subcommand — compatible follow-up in the goplint repository.

## Consumption model (invowk)

Root `go.mod` gains `tool` entries plus an exact `require` pin, extending the existing golangci-lint/go-mutesting pattern:

- `github.com/invowk/goplint` (analyzer; `main.go` at module root)
- `github.com/invowk/goplint/cmd/repository-audit`
- `github.com/invowk/goplint/cmd/subgate-report`
- `github.com/invowk/goplint/cmd/benchmark-policy`

`build-goplint` becomes `go build -o bin/goplint github.com/invowk/goplint`. A new `scripts/goplint.sh` mirrors `scripts/golangci-lint.sh`: resolve via `go tool -n`, assert the embedded module version via `go version -m`. The `repository-audit` input binding keeps digesting `bin/goplint`; the digest is deterministic per (module version, toolchain) pair. Dependabot's existing `gomod /` entry manages the pin; goplint's dependency graph becomes visible to invowk's `govulncheck-all.sh`.

Rejected: `go run module@version` (pin outside go.mod, no dependabot/go.sum coverage); released binaries as the primary path (adds release latency to analyzer fixes and per-OS download plumbing; goplint may still publish releases for other consumers).

## Invowk configuration home: `.goplint/`

Invowk-owned data relocating out of the goplint tree:

- `.goplint/baseline.toml` — invowk finding IDs (from `tools/goplint/baseline.toml`)
- `.goplint/exceptions.toml` — invowk patterns, plus the new `[test_home_env]` key re-enabling the testutil HOME-env rule (`testutil_packages = ["github.com/invowk/invowk/internal/testutil"]`)
- `.goplint/consumer-smoke.github-ubuntu-x64-4cpu.toml` — invowk-calibrated `[repository_full_scan]` limits
- `.goplint/consumer-gate.v1.json` — reduced manifest containing only the consumer subgates (`repository-audit`, `baseline`, `exceptions`, `full-scan`, consumer smoke), `working_directory: "."`, validated against the schema goplint publishes
- `.goplint/ownership.v1.json` — reduced two-class routing manifest: documentation paths skip, everything else and any unmatched path fails closed to `consumer`

`.goplint/` is preferred over `tools/goplint-config/` because it reads as consumer configuration and avoids stale `tools/goplint` matches in path-filter regexes being rewritten anyway.

## Invowk gate mapping

| Gate today | Post-split |
|---|---|
| `build-goplint` | `go build -o bin/goplint github.com/invowk/goplint` + `scripts/goplint.sh` version assert |
| `check-types{,-json,-all,-all-json}`, `update-baseline` | unchanged shape; `-config=.goplint/exceptions.toml`, `-update-baseline=.goplint/baseline.toml` |
| `check-baseline`, `check-goplint-exceptions`, `check-goplint-full-scan`, `check-goplint-repository-audit` | `go tool repository-audit -root . -analyzer bin/goplint -baseline .goplint/baseline.toml -exceptions .goplint/exceptions.toml -semantic-manifest .goplint/consumer-gate.v1.json -packages "./cmd/...,./internal/...,./pkg/..."` + `go tool subgate-report -root . -manifest .goplint/consumer-gate.v1.json -observation ...` |
| `check-goplint-performance-smoke` | new invowk-owned `scripts/goplint-consumer-smoke.sh` (port of the current script, invowk-relative paths, `go tool benchmark-policy`) |
| routed pre-commit (`goplint-behavior`) | new `scripts/goplint-consumer-routed.sh` + `make check-goplint-consumer-routed`; hook stays `always_run`. Local behavior identical — the hook is already consumer-capped today |
| `lint.yml` plan/worker/aggregate | replaced by single job **"goplint consumer gates"** (`go-version-file: go.mod`): audit + baseline + exceptions + full-scan + smoke; becomes a required status check |
| `lint-tools-goplint` and friends | removed; `make lint` is root-only; identical targets recreated in the goplint repository |
| `check-windows-build.sh` steps 3–4 | move to goplint repository CI |
| shellcheck list | drops 10 goplint scripts, adds `scripts/goplint{,-consumer-smoke,-consumer-routed}.sh` |
| all ~30 harness targets (`check-goplint-soundness*`, clean-tree, oracles, fuzz, race-repeat, mutation-kernel, docs-guard, ...) | goplint repository Makefile, rooted at `.` |
| mutation goplint profile (`tools/mutation/goplint-*`, baseline, workflow arm) | goplint repository; invowk keeps the root profile only |
| license-check / file-length / check-agent-docs / sonar / CodeQL / govulncheck | invowk scope shrinks naturally; goplint repository gains identical own gates (CodeQL is an explicit action item — silent loss otherwise); govulncheck covers goplint twice (tool pin in invowk's graph + goplint's own gate) |
| `//goplint:` directives (677 across 182 invowk files) | untouched comments, interpreted by the pinned analyzer |

## goplint repository design

- Extracted tree at repo root; module `github.com/invowk/goplint`; own `Makefile`, `.pre-commit-config.yaml`, `.golangci.toml` (already module-local), LICENSE (MPL-2.0), `AGENTS.md`, dependabot (`gomod /` + `github-actions`), Safety-equivalent ruleset (signatures, linear history, squash-only) plus required check `goplint soundness aggregate` (the worker contexts are matrix-dynamic; the aggregate is the stable context).
- **CI**: `lint.yml` with the distributed plan/worker/aggregate executor moved verbatim, routed by **ownership manifest v3** (goplint-repo-relative rules, three classes documentation < harness < analyzer-semantics, fail-closed to semantic); `ci.yml` (module tests, windows cross-build, license-check, file-length, shellcheck, agent-docs); `goplint-fuzz.yml` moved intact; `mutation.yml` with goplint's own go-mutesting tool pin; new `codeql.yml`; `reference-corpus-certification.yml` — checks out `invowk/invowk` at a pinned ref (`testdata/corpus/invowk.ref`) and runs full-scan + five-sample performance certification with corpus-keyed limits.
- **Clean-tree evidence v5**: base-commit ancestry in the goplint DAG, tree digests via ownership v3, path selection spanning the goplint tree, command list = the goplint repository's own gate suite. The `complete` profile references v5; v4-shaped records are rejected with a migration notice (established format-bump mechanism).
- **docs-guard re-anchored**: governed docs (`docs/goplint/*.md`) move to the goplint `docs/`; anchors resolve against the goplint Makefile; the two soundness capabilities convert to docs-guard-governed documents. Invowk keeps a short ungoverned consumer guide.
- **Code parameterization** (done in the pre-extraction invowk PR with defaults preserved, flipped in the new repo): eight commands' `-root "../.."` defaults; `cmd/repository-audit` explicit config flags; `cmd/subgate-report` gains `-root`/`-manifest`; `internal/docsguard` and `internal/profileownership` re-rooted; the testutil HOME-env rule becomes configurable via exceptions TOML.

## Migration sequence

Each phase leaves both repositories green.

**Phase 0 — this proposal** (docs-only invowk PR; routes to the documentation tier).

**Phase 1 — pre-extraction parameterization** (the ONE semantic-tier invowk PR): thread the repo-root seam through the eight commands and docs-guard with defaults preserving current behavior; verified by the existing green semantic tier; regenerate the retained v4 record once (semantic → generate → check → complete). No config moves, no format bumps here. Rollback: revert PR.

**Phase 2 — new-repo bootstrap** (zero invowk changes): `git filter-repo --path tools/goplint/ --path-rename tools/goplint/:` on a fresh clone; ONE signed commit renames the module (history is not content-rewritten; historical commits stay unsigned — the ruleset is forward-only); `gh repo create invowk/goplint --public`; ordered bindings → CI → evidence-bootstrap PRs (the `complete`-forcing schedule trigger is enabled only after `clean-tree-run.v5.json` exists — no fail-closed window); SonarCloud + ruleset + required check; signed tag `v0.1.0`. Rollback: delete the repository.

**Phase 3 — invowk cutover** (single atomic squash PR): relocate `baseline.toml`/`exceptions.toml` to `.goplint/`; `git rm -r tools/goplint`; go.mod tool pin `v0.1.0`; Makefile/lint.yml/pre-commit rewrites; delete `goplint-fuzz.yml` and the mutation goplint arm; prune path filters, sonar exclusions, shellcheck list, windows-build steps, `.gitignore` entries; ~30-file docs/rules/skills sweep. Atomicity is forced: `lint.yml`'s plan job does `cd tools/goplint` and `goplint-behavior` is `always_run`, so deletion and rewrites are inseparable. Pre-commit reads the working-tree config, so the branch is hook-clean without bypass; PR CI runs the head ref's rewritten `lint.yml`. Add the invowk required check after the new job's first green run on `main`. Rollback: revert the squash commit (the v4 `base_commit` remains an ancestor, so the restored evidence still verifies).

**Phase 4 — cleanup**: archive this change (remove the two migrated specs from invowk); accept the first dependabot goplint pin bump as the pin-mechanism smoke test; remove `GOPLINT_FORCE_SEMANTIC` in the goplint repository after one clean release.

## Evidence continuity

The last pre-cutover invowk `main` commit holds a fresh, verifying v4 record; goplint `v0.1.0` holds a verifying v5 record; the cutover deletes v4, its checker, and the `complete` profile demand in the same commit. At no commit on either `main` is a demanded evidence check unsatisfiable.

## Risks

1. Phase 1 is the only semantic-tier + evidence-regeneration PR; scope creep into it multiplies ~90-minute CI tiers and ~55-minute evidence regenerations.
2. Phase 3 atomicity: any split leaves either a broken plan job or a broken `always_run` hook.
3. goplint CodeQL coverage is the one silent-loss trap; the new workflow is mandatory.
4. Rewritten history is unsigned (accepted with history preservation; ruleset is forward-only).
5. Spec duplication across repositories during Phases 2–3 (days) — accepted, resolved at archive.
