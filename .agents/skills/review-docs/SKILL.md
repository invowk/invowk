---
name: review-docs
description: Comprehensive documentation review and audit for invowk. Checks README, website docs (next version only), MDX snippets, CUE schema alignment, i18n parity, architecture diagrams, and container image policy against the actual codebase. Use this when reviewing documentation accuracy, when code changes may have caused doc drift, after significant feature work, or before releases. Always use this skill for any documentation review task, even if the user doesn't explicitly say "review docs" — any mention of checking docs, verifying documentation accuracy, or ensuring docs match code should trigger this skill.
disable-model-invocation: false
---

# Documentation Review and Audit

This skill orchestrates a structured review of all documentation surfaces to ensure they
accurately reflect the current codebase. It is review-only — use the `/docs` skill for
editing and the `/d2-diagrams` skill for diagram creation.

## Purpose and Scope

**Review**: README.md, website docs (next version only), MDX snippets, CUE examples, i18n
parity, architecture diagrams, container image policy, and DefaultConfig() accuracy.

**Do NOT**:
- Edit documentation (use `/docs` for that after review identifies issues)
- Touch versioned docs (`versioned_docs/version-*/`) — they are frozen snapshots
- Touch version snapshot files (`Snippet/versions/`, `Diagram/versions/`) — auto-generated
- Flag intentional simplifications as errors (check `references/intentional-simplifications.md`)

---

## Review Surfaces

The review covers 8 surfaces. Each has a source of truth in the codebase.

### S1: README.md

The README (~2870 lines) is the primary external-facing documentation. Key drift-prone
sections: Invowkfile Format, Dependencies, Command Flags/Arguments, Module Dependencies,
Configuration, and TUI Components.

Read `references/readme-sync-map.md` for the full section → source-of-truth mapping.

### S2: Website Docs (next version only)

56+ MDX pages in `website/docs/` across 11 sections plus `architecture/`. Source of truth
is the Go code and CUE schemas. Only review `website/docs/` (the "Next" unreleased version
at `/docs/next/`), never `website/versioned_docs/`.

Read `references/consolidated-sync-map.md` for the full code → docs mapping.

### S3: Snippet Data and CUE Schema Drift

11 snippet data files in `website/src/components/Snippet/data/*.ts` contain all reusable
code examples. CUE snippets are the most drift-prone because the schema evolves faster than
the examples.

The #1 drift pattern is missing `platforms` field in implementation blocks. Five additional
patterns are cataloged in `references/cue-drift-patterns.md` (6 total).

**Important**: Partial/fragment snippets that show individual CUE fields (e.g., just
`runtimes:` config) are intentionally incomplete and exempt from schema completeness checks.
Only flag snippets that show a full `cmds` entry with implementation blocks but lack required
fields.

### S4: i18n Parity

pt-BR translations must mirror English docs in structure, `<Snippet>` IDs, and `<Diagram>` IDs.
Prose is translated; code examples are shared via the `<Snippet>` component.

Programmatic check: `cd website && npm run docs:parity`

Manual check: Use `git log --diff-filter=M -- website/docs/` to identify English pages modified
recently, then compare with the corresponding pt-BR files for stale or contradictory prose.

### S5: Architecture Diagrams

23 D2 source files in `docs/diagrams/`, 8 architecture prose docs in `docs/architecture/`,
and website architecture pages in `website/docs/architecture/`.

Focus: Do diagram nodes, labels, and relationships match current package names and code
structure? Delegate detailed D2 review (formatting, readability rules, rendering) to the
`/d2-diagrams` skill.

### S6: Container Image Policy

ALL container examples across all documentation surfaces must use `debian:stable-slim`.
No `ubuntu:*`, no Alpine, no Windows containers. Language-specific images (`golang:1.26`,
`python:3-slim`) are allowed only in language-specific runtime demonstrations.

### S7: DefaultConfig() vs Docs

