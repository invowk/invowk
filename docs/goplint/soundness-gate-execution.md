# Goplint soundness-gate execution

The goplint assurance gate uses one versioned manifest and three profiles. The
profile changes how much evidence is freshly executed; it never changes the
analyzer's production semantics or turns an inconclusive result into success.

| Profile | Intended use | Assurance claim |
|---|---|---|
| `consumer` | Root application changes proven not to affect goplint semantics or orchestration | One exact-tree repository audit serves baseline, full-scan, exception, and one-sample performance-smoke consumers. It makes no analyzer-soundness or completion claim. |
| `semantic` | Goplint code, tests, evidence, manifests, thresholds, workflows, governance, or governing specifications | Re-executes every causal soundness population and five-sample runner-class performance certification. |
| `complete` | Explicit completion, release, scheduled, and exhaustive dispatch contexts | Runs the semantic profile plus retained exact-tree freshness. |

`make check-goplint-soundness` classifies staged or event-provided changes
through `spec/soundness-ownership.v1.json`. Unknown paths, shallow history,
missing merge bases, empty or ambiguous diffs, and invalid context fail closed
to `semantic` or `complete`. The explicit targets bypass classification:

```bash
make check-goplint-soundness-consumer
make check-goplint-soundness-semantic
make check-goplint-soundness-complete
```

`check-goplint-soundness-core` remains a compatibility alias for the semantic
target. New automation and documentation should use `semantic`.

## Local resource policy

The default executor generates an immutable plan and schedules dependency-ready
work against separate CPU, memory, and worker-slot budgets. CPU defaults to the
effective process `GOMAXPROCS`; Linux memory discovery retains headroom, and
portable fallbacks are conservative. Each child receives bounded
`GOMAXPROCS`, Go build `-p`, and Go test `-parallel` values from its reservation.

Override discovery with command flags:

```bash
cd tools/goplint
go run ./cmd/soundness-gate -profile semantic \
  -cpu-budget 8 -memory-budget-bytes 34359738368 -max-workers 6
```

The equivalent environment variables are
`GOPLINT_SOUNDNESS_CPU_UNITS`, `GOPLINT_SOUNDNESS_MEMORY_BYTES`, and
`GOPLINT_SOUNDNESS_MAX_WORKERS`. `GOPLINT_SOUNDNESS_EXECUTOR=plan-serial`
selects the immutable-plan serial reference for parity diagnosis;
`legacy-serial` exists only while migration comparison evidence is retained.
Resource limits may delay or fail work, but cannot shrink required populations.
Dependency-ready work is admitted deterministically in descending reservation
order, with canonical work-unit identity as the final tie breaker. This keeps
small jobs from fragmenting the runner before a critical large unit can start.
The tight algorithmic certification phase reserves the full local plan CPU
budget so its runner-class thresholds are not distorted by colocated work.
The weighted analyzer race/repeat census executes as six deterministic
four-CPU work groups (`race-repeat-1` through `race-repeat-6`); on the
reviewed local 24-CPU class the groups run concurrently, while in distributed
CI each group reserves one complete four-CPU worker so the heaviest race
shard stays inside its weight-derived timeout. Group assignment partitions
every plan work unit exactly once by descending effective weight, so the
union of the groups is the exact analyzer census by construction.

## Race and repeat timing

`spec/goplint-test-timings.v1.json` contains three fresh samples for every live
top-level `Test`, `Fuzz`, and `Example`. The planner rejects unknown, duplicate,
or kind-mismatched entries, exposes conservative default weights for newly
discovered members, and uses deterministic longest-processing-time allocation.
Nested cases are included in the timing census and must satisfy the reviewed
dominance policy.

Refresh the manifest after adding, removing, renaming, or materially changing
analyzer tests:

```bash
make update-goplint-race-repeat-timings
make check-goplint-race-repeat
```

