# CUE-Aware goplint Enum Exhaustiveness (`--check-enum-sync`)

> **STATUS: IMPLEMENTED** (2026-02-28). The mode is available as `--check-enum-sync` with
> the `//goplint:enum-cue=<CUEPath>` directive. See `tools/goplint/goplint/analyzer_enum_sync.go`.
> Remaining work: annotate production enum types with the directive.

## Summary

Add a new goplint analysis mode that reads CUE schema disjunctions and verifies
Go switch-case `Validate()` methods have matching enum members. This catches drift
that Go's `exhaustive` linter cannot detect (it only knows about Go-defined iota
enums, not CUE disjunctions).

## Problem

When a CUE schema adds a new enum member (e.g., `"serverless"` added to `#RuntimeType`),
the Go `Validate()` switch statement must be updated manually. No current tool catches
this drift — the schema sync tests verify field alignment, and the behavioral sync tests
(Option A) verify input agreement, but neither specifically checks that the *complete set
of valid values* is identical.

## Proposed Solution

### Directive-Based Opt-In

Types opt in via a new directive:

```go
//goplint:enum-cue=#RuntimeType
type RuntimeMode string
```

### New Mode: `--check-enum-sync`

1. Parse the `//goplint:enum-cue=<CUE path>` directive on type declarations
2. Load the CUE schema from the embedded `*_schema.cue` file in the same package
3. Extract disjunction members from the CUE path (e.g., `["native", "virtual", "container"]`)
4. Walk the type's `Validate()` method body and extract switch case string literals
5. Compare sets and report:
   - `enum-cue-missing-go`: CUE member not in Go switch
   - `enum-cue-extra-go`: Go switch case not in CUE disjunction

### Scope

~12-15 enum types across 3 packages:

| Go Type | Package | CUE Path |
|---------|---------|----------|
| `RuntimeMode` | `pkg/invowkfile` | `#RuntimeType` |
| `PlatformType` | `pkg/invowkfile` | `#PlatformType` |
| `EnvInheritMode` | `pkg/invowkfile` | `#RuntimeConfigBase.env_inherit_mode` |
| `FlagType` | `pkg/invowkfile` | `#Flag.type` |
| `ArgumentType` | `pkg/invowkfile` | `#Argument.type` |
| `CapabilityName` | `pkg/invowkfile` | `#CapabilityName` |
| `ContainerEngine` | `internal/config` | `#Config.container_engine` |
| `config.RuntimeMode` | `internal/config` | `#Config.default_runtime` |
| `ColorScheme` | `internal/config` | `#UIConfig.color_scheme` |
| `ValidationIssueType` | `pkg/invowkmod` | (no CUE counterpart — skip) |
| `Severity` | `internal/discovery` | (no CUE counterpart — skip) |

### Implementation Notes

- goplint is a separate Go module (`tools/goplint/`). Adding CUE support requires
  adding `cuelang.org/go` to `tools/goplint/go.mod`.
- CUE schema location: The tool needs to find the schema file. Two approaches:
  1. Convention: look for `*_schema.cue` files in the package under analysis
  2. Explicit: the directive includes the schema path (`//goplint:enum-cue=invowkfile_schema.cue#RuntimeType`)
  Convention-based is simpler but less flexible.
- CUE disjunction extraction: Use `cue.Value.Eval()` and check for `cue.DisjunctionKind`,
  then iterate disjuncts. The `cue.Value` API has methods for this.
- Switch case extraction: Reuse the pattern from `--check-validate-delegation` —
  walk the function body AST looking for `*ast.SwitchStmt` with `*ast.CaseClause`
  containing `*ast.BasicLit` of kind `token.STRING`.

### Dependencies

- `cuelang.org/go/cue` and `cuelang.org/go/cue/cuecontext` added to `tools/goplint/go.mod`

### Prerequisites

- Option A (behavioral sync tests) should be in place first to establish the pattern
  and prove that CUE constraint synchronization has value.

### Files to Create/Modify

- `tools/goplint/goplint/analyzer_enum_sync.go` — new file (~200 lines)
- `tools/goplint/goplint/analyzer.go` — new flag, category constant
- `tools/goplint/goplint/inspect.go` — new directive key `enum-cue`
- `tools/goplint/go.mod` — add CUE dependency
- `tools/goplint/CLAUDE.md` — document new mode
- Type declarations — add `//goplint:enum-cue=...` directives
- Test fixtures in `tools/goplint/goplint/testdata/src/enumsync/`

### Estimated Effort

~400-500 lines of new code + test fixtures. Medium complexity.