`internal/config/types.go` `DefaultConfig()` is the source of truth for configuration
defaults. Must match:
- `website/docs/reference/config-schema.mdx` (default values)
- `website/docs/configuration/options.mdx` (default values)
- pt-BR mirrors of both pages

Also check dual-prefix config snippets: `config/*` and `reference/config/*` in
`Snippet/data/config.ts` must both be updated when config options change.

### S8: Homepage and Terminal Demo

`website/src/pages/index.tsx` contains an intentionally simplified terminal demo for
marketing purposes. Do NOT flag its simplifications as errors.

Check `references/intentional-simplifications.md` before flagging anything in the homepage,
Quick Start, or getting-started sections.

---

## Consistency Principles

These principles ensure that running the review multiple times produces the same results.
They apply to both the coordinator and all subagents.

1. **Checklist-driven review** — Each subagent follows its surface's checklist from
   `references/surface-checklists.md`. Every checklist item gets a status (PASS/FAIL/N-A).
   This is the primary review activity — open-ended exploration is secondary and optional.

2. **Pre-assigned severity** — Each checklist item has a severity level defined in
   `surface-checklists.md`. Subagents use that severity, not their own judgment. The reason:
   subjective severity classification is the second-largest source of run-to-run variance
   (after scope sampling). Fixing severity at definition time eliminates this.

3. **Deterministic file traversal** — Each checklist enumerates the exact files to review.
   Subagents check all listed files, not a sample. For surfaces with many files (S2, S3),
   the checklist specifies which files to examine.

4. **Structured context passing** — The coordinator passes programmatic check results to
   subagents using the Context Block format defined below, not free-form prose. This ensures
   every subagent receives identical context in an identical format.

5. **Complete reporting** — Every checklist item must appear in the subagent's output. Items
   that pass are reported as PASS with brief evidence. Items that cannot be checked are N/A
   with an explanation. The coordinator verifies completeness during merge.

---

## Orchestration Strategy

### Step 1: Run Programmatic Checks

Run automated checks first to catch mechanical issues. See `references/verification-commands.md`
for full details and failure triage.

```bash
# Parallel group 1
cd website && npm run docs:parity
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' README.md website/docs/ website/src/components/Snippet/data/ website/i18n/

# Parallel group 2
./scripts/check-diagram-readability.sh
for f in docs/diagrams/**/*.d2; do d2 validate "$f" 2>&1; done

# Sequential
make check-agent-docs
cd website && npm run build
```

Record results in the **Context Block** format:

```
PROGRAMMATIC CHECK RESULTS
==========================
docs:parity        : PASS | FAIL (detail)
container-grep     : PASS | FAIL (files: ...)
diagram-readability: PASS | FAIL (detail)
d2-validate        : PASS | FAIL (files: ...)
check-agent-docs   : PASS | FAIL (detail)
website-build      : PASS | FAIL (detail)
==========================
```

### Step 2: Spawn 8 Parallel Subagents

Spawn one subagent per surface. Each subagent receives:
1. The Context Block from Step 1
2. Its assigned surface checklist section from `references/surface-checklists.md`
3. The structured output format from `references/structured-output-format.md`

