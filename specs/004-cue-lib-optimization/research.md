# Research: CUE Library Usage Optimization

**Date**: 2026-01-30 | **Status**: Complete

## Executive Summary

After researching CUE's Go integration options, the recommended approach is **NOT** to generate Go structs from CUE schemas. Instead:

1. **Maintain Go structs as source of truth** with proper JSON tags
2. **Use CUE for validation** via `value.Decode()` + `Validate()` (already in place)
3. **Add schema sync tests** to detect tag/field mismatches at CI time
4. **Consolidate validation** in CUE schemas where possible

This is simpler, more maintainable, and aligns with how production CUE users work.

---

## Research Topics

### 1. CUE Code Generation: Go Struct Generation

**Decision**: Do NOT generate Go structs from CUE schemas

**Rationale**:
- CUE has no production-ready Go struct generator
- `gengotypes` is experimental and may produce incompatible types
- Third-party tools (hof, cuetsy) are either TypeScript-focused or heavyweight
- CUE's design philosophy is validation, not code generation

**Alternatives Considered**:

| Approach | Status | Why Rejected |
|----------|--------|--------------|
| `gengotypes` | Experimental | Unstable API, may produce breaking changes |
| Custom generator | High effort | Significant development/maintenance burden |
| Third-party (hof) | Overkill | Full framework when we just need structs |
| Manual sync | Current state | Works but lacks automation |

**Recommendation**: Add schema sync tests instead of generating structs.

---

### 2. Tagged Union/Discriminated Union Patterns

**Decision**: Current approach is acceptable; no `_type` field needed

**Rationale**:
- CUE automatically recognizes discriminator fields (like `runtimes[*].name`)
- The existing `RuntimeConfig.Name` field acts as the discriminator
- Adding explicit `_type` fields would be redundant

**Current Pattern in Invkfile**:
```cue
#RuntimeConfig: close({
    name: #RuntimeType  // Discriminator: "native" | "virtual" | "container"
    // Fields vary by name value
})
```

**Go Implementation**:
```go
type RuntimeConfig struct {
    Name RuntimeMode `json:"name"`  // Acts as discriminator
    // Container-only fields
    Containerfile string   `json:"containerfile,omitempty"`
    Image         string   `json:"image,omitempty"`
    // ... other fields
}
```

**Recommendation**: Keep current pattern. Add post-decode validation to ensure field presence matches runtime type.

---

### 3. CUE Decode() Best Practices

**Decision**: Continue using `value.Decode()` for type-safe extraction

**Rationale**:
- Current implementation already uses `Decode()` correctly
- Manual extraction (`String()`, `Int64()`) only needed for dynamic scenarios
- Adding `cuego` runtime validation is overkill for this project size

**Current Pattern (Correct)**:
```go
// pkg/invkfile/parse.go
unified := schema.Unify(userValue)
if err := unified.Validate(cue.Concrete(true)); err != nil {
    return nil, formatCUEError(err)
}
var invkfile Invkfile
if err := unified.Decode(&invkfile); err != nil {
    return nil, formatCUEError(err)
}
```

**Recommendation**: Document this pattern in `.claude/rules/cue.md` as canonical.

---

### 4. Redundant Validation Analysis

**Decision**: Audit existing Go validation and consolidate into CUE where possible

**Findings from Codebase Exploration**:

| Validation | Location | Should Be |
|------------|----------|-----------|
| Interpreter non-empty | CUE + Go | CUE only (already handled by regex) |
| Tool name format | CUE | Keep in CUE |
| Regex safety (ReDoS) | Go only | Keep in Go (CUE can't do this) |
| Path traversal (`..`) | CUE | Keep in CUE |
| Runtime-specific fields | Go only | Keep in Go (cross-field logic) |
| Script file existence | Go only | Keep in Go (filesystem access) |
| Integer/float parsing | Go only | Keep in Go (runtime type coercion) |

**Clear Responsibilities**:
- **CUE**: Format validation (regex patterns, length limits, enum values)
- **Go**: Security validation (ReDoS), filesystem access, runtime logic

**Recommendation**: Remove interpreter whitespace check from Go (already in CUE schema).

---

### 5. Schema Sync Testing

**Decision**: Add reflection-based sync tests

**Rationale**:
- Tests verify Go struct tags match CUE field names at compile/test time
- Catches mismatches before they cause silent parsing failures
- No code generation required

**Implementation Pattern**:

```go
func TestInvkfileFieldSync(t *testing.T) {
    ctx := cuecontext.New()
    schema := ctx.CompileString(invkfileSchema)
    invkfileDef := schema.LookupPath(cue.ParsePath("#Invkfile"))

    // Get CUE field names
    cueFields := extractCUEFields(invkfileDef)

    // Get Go struct JSON tags
    goFields := extractGoJSONTags(reflect.TypeOf(Invkfile{}))

    // Compare
    for _, field := range cueFields {
        if !goFields[field] {
            t.Errorf("CUE field %q not found in Go struct tags", field)
        }
    }
}
```

**Recommendation**: Implement sync tests for all three schemas (invkfile, invkmod, config).

---

### 6. CUE Library Version Management

**Decision**: Pin version + document upgrade process

**Current State**:
- `go.mod` pins `cuelang.org/go` to specific version
- No documented upgrade process

**Upgrade Process** (to document):
1. Review CUE changelog for breaking changes
2. Run full test suite including schema sync tests
3. Check for API deprecations
4. Update documentation if behavior changes

**Recommendation**: Add upgrade process to `.claude/rules/cue.md`.

---

## Revised Implementation Approach

Based on research findings, the implementation plan should:

1. **Drop code generation** - Too complex, not production-ready
2. **Add sync tests** - Verify struct tags match CUE fields
3. **Consolidate validation** - Remove redundant Go checks
4. **Expand documentation** - Full `.claude/rules/cue.md` rewrite
5. **Improve error messages** - Ensure CUE errors include full paths

---

## File Size Limit Implementation

**Decision**: Add early check before CUE parsing

**Rationale**:
- Spec requires rejecting files >5MB (configurable) before processing
- Prevents OOM on malicious large files
- Simple to implement at parse entry point

**Implementation**:
```go
const DefaultMaxCUEFileSize = 5 * 1024 * 1024 // 5MB

func ParseBytes(data []byte, path string) (*Invkfile, error) {
    if len(data) > maxCUEFileSize {
        return nil, fmt.Errorf("file size %d exceeds maximum %d bytes", len(data), maxCUEFileSize)
    }
    // ... rest of parsing
}
```

---

## Next Steps (Phase 1)

1. Create `data-model.md` documenting the validation layers
2. Update `.claude/rules/cue.md` with comprehensive patterns
3. Design schema sync test infrastructure
4. Audit and document Go-only validation justifications
