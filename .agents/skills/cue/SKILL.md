---
name: cue
description: CUE schema patterns for *.cue files, schema sync and behavioral sync tests, Go struct JSON tag alignment, parser/decode helpers, error formatting, and cuelang.org/go upgrades. Use when editing invowkfile_schema.cue, invowkmod_schema.cue, config_schema.cue, pkg/cueutil, CUE-backed Go types, or CUE parsing/validation behavior.
---

# CUE Schema Patterns

Always read `.agents/rules/cue-patterns.md` first; repository rules override this skill. Schema file locations are listed in `AGENTS.md` "Where things live".

## Rules

- All CUE structs MUST be closed with `close({...})`; open structs silently accept unknown fields (pitfall row in `.agents/rules/cue-patterns.md`).
- When adding CUE struct fields or definitions, always include validation constraints (`strings.MaxRunes()`, `=~` regex patterns, ranges such as `>=0 & <=255`), not just type declarations.
- Schema sync tests MUST exist for every Go struct that corresponds to a CUE definition. Add or update them when you:
  - add a CUE definition with a corresponding Go struct;
  - add fields to existing CUE/Go types;
  - rename fields (the test fails until both sides are updated).

## Schema Compilation Pattern

For new parsing paths, prefer `pkg/cueutil` helpers over reimplementing the CUE boilerplate: `cueutil.CompileSchema` + `cueutil.ParseAndDecodeCompiled` (with `sync.OnceValues` for embedded schemas parsed repeatedly), `cueutil.ParseAndDecode` / `ParseAndDecodeString` for one-off parses, `cueutil.WithFilename`, and `cueutil.FormatError` (which produces the repo-standard file/path-prefixed errors). Direct CUE parsing is reserved for cases that need custom lookup/merge behavior.

The helper-backed flow is still conceptually the 3-step parse flow:

```go
// Step 1: Compile the schema (embedded via //go:embed)
ctx := cuecontext.New()
schemaValue := ctx.CompileString(embeddedSchema)
if schemaValue.Err() != nil {
    return nil, fmt.Errorf("internal error: failed to compile schema: %w", schemaValue.Err())
}

// Step 2: Compile user data, unify with the root definition, validate concretely
userValue := ctx.CompileBytes(data, cue.Filename(path))
if userValue.Err() != nil {
    return nil, cueutil.FormatError(userValue.Err(), path)
}

schema := schemaValue.LookupPath(cue.ParsePath("#DefinitionName"))
unified := schema.Unify(userValue)
if err := unified.Validate(cue.Concrete(true)); err != nil {
    return nil, cueutil.FormatError(err, path)
}

// Step 3: Decode to Go struct
var result GoStructType
if err := unified.Decode(&result); err != nil {
    return nil, cueutil.FormatError(err, path)
}
```

**Key Points**:
- Schema is embedded via `//go:embed` for single-binary distribution.
- Use `LookupPath()` to get the root definition (e.g., `#Invowkfile`, `#Config`).
- Use `Validate(cue.Concrete(true))` so all values are concrete (`cueutil` defaults to concrete validation).
- Config parsing is also concrete; defaults and optional fields belong in the CUE schema/default layer before decode.
- Always use `Decode()` for type-safe extraction (see Decode Usage Rules).

**Reference Implementations**:
- `pkg/invowkfile/parse.go:ParseBytes()` - Invowkfile parsing
- `pkg/invowkmod/parse.go:ParseInvowkmodBytes()` - Invowkmod parsing
- `internal/config/config.go:decodeCUEConfigSource()` - Config loading

## Validation Responsibility Matrix

Validation is split between CUE and Go based on what each can handle. CUE is the
schema source of truth, but Go may deliberately mirror constraints for direct
construction, defense-in-depth, richer runtime checks, and drift-detection tests.

### CUE Handles (Declarative, Schema-Level)

| Validation Type | CUE Construct | Example |
|-----------------|---------------|---------|
| Type checking | Native | `name: string` |
| Field format (regex) | `=~` | `name: =~"^[a-zA-Z][a-zA-Z0-9_-]*$"` |
| Enum values | Disjunction | `"native" \| "virtual-sh" \| "virtual-lua" \| "container"` |
| Length limits | `strings.MaxRunes()` | `& strings.MaxRunes(256)` |
| Range constraints | Expressions | `>=0 & <=65535` |
| Required fields | No `?` suffix | `name: string` (required) |
| Optional fields | `?` suffix | `description?: string` (optional) |
| Closed structs | `close({})` | Rejects unknown fields |
| Mutual exclusivity | XOR constraints | See runtime config patterns |
| Non-empty lists | Pattern | `[_, ...]` (at least one) |

