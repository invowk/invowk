## Context

The current `core` soundness profile contains eighteen subgates and the aggregate runner executes them in manifest order. Several subgates invoke the same canonical repository analysis independently: standalone CI scan, baseline, exception governance, the aggregate baseline, five full-scan benchmark samples, and the aggregate full scan. The race/repeat script then enumerates hundreds of top-level analyzer tests, assigns them to sixteen modulo shards, runs at most four shard processes on one machine, and executes the census once with `-race` and three more times normally. Each process owns a 20-minute package timeout, so an imbalanced shard can fail even when the workflow job has a longer timeout.

This structure conflates three questions: whether application code passes the analyzer, whether changes to the analyzer preserve its soundness contract, and whether an exact tree has exhaustive completion evidence. It also treats a 4-vCPU hosted runner and this 24-logical-CPU, 93 GiB local machine as if they had the same execution budget.

The design must retain the existing guarantees: one canonical production semantics, fail-closed uncertainty, category-specific causal observations, complete manifest populations, mutation causality, deterministic results, reviewed performance limits, and exact-tree completion freshness. Faster execution is valid only when it removes duplicated work or safely overlaps independent work; it must not silently omit a required population.

## Goals / Non-Goals

**Goals:**

- Reduce ordinary application-change goplint feedback to one canonical repository audit.
- Reduce semantic-change wall time by at least half on the same runner through work reuse, balanced sharding, and distributed or local parallel execution.
- Saturate available local CPU during parallelizable phases without causing memory thrash, uncontrolled nested parallelism, or evidence races.
- Preserve or strengthen exact-tree, manifest, toolchain, command, population, and causal bindings for every accepted observation.
- Make performance bottlenecks and shard imbalance visible in retained machine-readable telemetry.
- Keep the execution model reproducible through versioned plans, manifests, timing data, and explicit resource overrides.

**Non-Goals:**

- Changing analyzer diagnostic semantics, accepted baseline findings, exception policy, oracle bounds, mutation populations, or completion criteria.
- Making virtual or cached evidence count as fresh causal execution when the governing profile requires the subgate to run.
- Replacing correctness checks with benchmarks or treating faster execution as evidence of semantic soundness.
- Depending on a particular number of GitHub-hosted runners for correctness.
- Introducing remote execution outside GitHub Actions or a general-purpose build system.

## Decisions

### 1. Produce one immutable execution plan before running work

Add a versioned execution-plan model to `internal/soundnessgate`. A plan records the profile, workspace digest, manifest and registry digests, Go toolchain identity, changed-path classification, resource budget, complete subgate/test census, dependencies, shard assignments, expected reports, and command digests. Planning is deterministic: the same inputs and timing metadata produce byte-identical normalized plans.

Every local worker, CI matrix worker, and final aggregator consumes that plan and recomputes the bindings it can observe. Missing, duplicate, overlapping, foreign-tree, foreign-toolchain, or foreign-command results fail aggregation. The aggregate report is sorted canonically, so concurrency affects timing only.

Alternative considered: let each shell script discover its own work. That is simpler but cannot prove that independently running jobs covered the same immutable census without gaps.

### 2. Route automatic checks through conservative assurance profiles

Define a versioned path-ownership manifest and three public profiles:

- `consumer`: one canonical production repository audit, including baseline comparison and exception governance, for changes that consume goplint but cannot alter the analyzer or its proof machinery.
- `semantic`: the complete current causal core population for changes to `tools/goplint` semantics, tests, manifests, scripts, thresholds, workflow orchestration, or governing goplint specifications. Performance certification is a parallel work group rather than a serial predecessor of unrelated correctness checks.
- `completion`: `semantic` plus clean-tree freshness and retained exact-tree proof requirements for completion, release, explicit dispatch, and scheduled certification.

Baseline/exception-only edits receive the consumer audit plus governance-contract validation. Unknown paths, shallow history, an unavailable merge base, an invalid ownership manifest, or ambiguous classification select `semantic` or `completion` according to event context; they never select a weaker profile. Explicit Make targets keep stable meanings and allow maintainers to force a stronger profile.

Alternative considered: trigger the exhaustive profile for every `cmd`, `internal`, and `pkg` edit. That preserves today’s behavior but spends most time re-proving unchanged analyzer implementation rather than checking the changed consumer code.

