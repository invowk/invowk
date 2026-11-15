# Tasks

## 1. Shared resolver package

- [x] 1.1 (2026-11-15: `internal/basedir/{doc.go,basedir.go}`; `go list -deps` shows only `pkg/platform` and `pkg/types`.) Create `internal/basedir` (`doc.go`, `basedir.go`) as design D1 describes.
  - API: `Dirs`, `New(goos, getenv, userHomeDir)`, `OS()` (and a zero `Dirs` that behaves as `OS()`), `Home`, `ConfigHome`, `DataHome`, `CacheHome`, `StateHome`, `RuntimeDir`, `HomeFromEnv(goos, getenv)` and `ErrNotAbsolute`.
  - On Windows the AppData fallbacks read `USERPROFILE` through `getenv` and keep S1's two texts (`internal/config/config.go:107`, `:110`).
  - `doc.go` records why this is one shared package and not a copy per domain.
  - It may import only the standard library, `pkg/platform` and `pkg/types`. Verify with `go list -deps ./internal/basedir | grep invowk`, which should list only those packages.
- [x] 1.2 (2026-11-15: `TestBaseDirs`, `TestHome`, `TestRuntimeDir`, `TestHomeFromEnv`, `TestZeroDirsBehavesAsOS` pass.) Add table tests in `internal/basedir/basedir_test.go` (injected `goos`, host-absolute fixtures, `t.Parallel()`). For each kind on linux, darwin and windows, cover:
  - absolute, relative and empty XDG/AppData values, the last two falling back;
  - a relative home and a failing home lookup, both rejected with `ErrNotAbsolute` or the lookup error;
  - an absolute XDG value with a relative home, which succeeds;
  - a relative `XDG_RUNTIME_DIR`, reported unavailable;
  - `HomeFromEnv` per OS.

  Verify that the tests pass.
- [x] 1.3 (2026-11-15: `TestWindowsPathForms`; only the `GOOS=windows go test -c` compile was proven locally, the behavioural proof is the windows-latest CI job.) Add `internal/basedir/basedir_windows_test.go` for `\Users\me`, `C:Users\me`, relative `APPDATA`/`LOCALAPPDATA`/`USERPROFILE` and an absolute UNC profile (design D4).
  - Locally, `GOOS=windows go test -c -o /dev/null ./internal/basedir` proves only that it compiles.
  - The behavioural proof is the windows-latest CI job on the PR.
- [x] 1.4 (2026-11-15: `make license-check` clean.) Run `make license-check` on the new files.
- [x] 1.5 (2026-11-15: `go test ./internal/basedir/` passes.) Group check: run `go test ./internal/basedir/` and confirm it passes.

## 2. Config adopts the resolver (no behaviour change)

- [x] 2.1 (2026-11-15: `absoluteEnv`, `absoluteHomeDir` and `errMsgHomeDir` deleted; `ConfigDir`/`CommandsDir` pass a nil home lookup, which the resolver maps to `os.UserHomeDir`; `TestConfigDir` and `TestCommandsDirFrom` pass unedited.) Make `configDirFrom` and `commandsDirFrom` (`internal/config/config.go:98-184`) build a `basedir.New(...)` from their existing parameters. Then set `ErrBaseDirNotAbsolute = basedir.ErrNotAbsolute` and delete `absoluteEnv` and `absoluteHomeDir`. Verify that `TestConfigDir` and the `commandsDirFrom` tests in `internal/config/config_paths_test.go` pass with no edits.
- [x] 2.2 (2026-11-15: `internal/config/config_paths_windows_test.go`; the defaults-plus-diagnostic step is `commandsvc.LoadConfigWithFallback`, already covered by `TestLoadConfigWithFallback`. Compile-checked locally only; behaviour proven on windows-latest CI.) Add the `config-source-resolution` delta's Windows scenarios to `config_paths_test.go`, in a Windows-only test: relative `APPDATA` falls back, and relative `USERPROFILE` gives defaults plus a diagnostic. Verify that they pass on windows-latest CI.
- [x] 2.3 (2026-11-15: passes.) Group check: run `go test ./internal/config/ ./internal/discovery/` and confirm it passes.

