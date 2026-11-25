---
name: go
description: Go coding and lint guardrails for Invowk. Use whenever you create, edit, review, refactor, or test Go code (*.go), change Go linters or formatters (.golangci.toml), fix golangci-lint/goplint/staticcheck/revive/decorder/funcorder/gocritic/modernize/perfsprint/wrapcheck/errcheck errors, or touch Go package architecture.
---

# Go

Apply this skill before editing Go and again before final verification. It
captures the project patterns most likely to fail `make lint`, `make
check-baseline`, commit hooks, or review.

## Mandatory Flow

1. Identify the touched package role: CLI adapter, app service/domain,
   infrastructure adapter, public `pkg/*`, test, or tool.
2. Preserve hexagonal boundaries: adapters parse/render/wire; services and
   domain packages return typed results/errors/diagnostics and never write
   terminal UI (`.agents/rules/package-design.md`; design questions:
   `go-hexagonal-ddd`).
3. Keep declaration order as `const`, `var`, `type`, `func`, with one block of
   each kind per file when needed. Place exported funcs before unexported funcs.
4. Thread caller contexts through all I/O, subprocess, HTTP, server lifecycle,
   discovery, runtime, and long-running operations.
5. Use project value types and `Validate()` contracts for domain data. Consult
   `invowk-typesystem` when adding or changing value types.
6. Before finishing, run the narrow relevant `go test` command first, then the
   completion gates in `.agents/rules/checklist.md`, unless the user explicitly
   scoped the work to a read-only review.

## File Shape

Use this order:

```go
// SPDX-License-Identifier: MPL-2.0

// Package name explains the package contract.
package name

import (...)

const (...)

var (...)

type (...)

func Exported() {}

func unexported() {}
```

- Use `doc.go` for multi-paragraph package documentation.
- Keep only one package comment per package. If `doc.go` exists, other files
  should have only `package name`.
- Import groups must be standard library, external modules, then
  `github.com/invowk/invowk/...`.
- Split large files with this deterministic protocol: move declarations exactly
  once, delete originals in the same patch, clean imports, run
  `go build ./path/to/package/...`, then run targeted tests for that package.

## Errors

- Sentinel errors must use constant-backed messages.

```go
const notFoundErrMsg = "resource not found"

var ErrNotFound = errors.New(notFoundErrMsg)
```

- Error variables start with `Err`; error types end with `Error`.
- Wrap external or boundary errors with context: `fmt.Errorf("load config: %w",
  err)` (`wrapcheck`: required for errors from external packages unless
  `.golangci.toml` explicitly ignores that signature/package).
- Use `errors.AsType[T]` in production code, not `errors.As` (`forbidigo`;
  test files are exempt).
- Use `errors.New` instead of `fmt.Errorf` when there is no formatting or
  wrapping (`perfsprint`).
- Test a wrapped error with `errors.Is(err, fs.ErrNotExist)`, never
  `os.IsNotExist(err)`: `os.Is*` sees through `*PathError` only, not `%w` or
  `errors.Join`, so a not-found branch on a wrapped error never runs (the
  `module add` not-found line, fixed in module-cli-messages).
- Never return nil error after checking a non-nil error.
- Do not discard errors unless the function is allowlisted or the discard is
  deliberate, commented, and accepted by lint.
- For resources, prefer named returns and deferred close aggregation. For
  best-effort cleanup, log or comment the intentional discard.

## Per-User Directories

- Never call `os.UserHomeDir`, `os.UserConfigDir`, `os.UserCacheDir`,
  `os.ExpandEnv` or `os.Expand`, import `os/user` or a home/XDG helper
  (`go-homedir`, `adrg/xdg`, ...), or read `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA` or an
  `XDG_*` variable outside `internal/basedir`. Take a `basedir.Dirs` (zero
  value = the real process) and call `Home`, `ConfigHome`, `DataHome`,
  `CacheHome`, `StateHome` or `RuntimeDir`; every result is absolute or an
  error. forbidigo flags the three calls; `TestBaseDirGuard`
  (`internal/basedir/guard_test.go`) is the authority and also catches
  aliases, dot imports, literals, `NAME=` prefixes and `$NAME`/`%NAME%`
  references, across the whole module. Help text that must name a variable
  goes in `cmd/invowk/config_paths_help.go`. Exceptions go in its
  `guardAllowlist` with a reason.
- Review new dependencies that read per-user files (`~/.ssh`, `known_hosts`,
  `~/.gitconfig`, engine configs): the guard cannot see inside them, so gate
  the call site the way `requireTrustedSSHFiles` (`internal/app/modulesync`)
  gates go-git's SSH transport.

## Context And Processes

- `context.Context` is the first parameter (`revive context-as-argument`).
- Use `cmd.Context()` at Cobra boundaries and pass `ctx context.Context` into
  helpers.
- Use `http.NewRequestWithContext`, `exec.CommandContext`, and
  context-aware engine/server calls.
- For long-running subprocesses, set `cmd.WaitDelay` when cancellation or
  pipe-draining can otherwise hang.
