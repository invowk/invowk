## MODIFIED Requirements

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

#### Scenario: A path mapping on an unavailable anchor fails closed
- **WHEN** `HOME=.` is set on Linux and the selected platform declares `virtual: {filesystem: {paths: {DOTFILES: "@home/.dotfiles"}}}`
- **THEN** the `virtual-sh` or `virtual-lua` command SHALL fail before its script runs, with an error that names `@home` as unavailable, and no path under the current directory SHALL become an allow root
