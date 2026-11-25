# cross-platform-path-anchors Specification

## Purpose
Defines the standard path anchors available to the virtual runtimes, the explicit allow roots they expand to, platform-scoped virtual filesystem path mappings, and how these paths are exposed consistently through the runtime bridges. It covers path naming and resolution only; the access modes that gate those paths are specified in virtual-filesystem-access and the broader harness policy in virtual-runtime-sandbox.

## Requirements

### Requirement: Standard path anchors
Invowk SHALL support logical path anchors `@config`, `@data`, `@cache`, `@state`, `@tmp`, `@home`, and `@work`. Anchor resolution SHALL be OS-native, normalized, and stable across `virtual-sh` and `virtual-lua`. The `@config`, `@data`, `@cache`, `@state` and `@home` bases SHALL be absolute and come from the shared base-directory resolver. An anchor with no base SHALL be unavailable: empty in `INVOWK_ANCHOR_*`, never an allow root, and an error when used.

#### Scenario: Linux anchors resolve using XDG variables
- **WHEN** Invowk resolves anchors on Linux
- **THEN** `@config` SHALL resolve to `$XDG_CONFIG_HOME/invowk` or `~/.config/invowk`
- **THEN** `@data` SHALL resolve to `$XDG_DATA_HOME/invowk` or `~/.local/share/invowk`
- **THEN** `@cache` SHALL resolve to `$XDG_CACHE_HOME/invowk` or `~/.cache/invowk`
- **THEN** `@state` SHALL resolve to `$XDG_STATE_HOME/invowk` or `~/.local/state/invowk`

#### Scenario: macOS anchors resolve using Library locations
- **WHEN** Invowk resolves anchors on macOS
- **THEN** `@config` SHALL resolve to `~/Library/Application Support/invowk`
- **THEN** `@data` SHALL resolve to `~/Library/Application Support/invowk`
- **THEN** `@cache` SHALL resolve to `~/Library/Caches/invowk`
- **THEN** `@state` SHALL resolve to `~/Library/Logs/invowk`

#### Scenario: Windows anchors resolve using AppData locations
- **WHEN** Invowk resolves anchors on Windows
- **THEN** `@config` SHALL resolve to `%APPDATA%\\invowk\\config`
- **THEN** `@data` SHALL resolve to `%LOCALAPPDATA%\\invowk\\data`
- **THEN** `@cache` SHALL resolve to `%LOCALAPPDATA%\\invowk\\cache`
- **THEN** `@state` SHALL resolve to `%LOCALAPPDATA%\\invowk\\state`

#### Scenario: Common anchors resolve on every platform
- **WHEN** Invowk resolves common anchors
- **THEN** `@tmp` SHALL resolve to the OS temp directory
- **THEN** `@home` SHALL resolve to the current user's home directory or profile
- **THEN** `@work` SHALL resolve to the effective command work directory

#### Scenario: Relative XDG variables fall back to home-based anchors
- **WHEN** `XDG_DATA_HOME=data` is set on Linux with an absolute home
- **THEN** `@data` SHALL resolve to `~/.local/share/invowk`, and no path under `./data` SHALL become an allow root

#### Scenario: Relative AppData variables fall back on Windows
- **WHEN** `LOCALAPPDATA` is relative on Windows with an absolute `USERPROFILE`
- **THEN** `@cache` SHALL resolve to `%USERPROFILE%\\AppData\\Local\\invowk\\cache`

#### Scenario: Relative home makes home-derived anchors unavailable
- **WHEN** `HOME=.` is set on Linux, the `XDG_*` variables are unset, and a `virtual-lua` script calls `invowk.path("@config")`
- **THEN** the call SHALL fail with an error that names the anchor as unavailable, `INVOWK_ANCHOR_HOME` and `INVOWK_ANCHOR_CONFIG` SHALL be empty, and no relative path such as `.config/invowk` SHALL become an allow root

#### Scenario: Unset home makes home-derived anchors unavailable
- **WHEN** `HOME` is unset on Linux and the `XDG_*` variables are unset
- **THEN** `@home`, `@config`, `@data`, `@cache` and `@state` SHALL be unavailable and their `INVOWK_ANCHOR_*` variables empty, and `@tmp` and `@work` SHALL resolve as before

#### Scenario: A command that uses no home-derived anchor runs without a home
- **WHEN** `HOME` is unset on Linux, as in a systemd system unit without `User=`, and a `virtual-sh` or `virtual-lua` command that reads only files under its work directory runs
- **THEN** the command SHALL run and succeed as before

#### Scenario: An absolute XDG value keeps its anchor without a home
- **WHEN** `HOME` is unset and `XDG_CONFIG_HOME` is an absolute path on Linux
- **THEN** `@config` SHALL resolve to `$XDG_CONFIG_HOME/invowk`

