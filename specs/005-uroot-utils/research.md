# Research: u-root Utils Integration

**Feature Branch**: `005-uroot-utils`
**Date**: 2026-01-30

## u-root Library Architecture

### Decision: Use `github.com/u-root/u-root/pkg/core` Package

**Rationale**: The u-root project provides both CLI executables (`cmds/core/*`) and importable library packages (`pkg/core/*`). The library packages expose a clean `core.Command` interface that integrates well with mvdan/sh's exec handler mechanism.

**Alternatives Considered**:
1. **Implement utilities from scratch** - Rejected; reinventing well-tested POSIX utilities is error-prone and time-consuming.
2. **Shell out to u-root binaries** - Rejected; defeats the purpose of having a self-contained virtual runtime.
3. **Use a different utility library (busybox-go, etc.)** - Rejected; u-root has the cleanest Go interface and is actively maintained by Google.

### The core.Command Interface

```go
type Command interface {
    SetIO(stdin io.Reader, stdout io.Writer, stderr io.Writer)
    SetWorkingDir(workingDir string)
    SetLookupEnv(lookupEnv LookupEnvFunc)
    Run(args ...string) error
    RunContext(ctx context.Context, args ...string) error
}
```

**Integration Pattern**:
1. In `execHandler`, intercept command names
2. Create command instance via `cmdpkg.New()`
3. Call `SetIO()`, `SetWorkingDir()`, `SetLookupEnv()` with shell context
4. Call `RunContext(ctx, args[1:]...)`
5. Map error to appropriate exit status

### Available Utilities in pkg/core

The following 15 utilities are available in `github.com/u-root/u-root/pkg/core`:

| Package | Utility | POSIX Flags |
|---------|---------|-------------|
| `base64` | Base64 encoding/decoding | `-d`, `-w` |
| `cat` | File concatenation | `-n`, `-b`, `-s` |
| `chmod` | Permission modification | `-R` |
| `cp` | File copying | `-r`, `-R`, `-f`, `-n`, `-P` |
| `find` | File searching | `-name`, `-type`, `-exec` |
| `gzip` | Compression | `-d`, `-c`, `-f` |
| `ls` | Directory listing | `-l`, `-a`, `-R`, `-h` |
| `mkdir` | Directory creation | `-p`, `-m` |
| `mktemp` | Temp file creation | `-d`, `-p` |
| `mv` | File moving | `-f`, `-n` |
| `rm` | File removal | `-r`, `-R`, `-f` |
| `shasum` | Checksum computation | Algorithm variants |
| `tar` | Archive handling | `-c`, `-x`, `-f`, `-v` |
| `touch` | Timestamp modification | `-c`, `-m`, `-a` |
| `xargs` | Argument processing | `-n`, `-I` |

### Missing Utilities

The following utilities from FR-001 are **NOT available** in `pkg/core`:

| Utility | Status | Solution |
|---------|--------|----------|
| `echo` | Built into mvdan/sh | No implementation needed |
| `pwd` | Built into mvdan/sh | No implementation needed |
| `head` | Not in pkg/core | Custom implementation required |
| `tail` | Not in pkg/core | Custom implementation required |
| `wc` | Not in pkg/core | Custom implementation required |
| `grep` | Not in pkg/core | Custom implementation required |
| `sort` | Not in pkg/core | Custom implementation required |
| `uniq` | Not in pkg/core | Custom implementation required |
| `cut` | Not in pkg/core | Custom implementation required |
| `tr` | Not in pkg/core | Custom implementation required |

### Decision: Implement Missing Utilities Natively

**Rationale**: The 8 missing utilities (`head`, `tail`, `wc`, `grep`, `sort`, `uniq`, `cut`, `tr`) require custom Go implementations following the same `core.Command` interface pattern. This is acceptable because:
1. These utilities have well-defined POSIX behavior
2. Streaming I/O implementations are straightforward
3. Keeps the architecture consistent

**Alternative Considered**:
- **Reduce scope to available pkg/core utilities only** - Rejected; the missing utilities are commonly used in shell scripts.

## mvdan/sh Integration Point

### Current Handler Structure

```go
// internal/runtime/virtual.go, lines 323-354
func (r *VirtualRuntime) execHandler(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
    return func(ctx context.Context, args []string) error {
        if r.EnableUrootUtils {
            if handled, err := r.tryUrootBuiltin(ctx, args); handled {
                return err
            }
        }
        return next(ctx, args)
    }
}
```

### Handler Context Access

The exec handler receives a `context.Context` that contains the shell's execution state. To access stdin/stdout/stderr and environment, we use `interp.HandlerCtx(ctx)`:

```go
hc := interp.HandlerCtx(ctx)
// hc.Stdin  - io.Reader
// hc.Stdout - io.Writer
// hc.Stderr - io.Writer
// hc.Dir    - working directory
// hc.Env    - expand.Environ interface
```

### Decision: Create Command Registry

**Rationale**: A registry pattern centralizes command lookup and enables lazy initialization.

