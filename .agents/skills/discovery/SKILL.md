---
name: discovery
description: >-
  Module/command discovery, precedence order, collision detection, source
  tracking, vendored module namespace policy, diagnostics, and command-scope
  discovery inputs. Use when working on internal/discovery/, discovery adapters,
  command dependency scope checks, or how invowkfiles/modules are found,
  namespaced, locked, and aggregated.
---

# Discovery Skill

This skill covers the discovery system in Invowk, which locates and aggregates commands from multiple sources with clear precedence rules and collision detection.

Use this skill when working on:
- `internal/discovery/` - Discovery and aggregation logic
- `internal/app/commandadapters/discovery.go` - production discovery adapter wiring
- `internal/app/deps/` - command-scope checks that consume discovered command identities
- Module resolution and dependency handling
- Command collision detection and disambiguation
- Source tracking and precedence

---

## Discovery Precedence Order

The discovery system implements a **strict 4-level precedence hierarchy**:

| Priority | Source | Description |
|----------|--------|-------------|
| 1 (Highest) | Current Directory | `invowkfile.cue` in the working directory |
| 2 | Local Modules | Sibling `*.invowkmod` directories in current directory and their vendored dependencies |
| 3 | Config Includes | Module paths from `config.Includes` and their vendored dependencies |
| 4 (Lowest) | User Commands | `~/.invowk/cmds/` — modules only, non-recursive, and their vendored dependencies |

**Key Behavior:**
- Non-module sources (current dir invowkfile): First source **shadows** later ones
- Module commands: **All included** with ambiguity flagging for transparent namespace
- User commands dir only discovers `*.invowkmod` immediate children (no loose invowkfiles, no recursion)

---

## File Discovery Algorithm

Discovery has two parallel tracks:

### Track A: Invowkfile Discovery

```go
// Single-level check (current directory only)
discoverInDir(dir)  // Looks for invowkfile.cue OR invowkfile
```

**File Priority:** `.cue` extension preferred over non-suffixed `invowkfile`

**Note:** Invowkfile discovery is limited to the current directory. The user commands directory (`~/.invowk/cmds/`) only discovers modules, not loose invowkfiles.

### Track B: Module Discovery

```go
// Non-recursive - only immediate subdirectories
discoverModulesInDirWithDiagnostics(dir) → ([]*DiscoveredFile, []Diagnostic)
```

**Module Validation:**
- Uses `invowkmod.IsModule()` to verify directory structure
- Skips reserved module name `"invowkfile"` (reserved for canonical namespace)
- **Graceful degradation**: Invalid modules emit diagnostics (warnings) and are skipped

### Track C: Vendored Module Discovery

After discovering each module (from any module source), the system may scan its `invowk_modules/` directory for vendored dependencies:

```go
// Flat, one-level scan of <parentModule.Path>/invowk_modules/
discoverVendoredModulesWithDiagnostics(parentModule) → ([]*DiscoveredFile, []Diagnostic)
```

**Key behaviors:**
- Loads the parent module lock file and includes vendored modules only when they are declared and locked by the parent dependency graph.
- Verifies vendored module content hashes; hash mismatches are hard errors, not warnings.
- Skips vendored modules with missing transitive dependencies from the parent graph.
- Sets `ParentModule` on each vendored `DiscoveredFile` for ownership tracking
- Vendored modules use `SourceModule` (no new Source enum value)
- **No recursion**: Only immediate `invowk_modules/` children are scanned; nested vendored modules are NOT recursed into (emits `vendored_nested_ignored` diagnostic)
- Vendored modules are always ordered after their parent in the files slice

**DRY helper** used at all 3 module scan sites (local, includes, user-dir):

```go
// Appends module files + scans vendored for each, consolidating diagnostics
appendModulesWithVendored(files, diagnostics, moduleFiles, moduleDiags) → (files, diagnostics)
```

---

## Source Tracking Types

### Source Enum

```go
const (
    SourceCurrentDir   Source = iota  // "current directory"
    SourceModule                      // "module" (from .invowkmod)
)
```

### DiscoveredFile

Captures discovery metadata for each found file:

The current struct lives in `internal/discovery/discovery_files.go`. Fields agents commonly need are `Path`, `Source`, `Invowkfile`, `Error`, `Module`, `ParentModule`, `CommandNamespace`, and `IsGlobalModule`.

### CommandInfo

Output of command aggregation:

The current struct lives in `internal/discovery/discovery_commands.go`. It uses typed command names, filesystem paths, `SourceID`, optional `*invowkmod.ModuleID`, `Description`, ambiguity state, and `IsGlobalModule`.

