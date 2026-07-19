# Stable Finding ID and Protocol Fact Migration Review

Date: 2026-07-16

## Stable finding IDs

The current finding-ID authority is schema `gpl4`. IDs use the full import
path and source-local semantic AST/object identities. Package leaf names,
diagnostic messages, raw `token.Pos` values, and file-set load order are not
identity inputs. Protocol procedure, dynamic-index, and unsupported-predicate
witness keys follow the same rule so repeated finding metadata is stable as
well as the diagnostic ID.

The retained migration report is
[`stable-id-migration.v2.json`](../../close-residual-goplint-soundness-gaps/evidence/stable-id-migration.v2.json).
It compares the retained pre-v4 scan with two independently executed current
production scans after canonical sorting.

| Measure | Result |
|---|---:|
| Pre-v4 population | 25 |
| Current population | 66 |
| Repeat population | 66 |
| Comparable IDs changed from `gpl3` to `gpl4` | 1 |
| Reviewed additions | 65 |
| Reviewed removals | 24 |
| Collisions | 0 |
| Duplicate semantic findings | 0 |

After canonical sorting, both complete current streams have byte SHA-256
`ff89eb76450e37b1d64d0b073af195b290980b099e1b488d34747b0106f83b70`.
The migration's finding-only canonical SHA-256 is
`a3e4ba13be989200d006183a3e0ba1ac91f9729808528cb2cc20d6680452f95e`
for both runs. The report is deterministic and accepted.

The 65 additions are `nonzero-value-field` findings. The retained pre-v4
capture covered the protocol, constructor, and boundary producers but omitted
that inventoried structural producer. The 24 removals are also reviewed by
exact set digest:

- three `unvalidated-boundary-request` findings disappeared after validation
  failures stopped returning the invalid request value;
- one production `unvalidated-cast` now reuses the validated lock path, while
  the other fourteen removed casts are under the configured
  `internal/testutil/` production-scan exclusion; and
- six `unvalidated-cast-inconclusive` findings disappeared after the code kept
  validated typed locals or optional typed wrappers through pointer,
  named-return, and file-set-sensitive paths.

These are code and production-scope corrections, not category suppression.
The exact sets are digest-bound in
[`stable-id-population-review.v1.json`](../../close-residual-goplint-soundness-gaps/evidence/stable-id-population-review.v1.json).
An unreviewed, stale, count-only, or digest-mismatched population change makes
the migration report fail.

After all prerequisite semantic and assurance gates passed, including the
unchanged existing mutation profile, `make update-baseline` migrated the
production baseline to 66 `gpl4` entries. Two independent repetitions of
`make check-baseline`, `make check-goplint-exceptions`, and
`make check-goplint-full-scan` then passed with identical populations and zero
stale exceptions. Mutation verification is in scope; mutation-suite and
workflow expansion are not.

## Protocol summary fact v4 to v5

The active production authority is `ProtocolSummaryFact` version 5 in
`tools/goplint/goplint/protocol_summary_fact.go`.

Version 5 binds an imported fact to its exact package path, function name,
stable function identity, completeness state, signature slots, slot roles,
compatible types, condition result slot, and ordered effects. Its effect
vocabulary is:

- `pure` and `preserve`;
- conditional validation tied to an exact result-nil or successful-return
  condition;
- mutation, replacement, escape, and consume effects; and
- terminal effects.

Consumers accept only version 5 facts attached to the matching `types.Func`
whose owner, signature, slots, conditions, types, completeness, and effect
shapes validate. Missing, legacy, malformed, or incompatible facts are never
partially reinterpreted as a complete no-effect summary; a relevant dependent
obligation remains blocking inconclusive.

## Review evidence

The current migration contract is exercised by:

- stable-ID collision, layout-drift, shadowing, dynamic-index, procedure, and
  unsupported-predicate regressions;
- repeated production scans plus collision and duplicate rejection;
- exact digest-reviewed population-change tests in
  `internal/stableidmigration` and `cmd/stable-id-migration`;
- `TestValidateProtocolSummaryFactFailsClosed`;
- `TestProtocolSummaryFactFailuresHaveDeterministicStatuses`;
- `TestProtocolSummaryFactAcceptsExplicitEffectVocabulary`;
- `TestProtocolSummaryFactsExportAllExplicitEffects`;
- `TestProtocolSummaryFactFailsClosedForComplexInputEffects`; and
- the production-integration, counterexample, architecture, semantic-spec,
  oracle, fuzz-seed, and determinism gates.

The unchanged existing mutation profile and combined clean-tree completion
remain separately retained blocking evidence and are not implied by this
report.
