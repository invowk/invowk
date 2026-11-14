---
name: review-docs
description: Comprehensive documentation review and audit for invowk. Checks README, website docs (next version only), MDX snippets, CUE schema alignment, i18n parity, architecture diagrams, container image policy, config defaults, security/audit docs, LLM authoring docs, and agent workflow docs against the actual codebase. Use when reviewing documentation accuracy, when code changes may have caused doc drift, after significant feature work, or before releases. Always use it for any documentation review task, even if the user doesn't say "review docs" — including checking docs, verifying documentation accuracy, or ensuring docs match code.
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

The review covers 11 surfaces. Each has a source of truth in the codebase; the exact file
scopes, sources and checks are in `references/surface-checklists.md` §S1-§S11.

| Surface | Scope | Key reference |
|---|---|---|
| S1: README.md | The live `README.md`, the primary external-facing doc | `readme-sync-map.md` |
| S2: Website Docs | MDX pages in `website/docs/` (the "Next" version at `/docs/next/`), from the live inventory | `doc-ownership.json`, `consolidated-sync-map.md` |
| S3: Snippet Data and CUE Schema Drift | Live inventory of `website/src/components/Snippet/data/*.ts` | `cue-drift-patterns.md` |
| S4: i18n Parity | pt-BR mirrors of English docs: structure, `<Snippet>` IDs, `<Diagram>` IDs | `verification-commands.md` §1, §5a |
| S5: Architecture Diagrams | D2 sources in `docs/diagrams/`, prose in `docs/architecture/`, pages in `website/docs/architecture/` | `consolidated-sync-map.md` (diagram section) |
| S6: Container Image Policy | Container examples across every documentation surface | `surface-checklists.md` §S6 |
| S7: Config Defaults vs Docs | Config defaults in docs, pt-BR mirrors and config snippets | `consolidated-sync-map.md` |
| S8: Homepage and Terminal Demo | `website/src/pages/index.tsx` terminal demo | `intentional-simplifications.md` |
| S9: Security Audit Docs | `website/docs/security/audit.mdx`, README security sections, security snippets | `surface-checklists.md` §S9 |
| S10: LLM-Assisted Agent Authoring Docs | `website/docs/advanced/llm-assisted-authoring.mdx`, README LLM authoring sections, related snippets | `surface-checklists.md` §S10 |
| S11: Agent Workflow Docs | `.agents/commands/review-docs.md`, `.agents/skills/docs/SKILL.md`, `AGENTS.md`, related skill references | `surface-checklists.md` §S11 |

Surface rules that subagents must keep in mind:

- **S1**: the drift-prone README sections are Invowkfile Format, Dependencies, Command
  Flags/Arguments, Module Dependencies, Configuration, and TUI Components.
- **S2**: review only `website/docs/`, never `website/versioned_docs/`. `doc-ownership.json` is
  the exact page → semantic check → source-of-truth contract.
- **S3**: missing `platforms` in implementation blocks is the #1 drift pattern (six patterns in
  total). Partial/fragment snippets that show individual CUE fields (e.g., just `runtimes:`)
  are intentionally incomplete and exempt; only flag snippets that show a full `cmds` entry
  with implementation blocks but lack required fields.
- **S4**: run `(cd website && npm run docs:parity)` for structure, and use the deterministic
  S4-C05 command in `references/verification-commands.md` to select the first three English
  pages for stale-prose review against their pt-BR files. Prose is translated; code examples
  are shared via `<Snippet>`.
- **S5**: check that diagram nodes, labels, and relationships match current package names and
  code structure. Delegate detailed D2 review (formatting, readability rules, rendering) to the
  `/d2-diagrams` skill; it is not an S5 finding.
- **S6**: every container example uses `debian:stable-slim`. No `ubuntu:*`, no Alpine, no
  Windows containers. Language-specific images (`golang:1.27`, `python:3-slim`,
  `node:22-slim`) are allowed only in language-specific runtime demonstrations.
- **S7**: `internal/config/config_schema.cue` is the source of truth for defaults, and
  `internal/config/types.go` `DefaultConfig()` must derive from it. Docs must match in
  `website/docs/reference/config-schema.mdx`, `website/docs/configuration/options.mdx`, and the
  pt-BR mirrors of both. Dual-prefix config snippets (`config/*` and `reference/config/*` in
  `Snippet/data/config.ts`) must both change when config options change.
