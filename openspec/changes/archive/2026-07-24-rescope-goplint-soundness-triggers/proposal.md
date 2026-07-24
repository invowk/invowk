## Why

The goplint assurance triggers conflate three distinct change classes — analyzer semantics, execution harness, and prose/bookkeeping — so editing documentation, an OpenSpec task checkbox, or a performance report costs the same full semantic re-execution (~25 minutes locally, again in pre-commit, again in CI) as rewriting the solver, and any byte of tree drift additionally invalidates the retained exact-tree completion record, forcing a ~45-minute regeneration that itself re-runs the semantic profile. Closing the `accelerate-goplint-soundness-gates` change demonstrated this empirically: pure bookkeeping edits triggered multiple redundant semantic executions and completion-evidence regenerations in one day.

## What Changes

- Reclassify the versioned ownership manifest into change classes: analyzer-semantics paths keep the `semantic` profile; a new prose/bookkeeping class (goplint docs, OpenSpec change artifacts, agent docs, performance reports, markdown inside `tools/goplint`) routes to a documentation tier that runs a fast static docs validator instead of any analyzer execution; unknown paths still fail closed to `semantic`.
- Introduce a `harness` assurance tier between `consumer` and `semantic` for orchestration-only changes (scheduler, distributed plumbing, workflow topology, gate scripts): module unit tests, one shared repository audit, and small-fixture serial-versus-parallel report parity — without re-executing the full semantic populations.
- Split the retained completion record's tree binding into a semantic-content digest (Go sources, spec manifests, scripts, testdata, baselines) and a prose digest; when only the prose digest drifts, evidence generation re-binds the retained record (re-hash, ledger revalidation) in seconds without re-running the semantic profile, while semantic-content drift keeps requiring full regeneration.
- Remove the `semantic` tier from the pre-commit hook: local commits run at most the documentation tier or one shared consumer audit; the semantic profile remains a CI gate and an explicit local Make target.
- Route CI semantic execution from the pull request's cumulative diff class: prose-only diffs run the documentation tier, harness diffs run the harness tier, and only analyzer-semantics diffs (or schedule/release/dispatch events) run `semantic` or `complete`.
- Replace the four-place task-ledger completion binding (plan JSON, Go constant, JSON schema `const`, test fixtures) with the reviewed plan JSON as the single source of truth, validated structurally by code and schema without duplicating the expected values.
- Add a static goplint documentation validator that cross-checks prose claims against executable anchors (evidence-index claim-to-test mapping, referenced Make targets, subgate IDs, file paths) in seconds, as the compensating guard for removing docs from the semantic class.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `lint-tooling-quality-gates`: Change-class taxonomy for routing (documentation, harness, consumer, semantic, complete), pre-commit tier ceiling, CI diff-class routing, and the static documentation validator requirement.
- `goplint-soundness-assurance`: Dual-digest completion-record freshness with prose-only re-binding, harness-tier evidence obligations, and single-source task-ledger binding.

## Impact

- Affected code: `tools/goplint/spec/soundness-ownership.v1.json` and its generator/validators, `tools/goplint/cmd/soundness-profile`, `tools/goplint/internal/cleantreeevidence` (dual digest, re-binding, ledger single-sourcing), `tools/goplint/internal/soundnessgate` (harness-tier profile), a new docs validator command, `.pre-commit-config.yaml`, `.github/workflows/lint.yml`, Makefile targets, and goplint agent documentation.
- No analyzer diagnostic semantics, baseline policy, exception policy, mutation requirement, or semantic population is weakened; the change alters when each tier is required, not what any tier proves.
- The retained completion record format gains fields (format version bump) and remains backward-readable for one transition version.
