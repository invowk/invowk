# Tasks

## 1. Shared resolver package

- [ ] 1.1 Create `internal/basedir` (`doc.go`, `basedir.go`) as design D1 describes.
  - API: `Dirs`, `New(goos, getenv, userHomeDir)`, `OS()` (and a zero `Dirs` that behaves as `OS()`), `Home`, `ConfigHome`, `DataHome`, `CacheHome`, `StateHome`, `RuntimeDir`, `HomeFromEnv(goos, getenv)` and `ErrNotAbsolute`.
  - On Windows the AppData fallbacks read `USERPROFILE` through `getenv` and keep S1's two texts (`internal/config/config.go:107`, `:110`).
  - `doc.go` records why this is one shared package and not a copy per domain.
  - It may import only the standard library, `pkg/platform` and `pkg/types`. Verify with `go list -deps ./internal/basedir | grep invowk`, which should list only those packages.
- [ ] 1.2 Add table tests in `internal/basedir/basedir_test.go` (injected `goos`, host-absolute fixtures, `t.Parallel()`). For each kind on linux, darwin and windows, cover:
  - absolute, relative and empty XDG/AppData values, the last two falling back;
  - a relative home and a failing home lookup, both rejected with `ErrNotAbsolute` or the lookup error;
  - an absolute XDG value with a relative home, which succeeds;
  - a relative `XDG_RUNTIME_DIR`, reported unavailable;
  - `HomeFromEnv` per OS.

  Verify that the tests pass.
- [ ] 1.3 Add `internal/basedir/basedir_windows_test.go` for `\Users\me`, `C:Users\me`, relative `APPDATA`/`LOCALAPPDATA`/`USERPROFILE` and an absolute UNC profile (design D4).
  - Locally, `GOOS=windows go test -c -o /dev/null ./internal/basedir` proves only that it compiles.
  - The behavioural proof is the windows-latest CI job on the PR.
- [ ] 1.4 Run `make license-check` on the new files.
- [ ] 1.5 Group check: run `go test ./internal/basedir/` and confirm it passes.

## 2. Config adopts the resolver (no behaviour change)

- [ ] 2.1 Make `configDirFrom` and `commandsDirFrom` (`internal/config/config.go:98-184`) build a `basedir.New(...)` from their existing parameters. Then set `ErrBaseDirNotAbsolute = basedir.ErrNotAbsolute` and delete `absoluteEnv` and `absoluteHomeDir`. Verify that `TestConfigDir` and the `commandsDirFrom` tests in `internal/config/config_paths_test.go` pass with no edits.
- [ ] 2.2 Add the `config-source-resolution` delta's Windows scenarios to `config_paths_test.go`, in a Windows-only test: relative `APPDATA` falls back, and relative `USERPROFILE` gives defaults plus a diagnostic. Verify that they pass on windows-latest CI.
- [ ] 2.3 Group check: run `go test ./internal/config/ ./internal/discovery/` and confirm it passes.

## 3. Module cache and git authentication

- [ ] 3.1 In `internal/app/modulecache/cache.go`:
  - Add `DefaultDirFrom(getenv, dirs basedir.Dirs)`, which rejects a relative `INVOWK_MODULES_PATH` with `ModulesPathNotAbsoluteError` (it wraps `basedir.ErrNotAbsolute`, names the variable and asks for an absolute path), and otherwise takes the default from `dirs.Home()`.
  - Keep `DefaultDirWith(getenv)` and `DefaultDir()` as wrappers that pass `basedir.OS()`.

  Verify with `DefaultDirFrom` tests: relative env rejected, relative home rejected, absolute values unchanged.
- [ ] 3.2 Give `GitFetcher` an unexported `dirs basedir.Dirs` and an injected `getenv`, both set by `NewGitFetcher` (`internal/app/modulesync/git.go:47-51`), plus a test constructor. Make `trySSHAuth` (`git.go:303-326`) use `f.dirs.Home()` and return nil on error. Verify with a unit test: an injected relative home with `.ssh/id_ed25519` in a `t.TempDir()` loads no key.
- [ ] 3.3 Add `(*GitFetcher).requireTrustedSSHFiles(gitURL)`, which returns `ErrSSHTrustFilesNotAbsolute` (design D2, Git).
  - Call it first in `ListVersions`, `Fetch`, `GetCommitForTag`, `CloneShallow`, `IsPrivateRepo` (return true without connecting) and `ListTagsWithCommits` (`git.go:59-252`).
  - For every URL that does not start with `https://`, it requires an absolute `Home()` and absolute `SSH_KNOWN_HOSTS` entries.

  Verify with unit tests that use an unroutable endpoint (`ssh://git@192.0.2.1/x.git`, TEST-NET-1) and a short context. The relative-home and relative-`SSH_KNOWN_HOSTS` cases return the typed error before any dial: the error must be the typed one, not a timeout. An `https://` URL with a relative home passes the gate.
