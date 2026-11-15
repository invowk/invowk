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
- **Package design:** `.agents/rules/package-design.md` "Utility Package Boundaries" says without qualification that "each gets its own copy" when several domain packages need similar helpers (`package-design.md:44`). A rule beats a `doc.go` note (AGENTS.md "Governance precedence"), so this change adds the `internal/basedir` exception to the rule itself (decision 14). D1 gives the reason.
- **F1's anchors binding (branch `feat/harden-formal-instruments`, #163, checked 2026-11-14):** `TestVirtualPathHarness_RootConstruction` (`internal/runtime/virtual_path_harness_formal_test.go:102` on that branch) reaches `standardVirtualAnchorsForOS` only through `newVirtualPathResolverForFilesystem` and `standardVirtualAnchors`, for the host GOOS. It never calls `standardVirtualAnchorsForOS` directly. It fails when `@config`, `@data`, `@cache`, `@state`, `@tmp`, `@work` or `@home` is empty, accepts each of the first six as an allowed root, and denies `@home/.ssh/...` under restricted access. F1 changes neither `virtual_policy.go` nor `virtual_policy_test.go`.
- **Home lookups in dependencies with no `HOME`:** go-git returns an error from `getDefaultKnownHostsFiles` when `SSH_KNOWN_HOSTS` is empty and `os.UserHomeDir` fails (`auth_method.go:267-283`), so an SSH fetch with no home and no `SSH_KNOWN_HOSTS` fails today at host-key verification. `ssh_config` takes its home from `os/user.Current()` and uses `$HOME` only when that lookup fails (`ssh_config@v1.6.0/config.go:63-70`). With both unavailable it joins `""` and reads `.ssh/config` under the current directory. Reproduced end to end on origin/main (`3760a4ba`) on 2026-11-15 (decision 20): it happens only when the uid has no passwd entry and `HOME` is unset, and the fetch reaches the lookup only with `SSH_KNOWN_HOSTS` set, because go-git builds the known-hosts callback (`common.go:123`) before it reads `Hostname`/`Port` (`common.go:127`, `:205-240`).
- **systemd and `HOME` (systemd 259, `systemd.exec(5)`, checked 2026-11-15):** `$USER` is always set, but `$HOME`, `$LOGNAME` and `$SHELL` are set only for units with `User=` (or `DynamicUser=`/`PAMName=`), unless `SetLoginEnvironment=` says otherwise (added in v255; `yes` sets them even for the default root user, `no` never sets them). User units inherit the user manager's environment, which has `HOME`; verified with `systemd-run --user --wait -P env` (`HOME=/var/home/<user>`). So a system unit without `User=` (a root service or timer, the case the CST evaluation considers in tracker N1) runs invowk with `HOME` unset and `WorkingDirectory=` defaulting to `/`. The system-unit case is taken from the man page only (running one needs root).
- **Provisioning callers without `CacheDir`:** `NewContainerRuntimeWithEngine` builds `provision.DefaultConfig()` without `applyHostProvisionDefaults` (`internal/runtime/container.go:131-133`), and `DefaultConfig` sets no `CacheDir`. That caller and an unavailable home are the ways `resolveBuildContextParent` reaches its home and cwd branches.

## Goals / Non-Goals

**Goals:**