## 3. Module cache and git authentication

- [x] 3.1 (2026-11-15: `DefaultDirFrom`, `ModulesPathNotAbsoluteError`; `TestDefaultDirFrom` passes.) In `internal/app/modulecache/cache.go`:
  - Add `DefaultDirFrom(getenv, dirs basedir.Dirs)`, which rejects a relative `INVOWK_MODULES_PATH` with `ModulesPathNotAbsoluteError` (it wraps `basedir.ErrNotAbsolute`, names the variable and asks for an absolute path), and otherwise takes the default from `dirs.Home()`.
  - Keep `DefaultDirWith(getenv)` and `DefaultDir()` as wrappers that pass `basedir.OS()`.

  Verify with `DefaultDirFrom` tests: relative env rejected, relative home rejected, absolute values unchanged.
- [x] 3.2 (2026-11-15: `newGitFetcherWith` is the test constructor; `TestTrySSHAuthIgnoresRelativeHome` also shows the same key loads under an absolute home.) Give `GitFetcher` an unexported `dirs basedir.Dirs` and an injected `getenv`, both set by `NewGitFetcher` (`internal/app/modulesync/git.go`), plus a test constructor. Make `trySSHAuth` use `f.dirs.Home()` and return nil on error. Verify with a unit test: an injected relative home with `.ssh/id_ed25519` in a `t.TempDir()` loads no key.
- [x] 3.3 (2026-11-15: `SSHTrustFilesError` wraps `ErrSSHTrustFilesNotAbsolute` plus the cause; `TestRequireTrustedSSHFiles` and `TestSSHGateRunsBeforeDial` pass.) Add `(*GitFetcher).requireTrustedSSHFiles(gitURL)`, which returns `ErrSSHTrustFilesNotAbsolute` (design D2, Git).
  - Call it first in `ListVersions`, `Fetch`, `GetCommitForTag`, `CloneShallow`, `IsPrivateRepo` (return true without connecting) and `ListTagsWithCommits` (all in `git.go`).
  - For every URL that does not start with `https://`, it requires `Home()` to succeed with an absolute path (an unavailable home is refused too, decision 18) and every non-empty `SSH_KNOWN_HOSTS` entry to be absolute.

  Verify with unit tests that use an unroutable endpoint (`ssh://git@192.0.2.1/x.git`, TEST-NET-1) and a short context. The relative-home, unavailable-home and relative-`SSH_KNOWN_HOSTS` cases return the typed error before any dial: the error must be the typed one, not a timeout. An `https://` URL with a relative home passes the gate.
- [x] 3.4 (2026-11-15: passes; the fixture declares an `https://` requirement so the cache dir is resolved before any git call.) Add the testscript `tests/cli/testdata/module_cache_relative_path.txtar`:
  - set `env INVOWK_MODULES_PATH=.modules`, then run `invowk module sync` in a directory that has an `invowkmod.cue`;
  - assert a non-zero exit, stderr naming `INVOWK_MODULES_PATH` and "absolute", and `! exists .modules`.

  It is not virtual, so it needs no mirror.
- [x] 3.5 (2026-11-15: README also notes the SSH-transport rule.) Update `INVOWK_MODULES_PATH` in three places (the value must be absolute): `README.md:2241`, `website/docs/modules/dependencies/cli-commands.mdx:238` and `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/modules/dependencies/cli-commands.mdx:229`. Verify that the text matches the spec.
- [x] 3.6 (2026-11-15: both pass; the unset-home SSH rows assert the BREAKING refusal.) Zero-change invariant (design D5 rows e and f): add `TestDefaultDirFromMatchesLegacy` in `internal/app/modulecache/` and `TestSSHGateMatchesLegacy` in `internal/app/modulesync/`. Each freezes today's behaviour as a legacy oracle in the `_test.go` file and compares over a table of absolute and unset environments (absolute existing home, absolute missing home, home unset, `INVOWK_MODULES_PATH` absolute and unset, `SSH_KNOWN_HOSTS` absolute and unset). For the gate, the oracle is "today proceeds to the transport" except where today fails at host-key verification; the unset-home SSH row is listed as the BREAKING expected difference.
- [x] 3.7 (2026-11-15: passes.) Group check: run `go test ./internal/app/modulecache/ ./internal/app/modulesync/ && go test ./tests/cli/ -run 'TestCLI/module_cache_relative_path'` and confirm it passes.

