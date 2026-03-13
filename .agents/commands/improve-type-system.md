# /improve-type-system

Four-phase workflow for converting bare primitives to DDD Value Types.
Implementation patterns live in the `invowk-typesystem` skill — this command focuses on analysis, prioritization, and workflow orchestration.

---

## Phase 1: Assess (Read-Only)

### 1.1 Verify clean baseline

```bash
make check-baseline
```

If this fails, fix regressions before proceeding. Do not start new type work on top of a broken baseline.

### 1.2 Generate findings report

```bash
make check-types-all-json 2>/dev/null
```

Parse the JSON output. Group findings by category and package. The most actionable categories for conversion work:

| Category | Signal | Typical resolution |
|----------|--------|-------------------|
| `primitive` | Bare primitive in struct field / param / return | Create wrapper type |
| `missing-validate` | Named type without `Validate() error` | Add Validate() |
| `missing-stringer` | Named type without `String() string` | Add String() |
| `missing-constructor` | Exported struct without constructor | Add NewXxx() |
| `unvalidated-cast` | DDD cast without Validate() on return path (CFA) | Wire Validate() call |
| `use-before-validate-same-block` | DDD variable used before Validate() | Reorder statements |
| `missing-constructor-error-return` | Constructor for validatable type missing `error` return | Add error return |
| `incomplete-validate-delegation` | Struct Validate() skips validatable field | Add field.Validate() call |
| `nonzero-value-field` | `//goplint:nonzero` type as non-pointer field | Use `*Type` or ensure construction guarantees |

### 1.3 Cross-reference coverage

- Read `type-catalog.md` → current typed counts (primitive-wrappers, composites, aliases)
- Read `baseline.toml` → accepted debt count and categories
- Check overdue review dates: `make check-types-all-json 2>/dev/null | jq '[.[] | select(.category == "overdue-review")]'`
- Read `exceptions.toml` header counts per reason category

### 1.4 Produce assessment summary

Present this to the user before proceeding:

```
| Metric                          | Value |
|---------------------------------|-------|
| Total Validate() types          | N     |
| Primitive-wrapper types         | N     |
| Composite validator types       | N     |
| Alias/re-export types           | N     |
| Baseline findings               | N     |
| Exception entries               | N     |
| Overdue review_after            | N     |
| Top finding categories:         |       |
|   primitive                     | N     |
|   unvalidated-cast              | N     |
|   missing-validate              | N     |
|   (next 2-3 highest)            | N     |
```

---

## Phase 2: Plan

### 2.1 Identify conversion units

A **conversion unit** is a single type creation plus its full ripple effect:
- Type definition + `Validate()` + `String()` + sentinel + error struct
- All callers that change from bare primitive to typed value
- All tests that need updating
- All `exceptions.toml` entries and `baseline.toml` findings resolved

Group related findings: if 8 findings across 3 packages all reference `commandName` as `string`, they resolve with one `CommandName` type. Use stable finding IDs from `goplint://finding/<id>` URLs to track these groupings.

### 2.2 Prioritize using the Impact Matrix

| Factor | Weight | How to assess |
|--------|--------|---------------|
| Findings resolved | 3x | Count all findings eliminated by this conversion |
| Domain clarity | 2x | Does typing prevent real bugs? (enum sets, validated formats, range constraints) |
| Caller simplicity | 1x | Net cast change: count added `.String()`/`string()` minus removed raw usages |
| Cross-package reach | 1x | Types used across 3+ packages score higher (consistency value) |
| Baseline shrinkage | 2x | Does this remove baseline entries? |

Compute a rough score: `(findings × 3) + (clarity × 2) + (net_cast × 1) + (reach × 1) + (baseline × 2)`.

### 2.3 Apply cost-benefit filter

**PROCEED** when:
- Net cast change is ≤ +2 (at most 2 new `.String()` / `string()` casts added)
- OR the type has validation semantics (regex pattern, port range, enum set, format constraint)
- OR the type crosses 3+ packages (consistency value outweighs cast overhead)

**DEFER** when:
- All callers pass directly to `filepath.Join` / `os.Stat` / `exec.Command` / `exec.LookPath` (pure OS/exec boundary)
- Net cast change is > +5
- Typing would require import cycle resolution without domain-clarity benefit

When deferring, add an exception to `exceptions.toml` with:
- `reason`: Use a category from the Exception Taxonomy below
- `review_after`: ISO date, 6-12 months out
- `blocked_by`: What must change before this can be typed (optional)

### 2.4 Select session scope

Pick **3-5 conversion units** ordered by priority score. Present to the user with:

```
| # | Type              | Package          | Findings resolved | Net casts | Concern |
|---|-------------------|------------------|-------------------|-----------|---------|
| 1 | FooName           | pkg/invowkfile   | 7                 | +1        | —       |
| 2 | BarToken          | internal/runtime | 4                 | 0         | —       |
| 3 | BazPath           | internal/config  | 3                 | +3        | filepath boundary |
```

