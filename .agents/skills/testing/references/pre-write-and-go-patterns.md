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
3. After moving or deleting tests, follow rule "File Shape" (clean imports,
   compile the package before widening verification).
4. Before adding `t.Parallel()`, check for process-global state, shared CUE
   values, shared channels/paths, and unsafe mocks. Follow the repository's
   parent/subtest placement policy.
5. Use `t.Fatalf` when continuing would dereference an invalid or nil result.
6. For three or more homogeneous cases, use concrete input/expected fields and
   one shared subtest body. Reserve `run func(*testing.T)` cases for genuinely
   heterogeneous scenarios, and keep `t.Run` visible in the owning test;
   package-level runner helpers hide callback semantics from `thelper` and
   `paralleltest`.

## Deterministic Tests

- Prefer events, channels, fake clocks, or polling with a deadline over sleeps.
  Keep sleeps only when the delay itself creates the behavior under test, such
  as watcher event separation or latency simulation, and document why.
- Use `t.Context()` by default and bound network, subprocess, and container
  work: rule "Test Context Usage" and `go-testing` "Context Patterns Decision
  Tree".
- Report goroutine failures to the test goroutine; do not call `t.Fatal` or
  `t.FailNow` from a worker.
- Run concurrency fixes repeatedly with cache bypass and the race detector:

  ```bash
  go test -count=10 -race -run '<TestName>' ./path/to/package/...
  ```

- Resolver-backed expected path strings and `os.SameFile` identity checks:
  `go-testing` "Cross-Platform Path Contract Tests".

## Helpers and Fixtures

Prefer existing helpers in `internal/testutil/` and
`internal/testutil/invowkfiletest/`. Inspect those packages for the live API;
do not copy a static exported-symbol inventory into this skill.

Use local helpers only when an import cycle prevents reuse, a signature is
package-specific, or the helper is genuinely single-use. Keep mocks local to
parallel subtests unless every access is synchronized.

For user-home tests, use `internal/testutil.SetHomeDir(t, dir)`, not a
`HOME`-only override: it sets `USERPROFILE` on Windows and `HOME` elsewhere,
and returns a restore func to register with `t.Cleanup`. Keep these tests
serial because home and environment state are process-global.

Host-absolute fixtures come from `filepath.Join(t.TempDir(), ...)` (rule
"Cross-Platform Testing"). Typed repository-relative path validators use the
seven `internal/testutil/pathmatrix` vectors on every OS, without platform
skips (rule "Cross-Platform Path Validator Matrix").

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