### 3. Schedule local work with explicit CPU and memory tokens

Extend each manifest subgate with dependencies, CPU units, estimated peak memory, exclusivity groups, and distributability. The local executor maintains separate CPU and memory token pools, starts ready work in stable priority order, reserves all declared resources before launch, and cancels remaining work on the first blocking failure while still collecting termination diagnostics.

The default CPU budget is the process’s effective `GOMAXPROCS`, so affinity and container limits are honored. Linux available-memory discovery supplies the local memory budget; other platforms use a tested platform adapter or a conservative fallback. Explicit flags/environment variables override CPU, memory, and maximum worker count. Child Go commands receive bounded `GOMAXPROCS`, `-p`, and `-parallel` settings derived from their reservation so nested tools do not multiply concurrency beyond the plan.

On the current workstation, auto mode must recognize 24 available CPUs and a high-memory class, admit materially more concurrent medium-weight work than the 4-vCPU CI class, and keep enough memory headroom to avoid new swap pressure. Heavy full-repository analyzer traversals are not overlapped unless the measured memory budget admits them.

Alternative considered: use a single `-j` count. A worker count alone cannot distinguish a lightweight manifest check from a multi-gigabyte analyzer traversal and either underutilizes large machines or exhausts small ones.

### 4. Use the same plan for local execution and CI fan-out

The planner can emit independent work units for GitHub Actions. A plan job uploads the immutable plan; matrix workers check out the same commit, verify the plan, execute assigned units, and upload report bundles. A final aggregate job downloads all bundles and validates exact coverage before emitting the canonical soundness report.

Local execution uses the identical work-unit format but schedules units as child processes on one host. Correctness therefore does not depend on GitHub Actions semantics, and local reproduction can execute the exact CI plan with explicit resource limits.

CI matrix concurrency is bounded to avoid waste, but changing the bound affects wall time only. A worker loss or artifact expiry is a missing required result and fails closed.

Alternative considered: maintain separate shell orchestration for local and CI. Two orchestrators would inevitably drift in census, command, or evidence rules.

### 5. Compute one canonical repository audit per exact run

Create a machine-readable repository-audit result from one analyzer traversal. It contains normalized findings, baseline matches/new/stale identities, exception matches/stale entries, scan metadata, and the bindings required by the soundness registry. Baseline checking, the blocking full scan, and stale-exception auditing consume this result. Review-date validation becomes a configuration-only check and does not load Go packages.

Consumers verify the audit’s workspace, analyzer binary, flags, configuration, baseline, exception, toolchain, and package-census digests. An audit from any different input is rejected. `update-baseline` remains a deliberate write path and does not consume a result whose mode or inputs differ.

Alternative considered: cache arbitrary analyzer output across invocations. Cross-run caching makes freshness and invalidation harder to prove; this change limits semantic reuse to a single immutable execution plan while retaining the Go build cache.

### 6. Replace modulo race/repeat shards with validated weighted shards

Generate a versioned timing manifest from `go test -json` at top-level test granularity. The planner validates that every live `Test`, `Fuzz`, and `Example` appears exactly once, rejects unknown or duplicate entries, assigns a conservative default weight to new tests, and uses deterministic longest-processing-time allocation to minimize the heaviest shard. Nested high-cost families that cannot be balanced at top-level are split into stable top-level entry points or given an explicit internal shard parameter with a validated case census.

Build the analyzer test binary once for normal execution and once with race instrumentation per workspace/toolchain/flag identity. Shards execute those binaries rather than recompiling the package. Race and repeat remain distinct required populations; every top-level test runs once under race and the configured repeat count under normal execution. Per-shard timeouts are derived from weight plus a reviewed floor/ceiling and are recorded in the plan rather than fixed blindly at 20 minutes.

Alternative considered: increase the package timeout. That hides imbalance and lengthens failure feedback without reducing work or proving even distribution.

### 7. Separate performance smoke detection from certification

The `consumer` profile runs one full-scan measurement and fast algorithmic/state-count checks with a deliberately conservative catastrophic-regression limit. It does not claim statistically stable performance certification. The `semantic` and `completion` profiles retain the reviewed five-sample median for solver, allocation, generated-analyzer, and full-scan thresholds, but execute that work as an independent work group that can overlap other heavy checks when resources permit or run on a dedicated matrix worker.

