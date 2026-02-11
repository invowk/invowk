# Data Model: CUE Library Usage Optimization

**Date**: 2026-01-30 | **Status**: Complete

## Overview

This document defines the validation architecture and data flow for CUE-based configuration in Invowk. The model establishes clear boundaries between CUE schema validation and Go runtime validation.

---

## Validation Layer Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     User Input Layer                            │
│                   (invkfile.cue files)                          │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Layer 0: Early Guards                          │
│  ┌────────────────┐  ┌─────────────────────────────────────┐   │
│  │ File Size      │  │ File Extension                      │   │
│  │ Check (≤5MB)   │  │ Check (.cue only)                   │   │
│  └────────────────┘  └─────────────────────────────────────┘   │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Layer 1: CUE Schema Validation                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ • Closed struct enforcement (no unknown fields)          │  │
│  │ • Type constraints (string, bool, int, list)            │  │
│  │ • Regex patterns (tool names, paths, env vars)          │  │
│  │ • Length limits (strings.MaxRunes)                       │  │
│  │ • Range constraints (>=0, <=255)                         │  │
│  │ • Mutual exclusivity (containerfile XOR image)          │  │
│  │ • Required field enforcement                             │  │
│  │ • Enum validation ("native" | "virtual" | "container")  │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Layer 2: CUE Decode                            │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ • value.Decode(&GoStruct) for type-safe extraction       │  │
│  │ • JSON tag alignment (CUE field names → Go struct tags)  │  │
│  │ • Optional field handling (omitempty, pointers)          │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Layer 3: Go Post-Decode Validation             │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ • Security validation (ReDoS pattern safety)             │  │
│  │ • Cross-field validation (runtime-specific fields)       │  │
│  │ • Filesystem access (script file existence)              │  │
│  │ • Runtime type coercion (flag/arg value parsing)         │  │
│  │ • Command hierarchy validation (leaf-only args)          │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────┬───────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Validated Go Struct                         │
│                    (ready for execution)                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## Schema Files

### invkfile_schema.cue

**Location**: `pkg/invkfile/invkfile_schema.cue`

**Root Definition**: `#Invkfile`

**Key Types**:

| CUE Type | Description | Go Type |
|----------|-------------|---------|
| `#Invkfile` | Root invkfile structure | `Invkfile` |
| `#Command` | Command definition | `Command` |
| `#Implementation` | Script implementation | `Implementation` |
| `#RuntimeConfig` | Runtime configuration | `RuntimeConfig` |
| `#PlatformConfig` | Platform specification | `PlatformConfig` |
| `#DependsOn` | Dependency declaration | `DependsOn` |
| `#Flag` | Command flag | `Flag` |
| `#Argument` | Positional argument | `Argument` |
| `#EnvConfig` | Environment configuration | `EnvConfig` |

**Validation Constraints**:

| Constraint Type | Example | Purpose |
|-----------------|---------|---------|
| Closed struct | `close({...})` | Reject unknown fields |
| Regex pattern | `=~"^[a-zA-Z][a-zA-Z0-9_-]*$"` | Validate name format |
| Length limit | `strings.MaxRunes(256)` | Prevent excessive size |
| Range | `>=0 & <=65535` | Port number validation |
| Mutual exclusion | `containerfile XOR image` | Exactly one option |
| Non-empty list | `[_, ...]` | At least one element |

### invkmod_schema.cue

**Location**: `pkg/invkmod/invkmod_schema.cue`

**Root Definition**: `#Invkmod`

**Key Types**:

| CUE Type | Description | Go Type |
|----------|-------------|---------|
| `#Invkmod` | Module metadata | `Invkmod` |
| `#ModuleRequirement` | Dependency declaration | `ModuleRequirement` |

### config_schema.cue

**Location**: `internal/config/config_schema.cue`

**Root Definition**: `#Config`

**Key Types**:

| CUE Type | Description | Go Type |
|----------|-------------|---------|
| `#Config` | Application config | `Config` |
| `#VirtualShellConfig` | Shell settings | `VirtualShellConfig` |
| `#UIConfig` | UI settings | `UIConfig` |
| `#ContainerConfig` | Container settings | `ContainerConfig` |

---

## Field Naming Convention

CUE field names use `snake_case`, mapped to Go `PascalCase` via JSON tags:

```cue
// CUE Schema
#Config: close({
    container_engine: #ContainerEngine
    includes:         [...#IncludeEntry]
    default_runtime:  #RuntimeType
})
```

```go
// Go Struct
type Config struct {
    ContainerEngine ContainerEngine `json:"container_engine"`
    Includes        []IncludeEntry  `json:"includes"`
    DefaultRuntime  RuntimeMode     `json:"default_runtime"`
}
```

**Rule**: Every CUE field name MUST have a matching JSON tag in Go.

---

## Validation Responsibility Matrix

This matrix defines which layer handles each validation concern:

