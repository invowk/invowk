# AGENTS.md - Coding Agent Guidelines for invowk-cli

This document provides instructions for AI coding agents working in this repository.

## Project Overview

Invowk is a dynamically extensible command runner (like `just`) written in Go 1.25+. It supports multiple execution runtimes: native shell, virtual shell (mvdan/sh), and containerized execution (Docker/Podman). Commands are defined in `invkfile` files using CUE format.

## License

This project is licensed under the Eclipse Public License 2.0 (EPL-2.0). See the [LICENSE](LICENSE) file for the full license text.

### SPDX License Headers

**All Go source files MUST include an SPDX license header** as the very first line(s) of the file, before any package documentation or code. This ensures clear and machine-readable licensing information.

#### Required Header Format

Every `.go` file must start with this exact header:

```go
// SPDX-License-Identifier: EPL-2.0
```

#### Placement Rules

1. The SPDX header MUST be the **first line** of every Go source file
2. A blank line MUST follow the SPDX header
3. Package documentation comments (if any) come after the blank line
4. The `package` declaration follows the documentation

#### Complete Example

```go
// SPDX-License-Identifier: EPL-2.0

// Package config handles application configuration using Viper with CUE.
package config

import (
    "fmt"
)
```

#### Example Without Package Documentation

```go
// SPDX-License-Identifier: EPL-2.0

package main

func main() {
    // ...
}
```

#### Verification

Run the license header check to verify all source files have proper headers:

```bash
make license-check
```

#### Adding Headers to New Files

When creating new Go source files, always include the SPDX header. The header format is intentionally minimal to reduce boilerplate while maintaining legal clarity.

**DO NOT** include:
- Copyright year (changes over time, creates maintenance burden)
- Copyright holder name (tracked in LICENSE file and git history)
- Full license text (referenced via SPDX identifier)

**DO** include:
- Only the SPDX-License-Identifier line with `EPL-2.0`

## Build Commands

```bash
# Build the binary (default, stripped)
# On x86-64, targets x86-64-v3 microarchitecture by default (Haswell+ CPUs, 2013+)
make build

# Build with debug symbols for development
make build-dev

# Build with UPX compression (smallest size, requires UPX)
make build-upx

# Build all variants
make build-all

# Cross-compile for multiple platforms (x86-64 targets use v3 by default)
make build-cross

# Build for maximum compatibility (baseline x86-64)
make build GOAMD64=v1

# Install to $GOPATH/bin
make install

# Clean build artifacts
make clean

# Tidy dependencies
make tidy
```

### x86-64 Microarchitecture Levels

The project defaults to `GOAMD64=v3` for x86-64 builds, targeting CPUs from 2013+ (Intel Haswell, AMD Excavator). This enables AVX, AVX2, BMI1/2, FMA, and other modern instructions for better performance.

Available levels:
- `v1` - Baseline x86-64 (maximum compatibility, any 64-bit x86 CPU)
- `v2` - Nehalem+ (2008+): SSE4.2, POPCNT
- `v3` - Haswell+ (2013+): AVX, AVX2, BMI1/2, FMA **(default)**
- `v4` - Skylake-X+ (2017+): AVX-512

## Test Commands

```bash
# Run all tests (verbose)
make test

# Run tests in short mode (skips integration tests)
make test-short

# Run integration tests only
make test-integration

# Run a single test by name
go test -v -run TestFunctionName ./path/to/package/...

# Run a single test file
go test -v ./internal/config/config_test.go ./internal/config/config.go

# Run tests with coverage
go test -v -cover ./...

# Run tests for a specific package
go test -v ./internal/runtime/...
go test -v ./pkg/invkfile/...
```

## Code Style Guidelines

### Package Structure

- `cmd/invowk/` - CLI commands using Cobra
- `internal/` - Private packages (config, container, discovery, issue, runtime, sshserver, tui)
- `pkg/` - Public packages (pack, invkfile)
- `packs/` - Sample invowk packs for validation and reference

### Import Ordering

Imports should be organized in three groups, separated by blank lines:

```go
import (
    // 1. Standard library
    "context"
    "fmt"
    "os"

    // 2. External dependencies
    "github.com/spf13/cobra"
    "cuelang.org/go/cue"

    // 3. Internal packages
    "invowk-cli/internal/config"
    "invowk-cli/pkg/invkfile"
)
```