```go
type UrootHandler func() core.Command

var urootRegistry = map[string]UrootHandler{
    "cat":   cat.New,
    "cp":    cp.New,
    "ls":    ls.New,
    // ... etc
}
```

## Error Handling

### Decision: Error Format with [uroot] Prefix

Per spec requirement FR-006 and `.claude/rules/uroot-utils.md`:

```go
return fmt.Errorf("[uroot] %s: %w", cmdName, err)
```

### Exit Status Mapping

The u-root commands return `error` not exit codes. Mapping:
- `nil` → exit status 0
- `non-nil error` → exit status 1

To propagate as shell exit status:

```go
if err != nil {
    return interp.NewExitStatus(1)
}
return nil
```

## Streaming I/O Requirement

Per `.claude/rules/uroot-utils.md` and spec edge cases:

**All file operations MUST use streaming I/O. Never buffer entire file contents.**

The u-root `pkg/cp` already uses `io.Copy()` internally. Custom utilities (head, tail, etc.) must follow the same pattern:

```go
// CORRECT: Streaming
_, err = io.Copy(dst, src)

// WRONG: Buffering
data, _ := io.ReadAll(src)
dst.Write(data)
```

## Symlink Handling

Per spec clarification:

**Default: Follow symlinks (copy target content, not the link)**

The u-root `cp` package provides:
```go
var Default = Options{}           // Follows symlinks
var NoFollowSymlinks = Options{NoFollowSymlinks: true}
```

We use `Default` unless user provides `-P` flag.

## Unsupported Flag Handling

Per spec clarification:

**Silently ignore unsupported flags.**

Implementation pattern:
```go
// Parse known flags
fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
fs.Bool("v", false, "verbose")  // supported
// Unknown flags (like --color) are collected in fs.Args()

// Execute with supported flags only
```

## Architecture Decision: Package Organization

### Decision: Internal Package `internal/uroot`

Create a new package for u-root integration:

```
internal/uroot/
├── doc.go           # Package documentation
├── registry.go      # Command registry
├── handler.go       # core.Command wrapper for custom implementations
├── cat.go           # Wrapper for pkg/core/cat
├── cp.go            # Wrapper for pkg/core/cp
├── ls.go            # Wrapper for pkg/core/ls
├── mkdir.go         # Wrapper for pkg/core/mkdir
├── mv.go            # Wrapper for pkg/core/mv
├── rm.go            # Wrapper for pkg/core/rm
├── touch.go         # Wrapper for pkg/core/touch
├── head.go          # Custom implementation
├── tail.go          # Custom implementation
├── wc.go            # Custom implementation
├── grep.go          # Custom implementation
├── sort.go          # Custom implementation
├── uniq.go          # Custom implementation
├── cut.go           # Custom implementation
├── tr.go            # Custom implementation
└── *_test.go        # Tests for each utility
```

**Rationale**:
1. Isolates u-root integration complexity from the virtual runtime
2. Allows independent testing of utility implementations
3. Clear separation between pkg/core wrappers and custom implementations

## Test Strategy

### Unit Tests
- Each utility in `internal/uroot/` has corresponding `*_test.go`
- Use `bytes.Buffer` for stdin/stdout/stderr capture
- Test common flag combinations
- Test error cases (file not found, permission denied)

### CLI Integration Tests
- Add testscript tests in `tests/cli/testdata/uroot_*.txtar`
- Verify behavior matches spec acceptance scenarios
- Test cross-platform consistency

### Benchmark Tests
- Verify streaming I/O doesn't buffer large files
- Memory usage should be constant regardless of file size

## Dependencies

### New Dependencies

```go
import "github.com/u-root/u-root/pkg/core"
import "github.com/u-root/u-root/pkg/core/cat"
import "github.com/u-root/u-root/pkg/core/cp"
// etc.
```

### Version Pin

The u-root v0.15.0 (latest stable as of 2025-08):
```
github.com/u-root/u-root v0.15.0
```

### Compatibility

- The u-root library requires Go 1.24.0+ (Invowk requires Go 1.25+, so compatible)
- The u-root library depends on mvdan.cc/sh (same as Invowk, no version conflict expected)
- BSD-3-Clause license compatible with MPL-2.0

## Pre-Existing Issues Identified

### None Blocking

The current codebase already has:
1. Handler mechanism in place (`execHandler`, `tryUrootBuiltin`)
2. Configuration schema defined (`enable_uroot_utils`)
3. Default set to `true` (DefaultConfig)

No architectural blockers discovered.

## Summary

| Item | Decision |
|------|----------|
| Library | `github.com/u-root/u-root/pkg/core` v0.15.0 |
| Missing utilities | Custom implementations following `core.Command` |
| Package location | `internal/uroot/` |
| Error format | `[uroot] cmd: message` |
| Streaming I/O | Required for all file operations |
| Symlinks | Follow by default |
| Unsupported flags | Silently ignore |
| Testing | Unit + CLI integration + benchmarks |
