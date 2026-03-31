# Known Test Pattern Exceptions Registry

Items listed here are DELIBERATELY different from standard test patterns.
Do NOT flag these as errors during review. Mark findings against these as severity **SKIP**.

## Registry

### Parallelism Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| Tests using `os.Chdir`, `os.Setenv`, `t.Setenv` | No `t.Parallel()` | Process-wide side effects; parallel execution causes race conditions |
| Tests using `testutil.MustSetenv()`, `testutil.MustChdir()` | No `t.Parallel()` | Wrappers for process-wide state mutation |
| Tests using `SetHomeDir` or similar process-wide overrides | No `t.Parallel()` | Modifies process-level configuration directory |
| Tests using `withPipeStdin()` or replacing `os.Stdin` | No `t.Parallel()` | `os.Stdin` is process-wide; concurrent replacement causes data races |
| CUE `cue.Value` / `*cue.Context` subtests | Serial subtests (no `t.Parallel()` on subtests) | CUE values and contexts are NOT thread-safe; `Unify()` and `CompileString()` mutate internal state. Use `//nolint:tparallel` when parent calls `t.Parallel()` but subtests must be serial |
| SSH server controller tests (`internal/sshserver/`) | No `t.Parallel()` on parent or subtests | `wish` library writes host keys to `.ssh/` in working directory; parallel tests in the same package collide |
| SSH server controller test (`internal/app/commandsvc/ssh_test.go`) | No `t.Parallel()` on parent or subtests | `wish` library writes host keys (`id_ed25519`) relative to CWD; parallel tests collide on the same key file |
| TUI client tests (`internal/tuiserver/client_test.go`) | Sequential subtests | Share request/response channels via `server.RequestChannel()`; parallel subtests would race on the channel |
| Mock exec command tests (`internal/container/engine_mock_test.go`) | No `t.Parallel()` | `withMockExecCommand` replaces the package-level `execCommand` variable, which is process-wide for the test binary |
| Container engine integration tests (`internal/container/engine_test.go`) | No `t.Parallel()` on parent (subtests serial) | `TestDockerEngine_Integration` and `TestPodmanEngine_Integration` share engine instance and context across subtests; per-subtest isolation would require separate engine connections |

### Context Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| `TestMain` functions | `context.Background()` | `*testing.M` has no `Context()` method |
| `testscript.Env.Defer()` cleanup callbacks | `context.Background()` | No `*testing.T` in scope; cleanup runs after test completes |
| Package-level variable init in test files | `context.Background()` | No `*testing.T` available at init time |

### Test Helper Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| Same-package test helpers that duplicate `testutil` | Local helper instead of shared `testutil` | Import cycle avoidance: `internal/testutil` cannot import the package under test, and the package under test cannot import `testutil` if `testutil` already imports it |
| Specialized test helper with unique signature | Local helper instead of `testutil` | Helper signature is specific to one package's testing needs; promoting to `testutil` would add unrelated dependencies |

### Container Test Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| Unit tests with mocked container engines | No `ContainerSemaphore()` | Mocked tests don't interact with the container daemon; semaphore only needed for real container operations |
| Validation-only tests (`Validate()`, type assertions) | No `ContainerSemaphore()`, no `ContainerTestContext()` | No container daemon interaction; these are pure logic tests |
| Error-path tests that fail before container operations | No `ContainerSemaphore()` | Execution never reaches container operations (e.g., missing SSH server) |
| `internal/runtime` container tests | No `AcquireContainerSuiteLock` | Semaphore alone provides concurrency control; suite lock is only for `tests/cli` cross-process serialization |

### Hardcoded Path Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| `internal/app/deps/filepaths_test.go` | Hardcoded `/tmp`, `/var/tmp` in container filepath fixtures | Container runtime is Linux-only by design; these are container-internal paths, not host paths. Tests have `runtime.GOOS` guards |
| `internal/uroot/dirname_test.go` | Hardcoded Unix paths (`/foo/bar`, `/a/b/c/d`) | u-root implements POSIX `path.Dir` (not `filepath.Dir`); runs exclusively in the virtual shell with POSIX semantics on all platforms |