### Naming Conventions

- **Packages**: lowercase, single word preferred (e.g., `config`, `runtime`, `discovery`)
- **Exported types**: PascalCase (e.g., `Config`, `RuntimeMode`, `ExecutionContext`)
- **Unexported types/vars**: camelCase (e.g., `globalConfig`, `configPath`)
- **Constants**: PascalCase for exported, camelCase for unexported
- **Interfaces**: Use action-oriented names (e.g., `Engine`, `Runtime`)
- **Error types**: Prefix with `Err` (e.g., `ErrEngineNotAvailable`)

### Error Handling

- Always wrap errors with context using `fmt.Errorf("context: %w", err)`
- Use custom error types for specific error cases that callers need to handle
- Return early on errors to reduce nesting

```go
// Good
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Custom error type example
type ErrEngineNotAvailable struct {
    Engine string
    Reason string
}

func (e *ErrEngineNotAvailable) Error() string {
    return fmt.Sprintf("container engine '%s' is not available: %s", e.Engine, e.Reason)
}
```

### Documentation

- Every exported type, function, and constant MUST have a doc comment
- Package comments should be in the format `// Package name description.`
- Use complete sentences starting with the item name

```go
// Package config handles application configuration using Viper with CUE.
package config

// Config holds the application configuration
type Config struct {
    // ContainerEngine specifies whether to use "podman" or "docker"
    ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
}

// Load reads and parses the configuration file
func Load() (*Config, error) { ... }
```

### Struct Tags

Use JSON tags with snake_case, add mapstructure tags when using Viper:

```go
type Config struct {
    ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
    SearchPaths     []string        `json:"search_paths" mapstructure:"search_paths"`
}
```

### Testing Patterns

- Test files are named `*_test.go` in the same package
- Use `t.TempDir()` for temporary directories (auto-cleaned)
- Use table-driven tests for multiple cases
- Skip integration tests with `if testing.Short() { t.Skip(...) }`
- Reset global state in tests using cleanup functions

```go
func TestExample(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    originalEnv := os.Getenv("VAR")
    defer os.Setenv("VAR", originalEnv)

    // Test
    result, err := DoSomething()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Assert
    if result != expected {
        t.Errorf("got %v, want %v", result, expected)
    }
}
```

## Built-in Examples of Invowk Commands (invkfile.cue at project root)

- Always update the example file when there are any invkfile definition changes or features added/modified/removed.
- All commands should be idempotent and not cause any side effects on the host.
- No commands should be related to building invowk itself or manipulating any of its source code.
- Examples should range from simple (e.g.: native 'hello-world') to complex (e.g.: container 'hello-world' with the enable_host_ssh feature).
- Examples should illustrate the use of different features of Invowk, such as:
  - Native vs. Container execution
  - Volume mounts for Container execution
  - Environment variables
  - Host SSH access enabled vs. disabled
  - Capabilities checks (with and without alternatives)
  - Tools checks (with and without alternatives)
  - Custom checks (with and without alternatives)

## Sample Packs (packs/ directory)

The `packs/` directory contains sample invowk packs that serve as reference implementations and validation tests for the pack feature.

### Maintenance Requirements

- **Always update sample packs** when the design and/or implementation of invowk packs changes
- All packs in this directory must remain valid and pass `invowk pack validate --deep`
- Packs should demonstrate pack-specific features (script file references, cross-platform paths, etc.)
- Run validation after any pack-related changes: `go run . pack validate packs/<pack-name> --deep`

### Current Sample Packs

- `io.invowk.sample.invkpack` - Minimal cross-platform pack with a simple greeting command

### Pack Validation Checklist

When modifying pack-related code, verify:
1. All packs in `packs/` pass validation: `go run . pack validate packs/*.invkpack --deep`
2. Pack naming conventions are correctly enforced
3. Script path resolution works correctly (forward slashes, relative paths)
4. Nested pack detection works correctly
5. The `pkg/pack/` tests pass: `go test -v ./pkg/pack/...`

## Documentation Website (website/ directory)

The `website/` directory contains a Docusaurus-based documentation website for Invowk.

### CRITICAL: Documentation Maintenance Requirement

