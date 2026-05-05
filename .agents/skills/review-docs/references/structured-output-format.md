# Review Findings Report Format

Use this format for all documentation review findings. The structured format enables
deduplication across subagents and prioritized triage.

## Report Header

```
# Documentation Review Report
- Date: YYYY-MM-DD
- Surfaces covered: [list of S1-S11 reviewed]
- Programmatic checks: [PASS/FAIL for each automated check]
```

## Checklist Status (per-subagent output)

Each subagent produces a checklist status table BEFORE listing findings. Every item from
`surface-checklists.md` for the assigned surface must appear — no item may be omitted.

```
## Checklist Status — S{N}: {Surface Name}

| Check ID | Status | Evidence |
|----------|--------|----------|
| S1-C01   | PASS   | Field names match (verified against invowkfile_schema.cue L45-L120) |
| S1-C02   | FAIL   | `custom` dependency type missing from README L486 |
| S1-C03   | N/A    | invowk init --help not available in this environment |
| ...      | ...    | ... |
```

**Status values**:
- **PASS** — Check verified, documentation is accurate. Include brief evidence (file + line or observation).
- **FAIL** — Documentation does not match source of truth. A finding entry is required below.
- **N/A** — Check could not be performed (e.g., command not available, file not found). Explain why.

Findings are generated FROM failed checklist items. Every finding must trace back to a specific
check ID. Do not generate findings that are not associated with a checklist item.

## Finding Entry Template

Each finding is one row. Use this format:

| Field | Description |
|---|---|
| **ID** | `RD-{NNN}` — sequential within the report |
| **Check ID** | The checklist item that produced this finding (e.g., `S1-C04`) |
| **Surface** | One of: README, Website, Snippet, i18n, Diagram, ContainerPolicy, Config, Homepage, SecurityAudit, LLMAuthoring, AgentDocs |
| **Finding Type** | Use the pre-assigned type from the checklist, e.g. `schema-drift`, `security-contract-drift`, or `coverage-gap` |
| **Severity** | ERROR / WARNING / INFO / SKIP — use the pre-assigned severity from the checklist |
| **File** | Path relative to repo root |
| **Line(s)** | Line number(s) or snippet ID |
| **Source of Truth** | The authoritative file/function that defines correct behavior |
| **Current Content** | Brief quote of what the doc says (keep concise) |
| **Expected Content** | What the doc should say based on the source of truth |
| **Rationale** | Why this is a finding (one sentence) |

### Severity Definitions

Severity is pre-assigned per checklist item in `surface-checklists.md`. Use the checklist's
severity — do not override it based on subjective judgment. The definitions below explain
what each level means:

| Severity | Criteria | Action Required |
|---|---|---|
| **ERROR** | Factually wrong: would mislead users, cause errors, or show invalid syntax | Must fix before next release |
| **WARNING** | Incomplete or outdated: missing recent feature, stale default value, or unclear | Should fix soon |
| **INFO** | Style issue, minor improvement opportunity, or non-blocking suggestion | Fix when convenient |
| **SKIP** | Intentional simplification (documented in `intentional-simplifications.md`) | No action needed |

### Finding Type Definitions

Finding Type is pre-assigned per checklist item in `surface-checklists.md`. Use the checklist's
type exactly to keep cross-run grouping stable:

| Type | Meaning |
|---|---|
| `coverage-gap` | A live documentation or workflow surface is not assigned to any checklist |
| `source-drift` | Prose no longer matches implementation behavior or source layout |
| `schema-drift` | Documented CUE/Go schema fields, types, defaults, or validation rules are stale |
| `cli-contract-drift` | CLI command names, flags, arguments, output, or exit behavior are stale |
| `snippet-drift` | Snippet data or snippet references are missing, stale, or syntactically invalid |
| `i18n-structural-drift` | Translation file, snippet ID, or diagram ID structure diverges from English |
| `i18n-prose-staleness` | Translated prose preserves stale facts after English changed |
| `diagram-drift` | D2 sources, rendered diagrams, or architecture diagram prose drifted |
| `policy-violation` | Documentation violates a repository policy such as container image policy |
| `security-contract-drift` | Security/audit docs drift from command, checker, JSON, or LLM contracts |
| `generated-asset-drift` | Generated snippet/diagram version assets are stale or not validated |
| `navigation-drift` | Docs exist but are missing from navigation, sidebars, links, or build references |
| `agent-docs-drift` | Agent-facing skills, commands, or indexes are stale relative to workflow contracts |

## Summary Table

End the report with aggregate counts:

```
## Summary

| Severity | Count |
|---|---|
| ERROR | N |
| WARNING | N |
| INFO | N |
| SKIP | N |

### Checklist Completion

| Surface | Total Items | PASS | FAIL | N/A |
|---|---|---|---|---|
| S1: README | 22 | ... | ... | ... |
| S2: Website | 14 | ... | ... | ... |
| S3: Snippet | 19 | ... | ... | ... |
| S4: i18n | 6 | ... | ... | ... |
| S5: Diagram | 11 | ... | ... | ... |
| S6: ContainerPolicy | 6 | ... | ... | ... |
| S7: Config | 7 | ... | ... | ... |
| S8: Homepage | 5 | ... | ... | ... |
| S9: SecurityAudit | 10 | ... | ... | ... |
| S10: LLMAuthoring | 8 | ... | ... | ... |
| S11: AgentDocs | 9 | ... | ... | ... |

### Breakdown by Surface

| Surface | ERROR | WARNING | INFO | SKIP |
|---|---|---|---|---|
| README | ... | ... | ... | ... |
| Website | ... | ... | ... | ... |
| Snippet | ... | ... | ... | ... |
| i18n | ... | ... | ... | ... |
| Diagram | ... | ... | ... | ... |
| ContainerPolicy | ... | ... | ... | ... |
| Config | ... | ... | ... | ... |
| Homepage | ... | ... | ... | ... |
| SecurityAudit | ... | ... | ... | ... |
| LLMAuthoring | ... | ... | ... | ... |
| AgentDocs | ... | ... | ... | ... |

### Priority Fix List
[ERRORs first, then WARNINGs, with file paths for quick navigation]
```

## Merge Procedure (for coordinator)

When merging findings from the 11 surface-dedicated subagents (SA-1 through SA-11):

1. **Verify completeness** — Each subagent must have reported on ALL checklist items for its
   surface. Flag any missing items (the subagent may need to re-run or explain the gap).
2. **Collect** all findings from SA-1 through SA-11.
3. **Deduplicate** by (file, line/snippet ID) — if two subagents found the same issue (possible
   for cross-cutting concerns), keep the one with higher severity and more detail.
4. **Cross-check** against `references/intentional-simplifications.md` — downgrade any finding
   that matches the registry to SKIP.
5. **Sort** by severity (ERROR → WARNING → INFO → SKIP), then by surface.
6. **Assign sequential IDs** (RD-001, RD-002, ...) to the merged list.
7. **Merge checklist tables** — Combine the per-subagent checklist tables into the unified
   Checklist Completion summary.
8. **Produce** the summary table, checklist completion, and priority fix list.
