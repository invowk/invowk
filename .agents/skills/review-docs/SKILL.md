---
name: review-docs
description: Comprehensive documentation review and audit for invowk. Checks README, website docs (next version only), MDX snippets, CUE schema alignment, i18n parity, architecture diagrams, container image policy, security/audit docs, LLM authoring docs, and agent workflow docs against the actual codebase. Use this when reviewing documentation accuracy, when code changes may have caused doc drift, after significant feature work, or before releases. Always use this skill for any documentation review task, even if the user doesn't explicitly say "review docs" — any mention of checking docs, verifying documentation accuracy, or ensuring docs match code should trigger this skill.
---

# Documentation Review and Audit

This skill orchestrates a structured review of all documentation surfaces to ensure they
accurately reflect the current codebase. It is review-only — use the `/docs` skill for
editing and the `/d2-diagrams` skill for diagram creation.

The review is closed-world and evidence-gated. A counted finding must come from a checklist
item, cite an exact documentation target, cite an exact source of truth, and describe an
objective factual mismatch. Style preferences, wording nuance, missing nice-to-have context,
and speculative risk are not findings unless a checklist item explicitly requires them.

## Purpose and Scope

**Review**: README.md, website docs (next version only), MDX snippets, CUE examples, i18n
parity, architecture diagrams, container image policy, DefaultConfig() accuracy, security/audit
docs, LLM-assisted authoring docs, and agent workflow docs.

**Do NOT**:
- Edit documentation (use `/docs` for that after review identifies issues)
- Touch versioned docs (`versioned_docs/version-*/`) — they are frozen snapshots
- Touch version snapshot files (`Snippet/versions/`, `Diagram/versions/`) — auto-generated
- Flag intentional simplifications as errors (check `references/intentional-simplifications.md`)

---

## Review Surfaces

The review covers 11 surfaces. Each has a source of truth in the codebase.

### S1: README.md

The README (~2870 lines) is the primary external-facing documentation. Key drift-prone
sections: Invowkfile Format, Dependencies, Command Flags/Arguments, Module Dependencies,
Configuration, and TUI Components.

Read `references/readme-sync-map.md` for the full section → source-of-truth mapping.

### S2: Website Docs (next version only)

MDX pages discovered by the live inventory in `website/docs/` across the current sections plus
`architecture/`. Source of truth is the Go code and CUE schemas. Only review `website/docs/`
(the "Next" unreleased version at `/docs/next/`), never `website/versioned_docs/`.

Read `references/consolidated-sync-map.md` for the full code → docs mapping.

### S3: Snippet Data and CUE Schema Drift

The live inventory of `website/src/components/Snippet/data/*.ts` contains all reusable code
examples. CUE snippets are the most drift-prone because the schema evolves faster than the
examples.

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

The live D2 source inventory in `docs/diagrams/`, architecture prose docs in
`docs/architecture/`, and website architecture pages in `website/docs/architecture/`.

Focus: Do diagram nodes, labels, and relationships match current package names and code
structure? Delegate detailed D2 review (formatting, readability rules, rendering) to the
`/d2-diagrams` skill.

### S6: Container Image Policy

ALL container examples across all documentation surfaces must use `debian:stable-slim`.
No `ubuntu:*`, no Alpine, no Windows containers. Language-specific images (`golang:1.26`,
`python:3-slim`, `node:22-slim`) are allowed only in language-specific runtime demonstrations.

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

### S9: Security Audit Docs

`website/docs/security/audit.mdx`, README security sections, and security snippets document the
`invowk audit` command. Source of truth is `cmd/invowk/audit.go`, `cmd/invowk/llm_flags.go`,
`internal/audit/`, `internal/auditllm/`, and `internal/app/llmconfig/`.

### S10: LLM-Assisted Command Authoring Docs

`website/docs/advanced/llm-assisted-authoring.mdx`, README LLM authoring sections, and related
snippets document `invowk agent cmd`. Source of truth is `cmd/invowk/agent.go`,
`internal/agentcmd/`, shared LLM flags, and the LLM config resolver.

### S11: Agent Workflow Docs

Agent-facing docs must stay in sync with this skill and the current repository surfaces. Review
`.agents/commands/review-docs.md`, `.agents/skills/docs/SKILL.md`, `AGENTS.md`, and related
skill references when the review workflow changes.

---

## Consistency Principles

These principles ensure that running the review multiple times produces the same results.
They apply to both the coordinator and all subagents.

