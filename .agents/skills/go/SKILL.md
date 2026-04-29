---
name: go
description: Go coding and lint guardrails for Invowk. Use whenever Codex creates, edits, reviews, refactors, or tests Go code (*.go), changes Go linters or formatters, fixes golangci-lint/goplint/staticcheck/revive/decorder/funcorder/gocritic/modernize/perfsprint/wrapcheck/errcheck errors, or touches Go package architecture.
---

# Go

Apply this skill before editing Go and again before final verification. It
captures the project patterns most likely to fail `make lint`, `make
check-baseline`, commit hooks, or review.

## Mandatory Flow

1. Identify the touched package role: CLI adapter, app service/domain,
   infrastructure adapter, public `pkg/*`, test, or tool.
2. Preserve hexagonal boundaries: adapters parse/render/wire; services return
   typed results/errors/diagnostics; domain packages do not write terminal UI.
3. Keep declaration order as `const`, `var`, `type`, `func`, with one block of
   each kind per file when needed. Place exported funcs before unexported funcs.
4. Thread caller contexts through all I/O, subprocess, HTTP, server lifecycle,
   discovery, runtime, and long-running operations.
5. Use project value types and `Validate()` contracts for domain data. Consult
   `invowk-typesystem` when adding or changing value types.
6. Before finishing, run the narrow relevant `go test` command, `make lint`,
   and `make check-baseline` when value types, validators, or exported structs
   changed.

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
- Split large files by moving declarations exactly once, deleting originals,
  cleaning imports, then running `go build` or targeted tests immediately.

## Errors

- Sentinel errors must use constant-backed messages.

```go
const notFoundErrMsg = "resource not found"

var ErrNotFound = errors.New(notFoundErrMsg)
```

- Error variables start with `Err`; error types end with `Error`.
- Wrap external or boundary errors with context: `fmt.Errorf("load config: %w",
  err)`.
- Use `errors.AsType[T]` in production code, not `errors.As`.
- Use `errors.New` instead of `fmt.Errorf` when there is no formatting or
  wrapping.
- Never return nil error after checking a non-nil error.
- Do not discard errors unless the function is allowlisted or the discard is
  deliberate, commented, and accepted by lint.
- For resources, prefer named returns and deferred close aggregation. For
  best-effort cleanup, log or comment the intentional discard.

## Context And Processes

- `context.Context` is the first parameter.
- Use `cmd.Context()` at Cobra boundaries and pass `ctx context.Context` into
  helpers.
- Use `http.NewRequestWithContext`, `exec.CommandContext`, and
  context-aware engine/server calls.
- `context.Background()` is only for true roots, nil fallback documented on an
  options struct, bounded availability probes, `TestMain`, or independent
  shutdown after caller cancellation.
- Tests should default to `t.Context()`.

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
  `tools/goplint/exceptions.toml` or justified with `//goplint:ignore -- ...`.

## Lint Traps

- `decorder`/`funcorder`: do not add extra `const`, `var`, or `type` blocks;
  group related declarations with `type (...)`, etc.
- `revive exported`: every exported type, const, var, function, and method needs
  a meaningful doc comment starting with the identifier.
- `revive context-as-argument`: context first, always.
- `wrapcheck`: wrap errors from external packages unless the config explicitly
  ignores that signature/package.
- `errcheck`: check returned errors. In tests, use helpers or assertions rather
  than blank identifiers.
- `nolintlint`: avoid `//nolint`; if unavoidable, name the exact linter and add
  a reason.
- `forbidigo`: production code uses `errors.AsType[T]`.
- `modernize`: prefer Go 1.26 forms such as `slices.Contains`,
  `strings.SplitSeq` in range-only loops, and `fmt.Appendf` instead of
  `[]byte(fmt.Sprintf(...))`.
- `gocritic rangeValCopy`: range over map/slice keys or indices when values are
  large structs.
- `gocritic`/`predeclared`: do not shadow builtins such as `len`, `cap`, `new`,
  `make`, `copy`, or `append`.
- `perfsprint`: use `errors.New`, `strconv`, or direct conversions where
  formatting is unnecessary.
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
- `goconst`: repeated production strings usually become constants.
- `iface`: avoid interfaces until they express a real port/contract. Keep them
  small and consumed by callers.

## Comments

- Comments document semantics: intent, ownership, lifecycle, side effects,
  security, precedence, and invariants.
- Do not restate obvious syntax.
- Comment subtle ordering, fallback, cleanup, or race-sensitive behavior.
- Guardrail tests scan comments with raw substring matching. Refer to removed or
  forbidden APIs indirectly; do not paste prohibited call expressions into
  comments.

## Tests

- Use standard `testing`, not `testify`.
- Test helpers call `t.Helper()` at the beginning.
- Use `t.Context()`, `t.TempDir()`, `t.Setenv()`, and `t.Chdir()` where
  applicable.
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