### Mirror Exemptions

| Category | Files | Rationale |
|---|---|---|
| u-root commands | `virtual_uroot_*.txtar` | u-root commands are virtual-shell built-ins with no native equivalent |
| Virtual-shell-specific | `virtual_shell.txtar` | Tests virtual-shell-specific features (u-root integration, cross-platform POSIX) |
| Container runtime | `container_*.txtar` | Linux-only by design; container runtime is not a native shell |
| CUE validation | `virtual_edge_cases.txtar`, `virtual_args_subcommand_conflict.txtar` | Tests schema parsing/validation, not runtime behavior |
| Discovery/diagnostics | `virtual_diagnostics_footer.txtar` | Tests diagnostics footer formatting, not shell execution |
| Dogfooding | `dogfooding_invowkfile.txtar` | Already exercises native runtime through the project's own invowkfile.cue |
| Built-in commands | `config_*.txtar`, `module_*.txtar`, `completion.txtar`, `tui_*.txtar`, `init_*.txtar`, `validate.txtar` | Built-in Cobra commands exercise CLI handlers directly, not user-defined command runtimes |

See `tests/cli/runtime_mirror_exemptions.json` for the machine-readable exemption list.

### Integration Test Gating Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| `TestCLI` in `tests/cli/cli_test.go` | Runs in short mode (`testing.Short()` not checked for gating) | Individual txtar tests handle their own skipping via built-in testscript conditions (`[windows]`, `[!container-available]`, etc.). The TestCLI harness intentionally does not gate on short mode because per-test conditions provide finer-grained control than a blanket integration skip |

### TUI Test Coverage Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| `internal/tui/choose_builders.go` | No dedicated `choose_builders_test.go` | All 29 exported methods are fully covered by `choose_test.go` (`TestChooseBuilder_FluentAPI`, `TestMultiChooseBuilder_FluentAPI`, `TestChooseStringBuilder_FluentAPI`, etc.) |
| `internal/tui/list_styles.go` | No dedicated `list_styles_test.go` | Both functions are unexported; tested indirectly via choose/filter model tests. Direct tests would be circular (asserting color hex == same hex) |
| `internal/tui/theme_colors.go` | No dedicated `theme_colors_test.go` | All 4 symbols are unexported constants; tested indirectly via `modalBaseStyle()` in `embeddable_test.go`. Direct tests would be trivially circular |
| `internal/tui/interactive_unix.go`, `interactive_windows.go` | No test files | Thin syscall wrappers (`unix.IoctlGetWinsize` / Windows API); impractical to unit test without mocking the terminal |

### Error String Matching Exceptions

| Category | Scope | Rationale |
|---|---|---|
| `ValidationErrors` flattening | ~40 occurrences in `pkg/invowkfile/` | `invowkfile.ValidationErrors` flattens sentinel errors into `ValidationError.Message` strings during CUE parsing. `errors.Is()` cannot reach the original sentinel through the error chain. Explicitly noted in `invowkfile_deps_custom_test.go:327`. |
| `Error()` format tests | ~15 occurrences across `cmd/`, `internal/`, `pkg/` | Tests verifying the output of typed error `Error()` methods (e.g., `DependencyError.Error()`, `CapabilityError.Error()`). The string IS the contract. |
| DDD `Invalid*Error` rendering | `internal/tui/`, `internal/tuiserver/`, `pkg/types/` | Tests that `Invalid*Error.Error()` includes the bad input value (e.g., "-5", "bad"). Verifies error message quality. |
| CUE library errors | `pkg/invowkfile/invowkfile_schema_test.go`, `pkg/cueutil/` | Errors from `cuelang.org/go` (e.g., "field not allowed", "conflict") have no sentinel API. |
| Supplementary checks | `internal/app/deps/`, `internal/uroot/`, `tools/goplint/` | Tests that already use `errors.Is()` for the primary assertion; `strings.Contains` verifies additional message content (e.g., flag name, file path, prefix). |
| External/OS errors | `internal/runtime/`, `internal/config/` | Errors from `os`, `strconv`, `exec.LookPath` without sentinel wrapping. |
| Non-empty error checks | `internal/tui/`, `internal/config/` | Tests checking `err.Error() == ""` or `err.Error() != ""` — verifying message exists, not content. |

