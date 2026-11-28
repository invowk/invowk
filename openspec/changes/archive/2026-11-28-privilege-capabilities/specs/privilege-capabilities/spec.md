# Spec Delta

## Purpose

Defines declarative `"elevated"` and `"unprivileged"` capability checks in `depends_on.capabilities`, consistent capability visibility in `--help` and `--ivk-dry-run`, and `invowk audit` file-permission checks for elevated commands.

## ADDED Requirements

### Requirement: #CapabilityName supports elevated and unprivileged checks
`#CapabilityName` SHALL admit `"elevated"` and `"unprivileged"` alongside existing capabilities. On Unix, `"elevated"` SHALL succeed iff effective UID is `0`; on Windows, iff the current process token is elevated. `"unprivileged"` SHALL succeed iff `"elevated"` fails. Inside container `runtime.depends_on`, `"elevated"` and `"unprivileged"` SHALL probe `id -u` inside the container. Invowk SHALL NOT synthesize an `INVOWK_INVOKING_USER` environment variable.

#### Scenario: Unprivileged user fails elevated capability check
- **WHEN** a command declares `capabilities: [{alternatives: ["elevated"]}]` and runs as a non-root / non-elevated user
- **THEN** dependency validation SHALL fail with exit status `1` before running the command script

#### Scenario: Non-root user satisfies unprivileged capability check
- **WHEN** a command declares `capabilities: [{alternatives: ["unprivileged"]}]` and runs as a non-root user
- **THEN** dependency validation SHALL pass and execute the command

### Requirement: Declared capabilities appear in command help and dry-run output
When a command (or its selected implementation) declares `depends_on.capabilities`, `invowk cmd <name> --help` and `invowk cmd --ivk-dry-run <name>` SHALL display the required capabilities uniformly.

#### Scenario: Dry-run displays elevated capability requirement
- **WHEN** a command declares `capabilities: [{alternatives: ["elevated"]}]` and the user runs `invowk cmd --ivk-dry-run <name>`
- **THEN** dry-run output SHALL list `elevated` under the command's required capabilities

### Requirement: invowk audit flags writable files referenced by elevated commands
When `invowk audit` scans a command declaring `"elevated"`, it SHALL emit a finding if `invowkfile.cue`, `invowkmod.cue`, or any referenced `script.file` or `env.files` target is writable by non-owners (accounting safely for user-private-group semantics vs world/shared-group write bits).

#### Scenario: World-writable script referenced by elevated command triggers audit finding
- **WHEN** a module command declares `"elevated"` and references a world-writable (`0o777`) `script.file`
- **THEN** `invowk audit` SHALL report a finding highlighting privilege tampering risk and exit `1`
