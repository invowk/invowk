## Context

See proposal.md, "Why", for the motivation. Requirements: `specs/base-directory-resolution/spec.md` and the two MODIFIED deltas.

Current state, verified on main at f4969e90 (stage-2 review re-checked every reference below):

- **S1 helpers (the model):** `configDirFrom` (`internal/config/config.go:98-132`) takes injectable `goos`, `getenv` and `userHomeDir`. `absoluteEnv` (`config.go:138-143`) treats a relative `XDG_CONFIG_HOME` or `APPDATA` as unset. `absoluteHomeDir` (`config.go:150-159`) returns `ErrBaseDirNotAbsolute` (`config.go:42-45`) with `home directory %q`.
  - On Windows the config dir does not use `userHomeDir`. It reads `USERPROFILE` through `getenv` (`config.go:105-112`), with two texts of its own: "neither APPDATA nor USERPROFILE environment variable is set" (`:107`) and `USERPROFILE=%q` (`:110`). `commandsDirFrom` uses `userHomeDir` on every OS (`config.go:178-184`).
  - The S1 tests depend on that split: "windows relative USERPROFILE rejected" injects an absolute home and `USERPROFILE=profile`, and expects `absolute` and `USERPROFILE` (`internal/config/config_paths_test.go:66-67`). "windows without APPDATA or USERPROFILE" expects both names (`:98-99`).
  - `ConfigDir`, `CommandsDir` and `HostKeyPath` (`config.go:161-195`) are built on these helpers, which are private to `internal/config`.
- **Module cache:** `DefaultDirWith` (`internal/app/modulecache/cache.go:36-55`) takes `INVOWK_MODULES_PATH` after only a non-empty check (`cache.go:37-42`; `FilesystemPath.Validate` checks non-blank only, `pkg/types/filesystem_path.go:36-41`). Otherwise it joins `os.UserHomeDir()` without an absolute check (`cache.go:45-50`). `newResolver` then runs `filepath.Abs` on the result (`internal/app/modulesync/resolver.go:78-89`), so a relative value lands under the current directory. Only module operations reach it: `modulesync` (`resolver.go:80`, `use_cases.go:265-270`) and `moduleops` (`vendor_dependencies.go:83`); discovery for `invowk cmd` does not.
- **Git auth:** `NewGitFetcher` calls `setupAuth` (`internal/app/modulesync/git.go:47-51`), which tries SSH keys first and HTTPS tokens second, whatever the URL (`git.go:286-300`). `trySSHAuth` (`git.go:303-326`) loads `~/.ssh/id_*` from `os.UserHomeDir()` (`git.go:305`) without an absolute check. HTTPS tokens come from `GITHUB_TOKEN`, `GITLAB_TOKEN` and `GIT_TOKEN` (`git.go:329-353`).
  - `ValidateAuth` (`git.go:226-237`) refuses an SSH URL without auth, but nothing in production calls it. Its only caller is `git_test.go:409-433`. It is not a gate today.
  - Transport calls: `ListVersions` (`git.go:59`, `Auth` at `:68`), `Fetch` (`:94`, through `clone` `:358-369` and `fetch` `:380-382`), `GetCommitForTag` (`:123`, `:131`), `CloneShallow` (`:161`, `:182`), `IsPrivateRepo` (`:212-223`, which lists with `Auth: nil`, so go-git falls back to SSH-agent auth for SSH URLs) and `ListTagsWithCommits` (`:245-252`; `ListTags` delegates to `ListVersions`, `:240-242`). `fetch(repo)` takes no URL, so the gate belongs in the public methods.
  - URLs are limited to `https://`, `git@` and `ssh://` (`pkg/invowkmod/git_types.go:61-67`), so "not `https://`" means SSH.
