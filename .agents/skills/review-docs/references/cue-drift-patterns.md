# CUE Snippet Drift Patterns

CUE code snippets in `website/src/components/Snippet/data/*.ts` can drift from the actual
CUE schemas over time. This reference catalogs known drift patterns, ordered by frequency,
with detection and correction guidance.

Validate snippets against: `pkg/invowkfile/invowkfile_schema.cue`, `pkg/invowkmod/invowkmod_schema.cue`,
`internal/config/config_schema.cue`.

## Pattern 1: Missing `platforms` Field (Most Common)

**What**: Implementation blocks in CUE snippets omit the required `platforms` field.

**Why it matters**: The schema requires `platforms: [...#PlatformConfig] & [_, ...]` — at least
one platform. Snippets without `platforms` are technically invalid CUE.

**Correct values by runtime**:
- Container runtime: `platforms: [{name: "linux"}]`
- Virtual runtime: `platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]`
- Native runtime: `platforms: [{name: "linux"}, {name: "macos"}]` (add `{name: "windows"}` if cross-platform)

**Exception**: Partial/fragment snippets that show individual CUE fields (e.g., just `runtimes:` config
or just `depends_on:`) are intentionally incomplete and do NOT need `platforms`. These snippets
focus on explaining one specific feature. Judge by context: if the snippet shows a full `cmds`
entry with an implementation block, it should have `platforms`.

**Detection**: Search for `implementations:` in snippet data files and verify each has a sibling
`platforms:` field (or is clearly a partial fragment).

**History**: A bulk fix applied 35 corrections in `dependencies.ts`, 10 in `core-concepts.ts`,
5 in `advanced.ts`. New snippets can reintroduce this drift.

## Pattern 2: `cmds` as Map Instead of List

**What**: Snippets show `cmds: { "name": { ... } }` (map syntax) instead of the correct
list syntax `cmds: [{ name: "...", ... }]`.

**Correct**: `cmds` is a list of structs, each with a `name` field.

## Pattern 3: `runtimes`/`platforms` as String Arrays

**What**: Snippets show `runtimes: ["virtual"]` or `platforms: ["linux", "macos"]` instead
of struct lists.

**Correct**: `runtimes: [{name: "virtual"}]` and `platforms: [{name: "linux"}, {name: "macos"}]`.

## Pattern 4: `git` Instead of `git_url` in Module Requires

**What**: Module dependency snippets use `git:` field.

**Correct**: The field is `git_url:` in the schema.

## Pattern 5: `v` Prefix in Version Constraints

**What**: Version constraints shown as `version: "v1.0.0"` or `min_version: "v0.2.0"`.

**Correct**: CUE semver constraints do NOT use the `v` prefix: `version: "1.0.0"`, `min_version: "0.2.0"`.

## Pattern 6: Missing `.invowkmod` Suffix in Includes

**What**: Config `includes` paths shown without the `.invowkmod` suffix.

**Correct**: Include paths must end with `.invowkmod` to match the module directory convention.

## Pattern 7: Nested Double Quotes in CUE String Fields

**What**: A CUE string field uses `"..."` to delimit the value but contains additional unescaped
`"` characters inside, terminating the literal early.

**Examples found in the wild** (literal source from snippet data files):
```
script: "echo "Deploying to $PLATFORM""
check_script: "test "$ENV" = 'production'"
check_script: "python3 -c 'import yaml; yaml.safe_load(open("config.yaml"))'"
```

**Why it slips through code review**: TypeScript backtick template literals accept the source
text verbatim — TS doesn't enforce CUE string semantics. The rendered CUE is `"echo "` (a closed
string) followed by `Deploying...` (a bare identifier), which fails `cue vet`/`cue eval`.

**Fix**: Use CUE triple-quoted strings (preferred — matches the existing convention for multi-line
scripts in the same files) or escape with `\"`:

```cue
script: """
    echo "Deploying to $PLATFORM"
    """
```

**Detection grep**:
```bash
grep -nE '(script|check_script): "[^"]*"[^,"]+"[^"]*"' \
  website/src/components/Snippet/data/*.ts
```
Empty output means no occurrences. Any hit is almost certainly a bug.

**Verification**: Render the snippet text (e.g., copy the relevant template-literal contents into
a `.cue` file) and run `cue eval`. The fix is correct iff the new `cue eval` output matches the
shell semantics the docs intend (e.g., `echo "Hello, world"` after parsing).

## Pattern 8: TS Template Literal Strips Single Backslashes

