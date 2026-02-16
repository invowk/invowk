---
paths:
  - "**/*.cue"
  - "pkg/invowkfile/**"
  - "pkg/invowkmod/**"
  - "internal/config/**"
  - "pkg/cueutil/**"
---

# CUE Patterns

## String Validation

### Use Regex Constraints, Not String Functions

CUE's `strings.HasPrefix()`, `strings.Contains()`, `strings.HasSuffix()` return **strings**, not booleans. You cannot negate them with `!`.

```cue
// ❌ WRONG - These functions return strings, not booleans
field?: string & !strings.HasPrefix("/")
field?: string & !strings.Contains("..")

// Error: "invalid operation !strings.HasPrefix(...) (! string)"

// ✅ CORRECT - Use regex constraints
field?: string & =~"^[^/]"      // Does not start with /
field?: string & !~"\\.\\."    // Does not contain ..
```

### Common Regex Patterns

| Pattern | Meaning | Example Use |
|---------|---------|-------------|
| `=~"^[^/]"` | Does not start with `/` | Relative paths only |
| `!~"\\.\\."` | Does not contain `..` | No path traversal |
| `=~"^[a-zA-Z]"` | Starts with letter | Identifiers |
| `=~"^\\s*\\S.*$"` | Non-empty after trim | Required text fields |
| `=~"^[A-Za-z_][A-Za-z0-9_]*$"` | POSIX identifier | Env var names |

### Safe String Functions

These CUE string functions ARE safe to use for constraints:

```cue
import "strings"

// ✅ These work correctly
field?: string & strings.MaxRunes(4096)  // Length limit
field?: string & strings.MinRunes(1)      // Non-empty
```

## Schema Organization

### Discriminated Unions for Runtime Types

Use closed structs with discriminated unions for type-safe variant handling:

```cue
#RuntimeConfig: #RuntimeConfigNative | #RuntimeConfigVirtual | #RuntimeConfigContainer

#RuntimeConfigNative: close({
    name: "native"
    interpreter?: string
})

#RuntimeConfigContainer: close({
    name: "container"
    image?: string
    containerfile?: string
})
```

### Required List with Minimum One Element

```cue
// Require at least one element
implementations: [...#Implementation] & [_, ...]
```

### Field Deprecation

Use bottom (`_|_`) to forbid deprecated fields with clear error:

```cue
// Forbid old field name
commands?: _|_  // Use 'cmds' instead
```

## Validation Responsibilities

### CUE Schema vs Go Validation

| Validation Type | Where | Why |
|-----------------|-------|-----|
| Format patterns (regex) | CUE | Static, declarative |
| Length limits | CUE | `strings.MaxRunes()` |
| Enum values | CUE | Type-safe unions |
| Path traversal security | Go | Requires `filepath.Rel()` |
| File existence | Go | Runtime filesystem access |
| Cross-field logic | Go | CUE can't express easily |

### Adding Comments for Split Validation

When validation is split between CUE and Go, add comments:

```cue
// Path must be relative (not start with /) and cannot contain path traversal (..)
// Note: Additional path security validation is performed in Go (see validation_filesystem.go)
containerfile?: string & strings.MaxRunes(4096) & =~"^[^/]" & !~"\\.\\."
```

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Using `!strings.HasPrefix()` | "invalid operation (! string)" | Use `=~"^[^x]"` regex |
| Using `!strings.Contains()` | "invalid operation (! string)" | Use `!~"pattern"` regex |
| Forgetting to escape `.` in regex | Matches any char instead of literal | Use `\\.` |
| Missing `close()` on struct | Extra fields silently allowed | Always use `close({...})` |