- **go-git's own home reads (go-git v5.19.2, `go.mod:22`; ssh_config v1.6.0, `go.mod:174`):** the git library also reads per-user files, which invowk's guard cannot see.
  - SSH host keys: `getDefaultKnownHostsFiles` splits `SSH_KNOWN_HOSTS` with `filepath.SplitList` and uses it as given, else `os.UserHomeDir()/.ssh/known_hosts` and `/etc/ssh/ssh_known_hosts` (`plumbing/transport/ssh/auth_method.go:268-283`). A relative `SSH_KNOWN_HOSTS` entry is the same hole as a relative home, even with an absolute home.
  - SSH client config: `DefaultSSHConfig = ssh_config.DefaultUserSettings` (`plumbing/transport/ssh/common.go:23-25`) reads `<home>/.ssh/config`. That home comes from `os/user.Current()`, and `$HOME` is used only when the passwd lookup fails (`ssh_config` `config.go:63-74`).
  - With no explicit auth, the default is SSH-agent auth (`common.go:36-40`).
  - go-git's global and system git config (`config/config.go:205-210`, read by `Repository.ConfigScoped`, `repository.go:573-587`) and the global gitignore/gitattributes loaders are not reached: invowk calls none of `ConfigScoped`, `gitignore.*` or `gitattributes.*` (grep of `cmd/`, `internal/` and `pkg/`).
- **Provisioning:** `applyHostProvisionDefaults` sets `CacheDir` to `<home>/.cache/invowk/provision` whenever `os.UserHomeDir()` succeeds (`internal/runtime/container_provision.go:247-250`). It checks neither that the home is absolute nor that it exists. A configured `container.auto_provision.cache_dir` wins (`container_provision.go:217-218`). `CacheDir` is used only as the build-context parent (`internal/provision/config.go:64-67`; `layer_provisioner.go:368-369`).
  - `resolveBuildContextParent` (`internal/provision/layer_provisioner.go:367-408`) returns `CacheDir` first (`:368-369`). Otherwise it tries `<home>/invowk-build` if the home exists (`:375-383`, no absolute check), then `<cwd>/.invowk-build` (`:385-391`), and last a fresh `invowk-build-*` dir under `os.TempDir()` with cleanup (`:395-407`). The build context is a `ctx-*` temp dir under that parent (`:294`), and the parent is created with `MkdirAll` (`:290`).
  - So in the runtime path `CacheDir` is always set when the home lookup succeeds. With `HOME=/no-home` it becomes `/no-home/.cache/invowk/provision`, `MkdirAll` fails (`:290`), and the home-existence check at `:376` is never reached. `<home>/invowk-build`, the Snap-friendly location named in the doc comment (`:277-282`), is reached only by callers that build a `provision.Config` without `CacheDir`.
- **Virtual anchors:** `standardVirtualAnchors` (`internal/runtime/virtual_policy.go:140-146`) turns a home lookup error into `""`. `standardVirtualAnchorsForOS` (`:148-186`) then uses `firstNonEmpty` (`:188-193`), so a relative XDG or AppData value is taken as given, and an empty or relative home gives relative anchors such as `.config/invowk` (`:168-169`, `:180-183`).
  - The resolver adds every non-empty anchor except `@home` to the allow roots (`:103-108`). `filesystem.paths` mappings are expanded against the anchors first (`:99`, `resolveVirtualFilesystemPaths` `:207-227`), so a mapping on a relative `@home` becomes a cwd-relative allow root.
  - An empty anchor is rejected when used, with "unknown virtual path anchor" (`:289-292`), and that error fails resolver construction when it comes from a mapping (`:217-219`). Every anchor, empty ones included, is injected as `INVOWK_ANCHOR_*` (`:299-304`).
  - Only the virtual runtimes inject anchors (`internal/runtime/lua.go:192-197`, `lua_interactive.go:211-222`, `sh.go:371-382`, `sh.go:446`). The native runtime has no `INVOWK_ANCHOR_*`.
  - `standardVirtualAnchorsForOS` is bound by the `VirtualPathHarness` correspondence table (`formal/alloy/VirtualPathHarness.als:31`, binding `TestVirtualPathHarness_GoldenVectors`). Its unit test calls it with a `home string` argument (`internal/runtime/virtual_policy_test.go:66`).
