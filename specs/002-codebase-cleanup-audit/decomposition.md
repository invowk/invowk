# Decomposition Design: cmd/invowk/cmd.go

**Date**: 2026-01-29
**Source File**: `cmd/invowk/cmd.go` (2,927 lines)
**Target**: 6 files, none exceeding 800 lines

## Overview

The monolithic `cmd.go` contains 59 functions, 5 types, and 6 global variables handling command discovery, execution, validation, and output rendering. This document specifies the exact function-to-file assignments.

## File Structure After Decomposition

```
cmd/invowk/
├── cmd.go              # Core: init, types, shared utilities (~350 lines)
├── cmd_discovery.go    # Command discovery and registration (~625 lines)
├── cmd_execute.go      # Execution orchestration (~555 lines)
├── cmd_validate.go     # Dependency validation (~770 lines) ⚠️ Over target
├── cmd_validate_input.go # Flag/argument validation (~160 lines)
└── cmd_render.go       # Output/error rendering (~420 lines)
```

**Note**: `cmd_validate.go` exceeds 800-line target at 770 lines but represents a cohesive responsibility. Further splitting would create unnecessary fragmentation.

---

## File 1: `cmd.go` (Core) — ~350 lines

### Purpose
Core package initialization, type definitions, shared utilities, and global state.

### Contents

#### Package Declaration & Imports
```go
// SPDX-License-Identifier: MPL-2.0

// Package cmd implements the invowk CLI commands.
package cmd
```

#### Global Variables (Lines 47-62)
```go
var (
    runtimeOverride  string
    fromSource       string
    sshServerInstance *sshserver.Server
    sshServerMu       sync.Mutex
    listFlag         bool
    cmdCmd           *cobra.Command
)
```

#### Constants (Lines 118-123)
```go
const (
    ArgErrMissingRequired = iota
    ArgErrTooMany
    ArgErrInvalidValue
)
```

#### Type Definitions (Lines 125-173)

| Type | Lines | Purpose |
|------|-------|---------|
| `DependencyError` | 126-135 | Missing dependency error |
| `ArgumentValidationError` | 141-151 | Invalid argument error |
| `SourceFilter` | 155-160 | Source disambiguation filter |
| `SourceNotFoundError` | 163-166 | Unknown source error |
| `AmbiguousCommandError` | 170-173 | Multiple source conflict |

#### Error Methods (Lines 186-209)
```go
func (e *DependencyError) Error() string
func (e *ArgumentValidationError) Error() string
func (e *SourceNotFoundError) Error() string
func (e *AmbiguousCommandError) Error() string
```

#### Functions to Keep

| Function | Lines | Purpose |
|----------|-------|---------|
| `init()` | 176-184 | Cobra initialization (calls discovery) |
| `normalizeSourceName()` | 214-229 | Source name normalization |
| `ParseSourceFilter()` | 234-260 | **Exported** - Source filter parsing |

---

## File 2: `cmd_discovery.go` — ~625 lines

### Purpose
Command discovery, registration with Cobra, and listing.

### Header
```go
// SPDX-License-Identifier: MPL-2.0

package cmd

// This file handles command discovery from invowkfiles and modules,
// registration with Cobra, and command listing.
```

### Functions

| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `registerDiscoveredCommands()` | 262-370 | 108 | Main discovery entry point |
| `buildLeafCommand()` | 371-532 | 162 | Builds Cobra command from CommandInfo |
| `buildCommandUsageString()` | 533-559 | 27 | Formats usage string with args |
| `buildArgsDocumentation()` | 560-589 | 30 | Generates arg docs |
| `buildCobraArgsValidator()` | 590-599 | 10 | Creates Cobra PositionalArgs |
| `completeCommands()` | 600-656 | 57 | Shell completion provider |
| `listCommands()` | 657-822 | 166 | Lists commands with sources |
| `formatSourceDisplayName()` | 823-837 | 15 | Formats source ID for display |

### Dependencies
- Imports: `discovery`, `invowkfile`, `config`
- Uses: `cmdCmd` (global), `listFlag` (global), `fromSource` (global)

---

## File 3: `cmd_execute.go` — ~555 lines

### Purpose
Command execution orchestration, runtime selection, and TUI/SSH integration.

### Header
```go
// SPDX-License-Identifier: MPL-2.0

package cmd

// This file handles command execution including runtime selection,
// context setup, TUI interaction, and SSH server management.
```

