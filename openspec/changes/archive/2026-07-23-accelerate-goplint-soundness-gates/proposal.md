## Why

The canonical goplint soundness gate currently serializes eighteen heterogeneous subgates, repeats the same multi-minute repository analysis many times, embeds five full-scan benchmark samples in the ordinary correctness path, and re-executes a large analyzer test census under race and repeat profiles with fixed sharding. This turns routine changes into hour-scale checks, produces shard timeouts on constrained GitHub runners, and still leaves most of a 24-core, 93 GiB local workstation idle during serial phases.

The gate should preserve its fail-closed semantic, causal-evidence, and completion-proof guarantees while executing only the assurance tier required by a change, sharing work within an exact-tree run, and scheduling independent work against the resources actually available.

## What Changes

- Introduce a versioned execution plan that classifies changed paths conservatively and selects a fast consumer-change profile, a goplint semantic-change profile, or the exhaustive completion profile; unknown or unavailable change context fails closed to the exhaustive profile.
- Replace serial aggregate execution with a dependency- and resource-aware scheduler that can use detected CPU and memory capacity locally, supports explicit overrides, keeps memory-heavy work bounded, cancels promptly on failure, and retains deterministic fail-closed evidence aggregation.
- Split distributable subgates and race/repeat shards into immutable, exhaustive work units that can run locally or as a GitHub Actions matrix, then validate exact workspace, manifest, toolchain, census, and no-gap/no-overlap bindings before accepting their combined evidence.
- Replace modulo-based test sharding with duration-weighted longest-processing-time allocation backed by validated timing metadata, while compiling normal and race test binaries once per exact execution plan.
- Collapse duplicate canonical repository traversals so baseline checking, full-scan enforcement, and stale-exception auditing consume one in-run analysis result; validate exception review dates without loading packages.
- Separate performance smoke protection from statistically stable certification: ordinary pull requests run a bounded regression smoke check, while the reviewed five-sample full-scan and benchmark thresholds run for performance-sensitive changes and exhaustive scheduled/release/completion certification, parallel to correctness work where possible.
- Narrow pre-commit and CI routing so ordinary application changes receive one blocking canonical scan, while changes to goplint semantics, evidence, manifests, thresholds, governing specifications, or orchestration receive the stronger profile.
- Emit machine-readable phase, shard, cache/build, wall-time, CPU, and peak-memory telemetry, with regression tests and documented targets demonstrating large wall-clock reductions without reducing required semantic populations.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `lint-tooling-quality-gates`: Define conservative change-aware routing, single-analysis repository auditing, CI/pre-commit topology, local resource discovery, distributed execution, and performance observability requirements.
- `goplint-analysis-soundness`: Preserve deterministic, performance-bounded analysis while separating smoke detection from reproducible performance certification and forbidding resource controls from weakening semantic coverage.
- `goplint-soundness-assurance`: Preserve causal evidence and completion accuracy across concurrent and distributed subgate execution, including exact-plan binding and exhaustive shard aggregation.

## Impact

- Affected implementation areas include `.github/workflows/lint.yml`, `.pre-commit-config.yaml`, `Makefile`, `tools/goplint/cmd/soundness-gate`, `tools/goplint/internal/soundnessgate`, the soundness manifest/schema, race/repeat and benchmark scripts, repository-scan and exception-audit plumbing, and supporting tests and documentation.
- GitHub Actions changes from one long serial goplint job plus duplicate scan jobs to plan, matrix execution, and aggregate jobs with explicit artifact handoff and bounded concurrency.
- Local execution gains automatic CPU/memory-aware parallelism and documented overrides; on this workstation the scheduler must recognize 24 logical CPUs and substantially more usable memory than the 4-vCPU hosted-runner profile.
- No analyzer diagnostic semantics, exception policy, baseline policy, mutation requirement, oracle population, or completion-proof requirement is relaxed. No new runtime dependency is intended unless platform-portable resource discovery cannot be implemented cleanly with the standard library.
