# mvdan/sh Virtual Shell

## Overview

The virtual runtime uses [mvdan/sh](https://github.com/mvdan/sh), a pure Go POSIX shell interpreter. This provides cross-platform bash-like script execution without requiring an external shell binary.

## Positional Arguments Gotcha

**CRITICAL: Always prepend `"--"` when passing positional arguments to `interp.Params()`.**

The `interp.Params()` function configures shell parameters, but it follows POSIX shell conventions where arguments starting with `-` are interpreted as shell options (like `-e`, `-u`, `-x`).

### The Problem

Without `"--"`, positional arguments like `-v` or `--env=staging` are interpreted as shell options:

```go
// WRONG: Will fail with "invalid option: -v"
if len(args) > 0 {
    opts = append(opts, interp.Params(args...))
}
```

Error messages you might see:
- `failed to create interpreter: invalid option: "-v"`
- `failed to create interpreter: invalid option: "--"`
- `failed to create interpreter: invalid option: "--env=staging"`

### The Solution

Prepend `"--"` to signal the end of options:

```go
// CORRECT: "--" signals end of options, remaining args become $1, $2, etc.
if len(args) > 0 {
    params := append([]string{"--"}, args...)
    opts = append(opts, interp.Params(params...))
}
```

### Why This Works

In POSIX shells, `"--"` is the standard delimiter that terminates option parsing. Everything after `"--"` is treated as a positional parameter, regardless of whether it starts with `-`:

```bash
# Shell equivalent:
set -- -v --env=staging  # Sets $1="-v", $2="--env=staging"
```

### Affected Locations

When working with mvdan/sh in this codebase, ensure `"--"` is prepended in:

1. `internal/runtime/virtual.go` - `Execute()` method
2. `internal/runtime/virtual.go` - `ExecuteCapture()` method
3. `cmd/invowk/internal_exec_virtual.go` - `runInternalExecVirtual()` function

### Testing

This issue manifests on Windows CI because the virtual runtime is the only bash-compatible option there. When adding new mvdan/sh integration points:

1. Test with arguments starting with `-` (e.g., `-v`, `--flag=value`)
2. Run `make test-cli` to verify flag handling works
3. Ensure Windows CI passes

## Common Pitfall

- **Missing `"--"` delimiter** - Always use `append([]string{"--"}, args...)` when calling `interp.Params()` with user-provided positional arguments.
