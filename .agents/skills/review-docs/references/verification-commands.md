# Verification Commands Reference

Run these automated checks BEFORE manual review to catch mechanical issues.
Any failure should be investigated and recorded as a finding before proceeding.

## 1. Documentation Parity

```bash
cd website && npm run docs:parity
```

**What it checks**: File parity between `docs/` and `i18n/pt-BR/.../current/`, plus
`<Snippet id="...">` and `<Diagram id="...">` reference parity between locales.

**Expected**: Exit 0, "All checks passed".

**Failure triage**: Lists missing files, mismatched snippet IDs, or missing diagram references.
Check `website/docs-parity-exceptions.json` for known exceptions. If the gap is intentional
(e.g., a page only exists in English), add an exception with justification.

## 2. Website Build

```bash
cd website && npm run build
```

**What it checks**: Full build of all locales. Catches broken links (`onBrokenLinks: 'throw'`),
missing MDX imports, syntax errors, and unresolved snippet/diagram references.

**Expected**: Exit 0, successful build for all locales (en + pt-BR).

**Failure triage**: Read the build error. Common causes: broken internal links, missing
`<Snippet>` import statement in MDX, unescaped `${...}` in snippet data, or new page not
added to `sidebars.ts`.

## 3. Diagram Readability

```bash
./scripts/check-diagram-readability.sh
```

**What it checks**: Four guardrails on all flowcharts in `docs/diagrams/flowcharts/`:
explicit `direction:` setting, `Start:` node exists, `Start` has `shape: oval`, and
at least one `Start -> ...` edge.

**Expected**: Exit 0, all flowcharts pass.

**Failure triage**: Fix the D2 source file to meet the readability requirements. Use the
`/d2-diagrams` skill for guidance.

## 4. D2 Syntax Validation

```bash
for f in docs/diagrams/**/*.d2; do
  echo "=== $f ===" && d2 validate "$f" 2>&1
done
```

**What it checks**: D2 syntax validity for all 23 diagram source files.

**Expected**: All files validate without errors.

**Failure triage**: Fix D2 syntax errors. Common issues: unquoted labels with special characters,
missing closing braces, invalid `vars` blocks.

## 5. Agent Docs Integrity

```bash
make check-agent-docs
```

**What it checks**: AGENTS.md index entries match the filesystem (no stale or missing entries).

**Expected**: Exit 0, no drift detected.

**Failure triage**: Update AGENTS.md to match the current filesystem state.

## 6. Container Image Policy Check

```bash
# README
grep -n 'ubuntu:\|alpine:\|mcr.microsoft.com' README.md || echo "README: PASS"

# Website docs
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' website/docs/ || echo "Website docs: PASS"

# Snippet data
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' website/src/components/Snippet/data/ || echo "Snippet data: PASS"

# i18n docs
grep -rn 'ubuntu:\|alpine:\|mcr.microsoft.com' website/i18n/ || echo "i18n: PASS"
```

**What it checks**: No prohibited container images (Ubuntu, Alpine, Windows) in any documentation.

**Expected**: No matches (all PASS), or only language-specific images like `golang:1.26`,
`python:3-slim` in language-specific runtime demos.

**Failure triage**: Replace with `debian:stable-slim`. See `.agents/rules/version-pinning.md`
Container Images section.

## 7. CUE Snippet Schema Spot-Check

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

## 8. Dual-Prefix Config Snippet Check

```bash
# List all config-related snippet IDs
grep -n "'config/" website/src/components/Snippet/data/config.ts
grep -n "'reference/config/" website/src/components/Snippet/data/config.ts
```

**What to check**: For each `config/X` snippet, verify a corresponding `reference/config/X`
exists with equivalent content (adapted for its page context). Common drift: one prefix
gets updated after a config change but the other is forgotten.

## Execution Order

Run checks in this order (fastest first, dependency-free checks in parallel):

1. `npm run docs:parity` + container image policy grep (parallel)
2. `check-diagram-readability.sh` + D2 validation loop (parallel)
3. `make check-agent-docs`
4. `npm run build` (slower, run last — also catches issues from steps 1-3)
5. CUE snippet spot-check + dual-prefix check (manual, during build)