Get user confirmation before proceeding.

---

## Phase 3: Execute

For each conversion unit, follow the `invowk-typesystem` skill:
1. Read `references/value-type-patterns.md` for the canonical pattern to apply
2. Check `references/type-catalog.md` for existing types to reuse or re-export
3. If a new type is needed, follow `references/maintenance-workflow.md`
4. Create type -> `Validate()` / `String()` -> sentinel -> error struct -> unit tests
5. Update callers, adding `.String()` casts only where unavoidable
6. Update or remove resolved exceptions from `exceptions.toml`
7. Run `make check-baseline` after each conversion to verify incremental progress

Between conversions, run `make lint` to catch cascading issues early.

### Directives to apply

- `//goplint:nonzero` — types where zero value is semantically invalid
- `//goplint:constant-only` — compile-time-constant-only types
- `//goplint:enum-cue=<path>` — enum types with CUE schema counterparts
- `//goplint:validate-all` — composite structs with validatable fields
- `//goplint:mutable` — structs that are intentionally mutable

### Import cycle resolution

If importing an existing type would create a circular dependency, move the type to `pkg/types/` (stdlib-only leaf package) and use the re-export alias pattern in the domain package.

### Typed path operations

Use `pkg/fspath/` wrappers (`JoinStr`, `Dir`, `Abs`, `Clean`, `FromSlash`, `IsAbs`) instead of manual `FilesystemPath(filepath.Join(string(path), ...))` casts.

---

## Phase 4: Verify

1. `make check-baseline` — must pass
2. `make update-baseline` — shrink if findings were resolved
3. `make test` — full suite (not `test-short`)
4. `make lint` — all linting passes
5. Refresh type catalog:
   ```bash
   .agents/skills/invowk-typesystem/scripts/extract_value_types.sh > \
     .agents/skills/invowk-typesystem/references/type-catalog.md
   ```
6. `make check-file-length` — all Go files under 1000 lines
7. Report session metrics to the user:

```
| Metric                 | Value |
|------------------------|-------|
| Types created          | N     |
| Findings resolved      | N     |
| Baseline entries removed | N   |
| Exceptions added       | N (with reasons) |
| Exceptions removed     | N     |
```

---

## Exception Taxonomy

When adding exceptions to `exceptions.toml`, use one of these canonical reason categories as the `reason` prefix:

| Category | When to use | Examples |
|----------|-------------|---------|
| `exec boundary` | Value passed to `exec.Command`, `exec.LookPath`, process spawn | Shell args, spawn params |
| `exec/OS boundary` | Value from/to `os.Stat`, `os.Environ`, `filepath.*` | Env maps, file paths at OS edge |
| `CLI boundary` | Value from Cobra `args[]`, flag binding, user input | Positional args, flag values |
| `display-only` | Value used only in `fmt.Sprintf`, error messages, TUI labels | Error detail, log text, display label |
| `parse boundary` | Raw input being validated/converted to typed value | `string` -> DDD conversion site |
| `import cycle` | Typing would create circular package dependency | Cross-server protocol types |
| `filepath boundary` | Value interacts heavily with `filepath.Join`, `os.Stat` | Directory paths with many Join sites |
| `service object` | Constructor-injected dependency, not a value type | Engines, configs, caches |
| `factory function` | Constructor returns always-valid value from pre-validated inputs | Result wrappers, internal helpers |
| `interface contract` | Signature dictated by external interface (bubbletea, list.Item) | `Height()`, `Spacing()`, `FilterValue()` |
| `framework boundary` | Value from framework API (Viper, CUE, huh) | Config decode, CUE parse output |
| `rendering internal` | ANSI/terminal raw text manipulation | Escape sequences, padding, color codes |

Exception fields:
- `pattern` (required): Dot-separated match pattern (supports `*` glob per segment)
- `reason` (required): Category prefix + specific context
- `review_after` (optional): ISO date for re-evaluation (6-12 months for cost-benefit deferrals)
- `blocked_by` (optional): Precondition that must change before this can be typed

---

## Relationship to Other Artifacts

| Artifact | Role | Boundary |
|----------|------|----------|
| `invowk-typesystem` SKILL | Implementation patterns (how to build types) | Patterns, contracts, anti-patterns |
| `/improve-type-system` (this) | Analysis + planning + workflow (what to convert next) | Prioritization, cost-benefit, orchestration |
| `/review-type-system` | Broad type safety review (abstractions, interfaces) | Strategic type architecture |
| `code-reviewer` agent | PR-time enforcement (conventions, contracts) | Review-time checks |
| `check-baseline` pre-commit hook | Regression gate | Prevents new findings |
| `make check-types-all-json` | Structured goplint output | Machine-readable findings |
