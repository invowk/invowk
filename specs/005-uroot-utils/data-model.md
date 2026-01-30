# Data Model: u-root Utils Integration

**Feature Branch**: `005-uroot-utils`
**Date**: 2026-01-30

## Core Entities

### 1. UrootCommand Interface

A wrapper interface for u-root commands that provides consistent handling.

```go
// UrootCommand wraps a u-root core.Command with additional context.
type UrootCommand interface {
    // Name returns the command name (e.g., "cp", "ls").
    Name() string

    // Run executes the command with the given context and arguments.
    // stdin/stdout/stderr and working directory are extracted from ctx.
    Run(ctx context.Context, args []string) error

    // SupportedFlags returns the flags this implementation supports.
    // Used for documentation; unsupported flags are silently ignored.
    SupportedFlags() []string
}
```

**Relationships**:
- Implemented by wrappers around `github.com/u-root/u-root/pkg/core/*` packages
- Implemented by custom utility implementations (head, tail, etc.)
- Used by `Registry` for command dispatch

### 2. Registry

Maps command names to their implementations.

```go
// Registry holds the mapping of command names to their implementations.
type Registry struct {
    commands map[string]UrootCommand
}

// Fields:
// - commands: Map from command name (string) to UrootCommand implementation
```

**Operations**:
- `Register(cmd UrootCommand)` - Adds a command to the registry
- `Lookup(name string) (UrootCommand, bool)` - Retrieves a command by name
- `Names() []string` - Returns list of registered command names

**Validation**:
- Command names must be non-empty
- No duplicate registrations allowed

### 3. HandlerContext

Extracted from mvdan/sh's context for u-root command execution.

```go
// HandlerContext provides execution context for u-root commands.
type HandlerContext struct {
    Stdin      io.Reader
    Stdout     io.Writer
    Stderr     io.Writer
    Dir        string
    LookupEnv  func(string) (string, bool)
}
```

**Extraction**:
```go
func ExtractHandlerContext(ctx context.Context) *HandlerContext {
    hc := interp.HandlerCtx(ctx)
    return &HandlerContext{
        Stdin:     hc.Stdin,
        Stdout:    hc.Stdout,
        Stderr:    hc.Stderr,
        Dir:       hc.Dir,
        LookupEnv: hc.Env.Get,
    }
}
```

## Command Implementations

### Category 1: pkg/core Wrappers

These wrap existing u-root library implementations.

| Command | Package | Key Config Fields |
|---------|---------|-------------------|
| `cat` | `pkg/core/cat` | numberLines, showEnds |
| `cp` | `pkg/core/cp` | recursive, force, noFollowSymlinks |
| `ls` | `pkg/core/ls` | longFormat, all, recursive, humanReadable |
| `mkdir` | `pkg/core/mkdir` | parents, mode |
| `mv` | `pkg/core/mv` | force, noClobber |
| `rm` | `pkg/core/rm` | recursive, force |
| `touch` | `pkg/core/touch` | noCreate, reference, time |

**Wrapper Pattern**:
```go
type catWrapper struct {
    name string
}

func (w *catWrapper) Name() string { return w.name }

func (w *catWrapper) Run(ctx context.Context, args []string) error {
    hc := ExtractHandlerContext(ctx)

    cmd := cat.New()
    cmd.SetIO(hc.Stdin, hc.Stdout, hc.Stderr)
    cmd.SetWorkingDir(hc.Dir)
    cmd.SetLookupEnv(hc.LookupEnv)

    if err := cmd.RunContext(ctx, args...); err != nil {
        return fmt.Errorf("[uroot] cat: %w", err)
    }
    return nil
}
```

### Category 2: Custom Implementations

These implement utilities not available in pkg/core.

| Command | Description | Key Logic |
|---------|-------------|-----------|
| `head` | Output first N lines | Line counter with streaming |
| `tail` | Output last N lines | Ring buffer for N lines |
| `wc` | Word/line/byte count | Streaming counters |
| `grep` | Pattern matching | Line-by-line regex match |
| `sort` | Sort lines | May use temp files for large inputs |
| `uniq` | Unique lines | Adjacent line comparison |
| `cut` | Select fields/chars | Field delimiter parsing |
| `tr` | Translate chars | Character mapping |

**Custom Implementation Pattern**:
```go
type headCommand struct {
    name string
}

func (c *headCommand) Name() string { return c.name }

func (c *headCommand) Run(ctx context.Context, args []string) error {
    hc := ExtractHandlerContext(ctx)

    // Parse flags
    fs := flag.NewFlagSet("head", flag.ContinueOnError)
    fs.SetOutput(io.Discard) // Silence unknown flag errors
    lines := fs.Int("n", 10, "number of lines")
    _ = fs.Parse(args) // Ignore errors (unsupported flags)

    // Get files from remaining args
    files := fs.Args()
    if len(files) == 0 {
        files = []string{"-"} // stdin
    }

    // Process each file with streaming I/O
    for _, file := range files {
        if err := c.processFile(ctx, hc, file, *lines); err != nil {
            return fmt.Errorf("[uroot] head: %w", err)
        }
    }
    return nil
}
```