- **Run locks:** `lockFilePathWith` (`internal/container/run_lock_linux.go:80-86`) uses `XDG_RUNTIME_DIR` as given and falls back to `os.TempDir()` only when it is empty. The lock is opened with `O_CREATE|O_RDWR`, without truncation (`:48`). `internal/testutil/container_suite_lock_linux.go:51-57` repeats the pattern.
- **Test infrastructure:**
  - `ContainerSafeTempDir` uses `os.UserHomeDir()` (`internal/testutil/testutil.go:148`).
  - `SetHomeDir` sets `HOME` or `USERPROFILE` (`internal/testutil/home.go:42-50`).
  - testscript sets `HOME=$WORK`, and on Windows `APPDATA=$WORK/appdata` and `USERPROFILE=$WORK` (`tests/cli/cmd_test.go:46-55`), so the testscript home is also the working directory.
  - `containerSuiteConfigDir` (`tests/cli/container_harness.go:363-375`) rebuilds the config dir from the testscript env by hand, without S1's absolute checks.
- **Search result:** a `grep` of every tracked non-test Go file (`cmd/`, `internal/`, `pkg/`, `tests/`, `main.go`) finds exactly the sites above. It searched for `os.UserHomeDir|UserCacheDir|UserConfigDir`, the literals `"HOME"`, `"XDG_*"`, `"APPDATA"`, `"LOCALAPPDATA"` and `"USERPROFILE"`, `"os/user"`, `user.Current`, `os.ExpandEnv` and `os.Expand(`.
  - Beyond those, `internal/uroot/mktemp.go:65` reads the script's `TMPDIR` (out of scope, see Non-Goals).
  - `specs/003-test-suite-audit/contracts/testutil_home.go:47` is a speckit design contract that writes `"HOME"`. It is not imported by any package.
