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
parity, architecture diagrams, container image policy, config default accuracy, security/audit
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

The live `README.md` is the primary external-facing documentation. Key drift-prone
sections: Invowkfile Format, Dependencies, Command Flags/Arguments, Module Dependencies,
Configuration, and TUI Components.

Read `references/readme-sync-map.md` for the full section → source-of-truth mapping.

### S2: Website Docs (next version only)

MDX pages discovered by the live inventory in `website/docs/` across the current sections plus
`architecture/`. Source of truth is the Go code and CUE schemas. Only review `website/docs/`
(the "Next" unreleased version at `/docs/next/`), never `website/versioned_docs/`.

Use `references/doc-ownership.json` as the exact page → semantic check → source-of-truth
contract. Read `references/consolidated-sync-map.md` for change-oriented code → docs guidance.

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

Programmatic check: `(cd website && npm run docs:parity)`

Manual check: Use the deterministic S4-C05 command in
`references/verification-commands.md` to select the first three English pages
for stale-prose review, then compare with the corresponding pt-BR files for
factual drift.

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

### S7: Config Defaults vs Docs

`internal/config/config_schema.cue` is the source of truth for configuration defaults.
`internal/config/types.go` `DefaultConfig()` must derive from that schema, and docs must match:
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

### S10: LLM-Assisted Agent Authoring Docs

`website/docs/advanced/llm-assisted-authoring.mdx`, README LLM authoring sections, and related
snippets document `invowk agent cmd` and `invowk agent mod`. Source of truth is `cmd/invowk/agent.go`,
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
   `references/surface-checklists.md`. Every checklist item gets a status
   (PASS/FAIL/SKIP/BLOCKED).
   This is the review activity. Open-ended exploration may only produce uncounted candidate
   observations for the coordinator, not findings.

2. **Pre-assigned severity** — Each checklist item has a severity level defined in
   `surface-checklists.md`. Subagents use that severity, not their own judgment. The reason:
   subjective severity classification is the second-largest source of run-to-run variance
   (after scope sampling). Fixing severity at definition time eliminates this.

3. **Deterministic file traversal** — `references/doc-ownership.json` gives every live website
   page exactly one non-mechanical semantic owner and resolvable sources of truth. The tooling
   rejects missing, stale, duplicate, case-colliding, or policy-only ownership. Subagents use the
   sorted inventories from `context.json`; they do not recalculate membership.

4. **Structured context passing** — Generate canonical `context.json` with
   `scripts/review_docs.py prepare`. Pass its path and context ID to every subagent; do not
   reconstruct context in prose.

5. **Complete reporting** — Every checklist item must appear in canonical result JSON. Use PASS
   with evidence, FAIL with admitted findings, SKIP only with an explicit whole-check exemption,
   or BLOCKED with a reason. Any BLOCKED status makes the audit INCOMPLETE.

6. **Finding admission gate** — A FAIL may become a finding only when it has all fields required
   by `references/structured-output-format.md`: check ID, exact doc path and line/snippet ID,
   exact source-of-truth file and symbol/command/schema section, current content, expected
   content, and one-sentence rationale. If required evidence cannot be gathered, report BLOCKED;
   never silently downgrade an incomplete finding.

7. **Inventory-first coverage** — Run `scripts/review_docs.py validate` before spawning
   subagents. Any live page without one semantic owner, any stale ownership entry, or any source
   glob that resolves to nothing is a contract failure and blocks the review.

8. **Stable audit snapshot** — Use the content hash in `context.json`, which includes tracked and
   untracked repository files, not HEAD alone. Validate it before accepting results and again
   before merge. If it changes, regenerate context and restart affected subagents.

---

## Orchestration Strategy

### Step 1: Run Programmatic Checks

Run automated checks first to catch mechanical issues. See `references/verification-commands.md`
for exact commands, the required checks JSON shape, and failure triage. Keep audit artifacts
outside the repository so they do not change the workspace snapshot.

```bash
export LC_ALL=C
git rev-parse HEAD

.agents/skills/review-docs/scripts/review_docs.py validate

# Parallel group 1
(cd website && npm run docs:parity)
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' README.md website/docs/ website/src/components/Snippet/data/ website/i18n/pt-BR/docusaurus-plugin-content-docs/current/ docs/architecture/

# Parallel group 2
./scripts/check-diagram-readability.sh
while IFS= read -r f; do
  d2 validate "$f" 2>&1
done < <(LC_ALL=C find docs/diagrams -path '*/experiments/*' -prune -o -type f -name '*.d2' -print | LC_ALL=C sort)
make check-diagram-renders

# Sequential
make check-agent-docs
node scripts/validate-version-assets.mjs
(cd website && npm run typecheck)
(cd website && npm run build)
```

Record the nine programmatic results in `/tmp/review-docs/checks.json`, then generate canonical
context and the content snapshot:

```bash
mkdir -p /tmp/review-docs/results
.agents/skills/review-docs/scripts/review_docs.py prepare \
  --checks /tmp/review-docs/checks.json \
  --output /tmp/review-docs/context.json
```

### Step 2: Run 11 Surface Subagents

Assign one subagent per surface. Use all available concurrency slots, queue remaining surfaces in
numeric order, and start the next pending surface whenever a slot opens. This rule is runtime
neutral; never assume a fixed agent limit.

Each subagent receives:

1. `/tmp/review-docs/context.json` and its `context_id`
2. Its assigned surface checklist section from `references/surface-checklists.md`
3. The structured output format from `references/structured-output-format.md`

