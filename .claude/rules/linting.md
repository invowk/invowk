# Linting

## Critical Linters

**CRITICAL: The `decorder` and `funcorder` linters MUST always remain enabled in `.golangci.toml`.**

These linters enforce:
- **decorder**: Consistent ordering of declarations within files (const → var → type → func)
- **funcorder**: Consistent ordering of functions (exported functions before unexported)

### Why These Are Critical

1. **Code consistency**: Ensures all Go files follow the same structural pattern
2. **Readability**: Makes it easy to find declarations and functions in any file
3. **Review efficiency**: Reduces cognitive load during code review

### Rules

- Never disable `decorder` or `funcorder` in `.golangci.toml`
- Never add `//nolint:decorder` or `//nolint:funcorder` directives
- When adding new code, follow the declaration order: `const`, `var`, `type`, `func`
- When adding new functions, place exported functions before unexported ones

### Declaration Order

Files should follow this structure:
```go
// SPDX-License-Identifier: EPL-2.0

// Package documentation
package mypackage

import (...)

const (...)

var (...)

type (...)

// Exported functions first
func PublicFunction() {}

// Unexported functions after
func privateFunction() {}
```

## Configuration Documentation

**The `.golangci.toml` file must be kept documented when linters, formatters, or settings are added or changed.**

### Documentation Requirements

When modifying `.golangci.toml`:

1. **New linters**: Add a comment above each linter entry with its official description from `golangci-lint linters`
2. **New settings**: Add inline comments explaining non-obvious configuration values
3. **Setting changes**: Update comments if the meaning or rationale changes
4. **Exclusions**: Document why specific rules, paths, or functions are excluded

### Comment Style

```toml
[linters]
enable = [
    # modernize: A suite of analyzers that suggest simplifications to Go code.
    "modernize",
]

[linters.settings.example]
setting-name = "value"  # Brief explanation of what this controls
```

### Rationale

- Self-documenting configuration reduces onboarding friction
- Comments explain the "why" behind non-obvious settings
- Future maintainers can understand exclusion rationale without archaeology