#### Scenario: A path mapping on an unavailable anchor fails closed
- **WHEN** `HOME=.` is set on Linux and the selected platform declares `virtual: {filesystem: {paths: {DOTFILES: "@home/.dotfiles"}}}`
- **THEN** the `virtual-sh` or `virtual-lua` command SHALL fail before its script runs, with an error that names `@home` as unavailable, and no path under the current directory SHALL become an allow root

### Requirement: Anchor allow roots are explicit
Invowk SHALL automatically allow restricted virtual runtime access to `@config`, `@data`, `@cache`, `@state`, `@tmp`, and `@work`, except that `@work` SHALL NOT be an allow root when the effective work directory is not one: a work directory that a module's own invowkfile declares and that resolves physically outside the module directory (virtual-runtime-sandbox "The work directory is an allowed root unless a module declares it outside the module"). `@work` SHALL still resolve to that directory. Invowk SHALL resolve `@home` but MUST NOT grant blanket recursive home-directory access in restricted mode unless the selected platform explicitly maps it through `virtual.filesystem.paths`.

#### Scenario: App-scoped anchor access is allowed
- **WHEN** a `virtual-lua` script opens `invowk.path("@cache/build.json")`
- **THEN** Invowk SHALL allow the access if the resolved path remains under the `@cache` root

#### Scenario: Home anchor does not grant implicit home access
- **WHEN** a virtual script tries to read `@home/.ssh/id_rsa` in restricted mode without an explicit selected-platform `virtual.filesystem.paths` mapping
- **THEN** Invowk SHALL block the access

#### Scenario: Home anchor can be explicitly mapped
- **WHEN** an implementation platform declares a `virtual.filesystem.paths` entry that resolves to a path under `@home`
- **THEN** virtual scripts SHALL be able to access only that mapped path subtree

#### Scenario: Work anchor of a trusted work directory is allowed
- **WHEN** the effective work directory is the `--ivk-workdir` override, a workdir of an invowkfile that is not in a module, a module's default work directory, or a module-declared workdir inside the module, and a restricted virtual script opens `invowk.path("@work/out.json")`
- **THEN** Invowk SHALL allow the access if the resolved path remains under `@work`

#### Scenario: Work anchor of a module workdir outside the module is not a root
- **WHEN** a module declares a workdir that resolves physically outside the module directory, and a restricted virtual script opens `invowk.path("@work/out.json")` or reads `$INVOWK_ANCHOR_WORK/data.txt`
- **THEN** `invowk.path` and `INVOWK_ANCHOR_WORK` SHALL report the work directory, and Invowk SHALL refuse the access unless another allowed root, such as a `virtual.filesystem.paths` entry for `@work`, encloses the resolved path

### Requirement: Platform-scoped virtual filesystem path mappings
Invowk SHALL support `platforms[].virtual.filesystem.paths` on implementation platform configuration. Each key SHALL be a logical path name usable by scripts and shell environment injection. Values SHALL be non-empty platform-local path strings for the selected platform.

#### Scenario: Selected platform logical path mapping
- **WHEN** a selected implementation platform declares `virtual: {filesystem: {paths: {DB_ROOT: "./db"}}}`
- **THEN** Invowk SHALL resolve `DB_ROOT` relative to the invowkfile/module context and expose that path handle for the selected platform

#### Scenario: Path mappings are not platform-keyed objects
- **WHEN** an implementation platform declares `virtual: {filesystem: {paths: {DB_ROOT: {linux: "/var/lib/app"}}}}`
- **THEN** Invowk SHALL reject the configuration before execution

#### Scenario: Logical path names are valid environment suffixes
- **WHEN** a `virtual.filesystem.paths` key contains lowercase letters, spaces, punctuation, or starts with a digit
- **THEN** Invowk SHALL reject the key before execution

### Requirement: Path bridge exposure is consistent
Invowk SHALL expose resolved anchors and logical path mappings consistently to `virtual-sh` and `virtual-lua`.

#### Scenario: Shell anchor variables are injected
- **WHEN** a `virtual-sh` command runs
- **THEN** Invowk SHALL inject `INVOWK_ANCHOR_CONFIG`, `INVOWK_ANCHOR_DATA`, `INVOWK_ANCHOR_CACHE`, `INVOWK_ANCHOR_STATE`, `INVOWK_ANCHOR_TMP`, `INVOWK_ANCHOR_HOME`, and `INVOWK_ANCHOR_WORK`

#### Scenario: Shell logical path variables are injected
- **WHEN** a selected implementation platform declares `virtual: {filesystem: {paths: {DB_ROOT: "./db"}}}`
- **THEN** a `virtual-sh` command SHALL receive `INVOWK_PATH_DB_ROOT` with the resolved path

#### Scenario: Lua path bridge resolves anchors and mappings
- **WHEN** a `virtual-lua` script calls `invowk.path("@data")` or `invowk.path("DB_ROOT")`
- **THEN** Invowk SHALL return the resolved, sanitized host path for the active platform

#### Scenario: Path bridge rejects unknown names
- **WHEN** a script calls `invowk.path("MISSING_PATH")`
- **THEN** Invowk SHALL return an error instead of guessing or falling back to a host path