| Subagent | Surface | References to Read | Focus |
|----------|---------|-------------------|-------|
| **SA-1: README** | S1 | `readme-sync-map.md`, `intentional-simplifications.md`, `surface-checklists.md` §S1 | Walk the README sync map, verify each section against its source of truth |
| **SA-2: Website Docs** | S2 | `doc-ownership.json`, `consolidated-sync-map.md`, `intentional-simplifications.md`, `surface-checklists.md` §S2 | Verify every assigned MDX page against its exact semantic sources |
| **SA-3: Snippet Data & CUE Drift** | S3 | `cue-drift-patterns.md`, `intentional-simplifications.md`, `surface-checklists.md` §S3 | Apply 6 CUE drift patterns to the live snippet inventory systematically |
| **SA-4: i18n Parity** | S4 | `intentional-simplifications.md`, `surface-checklists.md` §S4 | Structural parity via `docs:parity`; stale prose from computed English-vs-pt-BR commit lag |
| **SA-5: Architecture Diagrams** | S5 | `consolidated-sync-map.md` (diagram section), `surface-checklists.md` §S5 | D2 node/label accuracy vs current package names and code structure |
| **SA-6: Container Image Policy** | S6 | `surface-checklists.md` §S6 | Deep scan beyond Step 1 grep — CUE runtime fields, Dockerfiles in examples |
| **SA-7: Config Defaults vs Docs** | S7 | `consolidated-sync-map.md`, `surface-checklists.md` §S7 | Field-by-field comparison of CUE-derived defaults vs 4 doc pages + snippets |
| **SA-8: Homepage & Terminal Demo** | S8 | `intentional-simplifications.md`, `surface-checklists.md` §S8 | Verify simplifications are valid and syntax is not actively misleading |
| **SA-9: Security Audit Docs** | S9 | `surface-checklists.md` §S9, `consolidated-sync-map.md` | Verify audit command flags, JSON shape, checker categories, correlator rules, LLM opt-in behavior, and snippets |
| **SA-10: LLM-Assisted Agent Authoring** | S10 | `surface-checklists.md` §S10, `consolidated-sync-map.md` | Verify `invowk agent cmd` and `invowk agent mod` docs, shared LLM flags/config, write modes, validation behavior, and snippets |
| **SA-11: Agent Workflow Docs** | S11 | `surface-checklists.md` §S11, `verification-commands.md` | Verify this workflow's command wrapper, docs skill references, AGENTS index text, and stale surface counts |

#### Subagent Prompt Template

Use this template when spawning each subagent. Consistent prompting is important because
variation in how the task is described to subagents is a source of run-to-run inconsistency.

```
You are reviewing documentation surface S{N}: {Surface Name} for the invowk project.

## Your Task
Follow the checklist in `references/surface-checklists.md` §S{N} item by item. Return canonical
JSON conforming to `references/structured-output-format.md`, with exactly one item per check.

## Reference Files to Read
{list of reference files from the table above}

## Audit Context
Read `/tmp/review-docs/context.json`. Use context ID `{context_id}` and list every reviewed
repository path in the item's `targets` array.

## Per-Item Procedure
For each checklist item:
1. Read the source of truth file specified in the checklist
2. Read the documentation target file
3. Compare — does the doc accurately reflect the source of truth?
4. Check `references/intentional-simplifications.md` — is a mismatch deliberate?
5. Apply the Finding Admission Gate. Do not generate findings for style or speculation.
6. Record PASS, FAIL, SKIP, or BLOCKED according to the strict status contract.

## Output
Return only the canonical JSON object. The coordinator saves it as
`/tmp/review-docs/results/S{N}.json` and validates it with `scripts/review_docs.py validate-result`.
```

### Step 3: Merge and Report

Validate the snapshot and all 11 results, then merge deterministically:

```bash
.agents/skills/review-docs/scripts/review_docs.py snapshot-verify \
  --context /tmp/review-docs/context.json
.agents/skills/review-docs/scripts/review_docs.py merge \
  --context /tmp/review-docs/context.json \
  --results /tmp/review-docs/results \
  --output /tmp/review-docs/report.md \
  --output-json /tmp/review-docs/report.json
```

Do not hand-edit merged output. Exit 3 means the report exists but the audit is INCOMPLETE due to
BLOCKED work; resolve the blocker and rerun the affected surfaces before reporting completion.

---

## Detailed References

Read these when working on the corresponding review surface:

- **[references/surface-checklists.md](references/surface-checklists.md)** — Per-surface
  enumerated verification items. Derive totals with `scripts/review_docs.py validate`.
- **[references/doc-ownership.json](references/doc-ownership.json)** — Exact semantic owner and
  resolvable source-of-truth contract for every current website page.
- **[references/consolidated-sync-map.md](references/consolidated-sync-map.md)** — Superset
  code → docs mapping (website + diagrams + drift-prone areas)
- **[references/readme-sync-map.md](references/readme-sync-map.md)** — README section →
  source-of-truth mapping
- **[references/cue-drift-patterns.md](references/cue-drift-patterns.md)** — 6 CUE snippet
  drift patterns with detection and correction guidance
- **[references/intentional-simplifications.md](references/intentional-simplifications.md)** —
  Registry of known intentional omissions (do NOT flag as errors)
- **[references/structured-output-format.md](references/structured-output-format.md)** —
  Finding report template, checklist status format, severity definitions, merge procedure
- **[references/verification-commands.md](references/verification-commands.md)** — Full
  command reference with expected output and failure triage
- **[scripts/review_docs.py](scripts/review_docs.py)** — Contract validation, context/snapshot
  preparation, i18n lag selection, strict result validation, and deterministic report merging

---

## Related Skills

| Skill | When to Use |
|---|---|
| `/docs` | After review identifies issues — for editing documentation, syncing docs after code changes, creating pages, updating snippets |
| `/d2-diagrams` | For diagram creation, editing, rendering, and D2-specific readability review |
