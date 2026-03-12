# Review Findings Report Format

Use this format for all documentation review findings. The structured format enables
deduplication across subagents and prioritized triage.

## Report Header

```
# Documentation Review Report
- Date: YYYY-MM-DD
- Surfaces covered: [list of S1-S8 reviewed]
- Programmatic checks: [PASS/FAIL for each automated check]
```

## Finding Entry Template

Each finding is one row. Use this format:

| Field | Description |
|---|---|
| **ID** | `RD-{NNN}` — sequential within the report |
| **Surface** | One of: README, Website, Snippet, i18n, Diagram, ContainerPolicy, Config |
| **Severity** | ERROR / WARNING / INFO / SKIP |
| **File** | Path relative to repo root |
| **Line(s)** | Line number(s) or snippet ID |
| **Source of Truth** | The authoritative file/function that defines correct behavior |
| **Current Content** | Brief quote of what the doc says (keep concise) |
| **Expected Content** | What the doc should say based on the source of truth |
| **Rationale** | Why this is a finding (one sentence) |

### Severity Definitions

| Severity | Criteria | Action Required |
|---|---|---|
| **ERROR** | Factually wrong: would mislead users, cause errors, or show invalid syntax | Must fix before next release |
| **WARNING** | Incomplete or outdated: missing recent feature, stale default value, or unclear | Should fix soon |
| **INFO** | Style issue, minor improvement opportunity, or non-blocking suggestion | Fix when convenient |
| **SKIP** | Intentional simplification (documented in `intentional-simplifications.md`) | No action needed |

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

### Priority Fix List
[ERRORs first, then WARNINGs, with file paths for quick navigation]
```

## Merge Procedure (for coordinator)

When merging findings from multiple subagents:

1. **Collect** all findings from SA-1 through SA-4.
2. **Deduplicate** by (file, line/snippet ID) — if two subagents found the same issue, keep
   the one with higher severity and more detail.
3. **Cross-check** against `references/intentional-simplifications.md` — downgrade any finding
   that matches the registry to SKIP.
4. **Sort** by severity (ERROR → WARNING → INFO → SKIP), then by surface.
5. **Assign sequential IDs** (RD-001, RD-002, ...) to the merged list.
6. **Produce** the summary table and priority fix list.