- One package owns every read of the home directory and of the base-directory variables, with S1's semantics, and is unit tested for each OS.
- Zero-change invariant (decision 13): when every base variable is absolute or unset, nothing observable changes, except the rows D5 lists as **BREAKING**, each with a migration note. A test compares the new code with a frozen copy of today's code over a table of such environments.
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
- Why one shared package, against the package-design rule's preference for a copy per domain: this is a security invariant at the OS boundary, not a convenience helper. Copies are the drift this change removes: today each of the seven sites checks differently or not at all. The guard (D3) can enforce one owner, but it cannot enforce that copies stay equal. `doc.go` records this, and `.agents/rules/package-design.md` "Utility Package Boundaries" gains an explicit exception naming `internal/basedir` (decision 14).
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
  - One helper, `(*GitFetcher).requireTrustedSSHFiles(gitURL)`, runs first in each public transport method: `ListVersions`, `Fetch`, `GetCommitForTag`, `CloneShallow`, `IsPrivateRepo` and `ListTagsWithCommits`. For any URL that does not start with `https://`, it requires `Home()` to succeed with an absolute path and every non-empty `filepath.SplitList(SSH_KNOWN_HOSTS)` element to be absolute. Otherwise it returns a `*SSHTrustFilesError`, which names the value (or says the home is unavailable) and suggests an absolute home or an HTTPS URL with a token. It unwraps to the sentinel `ErrSSHTrustFilesNotAbsolute` and to the cause: `basedir.ErrNotAbsolute` for a relative value, or the lookup error for an unavailable home.
  - An unavailable home (for example `HOME` unset) is refused too, not only a relative one. With no home and no passwd entry, `ssh_config` reads `.ssh/config` under the current directory (Context; reproduced, decision 20). Today such a fetch already fails at host-key verification unless `SSH_KNOWN_HOSTS` is set, so the change is narrow; it is a **BREAKING** row of D5. The gate checks the environment home only, so it also refuses the safe case of a uid with a passwd entry and `HOME` unset (there `ssh_config` would use the absolute passwd home). One environment-only rule is kept over mirroring the passwd fallback: invowk's own key lookup uses the environment home too, and the guard forbids `os/user` outside `internal/basedir`.
  - `IsPrivateRepo` returns `bool` and has no production caller (only its definition at `git.go:211-224`), like `ValidateAuth`. On that error it reports "private" (true) without connecting, so a future caller cannot reach the network through it.
  - `SSH_KNOWN_HOSTS` is read through the fetcher's injected `getenv`. It is not a base-directory variable, so the guard allows it.
  - HTTPS URLs are unaffected.
  - `ValidateAuth` is left as it is: it is unused in production, and the new helper is the gate.
  - Rejected alternatives:
    - Setting `SSH_KNOWN_HOSTS` and blanking `DefaultSSHConfig`: that mutates process-global go-git state, and the SSH config file is still read through the passwd home.
    - Only skipping key lookup: that leaves the known-hosts trust hole open.
- Provisioning:
  - `applyHostProvisionDefaults` sets the default `CacheDir` whenever `Home()` succeeds with an absolute path, as today, and leaves it empty otherwise. It does not check that the home exists: an absolute home is not a current-directory hole, and an existence check would move the cache of an absolute but missing home that `MkdirAll` can create (decision 15, which supersedes decision 10). It reads the home from `provisionCfg.BaseDirs` (below).
  - `resolveBuildContextParent` uses `<home>/invowk-build` only for an absolute, existing `Home()`, as today apart from the absolute check, and otherwise the existing temp branch (`layer_provisioner.go:395-407`). The `<cwd>/.invowk-build` branch is deleted. `LayerProvisioner` gets the resolver through its config: `provision.Config` gains a `BaseDirs basedir.Dirs` field, set through a new `WithBaseDirs` option beside `WithCacheDir` (`internal/provision/config.go:246-251`), and its zero value means `basedir.OS()`. `Config` is never compared with `==` and `Validate` (`config.go:104`) checks named fields only, so a field holding funcs is safe. The doc comment at `:277-282` is updated to say that the cache dir wins, and that `invowk-build` and the temp dir are the fallbacks.
