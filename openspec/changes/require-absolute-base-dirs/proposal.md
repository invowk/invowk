## Why

`harden-config-trust` (S1, merged in f4969e90) made the user config directory and the user commands directory require absolute base directories (`internal/config/config.go:91-184`). Every other per-user directory invowk derives still trusts `os.UserHomeDir()` and the XDG/AppData variables as they are. A relative `HOME`, `XDG_*_HOME`, `APPDATA`, `LOCALAPPDATA` or `XDG_RUNTIME_DIR` therefore resolves under the current directory. A cloned repository could then supply the verified module cache, the SSH keys and known hosts used for git fetches, provisioning build contexts or virtual-runtime allow roots. Each call site checks the path, or skips the check, in its own way, so this has to be fixed in one place and kept fixed by a mechanical guard. It is part of the v0.17.0 security hardening.

## What Changes

- **New:** one internal package resolves every per-user base directory: home, config, data, cache, state and runtime dirs, plus Windows roaming and local AppData. It applies S1's rules everywhere. A relative XDG-style or AppData variable is ignored and the next base is used. A relative home directory or `USERPROFILE` is an error, and no directory is derived from it. `internal/config` moves onto it without changing behaviour.
- **BREAKING:** the module cache (`~/.invowk/modules`) is never derived from a relative home. A relative `INVOWK_MODULES_PATH` is rejected with an error that names the variable, as S1 does for `INVOWK_CONFIG`. Today it is resolved against the current directory (`internal/app/modulesync/resolver.go:86`).
- **BREAKING:** container provisioning never puts its cache or build contexts under the current directory. The default cache dir is set only for an absolute home that exists (`internal/runtime/container_provision.go:247-250` checks neither today). Otherwise the build-context parent is a fresh directory in the OS temp dir. The current `./.invowk-build` fallback (`internal/provision/layer_provisioner.go:385-391`) is removed. An absolute but missing home (for example `HOME=/no-home`), which fails today for a non-root user when the cache dir cannot be created, now uses the temp dir.
- **BREAKING:** git module fetches over SSH (any URL that is not `https://`) need an absolute home and absolute `SSH_KNOWN_HOSTS` entries, and are refused before connecting otherwise. Invowk's key lookup (`internal/app/modulesync/git.go:305`) and go-git's `known_hosts` lookup (`plumbing/transport/ssh/auth_method.go:268-283` in v5.19.2) both read from the home. go-git also uses `SSH_KNOWN_HOSTS` entries as given, so a relative value would let a repository supply the trusted host keys. HTTPS fetches are unchanged.
- Virtual-runtime anchors (`@config`, `@data`, `@cache`, `@state`, `@home`) are never relative. A relative variable falls back to the home-based default. When the home directory is unavailable, the anchors derived from it are unavailable: they are not granted as allow roots, and using one, directly or in a `virtual.filesystem.paths` mapping, is an error. Today they become relative paths such as `.config/invowk` (`internal/runtime/virtual_policy.go:141-144,180`).
- The container run lock ignores a relative `XDG_RUNTIME_DIR` and uses the OS temp dir.
- **New guard:** an AST test run by `make test` fails when non-test code outside the helper package does any of these: calls `os.UserHomeDir`, `os.UserConfigDir` or `os.UserCacheDir` under any import name; imports `os/user`; or names `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA`, an `XDG_*` variable, `$HOME` or `${HOME}` in a string literal. A forbidigo rule gives the same signal for the three calls in the editor.

## Capabilities

### New Capabilities

- `base-directory-resolution`: how invowk derives per-user base directories from the OS environment and which absolute-path rules apply. It also covers the module cache, provisioning, SSH trust-file and run-lock locations built on them, and the guard that keeps every caller on the shared resolver.

### Modified Capabilities

- `config-source-resolution`: "Platform base directories must be absolute" now derives the config and commands directories through the shared resolver, and gains Windows scenarios.
- `cross-platform-path-anchors`: "Standard path anchors" requires absolute anchor bases and defines what happens when a base is unavailable, including in `virtual.filesystem.paths` mappings.

## Impact

- Code: new package `internal/basedir` (imports only the standard library, `pkg/platform` and `pkg/types`). Adopters: `internal/config/config.go`, `internal/app/modulecache/cache.go`, `internal/app/modulesync/git.go`, `internal/provision/config.go` and `layer_provisioner.go`, `internal/runtime/container_provision.go`, `internal/runtime/virtual_policy.go`, `internal/container/run_lock_linux.go`, `internal/testutil/` (`ContainerSafeTempDir`, the suite lock) and `tests/cli/container_harness.go`.
- Lint: `.golangci.toml` gains forbidigo patterns with a path exclusion for the helper package.
- Formal: `standardVirtualAnchorsForOS` is bound by `formal/alloy/VirtualPathHarness.als:31`; its name is kept and the correspondence check runs.
- Tests: unit tests for the helper on Linux, macOS and Windows (injected `goos`, `getenv` and home lookup), regression tests at each adopter, and testscript coverage for relative `HOME` and `INVOWK_MODULES_PATH`. The anchor testscript is exempt from a native mirror, because only the virtual runtimes inject anchors.
- Docs: `README.md`, and on the website `modules/dependencies/cli-commands.mdx` (`INVOWK_MODULES_PATH`), `runtime-modes/virtual-lua.mdx` (anchors; `virtual.mdx` has no anchor section), plus the pt-BR translations of the current version.
- Merge order for v0.17.0: S1 -> C1 -> F1 -> S3 -> S4 (this change).

## Migration notes

- `INVOWK_MODULES_PATH` must now be an absolute path. A relative value such as `INVOWK_MODULES_PATH=.modules` used to resolve against the current directory and is now rejected. Use an absolute path, for example `INVOWK_MODULES_PATH="$PWD/.modules"`.
- If `HOME` (or `USERPROFILE` on Windows) is relative, invowk no longer creates or reads `.invowk/modules`, `.cache/invowk/provision`, `invowk-build` or `.ssh/id_*` under the current directory. Module commands that need the cache fail with an error that names the relative home. Provisioning uses a temporary build context. SSH-transport git fetches are refused with a relative home or a relative `SSH_KNOWN_HOSTS` entry. Use an absolute home, or an HTTPS URL that authenticates with `GITHUB_TOKEN`, `GITLAB_TOKEN` or `GIT_TOKEN` (`internal/app/modulesync/git.go:329-353`).
- A relative `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_CACHE_HOME`, `XDG_STATE_HOME`, `XDG_RUNTIME_DIR`, `APPDATA` or `LOCALAPPDATA` is now ignored everywhere, as the XDG Base Directory spec requires. Virtual-runtime anchors and the run lock fall back to their defaults.
- With `HOME` set to an absolute directory that does not exist (for example `HOME=/no-home`), invowk no longer fails while creating `<home>/.cache/invowk/provision` and uses a temporary build context instead. The container engine may still need a usable home.
