# Tasks

## 1. Shared resolver package

- [ ] 1.1 Create `internal/basedir` (`doc.go`, `basedir.go`) with `New(goos, getenv, userHomeDir)`, `OS()`, `Home`, `ConfigHome`, `DataHome`, `CacheHome`, `StateHome`, `RuntimeDir`, the getenv-based home lookup for harnesses, and `ErrNotAbsolute`. It may import only the standard library, `pkg/platform` and `pkg/types` (design D1). Verify with `go list -deps ./internal/basedir | grep invowk`, which should list only those packages.
- [ ] 1.2 Add table tests in `internal/basedir/basedir_test.go` for each kind on linux, darwin and windows (injected `goos`, host-absolute fixtures). Cover absolute, relative and empty XDG/AppData values falling back, relative and failing home rejected, and absolute XDG with relative home succeeding. Verify that the tests pass.
- [ ] 1.3 Add `internal/basedir/basedir_windows_test.go` (or a test skipped unless `runtime.GOOS == "windows"`) for `\Users\me`, `C:Users\me`, relative `APPDATA`/`LOCALAPPDATA`/`USERPROFILE`, and an absolute UNC profile (design D4). Verify by cross-compiling it with `GOOS=windows go test -c -o /dev/null ./internal/basedir`.
- [ ] 1.4 Run `make license-check` on the new files.
- [ ] 1.5 Group check: run `go test ./internal/basedir/` and confirm it passes.

## 2. Config adopts the resolver (no behaviour change)

- [ ] 2.1 Make `configDirFrom` and `commandsDirFrom` (`internal/config/config.go:98-184`) delegate to `basedir`, set `ErrBaseDirNotAbsolute = basedir.ErrNotAbsolute`, and delete `absoluteEnv`/`absoluteHomeDir`. Verify with the unchanged `TestConfigDir` cases in `internal/config/config_paths_test.go`.
- [ ] 2.2 Add the Windows scenarios from the `config-source-resolution` delta (relative `APPDATA` falls back, relative `USERPROFILE` gives defaults plus a diagnostic) to `config_paths_test.go`. Verify that they pass.
- [ ] 2.3 Group check: run `go test ./internal/config/ ./internal/discovery/` and confirm it passes.

## 3. Module cache and git authentication

- [ ] 3.1 In `internal/app/modulecache/cache.go`, reject a relative `INVOWK_MODULES_PATH` with a typed error that names the variable and asks for an absolute path, and take the default from `basedir` `Home()` through an injected lookup. Verify with new `DefaultDirWith` tests (relative env rejected, relative home rejected, absolute values unchanged).
- [ ] 3.2 Make `trySSHAuth` (`internal/app/modulesync/git.go:303-326`) use `basedir.OS().Home()` and return nil on error. Verify with a unit test showing that a relative home loads no key.
- [ ] 3.3 Before any transport call for an SSH URL (`git@` or `ssh://`; the same detection as `ValidateAuth`, `git.go:226-236`), refuse the fetch without an absolute home. Return a typed error that wraps `basedir.ErrNotAbsolute`, names the home and suggests an absolute home or an HTTPS URL with a token (design D2, Git). Verify with unit tests: an SSH URL with a relative home is refused with no dial (fake transport or an unroutable endpoint), and an HTTPS URL with a relative home is unaffected.
- [ ] 3.4 Add the testscript `tests/cli/testdata/module_cache_relative_path.txtar`: `INVOWK_MODULES_PATH=.modules` makes the module command fail with stderr naming the variable, and `./.modules` does not exist afterwards. It is not virtual, so it needs no mirror.
- [ ] 3.5 Update `INVOWK_MODULES_PATH` in `README.md` (around line 2241) and `website/docs/modules/dependencies/cli-commands.mdx:238` (it must be absolute), plus the pt-BR `current` copy. Verify by checking that the text matches the spec.
- [ ] 3.6 Group check: run `go test ./internal/app/modulecache/ ./internal/app/modulesync/ && go test ./tests/cli/ -run 'TestCLI/module_cache_relative_path'` and confirm it passes.

## 4. Provisioning, run locks and test infrastructure