- Virtual anchors:
  - `standardVirtualAnchorsForOS` keeps its name, because the formal table binds it. It takes `(goos, workDir, tempDir string, dirs basedir.Dirs)` and builds each anchor from the `basedir` method for its kind. `standardVirtualAnchors` passes `basedir.OS()`. `firstNonEmpty` is removed.
  - The signature change keeps F1's contract (decision 16): F1's `TestVirtualPathHarness_RootConstruction` reaches the function only through `newVirtualPathResolverForFilesystem` (Context), whose signature does not change. With an absolute home, as on every CI runner, all seven anchors stay non-empty and keep today's paths, so that test is unaffected. With no usable home it fails at "the resolver reports no ... anchor", as it already does today for `@home`. S4's stage 4 waits for F1's merge, rebases, and re-runs that test and `python3 scripts/formal.py correspondence`.
  - An unavailable base gives `""`. Existing code then keeps it out of the allow roots (`virtual_policy.go:105`), and rejects it when used directly (`:290`) or through a `filesystem.paths` mapping (`:217-219`). A mapping on an unavailable anchor therefore fails the command before the script runs, which is fail closed.
  - `expandVirtualAnchorPath` reports a known but empty anchor as `virtual path anchor "@home" is unavailable: its base directory is relative or unset`. Unknown names keep "unknown virtual path anchor".
  - `@tmp` stays `os.TempDir()`.
  - "Anchor allow roots are explicit" (main spec `cross-platform-path-anchors`) still holds: an unavailable anchor has no root to allow. "Path bridge exposure is consistent" still holds, because every `INVOWK_ANCHOR_*` is still injected, possibly empty. Neither is MODIFIED.
- Run locks: `lockFilePathWith` and the testutil suite lock use `RuntimeDir()`, then `os.TempDir()`.
- Test infrastructure:
  - `ContainerSafeTempDir` uses `basedir.OS().Home()` and calls `t.Fatalf` on error.
  - `containerSuiteConfigDir` builds `basedir.New(runtime.GOOS, env.Getenv, func() (string, error) { return basedir.HomeFromEnv(runtime.GOOS, env.Getenv) })` and uses `ConfigHome()`, so it reproduces exactly what the binary does.

### D3. Two-layer mechanical guard; the AST test is authoritative

- The AST test, `internal/basedir/guard_test.go`, finds the module root from `go.mod` and calls `scanBaseDirLookups(root, allowlist)`. The scan walks the file system below `root` and parses every non-test `.go` file under `cmd/`, `internal/`, `pkg/` and `tests/`, plus `main.go`. It skips any `testdata` directory and `internal/basedir/`, both matched on paths relative to `root`. It fails on:
  - a call or reference `X.UserHomeDir`, `X.UserConfigDir` or `X.UserCacheDir`, where `X` is the file's local name for the `os` import (aliases included), and a bare `UserHomeDir`, `UserConfigDir` or `UserCacheDir` identifier in a file that dot-imports `os`;
  - an import of `os/user`;
  - a string literal equal to `HOME`, `USERPROFILE`, `APPDATA` or `LOCALAPPDATA`, matching `^XDG_[A-Z_]+$`, or containing `$HOME` or `${HOME}`.
- Each failure reports file and line. The test has a file allowlist, each entry with a reason. The initial entry is `internal/testutil/home.go`, whose `SetHomeDir` writes `HOME` and `USERPROFILE` for tests.
- The walk is limited to `cmd/`, `internal/`, `pkg/`, `tests/` and `main.go`, so `specs/` (speckit design contracts, not built into the binary) is out of scope by construction.
- Non-vacuity (decision 17): a committed planted-violation fixture, `internal/basedir/testdata/guardfixture/`, mirrors the tree layout (`internal/planted/*.go`, plus an allowlisted path and an `internal/basedir/` copy that must be skipped). Scanning `testdata/guardfixture` must return exactly the expected findings, which the fixture marks with `// want: <kind>` comments, one per pattern, each with file and line: `os.UserHomeDir`, `os.UserConfigDir` and `os.UserCacheDir` under the plain name, under an alias and through a dot import, an `os/user` import, each exact literal, an `XDG_*` literal, and `$HOME` and `${HOME}` substrings. A second assertion checks that the skipped and allowlisted fixture files produce no finding. Fixture files carry the SPDX header and `gofmt` formatting, and the Go toolchain and golangci-lint ignore `testdata/`.
- It runs in `make test` (`go test ./...`), so it gates CI without a new target.
- forbidigo is the fast editor-time layer: text patterns `^os\.UserHomeDir$`, `^os\.UserConfigDir$` and `^os\.UserCacheDir$`, with one `[[linters.exclusions.rules]]` entry for `internal/basedir/`. Test files are already excluded.
  - `analyze-types` is not turned on, because it would change how the existing `errors.As` rule matches across the codebase. The AST test covers aliases instead.
  - forbidigo cannot see literals passed to an injected `getenv` (for example `run_lock_linux.go:81`), which is another reason the AST test is the authority.
