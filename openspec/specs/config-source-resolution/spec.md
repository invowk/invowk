# config-source-resolution Specification

## Purpose
This capability defines where invowk reads its configuration from: the `--ivk-config` flag, the `INVOWK_CONFIG` environment variable and the per-user config file, in that order, falling back to built-in defaults. It also defines how a missing or invalid source is handled and which `config` subcommands read or write which file. The current directory is never a config source, so a repository cannot change invowk's includes, LLM endpoint or provisioning just by being the working directory.

## Requirements

### Requirement: Config is never read from the current directory

invowk SHALL NOT load configuration from the process's current working directory or from any location derived from it. A `config.cue` in the current directory SHALL have no effect on any command.

#### Scenario: cwd config.cue is ignored when no user config exists
- **WHEN** the user config file does not exist, `--ivk-config` and `INVOWK_CONFIG` are unset, and the current directory contains a `config.cue` that sets `default_runtime: "virtual-sh"`
- **THEN** invowk uses the built-in defaults (`default_runtime` is `native`), and nothing in the output names the cwd `config.cue` as a config source

#### Scenario: cwd config.cue cannot add includes or redirect the LLM endpoint
- **WHEN** the current directory contains a `config.cue` that declares `includes` pointing at a module and sets `llm.api.base_url` and `llm.api.api_key_env`, and no explicit or user config exists
- **THEN** the included module's commands are not discovered, and LLM resolution does not use that `base_url` or `api_key_env`

#### Scenario: config show reports defaults instead of the cwd file
- **WHEN** `invowk config show` runs in a directory that contains `config.cue` and no other config source applies
- **THEN** the reported config source is the built-in defaults, not the cwd file

### Requirement: Recognised config sources

The only config sources SHALL be the `--ivk-config` flag, the `INVOWK_CONFIG` environment variable and the per-user config file `config.cue` in the OS-specific invowk config directory. When none of them applies, built-in defaults SHALL be used.

#### Scenario: user config file on Linux and other Unix-like systems
- **WHEN** invowk runs on Linux with no explicit source
- **THEN** the user config file is `$XDG_CONFIG_HOME/invowk/config.cue` when `XDG_CONFIG_HOME` is set, otherwise `~/.config/invowk/config.cue`

#### Scenario: user config file on macOS
- **WHEN** invowk runs on macOS with no explicit source
- **THEN** the user config file is `~/Library/Application Support/invowk/config.cue`

#### Scenario: user config file on Windows
- **WHEN** invowk runs on Windows with no explicit source
- **THEN** the user config file is `%APPDATA%\invowk\config.cue`

#### Scenario: defaults when no source applies
- **WHEN** `--ivk-config` and `INVOWK_CONFIG` are unset and the user config file does not exist
- **THEN** invowk uses the built-in defaults and reports the source as defaults

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

### Requirement: Config source precedence

invowk SHALL select exactly one config source, using this precedence: `--ivk-config`, then `INVOWK_CONFIG`, then the user config file, then built-in defaults. The selected source SHALL be used on its own; invowk SHALL NOT merge values from lower-precedence sources into it.

#### Scenario: flag beats environment variable
- **WHEN** `INVOWK_CONFIG` names a valid file A and `--ivk-config` names a valid file B
- **THEN** invowk loads B only

#### Scenario: environment variable beats user config
- **WHEN** a valid user config file exists, `--ivk-config` is not given, and `INVOWK_CONFIG` names a valid file A
- **THEN** invowk loads A only, and values from the user config file are not applied

#### Scenario: user config used when no explicit source
- **WHEN** `--ivk-config` is not given, `INVOWK_CONFIG` is unset or empty, and a valid user config file exists
- **THEN** invowk loads the user config file

### Requirement: Lower-precedence sources are not evaluated

Once a source is selected, invowk SHALL NOT read, parse or validate any lower-precedence source. When `--ivk-config` is given, `INVOWK_CONFIG` SHALL be neither read nor validated.

#### Scenario: flag masks an invalid environment value
- **WHEN** `INVOWK_CONFIG` is a relative path and `--ivk-config` names a valid file B
- **THEN** invowk loads B and reports no error about `INVOWK_CONFIG`

#### Scenario: environment variable masks an invalid user config
- **WHEN** `INVOWK_CONFIG` names a valid file A and the user config file contains invalid CUE
- **THEN** invowk loads A and reports no diagnostic about the user config file

### Requirement: Empty INVOWK_CONFIG means unset

An `INVOWK_CONFIG` value that is the empty string SHALL be treated exactly as if the variable were unset.

#### Scenario: empty environment variable is unset
- **WHEN** `INVOWK_CONFIG` is set to the empty string and a valid user config file exists
- **THEN** invowk loads the user config file, exactly as if `INVOWK_CONFIG` were unset

### Requirement: Every config-reading command applies the same precedence