- [ ] 4.1 `applyHostProvisionDefaults` (`internal/runtime/container_provision.go:247-250`): derive `CacheDir` from `basedir.OS().Home()`, and leave it empty on error. Verify with a unit test in `internal/runtime` that has relative `HOME` and checks for an empty `CacheDir`.
- [ ] 4.2 `resolveBuildContextParent` (`internal/provision/layer_provisioner.go:367-409`): use an absolute, existing `basedir` home, delete the `<cwd>/.invowk-build` branch, and update the doc comment at `:278-282`. Verify with unit tests showing that a relative home and an absolute missing home (`/no-home`) both yield a temp-dir parent and that nothing is created in the working directory.
- [ ] 4.3 Make `lockFilePathWith` (`internal/container/run_lock_linux.go:80-86`) and `internal/testutil/container_suite_lock_linux.go:52-54` use `RuntimeDir()` and fall back to `os.TempDir()`. Verify with a unit test for a relative `XDG_RUNTIME_DIR`.
- [ ] 4.4 Make `ContainerSafeTempDir` (`internal/testutil/testutil.go:148`) use `basedir`, and make `containerSuiteConfigDir` (`tests/cli/container_harness.go:363-375`) use a `basedir` resolver built from the testscript env. Verify that `go test ./internal/testutil/` passes.
- [ ] 4.5 Group check: run `go test ./internal/provision/ ./internal/container/ ./internal/runtime/ -run 'Provision|Lock|BuildContext'` and confirm it passes.

## 5. Virtual-runtime anchors

- [ ] 5.1 Make `standardVirtualAnchors`/`standardVirtualAnchorsForOS` (`internal/runtime/virtual_policy.go:140-186`) build each anchor from the matching `basedir` method, use `""` for an unavailable base, and drop `firstNonEmpty`. Verify by extending the existing `standardVirtualAnchorsForOS` tests with relative XDG values, relative `LOCALAPPDATA` on Windows, and a relative home.
- [ ] 5.2 In `expandVirtualAnchorPath` (`virtual_policy.go:284-297`), report a known but empty anchor as unavailable, and keep "unknown virtual path anchor" for unknown names. Verify with a unit test.
- [ ] 5.3 Add the testscript `virtual_anchor_relative_home.txtar` and its `native_anchor_relative_home.txtar` mirror. With `HOME=.`, `INVOWK_ANCHOR_CONFIG` is empty and the native mirror asserts the same environment contract. A Lua `invowk.path("@config")` case fails as unavailable. If the native side cannot express the case, add an entry to `tests/cli/runtime_mirror_exemptions.json` with a reason.
- [ ] 5.4 Update the anchor sections of `website/docs/runtime-modes/virtual.mdx` and `virtual-lua.mdx`, plus the pt-BR `current` copies: relative bases are ignored and home-derived anchors can be unavailable. Do not describe the virtual runtimes as sandboxed.
- [ ] 5.5 Group check: run `go test ./internal/runtime/ -run 'Anchor|VirtualPath' && go test ./tests/cli/ -run 'TestCLI/(virtual|native)_anchor_relative_home'` and confirm it passes.

## 6. Mechanical guard

- [ ] 6.1 Add forbidigo patterns `^os\.UserHomeDir$`, `^os\.UserConfigDir$` and `^os\.UserCacheDir$` to `.golangci.toml:422-426`, and a `[[linters.exclusions.rules]]` entry for `internal/basedir/` (design D3). Verify that `make lint` is clean, and that a scratch `os.UserHomeDir()` call in `internal/app` is reported and then removed.
- [ ] 6.2 Add `internal/basedir/guard_test.go`: an AST walk of non-test `.go` files under `cmd/`, `internal/`, `pkg/` and `tests/` that fails on the literals `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA` or `^XDG_[A-Z_]+$` outside `internal/basedir/`. It reports file:line and has a reasoned allowlist (initially `internal/testutil/home.go`). Verify that the test passes on the tree and fails on a planted literal in a temporary fixture dir.
- [ ] 6.3 Document the helper and the guard where home-directory handling is already covered: `.agents/skills/windows-testing/SKILL.md` (the `os.UserHomeDir`/`SetHomeDir` notes), `.agents/skills/go/SKILL.md` and `.agents/rules/testing.md`. Verify with `make check-agent-docs`.
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
  - `for m in samples/invowkmods/*.invowkmod; do go run . validate "$m"; done` (module cache change).
- [ ] 7.7 Confirm that the virtual/native mirror checks pass (`TestShRuntimeMirrorCoverage`, `TestVirtualNativeCommandPathAlignment`), and run `/learn`.
- [ ] 7.8 Run `openspec validate require-absolute-base-dirs --strict` and check for 0 warnings.