### Functions

| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `parseEnvVarFlags()` | 838-865 | 28 | Parses --env-var flags |
| `runCommandWithFlags()` | 866-1121 | 256 | **Core execution** |
| `runDisambiguatedCommand()` | 1122-1220 | 99 | Executes after disambiguation |
| `checkAmbiguousCommand()` | 1221-1278 | 58 | Detects source conflicts |
| `runCommand()` | 1279-1298 | 20 | Entry point (cmdCmd.RunE) |
| `executeInteractive()` | 1299-1387 | 89 | TUI execution handler |
| `bridgeTUIRequests()` | 1388-1399 | 12 | TUI server bridge |
| `createRuntimeRegistry()` | 1400-1424 | 25 | Runtime initialization |
| `ensureSSHServer()` | 1425-1444 | 20 | SSH server startup |
| `stopSSHServer()` | 1445-1464 | 20 | SSH server cleanup |

### Dependencies
- Imports: `runtime`, `sshserver`, `tuiserver`, `tui`, `config`, `invowkfile`
- Uses: `sshServerInstance`, `sshServerMu`, `runtimeOverride` (globals)
- Calls: `validateDependencies()` (from cmd_validate.go)
- Calls: `validateFlagValues()`, `validateArguments()` (from cmd_validate_input.go)
- Calls: `Render*Error()` functions (from cmd_render.go)

---

## File 4: `cmd_validate.go` — ~770 lines

### Purpose
All dependency validation: tools, commands, filepaths, capabilities, custom checks, env vars.

### Header
```go
// SPDX-License-Identifier: MPL-2.0

package cmd

// This file handles all dependency validation including tools, commands,
// filepaths, capabilities, custom checks, and environment variables.
```

### Functions

#### Orchestration
| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `validateDependencies()` | 1465-1515 | 51 | Main validation entry point |

#### Command Dependencies
| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `checkCommandDependenciesExist()` | 1516-1589 | 74 | Validates required commands |

#### Tool Dependencies
| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `checkToolDependenciesWithRuntime()` | 1590-1637 | 48 | Runtime-aware dispatcher |
| `validateToolNative()` | 1638-1647 | 10 | Native tool check |
| `validateToolInVirtual()` | 1648-1680 | 33 | Virtual runtime check |
| `validateToolInContainer()` | 1681-1711 | 31 | Container check |
| `checkToolDependencies()` | 1971-2010 | 40 | Legacy host-only check |

#### Custom Checks
| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `validateCustomCheckOutput()` | 1712-1755 | 44 | Evaluates check result |
| `checkCustomCheckDependencies()` | 1756-1809 | 54 | Runtime-aware dispatcher |
| `validateCustomCheckNative()` | 1810-1818 | 9 | Native custom check |
| `validateCustomCheckInVirtual()` | 1819-1845 | 27 | Virtual runtime check |
| `validateCustomCheckInContainer()` | 1846-1875 | 30 | Container check |
| `checkCustomChecks()` | 2011-2055 | 45 | Legacy host-only check |

#### Filepath Dependencies
| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `checkFilepathDependenciesWithRuntime()` | 1876-1908 | 33 | Runtime-aware dispatcher |
| `validateFilepathInContainer()` | 1909-1970 | 62 | Container filepath check |
| `checkFilepathDependencies()` | 2056-2081 | 26 | Host filepath check |
| `validateFilepathAlternatives()` | 2082-2111 | 30 | Alternative path validation |
| `validateSingleFilepath()` | 2112-2152 | 41 | Single path validation |

#### Permission Utilities
| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `isReadable()` | 2153-2171 | 19 | Read permission check |
| `isWritable()` | 2172-2193 | 22 | Write permission check |
| `isExecutable()` | 2194-2221 | 28 | Execute permission check |

#### Capability & Env Var Dependencies
| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `checkCapabilityDependencies()` | 2222-2286 | 65 | Container capability check |
| `checkEnvVarDependencies()` | 2287-2357 | 71 | Environment variable check |

### Dependencies
- Imports: `runtime`, `invowkfile`
- Returns: `*DependencyError` (from cmd.go)

---

## File 5: `cmd_validate_input.go` — ~160 lines

### Purpose
User input validation: flags and positional arguments.