## 4. Provisioning, run locks and test infrastructure

- [x] 4.1 (2026-11-15: `TestApplyHostProvisionDefaultsCacheDir` passes; no existence check, per decision 15.) Add `provision.Config.BaseDirs basedir.Dirs` and a `WithBaseDirs` option beside `WithCacheDir` (`internal/provision/config.go:246-251`; the zero value means `basedir.OS()`). Make `applyHostProvisionDefaults` (`internal/runtime/container_provision.go:232-255`, called once at `:190`) read the home from `provisionCfg.BaseDirs`.
  - It sets the default `CacheDir` whenever `Home()` succeeds with an absolute path, whether or not it exists, and otherwise leaves it empty (design decision 15, which supersedes decision 10).
  - Verify with unit tests: a relative home and a failing home lookup leave `CacheDir` empty; an absolute missing home (`filepath.Join(t.TempDir(), "missing")`) and an existing absolute home both give `<home>/.cache/invowk/provision`.
- [x] 4.2 (2026-11-15: `TestResolveBuildContextParentNeverUsesCwd` passes; the doc comment moved to `prepareBuildContext`, where the Snap note lives.) Update `resolveBuildContextParent`, which takes the home from `p.config.BaseDirs` (`internal/provision/layer_provisioner.go:367-408`):
  - use `<home>/invowk-build` only for an absolute, existing `Home()`;
  - delete the `<cwd>/.invowk-build` branch (`:385-391`);
  - update the doc comment at `:277-282`.

  Verify with unit tests: a relative home and an absolute missing home both give an `invowk-build-*` parent under `os.TempDir()` with a non-nil cleanup, and nothing is created in the test's working directory. Use `t.Chdir(t.TempDir())` and no `t.Parallel()` in the test that checks the working directory.
- [x] 4.3 (2026-11-15: `TestLockFilePath_IgnoresRelativeXDGRuntimeDir` passes; the testutil suite lock uses `basedir.OS().RuntimeDir()`.) Make `lockFilePathWith` (`internal/container/run_lock_linux.go:80-86`) and `containerSuiteLockFilePath` (`internal/testutil/container_suite_lock_linux.go:51-57`) use `RuntimeDir()` from a `basedir.New` value built over the given `getenv`, then `os.TempDir()`. Verify with a unit test of `lockFilePathWith` with `XDG_RUNTIME_DIR=run`.
- [x] 4.4 (2026-11-15: `containerSuiteConfigDir` now returns an error for a relative home; `TestContainerSuiteConfigDir` added in `tests/cli/container_harness_basedir_test.go` to keep `container_harness_test.go` short; both checks pass.) Make `ContainerSafeTempDir` (`internal/testutil/testutil.go:148`) use `basedir.OS().Home()`. Make `containerSuiteConfigDir` (`tests/cli/container_harness.go:363-375`) use `basedir.New(runtime.GOOS, env.Getenv, …HomeFromEnv…)` and `ConfigHome()` (design D2). Add `TestContainerSuiteConfigDir` in `tests/cli/container_harness_test.go`. It builds a testscript env with a relative `XDG_CONFIG_HOME` and expects `<HOME>/.config/invowk` on Linux. Verify that `go test ./internal/testutil/ && go test ./tests/cli/ -run 'TestContainerSuiteConfigDir|TestDecideContainerSuiteHarness'` passes.
- [x] 4.5 (2026-11-15: `TestDefaultCacheDirMatchesLegacy`, `TestResolveBuildContextParentMatchesLegacy` and `TestLockFilePathMatchesLegacy` pass; the legacy cwd rows assert a temp parent.) Zero-change invariant (design D5 rows b, c and d): add legacy-oracle table tests for the default `CacheDir` (`internal/runtime`), the build-context parent kind (`internal/provision`: cache, `<home>/invowk-build`, temp or cwd) and `lockFilePathWith` (`internal/container`), over absolute existing home, absolute missing home, home unset, and `XDG_RUNTIME_DIR` absolute and unset. The rows where today's oracle picks `<cwd>/.invowk-build` are the BREAKING expected differences (now temp); every other row must match.
- [x] 4.6 (2026-11-15: passes.) Group check: run `go test ./internal/provision/ ./internal/container/ ./internal/runtime/ -run 'Provision|Lock|BuildContext|Legacy'` and confirm it passes.

