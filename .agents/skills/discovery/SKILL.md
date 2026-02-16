---
name: discovery
description: Module/command discovery, precedence order, collision detection, source tracking. Use when working on internal/discovery/ files or modifying how invowkfiles and modules are found and aggregated.
disable-model-invocation: false
---

# Discovery Skill

This skill covers the discovery system in Invowk, which locates and aggregates commands from multiple sources with clear precedence rules and collision detection.

Use this skill when working on:
- `internal/discovery/` - Discovery and aggregation logic
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

After discovering each module (from any of the 3 module sources), the system scans its `invowk_modules/` directory for vendored dependencies:

```go
// Flat, one-level scan of <parentModule.Path>/invowk_modules/
discoverVendoredModulesWithDiagnostics(parentModule) → ([]*DiscoveredFile, []Diagnostic)
```

**Key behaviors:**
- Sets `ParentModule` on each vendored `DiscoveredFile` for ownership tracking
- Vendored modules use `SourceModule` (no new Source enum value)
- **No recursion**: Only immediate `invowk_modules/` children are scanned; nested vendored modules are NOT recursed into (emits `vendored_nested_ignored` diagnostic)
- **Graceful degradation**: Invalid vendored modules emit `vendored_module_load_skipped` diagnostic and are skipped
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

```go
type DiscoveredFile struct {
    Path         string           // Absolute path
    Source       Source           // Which source type
    Invowkfile   *invowkfile.Invowkfile  // Parsed content (lazy-loaded)
    Error        error            // Parse errors if applicable
    Module       *invowkmod.Module  // Non-nil if from .invowkmod
    ParentModule *invowkmod.Module  // Non-nil if vendored (tracks ownership)
}
```

### CommandInfo

Output of command aggregation:

```go
type CommandInfo struct {
    Name        string  // Full name with prefix (e.g., "foo build")
    SimpleName  string  // Unprefixed name (e.g., "build")
    Source      Source
    SourceID    string  // "invowkfile" or module short name
    ModuleID    string  // Full module ID (e.g., "io.invowk.sample")
    IsAmbiguous bool    // True if SimpleName conflicts across sources
    FilePath    string  // Absolute path to invowkfile
    Command     *invowkfile.Command
    Invowkfile    *invowkfile.Invowkfile
}
```

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

```go
type Module struct {
    Path    string          // Filesystem location
    Invowkmod *invowkmod.Invowkmod  // Parsed metadata from invowkmod.cue
}

// Module commands are automatically namespaced
// Module "foo" → commands like "foo build", "foo deploy"
```

### Module Collision Detection

When two modules have the same ID in different sources:

```go
type ModuleCollisionError struct {
    ModuleID string
    Sources  []string
}

// Validation (wired into LoadAll automatically)
err := discovery.CheckModuleCollisions()
// Returns actionable guidance: add alias in includes config
```

**Vendored annotation:** When a collision involves a vendored module, the error message includes `"(vendored in <parent>)"` for clearer diagnostics.

**Module Aliases:** Configured inline via `config.Includes` entries (each `IncludeEntry` can have an optional `Alias` field for module paths).

### Command Scope Rules

Commands can only call:

1. Commands from the **same module**
2. Commands from **globally installed modules** (`~/.invowk/modules/`)
3. Commands from **first-level requirements** (direct dependencies in `invowkmod.cue:requires`)

**CRITICAL:** Transitive dependencies are **NOT accessible**. Commands cannot call dependencies of dependencies. This enforces explicit, auditable dependency chains.

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
    Severity Severity  // "warning" or "error"
    Code     string    // Machine-readable code (e.g., "module_load_skipped")
    Message  string    // Human-readable description
    Path     string    // Associated file path (optional)
    Cause    error     // Underlying error (optional)
}
```

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

| Code | Severity | Meaning |
|------|----------|---------|
| `module_scan_path_invalid` | warning | Failed to resolve module scan directory |
| `module_scan_failed` | warning | Failed to list directory during module scan |
| `reserved_module_name_skipped` | warning | Module uses reserved name `"invowkfile"` |
| `module_load_skipped` | warning | Invalid module skipped during discovery |
| `include_not_module` | warning | Config include path is not a valid module |
| `include_reserved_module_skipped` | warning | Config include uses reserved module name |
| `include_module_load_failed` | warning | Failed to load configured include module |
| `vendored_scan_failed` | warning | Failed to read vendored modules directory |
| `vendored_reserved_module_skipped` | warning | Vendored module uses reserved name `"invowkfile"` |
| `vendored_module_load_skipped` | warning | Invalid vendored module skipped during discovery |
| `vendored_nested_ignored` | warning | Vendored module has its own `invowk_modules/` (not recursed) |
| `container_runtime_init_failed` | warning | Container engine unavailable during registry init |

---

## Usage Patterns

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
| `discovery.go` | Core API: LoadAll, LoadFirst, CheckModuleCollisions, GetEffectiveModuleID, loadAllWithDiagnostics |
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