1. **Checklist-driven review** — Each subagent follows its surface's checklist from
   `references/surface-checklists.md`. Every checklist item gets a status (PASS/FAIL/N/A).
   This is the review activity. Open-ended exploration may only produce uncounted candidate
   observations for the coordinator, not findings.

2. **Pre-assigned severity** — Each checklist item has a severity level defined in
   `surface-checklists.md`. Subagents use that severity, not their own judgment. The reason:
   subjective severity classification is the second-largest source of run-to-run variance
   (after scope sampling). Fixing severity at definition time eliminates this.

3. **Deterministic file traversal** — Each checklist enumerates the exact files to review.
   Subagents check all listed files, not a sample. Live inventories are sorted with
   `LC_ALL=C sort` and passed to every subagent in the Context Block. Subagents do not
   recalculate or reinterpret inventory membership.

4. **Structured context passing** — The coordinator passes programmatic check results to
   subagents using the Context Block format defined below, not free-form prose. This ensures
   every subagent receives identical context in an identical format.

5. **Complete reporting** — Every checklist item must appear in the subagent's output. Items
   that pass are reported as PASS with brief evidence. Items that cannot be checked are N/A
   with an explanation. The coordinator verifies completeness during merge.

6. **Finding admission gate** — A FAIL may become a finding only when it has all fields required
   by `references/structured-output-format.md`: check ID, exact doc path and line/snippet ID,
   exact source-of-truth file and symbol/command/schema section, current content, expected
   content, and one-sentence rationale. If any field is missing, report N/A or an uncounted
   candidate observation instead of a finding.

7. **Inventory-first coverage** — Before spawning subagents, the coordinator records a live
   inventory of docs, snippet data files, sidebars, diagram sources, and agent workflow docs.
   Any file or section not assigned to a checklist is itself a finding.

8. **Stable audit snapshot** — Record the audit date and `git rev-parse HEAD` once in Step 1.
   All subagents use that snapshot. If the working tree changes during the review, rerun Step 1
   and restart the affected subagents with the new Context Block.

---

## Orchestration Strategy

### Step 1: Run Programmatic Checks

Run automated checks first to catch mechanical issues. See `references/verification-commands.md`
for full details and failure triage.

```bash
export LC_ALL=C
git rev-parse HEAD

# Parallel group 1
cd website && npm run docs:parity
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' README.md website/docs/ website/src/components/Snippet/data/ website/i18n/

# Parallel group 2
./scripts/check-diagram-readability.sh
while IFS= read -r f; do
  d2 validate "$f" 2>&1
done < <(LC_ALL=C find docs/diagrams -path '*/experiments/*' -prune -o -type f -name '*.d2' -print | LC_ALL=C sort)

# Sequential
make check-agent-docs
node scripts/validate-version-assets.mjs
cd website && npm run typecheck
cd website && npm run build
```

Record results in the **Context Block** format:

```
PROGRAMMATIC CHECK RESULTS
==========================
audit-date         : YYYY-MM-DD
git-head           : <commit sha>
docs:parity        : PASS | FAIL (detail)
container-grep     : PASS | FAIL (files: ...)
diagram-readability: PASS | FAIL (detail)
d2-validate        : PASS | FAIL (files: ...)
check-agent-docs   : PASS | FAIL (detail)
version-assets     : PASS | FAIL (detail)
website-typecheck  : PASS | FAIL (detail)
website-build      : PASS | FAIL (detail)
doc-inventory      : PASS | FAIL (unassigned files/surfaces: ...)
inventory-counts   : website-docs=N, snippets=N, d2=N, architecture=N, agent-docs=N
==========================
```

### Step 2: Spawn 11 Parallel Subagents

Spawn one subagent per surface. Each subagent receives:
1. The Context Block from Step 1
2. Its assigned surface checklist section from `references/surface-checklists.md`
3. The structured output format from `references/structured-output-format.md`

**Codex CLI only**: Codex CLI currently supports at most 6 live subagents. When running
under Codex CLI, spawn subagents in deterministic order (SA-1, SA-2, ...) up to the
available live-subagent limit, then keep the remaining assigned surfaces pending. As each
subagent completes and a slot becomes available, launch the next pending surface with the
same prompt template and references below. The coordinator may run programmatic checks,
manage the pending queue, verify completeness, and merge reports; it must not perform
checklist review work assigned to any pending subagent.

