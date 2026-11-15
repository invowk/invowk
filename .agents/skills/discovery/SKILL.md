---
name: discovery
description: >-
  Module/command discovery, precedence order, collision detection, source
  tracking, vendored module (invowk_modules/) namespace policy, diagnostics,
  and command-scope discovery inputs. Use when working on internal/discovery/,
  discovery adapters, command dependency scope checks, or how
  invowkfiles/modules are found, namespaced, locked, and aggregated.
---

# Discovery Skill

This skill covers the discovery system in Invowk, which locates and aggregates
commands from multiple sources with clear precedence rules and collision
detection. Besides `internal/discovery/`, it applies to the production adapter
`internal/app/commandadapters/discovery.go` and the command-scope checks in
`internal/app/deps/` that consume discovered command identities.

- `internal/discovery/doc.go` is authoritative for scope and precedence; check
  it when this file and the code disagree.
- `discovery.go`, `discovery_files.go`, and `discovery_commands.go` are named by
  formal correspondence tables: renames or behavior changes there also follow
  `.agents/rules/formal-bindings.md`.
- Navigation tables (design decisions with rationale, file organization):
  `references/internals.md`.

---

## Discovery Precedence Order

Discovery follows a **6-level precedence order** (`doc.go`,
`discoverAllWithDiagnostics()` in `discovery_files.go`):

| Priority | Source | Description |
|----------|--------|-------------|
| 1 (Highest) | Current Directory | `invowkfile.cue` in the working directory |
| 2 | Local Modules | Sibling `*.invowkmod` directories in current directory and their vendored dependencies |
| 3 | Config Includes | Module paths from `config.Includes` and their vendored dependencies |
| 4 | Provisioned Modules | Module entries supplied by the command adapter for provisioned container images (`WithProvisionedModuleEntries`) and their vendored dependencies |
| 5 | Provisioned Global Modules | Global module entries supplied the same way (`WithProvisionedGlobalModuleEntries`), marked `IsGlobalModule`, and their vendored dependencies |
| 6 (Lowest) | User Commands | `~/.invowk/cmds/` — modules only, non-recursive, marked `IsGlobalModule`, and their vendored dependencies |

**Key Behavior:**
- The current-directory invowkfile is the only non-module source. Full command
  names are first-wins in precedence order (`DiscoveredCommandSet.ByName`), but
  a simple name defined by both the invowkfile and a module is flagged
  **ambiguous**, not shadowed.
- Module commands: **All included** with ambiguity flagging for transparent namespace.
- A local module with the same module ID as a global one wins by precedence and
  emits a `module_shadows_global` warning (`detectModuleShadowing`).
- User commands dir only discovers `*.invowkmod` immediate children (no loose invowkfiles, no recursion).
- When `baseDir` is empty (e.g. `os.Getwd()` failed because the working
  directory was deleted), current-directory and local-module discovery
  (levels 1-2) are skipped instead of resolving against the process cwd.
- `--ivk-no-cwd` / `INVOWK_NO_CWD` reuse that mechanism: the CLI resolves the
  switch (`resolveNoCwd`, explicit flag beats env) and attaches it with
  `commandadapters.ContextWithNoCwd`; `DiscoveryService.newDiscovery(ctx, cfg)`
  then appends `discovery.WithBaseDir("")` **last**, overriding an injected base
  dir, so levels 1-2 are skipped and `os.Getwd()` is never called. Levels 3-6
  stay. The per-request cache records the switch and is reused only by
  contexts with the same value. `internal/discovery` itself has no switch.
- Config never comes from the cwd: `config.SelectSource` picks `--ivk-config`
  > `INVOWK_CONFIG` (absolute) > user file > defaults; explicit sources fail
  closed (`config.ErrExplicitConfig`) before any `Discovery` is built.

---

## File Discovery Algorithm

Discovery has three tracks:

### Track A: Invowkfile Discovery

