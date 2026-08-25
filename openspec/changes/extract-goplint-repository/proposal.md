## Why

goplint has outgrown its home at `tools/goplint`. It is a 637-file, ~92.7k-LOC nested Go module with its own analyzer, IFDS/IDE solver, distributed soundness-gate executor, fuzz targets, mutation kernel, and retained completion evidence — none of which shares a single Go import edge with the invowk root module. Keeping it in-tree couples two unrelated release cadences (a command runner and a static-analysis toolchain), routes expensive analyzer-semantics CI tiers through invowk's pull-request path, denies goplint independent versioning and reuse by other consumers, and leaves its dependencies unmanaged (no dependabot entry for the nested module today). Extraction gives goplint its own repository, releases, and CI while invowk pins it like any other verified tool.

## What Changes

- Create the public `invowk/goplint` repository with preserved `tools/goplint` history (`git filter-repo`), the module renamed to `github.com/invowk/goplint`, its own Makefile, CI (lint/soundness executor, module tests, fuzz, mutation, CodeQL, windows cross-build, license/file-length/shellcheck), SonarCloud, dependabot, and a Safety-equivalent ruleset with a required `goplint soundness aggregate` status check.
- Re-root the soundness-assurance harness on the goplint repository: ownership manifest v3 with goplint-repo-relative classes, clean-tree completion evidence v5 bound to the goplint tree (v4 is retired by deletion — it binds invowk git objects and cannot survive extraction), docs-guard re-anchored to the goplint Makefile, and the two goplint soundness capabilities converted to docs-guard-governed documents in the new repository.
- Invowk consumes goplint via root `go.mod` `tool` directives with an exact pinned version (same pattern as golangci-lint and go-mutesting), managed by the existing dependabot `gomod /` entry.
- Invowk-owned analyzer data relocates from the goplint tree into a new `.goplint/` directory: `baseline.toml`, `exceptions.toml`, the consumer-smoke policy, a reduced consumer-gate manifest, and a reduced two-class ownership manifest (documentation vs consumer, fail-closed to consumer).
- Every invowk consumer gate survives with identical semantics: `check-types*`, `check-baseline`, `update-baseline`, `check-goplint-exceptions`, `check-goplint-full-scan`, `check-goplint-repository-audit`, and the consumer performance smoke, re-pointed at the pinned binary and `.goplint/` configs. The pre-commit `goplint-behavior` hook remains `always_run` at the consumer tier (its current local ceiling), and `lint.yml` replaces the plan/worker/aggregate executor with a single "goplint consumer gates" job that becomes a required status check.
- Performance gating lives on both sides: invowk keeps its live-tree consumer smoke; goplint CI adds a reference-corpus certification job that scans invowk at a pinned ref before any release.
- Before extraction, one semantic-tier invowk PR makes goplint root-agnostic (parameterized `-root` seams, docs-guard anchor, consumer-smoke target) with defaults preserving current behavior, verified by the existing green semantic tier, and regenerates the retained v4 record once.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `lint-tooling-quality-gates`: two-module lint requirements collapse to root-module scope; goplint consumer gates re-point at the pinned external tool and `.goplint/` configuration; the routed model reduces to documentation-vs-consumer classes in invowk.
- `mutation-testing`: the goplint mutation module, target manifests, and baseline migrate to the goplint repository; invowk retains only the root-module profile.
- `dependency-tooling-maintenance` and `go-dependency-maintenance`: the two-module inventory becomes single-module plus a pinned goplint tool dependency whose graph rides invowk's govulncheck and dependabot coverage.

### Removed Capabilities

- `goplint-analysis-soundness` and `goplint-soundness-assurance`: migrate to the goplint repository as docs-guard-governed documents. Removed from invowk's spec set at archive time, after the goplint repository's copies are authoritative.

## Impact

- Affected invowk code and config: `Makefile` (~45 goplint targets split), `.github/workflows/{lint,goplint-fuzz,mutation-testing}.yml`, `.pre-commit-config.yaml`, `go.mod` tool block, `scripts/{golangci-lint.sh,mutation.sh,check-windows-build.sh,check-agent-docs.sh}`, new `scripts/goplint*.sh`, sonar exclusions, `.gitignore`, `tools/mutation/goplint-*`, and ~30 documentation/rules/skills files.
- No invowk gate is weakened or lost. Every check either stays in invowk (consumer tier), relocates with the code it verifies (goplint self-verification), or is replaced by a functionally equivalent successor (clean-tree v4 → v5 in the goplint repository). The gate-loss audit in `design.md` maps every gate individually.
- Cross-repo window: during bootstrap the two soundness specs exist in both repositories for a few days; this duplication is accepted and resolved at archive time.
- The retained v4 completion record is regenerated once (pre-extraction PR) and retired by deletion in the atomic cutover commit; at no commit on either `main` is a demanded evidence check unsatisfiable.
