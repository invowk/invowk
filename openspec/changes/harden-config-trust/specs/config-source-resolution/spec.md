## ADDED Requirements

### Requirement: Config is never read from the current directory

invowk SHALL NOT load configuration from the process's current working directory or from any other location derived from it. The only config sources SHALL be the `--ivk-config` flag, the `INVOWK_CONFIG` environment variable, and the per-user config file at the OS-specific config directory (`$XDG_CONFIG_HOME/invowk/config.cue` or `~/.config/invowk/config.cue` on Linux and other Unix-like systems, `~/Library/Application Support/invowk/config.cue` on macOS, `%APPDATA%\invowk\config.cue` on Windows). When none of them applies, built-in defaults SHALL be used.

#### Scenario: cwd config.cue is ignored when no user config exists
- **WHEN** the user config file does not exist, `--ivk-config` and `INVOWK_CONFIG` are unset, and the current directory contains a `config.cue` that sets `default_runtime: "virtual-sh"`
- **THEN** invowk uses the built-in defaults (`default_runtime` is `native`), and nothing in the output names the cwd `config.cue` as a config source

#### Scenario: cwd config.cue cannot add includes or redirect the LLM endpoint
- **WHEN** the current directory contains a `config.cue` that declares `includes` pointing at a module and sets `llm.api.base_url` and `llm.api.api_key_env`, and no explicit or user config exists
- **THEN** the included module's commands are not discovered, and LLM resolution does not use that `base_url` or `api_key_env`

#### Scenario: config show reports defaults instead of the cwd file
- **WHEN** `invowk config show` runs in a directory that contains `config.cue` and no other config source applies
- **THEN** the reported config source is the built-in defaults, not the cwd file

### Requirement: Config source precedence

invowk SHALL select exactly one config source, using this precedence: `--ivk-config` first, then `INVOWK_CONFIG`, then the user config file, then built-in defaults. A source that applies SHALL be used on its own, and lower-precedence sources SHALL NOT be evaluated at all. For example, when `--ivk-config` is given, `INVOWK_CONFIG` is neither read nor validated. invowk SHALL NOT merge values from lower-precedence sources into it. Every command that loads config SHALL apply the same precedence, including `invowk cmd`, completion, dynamic command registration, `validate`, `audit`, `agent`, `config show`, `config dump` and `config path`. `config set` and `config init` are the exception (see the requirement on user-file targets).

#### Scenario: flag beats environment variable
- **WHEN** `INVOWK_CONFIG` names a valid file A and `--ivk-config` names a valid file B
- **THEN** invowk loads B only

#### Scenario: flag masks an invalid environment value
- **WHEN** `INVOWK_CONFIG` is a relative path and `--ivk-config` names a valid file B
- **THEN** invowk loads B and reports no error about `INVOWK_CONFIG`

#### Scenario: environment variable beats user config
- **WHEN** a valid user config file exists, `--ivk-config` is not given, and `INVOWK_CONFIG` names a valid file A
- **THEN** invowk loads A only, and values from the user config file are not applied

#### Scenario: user config used when no explicit source
- **WHEN** `--ivk-config` is not given, `INVOWK_CONFIG` is unset or empty, and a valid user config file exists
- **THEN** invowk loads the user config file

#### Scenario: empty environment variable is unset
- **WHEN** `INVOWK_CONFIG` is set to the empty string
- **THEN** invowk behaves exactly as if `INVOWK_CONFIG` were unset

#### Scenario: config show and config path report the effective source
- **WHEN** `INVOWK_CONFIG` names a valid file A and `invowk config show` or `invowk config path` runs
- **THEN** the output names A as the effective config file and identifies `INVOWK_CONFIG` as the source

#### Scenario: config path reports without loading
- **WHEN** `INVOWK_CONFIG` names an absolute path to a file that does not exist and `invowk config path` runs
- **THEN** the command succeeds and prints that path as the effective config file, marked as coming from `INVOWK_CONFIG` and as missing

#### Scenario: validate honours the flag
- **WHEN** `invowk --ivk-config B validate` runs with no path argument
- **THEN** workspace validation uses the config from B, including its `includes`

### Requirement: INVOWK_CONFIG must be an absolute path

A non-empty `INVOWK_CONFIG` value SHALL be an absolute path for the host OS. A relative value SHALL be rejected with an error that names the variable and asks for an absolute path. invowk SHALL NOT resolve it against the current directory. `--ivk-config` MAY stay relative, because the user typed it on that command line.

#### Scenario: relative INVOWK_CONFIG rejected
- **WHEN** `INVOWK_CONFIG=config.cue` and a file `config.cue` exists in the current directory
- **THEN** the invocation fails with a non-zero exit before any command runs, and the error names `INVOWK_CONFIG` and says it must be absolute

#### Scenario: relative flag still accepted
- **WHEN** `--ivk-config custom/config.cue` names an existing valid file relative to the current directory
- **THEN** invowk loads that file

### Requirement: Explicit config sources fail closed

When the selected source is `--ivk-config` or `INVOWK_CONFIG`, and the named file is missing, unreadable, not valid CUE, or fails schema or Go validation, invowk SHALL stop the invocation with a non-zero exit before discovering or running any user command. The error SHALL name the source (flag or environment variable) and the path. invowk SHALL NOT fall back to the user config file or to defaults in that case.

#### Scenario: missing file named by the flag
- **WHEN** `invowk cmd hello --ivk-config nonexistent.cue` runs
- **THEN** the exit code is non-zero, stderr names `nonexistent.cue` and `--ivk-config`, and the `hello` script does not run

#### Scenario: missing file named by the environment variable
- **WHEN** `INVOWK_CONFIG` names an absolute path that does not exist and `invowk cmd hello` runs
- **THEN** the exit code is non-zero, stderr names the path and `INVOWK_CONFIG`, and the `hello` script does not run

#### Scenario: invalid explicit config
- **WHEN** `INVOWK_CONFIG` names a file that contains invalid CUE or an unknown field
- **THEN** the exit code is non-zero, the existing actionable config-load error is shown, and no command runs

#### Scenario: invalid user config keeps the current behaviour
- **WHEN** no explicit source is given and the user config file contains invalid CUE
- **THEN** invowk reports a config-load diagnostic and continues with built-in defaults, as it does today

### Requirement: config set and config init write the user config file

`invowk config set` SHALL read its starting values from the user config file (or defaults when that file does not exist), not from `--ivk-config` or `INVOWK_CONFIG`. It SHALL write the result back to the user config file. `invowk config init` SHALL create the user config file. Neither command SHALL copy the contents of an explicitly selected file into the user config file.

#### Scenario: config set ignores INVOWK_CONFIG as its base
- **WHEN** `INVOWK_CONFIG` names a file A that sets `container_engine: "docker"`, no user config file exists, and `invowk config set default_runtime virtual-sh` runs
- **THEN** the user config file is created with `default_runtime: "virtual-sh"` and the default `container_engine`, and A is unchanged