### Go Handles (Dynamic, Runtime-Level)

| Validation Type | Why Go-Only | Example |
|-----------------|-------------|---------|
| ReDoS prevention | CUE cannot analyze regex complexity | `ValidateRegexPattern()` (`pkg/invowkfile/validation_primitives.go`) |
| File existence | Requires filesystem access | `os.Stat()` |
| Path traversal | Cross-platform normalization | `filepath.Clean()` checks, `ValidateEnvFilePath()` (`pkg/invowkfile/validation_filesystem.go`) |
| Cross-field logic | Conditional requirements | Runtime-specific fields |
| Command hierarchy | Requires tree analysis | Leaf-only args constraint |
| Length limits (defense-in-depth) | Some limits enforced in Go too | `MaxNameLength` checks |

### Justification Comments

All Go-only validations MUST include a `[GO-ONLY]` comment explaining why CUE cannot handle them. The repo places these comments both on the Go validator and on the corresponding CUE field or definition (e.g. `#InterpreterSpec` in `invowkfile_schema.cue`, `#LLMTimeoutDurationString` in `config_schema.cue`).

```go
// ValidateRegexPattern validates a user-provided regex pattern for safety.
// [GO-ONLY] ReDoS (Regular Expression Denial of Service) prevention MUST be in Go.
// CUE cannot analyze regex complexity or detect catastrophic backtracking patterns.
func ValidateRegexPattern(pattern string) error { ... }
```

```go
// ValidateEnvFilePath validates an env file path for security.
// [GO-ONLY] Path traversal prevention and cross-platform path handling require Go.
// CUE cannot perform filesystem operations or cross-platform path normalization.
func ValidateEnvFilePath(filePath string) error { ... }
```

## Decode Usage Rules

**ALWAYS use `value.Decode(&goStruct)` for type-safe extraction.**

```go
var invowkfile Invowkfile
if err := unified.Decode(&invowkfile); err != nil {
    return nil, cueutil.FormatError(err, path)
}
```

Manual extraction (`String()`, `Int64()`, `Bool()`) loses compile-time type checking, needs per-field error handling, and makes optional fields easy to mishandle.

**Exception**: Manual extraction is acceptable only for truly dynamic scenarios (e.g., extracting arbitrary user-defined keys). Document such cases.

## Field Naming Convention

CUE uses `snake_case`, Go uses `PascalCase`. The JSON tag bridges them; config structs also carry a matching `mapstructure` tag for Viper:

```cue
// CUE Schema (snake_case) - internal/config/config_schema.cue
#Config: close({
    container_engine: *"podman" | #ContainerEngineType
    includes:         *([]) | [...#IncludeEntry]
    default_runtime:  *"native" | #ConfigRuntimeType
})
```

```go
// Go Struct (PascalCase with JSON tags) - internal/config/types.go
type Config struct {
    ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
    Includes        []IncludeEntry  `json:"includes" mapstructure:"includes"`
    DefaultRuntime  RuntimeMode     `json:"default_runtime" mapstructure:"default_runtime"`
}
```

**Rule**: Every non-bottom CUE field name that decodes into Go must have a matching
JSON tag in the corresponding Go struct. Bottom-field tombstones (`_|_`) are
intentionally skipped by sync helpers.

## Schema Sync Tests

Structural sync tests compare Go struct JSON tags with CUE field names using the `internal/testutil/schematest` helpers (`LookupDefinition`, `ExtractCUEFields`, `ExtractGoJSONTags`, `AssertFieldsSync`); behavioral sync tests compare Go `Validate()` and CUE verdicts on the same inputs. invowkfile types go into the table-driven `TestSchemaSync` case list in `pkg/invowkfile/sync_test.go`. All of them run in `make test`.

The complete test-file inventory, the commands, the real failure messages and the add-a-field/add-a-type procedure live in `.agents/skills/schema-sync-check/SKILL.md`.

## Error Formatting Requirements

All CUE errors MUST include JSON path prefixes:

```
<file-path>: <json-path>: <message>
```

Examples:
```
invowkfile.cue: cmds[0].implementations[2].script: value exceeds maximum length
config.cue: container.auto_provision.enabled: expected bool, got string
```

Use `cueutil.FormatError(err, filePath)` (`pkg/cueutil/error.go`) instead of reimplementing CUE error walking; prefer the `cueutil.ParseAndDecode*` helpers when they fit. `pkg/cueutil/error.go` is the only non-test file that imports `cuelang.org/go/cue/errors`. If low-level code must walk CUE errors, import `cuelang.org/go/cue/errors`, not the standard library `errors` (which has no `Path()`), and keep it in that low-level utility.

