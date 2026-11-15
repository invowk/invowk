## Purpose

Defines how invowk derives the per-user base directories (home, config, data, cache, state, runtime and Windows AppData) from the OS environment. Every derived directory must be absolute, so no directory that invowk trusts or writes can resolve under the current directory. It also defines the directories built on them (module cache, provisioning, SSH trust files, run lock) and the gate that keeps every caller on the shared resolver. The OS temp directory is used as the OS reports it.

## ADDED Requirements

### Requirement: Per-user base directories are resolved in one place

Every per-user directory invowk derives SHALL come from one shared resolver. The resolver owns every read of the home directory and of `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA` and `XDG_*`. This covers the config and commands directories, the module cache, provisioning, SSH key lookup, virtual-runtime anchors and the container run lock. For the same environment, every caller SHALL get the same result.

#### Scenario: all callers agree on a relative XDG value
- **WHEN** `XDG_CONFIG_HOME=.config` is set on Linux with an absolute home, and both `invowk config path` and a `virtual-sh` command that prints `$INVOWK_ANCHOR_CONFIG` run
- **THEN** both report `~/.config/invowk` under the absolute home, and neither reports a path under the current directory

### Requirement: Relative XDG and AppData variables are ignored

A relative or empty `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_CACHE_HOME`, `XDG_STATE_HOME`, `XDG_RUNTIME_DIR`, `APPDATA` or `LOCALAPPDATA` SHALL be treated as unset, as the XDG Base Directory spec requires. The platform default for that base SHALL then be used. `XDG_RUNTIME_DIR` has no default, so the resolver SHALL report it as unavailable and the caller SHALL use its own fallback.

#### Scenario: relative XDG_CACHE_HOME falls back
- **WHEN** `XDG_CACHE_HOME=cache` is set on Linux with an absolute home
- **THEN** the cache base is `~/.cache`, and no path under `./cache` is used

#### Scenario: relative LOCALAPPDATA falls back on Windows
- **WHEN** `LOCALAPPDATA=AppData\Local` is set on Windows with an absolute `USERPROFILE`
- **THEN** the local AppData base is `%USERPROFILE%\AppData\Local`

#### Scenario: relative XDG_RUNTIME_DIR is unavailable
- **WHEN** `XDG_RUNTIME_DIR=run` is set on Linux
- **THEN** the resolver reports the runtime dir as unavailable, and `./run` is not returned

### Requirement: A relative home directory is rejected

A relative home directory (for example `HOME=.`) or a relative `USERPROFILE` on Windows SHALL be an error that says the base directory must be absolute. No directory SHALL be derived from it, and the current directory SHALL NOT be used in its place. A base that falls back to the home directory SHALL fail the same way.

#### Scenario: relative HOME on Linux
- **WHEN** `HOME=.` is set on Linux and `XDG_DATA_HOME` is unset
- **THEN** resolving the home or the data base fails with the absolute-path error, and no path under the current directory is returned

#### Scenario: relative USERPROFILE on Windows
- **WHEN** `APPDATA` is unset and `USERPROFILE=Users\me` is set on Windows
- **THEN** resolving the roaming AppData base fails with the absolute-path error, which names `USERPROFILE`

#### Scenario: drive-relative and root-relative profiles on Windows
- **WHEN** `USERPROFILE` is `C:Users\me` or `\Users\me` on Windows
- **THEN** the home is rejected as relative, because neither names both a volume and a root

#### Scenario: an absolute XDG value does not need the home directory
- **WHEN** `HOME=.` and `XDG_CONFIG_HOME=/etc/xdg-user` are set on Linux
- **THEN** the config base is `/etc/xdg-user`

### Requirement: The module cache location is absolute

The module cache directory SHALL be `INVOWK_MODULES_PATH` when it is set and non-empty, otherwise `<home>/.invowk/modules`. A relative `INVOWK_MODULES_PATH` SHALL be rejected with an error that names the variable and asks for an absolute path; it SHALL NOT be resolved against the current directory. A relative home directory SHALL make the default cache unavailable.

#### Scenario: relative INVOWK_MODULES_PATH is rejected
- **WHEN** `INVOWK_MODULES_PATH=.modules` is set and `invowk module sync` runs in a module directory
- **THEN** the command fails, stderr names `INVOWK_MODULES_PATH` and says it must be absolute, and `./.modules` is neither read nor created