- [ ] 3.4 Add the testscript `tests/cli/testdata/module_cache_relative_path.txtar`:
  - set `env INVOWK_MODULES_PATH=.modules`, then run `invowk module sync` in a directory that has an `invowkmod.cue`;
  - assert a non-zero exit, stderr naming `INVOWK_MODULES_PATH` and "absolute", and `! exists .modules`.

  It is not virtual, so it needs no mirror.
- [ ] 3.5 Update `INVOWK_MODULES_PATH` in three places (the value must be absolute): `README.md:2241`, `website/docs/modules/dependencies/cli-commands.mdx:238` and `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/modules/dependencies/cli-commands.mdx:229`. Verify that the text matches the spec.
- [ ] 3.6 Group check: run `go test ./internal/app/modulecache/ ./internal/app/modulesync/ && go test ./tests/cli/ -run 'TestCLI/module_cache_relative_path'` and confirm it passes.

## 4. Provisioning, run locks and test infrastructure

- [ ] 4.1 Add `provision.Config.BaseDirs basedir.Dirs` and a `WithBaseDirs` option beside `WithCacheDir` (`internal/provision/config.go:246-251`; the zero value means `basedir.OS()`). Make `applyHostProvisionDefaults` (`internal/runtime/container_provision.go:232-255`, called once at `:190`) read the home from `provisionCfg.BaseDirs`.
  - It sets the default `CacheDir` only when `Home()` is absolute and is an existing directory, and otherwise leaves it empty (design decision 10).
  - Verify with unit tests for a relative home and for an absolute missing home (`filepath.Join(t.TempDir(), "missing")`): both leave `CacheDir` empty, and an existing absolute home gives `<home>/.cache/invowk/provision`.
- [ ] 4.2 Update `resolveBuildContextParent`, which takes the home from `p.config.BaseDirs` (`internal/provision/layer_provisioner.go:367-408`):
  - use `<home>/invowk-build` only for an absolute, existing `Home()`;
  - delete the `<cwd>/.invowk-build` branch (`:385-391`);
  - update the doc comment at `:277-282`.

  Verify with unit tests: a relative home and an absolute missing home both give an `invowk-build-*` parent under `os.TempDir()` with a non-nil cleanup, and nothing is created in the test's working directory. Use `t.Chdir(t.TempDir())` and no `t.Parallel()` in the test that checks the working directory.
- [ ] 4.3 Make `lockFilePathWith` (`internal/container/run_lock_linux.go:80-86`) and `containerSuiteLockFilePath` (`internal/testutil/container_suite_lock_linux.go:51-57`) use `RuntimeDir()` from a `basedir.New` value built over the given `getenv`, then `os.TempDir()`. Verify with a unit test of `lockFilePathWith` with `XDG_RUNTIME_DIR=run`.
- [ ] 4.4 Make `ContainerSafeTempDir` (`internal/testutil/testutil.go:148`) use `basedir.OS().Home()`. Make `containerSuiteConfigDir` (`tests/cli/container_harness.go:363-375`) use `basedir.New(runtime.GOOS, env.Getenv, …HomeFromEnv…)` and `ConfigHome()` (design D2). Add `TestContainerSuiteConfigDir` in `tests/cli/container_harness_test.go`. It builds a testscript env with a relative `XDG_CONFIG_HOME` and expects `<HOME>/.config/invowk` on Linux. Verify that `go test ./internal/testutil/ && go test ./tests/cli/ -run 'TestContainerSuiteConfigDir|TestDecideContainerSuiteHarness'` passes.
- [ ] 4.5 Group check: run `go test ./internal/provision/ ./internal/container/ ./internal/runtime/ -run 'Provision|Lock|BuildContext'` and confirm it passes.

## 5. Virtual-runtime anchors

- [ ] 5.1 Change `standardVirtualAnchorsForOS` (`internal/runtime/virtual_policy.go:148-186`) to `(goos, workDir, tempDir string, dirs basedir.Dirs)`, keeping its name because the formal table binds it.
  - Build each anchor from the matching `basedir` method, and use `""` for an unavailable base.
  - Make `standardVirtualAnchors` (`:140-146`) pass `basedir.OS()`, and drop `firstNonEmpty` (`:188-193`).

  Verify by updating `internal/runtime/virtual_policy_test.go:66` and adding cases for relative XDG values, relative `LOCALAPPDATA` on Windows and a relative home.