## 5. Virtual-runtime anchors

- [x] 5.1 (2026-11-15: signature `(goos, workDir, tempDir string, dirs basedir.Dirs)`; `firstNonEmpty` removed; the existing cases were moved to host-absolute fixtures, because the old Windows case used `C:/Users/...`, which is relative on a Linux host.) Change `standardVirtualAnchorsForOS` (`internal/runtime/virtual_policy.go:148-186`) to `(goos, workDir, tempDir string, dirs basedir.Dirs)`, keeping its name because the formal table binds it.
  - Build each anchor from the matching `basedir` method, and use `""` for an unavailable base.
  - Make `standardVirtualAnchors` (`:140-146`) pass `basedir.OS()`, and drop `firstNonEmpty` (`:188-193`).

  - Keep F1's contract (design decision 16): `newVirtualPathResolverForFilesystem` keeps its signature, and with an absolute home every anchor stays non-empty with today's path.

  Verify by updating `internal/runtime/virtual_policy_test.go:66` and adding cases for relative XDG values, relative `LOCALAPPDATA` on Windows and a relative home.
- [x] 5.2 (2026-11-15: `TestExpandVirtualAnchorPathUnavailable` and `TestVirtualPathMappingOnUnavailableAnchorFails` pass; the message is `virtual path anchor "@home" is unavailable: its base directory is relative or unset`.) In `expandVirtualAnchorPath` (`virtual_policy.go:284-297`), report a known but empty anchor as `virtual path anchor %q is unavailable`, and keep "unknown virtual path anchor" for unknown names. Verify with a unit test covering both, and with a resolver test showing that a `filesystem.paths` entry `@home/.dotfiles` with an unavailable `@home` fails construction.
- [x] 5.3 (2026-11-15: passes on Linux; the `[windows]` branch runs on windows-latest CI only.) Add the testscript `tests/cli/testdata/virtual_anchor_relative_home.txtar`, split by platform:
  - on Linux and macOS, `env HOME=.` with the `XDG_*` variables cleared;
  - on Windows, `env USERPROFILE=. APPDATA= LOCALAPPDATA=`;
  - a `virtual-sh` command prints `[$INVOWK_ANCHOR_CONFIG]` and the test expects `[]`;
  - a `virtual-lua` `invowk.path("@config")` fails with "unavailable".

  Add `"virtual_anchor_relative_home.txtar": "path anchors are injected only by the virtual runtimes"` to `exact` in `tests/cli/runtime_mirror_exemptions.json` (design decision 12).
