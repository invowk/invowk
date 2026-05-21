## ADDED Requirements

### Requirement: Standard path anchors
Invowk SHALL support logical path anchors `@config`, `@data`, `@cache`, `@state`, `@tmp`, `@home`, and `@work`. Anchor resolution SHALL be OS-native, normalized, and stable across `virtual-sh` and `virtual-lua`.

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

### Requirement: Anchor allow roots are explicit
Invowk SHALL automatically allow restricted virtual runtime access to `@config`, `@data`, `@cache`, `@state`, `@tmp`, and `@work`. Invowk SHALL resolve `@home` but MUST NOT grant blanket recursive home-directory access in restricted mode unless the selected platform explicitly maps it through `virtual.filesystem.paths`.

#### Scenario: App-scoped anchor access is allowed
- **WHEN** a `virtual-lua` script opens `invowk.path("@cache/build.json")`
- **THEN** Invowk SHALL allow the access if the resolved path remains under the `@cache` root

#### Scenario: Home anchor does not grant implicit home access
- **WHEN** a virtual script tries to read `@home/.ssh/id_rsa` in restricted mode without an explicit selected-platform `virtual.filesystem.paths` mapping
- **THEN** Invowk SHALL block the access

#### Scenario: Home anchor can be explicitly mapped
- **WHEN** an implementation platform declares a `virtual.filesystem.paths` entry that resolves to a path under `@home`
- **THEN** virtual scripts SHALL be able to access only that mapped path subtree

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