### Header
```go
// SPDX-License-Identifier: MPL-2.0

package cmd

// This file handles validation of user-provided flags and arguments.
```

### Functions

| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `captureUserEnv()` | 2358-2368 | 11 | Captures current env |
| `isWindows()` | 2369-2374 | 6 | Platform detection |
| `FlagNameToEnvVar()` | 2375-2382 | 8 | **Exported** flag→env mapping |
| `ArgNameToEnvVar()` | 2383-2390 | 8 | **Exported** arg→env mapping |
| `validateFlagValues()` | 2391-2425 | 35 | Flag value validation |
| `validateArguments()` | 2426-2513 | 88 | Argument validation |

### Dependencies
- Returns: `*ArgumentValidationError` (from cmd.go)

---

## File 6: `cmd_render.go` — ~420 lines

### Purpose
Output formatting and error rendering using Lip Gloss styling.

### Header
```go
// SPDX-License-Identifier: MPL-2.0

package cmd

// This file handles output formatting and styled error rendering.
```

### Functions

| Function | Original Lines | Size | Purpose |
|----------|---------------|------|---------|
| `RenderArgumentValidationError()` | 2514-2597 | 84 | **Exported** arg error display |
| `RenderArgsSubcommandConflictError()` | 2598-2655 | 58 | **Exported** conflict display |
| `RenderDependencyError()` | 2656-2753 | 98 | **Exported** dep error display |
| `RenderHostNotSupportedError()` | 2754-2793 | 40 | **Exported** platform error |
| `RenderRuntimeNotAllowedError()` | 2794-2833 | 40 | **Exported** runtime error |
| `RenderSourceNotFoundError()` | 2834-2878 | 45 | **Exported** source error |
| `RenderAmbiguousCommandError()` | 2879-2927 | 49 | **Exported** ambiguity error |

### Dependencies
- Imports: `lipgloss`, `invowkfile`
- Uses: Error types from cmd.go

---

## Dependency Graph

```
cmd.go (core)
    │
    ├── cmd_discovery.go
    │       ↓ calls registerDiscoveredCommands from init()
    │
    ├── cmd_execute.go
    │       ↓ calls runCommand from cobra.RunE
    │       │
    │       ├──→ cmd_validate.go (validateDependencies)
    │       ├──→ cmd_validate_input.go (validateFlagValues, validateArguments)
    │       └──→ cmd_render.go (Render*Error)
    │
    ├── cmd_validate.go
    │       │ (standalone, called by execute)
    │
    ├── cmd_validate_input.go
    │       │ (standalone, called by execute)
    │
    └── cmd_render.go
            │ (standalone, called by execute and discovery)
```

No circular dependencies: All files depend on `cmd.go` for types/globals; `cmd_execute.go` orchestrates calls to validation and rendering.

---

## Test File Organization

Current: `cmd_test.go` (single file)

After split, consider:
```
cmd/invowk/
├── cmd_test.go            # Core type/utility tests
├── cmd_discovery_test.go  # Discovery/registration tests
├── cmd_execute_test.go    # Execution tests
├── cmd_validate_test.go   # Dependency validation tests
└── cmd_render_test.go     # Rendering tests (if any)
```

**Note**: Test file splitting is optional and can follow incrementally.

---

## Implementation Order

1. **Create `cmd_render.go`** - No dependencies on other new files
2. **Create `cmd_validate_input.go`** - Simple, standalone
3. **Create `cmd_validate.go`** - Standalone, uses types from cmd.go
4. **Create `cmd_execute.go`** - Depends on validate + render
5. **Create `cmd_discovery.go`** - Depends on render
6. **Update `cmd.go`** - Remove moved functions, keep core

Each step should:
1. Move functions to new file
2. Run `make lint` (verify no import cycles)
3. Run `make test` (verify no breakage)
4. Verify line count with `wc -l`

---

## Verification Checklist

After decomposition:

- [ ] `cmd.go` ≤ 400 lines
- [ ] `cmd_discovery.go` ≤ 700 lines
- [ ] `cmd_execute.go` ≤ 600 lines
- [ ] `cmd_validate.go` ≤ 800 lines
- [ ] `cmd_validate_input.go` ≤ 200 lines
- [ ] `cmd_render.go` ≤ 500 lines
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make test-cli` passes
- [ ] All new files have SPDX headers