- [x] 5.4 (2026-11-15: added a paragraph after the "Paths And Files" roots sentence (line 31) in both locales; the line-21 table entry needed no change.) Update the anchor text in `website/docs/runtime-modes/virtual-lua.mdx:21,31` and its pt-BR `current` copy: relative bases are ignored, and home-derived anchors can be unavailable. `virtual.mdx` has no anchor section today, so it is not changed. Do not describe the virtual runtimes as sandboxed.
- [x] 5.5 (2026-11-15: `TestStandardVirtualAnchorsMatchLegacy` passes over every subset of absolute base variables.) Zero-change invariant (design D5 row a): add `TestStandardVirtualAnchorsMatchLegacy` in `internal/runtime/`, with today's `standardVirtualAnchorsForOS` frozen as the oracle. Table over linux, darwin and windows `goos`, with the home absolute and unset and each XDG/AppData variable absolute and unset. The home-unset rows where today's oracle returns a relative home-derived anchor are the BREAKING expected differences (now `""`); every other anchor must match.
- [x] 5.6 (2026-11-15: passes.) Group check: run `go test ./internal/runtime/ -run 'Anchor|VirtualPath|Legacy' && go test ./tests/cli/ -run 'TestCLI/virtual_anchor_relative_home'` and confirm it passes.
- [x] 5.7 (2026-11-15: the `anchors` row abstraction is "-" and names no changed semantics, so the `.als` is unchanged (F1 rewrites that row); `python3 scripts/formal.py correspondence` and `TestVirtualPathHarness*` pass; `make formal` passed with Java.) Formal binding (`.agents/rules/formal-bindings.md`):
  - update the `anchors` row's abstraction note in `formal/alloy/VirtualPathHarness.als:31` if the semantics it names changed;
  - run `python3 scripts/formal.py correspondence` and `go test ./internal/runtime/ -run TestVirtualPathHarness`;
  - run `make formal` when Java is available.

## 6. Mechanical guard

- [x] 6.1 (2026-11-15: a scratch `os.UserHomeDir()` in `internal/app` was reported and removed; the exclusion is limited to `internal/basedir/` and these three messages, so `errors.As` stays enforced there; `config.ConfigDir`/`CommandsDir` now pass a nil lookup, which the resolver maps to the real home.) Add forbidigo text patterns `^os\.UserHomeDir$`, `^os\.UserConfigDir$` and `^os\.UserCacheDir$` to `.golangci.toml:422-426`, without `analyze-types`, plus a `[[linters.exclusions.rules]]` entry for `internal/basedir/` (design D3). Verify that `make lint` is clean, and that a scratch `os.UserHomeDir()` call in `internal/app` is reported and then removed.
- [x] 6.2 (2026-11-15: `TestBaseDirGuard` and `TestBaseDirGuardPlantedViolations` pass; the fixture marks each planted line with `// want: <kind>` and also holds a test file, a nested `testdata/`, an allowlisted file, an `internal/basedir/` file and an out-of-scope `docs/` file that must not be reported; a scratch aliased-`os` `XDG_DATA_HOME` literal in `internal/app` failed the real-tree guard.) Add `internal/basedir/guard_test.go`, the AST walk from design D3.
  - It covers: `os` imported under any name, including a dot import (bare `UserHomeDir` identifiers), `os/user` imports, exact literals, `^XDG_[A-Z_]+$` and `$HOME`/`${HOME}` substrings.
  - It reports file:line, and has a reasoned allowlist (initially `internal/testutil/home.go`).
  - Factor the walk as `scan(root string) []finding`, with the `testdata` and `internal/basedir/` skips matched relative to `root`.
  - Add the committed planted-violation fixture `internal/basedir/testdata/guardfixture/` (design decision 17): `internal/planted/*.go` with one violation per pattern, plus `internal/testutil/home.go` (allowlisted) and `internal/basedir/x.go` (skipped). Fixture files carry the SPDX header and are gofmt-clean.

  Verify that the test passes on the tree, and that `scan("testdata/guardfixture")` returns exactly the expected file:line findings, one per planted pattern, and none for the allowlisted and skipped fixture files.
