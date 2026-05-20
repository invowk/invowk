## ADDED Requirements

### Requirement: Explicit interpreter override diagnostics
Invowk SHALL emit advisory diagnostics when a concrete explicit `script.interpreter` overrides a different shebang parsed from the resolved script bytes. These diagnostics SHALL NOT make an otherwise valid script invalid, and execution SHALL continue to use the explicit interpreter.

#### Scenario: File script explicit interpreter overrides shebang
- **WHEN** an invowkmod implementation declares `script: {file: "scripts/build", interpreter: "python3"}` and the resolved `scripts/build` bytes start with `#!/bin/sh`
- **THEN** Invowk SHALL emit an advisory diagnostic identifying `scripts/build`, `python3`, `/bin/sh`, and that `script.interpreter` takes precedence

#### Scenario: Inline script explicit interpreter overrides shebang
- **WHEN** an inline implementation script declares `script: {content: "#!/bin/sh\nprint('ok')", interpreter: "python3"}`
- **THEN** Invowk SHALL emit an advisory diagnostic identifying the inline script, `python3`, `/bin/sh`, and that `script.interpreter` takes precedence

#### Scenario: Override diagnostic is not a validation failure
- **WHEN** a script has a valid explicit interpreter and a different valid shebang
- **THEN** Invowk SHALL keep the script valid and SHALL use the explicit interpreter for native or container execution

#### Scenario: Auto interpreter does not warn
- **WHEN** a script declares `script.interpreter: "auto"` and the resolved script bytes start with a valid shebang
- **THEN** Invowk SHALL use the shebang for interpreter selection and SHALL NOT emit an override diagnostic

#### Scenario: Equivalent interpreter selections do not warn
- **WHEN** a script declares an explicit interpreter whose parsed interpreter and arguments match the parsed shebang selection
- **THEN** Invowk SHALL NOT emit an override diagnostic

#### Scenario: Interpreter argument differences warn
- **WHEN** a script declares `script.interpreter: "python3"` and the resolved script bytes start with `#!/usr/bin/env -S python3 -u`
- **THEN** Invowk SHALL emit an advisory diagnostic because the explicit interpreter overrides the shebang arguments

### Requirement: Diagnostics are based on resolved script bytes
Invowk SHALL analyze shebang overrides only after the script source has been resolved and validated according to the existing `script.content` or module-contained `script.file` rules.

#### Scenario: File script is contained before diagnostics
- **WHEN** an invowkmod implementation declares `script.file` and an explicit `script.interpreter`
- **THEN** Invowk SHALL validate module containment and read the resolved file before parsing the shebang for override diagnostics

#### Scenario: Invalid file script does not emit override diagnostic
- **WHEN** a `script.file` reference is rejected because it is outside the source module or cannot be read
- **THEN** Invowk SHALL report the file-resolution error and SHALL NOT emit a shebang override diagnostic for unread bytes

#### Scenario: Authored file path is used in diagnostics
- **WHEN** a module-contained `script.file` uses an authored relative path that resolves to a host-specific absolute path
- **THEN** Invowk SHALL identify the authored `script.file` path in the advisory diagnostic

### Requirement: Dry-run shows interpreter provenance
Invowk dry-run output SHALL disclose the source of the effective interpreter decision for selected implementation scripts.

#### Scenario: Dry-run shows explicit interpreter
- **WHEN** `--ivk-dry-run` is used for a selected implementation with concrete `script.interpreter`
- **THEN** the dry-run output SHALL identify the interpreter as explicit

#### Scenario: Dry-run shows shebang interpreter
- **WHEN** `--ivk-dry-run` is used for a selected implementation whose omitted or `"auto"` interpreter resolves to a shebang
- **THEN** the dry-run output SHALL identify the interpreter as shebang-detected

#### Scenario: Dry-run shows default shell behavior
- **WHEN** `--ivk-dry-run` is used for a selected implementation with no explicit interpreter and no shebang
- **THEN** the dry-run output SHALL identify the interpreter decision as default shell behavior for the selected runtime

#### Scenario: Dry-run shows virtual shell normalization
- **WHEN** `--ivk-dry-run` is used for a virtual implementation with shell-compatible interpreter intent
- **THEN** the dry-run output SHALL identify that the implementation will run through the embedded mvdan/sh virtual shell

#### Scenario: Dry-run shows override warning
- **WHEN** `--ivk-dry-run` is used for a selected implementation whose explicit interpreter overrides a different shebang
- **THEN** the dry-run output SHALL include the override diagnostic with the script source, explicit interpreter, and shebang interpreter

### Requirement: Custom checks surface interpreter override diagnostics
Invowk SHALL apply the same interpreter override diagnostics to custom-check scripts after custom-check script source resolution.

#### Scenario: Host custom check warns for explicit override
- **WHEN** a host custom check declares `script: {content: "#!/bin/sh\nprint('ok')", interpreter: "python3"}`
- **THEN** Invowk SHALL emit an advisory diagnostic before running the check with the host `python3` interpreter

#### Scenario: Container custom check warns for explicit override
- **WHEN** a selected container runtime declares a custom check with `script: {content: "#!/bin/sh\nprint('ok')", interpreter: "python3"}`
- **THEN** Invowk SHALL emit an advisory diagnostic before running the check with `python3` inside the container

#### Scenario: Virtual-compatible host custom check does not require host shell
- **WHEN** a host custom check declares a shell-compatible explicit interpreter or shell shebang
- **THEN** Invowk SHALL continue to run the check through the embedded mvdan/sh shell and SHALL NOT require a host shell path for the diagnostic

### Requirement: Interpreter diagnostic behavior is test covered
Invowk SHALL include automated tests covering interpreter override diagnostics, interpreter provenance output, and no-warning cases.

#### Scenario: Runtime tests cover explicit-over-shebang precedence
- **WHEN** runtime tests exercise inline and file-backed scripts with both explicit interpreters and shebangs
- **THEN** they SHALL verify the explicit interpreter remains authoritative and the override diagnostic is produced only for non-equivalent selections

#### Scenario: Dry-run tests cover provenance
- **WHEN** CLI tests exercise dry-run for explicit, shebang-detected, default-shell, and virtual-shell interpreter decisions
- **THEN** they SHALL verify the output identifies the interpreter provenance

#### Scenario: Dependency tests cover custom checks
- **WHEN** dependency tests exercise host and container custom checks with explicit interpreter and shebang combinations
- **THEN** they SHALL verify override diagnostics and preserve existing custom-check execution semantics