### Container Exit-Code Stderr Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| `container_exitcode.txtar` | No `stderr` assertions on `! exec` blocks | Exit-code propagation tests verify that non-zero exit codes pass through invowk. The scripts exit intentionally (e.g., `exit 42`); invowk does not render errors to stderr for propagated exit codes. Container stderr may include incidental noise (shell prompt `#`), making `! stderr .` fragile. |

### time.Sleep Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| `internal/testutil/poll_test.go` | `time.Sleep(50ms)` in goroutine | Tests the `PollUntil` polling helper itself; the goroutine simulates delayed readiness. |
| `internal/watch/watcher_test.go:83` | `time.Sleep(20ms)` between writes | Forces separate fsnotify events; macOS kqueue coalesces rapid writes into one event. |
| `internal/watch/watcher_test.go:300` | `time.Sleep(300ms)` in callback | Simulates a slow callback to test serialization behavior (debounce guard). |

### SonarCloud Configuration Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| `sonar-project.properties` | `sonar.go.coverageReportPaths` is intentionally omitted | SonarCloud is installed as a GitHub App using automatic analysis. The App runs its own analysis on PR events — it cannot access CI-generated artifacts like `coverage.out`. Coverage is collected by CI and uploaded as a GitHub Actions artifact for manual inspection, but not fed to SonarCloud. This is documented in `sonar-project.properties` lines 9-11. |
| `sonar-project.properties` | `sonar.tests` omits `tools/` directory | `tools/goplint/` is a separate development tool with its own Go module and quality governance via `lint.yml`. It is explicitly excluded from SonarCloud analysis via `sonar.exclusions=...tools/goplint/**`. The omission from `sonar.tests` is consistent with the exclusion. |

### Test Naming Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| `TestFoo_UnicodeAndLongInputs` (8 TUI test files) | "And" in test name | "And" describes two aspects of a single concern (edge-case input handling), not unrelated subjects. Idiomatic Go test naming for multi-faceted scenarios. |
| `TestFoo_ErrorAndUnwrap` (2 files) | "And" in test name | Tests `Error()` and `Unwrap()` on the same error type — a single concern (error interface compliance). |
| `TestFoo_MixedXAndY` / `_BothXAndY` / `_WithXAndY` (~25 files) | "And" in test name | "And" describes the test fixture or scenario inputs (e.g., "both tools and commands present"), not unrelated test subjects. |

### Platform Skip Exceptions

| Location | What Is Different | Rationale |
|---|---|---|
| `[!container-available] skip` in container txtar | Skips entire test on non-container hosts | Container tests are Linux-only by design |
| `[!net] skip` in network-dependent tests | Skips when no network | Tests requiring external connectivity |
| `[in-sandbox] skip` in sandbox-sensitive tests | Skips in Flatpak/Snap | Sandbox environments restrict filesystem/network access |
| `[windows] skip` with documented OS limitation | Skips genuine Windows limitation | e.g., Unix permission bits, hardcoded `/tmp` in upstream code |

---

## When IS a Deviation a Real Finding?

A deviation from standard patterns becomes a real finding when:

1. **No documented rationale** — The test deviates from the pattern but there is no comment
   explaining why, and it doesn't match any category in this registry.
2. **The rationale is stale** — The original reason for the exception no longer applies
   (e.g., import cycle was resolved, CUE thread safety was fixed upstream).
3. **The exception masks a real issue** — For example, `skipOnWindows` is used to hide
   a missing Windows implementation rather than a genuine platform limitation.
4. **The scope is too broad** — For example, an entire test file skips `t.Parallel()` when
   only one subtest mutates global state (the parent should be serial, but other subtests
   that don't mutate state could be parallelized in a separate test function).

## How to Add Entries

When a review finding is determined to be an intentional exception:

1. Add a row to the appropriate section of the Registry table above.
2. Describe what is different (be specific about the pattern deviation).
3. Explain why it is intentional (the technical or design reason).
4. Mark the original finding as severity **SKIP** with a reference to this entry.