Every command that reads config SHALL select its source with the same precedence. This covers `invowk cmd` (listing, execution, dry-run, watch), shell completion, the dynamic command registration that runs before flag parsing, `invowk validate`, `invowk audit`, `invowk agent`, `invowk config show` and `invowk config dump`. `config set` and `config init` are governed by their own requirements.

#### Scenario: validate honours the flag
- **WHEN** `invowk --ivk-config B validate` runs with no path argument
- **THEN** workspace validation uses the config from B, including its `includes`

#### Scenario: config show and config dump honour the environment variable
- **WHEN** `INVOWK_CONFIG` names a valid file A and `invowk config show` or `invowk config dump` runs
- **THEN** the output reflects A's values, and `config show` names A as the effective config file with `INVOWK_CONFIG` as its source

#### Scenario: config show honours the flag
- **WHEN** `invowk --ivk-config B config show` runs
- **THEN** the output reflects B's values and names B as the effective config file with `--ivk-config` as its source

#### Scenario: audit honours the environment variable
- **WHEN** `INVOWK_CONFIG` names a valid file A whose `includes` name a module and `invowk audit` runs on a target that needs config
- **THEN** the audit uses A's config

### Requirement: config path reports the effective source without loading it

`invowk config path` SHALL print the config directory and the user config file. When `--ivk-config` or `INVOWK_CONFIG` selects a file, it SHALL also print that file as the effective config file and name its source. It SHALL NOT load or validate that file; a missing file or a relative `INVOWK_CONFIG` SHALL be marked in the output instead of failing.

#### Scenario: config path names the environment source
- **WHEN** `INVOWK_CONFIG` names a valid file A and `invowk config path` runs
- **THEN** the output names A as the effective config file and identifies `INVOWK_CONFIG` as the source

#### Scenario: config path reports a missing file without failing
- **WHEN** `INVOWK_CONFIG` names an absolute path to a file that does not exist and `invowk config path` runs
- **THEN** the command exits 0 and prints that path as the effective config file, marked as coming from `INVOWK_CONFIG` and as missing

#### Scenario: config path reports a relative environment value without failing
- **WHEN** `INVOWK_CONFIG=config.cue` and `invowk config path` runs
- **THEN** the command exits 0 and marks the `INVOWK_CONFIG` value as invalid because it must be absolute

### Requirement: INVOWK_CONFIG must be an absolute path

A non-empty `INVOWK_CONFIG` value SHALL be an absolute path for the host OS. A relative value SHALL be rejected as an explicit-config error, with a message that names the variable and asks for an absolute path. invowk SHALL NOT resolve it against the current directory. `--ivk-config` MAY be relative, because the user typed it on that command line.

#### Scenario: relative INVOWK_CONFIG rejected
- **WHEN** `INVOWK_CONFIG=config.cue` and a file `config.cue` exists in the current directory, and `invowk cmd hello` runs
- **THEN** the invocation exits 1 before any command runs, and stderr names `INVOWK_CONFIG` and says it must be absolute

#### Scenario: relative flag still accepted
- **WHEN** `--ivk-config custom/config.cue` names an existing valid file relative to the current directory
- **THEN** invowk loads that file

### Requirement: Explicit config sources fail closed

When the selected source is `--ivk-config` or `INVOWK_CONFIG` and the named file is missing, unreadable, not valid CUE, or fails schema or Go validation, invowk SHALL stop before discovering or running any user command, and SHALL NOT fall back to the user config file or to defaults. Except for the commands in "Explicit config failures outside the exit-1 flows", the exit code SHALL be 1.

#### Scenario: missing file named by the flag
- **WHEN** `invowk cmd hello --ivk-config nonexistent.cue` runs
- **THEN** the exit code is 1 and the `hello` script does not run

#### Scenario: missing file named by the environment variable
- **WHEN** `INVOWK_CONFIG` names an absolute path that does not exist and `invowk cmd hello` runs
- **THEN** the exit code is 1 and the `hello` script does not run

#### Scenario: invalid explicit config
- **WHEN** `INVOWK_CONFIG` or `--ivk-config` names a file that contains invalid CUE or an unknown field
- **THEN** the exit code is 1, the actionable config-load error is shown, and no command runs

#### Scenario: no fallback to the user config file
- **WHEN** a valid user config file exists and `--ivk-config` names a missing file
- **THEN** the exit code is 1 and the user config file is not loaded

#### Scenario: config show fails on a missing explicit file
- **WHEN** `invowk --ivk-config nonexistent.cue config show` runs
- **THEN** the exit code is 1 and no config values are printed

#### Scenario: validate fails on a missing explicit file
- **WHEN** `invowk --ivk-config nonexistent.cue validate` runs with no path argument
- **THEN** the exit code is 1 and no workspace validation result is printed

### Requirement: Explicit config failures outside the exit-1 flows