- Rejected alternative: a shell `grep` in `scripts/`. It is weaker against formatting and comments, and it is not part of `make test`.

### D4. Windows coverage

- The Windows cases (relative `APPDATA`, `LOCALAPPDATA` or `USERPROFILE`; drive-relative `C:Users\me`; root-relative `\Users\me`; an absolute UNC profile) run as table tests skipped unless `runtime.GOOS == "windows"`, because `filepath.IsAbs` follows the host.
- The injected-`goos` tests use host-absolute fixtures, as S1 does.
- A local `GOOS=windows go test -c` only proves the Windows tests compile. The behavioural proof is the windows-latest CI job, which the PR must pass.

### D5. Zero-change invariant for absolute or unset bases

When every base variable (`HOME`, `USERPROFILE`, `XDG_*`, `APPDATA`, `LOCALAPPDATA`, `INVOWK_MODULES_PATH`, `SSH_KNOWN_HOSTS`) is absolute or unset, S4 changes nothing observable, except the rows marked **BREAKING** below. Each review item was checked against today's code, with `XDG_RUNTIME_DIR` unset and set:

| Item | Absolute bases | A base unset | Outcome |
|---|---|---|---|
| (a) `filesystem.paths` mapping on an unavailable anchor fails | No anchor is unavailable; same paths (`virtual_policy.go:148-186`) | `HOME` unset (and the XDG/AppData variable for that kind unset): today `@config`, `@data`, `@cache` and `@state` are cwd-relative (`.config/invowk`, `:180-183`) and a mapping on them resolves under the current directory; `@home` is already `""` and a mapping on it already fails (`:217-219`, `:289-292`) | **BREAKING**, kept: a cwd-relative allow root is the defect. Home-derived anchors become unavailable, `INVOWK_ANCHOR_*` for them becomes empty, and a mapping on them fails. For `@home` only the error text changes (exit status the same). Migration note added. |
| (b) `XDG_RUNTIME_DIR` reported unavailable | Used as given (`run_lock_linux.go:81`) | Unset: `os.TempDir()` (`:82-84`) | No change. Only a relative value changes, which is outside the invariant. No anchor reads `XDG_RUNTIME_DIR`. |
| (c) default `CacheDir` only for an existing absolute home | An absolute but missing home that `MkdirAll` can create would move from `<home>/.cache/invowk/provision` to the temp dir | `HOME` unset: `CacheDir` stays empty (`container_provision.go:248`), same | **Removed**: the existence check is dropped; `CacheDir` is defaulted for any absolute home, as today (decision 15). |
| (d) `<cwd>/.invowk-build` branch removed | Reached with an absolute but missing home and an empty `CacheDir` (`NewContainerRuntimeWithEngine`, Context): today `<cwd>/.invowk-build`, now a temp dir | `HOME` unset: today `<cwd>/.invowk-build` (`layer_provisioner.go:385-391`), now a temp dir | **BREAKING**, kept (decision 5): a repository can plant `.invowk-build`. Migration note added. |
| (e) SSH gate | Absolute home and absolute or unset `SSH_KNOWN_HOSTS`: passes, same | `HOME` unset: today fails at host-key verification unless `SSH_KNOWN_HOSTS` is set; now refused before connecting | **BREAKING**, kept (D2, Git; decision 20): with no passwd entry for the uid, `ssh_config` falls back to a cwd-relative `.ssh/config`, which origin/main follows when `SSH_KNOWN_HOSTS` is set (Context). Migration note added. |
| (f) module cache | Same path | `HOME` unset: an error today and after, with the same wrapping text | No change. |
| (g) config and commands dirs, `trySSHAuth`, `@tmp`, `@work` | Same | Same | No change. |

