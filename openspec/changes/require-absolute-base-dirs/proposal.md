## Why

`harden-config-trust` (S1, merged in f4969e90) made the user config directory and the user commands directory require absolute base directories (`internal/config/config.go:91-159`). Every other per-user directory invowk derives still trusts `os.UserHomeDir()` and the XDG/AppData variables as they are. A relative `HOME`, `XDG_*_HOME`, `APPDATA`, `LOCALAPPDATA` or `XDG_RUNTIME_DIR` therefore resolves under the current directory. A cloned repository could then supply the verified module cache, the SSH keys and known hosts used for git fetches, provisioning build contexts or virtual-runtime allow roots. Each call site checks the path, or skips the check, in its own way, so this has to be fixed in one place and kept fixed by a mechanical guard. It is part of the v0.17.0 security hardening.

## What Changes

- **New:** one internal package resolves every per-user base directory: home, config, data, cache, state and runtime dirs, plus Windows roaming and local AppData. It applies S1's rules everywhere. A relative XDG-style or AppData variable is ignored and the next base is used. A relative home directory or `USERPROFILE` is an error, and no directory is derived from it. `internal/config` moves onto it without changing behaviour.
- **BREAKING:** the module cache (`~/.invowk/modules`) is never derived from a relative home. A relative `INVOWK_MODULES_PATH` is rejected with an error that names the variable, as S1 does for `INVOWK_CONFIG`. Today it is resolved against the current directory (`internal/app/modulesync/resolver.go:86`).
- **BREAKING:** container provisioning never puts its cache or build contexts under the current directory. When no absolute, existing home is available, the build-context parent falls back to a fresh directory in the OS temp dir. The current `./.invowk-build` fallback (`internal/provision/layer_provisioner.go:385-391`) is removed.
- **BREAKING:** git module fetches over SSH (`git@`, `ssh://`) need an absolute home and are refused before connecting otherwise. Both invowk's key lookup (`internal/app/modulesync/git.go:305`) and go-git's `known_hosts` lookup (`plumbing/transport/ssh/auth_method.go:268-283` in v5.19.2) read from the home, so a relative home would let a repository supply the trusted host keys. HTTPS fetches are unchanged.
- Virtual-runtime anchors (`@config`, `@data`, `@cache`, `@state`, `@home`) are never relative. A relative variable falls back to the home-based default. When the home directory is unavailable, the anchors derived from it are unavailable: they are not granted as allow roots, and using one is an error. Today they become relative paths such as `.config/invowk` (`internal/runtime/virtual_policy.go:141-144,180`).
- The container run lock ignores a relative `XDG_RUNTIME_DIR` and uses the OS temp dir.
- **New guard:** a forbidigo rule bans `os.UserHomeDir`, `os.UserConfigDir` and `os.UserCacheDir` outside the helper package. A repository test fails when production code outside the helper reads `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA` or any `XDG_*` variable by name.

## Capabilities

### New Capabilities

- `base-directory-resolution`: how invowk derives per-user base directories from the OS environment, the absolute-path rules, the module cache, provisioning, SSH-key and run-lock locations built on them, and the guard that keeps every caller on the shared resolver.

### Modified Capabilities

- `config-source-resolution`: "Platform base directories must be absolute" now derives the config and commands directories through the shared resolver, and gains Windows scenarios.
- `cross-platform-path-anchors`: "Standard path anchors" requires absolute anchor bases and defines what happens when a base is unavailable.

## Impact

- Code: new package `internal/basedir` (imports only the standard library and `pkg/platform`). Adopters: `internal/config/config.go`, `internal/app/modulecache/cache.go`, `internal/app/modulesync/git.go`, `internal/provision/layer_provisioner.go`, `internal/runtime/container_provision.go`, `internal/runtime/virtual_policy.go`, `internal/container/run_lock_linux.go`, `internal/testutil/` (`ContainerSafeTempDir`, the suite lock).
- Lint: `.golangci.toml` gains forbidigo patterns with a path exclusion for the helper package.
- Tests: unit tests for the helper on Linux, macOS and Windows (injected `goos`, `getenv` and home lookup), regression tests at each adopter, and testscript coverage for relative `HOME` and `INVOWK_MODULES_PATH`.
- Docs: `README.md`, and on the website `modules/dependencies/cli-commands.mdx` (`INVOWK_MODULES_PATH`), `runtime-modes/virtual.mdx` and `virtual-lua.mdx` (anchors), plus the pt-BR translations of the current version.
- Merge order for v0.17.0: S1 -> C1 -> F1 -> S3 -> S4 (this change).

## Migration notes

- `INVOWK_MODULES_PATH` must now be an absolute path. A relative value such as `INVOWK_MODULES_PATH=.modules` used to resolve against the current directory and is now rejected. Use an absolute path, for example `INVOWK_MODULES_PATH="$PWD/.modules"`.
- If `HOME` (or `USERPROFILE` on Windows) is relative, invowk no longer creates or reads `.invowk/modules`, `.cache/invowk/provision`, `invowk-build` or `.ssh/id_*` under the current directory. Module commands that need the cache fail with an error that names the relative home. Provisioning uses a temporary build context. SSH-transport git fetches are refused with a relative home. Use an absolute home, or an HTTPS URL that authenticates with `GITHUB_TOKEN`, `GITLAB_TOKEN` or `GIT_TOKEN` (`internal/app/modulesync/git.go:329-353`).
- A relative `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_CACHE_HOME`, `XDG_STATE_HOME`, `XDG_RUNTIME_DIR`, `APPDATA` or `LOCALAPPDATA` is now ignored everywhere, as the XDG Base Directory spec requires. Virtual-runtime anchors and the run lock fall back to their defaults.