`invowk audit` SHALL report a failed explicit config source through its documented scan-error exit code, 2, not 1 (`audit` uses 1 for findings). `invowk agent` SHALL follow the general exit-code contract and exit 1. Shell completion SHALL offer no candidates. `invowk cmd --help` MAY render help without discovered commands. None of them SHALL fall back to the user config file or to defaults.

#### Scenario: audit reports the config failure as an audit error
- **WHEN** `INVOWK_CONFIG` names an absolute path that does not exist and `invowk audit` runs on a target that needs config
- **THEN** the exit code is audit's error code, not its findings code, and no scan result is printed

#### Scenario: agent reports the config failure as an agent error
- **WHEN** `--ivk-config` names a missing file and an `invowk agent cmd create` command runs
- **THEN** the exit code is 1, no LLM request is made and no file is written

#### Scenario: completion offers nothing
- **WHEN** `--ivk-config` names a missing file and `invowk __complete cmd ''` runs in a directory with an `invowkfile.cue`
- **THEN** no command candidates are offered

### Requirement: Explicit config errors name the source

The error for a failed explicit config source SHALL name the source (`--ivk-config` or `INVOWK_CONFIG`) and the path, and SHALL give a hint for fixing it: correct the path, or unset the variable. The hint SHALL mention the absolute-path rule only when a relative `INVOWK_CONFIG` caused the failure.

#### Scenario: flag error names the flag and the path
- **WHEN** `invowk cmd hello --ivk-config nonexistent.cue` runs
- **THEN** stderr names `nonexistent.cue` and `--ivk-config`

#### Scenario: environment error names the variable and the path
- **WHEN** `INVOWK_CONFIG` names an absolute path that does not exist and `invowk cmd hello` runs
- **THEN** stderr names the path and `INVOWK_CONFIG`, and the hint does not say the path must be absolute

#### Scenario: relative environment value hint names the absolute-path rule
- **WHEN** `INVOWK_CONFIG=config.cue` is set and `invowk config dump` runs
- **THEN** the hint says to fix or unset `INVOWK_CONFIG` and that it must be an absolute path

### Requirement: Explicit config failures give source-specific guidance

The guidance shown for a failed `--ivk-config` or `INVOWK_CONFIG` source SHALL NOT advise running `invowk config init` or deleting the user config file, because neither fixes the explicit source. It SHALL say how to fix the named file or drop the explicit source. A failed user config file keeps its existing guidance.

#### Scenario: explicit failure guidance does not touch the user config
- **WHEN** `INVOWK_CONFIG` or `--ivk-config` names a missing file and `invowk config show` runs
- **THEN** stderr says how to fix or drop the explicit source, and does not suggest `invowk config init` or removing the user config file

#### Scenario: user config failure keeps the user-file guidance
- **WHEN** no explicit source is set, the user config file is invalid and `invowk config show` runs
- **THEN** stderr still suggests `invowk config init`

### Requirement: Invalid user config keeps falling back to defaults

When no explicit source is given and the user config file is invalid, command discovery and execution SHALL report a config-load diagnostic and continue with built-in defaults, as before this change.

#### Scenario: invalid user config keeps the current behaviour
- **WHEN** no explicit source is given and the user config file contains invalid CUE, and `invowk cmd hello` runs
- **THEN** invowk reports a config-load diagnostic and runs `hello` with built-in defaults

### Requirement: config set refuses while an explicit source is active

`invowk config set` SHALL read and write only the user config file. When `--ivk-config` is given or `INVOWK_CONFIG` is non-empty, it SHALL refuse with a non-zero exit and an error that names the active source and the user config file, and SHALL NOT read the explicit file or write any file.

#### Scenario: config set refuses under the flag
- **WHEN** `invowk --ivk-config B config set default_runtime virtual-sh` runs
- **THEN** the exit code is non-zero, stderr names `--ivk-config`, and neither B nor the user config file is created or changed

#### Scenario: config set refuses under the environment variable
- **WHEN** `INVOWK_CONFIG` names a file A that sets `container_engine: "docker"` and `invowk config set default_runtime virtual-sh` runs
- **THEN** the exit code is non-zero, stderr names `INVOWK_CONFIG`, A is unchanged, and the user config file is not created or changed

#### Scenario: config set writes the user file without an explicit source
- **WHEN** `--ivk-config` and `INVOWK_CONFIG` are unset, no user config file exists, and `invowk config set default_runtime virtual-sh` runs
- **THEN** the user config file is created with `default_runtime: "virtual-sh"` and the other values at their defaults

### Requirement: config init creates only the user config file

`invowk config init` SHALL create the user config file and SHALL NOT read or write a file named by `--ivk-config` or `INVOWK_CONFIG`.

#### Scenario: config init ignores the environment variable
- **WHEN** `INVOWK_CONFIG` names a file A, no user config file exists, and `invowk config init` runs
- **THEN** the user config file is created with the default contents and A is unchanged