- **S8**: the homepage terminal demo is intentionally simplified for marketing. Do NOT flag its
  simplifications; check `references/intentional-simplifications.md` before flagging anything
  in the homepage, Quick Start, or getting-started sections.
- **S9** sources of truth: `cmd/invowk/audit.go`, `cmd/invowk/llm_flags.go`, `internal/audit/`,
  `internal/auditllm/`, and `internal/app/llmconfig/` (the `invowk audit` command).
- **S10** sources of truth: `cmd/invowk/agent.go`, `internal/agentcmd/`, the shared LLM flags,
  and the LLM config resolver (`invowk agent cmd` and `invowk agent mod`).
- **S11**: agent-facing docs must stay in sync with this skill and the current repository
  surfaces whenever the review workflow changes.

---

## Consistency Principles

These keep repeated runs identical. They apply to the coordinator and every subagent.

1. **Checklist-driven review** — follow `references/surface-checklists.md`; every item gets PASS,
   FAIL, SKIP, or BLOCKED. Open-ended exploration may only produce uncounted candidate
   observations, never findings.
2. **Pre-assigned severity** — use the checklist severity, never your own judgment (subjective
   severity is the second-largest source of run-to-run variance, after scope sampling).
3. **Deterministic file traversal** — `references/doc-ownership.json` gives every live page
   exactly one non-mechanical semantic owner with resolvable sources; the tooling rejects
   missing, stale, duplicate, case-colliding, or policy-only ownership. Subagents use the sorted
   inventories in `context.json` and never recalculate membership.
4. **Structured context passing** — generate canonical `context.json` with
   `scripts/review_docs.py prepare` and pass its path and context ID to every subagent; never
   reconstruct context in prose.
5. **Complete reporting** — every checklist item must appear in canonical result JSON; any
   BLOCKED item makes the audit INCOMPLETE. Status meanings: `references/structured-output-format.md`
   § "Status Contract".
6. **Finding admission gate** — a FAIL becomes a finding only with every field the gate
   requires (`references/structured-output-format.md` § "Finding Admission Gate"). If the
   evidence cannot be gathered, report BLOCKED; never silently downgrade an incomplete finding.
7. **Inventory-first coverage** — run `scripts/review_docs.py validate` before spawning
   subagents. An unowned live page, a stale ownership entry, or a source glob that resolves to
   nothing is a contract failure and blocks the review.
8. **Stable audit snapshot** — use the content hash in `context.json` (tracked and untracked
   files, not HEAD alone). Validate it before accepting results and again before merge; if it
   changed, regenerate context and restart affected subagents.

---

## Orchestration Strategy

### Step 1: Run Programmatic Checks

Run automated checks first to catch mechanical issues. See `references/verification-commands.md`
for exact commands, the required checks JSON shape, and failure triage. Keep audit artifacts
outside the repository so they do not change the workspace snapshot (`prepare` rejects
in-repo outputs).

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

- [`references/surface-checklists.md`](references/surface-checklists.md) — per-surface checks with pre-assigned severity and finding type; derive totals with `scripts/review_docs.py validate`.
- [`references/doc-ownership.json`](references/doc-ownership.json) — exact semantic owner and resolvable sources for every current website page.
- [`references/consolidated-sync-map.md`](references/consolidated-sync-map.md) — canonical code → docs and code → diagram map, plus drift-prone areas.
- [`references/readme-sync-map.md`](references/readme-sync-map.md) — README section → source-of-truth map with heading locators.
- [`references/cue-drift-patterns.md`](references/cue-drift-patterns.md) — the 6 CUE snippet drift patterns with detection and correction.
- [`references/intentional-simplifications.md`](references/intentional-simplifications.md) — registry of intentional omissions (not findings).
- [`references/structured-output-format.md`](references/structured-output-format.md) — result JSON, status contract, admission gate, severity and types, merge contract.
- [`references/verification-commands.md`](references/verification-commands.md) — programmatic gates with expected output and failure triage.
- [`scripts/review_docs.py`](scripts/review_docs.py) — contract validation, context/snapshot preparation, i18n lag selection, strict result validation, deterministic merge.

---

## Related Skills

| Skill | When to Use |
|---|---|
| `/docs` | After review identifies issues — for editing documentation, syncing docs after code changes, creating pages, updating snippets |
| `/d2-diagrams` | For diagram creation, editing, rendering, and D2-specific readability review |
