---
paths:
  - "**/*_test.go"
  - "tests/cli/**"
  - "**/testdata/**"
  - "internal/testutil/**"
---

# Testing

Deeper guidance: `go-testing` (toolchain, flags, race detector, gotestsum), `testing` (Invowk patterns, testscript, TUI, containers), `native-mirror`, `review-tests`, and the platform skills `windows-testing`, `macos-testing`, `linux-testing`.

## File Shape

- Test files MUST NOT exceed 1000 lines (`make check-file-length`). Split by concern into `<package>_<concern>_test.go`, one logical area per file, following `.agents/skills/go/SKILL.md` "File Shape".
- When moving tests: delete the moved `Test*` functions from the source file (same-package test files share a namespace), clean up orphaned imports, and run `go build ./path/to/pkg/...` before `make test`.
- Before deleting a shared `_test.go` helper, grep the whole package for callers.
- Prefer table-driven tests for functions with several cases.

## Test Context Usage

- **Use `t.Context()` as the default context in tests.** Do NOT use `context.Background()` unless structurally forced.
- `context.WithCancel(t.Context())` only when the scenario needs explicit mid-test cancellation; `context.WithTimeout(t.Context(), ...)` only when a deadline is part of the test logic.
- `context.Background()` only in `TestMain` (`*testing.M` has no `Context()`) or in `testscript.Env.Defer()` cleanup hooks (no `*testing.T` in scope), always with a comment saying why.
- Benchmarks use `b.Context()`.

## Test Parallelism

All new test functions MUST call `t.Parallel()` unless they mutate global/process-wide state. **`t.Parallel()` must be the first call in the function** — before any `t.Skip()` guards or other setup. The `paralleltest` linter enforces missing `t.Parallel()` calls; the `tparallel` linter enforces placement and parent/subtest consistency.

- When a parent calls `t.Parallel()`, ALL its `t.Run()` subtests must too (`tparallel`). If one subtest cannot be parallel, drop `t.Parallel()` from the parent or add a local `//nolint:paralleltest // <reason>` to the intentionally serial parent.
- Never parallelize tests that use `os.Chdir`, `os.Setenv`, `t.Setenv`, `testutil.MustSetenv()`, `testutil.MustChdir()`, `SetHomeDir` or similar process-wide overrides, or any `os.Stdin` replacement.
- Use `t.TempDir()`, never `os.MkdirTemp` + `defer os.RemoveAll`: a parent's `defer` runs while its parallel subtests are still executing.
- No `tt := tt` / `tc := tc` rebinding (`modernize` `forvar`); clone maps with `maps.Copy(dst, src)` (`mapsloop`).
- Test semaphores: a helper that may acquire more than once per test releases its token in the same call (`defer`), never through `t.Cleanup()`, which holds tokens until test end and can stall parallel suites. Once-per-test container acquisition uses `testutil.AcquireContainerSemaphore(t)`, which releases via `t.Cleanup` (see "Integration and Container Tests").

## Integration and Container Tests

- Tests that need external resources (container engine, network) skip under `testing.Short()`.
- Real container integration tests call `testutil.AcquireContainerSemaphore(t)` once per test, after `t.Parallel()` and the Short skip and before any container operation. Not needed for mocked engines, validation-only tests (`Validate()`, type assertions), or error paths that fail before container operations. Capacity and `INVOWK_TEST_CONTAINER_PARALLEL`: `.agents/skills/linux-testing/references/container-testing-deep.md`.
- `internal/runtime` container tests do NOT use `AcquireContainerSuiteLock`; the semaphore alone controls concurrency. The suite lock is only for `tests/cli` container tests (cross-process serialization during `make test`).
- **CRITICAL:** `internal/runtime` container tests that call `Execute()` or `ExecuteCapture()` MUST use `testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)` instead of bare `t.Context()`, which has no deadline. Not needed for `Validate()`-only or type-assertion tests.
- CLI container tests set a per-test testscript `Deadline` (`containerTestTimeout`) and clean up containers by prefix in `env.Defer` via `cleanupTestContainersForHarness(prefix, currentContainerHarness())`. Raise `containerTestTimeout` if tests consistently need more (e.g., large image pulls); CI step timeouts must exceed the sum of sequential container test deadlines.
- testscript sets `HOME=/no-home`, which breaks `docker`/`podman build`; `commonSetup` in `tests/cli/cmd_test.go` sets `HOME` to `env.WorkDir`. On Windows it must also set `APPDATA=WorkDir/appdata` and `USERPROFILE=WorkDir`, or tests share the real `%APPDATA%\invowk`.

## Cross-Platform Testing

- Never hardcode path separators in assertions: use `filepath.Join()` for host paths. Container paths (always `/`) are fine as literals. Path domains: `windows.md` "Path Handling".
- Use a `skipOnWindows bool` table field (checked with `goruntime.GOOS == "windows"`) only for Unix-style absolute paths, Unix-only behavior (symlinks, permissions) or assumptions that do not translate to Windows. Do not skip when `filepath.Join()` makes the case portable, when the difference is an implementation bug, or when the type must enforce a platform-agnostic contract (e.g., repo-relative paths).
- Valid host-absolute fixtures come from `filepath.Join(t.TempDir(), ...)`, never `/foo/bar` (not absolute on Windows); keep relative negative cases (`relative/...`, `./...`) explicit.
- Import the stdlib package as `goruntime "runtime"` when `internal/runtime` is also imported.
- Assert `%q`-rendered paths with `fmt.Sprintf("%q", path)`: `%q` doubles Windows backslashes.

