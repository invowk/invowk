## Purpose

Defines how invowk derives the per-user base directories (home, config, data, cache, state, runtime and Windows AppData) from the OS environment. It requires every derived directory to be absolute, so no directory invowk trusts or writes can resolve under the current directory.

## ADDED Requirements

### Requirement: Per-user base directories are resolved in one place

Every per-user directory invowk derives SHALL come from one shared resolver, which owns the reads of the home directory and of the `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA` and `XDG_*` variables. This covers the user config and commands directories, the module cache, provisioning directories, SSH key lookup, virtual-runtime anchors and the container run lock. The resolver SHALL return the same result for the same environment on every call site.

#### Scenario: all callers agree on a relative XDG value
- **WHEN** `XDG_CONFIG_HOME=.config` is set on Linux and both `invowk config path` and a `virtual-sh` command that prints `$INVOWK_ANCHOR_CONFIG` run
- **THEN** both report a directory under the absolute home (`~/.config/invowk`), and neither reports a path under the current directory

### Requirement: Relative XDG and AppData variables are ignored

A relative value of `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_CACHE_HOME`, `XDG_STATE_HOME`, `XDG_RUNTIME_DIR`, `APPDATA` or `LOCALAPPDATA` SHALL be treated as unset, as the XDG Base Directory spec requires, and the platform default for that base SHALL be used. An empty value SHALL also count as unset.

#### Scenario: relative XDG_CACHE_HOME falls back
- **WHEN** `XDG_CACHE_HOME=cache` is set on Linux with an absolute home
- **THEN** the cache base is `~/.cache`, and no path under `./cache` is used

#### Scenario: relative LOCALAPPDATA falls back on Windows
- **WHEN** `LOCALAPPDATA=AppData\Local` is set on Windows with an absolute `USERPROFILE`
- **THEN** the local AppData base is `%USERPROFILE%\AppData\Local`

### Requirement: A relative home directory is rejected

A relative home directory (for example `HOME=.`) or a relative `USERPROFILE` on Windows SHALL be an error that says the base directory must be absolute. No directory SHALL be derived from it, and the current directory SHALL NOT be used in its place. A base that falls back to the home directory SHALL fail the same way.

#### Scenario: relative HOME on Linux
- **WHEN** `HOME=.` is set on Linux and `XDG_DATA_HOME` is unset
- **THEN** resolving the home or the data base fails with the absolute-path error, and no path under the current directory is returned

#### Scenario: relative USERPROFILE on Windows
- **WHEN** `APPDATA` is unset and `USERPROFILE=Users\me` is set on Windows
- **THEN** resolving the roaming AppData base fails with the absolute-path error

#### Scenario: an absolute XDG value does not need the home directory
- **WHEN** `HOME=.` and `XDG_CONFIG_HOME=/etc/xdg-user` are set on Linux
- **THEN** the config base is `/etc/xdg-user`

### Requirement: The module cache location is absolute

The module cache directory SHALL be `INVOWK_MODULES_PATH` when it is set and non-empty, otherwise `<home>/.invowk/modules`. A relative `INVOWK_MODULES_PATH` SHALL be rejected with an error that names the variable and asks for an absolute path; it SHALL NOT be resolved against the current directory. A relative home directory SHALL make the default cache unavailable.

#### Scenario: relative INVOWK_MODULES_PATH is rejected
- **WHEN** `INVOWK_MODULES_PATH=.modules` is set and `invowk module sync` runs
- **THEN** the command fails, stderr names `INVOWK_MODULES_PATH` and says it must be absolute, and nothing is read from or written to `./.modules`

#### Scenario: relative home disables the default cache
- **WHEN** `INVOWK_MODULES_PATH` is unset, `HOME=.` is set, and the current directory contains `.invowk/modules/`
- **THEN** commands that need the module cache fail with the absolute-path error, and that directory is never treated as the cache

### Requirement: Provisioning directories never resolve under the current directory

The default provisioning cache (`<home>/.cache/invowk/provision`) and the default build-context parent (`<home>/invowk-build`) SHALL be derived only from an absolute home. When no absolute home is available, the build-context parent SHALL be a fresh directory created in the OS temp directory and removed after the build. The current directory SHALL never be used.

#### Scenario: relative HOME uses a temporary build context
- **WHEN** `HOME=.` is set, no cache dir is configured, and a container command with auto-provisioning runs
- **THEN** the build context is created under the OS temp directory, and no `.cache/invowk`, `invowk-build` or `.invowk-build` directory is created in the current directory

### Requirement: SSH keys for git fetches come only from an absolute home

Git module fetches SHALL look for SSH private keys (`.ssh/id_ed25519`, `.ssh/id_rsa`, `.ssh/id_ecdsa`) only under an absolute home directory. When the home directory is relative or unavailable, SSH key lookup SHALL be skipped and the HTTPS token sources SHALL still be tried.

#### Scenario: planted key under the current directory is ignored
- **WHEN** `HOME=.` is set, the current directory contains `.ssh/id_ed25519`, and a git module is fetched
- **THEN** that key is not loaded, and authentication uses a configured HTTPS token or none

### Requirement: The container run lock ignores a relative runtime dir

The container run lock file SHALL be placed in `XDG_RUNTIME_DIR` only when that variable is an absolute path, and otherwise in the OS temp directory.

#### Scenario: relative XDG_RUNTIME_DIR
- **WHEN** `XDG_RUNTIME_DIR=run` is set on Linux and a container command takes the run lock
- **THEN** the lock file is created in the OS temp directory and not under `./run`

### Requirement: Direct base-directory lookups are rejected by a repository gate

The repository SHALL have a mechanical gate, run by `make lint` or `make test`, that fails when production Go code outside the shared resolver calls `os.UserHomeDir`, `os.UserConfigDir` or `os.UserCacheDir`, or names `HOME`, `USERPROFILE`, `APPDATA`, `LOCALAPPDATA` or an `XDG_*` variable as a string literal. Exceptions SHALL be listed by file, each with a reason.

#### Scenario: a new direct lookup fails the gate
- **WHEN** a production file outside the resolver package adds `os.UserHomeDir()` or `os.Getenv("XDG_DATA_HOME")`
- **THEN** the gate fails and names the file and line

#### Scenario: listed exceptions pass
- **WHEN** an exempt file, such as the test helper that sets `HOME` for tests, names `HOME`
- **THEN** the gate passes
