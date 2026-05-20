## MODIFIED Requirements

### Requirement: Dry-run shows interpreter provenance
Invowk dry-run output SHALL disclose the source of the effective interpreter decision for selected implementation scripts and SHALL identify virtual runtime normalization.

#### Scenario: Dry-run shows explicit interpreter
- **WHEN** `--ivk-dry-run` is used for a selected implementation with concrete `script.interpreter`
- **THEN** the dry-run output SHALL identify the interpreter as explicit

#### Scenario: Dry-run shows shebang interpreter
- **WHEN** `--ivk-dry-run` is used for a selected implementation whose omitted or `"auto"` interpreter resolves to a shebang
- **THEN** the dry-run output SHALL identify the interpreter as shebang-detected

#### Scenario: Dry-run shows default shell behavior
- **WHEN** `--ivk-dry-run` is used for a selected implementation with no explicit interpreter and no shebang
- **THEN** the dry-run output SHALL identify the interpreter decision as default shell behavior for the selected runtime

#### Scenario: Dry-run shows virtual-sh normalization
- **WHEN** `--ivk-dry-run` is used for a `virtual-sh` implementation with shell-compatible interpreter intent
- **THEN** the dry-run output SHALL identify that the implementation will run through the embedded mvdan/sh virtual shell

#### Scenario: Dry-run shows virtual-lua normalization
- **WHEN** `--ivk-dry-run` is used for a `virtual-lua` implementation with Lua interpreter intent
- **THEN** the dry-run output SHALL identify that the implementation will run through the embedded Lua VM

#### Scenario: Dry-run shows override warning
- **WHEN** `--ivk-dry-run` is used for a selected implementation whose explicit interpreter overrides a different shebang
- **THEN** the dry-run output SHALL include the override diagnostic with the script source, explicit interpreter, and shebang interpreter

## ADDED Requirements

### Requirement: Dry-run shows virtual safety settings
Invowk dry-run output SHALL summarize virtual runtime safety settings that affect execution.

#### Scenario: Dry-run shows host binary denial
- **WHEN** `--ivk-dry-run` is used for a virtual runtime with omitted or empty `allowed_binaries`
- **THEN** the dry-run output SHALL identify that host binaries are denied by default

#### Scenario: Dry-run shows wildcard host binary opt-out
- **WHEN** `--ivk-dry-run` is used for a virtual runtime with `allowed_binaries: ["*"]`
- **THEN** the dry-run output SHALL identify that all host binaries are allowed and that launched host binaries are outside the Go-level path sandbox

#### Scenario: Dry-run shows binary lookup mode
- **WHEN** `--ivk-dry-run` is used for a virtual runtime
- **THEN** the dry-run output SHALL identify the effective `binary_lookup_mode`

#### Scenario: Dry-run shows allowed path names
- **WHEN** `--ivk-dry-run` is used for a virtual runtime with implementation-scoped `allowed_paths`
- **THEN** the dry-run output SHALL list logical path names without leaking unrelated host filesystem details

### Requirement: Custom checks surface interpreter override diagnostics
Invowk SHALL apply the same interpreter override diagnostics to custom-check scripts after custom-check script source resolution.

#### Scenario: Host custom check warns for explicit override
- **WHEN** a host custom check declares `script: {content: "#!/bin/sh\nprint('ok')", interpreter: "python3"}`
- **THEN** Invowk SHALL emit an advisory diagnostic before running the check with the host `python3` interpreter

#### Scenario: Container custom check warns for explicit override
- **WHEN** a selected container runtime declares a custom check with `script: {content: "#!/bin/sh\nprint('ok')", interpreter: "python3"}`
- **THEN** Invowk SHALL emit an advisory diagnostic before running the check with `python3` inside the container

#### Scenario: Virtual-sh compatible host custom check does not require host shell
- **WHEN** a host custom check declares a shell-compatible explicit interpreter or shell shebang
- **THEN** Invowk SHALL continue to run the check through the embedded mvdan/sh shell and SHALL NOT require a host shell path for the diagnostic