---

## Command Aggregation & Collision Detection

The aggregation system uses a **two-phase process with transparent namespace**:

### Phase 1: Flatten & Index

```go
// Get all commands with proper namespacing
commands := invowkfile.FlattenCommands()

// Modules have commands prefixed:
// Module "foo" with command "build" → "foo build"
```

### Phase 2: Conflict Analysis

The `DiscoveredCommandSet` provides:

| Field | Purpose |
|-------|---------|
| `Commands` | All discovered commands |
| `BySimpleName` | Index: simple name → all commands with that name |
| `AmbiguousNames` | Set of names that exist in >1 source |
| `BySource` | Groups commands by source ID |
| `SourceOrder` | Pre-sorted: "invowkfile" first, then modules alphabetically |

### Precedence vs. Collision Handling

| Source Type | Behavior |
|-------------|----------|
| Non-module (current dir invowkfile) | First source **WINS** (shadows later) |
| Module (local, config includes, user-dir) | **ALL included** with ambiguity flagging |

**IsAmbiguous Flag:** Set to `true` when a simple name conflicts across sources. This enables:
- **Transparent namespace** for unambiguous commands: `invowk cmd build`
- **Explicit disambiguation** for ambiguous ones: `invowk cmd @foo build`

---

## Module Dependency Handling

### Module Identity & Visibility

Module metadata comes from `pkg/invowkmod.Module` and the parsed module's
metadata. Do not copy a hand-written struct shape into docs or tests; read the
current type before changing discovery code:

```bash
rg -n 'type .*Module|ModuleID|ModuleNamespace|CommandNamespace|SourceID' pkg/invowkmod internal/discovery
```

Module commands are automatically namespaced by the effective command namespace:
the module ID by default, an include alias when configured, or the locked
vendored command source ID when a parent lock records one.

### Module Collision Detection

When two modules have the same ID in different sources:

`ModuleCollisionError` is defined in `internal/discovery/discovery.go` and reports namespace, first source, second source, and whether the second source is local or vendored.

**Vendored annotation:** When a collision involves a vendored module, the error message includes `"(vendored in <parent>)"` for clearer diagnostics.

**Module Aliases:** Configured inline via `config.Includes` entries (each `IncludeEntry` can have an optional `Alias` field for module paths).

### Command Scope Rules

Commands can only call:

1. Commands from the **same module**
2. Commands from **globally installed user command modules** (`~/.invowk/cmds/`)
3. Commands from **first-level requirements** (direct dependencies in `invowkmod.cue:requires`) that also match lock-file identity/source metadata

**CRITICAL:** Transitive dependencies are **NOT accessible**. Commands cannot call dependencies of dependencies. This enforces explicit, auditable dependency chains.

Scope enforcement is in `internal/app/deps/`. A direct dependency is authorized
only when the declaration and `invowkmod.lock.cue` agree on both stable module
identity and effective command source ID. This prevents alias collisions from
accidentally granting command visibility.

---

## Validation

### Command Tree Validation

`ValidateCommandTree()` enforces the **leaf-only args** rule:

```go
// A command cannot have both positional args AND subcommands
// Positional args would be unreachable if subcommands exist

type ArgsSubcommandConflictError struct {
    CommandPath []string  // Path to the conflicting command
    ArgNames    []string  // The args that conflict
}
```

---

## Discovery Diagnostics

The discovery system returns structured diagnostics alongside results rather than writing to stderr. This follows the "return diagnostics, don't render them" pattern — domain packages produce structured data; the CLI layer decides how to render it.

### Diagnostic Type

```go
type Diagnostic struct {
    // Fields are unexported; use accessors.
}
```

Fields are immutable and unexported. Use `Severity()`, `Code()`, `Message()`,
`Path()`, and `Cause()` accessors rather than field access in tests or renderers.

### Diagnostic-Returning APIs

Internal methods return `(results, []Diagnostic, error)` tuples:

```go
// Internal: returns files + diagnostics
discoverAllWithDiagnostics() → ([]*DiscoveredFile, []Diagnostic, error)
loadAllWithDiagnostics()     → ([]*DiscoveredFile, []Diagnostic, error)

// Public wrappers drop diagnostics for backward compatibility
DiscoverAll() → ([]*DiscoveredFile, error)
LoadAll()     → ([]*DiscoveredFile, error)
```

### Result Types with Diagnostics

Public aggregation APIs return result types that bundle diagnostics:

```go
type CommandSetResult struct {
    Set         *DiscoveredCommandSet
    Diagnostics []Diagnostic
}

type LookupResult struct {
    Command     *CommandInfo
    Diagnostics []Diagnostic
}
```

### Diagnostic Codes

Use `internal/discovery/diagnostic.go` as the source of truth for diagnostic codes. Current families cover init/config/lookup failures, local/include/vendored module scan and load outcomes, vendored undeclared or transitive-dependency skips, container runtime initialization, shebang/interpreter overrides, local-over-global shadowing, and symlink skips.

---

## Usage Patterns

### Deterministic Tests

Prefer injected directories over process-global `os.Chdir`:

```go
d := discovery.New(cfg,
    discovery.WithBaseDir(types.FilesystemPath(workDir)),
    discovery.WithCommandsDir(types.FilesystemPath(userCmdsDir)),
)
```

Use `WithCommandsDir("")` when a test intentionally disables user command
discovery. This sets the explicit "provided" flag and avoids falling back to the
real `~/.invowk/cmds`.

### CLI Listing with Grouping

```go
disc := discovery.New(cfg)
result, err := disc.DiscoverCommandSet(ctx)  // Returns CommandSetResult with diagnostics

for _, sourceID := range result.Set.SourceOrder {
    for _, cmd := range result.Set.BySource[sourceID] {
        if cmd.IsAmbiguous {
            // Show with disambiguation prefix
        } else {
            // Show with transparent namespace
        }
    }
}
```

### Validation Before Execution

```go
result, err := disc.DiscoverAndValidateCommandSet(ctx)  // Includes tree validation + diagnostics
```

### Get Specific Command

```go
lookup, err := disc.GetCommand(ctx, "foo build")
// Returns LookupResult with Command, Invowkfile, Module metadata, and Diagnostics
```

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Combined file discovery + command aggregation | Tightly coupled; splitting would require large intermediate data structures |
| Non-module precedence shadows | User dir and config paths are fallback sources |
| Module commands all included | Modules in current dir are first-class and should all be visible |
| SimpleName-based collision detection | Enables transparent namespace while flagging attention for conflicts |
| Lazy parsing (LoadAll vs DiscoverAll) | Parsing deferred until needed; discovery is I/O only |
| Module aliases in config | Keeps discovery focused on filesystem; config handles naming |
| Graceful error handling | Invalid modules skipped; one bad module doesn't block others |
| Vendored modules use `SourceModule` | No new Source enum; vendored modules are modules, distinguished by `ParentModule != nil` |
| Single-level vendored scanning | No recursive nesting; vendored module's own `invowk_modules/` is ignored with diagnostic |
| `ParentModule` field for ownership | Minimal, backward-compatible; enables collision annotation and future scope enforcement |

---

## File Organization

| File | Purpose |
|------|---------|
| `discovery.go` | Core API: LoadAll, LoadFirst, CheckModuleCollisions, GetEffectiveCommandNamespace, loadAllWithDiagnostics |
| `discovery_files.go` | File/module discovery: DiscoverAll, discoverInDir, discoverModulesInDirWithDiagnostics, discoverVendoredModulesWithDiagnostics, appendModulesWithVendored, loadIncludesWithDiagnostics, Source enum, DiscoveredFile (with ParentModule) |
| `discovery_vendored_test.go` | Vendored module discovery tests: parent tracking, ordering, nested blocking, all 3 source types, collision annotation, reserved module skip, scan failure diagnostics |
| `discovery_commands.go` | Command aggregation: DiscoverCommandSet, DiscoverAndValidateCommandSet, GetCommand, DiscoveredCommandSet |
| `diagnostic.go` | Diagnostic type, Severity constants, CommandSetResult, LookupResult |
| `validation.go` | Command tree validation (leaf-only args rule) |
| `doc.go` | Package documentation |

---

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Expecting transitive deps | Command can't call dep-of-dep | Add explicit first-level requirement |
| Forgetting disambiguation | "ambiguous command" error | Use `@source` prefix or `--ivk-from` flag |
| Args + subcommands together | ArgsSubcommandConflictError | Make args-only or subcommands-only |
| Testing non-module shadowing | Later source visible | Only first non-module source wins |
| Module with reserved name "invowkfile" | Module silently skipped | Use different module name |
| Expecting nested vendored modules | Only first level of `invowk_modules/` scanned | Flatten deps into the parent module's vendor dir |
| Vendored module collision | `ModuleCollisionError` with "(vendored in parent)" | Add alias in includes config for one of the colliding modules |