- When capturing subprocess stdout/stderr that may be inspected before
  `Run`/`Wait` has completed, do not expose a raw `bytes.Buffer`; use a
  synchronized writer/snapshot type or wait for the process first.
- `context.Background()` is only for true roots, nil fallback documented on an
  options struct, bounded availability probes, `TestMain`, or independent
  shutdown after caller cancellation. Tests use `t.Context()` (see Tests).

## Validation And Value Types

- Constructors or factories for structs with invariants should call
  `Validate()` before returning.
- Never ignore `Validate()` results.
- A struct `Validate()` must delegate to every field that has `Validate()`.
  For maps/slices of validating values, create a named collection type with its
  own `Validate()` when that makes delegation explicit.
- `Validate()` methods need production call sites; test-only validators are
  misleading.
- Keep `Invalid*Error` wrappers and sentinel `Unwrap()` behavior consistent.
- Avoid bare primitives in domain structs, params, and returns unless the value
  is a real boundary/display/free-form exception already documented in
  `.goplint/exceptions.toml` or justified with `//goplint:ignore -- ...`.

## Lint Traps

Error and context linters (`wrapcheck`, `forbidigo`, `perfsprint` for errors,
`revive context-as-argument`) are covered in the sections above.

- `decorder`/`funcorder`: do not add extra `const`, `var`, or `type` blocks;
  group related declarations with `type (...)`, etc.
- `revive exported`: every exported type, const, var, function, and method needs
  a meaningful doc comment starting with the identifier.
- `errcheck`: check returned errors. In tests, use helpers or assertions rather
  than blank identifiers.
- `forcetypeassert`: production type assertions use the checked `v, ok :=`
  form (errcheck's `check-type-assertions` is off because this linter owns it;
  test files are exempt).
- `nolintlint`: avoid `//nolint`; if unavoidable, name the exact linter and add
  a reason.
- `modernize`: prefer Go 1.27 forms such as `slices.Contains`,
  `strings.SplitSeq` in range-only loops, and `fmt.Appendf` instead of
  `[]byte(fmt.Sprintf(...))`.
- `gocritic rangeValCopy`: range over map/slice keys or indices when values are
  large structs.
- `gocritic`/`predeclared`: do not shadow builtins such as `len`, `cap`, `new`,
  `make`, `copy`, or `append`.
- `perfsprint`: use `strconv` or direct conversions instead of `fmt.Sprintf`
  where formatting is unnecessary.
- `staticcheck`: remove redundant struct literals and unnecessary conversions.
- `unconvert`: avoid conversions that do not change type.
- `exhaustive`: list every enum value in switches, even when a case matches the
  default behavior.
- `tagliatelle`: JSON tags are `snake_case`; add `mapstructure` tags for Viper
  config structs.
- `sloglint`: use key-value logging with snake_case keys.
- `nosprintfhostport`: use `net.JoinHostPort`.
- `bodyclose`: close HTTP response bodies.
- `makezero`: do not append into slices created with non-zero length unless that
  is really intended.
- `depguard`: no `io/ioutil`; tests do not use `testify`.
- `mnd`: name non-obvious numbers, except common ignored cases in
  `.golangci.toml` and test data.
- `goconst`: repeated production strings usually become constants (fires at 5
  occurrences of strings of 6+ characters; test files are excluded).
- `iface`: avoid interfaces until they express a real port/contract. Keep them
  small and consumed by callers.

## Comments

- Comments document semantics: intent, ownership, lifecycle, side effects,
  security, precedence, and invariants.
- Do not restate obvious syntax.
- Comment subtle ordering, fallback, cleanup, or race-sensitive behavior.
- Guardrail tests scan source, comments included, with raw substring matching
  (e.g. `TestNoGlobalConfigAccess` in `internal/config/config_generate_test.go`
  checks non-test `.go` files with `strings.Contains`). Refer to removed or
  forbidden APIs indirectly; do not paste prohibited call expressions into
  comments.

## Tests

- Use standard `testing`, not `testify`.
- Test helpers call `t.Helper()` at the beginning.
- Default to `t.Context()` for test contexts; use `t.TempDir()`, `t.Setenv()`,
  and `t.Chdir()` where applicable.
- When tests exercise libraries that write relative artifacts, isolate with
  `t.Chdir(t.TempDir())`.
- For Go test execution strategy, race detector behavior, platform flakes, or
  CLI testscript guidance, also use `go-testing` and the relevant platform
  testing skill.

## Linter Config Changes

- Keep `.golangci.toml` documented when adding, removing, or configuring a
  linter.
- Never disable `decorder` or `funcorder`.
- Exclusions must explain why the policy exception is acceptable.
- After linter config changes, run `make lint` and the relevant hook/check
  (`make check-baseline` for goplint-sensitive changes).
- Analyzer behavior changes belong in the standalone
  [`github.com/invowk/goplint`](https://github.com/invowk/goplint) repository,
  which owns its own soundness gates; invowk-side changes run the consumer
  gates (`make check-goplint-consumer-routed`).