**What**: A CUE string field contains `\X` where `X` is a non-special character (a letter or
period). TypeScript backtick template literals silently drop such `\` (per JS spec: unrecognized
escape sequences degrade to the character without the backslash). The rendered CUE no longer
contains the backslash, breaking regex semantics or Windows path display.

**Examples found in the wild**:
- Regex literal dot: `validation: "^[0-9]+\.[0-9]+\.[0-9]+$"` → renders as
  `"^[0-9]+.[0-9]+.[0-9]+$"` (any-char now passes — quietly accepts `1a0a0`).
- Regex character class: `validation: "^https?://[^\s]+$"` → renders as `"^https?://[^s]+$"`
  (whitespace class becomes literal letter `s`).
- Windows env var path: `CONFIG_PATH: "%APPDATA%\myapp\config.yaml"` → renders as
  `"%APPDATA%myappconfig.yaml"` (path separators silently collapsed).
- Pedagogical "bad example" path: `workdir: ".\src\app"` → renders as `".srcapp"` (the
  Windows-style slash being warned against is invisible in the rendered output).
- The special case `\b`: TS recognizes `\b` as backspace (``), so `"scripts\build.sh"`
  renders with an embedded control character — even worse than dropping the backslash.

**Why it slips through code review**: Same root cause as Pattern 7 — TS template literals don't
validate the embedded language. CUE-side validation never runs at the TS layer, and rendered
regex strings still match *something* (just the wrong thing), so tests rarely fail loudly.

**Fix**: Use a CUE raw string `#"..."#` (no escape processing) and double the backslashes in TS
source so the rendered text retains a literal `\`:

```ts
// TS source:
validation: #"^[0-9]+\\.[0-9]+\\.[0-9]+$"#
//                   ^^^^             ^^^^   <- two backslashes per literal-dot
// Rendered to user (one backslash per pair):
validation: #"^[0-9]+\.[0-9]+\.[0-9]+$"#
// CUE raw string value (no escape processing): ^[0-9]+\.[0-9]+\.[0-9]+$
// Go regex compiles \. as literal "." — correct semver match.
```

For Windows paths the same shape applies:
```ts
CONFIG_PATH: #"%APPDATA%\\myapp\\config.yaml"#  // TS source
// Rendered:  #"%APPDATA%\myapp\config.yaml"#
// CUE raw string value: %APPDATA%\myapp\config.yaml
```

For PowerShell or bash blocks (non-CUE language) that include Windows paths, use TS escape only:
```ts
notepad "$env:APPDATA\\invowk\\config.cue"  // renders as: notepad "$env:APPDATA\invowk\config.cue"
```

**Detection greps**:
```bash
# Regex / charclass / file extension patterns:
grep -nE 'validation: "[^"]*\\[a-zA-Z]' website/src/components/Snippet/data/*.ts

# Windows-style env-var path in CUE strings (covers %APPDATA%\X, %LOCALAPPDATA%\X, etc.):
grep -nE '"[A-Z_%]+\\[a-z]' website/src/components/Snippet/data/*.ts

# Generic catch-all for stray single backslashes inside double-quoted strings,
# excluding the TS-recognized escapes:
grep -nE '"[^"]*\\[a-zA-Z][^"]*"' website/src/components/Snippet/data/*.ts \
  | grep -vE 'language:|\\n|\\t|\\r|\\b|\\f|\\v|\\\\|\\"|\\$\{|\\u'
```
Any non-empty result is suspect. Hits already wrapped in `#"..."#` (raw string) with `\\` in
source are correct; everything else needs converting.

**Verification**:
1. Write a small Node script that constructs the same template literal and prints its value.
2. Place the rendered text into a `.cue` file and run `cue eval`. The string value should
   contain literal backslashes where the spec requires them.

**History**: A bulk fix corrected 4 sites in `flags-args.ts`, 5 in `dependencies.ts`, 1 in
`environment.ts`, 2 in `advanced.ts`, 2 in `modules.ts`, and 1 in `config.ts` (PowerShell
example, same TS-stripping issue even though the language was bash, not CUE).

## Review Procedure

For each snippet data file (`website/src/components/Snippet/data/*.ts`):

1. **Identify CUE snippets**: Look for `language: 'cue'` entries that contain CUE syntax.
2. **Check each pattern**: Apply the 8 patterns above in order.
3. **Classify findings**: Full `cmds` entries with implementation blocks are subject to all
   patterns. Partial fragments showing single fields are exempt from Pattern 1.
4. **Cross-reference schema**: When in doubt, read the relevant `.cue` schema file and compare
   field names, types, and constraints literally.
5. **Render-and-validate for Patterns 7 and 8**: When a snippet might exhibit either pattern,
   reproduce the rendered text via a Node template literal and run `cue eval` (or `cue vet`)
   on the result. The TS layer cannot detect these issues — only the rendered output can.

## Files to Check

- `website/src/components/Snippet/data/core-concepts.ts` — invowkfile format examples
- `website/src/components/Snippet/data/dependencies.ts` — dependency config examples
- `website/src/components/Snippet/data/advanced.ts` — interpreter, workdir, platform examples
- `website/src/components/Snippet/data/modules.ts` — module creation, requires examples
- `website/src/components/Snippet/data/runtime-modes.ts` — runtime config examples
- `website/src/components/Snippet/data/config.ts` — config file examples (dual-prefix: `config/*` and `reference/config/*`)
- `website/src/components/Snippet/data/getting-started.ts` — quickstart CUE examples
- `website/src/components/Snippet/data/flags-args.ts` — flag/argument CUE examples
- `website/src/components/Snippet/data/environment.ts` — env config examples
- `website/src/components/Snippet/data/tui.ts` — TUI component examples
- `website/src/components/Snippet/data/cli.ts` — CLI usage examples