The race/repeat command has two independently planned phases. The analyzer
phase builds and digests one normal and one race analyzer test binary per
exact plan, compares the top-level census compiled into both binaries, and
fails closed if normal-only or race-only build-tagged members would escape
either mode. It then executes the weighted analyzer census; in the aggregate
manifest this phase is expressed as the six deterministic work groups
`race-repeat-1` through `race-repeat-6` (`--group index/6`), each executing
its exact disjoint share of the plan work units with two workers of two CPUs.
`race-repeat-supporting` runs the remaining module packages once under race
and exactly three times normally. The direct Make target runs both phases in
full. Every required member must execute at the declared count; missing,
duplicated, skipped, crashed, timed-out, or wrong-binary results are blocking.

## Smoke versus certification

`make check-goplint-performance-smoke` runs one full-scan measurement plus one
iteration of fast solver and recursive-tabulation workloads under broad
catastrophic-regression limits. Passing smoke is not statistically stable
performance certification and does not support an analyzer-soundness claim.

`make check-goplint-benchmarks` is the semantic/completion certification tier.
It runs the algorithmic and full-scan phases together for direct use. The
immutable plan schedules those phases independently: `benchmarks` enforces the
tight solver, allocation, generated-analyzer, and state-count thresholds, while
`benchmark-full-scan` retains five fresh wall/RSS samples. Both use the same
reviewed policy, medians, exact toolchain, and runner class. A smoke result
cannot be relabeled as certification, and a runner/toolchain mismatch is
blocking.

## Immutable plans and distributed bundles

The distributed interface is the same contract used by GitHub Actions:

```bash
cd tools/goplint
out="$(mktemp -d)"
go run ./cmd/soundness-gate -action plan -profile semantic \
  -runner-class github-ubuntu-x64-4cpu -plan "$out/plan.json"
go run ./cmd/soundness-gate -action work -plan "$out/plan.json" \
  -work-unit aggregate-contract -bundle-dir "$out/bundles"
go run ./cmd/soundness-gate -action aggregate -plan "$out/plan.json" \
  -bundle-dir "$out/bundles" \
  -repository-audit "$out/bundles/repository-audit/repository-audit.json" \
  -report "$out/report.json" -telemetry "$out/telemetry.json"
```

The final aggregate requires a bundle for every planned work unit, so the last
command is expected to fail with the missing identities until all units have
run. Repository-audit consumers additionally receive the exact audit artifact
through `-repository-audit`.

The checked contracts are:

- `spec/soundness-execution-plan.v1.schema.json` for workspace, manifest,
  registry, toolchain, command/binary, resources, dependencies, census,
  shards, and expected reports;
- `spec/soundness-work-bundle.v2.schema.json` for a worker's timestamped
  terminal outcome, exact bindings, content-bound report/audit digests,
  observations, populations, metrics, and self-digest;
- `spec/soundness-run-report.v1.schema.json` for the no-gap aggregate verdict;
- `spec/soundness-run-telemetry.v1.schema.json` for queue, wall, CPU, peak RSS,
  reservations, timeout cause, populations, critical path, and maximum
  concurrent reservations.

Workers recompute the checkout, plan, manifest, registry, toolchain, command,
binary, census, embedded-report, and dependency-artifact digests. The
aggregator recomputes the downloaded shared-audit digest, rejects stale,
foreign, duplicate, missing, partial, or conflicting bundles, and retains both
the normalized report and aggregate telemetry. Worker loss, timeout,
cancellation, artifact expiry, and partial upload therefore surface as
actionable missing-work or stale-binding failures instead of partial success.

GitHub Actions reproduces this as plan/audit, bounded matrix-worker, and final
aggregate jobs. Artifacts have unique work-unit names, support safe job reruns,
and are merged only for strict aggregation. Scheduled and release events force
the completion profile. During migration, semantic events also retain the
single-job legacy serial reference and its report/telemetry; it is removed only
after the `goplint-parity` job has used `soundness-report-compare` to establish
byte-identical normalized evidence for three CI comparisons and those runs meet
the required wall-time reduction.

## Completion proof

After all intended files and task bookkeeping are final, generate and verify
the retained exact-tree evidence, then run the completion profile:

```bash
make check-goplint-soundness-semantic
make generate-goplint-clean-tree-evidence
make check-goplint-clean-tree-evidence
make check-goplint-soundness-complete
```

Any subsequent change makes the retained proof stale and requires regeneration.