- The proof is a test per adopter package that freezes today's functions as a legacy oracle in a `_test.go` file (test files are outside the guard and forbidigo), with injected `goos`, `getenv` and home lookup and host-absolute fixtures. A table over representative environments compares the new result with the oracle. Linux, macOS and Windows `goos` values are covered where the code branches on `goos`. The environments are: absolute existing home; absolute missing home; home unset; each XDG/AppData variable absolute and unset; `XDG_RUNTIME_DIR` absolute and unset; `SSH_KNOWN_HOSTS` absolute and unset. BREAKING rows stay in the table as explicit expected differences, so the test fails if any other row drifts.
- Covered functions: `standardVirtualAnchorsForOS`, the default `CacheDir` of `applyHostProvisionDefaults`, `resolveBuildContextParent` (parent kind: cache, `<home>/invowk-build`, temp), `lockFilePathWith`, `modulecache.DefaultDirFrom`, and the SSH gate's verdict against "today proceeds to the transport".

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
10. ~~**Provisioning checks that the home exists where `CacheDir` is defaulted.**~~ Superseded by decision 15.
11. **The SSH gate tests "not `https://`"**, not a list of SSH prefixes. `GitURL.Validate` allows only three schemes, so the gate fails closed if a scheme is ever added.
12. **Virtual anchors on the native runtime:** the new `virtual_anchor_relative_home.txtar` gets an exemption in `tests/cli/runtime_mirror_exemptions.json` ("path anchors are injected only by the virtual runtimes"), not a native mirror. The native runtime has no `INVOWK_ANCHOR_*` to assert (Context).

The stage-3 review added these, also advisor-reviewed and taken without asking:

13. **Zero-change invariant** (D5): with every base absolute or unset, nothing observable changes except the BREAKING rows in D5, each with a migration note. A legacy-oracle test per adopter proves it.
14. **The package-design rule gets the exception**, not only `doc.go`: `.agents/rules/package-design.md:44` reads as unconditional, and a rule beats a doc note. The exception names `internal/basedir` and its reason (one owner for a security invariant at the OS boundary, enforced by a guard), and `make check-agent-docs` runs.
15. **The default `CacheDir` needs an absolute home, not an existing one** (D5 row c). It supersedes decision 10. `HOME=/no-home` therefore keeps today's behaviour: the cache path is `/no-home/.cache/invowk/provision`, and provisioning fails at `MkdirAll` when that path cannot be created.
16. **F1's anchors contract is kept** (D2, Virtual anchors): `newVirtualPathResolverForFilesystem` keeps its signature, and every anchor stays non-empty with an absolute home. Stage 4 waits for F1's merge, rebases, and re-runs `TestVirtualPathHarness_RootConstruction` and the correspondence check.
17. **The guard proves it is not vacuous** with a committed planted-violation fixture (D3), one finding per claimed pattern. `import . "os"` is covered by flagging the bare identifiers.
18. **An unavailable home also refuses SSH fetches** (D2, Git; D5 row e), because `ssh_config` then reads `.ssh/config` under the current directory.

The stage-4a review (2026-11-15) checked the three `HOME`-unset BREAKING rows (D5 a, d, e) against how invowk is actually run, and added these, taken without asking under the standing instruction:

19. **systemd gives a root system unit no `HOME`** (Context, "systemd and `HOME`"), so all three rows apply to it: home-derived anchors are unavailable, provisioning builds from a temp dir, and SSH fetches are refused. They stay BREAKING and kept, because the old results were a cwd-relative allow root (with the default `WorkingDirectory=/`, `/.config/invowk`) and a plantable `<cwd>/.invowk-build`. The migration notes name the unit-level fixes: `Environment=HOME=/root` (or another absolute directory), `SetLoginEnvironment=yes`, absolute `XDG_*` variables for the anchors, and an HTTPS URL with a token for fetches.
20. **The SSH row is real, so the gate keeps refusing an unavailable home, not only a relative one.** Reproduced against a binary built from origin/main `3760a4ba`, with a planted `<cwd>/.ssh/config` that maps a fake host to a local listener, `HOME` unset, an absolute `SSH_KNOWN_HOSTS` and an SSH agent:
    - uid with a passwd entry: `ssh_config` uses the passwd home; the cwd file is not read (the fetch fails at DNS for the fake host);
    - uid with no passwd entry (`unshare -U --map-user=54321`): the old binary connected to the planted host's listener. The same holds with cgo (`user: unknown userid`) and without (`user: Current requires cgo or $HOME set`), and with `HOME=rel` the file under `rel/.ssh/config` is read;
    - `SSH_KNOWN_HOSTS` unset: the fetch fails earlier, at "cannot create known hosts callback: $HOME is not defined";
    - on the Fedora development host `ssh_config` v1.6.0 cannot parse the system `/etc/ssh/ssh_config` (`Match final`), which hides every value, so the repro bind-mounted an empty system config in a user and mount namespace. A host whose system config parses has no such mask.
