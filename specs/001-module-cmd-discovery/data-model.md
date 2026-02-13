# Data Model: Module-Aware Command Discovery

**Date**: 2026-01-21
**Feature**: 001-module-cmd-discovery

## Entities

### CommandInfo (Extended)

**Location**: `internal/discovery/discovery.go`

**Current Fields** (unchanged):
```go
type CommandInfo struct {
    Name        string              // Fully qualified name
    Description string              // Command description
    Source      Source              // SourceCurrentDir, SourceModule
    FilePath    string              // Path to invowkfile
    Command     *invowkfile.Command   // Command definition
    Invowkfile    *invowkfile.Invowkfile  // Parent invowkfile
}
```

**New Fields**:
```go
type CommandInfo struct {
    // ... existing fields ...

    SimpleName  string  // Command name without module prefix (e.g., "deploy")
    SourceID    string  // "invowkfile" or module short name (e.g., "foo")
    ModuleID    string  // Full module ID if from module (e.g., "io.invowk.sample"), empty for invowkfile
    IsAmbiguous bool    // True if SimpleName conflicts with another command
}
```

**Validation Rules**:
- `SimpleName`: Must be non-empty, derived from command's base name
- `SourceID`: Must be "invowkfile" or valid module short name (no `.invowkmod` suffix)
- `ModuleID`: Valid module ID format or empty string
- `IsAmbiguous`: Set during conflict detection phase

### DiscoveredCommandSet (New)

**Location**: `internal/discovery/discovery.go`

```go
// DiscoveredCommandSet holds aggregated discovery results with conflict analysis
type DiscoveredCommandSet struct {
    // All discovered commands
    Commands []*CommandInfo

    // Commands indexed by simple name for conflict detection
    // Key: simple command name (e.g., "deploy")
    // Value: all commands with that name from different sources
    BySimpleName map[string][]*CommandInfo

    // Set of simple names that have conflicts (>1 source)
    AmbiguousNames map[string]bool

    // Commands indexed by source for grouped listing
    // Key: SourceID (e.g., "invowkfile", "foo")
    BySource map[string][]*CommandInfo

    // Ordered list of sources for consistent display
    // ["invowkfile", "alpha", "beta", ...] - invowkfile first, then modules alphabetically
    SourceOrder []string
}
```

**Validation Rules**:
- `BySimpleName`: Built during aggregation, all keys lowercase
- `AmbiguousNames`: Populated when `len(BySimpleName[name]) > 1`
- `SourceOrder`: "invowkfile" always first if present, modules sorted alphabetically

**State Transitions**:
1. **Empty** → Created with `NewDiscoveredCommandSet()`
2. **Aggregating** → Commands added via `Add(cmd *CommandInfo)`
3. **Analyzed** → Conflicts detected via `Analyze()`
4. **Finalized** → Ready for listing/execution

### SourceFilter (New)

**Location**: `cmd/invowk/cmd.go`

```go
// SourceFilter represents a user-specified source constraint
type SourceFilter struct {
    SourceID string  // Normalized source name (e.g., "foo" not "foo.invowkmod")
    Raw      string  // Original input (e.g., "@foo.invowkmod" or "--from foo")
}

// ParseSourceFilter extracts source from @prefix or --from flag
func ParseSourceFilter(args []string, fromFlag string) (*SourceFilter, []string, error)
```

**Validation Rules**:
- `SourceID`: Must match existing source (invowkfile or module)
- Accepts: "foo", "foo.invowkmod", "invowkfile", "invowkfile.cue"
- Normalizes to: "foo" or "invowkfile"

## Relationships

```
┌─────────────────────┐
│DiscoveredCommandSet │
└─────────────────────┘
         │
         │ contains
         ▼
┌─────────────────────┐
│   CommandInfo[]     │
└─────────────────────┘
         │
         │ references
         ▼
┌─────────────────────┐      ┌─────────────────────┐
│  invowkfile.Command   │◄─────│  invowkfile.Invowkfile  │
└─────────────────────┘      └─────────────────────┘
                                      │
                                      │ may have
                                      ▼
                             ┌─────────────────────┐
                             │  invowkmod.Invowkmod    │
                             │  (Metadata)         │
                             └─────────────────────┘
```

## Indexes

### By Simple Name
```go
BySimpleName["deploy"] = []*CommandInfo{
    {SourceID: "invowkfile", SimpleName: "deploy", ...},
    {SourceID: "foo", SimpleName: "deploy", ...},
}
```

### By Source (for grouped listing)
```go
BySource["invowkfile"] = []*CommandInfo{
    {SimpleName: "hello", ...},
    {SimpleName: "deploy", ...},
}
BySource["foo"] = []*CommandInfo{
    {SimpleName: "build", ...},
    {SimpleName: "deploy", ...},
}
```

## Canonical Namespace Format

**Internal representation** (FR-004):
```
<module-id | "invowkfile">:<invowkfile-path>:<cmd-name>
```

Examples:
- `invowkfile:/path/to/invowkfile.cue:deploy`
- `io.invowk.sample:/path/to/foo.invowkmod/invowkfile.cue:deploy`

**User-facing disambiguation**:
- `@foo deploy` or `@foo.invowkmod deploy`
- `@invowkfile deploy` or `@invowkfile.cue deploy`
- `--from foo deploy` or `--from invowkfile deploy`

## Reserved Names

| Name | Reserved For | Validation |
|------|--------------|------------|
| `invowkfile` | Root invowkfile source | FR-015: Reject `invowkfile.invowkmod` |
| `invowkfile.cue` | Root invowkfile source | Alias for `invowkfile` |

## Migration Notes

No data migration required. Changes are additive:
- New fields on `CommandInfo` default to empty/false for backward compatibility
- Existing discovery code continues to work; new features opt-in
- CLI output changes only when multiple sources present
