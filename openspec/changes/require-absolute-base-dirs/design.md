## Context

See proposal.md, "Why", for the motivation. Requirements: `specs/base-directory-resolution/spec.md` and the two MODIFIED deltas.

Current state, verified on main at f4969e90:

- **S1 helpers (the model):** `configDirFrom` (`internal/config/config.go:98-132`) takes injectable `goos`, `getenv` and `userHomeDir`. `absoluteEnv` (`config.go:138-143`) treats a relative `XDG_CONFIG_HOME` or `APPDATA` as unset. `absoluteHomeDir` (`config.go:150-159`) and the `USERPROFILE` branch (`config.go:109-111`) return `ErrBaseDirNotAbsolute` (`config.go:42-45`). `ConfigDir`, `CommandsDir` and `HostKeyPath` (`config.go:166-195`) are built on them. These are private to `internal/config`, so no other package can reuse them.
- **Module cache:** `DefaultDirWith` (`internal/app/modulecache/cache.go:36-55`) takes `INVOWK_MODULES_PATH` after only a non-empty check (`cache.go:37-42`; `FilesystemPath.Validate` checks non-blank only, `pkg/types/filesystem_path.go:36-41`). Otherwise it joins `os.UserHomeDir()` without an absolute check (`cache.go:45-50`). `newResolver` then runs `filepath.Abs` on the result (`internal/app/modulesync/resolver.go:78-89`), so a relative value lands under the current directory.
- **Git auth:** `trySSHAuth` (`internal/app/modulesync/git.go:303-326`) loads `~/.ssh/id_*` from `os.UserHomeDir()` (`git.go:305`) without an absolute check. These are SSH private keys, not git config: the task text said "git config", but the code reads key files. HTTPS tokens come from `GITHUB_TOKEN`, `GITLAB_TOKEN` and `GIT_TOKEN` (`git.go:329-353`). There is no SSH-agent fallback.
- **Provisioning:** `applyHostProvisionDefaults` sets `CacheDir` to `<home>/.cache/invowk/provision` from `os.UserHomeDir()` (`internal/runtime/container_provision.go:247-250`). With a relative home, that path is relative too. `resolveBuildContextParent` (`internal/provision/layer_provisioner.go:367-409`) uses `CacheDir`, then `<home>/invowk-build` (`:375-383`, existence check only), then `<cwd>/.invowk-build` (`:384-390`), and last a temp dir (`:395`). The build context is a `ctx-*` temp dir under that parent (`:294`).
- **Virtual anchors:** `standardVirtualAnchors` (`internal/runtime/virtual_policy.go:140-145`) turns a home lookup error into `""`. `standardVirtualAnchorsForOS` (`:147-186`) then uses `firstNonEmpty` (`:188-193`), so a relative XDG or AppData value is taken as given, and an empty home produces relative anchors such as `.config/invowk` (`:180-183`, `:168-169`). The resolver adds every non-empty anchor except `@home` to the allow roots (`:103-108`). An empty anchor is already rejected when used, with the message "unknown virtual path anchor" (`:289-292`). Every anchor, empty ones included, is injected as `INVOWK_ANCHOR_*` (`:300-304`).
- **Run locks:** `lockFilePathWith` (`internal/container/run_lock_linux.go:80-86`) uses `XDG_RUNTIME_DIR` as given, and falls back to `os.TempDir()` only when it is empty. `internal/testutil/container_suite_lock_linux.go:52-54` repeats the pattern.
- **Test infrastructure:** `ContainerSafeTempDir` uses `os.UserHomeDir()` (`internal/testutil/testutil.go:148`). `SetHomeDir` sets `HOME` or `USERPROFILE` (`internal/testutil/home.go:42-50`). `tests/cli/container_harness.go:363-375` rebuilds the config dir from the testscript env by hand.
- **Search result:** `grep` for `os.UserHomeDir|UserCacheDir|UserConfigDir`, and for the string literals `HOME`, `XDG_*`, `APPDATA`, `LOCALAPPDATA` and `USERPROFILE` in non-test Go code, finds exactly the sites above. Nothing else exists.
- **Lint:** forbidigo is enabled (`.golangci.toml:164-165`) with one pattern (`:422-426`) and is already excluded for `_test.go` (`:698-700`).
- **Windows semantics:** Go's `filepath.IsAbs` on Windows requires a volume and a root, or a UNC path. So `\Users\me` and `C:Users\me` are relative and are rejected or ignored. `filepath.IsAbs` follows the host OS, not an injected `goos`. S1's tests already work around this with host-absolute fixtures (`internal/config/config_paths_test.go:27-29`).

