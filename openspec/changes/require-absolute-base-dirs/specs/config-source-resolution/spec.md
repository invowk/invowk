## MODIFIED Requirements

### Requirement: Platform base directories must be absolute

Every base directory that the user config directory and the user commands directory (`~/.invowk/cmds`) derive from SHALL be absolute, and SHALL be resolved through the shared per-user base-directory resolver (`base-directory-resolution`). A relative `XDG_CONFIG_HOME` or `APPDATA` SHALL be ignored, as the XDG Base Directory spec requires, and the next base used. A relative home directory or `USERPROFILE` SHALL be an error, and no directory SHALL be derived from it.

#### Scenario: relative XDG_CONFIG_HOME is ignored
- **WHEN** `XDG_CONFIG_HOME=.config` is set and the current directory contains `.config/invowk/config.cue`
- **THEN** the user config file is `~/.config/invowk/config.cue`, and the file under the current directory is not loaded

#### Scenario: relative home directory is rejected
- **WHEN** the home directory is relative (for example `HOME=.`) and the current directory contains `.invowk/cmds/<module>.invowkmod`
- **THEN** no user config directory or user commands directory is derived from it, and that module is not discovered as a global module

#### Scenario: relative APPDATA is ignored on Windows
- **WHEN** `APPDATA=Roaming` is set on Windows with an absolute `USERPROFILE`
- **THEN** the user config file is `%USERPROFILE%\AppData\Roaming\invowk\config.cue`

#### Scenario: relative USERPROFILE is rejected on Windows
- **WHEN** `APPDATA` is unset and `USERPROFILE` is relative on Windows
- **THEN** no user config directory is derived, and invowk reports the absolute-path error as a diagnostic and uses the built-in defaults