21. **Plain commands still run with `HOME` unset**, which the BREAKING rows must not change: `virtual_home_unset.txtar` (virtual-sh and virtual-lua), its mirror `native_home_unset.txtar`, and `container_home_unset.txtar` (an auto-provisioned container command, with the suite's engine config passed through `INVOWK_CONFIG`; it also asserts no `<cwd>/.invowk-build`, which origin/main leaves behind in the same scenario). testscript cannot remove a variable, so the tests set it empty, which `os.UserHomeDir` treats as unset.

## Risks / Trade-offs

- [CI or containers that run with `HOME` unset or relative lose the module cache] → The error names the variable and the absolute-path rule. `INVOWK_MODULES_PATH` set to an absolute path is the documented workaround (Migration notes).
- [A temp-dir build context may not be visible to Docker Snap, which cannot reach `/tmp`] → It only applies when the home is relative or unavailable, or when a caller without `CacheDir` has an absolute but missing home (D5 row d). In the runtime path an absolute home always gets the default `CacheDir`, as today (decision 15). A configured `container.auto_provision.cache_dir` still wins.
- [The legacy oracle in the zero-change tests drifts from what main actually did] → Each oracle is main's function at the merge base with only the home lookup injected (the build-context oracle keeps main's branch choice and drops the directory creation). It is never edited afterwards, and the BREAKING rows are the only listed differences.
- [F1 lands with a test that needs every anchor non-empty] → With an absolute home every anchor stays non-empty (decision 16); stage 4 re-runs that test after the rebase.
- [The guard covers invowk's code only, so third-party libraries can still read `HOME` (go-git, and `ssh_config` as a fallback)] → Fetches that reach those reads (SSH transport) are refused without an absolute home and absolute `SSH_KNOWN_HOSTS` entries. Any new dependency that reads per-user files needs the same review; this is noted in the go skill (task 6.3).
- [SSH fetches with a relative home now fail, even when `SSH_KNOWN_HOSTS` is absolute] → Decision 8. The error says to use an absolute home or an HTTPS URL with a token.
- [The guard has false positives, for example a log string "HOME"] → Only exact literal matches (and `$HOME`/`${HOME}` substrings) count. The allowlist with reasons is the escape hatch, and it is reviewed like any lint exception.
- [Changing the anchor error text breaks tests that match "unknown virtual path anchor"] → Unknown names keep that text. Only known but unavailable anchors get the new message.
- [The formal `VirtualPathHarness` binding drifts] → The symbol name is kept, task 5.7 runs the correspondence check and the golden vectors, and task 7.0 re-runs them after the rebase on F1.

## Migration Plan

Release notes take the proposal's Migration notes. There is no data migration. Rollback is reverting the change. This lands last in the v0.17.0 order S1 -> C1 -> scripts-safety -> F1 -> S3 -> S4, so it rebases on the changes before it; W2 follows after v0.17.0. Stage 4 (the completion gates) starts only after F1 has merged and this branch is rebased on it (decision 16).

## Open questions

None. The three questions from the proposal stage are settled by maintainer decisions 6, 7 and 8. Follow-up candidates, which are not part of this change, are tracked in `.claude/issues/cst-adoption-gaps.md`: `TMPDIR`/`TMP`/`TEMP` validation through a `basedir.TempDir()` (decision 6, N8), Docker/Podman per-user config read through a relative `HOME`/XDG environment (decision 9, a product decision, N9), and `setupAuth` trying SSH keys before HTTPS tokens whatever the URL (`internal/app/modulesync/git.go:286-300`, N10). Moving the provisioning cache (decision 7) stays a candidate.