**The documentation website MUST be kept in sync with the codebase.** When making changes to the following, you MUST update the corresponding documentation:

1. **Invkfile Schema Changes** (`pkg/invkfile/invkfile_schema.cue`):
   - Update `website/docs/reference/invkfile-schema.md`
   - Update relevant feature documentation (e.g., new runtime options go in `runtime-modes/`)
   - Update examples in `website/docs/getting-started/` if affected

2. **Configuration Schema Changes** (`internal/config/config_schema.cue`):
   - Update `website/docs/reference/config-schema.md`
   - Update `website/docs/configuration/options.md`

3. **CLI Command Changes** (`cmd/invowk/*.go`):
   - Update `website/docs/reference/cli.md`
   - Update relevant feature documentation

4. **New Features or Major Changes**:
   - Add or update the appropriate section in `website/docs/`
   - Follow the existing documentation structure and tone (friendly, slightly humorous, progressive disclosure)

5. **TUI Component Changes** (`cmd/invowk/tui_*.go`):
   - Update relevant files in `website/docs/tui/`

### Documentation Structure

```
website/docs/
├── getting-started/     # Installation, quickstart, first invkfile
├── core-concepts/       # Invkfile format, commands, implementations
├── runtime-modes/       # Native, virtual, container execution
├── dependencies/        # Tools, filepaths, capabilities, custom checks
├── flags-and-arguments/ # CLI flags and positional arguments
├── environment/         # Env files, env vars, precedence
├── advanced/            # Interpreters, workdir, platform-specific
├── packs/               # Pack creation, validation, distribution
├── tui/                 # TUI components reference
├── configuration/       # Config file and options
└── reference/           # CLI, invkfile schema, config schema
```

### Documentation Style Guide

- Use a friendly, approachable tone with occasional humor
- Follow "progressive disclosure" - start simple, add complexity gradually
- Include practical examples for every feature
- Use admonitions (:::tip, :::warning, :::note) for important callouts
- Keep code examples concise and focused

### CRITICAL: Reusable Code Snippets Pattern

**All code blocks, CUE syntax examples, CLI commands, and technical snippets in documentation MUST use the reusable `<Snippet>` component** to avoid duplication across translations. This ensures:

1. Code examples are updated in ONE place, not in every translation file
2. Translations only contain translatable prose, not duplicated code
3. Consistency across all language versions

#### Snippet Component Usage

Documentation files MUST use `.mdx` extension (not `.md`) to use React components.

**Basic Usage:**

```mdx
---
sidebar_position: 1
---

import Snippet from '@site/src/components/Snippet';

# Page Title

Here's an example of an invkfile:

<Snippet id="getting-started/invkfile-basic-structure" />

Run the following command:

<Snippet id="cli/list-commands" />
```

**With Optional Title:**

```mdx
<Snippet id="runtime-modes/container-basic" title="container-example.cue" />
```

#### Adding New Snippets

All snippets are defined in `website/src/components/Snippet/snippets.ts`. When adding new code examples:

1. **Add the snippet** to `snippets.ts` with a descriptive hierarchical ID:
   ```typescript
   'section/feature/example-name': {
     language: 'cue',  // or 'bash', 'text', 'dockerfile', etc.
     code: `your code here`,
   },
   ```

2. **Use the snippet** in both English and translated MDX files:
   ```mdx
   <Snippet id="section/feature/example-name" />
   ```

#### Snippet Naming Convention

- Use hierarchical IDs matching documentation sections: `section/subsection/name`
- Examples:
  - `getting-started/invkfile-basic-structure`
  - `cli/list-commands`
  - `cli/output-list-commands`
  - `runtime-modes/container-basic`
  - `dependencies/tools-alternatives`

#### Escaping Template Literals

When snippets contain `${variable}` syntax (e.g., shell variables), escape them:

```typescript
// WRONG - will be interpreted as JS template literal
code: `files: [".env.${INVOWK_ENV}"]`

// CORRECT - escaped
code: `files: [".env.\${INVOWK_ENV}"]`
```

#### Current Snippet Categories