## CUE Library Version Pinning

Read the current CUE version from `go.mod` or `go list -m cuelang.org/go`; do not
hard-code the version in this skill.

### Upgrade Process

When upgrading the CUE library version:

1. **Review Changelog**: Check for breaking changes in the CUE release notes (for CUE `v0.18.0+`, `explicitopen` is stable and enabled by default, requiring `#Base...` when embedding shared definitions inside derived or `close({...})` structs)
2. **Run Full Test Suite**: `make test` including all schema sync tests
3. **Check API Deprecations**: Search for deprecated function usage
4. **Verify Error Formats**: Manually test that error messages still include paths
5. **Update Documentation**: If CUE behavior changes, update this skill and `.agents/rules/cue-patterns.md`
6. **Test Cross-Platform**: Run CI on all platforms (Linux, macOS, Windows).
   Windows-specific gotcha: CUE v0.17+ rejects carriage returns in multiline-string
   indentation ("non-matching whitespace for multiline strings"), so `*.cue` files
   must never be checked out with CRLF — `.gitattributes` forces `*.cue text eol=lf`.
   Keep that rule when touching `.gitattributes`.

### Known CUE Limitations

- **Encode is value conversion, not source generation**: `cue.Context.Encode`
  converts a Go value into a `cue.Value` for validation, unification, and other
  CUE operations. Do not describe it as a CUE source-code generator.
- **No Code Generation**: `gengotypes` is experimental; we use sync tests instead
- **Context lifecycle and cross-context unification**: A `cue.Context` retains compiled instances (`AddInst`), so never compile unbounded user inputs into a long-lived context. Instead, compile static embedded schemas once via `cueutil.CompileSchema` and compile each user input in a fresh per-parse `cuecontext.New()`, unifying with `schemaRoot.Unify(userValue)` (receiver first so stdlib builtins like `"strings"` stay cached on the schema runtime while the user context is GC'd; CUE supports concurrent cross-context `Unify` on finalized values).

## Common Pitfalls

### Unclosed CUE Structs

See the "Missing `close()` on struct" row in `.agents/rules/cue-patterns.md` and the first item under Rules.

### Redundant Validation

**Problem**: Accidental duplication of schema validation in Go creates
maintenance burden and drift.

**Rule**: CUE remains the schema source of truth for parsed data. Mirror a CUE
constraint in Go only when direct programmatic construction, defense-in-depth,
or a richer runtime contract requires it; document the reason and keep a
behavioral sync test. Go also owns filesystem, security, and runtime-dependent
validation that CUE cannot perform.

**Anti-Pattern**:
```go
// WRONG: Duplicates CUE regex validation
if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`).MatchString(name) {
    return fmt.Errorf("invalid name format")
}
```

**Correct Pattern**:
```go
// CORRECT: CUE handles format; Go handles length (defense-in-depth)
// [GO-ONLY] Length limit not in CUE for simplicity; checked here for defense-in-depth.
if len(name) > MaxNameLength {
    return fmt.Errorf("name too long (%d chars, max %d)", len(name), MaxNameLength)
}
```

### Missing JSON Tags

**Symptom**: A Go field without a JSON tag is never populated by `Decode()`; it stays at its zero value despite being in the CUE file.

**Fix**: Add a JSON tag matching the CUE field name:
```go
type Config struct {
    Includes  []IncludeEntry `json:"includes"`  // Matches CUE field
    CachePath string         // No JSON tag - will be empty!
}
```

`ExtractGoJSONTags` skips untagged Go fields, so sync tests catch this only when the CUE field exists: they report `CUE field "..." not found in Go struct (missing JSON tag)`.

### Stale Schema Sync Tests

After any CUE/Go struct change, run `make test` and keep the sync tests current:

- **Structural**: keep the `TestSchemaSync` case table (and the per-package sync tests) in step with the definitions. Go-only fields carry `json:"-"` (skipped by `ExtractGoJSONTags`); CUE tombstones are `_|_` (skipped by `ExtractCUEFields`). There is no other exclusion list.
- **Behavioral**: an intended Go/CUE disagreement is recorded as a non-empty `divergeNote` on a `behavioralSyncCase` (`pkg/invowkfile`, `pkg/invowkmod`, `internal/config` sync tests) and should pair with a `[GO-ONLY]` comment. A case with a `divergeNote` only logs the disagreement (`Expected divergence: ...`) instead of failing on drift. Re-check every non-empty `divergeNote` touched by the change; when the two sides agree again, update `goExpect`/`cueExpect` and remove the note.
