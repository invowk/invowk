## 1. Change-Class Taxonomy

- [x] 1.1 Extend the ownership manifest schema and generator with the four reviewed change classes (documentation, consumer, harness, analyzer-semantics) and enumerate every governed path family under exactly one class.
- [x] 1.2 Add manifest validation rejecting a documentation classification for any executable-input family (configuration, manifests, schemas, scripts, baselines, thresholds).
- [x] 1.3 Implement highest-class diff resolution in `cmd/soundness-profile` with fail-closed handling for unknown paths, missing merge bases, and ambiguous context.
- [x] 1.4 Add table-driven routing tests covering every governed family, mixed-class diffs, renames, deletions, empty diffs, and all event contexts.

## 2. Static Documentation Validator

- [x] 2.1 Implement `cmd/docs-guard` validating evidence-index claim rows, referenced Make targets, subgate identifiers, commands, and repository paths without loading Go packages.
- [x] 2.2 Add tests proving stale references (removed target, renamed subgate, missing path, dangling claim) fail with the exact reference and cannot be suppressed.
- [x] 2.3 Wire the validator as the documentation tier in the routed gate and add a `check-goplint-docs` Make target.

## 3. Harness Tier

- [x] 3.1 Define the harness tier in the soundness manifest/planner: module test suites, one shared repository audit, and fixture-driven serial-versus-parallel normalized-report parity within a one-minute fixture budget.
- [x] 3.2 Add tests proving parity divergence, lost observations, and population mismatches fail the tier, and that analyzer-semantics paths or forced events always escalate past it.
- [x] 3.3 Route harness-class diffs to the tier in pre-commit reporting and CI execution.

## 4. Dual-Digest Completion Record

- [x] 4.1 Bump the retained record format with semantic-content and prose digests computed from the ownership classes, keeping the previous format rejected with an explicit migration notice.
- [x] 4.2 Implement prose-only re-binding in evidence generation: recompute digests, revalidate ledgers and diff census, carry the retained aggregate report forward untouched, and record the carried-forward provenance.
- [x] 4.3 Fail re-binding closed on any semantic-content drift, naming the drifted paths; add adversarial tests for report tampering, digest mismatch, and mixed drift.
- [x] 4.4 Update the freshness verifier and completion gate to accept re-bound records and reject prose-stale single-digest claims.

## 5. Single-Source Task Ledgers

- [x] 5.1 Move expected task-ledger names, paths, order, and pending sets exclusively into the reviewed completion plan; delete the duplicated Go constants, schema value constants, and fixture copies.
- [x] 5.2 Keep structural validation (dependency order, archived-predecessor policy, well-formed pending sets) in code and schema without embedded expected values, with tests for each rejection.
- [x] 5.3 Prove closing or archiving a change requires editing only the reviewed plan, including the archive-path rename case.

## 6. Trigger Surfaces

- [x] 6.1 Cap the pre-commit hook at the documentation/consumer tiers, print the authoritative CI tier for heavier diffs, and keep explicit semantic/completion Make targets.
- [x] 6.2 Route CI from the cumulative pull-request diff classification; keep forced profiles for push-to-main, schedule, release, and dispatch.
- [x] 6.3 Add the one-release `GOPLINT_FORCE_SEMANTIC` escape hatch and document its removal criterion.

## 7. Documentation and Verification

- [x] 7.1 Update goplint agent docs, execution docs, commands reference, and rules for the four-class taxonomy, tier obligations, re-binding semantics, and single-source ledgers; run `make check-agent-docs`.
- [x] 7.2 Run both modules' test suites, lint, the routed gate at each tier on representative diffs, and one full semantic run; record tier timings in the performance report.
- [x] 7.3 Regenerate the retained completion record in the new format, verify freshness, run the completion gate, and validate the change strictly.
