# Research: Module-Aware Command Discovery

**Date**: 2026-01-21
**Feature**: 001-module-cmd-discovery

## Research Questions

### RQ-1: How does the current discovery system work?

**Decision**: Extend existing `Discovery.DiscoverCommands()` with source aggregation

**Rationale**: The current system already supports:
- Multi-location discovery (current dir, user dir, config paths)
- Module discovery via `discoverModulesInDir()`
- `DiscoveredFile` struct with `Source` enum and `Module` pointer
- Command flattening via `Invowkfile.FlattenCommands()`

The architecture is well-suited for extension. Key insight: modules are already discovered but their commands are namespaced with full module ID (e.g., `io.invowk.sample hello`). The feature requires:
1. Changing module command naming to use short names by default
2. Adding conflict detection when short names collide
3. Tracking source for disambiguation

**Alternatives Considered**:
- New parallel discovery system: Rejected (duplication, maintenance burden)
- Plugin-based discovery: Rejected (over-engineering for the scope)

### RQ-2: Where should CommandSource tracking be added?

**Decision**: Extend `CommandInfo` struct in `internal/discovery/discovery.go`

**Rationale**: `CommandInfo` already has:
```go
type CommandInfo struct {
    Name        string
    Description string
    Source      Source              // Already tracks SourceCurrentDir, SourceModule, etc.
    FilePath    string
    Command     *invowkfile.Command
    Invowkfile    *invowkfile.Invowkfile
}
```

The `Source` enum distinguishes location types but not specific modules. Add:
```go
type CommandInfo struct {
    // ... existing fields ...
    SourceID    string  // NEW: "invowkfile" or module short name (e.g., "foo")
    ModuleID    string  // NEW: Full module ID if from module (e.g., "io.invowk.sample")
}
```

**Alternatives Considered**:
- New `CommandSource` struct: Possible but adds indirection; fields on CommandInfo are simpler
- Map-based tracking: Rejected (complicates iteration)

### RQ-3: How should ambiguity detection work?

**Decision**: Build conflict map during command aggregation in `DiscoverCommands()`

**Rationale**: After collecting all commands:
1. Group commands by their simple name (without module prefix)
2. If a name maps to >1 command, mark all as ambiguous
3. Store ambiguous names in a set for quick lookup during execution

```go
type DiscoveredCommandSet struct {
    Commands      []*CommandInfo
    BySimpleName  map[string][]*CommandInfo  // For conflict detection
    AmbiguousNames map[string]bool           // Names with conflicts
}
```

**Alternatives Considered**:
- Detect at execution time only: Rejected (poor UX - user doesn't see conflicts in listing)
- Fail-fast on first conflict: Rejected (user should see all available options)

### RQ-4: How should `@source` prefix parsing work?

**Decision**: Parse in `cmd/invowk/cmd.go` before Cobra command matching

**Rationale**: The `@source` prefix must be detected before Cobra tries to match it as a command name. In `registerDiscoveredCommands()` or a pre-run hook:

```go
// Example: invowk cmd @foo deploy arg1 arg2
// args[0] = "@foo"  -> sourceFilter = "foo"
// args[1:] = ["deploy", "arg1", "arg2"] -> passed to Cobra
```

Parse logic:
1. Check if first arg starts with `@`
2. Extract source name (strip `@` and optional `.invowkmod`/`.cue` suffix)
3. Validate source exists
4. Filter commands to that source before execution

**Alternatives Considered**:
- Cobra custom argument parser: Complex, fights Cobra's design
- Environment variable: Poor UX, not inline with command

### RQ-5: How should `--from` flag work?

**Decision**: Add as persistent flag on `cmdCmd` (the `invowk cmd` command)

**Rationale**: Per spec clarification, `--from` must appear immediately after `invowk cmd`:
```
invowk cmd --from foo deploy arg1
```

This is a standard Cobra persistent flag pattern:
```go
cmdCmd.PersistentFlags().StringVar(&fromSource, "from", "", "Source to run command from")
```

The flag is parsed before subcommand matching, making it available in pre-run hooks.

**Alternatives Considered**:
- Global flag on root: Rejected (only meaningful for `cmd` subcommand)
- Per-command flag: Rejected (would need to be on every generated command)

### RQ-6: How should the grouped listing be rendered?

**Decision**: Extend `listCommands()` in `cmd/invowk/cmd.go`

**Rationale**: Current listing already groups by `Source` enum. Extend to:
1. Group by `SourceID` (specific module or "invowkfile")
2. Render section headers with source name
3. Show ambiguity annotation when conflicts exist

```
Available Commands
  (* = default runtime)

From invowkfile:
  hello          - Greet the world [native*] (linux, macos, windows)
  deploy         - Deploy application (@invowkfile) [container] (linux)

From foo.invowkmod:
  build          - Build project [native*, virtual]
  deploy         - Deploy to staging (@foo) [native*]
```

Note: `(@invowkfile)` and `(@foo)` only shown when `deploy` is ambiguous.

**Alternatives Considered**:
- Flat list with prefixes: Rejected (less scannable)
- Tree view: Rejected (over-engineering)

### RQ-7: How should reserved name validation work?

**Decision**: Add check in `discoverModulesInDir()` and `invowkmod.Validate()`

**Rationale**: Per FR-015, `invowkfile.invowkmod` is reserved. Check at two points:
1. During discovery: Skip with warning
2. During `invowk module validate`: Report as validation error

```go
// In discoverModulesInDir()
if strings.TrimSuffix(filepath.Base(path), ".invowkmod") == "invowkfile" {
    log.Warn("Skipping reserved module name: invowkfile.invowkmod")
    continue
}
```

**Alternatives Considered**:
- Only check at validate time: Rejected (confusing runtime behavior)
- Allow with full-name requirement: Rejected (spec decision was to reject)

## Codebase Patterns to Follow

### Discovery Pattern
```go
// From internal/discovery/discovery.go
func (d *Discovery) DiscoverCommands() ([]*CommandInfo, error) {
    files, err := d.LoadAll()
    // ... flatten commands from each file ...
    // ... sort by name ...
    return commands, nil
}
```

### Error Handling Pattern
```go
// Named returns for resource cleanup
func (d *Discovery) LoadAll() (files []DiscoveredFile, err error) {
    // ... operations ...
    return files, nil
}
```

### Testing Pattern
```go
// Table-driven tests in *_test.go
func TestAmbiguityDetection(t *testing.T) {
    tests := []struct {
        name     string
        commands []*CommandInfo
        want     map[string]bool
    }{
        // ... test cases ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

### CLI Test Pattern (testscript)
```txtar
# Test: Multi-source discovery
exec invowk cmd
stdout 'From invowkfile:'
stdout 'From foo.invowkmod:'

# Test: Disambiguation
exec invowk cmd @foo deploy
! stderr .
```

## Key Files Reference

| File | Role | Modification |
|------|------|--------------|
| `internal/discovery/discovery.go` | Discovery orchestration | Add SourceID/ModuleID to CommandInfo, conflict detection |
| `internal/discovery/validation.go` | Command tree validation | Add ambiguity validation |
| `cmd/invowk/cmd.go` | CLI command handling | `@source` parsing, `--from` flag, grouped listing |
| `pkg/invowkmod/operations.go` | Module validation | Reserved name check |
| `tests/cli/testdata/*.txtar` | CLI tests | New test files |

## Open Questions

None - all questions resolved through research.
