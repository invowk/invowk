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

## Review Procedure

For each snippet data file (`website/src/components/Snippet/data/*.ts`):

1. **Identify CUE snippets**: Look for `language: 'cue'` entries that contain CUE syntax.
2. **Check each pattern**: Apply the 6 patterns above in order.
3. **Classify findings**: Full `cmds` entries with implementation blocks are subject to all
   patterns. Partial fragments showing single fields are exempt from Pattern 1.
4. **Cross-reference schema**: When in doubt, read the relevant `.cue` schema file and compare
   field names, types, and constraints literally.

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
