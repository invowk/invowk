# Linting

## Critical Linters

**CRITICAL: The `decorder` and `funcorder` linters MUST always remain enabled in `.golangci.yaml`.**

These linters enforce:
- **decorder**: Consistent ordering of declarations within files (const → var → type → func)
- **funcorder**: Consistent ordering of functions (exported functions before unexported)

### Why These Are Critical

1. **Code consistency**: Ensures all Go files follow the same structural pattern
2. **Readability**: Makes it easy to find declarations and functions in any file
3. **Review efficiency**: Reduces cognitive load during code review

### Rules

- Never disable `decorder` or `funcorder` in `.golangci.yaml`
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
