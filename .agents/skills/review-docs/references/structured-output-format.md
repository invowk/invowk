# Review Findings Report Format

Use canonical JSON for subagent results. Validate and merge it with `scripts/review_docs.py`;
never merge free-form Markdown by hand.

## Contents

- [Subagent Result JSON](#subagent-result-json)
- [Status Contract](#status-contract)
- [Finding Admission Gate](#finding-admission-gate)
- [Finding Object](#finding-object)
- [Candidate Observations](#candidate-observations)
- [Severity and Finding Types](#severity-and-finding-types)
- [Generated Report](#generated-report)
- [Merge Contract](#merge-contract)

## Subagent Result JSON

Return exactly one JSON object with one item for every assigned check. Save it outside the
repository as `S{N}.json`, then validate it before accepting the subagent result:

```json
{
  "schema_version": 1,
  "surface": "S1",
  "context_id": "<context_id from context.json>",
  "items": [
    {
      "check_id": "S1-C01",
      "status": "PASS",
      "targets": ["README.md"],
      "evidence": ["README.md heading Features matches pkg and cmd capabilities"],
      "findings": []
    }
  ],
  "candidates": []
}
```

Run:

```bash
.agents/skills/review-docs/scripts/review_docs.py validate-result \
  --context /tmp/review-docs/context.json \
  --input /tmp/review-docs/results/S1.json
```

Unknown fields, duplicate or missing checks, stale snapshots, invalid repository paths, and
status/finding inconsistencies are validation errors.

Active surfaces are fixed and ordered:

| ID | Surface |
|---|---|
| S1 | README |
| S2 | Website Docs |
| S3 | Snippet Data and CUE Drift |
| S4 | i18n Parity |
| S5 | Architecture Diagrams |
| S6 | Container Image Policy |
| S7 | Config Defaults vs Docs |
| S8 | Homepage and Terminal Demo |
| S9 | Security Audit Docs |
| S10 | LLM-Assisted Agent Authoring |
| S11 | Agent Workflow Docs |

## Status Contract

- **PASS** — Verification completed and the documentation is accurate. Require non-empty evidence
  and no findings.
- **FAIL** — Verification completed and found an objective mismatch. Require evidence and at least
  one complete finding.
- **SKIP** — A registered simplification explicitly exempts the entire check across all targets.
  Require `simplification_id`, a matching Whole-Check SKIP ID in the registry, and no findings.
- **BLOCKED** — Verification could not be completed. Require a non-empty `reason` and no findings.

Every item must list the exact repository-relative `targets` it reviewed. The validator requires
all pages assigned to that check by `doc-ownership.json`; generic evidence cannot substitute for
page-level coverage. Never convert missing evidence or an incomplete finding into PASS, SKIP, or
a candidate. Use BLOCKED. Any BLOCKED item makes the audit verdict **INCOMPLETE**.

## Finding Admission Gate

Before emitting a finding, verify that all of these are true:

1. The issue maps to one explicit check ID from `surface-checklists.md`.
2. The documentation target is exact: repository-relative path plus line number, heading, or
   snippet ID.
3. The source of truth is exact: repository-relative path plus function, command, schema
   definition, constant, or programmatic result.
4. The mismatch is objective and factual: invalid syntax, wrong field/flag/default/command,
   missing required coverage, stale generated asset, broken navigation, or policy violation.
5. The expected content is directly inferable from the source of truth.
6. The issue is not registered in `intentional-simplifications.md`.

If conditions 1-5 cannot be established, mark the item BLOCKED. If the documentation is
acceptable, mark it PASS. If condition 6 applies only to a narrow mismatch, omit that finding,
cite the registry ID in evidence, and continue reviewing the check. Mark the entire item SKIP only
when the registry explicitly lists that check under Whole-Check SKIP IDs.

Do not create findings for style preference, tone, wording nuance, readability taste, unmeasured
risk, or possible contradictions without exact evidence.

## Finding Object

Place findings inside the producing FAIL item. Do not include severity, finding type, surface, or
RD ID; the merge tool derives them from the checklist and assigns IDs after deterministic sorting.

```json
{
  "target": {"path": "README.md", "locator": "Dependencies heading"},
  "source": {"path": "pkg/invowkfile/invowkfile_schema.cue", "symbol": "#DependsOn"},
  "current": "Brief current content",
  "expected": "Content directly implied by the source of truth",
  "rationale": "One sentence explaining the objective mismatch."
}
```

## Candidate Observations

Use the top-level `candidates` array only for non-blocking notes that do not satisfy a checklist
finding. Candidates never receive RD IDs or severity counts. Do not use candidates to hide missing
evidence; use BLOCKED for that.

## Severity and Finding Types

Take severity and type from `surface-checklists.md`; agents must not override them.

| Severity | Meaning |
|---|---|
| ERROR | Factually wrong and likely to cause errors or materially mislead users |
| WARNING | Objectively incomplete or outdated |
| INFO | Objective non-blocking drift |

SKIP is a checklist status, not a finding severity. Use only the finding types preassigned in the
checklist (`coverage-gap`, `source-drift`, `schema-drift`, `cli-contract-drift`, `snippet-drift`,
`i18n-structural-drift`, `i18n-prose-staleness`, `diagram-drift`, `policy-violation`,
`security-contract-drift`, `generated-asset-drift`, `navigation-drift`, and `agent-docs-drift`).

## Generated Report

Generate Markdown and canonical JSON only through the merge command:

```bash
.agents/skills/review-docs/scripts/review_docs.py merge \
  --context /tmp/review-docs/context.json \
  --results /tmp/review-docs/results \
  --output /tmp/review-docs/report.md \
  --output-json /tmp/review-docs/report.json
```

The report includes verdict COMPLETE or INCOMPLETE, programmatic checks, findings, and completion
counts for PASS, FAIL, SKIP, and BLOCKED. Exit code 3 means a report was produced but the audit is
incomplete; exit code 2 means the input contract is invalid.

## Merge Contract

1. Validate all 11 result artifacts before merging anything.
2. Require the context and every result to use the current workspace snapshot.
3. Reject incomplete findings instead of silently downgrading them.
4. Deduplicate by normalized target path, locator, current content, and expected content so two
   distinct issues at one line remain distinct.
5. Preserve all producing check IDs as provenance and select the highest fixed severity.
6. Sort by severity, numeric surface, numeric check, target path, and locator.
7. Assign sequential RD IDs only after sorting.
8. Emit INCOMPLETE whenever a checklist item, programmatic gate, or i18n lag calculation is
   BLOCKED.
