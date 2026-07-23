## 1. Baseline and Observability

- [x] 1.1 Add per-subgate queue, wall, CPU, peak-RSS, reserved-resource, timeout, and population fields to a versioned aggregate telemetry model with deterministic normalization tests.
- [x] 1.2 Instrument the existing serial aggregate runner without changing its commands, evidence bindings, ordering, or verdicts.
- [x] 1.3 Capture and retain at least three comparable legacy consumer-like and semantic-profile runs on the reviewed 4-vCPU GitHub runner class.
- [x] 1.4 Capture and retain at least three comparable legacy semantic-profile runs on the local 24-CPU, 93 GiB workstation, including critical-path and resource-utilization evidence.
- [x] 1.5 Add a checked-in migration performance report that records exact commits, toolchain, cache policy, runner classes, medians, peak memory, and current shard imbalance.

## 2. Immutable Execution Plan and Manifest Contract

- [x] 2.1 Define the versioned execution-plan schema for workspace, manifest, registry, toolchain, command, resource, dependency, census, shard, and expected-report bindings.
- [x] 2.2 Extend the soundness manifest/schema with dependencies, CPU units, estimated peak memory, exclusivity groups, distributability, and profile membership for every subgate.
- [x] 2.3 Implement deterministic plan generation and canonical JSON normalization in `internal/soundnessgate`.
- [x] 2.4 Add planner validation for dependency cycles, unknown subgates, impossible resources, unsafe commands, duplicate work-unit identities, and incomplete expected populations.
- [x] 2.5 Add adversarial tests proving that changed workspace, manifest, registry, toolchain, command, binary, or census bindings invalidate a plan or worker result.
- [x] 2.6 Add serial reference execution through the new plan format and prove normalized report/evidence parity with the legacy serial runner.

## 3. Resource-Aware Local Scheduler

- [x] 3.1 Implement effective-CPU discovery from the process runtime limit and Linux available-memory discovery with tested portable conservative fallbacks.
- [x] 3.2 Add validated CLI and environment overrides for CPU budget, memory budget, maximum workers, and serial-reference mode.
- [x] 3.3 Implement deterministic dependency-ready scheduling with separate CPU/memory token pools, exclusivity groups, isolated evidence directories, and fail-fast cancellation.
- [x] 3.4 Propagate bounded `GOMAXPROCS`, Go build parallelism, and Go test parallelism to each child from its resource reservation.
- [x] 3.5 Add scheduler tests for admission, fairness, deterministic ready ordering, cancellation, timeout cleanup, resource release, and no overcommit under concurrent completion.
- [x] 3.6 Calibrate and commit reviewed CPU/memory weights for every subgate on the 4-vCPU CI class and local high-capacity class.
- [x] 3.7 Verify auto mode recognizes 24 effective local CPUs, avoids new swap pressure, and reserves at least 75 percent of available CPU while sufficient independent work exists.

## 4. Single-Pass Repository Audit

- [x] 4.1 Define a versioned repository-audit result containing normalized findings, baseline matches/new/stale IDs, exception matches/stale entries, package census, scan metadata, and exact input digests.
- [x] 4.2 Refactor the canonical goplint invocation to compute full-scan, baseline, and stale-exception data from one superset analyzer traversal.
- [x] 4.3 Make `check-baseline`, `check-goplint-full-scan`, and stale-exception consumers validate and reuse the in-plan repository-audit result.
- [x] 4.4 Split exception review-date validation into a configuration-only path that never loads Go packages.
- [x] 4.5 Preserve deliberate `update-baseline` write semantics and reject audit reuse when analyzer mode, inputs, or output purpose differ.
- [x] 4.6 Add parity and invalidation tests comparing shared-audit and standalone verdicts across findings, inconclusives, stale IDs, stale exceptions, malformed configuration, and tool failures.
- [x] 4.7 Remove duplicate canonical scan jobs/recipes only after the parity suite and exact-input binding tests pass.

## 5. Balanced Race and Repeat Execution

- [x] 5.1 Add a deterministic command that records top-level analyzer test durations from `go test -json` into a versioned timing manifest.
- [x] 5.2 Validate the timing manifest against the live `Test`, `Fuzz`, and `Example` census, rejecting duplicates/unknown entries and assigning visible conservative weights to new entries.
- [x] 5.3 Implement deterministic longest-processing-time shard allocation and tests proving complete no-gap/no-overlap assignment and improved maximum-shard weight over modulo allocation.
- [x] 5.4 Identify dominating nested test families and either split them into stable top-level tests or add a machine-validated internal case-shard protocol.
- [x] 5.5 Build and digest the normal analyzer test binary once and the race-instrumented analyzer test binary once per exact plan.
- [x] 5.6 Execute race and repeat work units from the prebuilt binaries with weight-derived reviewed timeouts and isolated output/evidence paths.
- [x] 5.7 Prove every top-level or internal case executes exactly once under race and exactly the configured repeat count normally, including timeout/crash/missing-result adversarial tests.

