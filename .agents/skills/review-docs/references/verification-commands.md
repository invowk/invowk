# Verification Commands Reference

Run these automated checks BEFORE manual review to catch mechanical issues.
Record every result as PASS, FAIL, or BLOCKED. A missing command or unavailable environment is
BLOCKED, not a passing review. Any BLOCKED result makes the final audit INCOMPLETE.

## Contents

- [Deterministic Run Setup](#0-deterministic-run-setup)
- [Documentation Parity](#1-documentation-parity)
- [Website Build](#2-website-build)
- [Website Typecheck](#3-website-typecheck)
- [Version Asset Validation](#4-version-asset-validation)
- [Live Documentation Ownership](#5-live-documentation-ownership)
- [i18n Stale-Prose Candidates](#5a-deterministic-i18n-stale-prose-candidates)
- [Diagram Checks](#6-diagram-readability)
- [Agent Docs Integrity](#8-agent-docs-integrity)
- [Container Image Policy](#9-container-image-policy-check)
- [CUE and Config Snippets](#10-cue-snippet-schema-spot-check)
- [Context Artifact](#12-programmatic-results-and-context-artifact)
- [Execution Order](#execution-order)

## 0. Deterministic Run Setup

```bash
export LC_ALL=C
date +%F
git rev-parse HEAD
git status --short
```

HEAD and porcelain status are diagnostic only. `review_docs.py prepare` records a content hash of
all tracked and untracked repository files; `snapshot-verify` is the authoritative stability gate.

## 1. Documentation Parity

```bash
(cd website && npm run docs:parity)
```

**What it checks**: File parity between `docs/` and `i18n/pt-BR/.../current/`, plus
`<Snippet id="...">` and `<Diagram id="...">` reference parity between locales.

**Expected**: Exit 0, "All checks passed".

**Failure triage**: Lists missing files, mismatched snippet IDs, or missing diagram references.
Check `website/docs-parity-exceptions.json` for known exceptions. If the gap is intentional
(e.g., a page only exists in English), add an exception with justification.

## 2. Website Build

```bash
(cd website && npm run build)
```

**What it checks**: Full build of all locales. Catches broken links (`onBrokenLinks: 'throw'`),
missing MDX imports, syntax errors, and unresolved snippet/diagram references.

**Expected**: Exit 0, successful build for all locales (en + pt-BR).

**Failure triage**: Read the build error. Common causes: broken internal links, missing
`<Snippet>` import statement in MDX, unescaped `${...}` in snippet data, or new page not
added to `sidebars.ts`.

## 3. Website Typecheck

```bash
(cd website && npm run typecheck)
```

**What it checks**: TypeScript correctness for Docusaurus config, snippet aggregation, custom
components, and page code.

**Expected**: Exit 0.

**Failure triage**: Fix TypeScript errors in website source files. Common causes: stale snippet
imports, removed snippet data exports, or component prop drift.

## 4. Version Asset Validation

```bash
node scripts/validate-version-assets.mjs
```

**What it checks**: Current and versioned snippet/diagram asset registries are internally
consistent without manually reviewing frozen versioned docs.

**Expected**: Exit 0.

**Failure triage**: Regenerate or repair version asset snapshots with the repository's version
asset scripts. Do not hand-edit frozen versioned docs unless explicitly backporting.

## 5. Live Documentation Ownership

Validate exact page ownership, checklist IDs, source paths/globs, counts, and inventories before
spawning subagents:

```bash
.agents/skills/review-docs/scripts/review_docs.py validate
```

**What it checks**: Every live website page has exactly one literal ownership entry, one
non-mechanical semantic check, and at least one resolvable source of truth. It rejects missing or
stale pages, duplicate or case-colliding paths, unknown/cross-surface checks, policy-only owners,
excluded versioned docs, empty source lists, and source globs that match nothing. It also validates
contiguous checklist IDs and registered simplification IDs.

**Expected**: JSON summary with current derived counts and exit 0.

**Failure triage**: Add a semantic checklist check or repair
`references/doc-ownership.json`. Never satisfy coverage with a broad directory claim or a
mechanical navigation/policy check.

## 5a. Deterministic i18n Stale-Prose Candidate Set

Use this command for S4-C05:

```bash
.agents/skills/review-docs/scripts/review_docs.py i18n-candidates --days 60 --limit 3
```

The tool reads the latest commit epoch for each exact English/pt-BR pair with `git log --follow`.
It selects a page only when the English page is strictly more than 60 days newer than its locale
mirror, sorts by descending lag then path, and takes three. Missing mirrors remain structural
parity failures. Missing history, shallow-history gaps, or uncommitted locale-pair changes are
BLOCKED; the tool never substitutes file mtimes or today's date.

Only produce a finding when a selected page has an exact factual mismatch and satisfies the
Finding Admission Gate. General translation quality concerns remain uncounted candidates.

## 6. Diagram Readability

```bash
./scripts/check-diagram-readability.sh
```

**What it checks**: Five guardrails on all flowcharts in `docs/diagrams/flowcharts/`:
explicit `direction:` setting, `Start:` node exists, `Start` has `shape: oval`,
at least one `Start -> ...` edge, and explicit labels on every edge leaving a decision diamond.

**Expected**: Exit 0, all flowcharts pass.

**Failure triage**: Fix the D2 source file to meet the readability requirements. Use the
`/d2-diagrams` skill for guidance.

## 7. D2 Syntax Validation

```bash
while IFS= read -r f; do
  echo "=== $f ==="
  d2 validate "$f" 2>&1
done < <(LC_ALL=C find docs/diagrams -path '*/experiments/*' -prune -o -type f -name '*.d2' -print | LC_ALL=C sort)
```

**What it checks**: D2 syntax validity for every live diagram source file from the inventory.

**Expected**: All files validate without errors.

**Failure triage**: Fix D2 syntax errors. Common issues: unquoted labels with special characters,
missing closing braces, invalid `vars` blocks.

## 7a. Rendered Diagram Manifest Validation

```bash
make check-diagram-renders
```

**What it checks**: Committed rendered SVG manifests and source hashes are current, without
rerendering diagrams.

**Expected**: Exit 0, no stale, missing, or orphaned rendered diagrams.

**Failure triage**: Run `make render-diagrams`, review the D2/SVG changes, then rerun this check.

## 8. Agent Docs Integrity

```bash
make check-agent-docs
```

**What it checks**: AGENTS.md index entries match the filesystem (no stale or missing entries).

**Expected**: Exit 0, no drift detected.

**Failure triage**: Update AGENTS.md to match the current filesystem state.

## 9. Container Image Policy Check

```bash
# README
grep -n 'ubuntu:\|alpine:\|mcr.microsoft.com' README.md || echo "README: PASS"

# Website docs (current)
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' website/docs/ || echo "Website docs: PASS"

# Snippet data
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' website/src/components/Snippet/data/ || echo "Snippet data: PASS"

# Current i18n docs
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' website/i18n/pt-BR/docusaurus-plugin-content-docs/current/ || echo "Current i18n: PASS"

# Architecture prose
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' docs/architecture/ || echo "Architecture prose: PASS"
```

**What it checks**: No prohibited container images (Ubuntu, Alpine, Windows) in any live/current
documentation surface covered by the review. Frozen versioned snapshots are validated by version
asset checks and are not manually reviewed by this gate.

**Expected**: No matches (all PASS), or only language-specific images like `golang:1.26`,
`python:3-slim` in language-specific runtime demos.

**Failure triage**: Replace with `debian:stable-slim`. See `.agents/rules/version-pinning.md`
Container Images section.

## 10. CUE Snippet Schema Spot-Check

This is a manual review — no single command catches all drift patterns. The most effective
approach is:

```bash
# Find all CUE snippets with implementation blocks
grep -n 'implementations:' website/src/components/Snippet/data/*.ts

# Cross-check: each should have a nearby platforms: field
# (unless it's a partial/fragment snippet)
```

Then apply the full pattern checklist from `references/cue-drift-patterns.md`:
- `implementations:` blocks have `platforms:`
- `cmds` uses list syntax `[{...}]` not map syntax `{...}`
- `runtimes`/`platforms` are struct lists not string arrays
- Module requires use `git_url` not `git`
- Version constraints have no `v` prefix
- Module `includes` paths end with `.invowkmod`

## 11. Dual-Prefix Config Snippet Check

```bash
# List all config-related snippet IDs
grep -n "'config/" website/src/components/Snippet/data/config.ts
grep -n "'reference/config/" website/src/components/Snippet/data/config.ts
```

**What to check**: For each `config/X` snippet, verify a corresponding `reference/config/X`
exists with equivalent content (adapted for its page context). Common drift: one prefix
gets updated after a config change but the other is forgotten.

## 12. Programmatic Results and Context Artifact

Write the nine gate results to an external JSON file. Use `FAIL` when the command ran and found a
problem; use `BLOCKED` when the command or required environment was unavailable.

```json
{
  "docs-parity": {"status": "PASS", "detail": "All checks passed", "exit_code": 0},
  "container-policy": {"status": "PASS", "detail": "No prohibited images", "exit_code": 0},
  "diagram-readability": {"status": "PASS", "detail": "All checks passed", "exit_code": 0},
  "d2-validate": {"status": "PASS", "detail": "All sources valid", "exit_code": 0},
  "diagram-renders": {"status": "PASS", "detail": "Manifest current", "exit_code": 0},
  "check-agent-docs": {"status": "PASS", "detail": "Indexes current", "exit_code": 0},
  "version-assets": {"status": "PASS", "detail": "Snapshots valid", "exit_code": 0},
  "website-typecheck": {"status": "PASS", "detail": "Typecheck passed", "exit_code": 0},
  "website-build": {"status": "PASS", "detail": "Build passed", "exit_code": 0}
}
```

Generate canonical context outside the repository:

```bash
mkdir -p /tmp/review-docs/results
.agents/skills/review-docs/scripts/review_docs.py prepare \
  --checks /tmp/review-docs/checks.json \
  --output /tmp/review-docs/context.json
```

The command revalidates ownership, embeds sorted live inventories, computes the i18n candidate
set, records HEAD, and hashes tracked plus untracked repository content. Audit outputs inside the
repository are rejected so the tool cannot invalidate its own snapshot.

## Execution Order

Run checks in this order (fastest first, dependency-free checks in parallel):

0. Deterministic run setup + `review_docs.py validate`
1. `npm run docs:parity` + container image policy grep (parallel)
2. `check-diagram-readability.sh` + D2 validation loop + `make check-diagram-renders` (parallel)
3. `make check-agent-docs`
4. `node scripts/validate-version-assets.mjs`
5. `npm run typecheck`
6. `npm run build` (slower, run last — also catches issues from earlier steps)
7. CUE snippet spot-check + dual-prefix check (manual, during build)
8. Record checks JSON and run `review_docs.py prepare`
