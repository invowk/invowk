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
for f in $(find docs/diagrams -name '*.d2'); do
  d2 fmt --check "$f" 2>&1 || echo "FAIL: $f"
done
```

**What it checks**: D2 syntax validity (and formatter cleanliness) for all 23 diagram source
files. Note: `d2 fmt --check` is the supported syntax check in current D2 versions; older
docs referenced `d2 validate`, which is not available as a subcommand.

**Expected**: All files validate without errors and produce no `FAIL:` lines.

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

## 7b. TS-Template / CUE-String Interaction (Patterns 7 and 8)

These bugs are invisible at the TS layer — they only surface in the rendered CUE. Always run
these greps as part of S3 review:

```bash
# Pattern 7: nested unescaped " in script:/check_script: CUE strings
grep -nE '(script|check_script): "[^"]*"[^,"]+"[^"]*"' \
  website/src/components/Snippet/data/*.ts

# Pattern 8a: regex/charclass escapes that TS strips silently
grep -nE 'validation: "[^"]*\\[a-zA-Z]' website/src/components/Snippet/data/*.ts

# Pattern 8b: Windows env-var paths in CUE strings
grep -nE '"[A-Z_%]+\\[a-z]' website/src/components/Snippet/data/*.ts

# Pattern 8c: generic catch-all (excludes TS-recognized escapes and existing \\\\ pairs)
grep -nE '"[^"]*\\[a-zA-Z][^"]*"' website/src/components/Snippet/data/*.ts \
  | grep -vE 'language:|\\n|\\t|\\r|\\b|\\f|\\v|\\\\|\\"|\\$\{|\\u'
```

**Expected**: All four greps produce empty output, OR all hits are already wrapped in raw
strings `#"..."#` with `\\` doubling in TS source.

**Verification**: For any non-trivial fix, render the TS template literal in a small Node
script and feed the output to `cue eval` to confirm correctness. The TS layer cannot self-validate
these patterns — only `cue eval` on the rendered text exposes them.

See `cue-drift-patterns.md` Patterns 7 and 8 for full guidance.

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