#### Scenario: relative home disables the default cache
- **WHEN** `INVOWK_MODULES_PATH` is unset, `HOME=.` is set, and the current directory contains `.invowk/modules/`
- **THEN** commands that need the module cache fail with the absolute-path error, and that directory is never treated as the cache

### Requirement: Provisioning directories never fall back to the current directory

The default provisioning directories (`<home>/.cache/invowk/provision` and `<home>/invowk-build`) SHALL be derived only from an absolute home that exists. Otherwise, unless a cache dir is configured, the build-context parent SHALL be a new, randomly named directory that invowk creates in the OS temp directory and removes after the build. The current directory SHALL NOT be used as a fallback.

#### Scenario: relative HOME uses a temporary build context
- **WHEN** `HOME=.` is set, no cache dir is configured, and a container command with auto-provisioning runs
- **THEN** the build context is created under the OS temp directory, and no `.cache/invowk`, `invowk-build` or `.invowk-build` directory is created in the current directory

#### Scenario: absolute but missing home uses a temporary build context
- **WHEN** `HOME=/no-home` is set (absolute, does not exist), no cache dir is configured, and auto-provisioning builds an image
- **THEN** the build context is created under the OS temp directory and removed after the build, and no directory under `/no-home` or the current directory is created

### Requirement: SSH git fetches require trusted SSH files

A git fetch whose URL is not `https://` (an SSH URL) SHALL run only with an absolute home, because invowk reads SSH keys from it and the git library reads `known_hosts` from it. It SHALL also need every `SSH_KNOWN_HOSTS` entry to be absolute. Otherwise it SHALL fail before connecting, with an error that names the relative value and the absolute-path rule. HTTPS fetches SHALL be unaffected.

#### Scenario: SSH fetch with a relative home is refused
- **WHEN** `HOME=.` is set, the current directory contains `.ssh/id_ed25519` and `.ssh/known_hosts`, and a module with a `git@` URL is fetched
- **THEN** the fetch fails before any network connection, stderr names the relative home, and none of those files is read

#### Scenario: relative SSH_KNOWN_HOSTS entry is refused
- **WHEN** the home is absolute, `SSH_KNOWN_HOSTS=known_hosts` is set, and a module with an `ssh://` URL is fetched
- **THEN** the fetch fails before any network connection, and stderr names `SSH_KNOWN_HOSTS`

#### Scenario: HTTPS fetch with a relative home still works
- **WHEN** `HOME=.` is set and a module with an `https://` URL is fetched with `GIT_TOKEN` set
- **THEN** the fetch proceeds with token authentication, and no file under the current directory is read as an SSH key

### Requirement: The container run lock ignores a relative runtime dir

The container run lock file SHALL be placed in `XDG_RUNTIME_DIR` only when that variable is an absolute path, and otherwise in the OS temp directory.

#### Scenario: relative XDG_RUNTIME_DIR
- **WHEN** `XDG_RUNTIME_DIR=run` is set on Linux and a container command takes the run lock
- **THEN** the lock file is created in the OS temp directory and not under `./run`

### Requirement: Direct base-directory lookups are rejected by a repository gate

`make test` SHALL fail when a non-test Go file outside the shared resolver calls `os.UserHomeDir`, `os.UserConfigDir` or `os.UserCacheDir` (under any import name), imports `os/user`, or has a string literal that is `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA` or an `XDG_*` name, or that contains `$HOME` or `${HOME}`. Exceptions SHALL be listed by file, each with a reason.

#### Scenario: a new direct lookup fails the gate
- **WHEN** a production file outside the resolver package adds `os.UserHomeDir()` or `os.Getenv("XDG_DATA_HOME")`
- **THEN** the gate fails and names the file and line

#### Scenario: an aliased import does not bypass the gate
- **WHEN** a production file imports `os` as `goos` and calls `goos.UserHomeDir()`
- **THEN** the gate fails and names the file and line

#### Scenario: listed exceptions pass
- **WHEN** an exempt file, such as the test helper that sets `HOME` for tests, names `HOME`
- **THEN** the gate passes