- [ ] 5.2 In `expandVirtualAnchorPath` (`virtual_policy.go:284-297`), report a known but empty anchor as `virtual path anchor %q is unavailable`, and keep "unknown virtual path anchor" for unknown names. Verify with a unit test covering both, and with a resolver test showing that a `filesystem.paths` entry `@home/.dotfiles` with an unavailable `@home` fails construction.
- [ ] 5.3 Add the testscript `tests/cli/testdata/virtual_anchor_relative_home.txtar`, split by platform:
  - on Linux and macOS, `env HOME=.` with the `XDG_*` variables cleared;
  - on Windows, `env USERPROFILE=. APPDATA= LOCALAPPDATA=`;
  - a `virtual-sh` command prints `[$INVOWK_ANCHOR_CONFIG]` and the test expects `[]`;
  - a `virtual-lua` `invowk.path("@config")` fails with "unavailable".

  Add `"virtual_anchor_relative_home.txtar": "path anchors are injected only by the virtual runtimes"` to `exact` in `tests/cli/runtime_mirror_exemptions.json` (design decision 12).
- [ ] 5.4 Update the anchor text in `website/docs/runtime-modes/virtual-lua.mdx:21,31` and its pt-BR `current` copy: relative bases are ignored, and home-derived anchors can be unavailable. `virtual.mdx` has no anchor section today, so it is not changed. Do not describe the virtual runtimes as sandboxed.
- [ ] 5.5 Group check: run `go test ./internal/runtime/ -run 'Anchor|VirtualPath' && go test ./tests/cli/ -run 'TestCLI/virtual_anchor_relative_home'` and confirm it passes.
- [ ] 5.6 Formal binding (`.agents/rules/formal-bindings.md`):
  - update the `anchors` row's abstraction note in `formal/alloy/VirtualPathHarness.als:31` if the semantics it names changed;
  - run `python3 scripts/formal.py correspondence` and `go test ./internal/runtime/ -run TestVirtualPathHarness`;
  - run `make formal` when Java is available.

## 6. Mechanical guard

- [ ] 6.1 Add forbidigo text patterns `^os\.UserHomeDir$`, `^os\.UserConfigDir$` and `^os\.UserCacheDir$` to `.golangci.toml:422-426`, without `analyze-types`, plus a `[[linters.exclusions.rules]]` entry for `internal/basedir/` (design D3). Verify that `make lint` is clean, and that a scratch `os.UserHomeDir()` call in `internal/app` is reported and then removed.
- [ ] 6.2 Add `internal/basedir/guard_test.go`, the AST walk from design D3.
  - It covers: `os` imported under any name, `os/user` imports, exact literals, `^XDG_[A-Z_]+$` and `$HOME`/`${HOME}` substrings.
  - It reports file:line, and has a reasoned allowlist (initially `internal/testutil/home.go`).
  - Factor the walk as `scan(root string) []finding`.

  Verify that the test passes on the tree, and that `scan` reports each pattern, including an aliased `goos.UserHomeDir()`, on fixture files written to `t.TempDir()`.
- [ ] 6.3 Document the helper and the guard where home-directory handling is already covered: `.agents/skills/windows-testing/SKILL.md` (the `os.UserHomeDir`/`SetHomeDir` notes), `.agents/skills/go/SKILL.md` (including "review new dependencies that read per-user files"), and `.agents/rules/testing.md`. Verify with `make check-agent-docs`.
- [ ] 6.4 Group check: run `go test ./internal/basedir/ -run Guard && make lint` and confirm it passes.

## 7. Completion gates (`.agents/rules/checklist.md`)

- [ ] 7.1 Review the diff for reuse and simplification (`/simplify`), then run `make tidy` and `make license-check`.
- [ ] 7.2 Run `make lint` and fix every issue.
- [ ] 7.3 Run `make test` (full suite, including CLI and container tests).
- [ ] 7.4 Run `make check-baseline` and triage every new goplint finding. Never baseline protocol inconclusives.
- [ ] 7.5 Run `make check-file-length` and `make sonar-local`, and keep the suppressions in both Sonar files in sync.
- [ ] 7.6 Conditional checks:
  - `cd website && npm run build` (docs changed);
  - `make check-agent-docs` (agent docs changed);
  - `make check-windows-build`;
  - `python3 scripts/formal.py correspondence` (formal-bound `internal/runtime`, `internal/provision`, `internal/app/modulesync` and `internal/app/modulecache` changed);
  - `for m in samples/invowkmods/*.invowkmod; do go run . validate "$m"; done` (module cache change).
- [ ] 7.7 Confirm that the virtual/native mirror checks pass (`TestShRuntimeMirrorCoverage`, `TestVirtualNativeCommandPathAlignment`), and run `/learn`.
- [ ] 7.8 Run `openspec validate require-absolute-base-dirs --strict` and check for 0 warnings.