## 6. Performance Smoke and Certification Tiers

- [x] 6.1 Split benchmark configuration into separately named consumer smoke and runner-class certification policies without changing certified five-sample thresholds.
- [x] 6.2 Implement the consumer one-sample full-scan and fast algorithmic/state-count smoke checks with conservative catastrophic-regression limits.
- [x] 6.3 Keep five fresh samples and median enforcement for solver, allocation, generated-analyzer, and full-scan certification on semantic/completion runner classes.
- [x] 6.4 Make performance certification an independent planned work group that can overlap unrelated correctness work when resource tokens or CI workers permit.
- [x] 6.5 Add tests rejecting runner-class/toolchain mismatch, missing samples, relabeled smoke as certification, omitted analyzer phases, and weakened semantic populations.

## 7. Conservative Change-Aware Profiles

- [x] 7.1 Add a versioned ownership manifest covering root consumer paths, goplint semantics/tests, evidence and gate code, baseline/exceptions, thresholds, workflows, agent rules, documentation, and governing OpenSpec capabilities.
- [x] 7.2 Implement deterministic `consumer`, `semantic`, and `completion` profile selection from event and changed-path context.
- [x] 7.3 Fail closed to `semantic` or `completion` for missing merge bases, shallow history, unknown paths, invalid ownership data, or ambiguous event context.
- [x] 7.4 Preserve stable explicit Make targets that force semantic core and completion execution independently of automatic change classification.
- [x] 7.5 Update pre-commit routing so ordinary root consumer changes run one canonical audit while goplint-owned changes run the semantic profile.
- [x] 7.6 Add table-driven routing tests for every governed path family, multi-area diffs, renames/deletions, empty diffs, workflow dispatch, scheduled runs, releases, and failure fallbacks.

## 8. Distributed GitHub Actions Execution

- [x] 8.1 Add soundness-gate commands to emit a plan, execute one assigned work unit, and aggregate a complete set of result bundles.
- [x] 8.2 Add bundle validation for plan identity, exact checkout tree, command/binary/toolchain digests, terminal status, observations, and required populations.
- [x] 8.3 Replace the single long semantic job with plan, bounded matrix-worker, and aggregate jobs using immutable artifact handoff.
- [x] 8.4 Run the consumer repository audit once and remove standalone baseline and exception package-analysis jobs while retaining their distinct blocking verdicts.
- [x] 8.5 Ensure matrix cancellation, worker loss, artifact expiry, duplicate bundles, and partial uploads fail the aggregate job with actionable missing-work diagnostics.
- [x] 8.6 Add workflow syntax/action validation and a local fixture-driven simulation of plan, multi-worker bundle production, and final aggregation.
- [ ] 8.7 Run optimized and legacy CI topologies side by side until three semantic runs prove exact evidence parity and at least 50 percent median wall-time reduction.

## 9. Documentation and Developer Interfaces

- [x] 9.1 Update Make help, `AGENTS.md`, `tools/goplint/AGENTS.md`, `.agents/rules/commands.md`, and goplint documentation with profile semantics, automatic routing, resource auto-detection, overrides, timing refresh, CI reproduction, and completion commands.
- [x] 9.2 Document that consumer smoke is not certification and that only semantic/completion profiles make analyzer soundness or exact-tree completion claims.
- [x] 9.3 Document the execution-plan and result-bundle schemas, digest/freshness boundaries, telemetry fields, and failure diagnostics.
- [x] 9.4 Run `make check-agent-docs` and fix all agent/rule/skill index or synchronization drift caused by the documentation changes.

## 10. Final Parity, Performance, and Cleanup

- [x] 10.1 Run unit, integration, race, repeat, deterministic scheduling, no-gap aggregation, repository-audit parity, ownership-routing, and resource-bound test suites in both Go modules.
- [ ] 10.2 Demonstrate consumer CI feedback at or below 10 minutes on the reviewed warm-cache 4-vCPU class.
- [ ] 10.3 Demonstrate semantic CI and local 24-CPU median wall times at least 50 percent below their recorded serial baselines without a required shard depending on a fixed 20-minute package timeout.
- [x] 10.4 Compare serial-reference and optimized normalized findings, observations, populations, and verdicts byte-for-byte on the same exact tree.
- [x] 10.5 Run `make lint`, `make test`, baseline, exception, full-scan, performance smoke, performance certification, semantic soundness, retained clean-tree generation/verification, and completion gates.
- [x] 10.6 Run strict OpenSpec validation and `git diff --check`, then generate the final exact-tree completion evidence after all task bookkeeping and intended files are final.
- [ ] 10.7 Remove the legacy serial scripts/workflow topology and temporary comparison switches only after parity, performance, and rollback criteria pass; verify no stale command or documentation references remain.