## State Transitions

### Command Execution Flow

```
args received
    │
    ▼
┌─────────────────┐
│ Parse Flags     │
│ (flag.FlagSet)  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     unknown flags
│ Validate Args   │───────────────────▶ (silently ignored)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Extract Context │
│ (HandlerCtx)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Execute Command │
│ (streaming I/O) │
└────────┬────────┘
         │
         ├──────────▶ success: return nil
         │
         ▼
┌─────────────────┐
│ Wrap Error      │
│ [uroot] prefix  │
└────────┬────────┘
         │
         ▼
    return error
```

### Registry Initialization

```
Package init
    │
    ▼
┌─────────────────┐
│ Create Registry │
│ (empty map)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Register pkgcore│
│ wrappers        │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Register custom │
│ implementations │
└────────┬────────┘
         │
         ▼
   Registry ready
```

## Validation Rules

### Flag Validation
- Unknown flags: Silently ignored per spec
- Invalid flag values: Return error with `[uroot]` prefix
- Conflicting flags: Implementation-specific (generally last wins)

### Argument Validation
- Missing required args: Return error (e.g., `cp` with no source)
- Invalid paths: Return error (e.g., path traversal outside allowed directory)
- Too many args: Implementation-specific handling

### File Operation Validation
- Non-existent source: Return error
- Permission denied: Return error
- Destination exists: Depends on `-f`/`-n` flags

## Error Types

All errors follow the format: `[uroot] <cmd>: <message>`

| Error Scenario | Example Message |
|----------------|-----------------|
| File not found | `[uroot] cat: /path/to/file: no such file or directory` |
| Permission denied | `[uroot] rm: /protected: permission denied` |
| Is a directory | `[uroot] cat: /dir: is a directory` |
| Invalid option | `[uroot] head: invalid line count: abc` |
| Read error | `[uroot] cp: error reading source: <detail>` |
| Write error | `[uroot] cp: error writing destination: <detail>` |

## Relationships Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                     VirtualRuntime                               │
│                                                                  │
│  execHandler() ─────▶ tryUrootBuiltin() ─────▶ registry.Lookup()│
│                                                      │           │
└──────────────────────────────────────────────────────┼───────────┘
                                                       │
                                                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Registry                                  │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    commands map                          │    │
│  │  "cat"   ──▶ catWrapper (uses pkg/core/cat)             │    │
│  │  "cp"    ──▶ cpWrapper (uses pkg/core/cp)               │    │
│  │  "ls"    ──▶ lsWrapper (uses pkg/core/ls)               │    │
│  │  "mkdir" ──▶ mkdirWrapper (uses pkg/core/mkdir)         │    │
│  │  "mv"    ──▶ mvWrapper (uses pkg/core/mv)               │    │
│  │  "rm"    ──▶ rmWrapper (uses pkg/core/rm)               │    │
│  │  "touch" ──▶ touchWrapper (uses pkg/core/touch)         │    │
│  │  "head"  ──▶ headCommand (custom)                       │    │
│  │  "tail"  ──▶ tailCommand (custom)                       │    │
│  │  "wc"    ──▶ wcCommand (custom)                         │    │
│  │  "grep"  ──▶ grepCommand (custom)                       │    │
│  │  "sort"  ──▶ sortCommand (custom)                       │    │
│  │  "uniq"  ──▶ uniqCommand (custom)                       │    │
│  │  "cut"   ──▶ cutCommand (custom)                        │    │
│  │  "tr"    ──▶ trCommand (custom)                         │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
                                │
                                │ each implements
                                ▼
                    ┌─────────────────────┐
                    │   UrootCommand      │
                    │   interface         │
                    │   - Name()          │
                    │   - Run(ctx, args)  │
                    │   - SupportedFlags()│
                    └─────────────────────┘
```

## Cross-Platform Considerations

### Path Handling
- Use `filepath.Join()` for path construction
- Use `filepath.Clean()` for path normalization
- Accept forward slashes on Windows (convert internally)

### Line Endings
- `head`, `tail`, `wc`: Handle both `\n` and `\r\n`
- `grep`: Pattern matching works on logical lines

### Permissions
- `chmod`: Limited on Windows (basic read/write/execute)
- `rm`, `mkdir`: Windows-compatible permission handling