| Validation Concern | Layer | Rationale |
|--------------------|-------|-----------|
| Unknown field rejection | CUE | `close({...})` handles this |
| Type checking | CUE | Native CUE capability |
| String format (regex) | CUE | Declarative, visible in schema |
| Length limits | CUE | `strings.MaxRunes()` |
| Numeric ranges | CUE | Constraint expressions |
| Required fields | CUE | Field without `?` suffix |
| Optional fields | CUE | Field with `?` suffix |
| Enum values | CUE | Disjunction (`\|`) |
| Mutual exclusivity | CUE | XOR constraints |
| ReDoS prevention | Go | Requires regex analysis |
| Path traversal (filesystem) | Go | Requires `filepath.Walk` |
| File existence | Go | Requires `os.Stat` |
| Cross-field logic | Go | Complex conditional logic |
| Runtime-specific fields | Go | Depends on runtime selection |
| Command hierarchy | Go | Requires tree analysis |

---

## Error Message Format

All validation errors MUST include a JSON path prefix:

```
cmds[0].implementations[2].script: value exceeds maximum length (10MB)
```

**Format**: `<path>: <message>`

**Path Components**:
- Field names: `cmds`, `implementations`, `script`
- Array indices: `[0]`, `[2]`
- Nested paths: `cmds[0].implementations[2].script`

**CUE Error Formatting**:

```go
func formatCUEError(err errors.Error) error {
    var msgs []string
    for _, e := range errors.Errors(err) {
        path := e.Path().String()
        msg := e.Error()
        msgs = append(msgs, fmt.Sprintf("%s: %s", path, msg))
    }
    return fmt.Errorf("validation failed:\n  %s", strings.Join(msgs, "\n  "))
}
```

---

## Discriminated Union Pattern

For runtime configuration, the `name` field acts as discriminator:

```cue
#RuntimeConfig: close({
    name: "native" | "virtual" | "container"

    // Common fields (all runtimes)
    env_inherit_mode?: #EnvInheritMode

    // Native/Container only
    interpreter?: string

    // Container only
    containerfile?: string
    image?:         string
    volumes?:       [...string]
    ports?:         [...string]
})
```

**Go Validation** (post-decode):

```go
func (r *RuntimeConfig) Validate() error {
    switch r.Name {
    case RuntimeNative:
        // interpreter optional, container fields forbidden
        if r.Containerfile != "" || r.Image != "" {
            return fmt.Errorf("container fields not allowed for native runtime")
        }
    case RuntimeVirtual:
        // No interpreter, no container fields
        if r.Interpreter != "" {
            return fmt.Errorf("interpreter not supported for virtual runtime")
        }
    case RuntimeContainer:
        // Must have exactly one of containerfile or image
        if (r.Containerfile == "") == (r.Image == "") {
            return fmt.Errorf("container runtime requires exactly one of containerfile or image")
        }
    }
    return nil
}
```

---

## Schema Sync Test Design

Tests verify Go struct JSON tags match CUE schema field names:

```go
// pkg/invkfile/sync_test.go
func TestInvkfileSchemaSync(t *testing.T) {
    // Load CUE schema
    ctx := cuecontext.New()
    schema := ctx.CompileString(invkfileSchema)

    // Extract all field names from CUE #Invkfile definition
    invkfileDef := schema.LookupPath(cue.ParsePath("#Invkfile"))
    cueFields := extractCUEFields(t, invkfileDef)

    // Extract JSON tags from Go Invkfile struct
    goFields := extractGoJSONTags(t, reflect.TypeOf(Invkfile{}))

    // Verify every CUE field has matching Go tag
    for field := range cueFields {
        if !goFields[field] {
            t.Errorf("CUE field %q not found in Go struct", field)
        }
    }

    // Verify every Go tag has matching CUE field
    for field := range goFields {
        if !cueFields[field] {
            t.Errorf("Go JSON tag %q not found in CUE schema", field)
        }
    }
}
```

---

## File Size Limit

Early guard against large files:

| Limit | Default | Configurable | Error Message |
|-------|---------|--------------|---------------|
| Maximum file size | 5 MB | Yes | `file exceeds maximum size of 5MB` |

**Implementation**:

```go
const DefaultMaxCUEFileSize = 5 * 1024 * 1024 // 5 MB

func ParseBytes(data []byte, path string, opts ...ParseOption) (*Invkfile, error) {
    cfg := parseConfig{maxSize: DefaultMaxCUEFileSize}
    for _, opt := range opts {
        opt(&cfg)
    }

    if int64(len(data)) > cfg.maxSize {
        return nil, fmt.Errorf("%s: file size %d bytes exceeds maximum %d bytes",
            path, len(data), cfg.maxSize)
    }

    // Continue with CUE parsing...
}
```

---

## Migration Notes

Changes from current implementation:

1. **Add file size guard** - New check before CUE parsing
2. **Add schema sync tests** - New test file per package
3. **Remove redundant Go validation** - Interpreter whitespace check
4. **Expand cue.md rules** - Comprehensive documentation
5. **Improve error formatting** - Consistent path prefixes