- `getting-started/*` - First invkfile examples
- `core-concepts/*` - Schema, structure, syntax examples
- `runtime-modes/*` - Native, virtual, container examples
- `dependencies/*` - Tools, filepaths, capabilities, custom checks
- `environment/*` - Env files and variables
- `flags-args/*` - Flags and positional arguments
- `advanced/*` - Interpreters, workdir, platform-specific
- `packs/*` - Pack creation and validation
- `tui/*` - TUI component examples
- `config/*` - Configuration examples
- `cli/*` - CLI commands and output examples

#### Converting Existing Documentation

When updating or creating documentation:

1. **File extension**: Rename `.md` to `.mdx`
2. **Add import**: Add `import Snippet from '@site/src/components/Snippet';` after frontmatter
3. **Replace code blocks**: Replace inline code blocks with `<Snippet id="..." />`
4. **Keep prose in translations**: Only translatable text should differ between locales
5. **Update all locales**: Apply the same structure to all translation files

### Testing Documentation Changes

After making documentation changes:

```bash
cd website
npm install    # First time only
npm start      # Start dev server at localhost:3000 (English only)
```

Verify:
1. No build errors
2. Navigation works correctly
3. Code examples render properly
4. Links are not broken

**Testing specific locales in dev mode:**
```bash
npm start -- --locale pt-BR   # Start dev server with Portuguese
```

**Testing all locales (recommended before committing):**
```bash
npm run build    # Build all locales
npm run serve    # Serve at localhost:3000, language switcher works
```

### CRITICAL: Internationalization (i18n) Requirements

The documentation website supports multiple languages. **All supported locales MUST be kept in sync.**

**Supported Locales:**
- `en` (English) - Primary/source language in `website/docs/`
- `pt-BR` (Português Brasil) - Translations in `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`

**Important:** The dev server (`npm start`) only serves ONE locale at a time. To test the language switcher, use `npm run build && npm run serve`.

**When updating documentation:**

1. **Always update the English version first** (`website/docs/`)
2. **Then update the same file in ALL other locales** - The file structure must mirror exactly:
   - English: `website/docs/getting-started/installation.md`
   - Portuguese: `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/getting-started/installation.md`

3. **When adding new documentation files:**
   - Create the file in `website/docs/`
   - Copy it to ALL locale directories under `website/i18n/<locale>/docusaurus-plugin-content-docs/current/`

4. **When deleting documentation files:**
   - Remove from `website/docs/`
   - Remove from ALL locale directories

5. **Translation JSON files** (`website/i18n/<locale>/*.json`):
   - These contain UI string translations (navbar, footer, theme labels)
   - Regenerate with: `cd website && npx docusaurus write-translations --locale <locale>`

**Verification:**
```bash
cd website
npm run build  # Must succeed for ALL locales without errors
npm run serve  # Test language switcher at localhost:3000
```

The build will fail if locale files are missing or malformed. Always test the language switcher after documentation changes.

## Key Guidelines

- In all planning and design decisions, always consider that the code must be highly testable, maintainable, and extensible.
- Always add unit and integration tests to new code.
- Always document the code (functions, structs, etc.) with comments.
- Always use descriptive variable names.
- Always adjust the README and other documentation as needed when making significant changes to the codebase.
- Always refactor unit and integration tests when needed after code changes, considering both the design and semantics of the code changes.
- After you finish code design and implementation changes, always double-check for leftovers that were not removed or changed after refactoring (e.g.: tests, CUE type definitions, README or documentation instructions, etc.).
- Always follow the best practices for the programming language being used.

## Interface Design

Define interfaces for testability and extensibility:

```go
// Runtime defines the interface for command execution
type Runtime interface {
    Name() string
    Execute(ctx *ExecutionContext) *Result
    Available() bool
    Validate(ctx *ExecutionContext) error
}
```

## Context Usage

Use `context.Context` for cancellation and timeouts in long-running operations:

```go
func (e *Engine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
    // Check context before expensive operations
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    // ... implementation
}
```

## File Organization

- Keep files focused on a single responsibility
- Separate test helpers into dedicated functions
- Use `_test.go` suffix for test files only
- Schema files use `.cue` extension (e.g., `config_schema.cue`, `invkfile_schema.cue`)

## Dependencies

Key dependencies:
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `cuelang.org/go` - CUE language support for configuration/schema
- `github.com/charmbracelet/*` - TUI components (lipgloss, bubbletea, huh)
- `mvdan.cc/sh/v3` - Virtual shell implementation
