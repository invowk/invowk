## Context

The routed soundness gate classifies changed paths through `spec/soundness-ownership.v1.json` into `consumer`, `semantic`, or `complete`. Every goplint-adjacent path — including pure prose such as `docs/goplint/**`, `openspec/changes/**`, and markdown inside `tools/goplint/` — is owned by the `semantic` class, and the retained completion record (`clean-tree-run.v3.json`) binds the digest of every selected path, prose included. Consequences observed while closing `accelerate-goplint-soundness-gates`: bookkeeping edits triggered the full semantic profile in pre-commit and CI, and each such edit also staled the completion record, whose regeneration re-runs the semantic profile internally. The same tree was semantically re-proven up to three times per commit with zero semantic delta. Additionally, closing a change requires editing the expected task-ledger state in four synchronized places (plan JSON, `requiredTaskLedgers` in Go, a JSON-schema `const`, and test fixtures).

Constraints: the governing specs require conservative fail-closed routing, forbid weakening any semantic population within a tier, and require stale completion evidence to be blocking and non-baselinable. The maintainer's directive is that nothing expensive should run unless goplint itself changed substantially.

## Goals / Non-Goals

**Goals**

- Prose and bookkeeping changes cost seconds (static validation only) at every trigger point: pre-commit, CI, and completion-evidence freshness.
- Harness-only changes prove orchestration integrity without re-executing analyzer populations.
- The completion record survives prose drift through cheap, sound re-binding.
- One reviewed source of truth for task-ledger completion expectations.
- Every reclassification is compensated by an explicit cheaper guard, and unknown context still fails closed to `semantic`.

**Non-Goals**

- No change to analyzer diagnostics, populations, thresholds, baselines, exceptions, or mutation requirements inside any tier.
- No caching or reuse of semantic evidence across different semantic-content digests.
- No change to the `complete` profile's obligations for release/schedule/completion events beyond the freshness split.

## Decisions

**D1. Four-class ownership taxonomy.** The ownership manifest gains a `class` per path family: `documentation`, `consumer`, `harness`, `analyzer-semantics`. Routing resolves the highest class present in the diff (documentation < consumer < harness < analyzer-semantics). Unknown paths, missing merge bases, and ambiguous context keep failing closed to `semantic`/`complete`. Alternative considered: a boolean "docs exemption" bolted onto the current model — rejected because harness-only changes would still pay full semantic cost and the taxonomy would stay implicit.

**D2. Documentation tier = static docs validator, not "skip".** A new `cmd/docs-guard` validates in seconds that goplint prose stays anchored to executables: every claim row in `docs/goplint/evidence-index.md` names an existing test or gate, referenced Make targets exist in the Makefile, referenced subgate IDs exist in the generated manifest, and referenced repository paths exist. Rationale: the original reason docs were semantic-owned is claim/evidence desynchronization; a purpose-built validator guards exactly that risk at ~0.1 percent of the cost. Alternative: route docs to `consumer` — rejected as it still runs a multi-minute repository audit that cannot detect prose drift anyway.

**D3. Harness tier obligations.** For diffs whose highest class is `harness` (scheduler, distributed executor, plan runner, workflow YAML, gate scripts, Makefile gate recipes): run the goplint module unit/integration test suite, one shared repository audit, and a fixture-driven `plan-serial` versus `parallel` normalized-report parity check. Rationale: harness changes can lose or corrupt evidence but cannot change analyzer verdicts; parity plus the executor test suites are the proof obligations that match that failure mode. The full semantic profile remains reachable explicitly and on schedule/release.

**D4. Dual-digest completion record.** The v4 record splits the tree binding into `semantic_tree_digest` (path classes `analyzer-semantics`, `harness`, `consumer` — everything that can influence any executed gate) and `prose_tree_digest` (class `documentation`). Freshness verification fails closed on semantic-digest drift exactly as today. On prose-only drift, `generate-goplint-clean-tree-evidence` re-binds: it recomputes both digests, revalidates task ledgers and the diff census, carries forward the retained aggregate report untouched, and rewrites the record without executing any profile. Soundness argument: the retained aggregate report is evidence about the semantic content that produced it; re-binding never mutates the report, only the prose identity, and the semantic digest proves the evidence-producing content is byte-identical. Alternative considered: shrinking the path selection to exclude prose entirely — rejected because task ledgers and governing docs must remain part of the auditable proof identity, just not of the re-execution trigger.

**D5. Pre-commit ceiling.** The `goplint-behavior` hook routes as today but caps execution at the `consumer` tier: documentation diffs run `docs-guard`, consumer diffs run the shared audit, and harness or analyzer-semantics diffs run the fast tier locally while printing the authoritative CI tier that will run. The `check-goplint-soundness-semantic` and `-complete` Make targets remain for explicit local use. Rationale: local semantic execution duplicated CI verbatim on every commit; CI is parallel, authoritative, and archived. Alternative: keep semantic in pre-commit behind an env opt-out — rejected as an inverted default that preserves the pain.

**D6. CI routing from cumulative diff class.** The plan job classifies the pull request's full base-to-head diff: documentation → docs-guard job only; consumer → shared audit plus consumer verdict workers; harness → harness tier; analyzer-semantics → `semantic`. Push to main, schedule, release, and dispatch events keep their current forced profiles. Rationale: per-push routing on PR branches previously re-ran semantic for every fixup commit regardless of content.

**D7. Single-source ledger binding.** `requiredTaskLedgers` moves entirely into the reviewed plan JSON (`clean-tree-v3.json` → v4). Code validates structure (dependency order, archived-predecessor policy, non-negative pending sets) without embedding expected names, paths, or pending IDs; the JSON schema validates shape, not values; tests use fixtures independent of the live repository state. Closing a change then edits exactly one reviewed file. Alternative: generate the Go constant from the JSON — rejected as needless codegen for data that only the JSON needs to own.

## Risks / Trade-offs

- **Misclassification risk**: a path that can affect verdicts landing in the `documentation` class. Mitigated by fail-closed unknown-path handling, a table-driven routing test for every governed family, and review of the manifest as a versioned reviewed artifact.
- **Prose that encodes policy** (for example baseline documentation embedded in markdown) could drift without semantic re-proof. Mitigated by `docs-guard` anchor checks and by keeping any file that a gate actually reads (TOML, JSON, scripts) out of the documentation class by construction — classes are assigned to path families, and executable-input families are enumerated explicitly.
- **Harness-tier blind spots**: an orchestration bug that only manifests at full scale. Mitigated by keeping `semantic` on schedule and release, so full re-execution still happens periodically and before any published artifact.
- **Record format migration**: v4 must remain backward-readable for one transition version; verification of a v3 record reports a visible migration notice rather than silent acceptance.

## Migration Plan

1. Land the taxonomy, docs validator, and routing tests behind the existing profiles (no behavior change yet).
2. Switch pre-commit and CI to class-based routing; keep a one-release environment escape (`GOPLINT_FORCE_SEMANTIC=1`) for diagnosis.
3. Ship the v4 record with dual digests and re-binding; regenerate once; verify v3 rejection-with-notice.
4. Collapse the four-place ledger binding into the plan JSON in the same release as the v4 record (both touch the same files).
5. Remove the escape hatch after one quiet release.

## Open Questions

- Should `openspec/specs/goplint-*` deltas (as opposed to `openspec/changes/**` planning prose) stay in the `analyzer-semantics` class? Default: yes — governing requirement text changes what must be proven, so it stays expensive until reviewed otherwise.
- Exact fixture size for the harness-tier parity check (small synthetic manifest versus a trimmed real profile) is left to implementation, bounded by a one-minute budget.