```go
// Single-level check (current directory only)
discoverInDir(dir, source)  // Looks for invowkfile.cue OR invowkfile
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
- Skips reserved module name `"invowkfile"` (reserved for canonical namespace) with a warning diagnostic
- Skips symlinked `.invowkmod` directories to prevent boundary escape (`module_symlink_skipped`; vendored: `vendored_symlink_skipped`)
- **Graceful degradation**: Invalid modules emit diagnostics (warnings) and are skipped

### Track C: Vendored Module Discovery

After discovering each module (from any module source), the system may scan its `invowk_modules/` directory for vendored dependencies:

```go
// Flat, one-level scan of <parentModule.Path>/invowk_modules/
discoverVendoredModulesWithDiagnostics(parentModule) → ([]*DiscoveredFile, []Diagnostic, error)
```

**Key behaviors:**
- Loads the parent module lock file and includes vendored modules only when they are declared and locked by the parent dependency graph.
- Verifies vendored module content hashes; hash mismatches are hard errors (discovery aborts), not warnings. On by default; the audit scan context disables it with `WithVendoredIntegrityVerification(false)` (`internal/audit/scan_context.go`).
- Skips vendored modules with missing transitive dependencies from the parent graph, and modules that match several declared lock entries for the same identity (`vendored_ambiguous_lock_skipped`).
- Sets `ParentModule` on each vendored `DiscoveredFile` for ownership tracking
- Vendored modules use `SourceModule` (no new Source enum value)
- **No recursion**: Only immediate `invowk_modules/` children are scanned; nested vendored modules are NOT recursed into (emits `vendored_nested_ignored` diagnostic)
- Vendored modules are always ordered after their parent in the files slice
- Vendored children inherit `IsGlobalModule` from their parent.

**DRY helper** used at all 5 module scan sites (local, includes, provisioned,
provisioned-global, user-dir):

```go
// Appends module files + scans vendored for each, consolidating diagnostics;
// a vendored integrity error aborts discovery.
appendModulesWithVendored(files, diagnostics, moduleFiles, moduleDiags) → (files, diagnostics, error)
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
// Per discovered file: flatten its commands, then publish module commands
// under the effective command namespace (module ID, include alias, or
// locked vendored source ID).
for _, cmd := range file.Invowkfile.FlattenCommands() {
    fullName := cmd.Name                       // root invowkfile: "build"
    if isModule {
        fullName, _ = buildModuleCommandName(sourceID, cmd.Name) // module "foo": "foo build"
    }
}
```

### Phase 2: Conflict Analysis

The `DiscoveredCommandSet` provides:

| Field | Purpose |
|-------|---------|
| `Commands` | All discovered commands |
| `ByName` | Index: full name → first command added with it (precedence order); used by `GetCommand` |
| `BySimpleName` | Index: simple name → all commands with that name |
| `AmbiguousNames` | Set of names that exist in >1 source |
| `BySource` | Groups commands by source ID |
| `SourceOrder` | Pre-sorted: "invowkfile" first, then modules alphabetically |

### Precedence vs. Collision Handling

| Situation | Behavior |
|-----------|----------|
| Same full name from several files | First in precedence order **WINS** in `ByName` |
| Same simple name across sources (invowkfile vs module, or module vs module) | **ALL included**; `Analyze()` marks every one `IsAmbiguous` |

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
2. Commands from **global modules** (`IsGlobalModule`): `~/.invowk/cmds/` modules, provisioned global entries, and their vendored children
3. Commands from **first-level requirements** (direct dependencies in `invowkmod.cue:requires`) that also match lock-file identity/source metadata

**CRITICAL:** Transitive dependencies are **NOT accessible**. Commands cannot call dependencies of dependencies. This enforces explicit, auditable dependency chains.

Scope enforcement is in `internal/app/deps/` (`CheckCommandDependenciesExist()`
in `deps.go`), applying the policy type `CommandScope.CanCallTarget`
(`pkg/invowkmod/command_scope.go`). A direct dependency is authorized
only when the declaration and `invowkmod.lock.cue` agree on both stable module
identity and effective command source ID. This prevents alias collisions from
accidentally granting command visibility. It is a static gate on
`depends_on.cmds`, not a runtime interceptor (AGENTS.md hard invariant).

---

## Validation

### Command Tree Validation

`ValidateCommandTree()` (`validation.go`, delegating to
`invowkfile.ValidateCommandTree`) enforces the **leaf-only args** rule:

```go
// A command cannot have both positional args AND subcommands
// Positional args would be unreachable if subcommands exist

// discovery.ArgsSubcommandConflictError is an alias of:
type ArgsSubcommandConflictError struct { // pkg/invowkfile/command_tree.go
    CommandName CommandName
    Args        []Argument
    Subcommands []CommandName
    FilePath    types.FilesystemPath
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

// Command is nil when the command was not found; Diagnostics then contains a
// command_not_found entry.
type LookupResult struct {
    Command     *CommandInfo
    Diagnostics []Diagnostic
}
```

`DiscoverModules()` returns `ModuleListResult{Modules, Diagnostics}` the same way.

### Diagnostic Codes

Use `internal/discovery/diagnostic.go` as the source of truth for diagnostic
codes. Current families cover init/config/lookup failures, local/include/vendored
module scan and load outcomes, reserved-name skips, vendored undeclared,
ambiguous-lock, or transitive-dependency skips, provisioned module manifest
errors, container runtime initialization, shebang/interpreter overrides,
local-over-global shadowing, and symlink skips (symlinked modules are skipped to
prevent boundary escape).

---

## Usage Patterns

Production CLI code reaches discovery through
`commandadapters.DiscoveryService` (config loading plus a per-request cache;
see the cli skill). The raw `discovery.Discovery` API below is what that
adapter, the audit scanner, and tests use.

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
// lookup.Command (*CommandInfo, nil if not found) carries the invowkfile and
// module metadata; lookup.Diagnostics includes command_not_found on a miss.
```

---

## Design Invariants

- File discovery and command aggregation stay in one package (`doc.go` explains
  why); separate concerns by file.
- Vendored modules use `SourceModule`; never add a new `Source` enum value for
  them (`ParentModule != nil` distinguishes them).
- Vendoring is single-level: never recurse into a vendored module's own
  `invowk_modules/`.
- Invalid modules degrade to warning diagnostics; only vendored integrity
  failures abort discovery.

Rationale for these and the other design decisions: `references/internals.md`.

---

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| Expecting transitive deps | Command can't call dep-of-dep | Add explicit first-level requirement |
| Forgetting disambiguation | "ambiguous command" error | Use `@source` prefix or `--ivk-from` flag |
| Args + subcommands together | ArgsSubcommandConflictError | Make args-only or subcommands-only |
| Expecting the root invowkfile to shadow a module command | Both appear, flagged ambiguous | Only full-name lookups are first-wins; same simple names need disambiguation |
| Module with reserved name "invowkfile" | Module skipped with a `reserved_module_name_skipped` warning (`include_reserved_module_skipped` / `vendored_reserved_module_skipped` for includes/vendored) | Use different module name |
| Expecting nested vendored modules | Only first level of `invowk_modules/` scanned | Flatten deps into the parent module's vendor dir |
| Vendored module collision | `ModuleCollisionError` with "(vendored in parent)" | Add alias in includes config for one of the colliding modules |