## Goals / Non-Goals

**Goals:**

- One package owns every read of the home directory and of the base-directory variables, with S1's semantics, and is unit tested for each OS.
- Each adopter keeps its current absolute-path behaviour, so the paths users have today do not change. Only the relative cases change.
- A gate makes new direct lookups fail CI.

**Non-Goals:**

- Moving any directory. For example, the provisioning cache stays at `<home>/.cache/invowk/provision` and does not start honouring `XDG_CACHE_HOME`.
- `TMPDIR`, `TEMP` and `TMP` validation, `os.TempDir()` and `os.MkdirTemp("", …)`. See Open questions.
- Script-visible environment variables (for example `TMPDIR` in u-root `mktemp`, `internal/uroot/mktemp.go:65`). They belong to the script's own environment, not to invowk's trusted directories.
- `container.auto_provision.cache_dir` from config. Config is a trusted source after S1.

## Decisions

### D1. New leaf package `internal/basedir`

- It imports only the standard library, `pkg/platform` (OS name constants) and `pkg/types`. It is internal because it is not a public API. It is a leaf, so `internal/config`, `internal/app/*`, `internal/provision`, `internal/runtime`, `internal/container` and `internal/testutil` can all import it without cycles.
- The API is a small value type built from injectable inputs, the same as S1's `configDirFrom`: `basedir.New(goos, getenv, userHomeDir)`, and `basedir.OS()` for the real process.
  - Methods: `Home()`, `ConfigHome()`, `DataHome()`, `CacheHome()`, `StateHome()`, and `RuntimeDir()`, which returns the directory and whether it is available.
  - The platform defaults per kind are those the anchors use today: Linux XDG, macOS `Library/...`, Windows `APPDATA` for config and `LOCALAPPDATA` for data, cache and state.
  - Return values are `types.FilesystemPath`.
- Errors: `basedir.ErrNotAbsolute`, wrapped with the source, for example `home directory "home"` or `USERPROFILE="profile"`. `config.ErrBaseDirNotAbsolute` becomes `= basedir.ErrNotAbsolute`, so `errors.Is` at existing call sites keeps working and the S1 message text is kept.
- Rejected alternatives:
  - Putting the helpers in `internal/config`: `internal/provision` and `internal/container` would then depend on config for a cross-cutting OS concern.
  - Putting them in `pkg/platform`: that would add new public API surface.
  - One helper copy per package: that is the drift this change removes.

### D2. Adopters keep their paths, and each adopter handles its own unavailable case

- `internal/config`: `configDirFrom` and `commandsDirFrom` delegate to `basedir`. `ConfigDir` on Windows uses `ConfigHome`, which is `APPDATA`, or else `USERPROFILE\AppData\Roaming`. The S1 tests stay green unchanged.
- Module cache:
  - A relative `INVOWK_MODULES_PATH` is rejected with a new typed error that names the variable. This check belongs to `modulecache`, because the variable is invowk's own.
  - The default comes from `Home()`.
  - `DefaultDirWith` gains the home lookup as an injected input, so tests do not mutate `HOME`.
- Git: `trySSHAuth` calls `basedir.OS().Home()` and returns nil on error, which skips straight to HTTPS tokens.
- Provisioning:
  - `applyHostProvisionDefaults` leaves `CacheDir` empty when `Home()` fails.
  - `resolveBuildContextParent` uses `Home()` and then the temp dir. The `<cwd>/.invowk-build` branch is deleted.
- Virtual anchors:
  - `standardVirtualAnchorsForOS` builds each anchor from the `basedir` method for its kind.
  - An unavailable base gives `""`. Existing code then keeps it out of the allow roots (`virtual_policy.go:105`) and rejects it when used (`:290`).
  - The error text changes from "unknown" to "unavailable", with the reason, when the name is a known anchor.
  - `@tmp` stays `os.TempDir()`.
- Run locks: `lockFilePathWith` and the testutil suite lock use `RuntimeDir()`, then `os.TempDir()`.
- Test infrastructure:
  - `ContainerSafeTempDir` uses `basedir.OS().Home()` and calls `t.Fatalf` on error.
  - `tests/cli/container_harness.go` builds a resolver from the testscript env (`basedir.New(goos, env.Getenv, homeFromEnv)`), where `homeFromEnv` comes from `basedir`, so it reproduces exactly what the binary does.

