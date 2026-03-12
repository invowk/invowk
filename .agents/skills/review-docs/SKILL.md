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

Record any failures as findings before proceeding to manual review.

### Step 2: Spawn Parallel Subagents

Assign non-overlapping surface groups to up to 4 subagents. Pass Step 1 programmatic check
results to each subagent so they do not repeat automated checks.

| Subagent | Surfaces | References to Read | Focus |
|---|---|---|---|
| **SA-1: README** | S1 | `readme-sync-map.md`, `intentional-simplifications.md` | Read each README section vs its source of truth. Check CUE field names, CLI output, feature descriptions. |
| **SA-2: Website + Snippets + Config** | S2, S3, S7 | `consolidated-sync-map.md`, `cue-drift-patterns.md`, `intentional-simplifications.md` | Read website docs vs Go code and CUE schemas. Check snippet data for CUE drift. Verify config defaults match `DefaultConfig()`. Check dual-prefix config snippets. |
| **SA-3: Diagrams** | S5 | `consolidated-sync-map.md` (diagram section) | Read D2 sources and architecture docs vs current code structure. Verify nodes/labels match package names. Use `/d2-diagrams` skill for readability review. |
| **SA-4: i18n + Container Policy** | S4, S6 | `intentional-simplifications.md` | Structural parity via `docs:parity` results. Check for stale pt-BR prose using git log dates. Verify container image policy across all doc surfaces using grep results from Step 1. |

Each subagent should:
1. Read `references/intentional-simplifications.md` and their listed reference files
2. Follow the per-surface review procedure (below)
3. Produce findings in the structured format from `references/structured-output-format.md`

### Step 3: Merge and Report

The coordinator:
1. Collects findings from all subagents
2. Deduplicates by (file, line/snippet ID)
3. Cross-checks against `references/intentional-simplifications.md`
4. Sorts by severity (ERROR first)
5. Produces the final report (see `references/structured-output-format.md`)

---

## Review Methodology

### Per-Surface Procedure

For each documentation surface:

1. **Run programmatic checks** — fail-fast on mechanical errors
2. **Read source of truth** — the schema file, Go code, or CLI behavior that defines correctness
3. **Read the documentation** — the file being reviewed
4. **Compare** — does the doc accurately reflect the source of truth?
5. **Check intentional simplifications** — is the mismatch deliberate? (check the registry)
6. **Record finding** — use the structured format with appropriate severity

### What to Look For

**Factual accuracy** (→ ERROR):
- CUE field names that don't match the schema
- CLI flags or commands that have been renamed/removed
- Default values that have changed
- Feature descriptions that contradict current behavior
- Invalid CUE syntax in examples (e.g., missing required `platforms` in full implementation blocks)
- Prohibited container images

**Completeness** (→ WARNING):
- New features or config options not documented
- Schema fields added but not reflected in reference docs
- Runtime behavior changes not reflected in runtime-modes docs
- New TUI components or flags not in the TUI docs
- Missing i18n mirrors for recently added pages

**Style and clarity** (→ INFO):
- Outdated terminology
- Confusing phrasing that could mislead
- Missing admonitions for important caveats

**Intentional omissions** (→ SKIP):
- Simplified examples in getting-started/quickstart
- Homepage terminal demo
- One-feature-per-example pattern in core-concepts

Severity levels (ERROR/WARNING/INFO/SKIP) are defined in `references/structured-output-format.md`.
The "What to Look For" categories above map to these severities.

---

## Detailed References

Read these when working on the corresponding review surface:

- **[references/consolidated-sync-map.md](references/consolidated-sync-map.md)** — Superset
  code → docs mapping (website + diagrams + drift-prone areas)
- **[references/readme-sync-map.md](references/readme-sync-map.md)** — README section →
  source-of-truth mapping (~22 sections)
- **[references/cue-drift-patterns.md](references/cue-drift-patterns.md)** — 6 CUE snippet
  drift patterns with detection and correction guidance
- **[references/intentional-simplifications.md](references/intentional-simplifications.md)** —
  Registry of known intentional omissions (do NOT flag as errors)
- **[references/structured-output-format.md](references/structured-output-format.md)** —
  Finding report template, severity definitions, merge procedure
- **[references/verification-commands.md](references/verification-commands.md)** — Full
  command reference with expected output and failure triage

---

## Related Skills

| Skill | When to Use |
|---|---|
| `/docs` | After review identifies issues — for editing documentation, syncing docs after code changes, creating pages, updating snippets |
| `/d2-diagrams` | For diagram creation, editing, rendering, and D2-specific readability review |