| Subagent | Surface | References to Read | Focus |
|----------|---------|-------------------|-------|
| **SA-1: README** | S1 | `readme-sync-map.md`, `intentional-simplifications.md`, `surface-checklists.md` §S1 | Walk 22-section sync map, verify each section against its source of truth |
| **SA-2: Website Docs** | S2 | `consolidated-sync-map.md`, `intentional-simplifications.md`, `surface-checklists.md` §S2 | Verify MDX pages against Go code and CUE schemas using the code→docs map |
| **SA-3: Snippet Data & CUE Drift** | S3 | `cue-drift-patterns.md`, `intentional-simplifications.md`, `surface-checklists.md` §S3 | Apply 6 CUE drift patterns to the live snippet inventory systematically |
| **SA-4: i18n Parity** | S4 | `intentional-simplifications.md`, `surface-checklists.md` §S4 | Structural parity via `docs:parity` results, detect stale prose via the deterministic S4-C05 command |
| **SA-5: Architecture Diagrams** | S5 | `consolidated-sync-map.md` (diagram section), `surface-checklists.md` §S5 | D2 node/label accuracy vs current package names and code structure |
| **SA-6: Container Image Policy** | S6 | `surface-checklists.md` §S6 | Deep scan beyond Step 1 grep — CUE runtime fields, Dockerfiles in examples |
| **SA-7: DefaultConfig() vs Docs** | S7 | `consolidated-sync-map.md`, `surface-checklists.md` §S7 | Field-by-field comparison of DefaultConfig() output vs 4 doc pages + snippets |
| **SA-8: Homepage & Terminal Demo** | S8 | `intentional-simplifications.md`, `surface-checklists.md` §S8 | Verify simplifications are valid and syntax is not actively misleading |
| **SA-9: Security Audit Docs** | S9 | `surface-checklists.md` §S9, `consolidated-sync-map.md` | Verify audit command flags, JSON shape, checker categories, correlator rules, LLM opt-in behavior, and snippets |
| **SA-10: LLM-Assisted Command Authoring** | S10 | `surface-checklists.md` §S10, `consolidated-sync-map.md` | Verify `invowk agent cmd` docs, shared LLM flags/config, write modes, validation behavior, and snippets |
| **SA-11: Agent Workflow Docs** | S11 | `surface-checklists.md` §S11, `verification-commands.md` | Verify this workflow's command wrapper, docs skill references, AGENTS index text, and stale surface counts |

#### Subagent Prompt Template

Use this template when spawning each subagent. Consistent prompting is important because
variation in how the task is described to subagents is a source of run-to-run inconsistency.

```
You are reviewing documentation surface S{N}: {Surface Name} for the invowk project.

## Your Task
Follow the checklist in `references/surface-checklists.md` §S{N} item by item. For each
checklist item, report PASS, FAIL, or N/A with evidence. Then generate findings for
all FAIL items that satisfy the Finding Admission Gate in
`references/structured-output-format.md`.

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
5. Apply the Finding Admission Gate. If the mismatch is stylistic, subjective, speculative, or
   missing exact evidence, do not generate a finding.
6. Record status: PASS (with evidence), FAIL (generate finding), or N/A (with reason)

## Output
1. Checklist Status table (every item, no omissions)
2. Findings list (one entry per FAIL item, using the checklist's pre-assigned severity)
3. Candidate observations (optional, uncounted; coordinator does not include these in RD-* IDs
   unless they are converted into a checklist-backed coverage-gap finding)
```

### Step 3: Merge and Report

The coordinator:
1. **Verifies completeness** — Each subagent reported on all checklist items for its surface
2. **Rejects incomplete findings** — Findings that fail the admission gate are returned to the
   subagent as N/A/candidate observations, not merged
3. **Collects** findings from SA-1 through SA-11
4. **Deduplicates** by (file, line/snippet ID) — keep higher severity on conflicts
5. **Cross-checks** against `references/intentional-simplifications.md`
6. **Sorts** by severity (ERROR first), then surface ID, check ID, file path, and line/snippet ID
7. **Assigns** sequential IDs (RD-001, RD-002, ...) to the merged list
8. **Merges** checklist tables into a unified Checklist Completion summary
9. **Produces** the final report (see `references/structured-output-format.md`)

---

## Detailed References

Read these when working on the corresponding review surface:

- **[references/surface-checklists.md](references/surface-checklists.md)** — Per-surface
  enumerated verification items (117 total across 11 surfaces). This is the primary review driver.
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