### D3. Two-layer mechanical guard

- forbidigo patterns `^os\.UserHomeDir$`, `^os\.UserConfigDir$` and `^os\.UserCacheDir$` go in `.golangci.toml`, with one `[[linters.exclusions.rules]]` entry for `internal/basedir/`. Test files are already excluded.
- An AST test, `internal/basedir/guard_test.go`, parses every non-test `.go` file under `cmd/`, `internal/`, `pkg/` and `tests/`.
  - It fails on a string literal equal to `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA` or matching `^XDG_[A-Z_]+$` outside `internal/basedir/`.
  - Each failure reports file and line.
  - It has a file allowlist, each entry with a reason. The initial entry is `internal/testutil/home.go`, whose `SetHomeDir` writes those variables for tests.
  - forbidigo alone cannot see literals passed to an injected `getenv`, for example `run_lock_linux.go:81`, so the second layer is needed.
- Rejected alternative: a shell `grep` in `scripts/`. It is weaker against formatting and comments, and it is not part of `make test`.

### D4. Windows coverage

- The Windows cases (relative `APPDATA`, `LOCALAPPDATA` or `USERPROFILE`; volume-relative `C:foo`; root-relative `\foo`) run as table tests that are skipped unless `runtime.GOOS == "windows"`, because `filepath.IsAbs` follows the host.
- The injected-`goos` tests use host-absolute fixtures, as S1 does. windows-latest CI runs both.

## Maintainer decisions (2026-11-14)

1. Scope: centralise base-dir resolution in one helper package that follows the package-design rule, uses S1's semantics, is adopted everywhere and is tested on Windows too. Implemented by D1, D2 and D4.
2. XDG-style variables (including `XDG_RUNTIME_DIR`, `APPDATA` and `LOCALAPPDATA`) are ignored when relative and fall back. A relative home or `USERPROFILE` is rejected.
3. Add a mechanical guard so new code cannot bypass the helper. Implemented by D3.
4. Prefer MODIFIED of S1's base-dir requirement where it generalises. "Platform base directories must be absolute" is MODIFIED to point at the shared resolver, and the general rules live in the new `base-directory-resolution` capability.
5. Recommended by the advisor workflow and taken without asking, per the standing instruction for this programme:
   - reject a relative `INVOWK_MODULES_PATH`, which matches S1's `INVOWK_CONFIG` rule;
   - remove the `<cwd>/.invowk-build` fallback in favour of the temp dir;
   - make home-derived anchors unavailable, not relative, when the home directory is unavailable;
   - keep every existing absolute path unchanged.

## Risks / Trade-offs

- [CI or containers that run with `HOME` unset or relative lose the module cache] → The error names the variable and the absolute-path rule. `INVOWK_MODULES_PATH` set to an absolute path is the documented workaround (Migration notes).
- [A temp-dir build context may not be visible to Docker Snap, which cannot reach `/tmp`] → It only applies when no absolute home exists. The old cwd fallback was a trust hole. A configured `container.auto_provision.cache_dir` still wins.
- [The guard has false positives, for example a log string "HOME"] → Only exact literal matches count. The allowlist with reasons is the escape hatch, and it is reviewed like any lint exception.
- [Changing the anchor error text breaks tests that match "unknown virtual path anchor"] → Unknown names keep that text. Only known but unavailable anchors get the new message.

## Migration Plan

Release notes take the proposal's Migration notes. There is no data migration. Rollback is reverting the change. This lands last in the v0.17.0 order S1 -> C1 -> F1 -> S3 -> S4, so it rebases on the config and audit changes before it.

## Open questions

- Should `os.TempDir()` (driven by `TMPDIR`, or `TMP`/`TEMP` on Windows) get the same treatment? A relative `TMPDIR` makes `@tmp`, `os.CreateTemp("", …)` and the temp-dir fallbacks resolve under the current directory. It is used at about ten sites (for example `internal/runtime/native.go:276` and `internal/container/run_lock_linux.go:83`). It is deferrable: adding a `basedir.TempDir()` later fits D1 without changing these specs. Proposed as a follow-up change.
- Should the provisioning cache move to `CacheHome()` (honouring `XDG_CACHE_HOME`) in a later release? It would be a user-visible relocation, so it is out of scope here.