### Cross-Platform Path Validator Matrix

Value-type path validators and resolvers that must behave identically across OSes are tested against the seven canonical vectors — UnixAbsolute (`/absolute/path`), WindowsDriveAbs (`C:\absolute\path`), WindowsRooted (`\absolute\path`), UNC (`\\server\share`), SlashTraversal (`a/../../escape`), BackslashTraversal (`a\..\..\escape`), ValidRelative (`tools`, `modules/tools`, `./tools`) — on every OS, using `internal/testutil/pathmatrix` (`Validator` / `Resolver`). This keeps Linux and macOS runs protective against Windows-only regressions.

## Platform Coverage and Runtime Mirrors

- **CRITICAL:** all user-facing features are tested on Linux, macOS AND Windows. CI: Linux runs the full suite (unit, integration, CLI, container); macOS and Windows run short unit tests plus CLI tests.
- New shell-feature tests default to `virtual-sh` plus a native mirror.
- **CRITICAL:** every `virtual_<feature>.txtar` needs a `native_<feature>.txtar` mirror testing the same behaviors (`TestShRuntimeMirrorCoverage`, `TestVirtualNativeCommandPathAlignment`). Exceptions for virtual files live only in `tests/cli/runtime_mirror_exemptions.json`, each with a reason, and only for virtual-runtime built-ins, CUE-validation-only or diagnostics-formatting tests (`exact`/`prefix`); its `command_path_exempt` map only lets an existing mirror pair invoke different command paths. Non-virtual tests (`container_*`, `dogfooding_*`, built-in command tests such as `config_*`, `module_*`, `completion`, `tui_*`, `init_*`, `validate`) need no mirror.
- New testscript tests MUST use inline CUE. Only `dogfooding_*.txtar` may reference `$PROJECT_ROOT`.
- Every implementation declares all applicable platforms.

### Native tests

- Always platform-split: one implementation for `linux`/`macos`, one for `windows`. The Unix one runs under `$SHELL`, falling back to bash (zsh on macOS, bash on Linux CI), so use syntax valid in both.
- `stdout`/`stderr` assertions are identical across platforms; only the script syntax differs.
- Declare `runtimes: [{name: "native"}]` only; a virtual fallback defeats the test.
- PowerShell implementations (canonical rules; `windows.md` points here):
  - compatible with both PowerShell 5.1 and 7+: no ternary `? :`, no `??`, no `&&`/`||` pipeline chains
  - `$env:VAR` for environment variables, never `$VAR`
  - `Write-Output`, not `echo`
  - `-eq`, `-ne`, `-lt`, `-gt`, `-like`, `-match`, never bash comparisons
  - `$ErrorActionPreference = 'Stop'` instead of `set -e`
  - anchored regex assertions use `\r?$` (`stdout '^done\r?$'`); substring assertions need nothing
  - run one with `go test -run TestCLI/native_<name> ./tests/cli/`; details: `.agents/skills/windows-testing/references/powershell-testing.md`

### Platform skips

- Allowed, for design constraints: `[!container-available] skip`, `[!net] skip`, `[in-sandbox] skip`, `[windows] skip` for genuine Windows limits (Unix permission bits, hardcoded `/tmp` upstream).
- Not allowed: skipping a platform because an implementation is missing (e.g., `[macos] skip 'not implemented'`); fix the implementation.
- Use the built-in `[windows]`, `[linux]`, `[darwin]` conditions, never `[GOOS:windows]`; unknown conditions fail through `commonCondition()`.

## Pitfalls

- testscript `stdout`/`stderr` patterns are Go regex: escape `\(s\)` or use simpler patterns.
- CLI handler error tests assert both channels: the handler's styled stdout and Cobra's stderr.
- All txtar file entries land under `$WORK`; isolate tests that need a clean workspace in a subdirectory (`cd $WORK/clean_workspace`) so broken fixtures are not discovered.
- `*.txtar` is forced to `eol=lf` in `.gitattributes`; keep it, or Windows checkouts change fixture checksums.
- Ubuntu runners preset `XDG_CONFIG_HOME`; `unset XDG_CONFIG_HOME` before testing the XDG fallback.
- Make uniqueness deterministic with an atomic counter, never with sleeps or time-based values.
- Test behavioral contracts (sentinel errors via `errors.Is`, defaults that affect users, state transitions), not constants or zero values.
- Helpers: `internal/testutil` (`MustSetenv`, `MustChdir`, `MustRemoveAll`, `SetHomeDir`, `pathmatrix`, `invowkfiletest`, container semaphore and context).

## Guardrail Tests

- `TestNoGlobalConfigAccess` (`internal/config`) scans non-test `.go` files with `strings.Contains`; comments must not spell out the deprecated global config accessor call.
- `TestBuiltinCommandTxtarCoverage` (`cmd/invowk/cmd_coverage_test.go`) fails when a non-hidden, runnable leaf built-in command has no `.txtar` test in `tests/cli/testdata/`.
- `TestIssueTemplates_NoStaleGuidance` (`internal/issue`) rejects stale tokens such as `invowk fix` and `apk add --no-cache` in issue templates.