- [x] 6.3 (2026-11-15: windows-testing, go skill "Per-User Directories" and the testing rule guardrail list updated; `make check-agent-docs` passes.) Document the helper and the guard where home-directory handling is already covered: `.agents/skills/windows-testing/SKILL.md` (the `os.UserHomeDir`/`SetHomeDir` notes), `.agents/skills/go/SKILL.md` (including "review new dependencies that read per-user files"), and `.agents/rules/testing.md`. Verify with `make check-agent-docs`.
- [x] 6.4 (2026-11-15: exception added under "Utility Package Boundaries"; `make check-agent-docs` passes.) Add the explicit `internal/basedir` exception to `.agents/rules/package-design.md` "Utility Package Boundaries" (design decision 14), next to the "each gets its own copy" bullet, with its reason and a pointer to the guard. Verify with `make check-agent-docs`.
- [x] 6.5 (2026-11-15: passes after the 6b guard extension.) Group check: run `go test ./internal/basedir/ -run Guard && make lint` and confirm it passes.

## 6a. Stage 4a: `HOME`-unset checks and Windows CI (2026-11-15)

- [x] 6a.1 (2026-11-15: done; S4's tracker items are now N8, N9 and N10 after main's N6 and S3's N7.) Rebase on origin/main (#174 and C1) and renumber S4's tracker items in `.claude/issues/cst-adoption-gaps.md`, `design.md` and the PR body.
- [x] 6a.2 (2026-11-15: the only Windows failure on `aa3682e2` was `TestGetDefaultCacheDir/with_env_var`, whose `/custom/path/to/modules` has no volume and is relative on Windows; it now uses a `t.TempDir()` path.) Fix the Windows-only failures of PR #172's `Test (windows)` job and get it green on CI.
- [x] 6a.3 (2026-11-15: recorded in design Context and decision 19; the migration notes name the unit-level fixes.) Check what `HOME` systemd gives user and system units.
- [x] 6a.4 (2026-11-15: `virtual_home_unset.txtar`, `native_home_unset.txtar` and `container_home_unset.txtar` pass on Linux, decision 21.) Add tests that plain native, virtual and container commands run with `HOME` unset.
- [x] 6a.5 (2026-11-15: reproduced, decision 20; the SSH row stays BREAKING and the gate keeps refusing an unavailable home.) Reproduce the cwd-relative `.ssh/config` read on origin/main with `HOME` unset.

## 6b. Stage 4a review fixes (2026-11-15)

- [x] 6b.1 (2026-11-15: done, decision 22; the planted fixture failed before the scan change and passes after; moving the help text left `invowk --help` and `invowk config --help` byte-identical.) Extend the guard (spec first): `os.ExpandEnv`/`os.Expand`, helper-library imports, case-insensitive Windows names, `NAME=` prefixes, the `XDG_` prefix, `$NAME`/`${NAME}`/`%NAME%` references, and a whole-module scan with `vendor`, `node_modules`, hidden and nested-module skips. Move the help strings that name `$XDG_CONFIG_HOME`/`%APPDATA%` to an allowlisted `cmd/invowk/config_paths_help.go`. Plant each new pattern in `testdata/guardfixture`.
- [x] 6b.2 (2026-11-15: done.) Document the SSH absolute-home and absolute-`SSH_KNOWN_HOSTS` requirement under "SSH Keys" in `website/docs/modules/dependencies/declaring-dependencies.mdx` and its pt-BR copy.
- [x] 6b.3 (2026-11-15: done.) Replace `git.go` line numbers with function names in the proposal, design, tasks and tracker; restore the tracker's original merge-order line and add the v0.17.0 order as its own line.
- [x] 6b.4 (2026-11-15: done, decision 23.) Record the Windows `/etc/ssh/ssh_known_hosts` default as a known limitation in the SSH requirement and fold the fix into N10.
- [x] 6b.5 (2026-11-15: done, decisions 24 and 25.) Close the spec test gaps: `module_sync_ssh_untrusted_files.txtar` (relative, unset home and relative `SSH_KNOWN_HOSTS`, planted `./.ssh/` files), `TestHTTPSAuthWithRelativeHome`, `virtual_anchor_relative_xdg.txtar` (config path and `@config` agree; mirror exemption), `module_cache_relative_home.txtar`, `TestWithRunLock_RelativeXDGRuntimeDirCreatesNoCwdLock`, and the config legacy oracle `TestConfigPathsMatchLegacy`.

## 7. Completion gates (`.agents/rules/checklist.md`)

- [x] 7.0 (2026-11-15: rebased on `d7e4feb4` (F1 #163 and S3 #173 merged); tracker conflict resolved with S3 = N7, S4 = N8-N10, the capture defect = N11 (decisions 26, 30). `TestVirtualPathHarness*`, `python3 scripts/formal.py correspondence`, `make formal-reach` (11.5 s, budget 36 s), `formal_mutation.py triage --check` (419 survivors, 19 closed) and the basedir guard pass on the rebased tree. A report-only `make mutation-formal` in a throwaway worktree: the new body has 14 mutants, 9 killed and 5 escaped, all in the `windows` case; the 22 ledger entries mutate the removed body. Recorded in `tasks/next/formal-binding-gaps.md`; ledger and baseline unchanged (F1 D9).) Wait for F1 (`harden-formal-instruments`, #163) to merge, then rebase this branch on main (design decision 16). Re-run `go test ./internal/runtime/ -run 'TestVirtualPathHarness_RootConstruction|TestVirtualPathHarness'` and `python3 scripts/formal.py correspondence`, and re-check the guard against any new non-test files that the rebase brings in. Re-triage the formal-bindings mutation survivors of the rewritten `standardVirtualAnchorsForOS` (`tools/mutation/triage/formal-bindings.toml`, which names mutants of the removed `getenv`/`firstNonEmpty` body, and F1's statement and survivor counts for the anchors row), after committing (`make test-scripts` restores the worktree).
- [x] 7.1 (2026-11-15: single-pass `/simplify` (no Agent fan-out); extracted `Dirs.underHome`; skipped the run-lock `XDG_RUNTIME_DIR`-or-temp duplication across `internal/container` and `internal/testutil`; `make tidy`, `make license-check` pass.) Review the diff for reuse and simplification (`/simplify`), then run `make tidy` and `make license-check`.
- [x] 7.2 (2026-11-15: `make lint`: 0 issues.) Run `make lint` and fix every issue.
- [x] 7.3 (2026-11-15: `make test` exit 0: 9205 tests, 25 skipped, CLI 202 tests. `TestSyncFreshCacheReproducesLockedHash` failed once in the first pass and passed in the re-run; a known flake, recurrence recorded in the tracker; 200 isolated runs pass.) Run `make test` (full suite, including CLI and container tests).
- [x] 7.4 (2026-11-15: `make check-baseline` exit 0, no new findings; `make check-goplint-consumer-routed` passes.) Run `make check-baseline` and triage every new goplint finding. Never baseline protocol inconclusives.
- [ ] 7.5 Run `make check-file-length` and `make sonar-local`, and keep the suppressions in both Sonar files in sync.
- [x] 7.6 (2026-11-15: website `npm run build` OK (en, pt-BR); `make check-agent-docs` OK; `make check-windows-build` OK; correspondence OK; the three sample modules validate; schema sync tests pass.) Conditional checks:
  - `cd website && npm run build` (docs changed);
  - `make check-agent-docs` (agent docs changed);
  - `make check-windows-build`;
  - `python3 scripts/formal.py correspondence` (formal-bound `internal/runtime`, `internal/provision`, `internal/app/modulesync` and `internal/app/modulecache` changed);
  - `for m in samples/invowkmods/*.invowkmod; do go run . validate "$m"; done` (module cache change).
- [x] 7.7 (2026-11-15: both mirror checks pass inside `make test`; `/learn` added the report-only survivor retriage procedure to the formal-verification skill.) Confirm that the virtual/native mirror checks pass (`TestShRuntimeMirrorCoverage`, `TestVirtualNativeCommandPathAlignment`), and run `/learn`.
- [x] 7.8 (2026-11-15: valid, 0 warnings; delta headers match the current main specs.) Run `openspec validate require-absolute-base-dirs --strict` and check for 0 warnings.