Threshold manifests name their runner class and toolchain. Results from a mismatched runner class cannot satisfy certification. Scheduled and release executions always select certification even when change classification would otherwise be weaker.

Alternative considered: remove full-scan performance checks from pull requests entirely. A smoke tier catches catastrophic regressions early while the certified tier retains reproducibility where analyzer code can change.

### 8. Make performance and utilization acceptance evidence explicit

Every work unit reports queue time, start/end time, wall time, CPU time where available, peak RSS, exit/timeout cause, cache/build identity, reserved resources, and population counts. The aggregate report derives critical path, parallelism, maximum concurrent CPU/memory reservations, and shard imbalance.

Before migration completes, capture at least three comparable legacy runs for consumer-like and semantic-like inputs on the 4-vCPU CI class and this local workstation. Acceptance requires:

- consumer CI feedback at or below 10 minutes in the reviewed warm-cache measurement;
- semantic CI median wall time at least 50% below the legacy median, with no per-shard 20-minute timeout dependency;
- local semantic median wall time at least 50% below the serial legacy median;
- at least 75% of the effective CPU budget reserved while sufficient independent runnable work exists on the 24-CPU workstation; and
- byte-identical normalized findings, observations, required populations, and completion verdicts between serial reference and optimized execution.

The measurements are guardrails for this migration, not timeless runner-independent thresholds. Ongoing manifests retain reviewed absolute limits per runner class.

## Risks / Trade-offs

- [Path classification omits a semantic owner] → Keep ownership versioned, test every governed path class, fail unknown paths closed, and run completion on main/release/schedule.
- [Concurrent subgates contend for memory and become slower or unstable] → Require measured memory weights, token admission, nested parallelism limits, peak-RSS telemetry, and conservative fallbacks.
- [Distributed artifacts are incomplete or belong to another tree] → Bind every work unit to plan/workspace/manifest/toolchain/command digests and require an exact no-gap/no-overlap set.
- [Timing metadata becomes stale] → Validate the live census, assign new tests a conservative high weight, report imbalance, and provide a deterministic timing-refresh command reviewed like benchmark thresholds.
- [Single-scan reuse hides mode differences] → Define one canonical superset scan and make each consumer verify all analyzer/configuration/package inputs; retain dedicated tests that compare shared and standalone verdicts.
- [Performance smoke checks are noisy] → Use a wide catastrophic threshold for one-sample smoke and reserve regression claims for five-sample runner-class certification.
- [Matrix fan-out increases CI cost] → Bound concurrency, group small work units, record runner-minutes as telemetry, and optimize for critical-path reduction rather than maximum job count.
- [Fail-fast cancellation loses useful diagnostics] → Retain completed reports and cancellation causes for debugging, but never accept a partial aggregate as passing evidence.

## Migration Plan

1. Add telemetry to the current serial runner and capture legacy CI/local baselines without changing verdicts.
2. Introduce the versioned plan, resource model, and serial reference executor; prove plan validation and normalized report parity against the existing runner.
3. Implement the shared repository audit and configuration-only review-date validation; keep old standalone targets as comparison wrappers until parity tests pass.
4. Implement weighted test planning and prebuilt normal/race binaries; compare exact test censuses and repeat/race populations with the old script.
5. Enable the resource-aware local executor behind an explicit flag, then make it default after deterministic and resource-bound tests pass.
6. Add CI plan/matrix/aggregate jobs and run them alongside the legacy job for a measured observation period. The optimized jobs are not authoritative until parity and no-gap aggregation pass.
7. Enable conservative change-aware profile routing and remove duplicate CI/pre-commit scans only after stronger-profile fallbacks and path-classification tests pass.
8. Remove legacy orchestration after the performance targets and exact evidence parity are demonstrated in the retained migration report. Rollback consists of restoring the serial executor and exhaustive routing; manifests and reports remain backward-readable during one transition version.

## Open Questions

- Exact CPU/memory weights and CI matrix concurrency will be calibrated from the telemetry baseline before the optimized executor becomes authoritative.
- The implementation should determine whether high-cost nested analyzer test families can be split cleanly into top-level tests or require an explicit case-shard protocol; either choice must preserve a machine-validated census.
- If portable available-memory discovery cannot be implemented cleanly with the standard library, the implementation must present the smallest maintained dependency option for review before adding it.
