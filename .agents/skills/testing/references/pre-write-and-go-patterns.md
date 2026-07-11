# Pre-Write and Go Test Patterns

Read this before creating or modifying `*_test.go`. Repository policy remains
authoritative in `.agents/rules/testing.md`; use `go-testing` for toolchain
details and its parallelism safety matrix.

## Pre-Write Checks

1. Inspect nearby `//nolint:` directives. Add only a specific, justified
   suppression after reproducing the lint finding. Move a directive with the
   code it suppresses and remove it when no longer needed.
2. Call `t.Helper()` first in helpers that report failures, including wrappers
   around other assertion helpers. Do not add it to a `t.Run` body.
3. Clean imports after moving or deleting tests and compile the affected
   package before widening verification.
4. Before adding `t.Parallel()`, check for process-global state, shared CUE
   values, shared channels/paths, and unsafe mocks. Follow the repository's
   parent/subtest placement policy.
5. Use `t.Fatalf` when continuing would dereference an invalid or nil result.

## Deterministic Tests

- Prefer events, channels, fake clocks, or polling with a deadline over sleeps.
  Keep sleeps only when the delay itself creates the behavior under test, such
  as watcher event separation or latency simulation, and document why.
- Use `t.Context()` by default. Bound network, subprocess, and container work.
- Report goroutine failures to the test goroutine; do not call `t.Fatal` or
  `t.FailNow` from a worker.
- Run concurrency fixes repeatedly with cache bypass and the race detector:

  ```bash
  go test -count=10 -race -run '<TestName>' ./path/to/package/...
  ```

- Build expected resolver-backed path strings from the same resolver as
  production. Use `os.SameFile` when only filesystem identity matters.

## Helpers and Fixtures

Prefer existing helpers in `internal/testutil/` and
`internal/testutil/invowkfiletest/`. Inspect those packages for the live API;
do not copy a static exported-symbol inventory into this skill.

Use local helpers only when an import cycle prevents reuse, a signature is
package-specific, or the helper is genuinely single-use. Keep mocks local to
parallel subtests unless every access is synchronized.

For user-home tests, use `internal/testutil.SetHomeDir(t, dir)`, not a
`HOME`-only override. Keep these tests serial because home and environment
state are process-global.

For host-path validation, create absolute paths with `t.TempDir()` and
`filepath.Join`. For typed repository-relative paths, cover Unix absolute,
Windows drive/rooted/UNC, and slash/backslash traversal inputs without
platform skips.

## Verification

Start narrow, then run the owning repository target:

```bash
go test -count=1 -run '<TestName>' ./path/to/package/...
go test -count=1 ./path/to/package/...
make lint
make test
```

Also run `make check-windows-build` for path/process changes and the additional
surface gates required by `.agents/rules/checklist.md`.
