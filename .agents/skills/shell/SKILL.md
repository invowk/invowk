---
name: shell
description: Shell runtime rules for the mvdan/sh virtual-sh runtime (internal/runtime/sh.go), the hidden exec-virtual-sh command (internal_exec_sh.go), shared virtual-shell interactive plumbing, and repository bash scripts. Covers the interp.Params positional arguments gotcha (prepend "--", invalid option errors), bash strict mode, and ((var++)) arithmetic increment pitfalls with set -e.
---

# Invowk Shell Runtime Rules

This skill covers Invowk shell internals and repository bash script conventions,
not general bash tutoring.

## Directives

- **CRITICAL:** always pass `append([]string{"--"}, args...)` to `interp.Params()` for user-provided positional arguments.
- **CRITICAL:** in scripts with `set -e`, use `VAR=$((VAR + 1))`, never `((VAR++))`, when `VAR` might be 0.

Use this skill when working on:
- `internal/runtime/sh.go` - virtual-sh runtime
- `cmd/invowk/internal_exec_sh.go` - hidden `exec-virtual-sh` command
- `internal/app/commandadapters/sh_interactive.go` and shared virtual interactive helpers
- Repository bash script execution logic

---

## Virtual Shell (mvdan/sh)

The virtual-sh runtime uses [mvdan/sh](https://github.com/mvdan/sh), a pure Go POSIX shell interpreter. This provides cross-platform bash-like script execution without requiring an external shell binary.

### Positional Arguments Gotcha

**CRITICAL: Always prepend `"--"` when passing positional arguments to `interp.Params()`.**

The `interp.Params()` function configures shell parameters, but it follows POSIX shell conventions where arguments starting with `-` are interpreted as shell options (like `-e`, `-u`, `-x`).

#### The Problem

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

#### The Solution

Prepend `"--"` to signal the end of options:

```go
// CORRECT: "--" signals end of options, remaining args become $1, $2, etc.
if len(args) > 0 {
    params := append([]string{"--"}, args...)
    opts = append(opts, interp.Params(params...))
}
```

`"--"` terminates option parsing, exactly like the shell's `set -- -v --env=staging` (sets `$1="-v"`, `$2="--env=staging"`).

### Affected Locations

When working with mvdan/sh in this codebase, ensure `"--"` is prepended in:

1. `internal/runtime/sh.go` - `prepareShExec()` for normal virtual-sh execution
2. `internal/runtime/sh.go` - `RunShScript()` for the hidden `exec-virtual-sh` command path (`cmd/invowk/internal_exec_sh.go`)

`internal/app/commandadapters/sh_interactive.go` transports arguments through
`--args`; verify that transport stays aligned with the runtime-side
`interp.Params` delimiter, but do not copy the delimiter pattern there unless it
is directly calling mvdan/sh.

### Testing

This issue manifests on Windows CI because virtual-sh is the bash-compatible embedded shell option there. When adding new mvdan/sh integration points:

1. Test with arguments starting with `-` (e.g., `-v`, `--flag=value`)
2. Run `go test ./internal/runtime -run TestShRuntime_PositionalArgs_DashPrefix -count=1`
3. Run relevant interactive adapter tests when `internal/app/commandadapters/*sh*` changes
4. Run `make test-cli` when CLI behavior or txtar fixtures changed

---

## Bash Script Execution

### Strict Mode (`set -euo pipefail`)

Project-level bash entrypoints should use strict mode for safety:

```bash
set -euo pipefail
```

This exits on command failures, undefined variables, and failed pipeline
segments.

- POSIX `#!/bin/sh` scripts (e.g. `scripts/install.sh`, `scripts/test_install.sh`) use `set -eu`, because `pipefail` is not POSIX.
- Short sourced helpers that only emit shell fragments, such as the Bencher helper scripts (`scripts/bencher-*.sh`), may omit strict mode deliberately.
- Strict mode applies to repository bash scripts, not to CUE command scripts executed via container runtimes (`/bin/sh -c` does not imply `set -e`); see `.agents/skills/testing/references/container-and-tui.md`.

Verify changed repo bash scripts with `bash -n <script>` and `make lint-scripts`
when applicable. Without `shellcheck` on `PATH`, `make lint-scripts` prints a
skip notice and exits 0, so install it before trusting a pass.

### Arithmetic Increment Gotcha

**CRITICAL: Never use `((var++))` with `set -e` when `var` might be 0.**

In bash, arithmetic expressions return exit status based on the expression's value:
- `((0))` returns exit status 1 (false)
- `((1))` returns exit status 0 (true)

The post-increment `((x++))` evaluates to the *original* value of `x`, so `((COUNTER++))`, `((FAILED++))` or `((SKIPPED++))` starting from 0 returns status 1 and `set -e` terminates the script. Use assignment syntax, which always succeeds:

```bash
# WRONG - Script exits on first skip when SKIPPED is 0:
SKIPPED=0
if [[ ! -f "$golden_file" ]]; then
    ((SKIPPED++))  # Exit status 1, script terminates!
    continue
fi

# CORRECT - Assignment syntax always works:
SKIPPED=0
if [[ ! -f "$golden_file" ]]; then
    SKIPPED=$((SKIPPED + 1))  # Always succeeds
    continue
fi
```