- **In-flight siblings (checked 2026-11-14):**
  - C1 (#162, `cli-exit-code-contract`) MODIFIES `config-source-resolution` "Explicit config failures outside the exit-1 flows", a different requirement from the one MODIFIED here.
  - F1 (#163, `harden-formal-instruments`) and S3 (#173, `restrict-tui-server-bind`) touch neither capability.
  - No open change touches `cross-platform-path-anchors`. Recheck before S4's archive.
- **Lint:** forbidigo is enabled (`.golangci.toml:164-165`) with one text pattern (`:422-426`) and no `analyze-types`. It is already excluded for `_test.go` (`:698-700`). Text matching does not see an aliased `os` import.
- **Windows semantics:** Go's `filepath.IsAbs` on Windows requires a volume and a root, or a UNC path. So `\Users\me` and `C:Users\me` are relative and are rejected or ignored. `filepath.IsAbs` follows the host OS, not an injected `goos`. S1's tests already work around this with host-absolute fixtures (`internal/config/config_paths_test.go:27-29`).
- **Package design:** `.agents/rules/package-design.md` "Utility Package Boundaries" prefers a copy of a helper per domain over a shared utility. D1 explains why this case is the exception.

## Goals / Non-Goals

**Goals:**

- One package owns every read of the home directory and of the base-directory variables, with S1's semantics, and is unit tested for each OS.
- Each adopter keeps its current behaviour for an absolute, existing home, so the paths users have today do not change. Only relative, unavailable and missing-home cases change.
- A gate in `make test` makes new direct lookups fail CI.

**Non-Goals:**

- Moving any directory. For example, the provisioning cache stays at `<home>/.cache/invowk/provision` and does not start honouring `XDG_CACHE_HOME`.
- Validating `TMPDIR`, `TEMP` and `TMP`, or changing `os.TempDir()` and `os.MkdirTemp("", …)` (maintainer decision 6).
- Script-visible environment variables, for example `TMPDIR` in u-root `mktemp` (`internal/uroot/mktemp.go:65`). They belong to the script's own environment, not to invowk's trusted directories.
- `container.auto_provision.cache_dir` from config. Config is a trusted source after S1.
- The environment that invowk passes to Docker, Podman or the native shell. Those tools read their own per-user files (decision 9).

## Decisions

### D1. New leaf package `internal/basedir`

- It imports only the standard library, `pkg/platform` (OS name constants) and `pkg/types` (for the `types.FilesystemPath` return type). It is internal because it is not a public API. It is a leaf, so `internal/config`, `internal/app/*`, `internal/provision`, `internal/runtime`, `internal/container`, `internal/testutil` and `tests/cli` can all import it without cycles.
- The API is a small value type, `basedir.Dirs`, built from injectable inputs, the same as S1's `configDirFrom`: `basedir.New(goos, getenv, userHomeDir)`, and `basedir.OS()` for the real process (`runtime.GOOS`, `os.Getenv`, `os.UserHomeDir`). The zero `Dirs` behaves as `OS()`, so struct fields that hold one need no constructor change.
  - Methods: `Home()`, `ConfigHome()`, `DataHome()`, `CacheHome()`, `StateHome()`, each returning `(types.FilesystemPath, error)`, and `RuntimeDir()`, which returns the directory and whether it is available.
  - Platform defaults per kind are the ones the anchors use today: Linux XDG, macOS `Library/...`, Windows `APPDATA` for config and `LOCALAPPDATA` for data, cache and state.
  - On Windows the AppData fallbacks read `USERPROFILE` through `getenv`, which is what `os.UserHomeDir` itself reads on Windows. They keep S1's two texts (`config.go:107`, `:110`). `Home()` uses `userHomeDir` on every OS and keeps `home directory %q`. This keeps the S1 tests green with no edits (Context).
  - `HomeFromEnv(goos, getenv)` reproduces `os.UserHomeDir`'s variable choice (`USERPROFILE` on Windows, `home` on Plan 9, else `HOME`) for harnesses that only have an env map.
- Errors: `basedir.ErrNotAbsolute`, wrapped with the source. `config.ErrBaseDirNotAbsolute` becomes `= basedir.ErrNotAbsolute`, so `errors.Is` at existing call sites keeps working, and the sentinel text "base directory must be an absolute path" (`config.go:45`) is kept.
- Why one shared package, against the package-design rule's preference for a copy per domain: this is a security invariant at the OS boundary, not a convenience helper. Copies are the drift this change removes: today each of the seven sites checks differently or not at all. The guard (D3) can enforce one owner, but it cannot enforce that copies stay equal. `doc.go` records this.
- Rejected alternatives:
  - Putting the helpers in `internal/config`: `internal/provision` and `internal/container` would then depend on config for a cross-cutting OS concern.
  - Putting them in `pkg/platform`: that would add new public API surface.
  - One helper copy per package: see above.

### D2. Adopters keep their paths, and each adopter handles its own unavailable case

Each adopter takes a `basedir` value (or an injected lookup) at its seam, so tests do not mutate `HOME` or the working directory.

- `internal/config`: `configDirFrom` and `commandsDirFrom` keep their signatures and build a `basedir.New(goos, getenv, userHomeDir)`. `ConfigDir` on Windows uses `ConfigHome`, which is `APPDATA`, or else `USERPROFILE\AppData\Roaming`. The S1 tests in `config_paths_test.go` stay green unchanged.
- Module cache:
  - A relative `INVOWK_MODULES_PATH` is rejected with a new typed error, `modulecache.ModulesPathNotAbsoluteError`, which wraps `basedir.ErrNotAbsolute` and names the variable. This check belongs to `modulecache`, because the variable is invowk's own.
  - The default comes from `Home()`.
  - The new `DefaultDirFrom(getenv, dirs basedir.Dirs)` holds the logic. `DefaultDirWith(getenv)` keeps its signature and delegates with `basedir.OS()`.
- Git (fail closed):
  - `trySSHAuth` takes the resolver from the fetcher and returns nil when `Home()` fails. `GitFetcher` gets an unexported `dirs basedir.Dirs` field: `NewGitFetcher` sets `basedir.OS()`, and tests build the fetcher with an injected one.
  - One helper, `(*GitFetcher).requireTrustedSSHFiles(gitURL)`, runs first in each public transport method: `ListVersions`, `Fetch`, `GetCommitForTag`, `CloneShallow`, `IsPrivateRepo` and `ListTagsWithCommits`. For any URL that does not start with `https://`, it requires an absolute `Home()` and an absolute entry in every `filepath.SplitList(SSH_KNOWN_HOSTS)` element. Otherwise it returns `ErrSSHTrustFilesNotAbsolute`, a typed error that wraps `basedir.ErrNotAbsolute`, names the value and suggests an absolute home or an HTTPS URL with a token.
  - `IsPrivateRepo` returns `bool` and has no production caller (only its definition at `git.go:211-224`), like `ValidateAuth`. On that error it reports "private" (true) without connecting, so a future caller cannot reach the network through it.
  - `SSH_KNOWN_HOSTS` is read through the fetcher's injected `getenv`. It is not a base-directory variable, so the guard allows it.
  - HTTPS URLs are unaffected.
  - `ValidateAuth` is left as it is: it is unused in production, and the new helper is the gate.
  - Rejected alternatives:
    - Setting `SSH_KNOWN_HOSTS` and blanking `DefaultSSHConfig`: that mutates process-global go-git state, and the SSH config file is still read through the passwd home.
    - Only skipping key lookup: that leaves the known-hosts trust hole open.
- Provisioning:
  - `applyHostProvisionDefaults` sets the default `CacheDir` only when `Home()` is absolute and is an existing directory. Otherwise it leaves `CacheDir` empty. It reads the home from `provisionCfg.BaseDirs` (below). That existence check is what makes the `HOME=/no-home` scenario reach the temp branch (Context).
  - `resolveBuildContextParent` uses `<home>/invowk-build` only for an absolute, existing `Home()`, and otherwise the existing temp branch (`layer_provisioner.go:395-407`). The `<cwd>/.invowk-build` branch is deleted. `LayerProvisioner` gets the resolver through its config: `provision.Config` gains a `BaseDirs basedir.Dirs` field, set through a new `WithBaseDirs` option beside `WithCacheDir` (`internal/provision/config.go:246-251`), and its zero value means `basedir.OS()`. `Config` is never compared with `==` and `Validate` (`config.go:104`) checks named fields only, so a field holding funcs is safe. The doc comment at `:277-282` is updated to say that the cache dir wins, and that `invowk-build` and the temp dir are the fallbacks.
- Virtual anchors:
  - `standardVirtualAnchorsForOS` keeps its name, because the formal table binds it. It takes `(goos, workDir, tempDir string, dirs basedir.Dirs)` and builds each anchor from the `basedir` method for its kind. `standardVirtualAnchors` passes `basedir.OS()`. `firstNonEmpty` is removed.
  - An unavailable base gives `""`. Existing code then keeps it out of the allow roots (`virtual_policy.go:105`), and rejects it when used directly (`:290`) or through a `filesystem.paths` mapping (`:217-219`). A mapping on an unavailable anchor therefore fails the command before the script runs, which is fail closed.
  - `expandVirtualAnchorPath` reports a known but empty anchor as `virtual path anchor "@home" is unavailable: <reason>`. Unknown names keep "unknown virtual path anchor".
  - `@tmp` stays `os.TempDir()`.
  - "Anchor allow roots are explicit" (main spec `cross-platform-path-anchors`) still holds: an unavailable anchor has no root to allow. "Path bridge exposure is consistent" still holds, because every `INVOWK_ANCHOR_*` is still injected, possibly empty. Neither is MODIFIED.
- Run locks: `lockFilePathWith` and the testutil suite lock use `RuntimeDir()`, then `os.TempDir()`.
- Test infrastructure:
  - `ContainerSafeTempDir` uses `basedir.OS().Home()` and calls `t.Fatalf` on error.
  - `containerSuiteConfigDir` builds `basedir.New(runtime.GOOS, env.Getenv, func() (string, error) { return basedir.HomeFromEnv(runtime.GOOS, env.Getenv) })` and uses `ConfigHome()`, so it reproduces exactly what the binary does.

### D3. Two-layer mechanical guard; the AST test is authoritative

- The AST test, `internal/basedir/guard_test.go`, finds the module root from `go.mod`. It parses every tracked non-test `.go` file under `cmd/`, `internal/`, `pkg/` and `tests/`, plus `main.go`, and skips `testdata/` and `internal/basedir/` itself. It fails on:
  - a call or reference `X.UserHomeDir`, `X.UserConfigDir` or `X.UserCacheDir`, where `X` is the file's local name for the `os` import (aliases included);
  - an import of `os/user`;
  - a string literal equal to `HOME`, `USERPROFILE`, `APPDATA` or `LOCALAPPDATA`, matching `^XDG_[A-Z_]+$`, or containing `$HOME` or `${HOME}`.
- Each failure reports file and line. The test has a file allowlist, each entry with a reason. The initial entry is `internal/testutil/home.go`, whose `SetHomeDir` writes `HOME` and `USERPROFILE` for tests.
- The walk is limited to `cmd/`, `internal/`, `pkg/`, `tests/` and `main.go`, so `specs/` (speckit design contracts, not built into the binary) is out of scope by construction.
- It runs in `make test` (`go test ./...`), so it gates CI without a new target.
- forbidigo is the fast editor-time layer: text patterns `^os\.UserHomeDir$`, `^os\.UserConfigDir$` and `^os\.UserCacheDir$`, with one `[[linters.exclusions.rules]]` entry for `internal/basedir/`. Test files are already excluded.
  - `analyze-types` is not turned on, because it would change how the existing `errors.As` rule matches across the codebase. The AST test covers aliases instead.
  - forbidigo cannot see literals passed to an injected `getenv` (for example `run_lock_linux.go:81`), which is another reason the AST test is the authority.
- Rejected alternative: a shell `grep` in `scripts/`. It is weaker against formatting and comments, and it is not part of `make test`.

### D4. Windows coverage

- The Windows cases (relative `APPDATA`, `LOCALAPPDATA` or `USERPROFILE`; drive-relative `C:Users\me`; root-relative `\Users\me`; an absolute UNC profile) run as table tests skipped unless `runtime.GOOS == "windows"`, because `filepath.IsAbs` follows the host.
- The injected-`goos` tests use host-absolute fixtures, as S1 does.
- A local `GOOS=windows go test -c` only proves the Windows tests compile. The behavioural proof is the windows-latest CI job, which the PR must pass.

## Maintainer decisions (2026-11-14)

1. Scope: centralise base-dir resolution in one helper package that follows the package-design rule, uses S1's semantics, is adopted everywhere and is tested on Windows too. Implemented by D1, D2 and D4.
2. XDG-style variables (including `XDG_RUNTIME_DIR`, `APPDATA` and `LOCALAPPDATA`) are ignored when relative and fall back. A relative home or `USERPROFILE` is rejected.
3. Add a mechanical guard so new code cannot bypass the helper. Implemented by D3.
4. Prefer MODIFIED of S1's base-dir requirement where it generalises. "Platform base directories must be absolute" is MODIFIED to point at the shared resolver, and the general rules live in the new `base-directory-resolution` capability.
5. Recommended by the advisor workflow and taken without asking, per the standing instruction for this programme:
   - reject a relative `INVOWK_MODULES_PATH`, which matches S1's `INVOWK_CONFIG` rule;
   - remove the `<cwd>/.invowk-build` fallback in favour of the temp dir, including for an absolute but missing home;
   - make home-derived anchors unavailable, not relative, when the home directory is unavailable;
   - refuse SSH-transport git fetches without an absolute home, because go-git reads `known_hosts` from it;
   - keep every existing absolute path unchanged.

The stage-2 review added these, also advisor-reviewed and taken without asking:

6. **`TMPDIR` stays out of scope; this change makes no claim beyond "never falls back to the current directory".** The temp fallbacks this change relies on are safe even with a relative `TMPDIR`. The build-context parent is a new random `invowk-build-*` directory that invowk creates with `os.MkdirTemp` (`layer_provisioner.go:395`), so a repository cannot plant its contents. The run lock is opened without truncation (`run_lock_linux.go:48`). When the command workdir is the current directory (the default), a cwd-relative `@tmp` allow root grants nothing beyond `@work`, which is already an allow root (`virtual_policy.go:104`). With a different workdir it adds a fixed cwd subdirectory, which the user's own `TMPDIR` chose. Validating `TMPDIR` would touch about ten unrelated `os.TempDir`/`os.CreateTemp` sites (for example `internal/runtime/native.go:276`). It would also need per-OS rules, because Windows derives the temp dir from `TMP`, `TEMP` and `USERPROFILE` inside the OS call. It is a candidate follow-up change, and adding `basedir.TempDir()` later fits D1.
7. **The provisioning cache keeps its location** (`<home>/.cache/invowk/provision`). Moving it to `CacheHome()` would relocate a user-visible directory, so it is not done here.
8. **No relaxation for an absolute `SSH_KNOWN_HOSTS` under a relative home.** The key lookup and the SSH-agent default still depend on the home, and one rule is easier to reason about and test. Conversely, a relative `SSH_KNOWN_HOSTS` entry is refused even with an absolute home (D2, Git), because go-git uses it as given.
9. **Engine and shell environments are not rewritten.** Docker and Podman read their own per-user config (`~/.docker`, `$XDG_CONFIG_HOME/containers`), and invowk passes them the inherited environment. With a relative home or XDG value, those tools may read files under the current directory. Fixing that means invowk refusing to launch engines, or rewriting their environment. That is a separate product decision, so it is recorded as a follow-up and not taken here.
10. **Provisioning checks that the home exists where `CacheDir` is defaulted**, not only in `resolveBuildContextParent`. Without this, the `HOME=/no-home` scenario fails at `MkdirAll` and never reaches the temp branch (Context).
11. **The SSH gate tests "not `https://`"**, not a list of SSH prefixes. `GitURL.Validate` allows only three schemes, so the gate fails closed if a scheme is ever added.
12. **Virtual anchors on the native runtime:** the new `virtual_anchor_relative_home.txtar` gets an exemption in `tests/cli/runtime_mirror_exemptions.json` ("path anchors are injected only by the virtual runtimes"), not a native mirror. The native runtime has no `INVOWK_ANCHOR_*` to assert (Context).

## Risks / Trade-offs

- [CI or containers that run with `HOME` unset or relative lose the module cache] → The error names the variable and the absolute-path rule. `INVOWK_MODULES_PATH` set to an absolute path is the documented workaround (Migration notes).
- [A temp-dir build context may not be visible to Docker Snap, which cannot reach `/tmp`] → It only applies when the home is relative, unavailable or missing (for example `HOME=/no-home`). Today the missing-home case fails at `MkdirAll` (Context). Now invowk no longer fails creating its own directory. The engine CLI may still need a usable home: `tests/cli/cmd_test.go:42-45` records Docker failing with `mkdir /no-home`. A configured `container.auto_provision.cache_dir` still wins.
- [The guard covers invowk's code only, so third-party libraries can still read `HOME` (go-git, and `ssh_config` as a fallback)] → Fetches that reach those reads (SSH transport) are refused without an absolute home and absolute `SSH_KNOWN_HOSTS` entries. Any new dependency that reads per-user files needs the same review; this is noted in the go skill (task 6.3).
- [SSH fetches with a relative home now fail, even when `SSH_KNOWN_HOSTS` is absolute] → Decision 8. The error says to use an absolute home or an HTTPS URL with a token.
- [The guard has false positives, for example a log string "HOME"] → Only exact literal matches (and `$HOME`/`${HOME}` substrings) count. The allowlist with reasons is the escape hatch, and it is reviewed like any lint exception.
- [Changing the anchor error text breaks tests that match "unknown virtual path anchor"] → Unknown names keep that text. Only known but unavailable anchors get the new message.
- [The formal `VirtualPathHarness` binding drifts] → The symbol name is kept, and task 5.6 runs the correspondence check and the golden vectors.

## Migration Plan

Release notes take the proposal's Migration notes. There is no data migration. Rollback is reverting the change. This lands last in the v0.17.0 order S1 -> C1 -> F1 -> S3 -> S4, so it rebases on the config and audit changes before it.

## Open questions

None. The three questions from the proposal stage are settled by maintainer decisions 6, 7 and 8. Follow-up candidates, which are not part of this change: `TMPDIR` validation (decision 6), the engine environment (decision 9), and moving the provisioning cache (decision 7).