| Subagent | Surface | References to Read | Focus |
|----------|---------|-------------------|-------|
| **SA-1: README** | S1 | `readme-sync-map.md`, `intentional-simplifications.md`, `surface-checklists.md` §S1 | Walk 22-section sync map, verify each section against its source of truth |
| **SA-2: Website Docs** | S2 | `consolidated-sync-map.md`, `intentional-simplifications.md`, `surface-checklists.md` §S2 | Verify MDX pages against Go code and CUE schemas using the code→docs map |
| **SA-3: Snippet Data & CUE Drift** | S3 | `cue-drift-patterns.md`, `intentional-simplifications.md`, `surface-checklists.md` §S3 | Apply 6 CUE drift patterns to all 11 snippet data files systematically |
| **SA-4: i18n Parity** | S4 | `intentional-simplifications.md`, `surface-checklists.md` §S4 | Structural parity via `docs:parity` results, detect stale prose via git dates |
| **SA-5: Architecture Diagrams** | S5 | `consolidated-sync-map.md` (diagram section), `surface-checklists.md` §S5 | D2 node/label accuracy vs current package names and code structure |
| **SA-6: Container Image Policy** | S6 | `surface-checklists.md` §S6 | Deep scan beyond Step 1 grep — CUE runtime fields, Dockerfiles in examples |
| **SA-7: DefaultConfig() vs Docs** | S7 | `consolidated-sync-map.md`, `surface-checklists.md` §S7 | Field-by-field comparison of DefaultConfig() output vs 4 doc pages + snippets |
| **SA-8: Homepage & Terminal Demo** | S8 | `intentional-simplifications.md`, `surface-checklists.md` §S8 | Verify simplifications are valid and syntax is not actively misleading |

#### Subagent Prompt Template

Use this template when spawning each subagent. Consistent prompting is important because
variation in how the task is described to subagents is a source of run-to-run inconsistency.

```
You are reviewing documentation surface S{N}: {Surface Name} for the invowk project.

## Your Task
Follow the checklist in `references/surface-checklists.md` §S{N} item by item. For each
checklist item, report PASS, FAIL, or N/A with evidence. Then generate findings for
all FAIL items using the format in `references/structured-output-format.md`.

## Reference Files to Read
{list of reference files from the table above}

## Programmatic Check Results (from coordinator)
{paste the Context Block here}

## Per-Item Procedure
For each checklist item:
1. Read the source of truth file specified in the checklist
2. Read the documentation target file
3. Compare — does the doc accurately reflect the source of truth?
4. Check `references/intentional-simplifications.md` — is a mismatch deliberate?
5. Record status: PASS (with evidence), FAIL (generate finding), or N/A (with reason)

## Output
1. Checklist Status table (every item, no omissions)
2. Findings list (one entry per FAIL item, using the checklist's pre-assigned severity)
```

### Step 3: Merge and Report

The coordinator:
1. **Verifies completeness** — Each subagent reported on all checklist items for its surface
2. **Collects** findings from SA-1 through SA-8
3. **Deduplicates** by (file, line/snippet ID) — keep higher severity on conflicts
4. **Cross-checks** against `references/intentional-simplifications.md`
5. **Sorts** by severity (ERROR first), then by surface
6. **Assigns** sequential IDs (RD-001, RD-002, ...) to the merged list
7. **Merges** checklist tables into a unified Checklist Completion summary
8. **Produces** the final report (see `references/structured-output-format.md`)

---

## Detailed References

Read these when working on the corresponding review surface:

- **[references/surface-checklists.md](references/surface-checklists.md)** — Per-surface
  enumerated verification items (88 total across 8 surfaces). This is the primary review driver.
- **[references/consolidated-sync-map.md](references/consolidated-sync-map.md)** — Superset
  code → docs mapping (website + diagrams + drift-prone areas)
- **[references/readme-sync-map.md](references/readme-sync-map.md)** — README section →
  source-of-truth mapping (~22 sections)
- **[references/cue-drift-patterns.md](references/cue-drift-patterns.md)** — 6 CUE snippet
  drift patterns with detection and correction guidance
- **[references/intentional-simplifications.md](references/intentional-simplifications.md)** —
  Registry of known intentional omissions (do NOT flag as errors)
- **[references/structured-output-format.md](references/structured-output-format.md)** —
  Finding report template, checklist status format, severity definitions, merge procedure
- **[references/verification-commands.md](references/verification-commands.md)** — Full
  command reference with expected output and failure triage

---

## Related Skills

| Skill | When to Use |
|---|---|
| `/docs` | After review identifies issues — for editing documentation, syncing docs after code changes, creating pages, updating snippets |
| `/d2-diagrams` | For diagram creation, editing, rendering, and D2-specific readability review |
