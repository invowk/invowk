## ADDED Requirements

### Requirement: Container runtime persistent configuration
The invowkfile schema SHALL allow persistent targeting only inside `runtime` entries whose `name` is `container`.

#### Scenario: Persistent block is accepted for container runtime
- **WHEN** an implementation declares a container runtime with `persistent.create_if_missing` and optional `persistent.name`
- **THEN** invowkfile parsing SHALL accept the runtime configuration when all persistent fields are valid

#### Scenario: Persistent block is rejected outside container runtime
- **WHEN** an implementation declares `persistent` on a native or virtual runtime
- **THEN** invowkfile parsing SHALL fail with an actionable validation error

#### Scenario: Disposable execution remains default
- **WHEN** a container runtime omits `persistent` and the user does not pass a persistent container override flag
- **THEN** Invowk SHALL use the existing disposable container execution behavior

#### Scenario: Create-if-missing defaults to false
- **WHEN** a container runtime declares `persistent` without `create_if_missing`
- **THEN** Invowk SHALL NOT create a missing persistent container automatically

### Requirement: Persistent container target name resolution
Invowk SHALL resolve a single effective persistent container name from CLI override, runtime configuration, or deterministic derivation.

#### Scenario: CLI override wins
- **WHEN** the user passes `--ivk-container-name <name>`
- **THEN** Invowk SHALL use `<name>` as the effective persistent container target name for that invocation

#### Scenario: Runtime persistent name is used
- **WHEN** no CLI container-name override is present and `runtime.persistent.name` is set
- **THEN** Invowk SHALL use `runtime.persistent.name` as the effective persistent container target name

#### Scenario: Derived name is used
- **WHEN** persistent targeting is enabled and no CLI or runtime name is provided
- **THEN** Invowk SHALL derive a deterministic name from `invowk`, the command's full namespace, and a collision-resistant source identity suffix

#### Scenario: Derived name is portable
- **WHEN** Invowk derives a persistent container name
- **THEN** the name SHALL start with `invowk-`, be compatible with supported Docker and Podman container-name flags, and contain only lowercase ASCII letters, digits, `.`, `_`, and `-`

#### Scenario: Explicit invalid name is rejected
- **WHEN** `runtime.persistent.name` or `--ivk-container-name` contains whitespace, slashes, colons, uppercase-only transformations that would require rewriting, or any other unsupported character
- **THEN** Invowk SHALL reject the explicit name instead of rewriting it silently

#### Scenario: Explicit name can target user namespace
- **WHEN** `runtime.persistent.name` or `--ivk-container-name` is a valid portable container name that does not start with `invowk-`
- **THEN** Invowk SHALL accept the explicit name

### Requirement: Managed persistent container lifecycle
Invowk SHALL create, reuse, start, and validate Invowk-managed persistent containers idempotently.

#### Scenario: Missing managed container is created
- **WHEN** persistent targeting is enabled, the effective target does not exist, and `persistent.create_if_missing` is true
- **THEN** Invowk SHALL create a named managed persistent container with Invowk ownership labels and then execute the command in that container

#### Scenario: Missing managed container is not created without opt-in
- **WHEN** persistent targeting is enabled, the effective target does not exist, and `persistent.create_if_missing` is false
- **THEN** Invowk SHALL fail with an actionable error and SHALL NOT create a container

#### Scenario: Existing matching managed container is reused
- **WHEN** the effective target exists with Invowk managed labels and a matching creation-time fingerprint
- **THEN** Invowk SHALL execute the command in the existing container without recreating it

#### Scenario: Stopped managed container is started
- **WHEN** the effective target exists, is Invowk-managed, has a matching creation-time fingerprint, and is stopped
- **THEN** Invowk SHALL start the container and then execute the command in it

#### Scenario: Managed container state persists across runs
- **WHEN** a command writes a file or cache inside a managed persistent container
- **THEN** a later invocation targeting the same managed container SHALL be able to observe that persisted state

#### Scenario: Managed container drift fails
- **WHEN** the effective target exists with Invowk managed labels but its creation-time fingerprint differs from the current runtime configuration
- **THEN** Invowk SHALL fail with drift guidance and SHALL NOT mutate, recreate, or remove the container

#### Scenario: Unmanaged config target is protected
- **WHEN** the effective target came from runtime configuration or derivation and a container with that name exists without Invowk managed labels
- **THEN** Invowk SHALL fail rather than executing in or taking ownership of the unmanaged container

### Requirement: CLI override can target pre-existing containers
The `--ivk-container-name` flag SHALL allow users to explicitly target a pre-existing container for a single invocation.

#### Scenario: Existing external running container is targeted
- **WHEN** the user passes `--ivk-container-name <name>` and `<name>` identifies an existing running container without Invowk managed labels
- **THEN** Invowk SHALL execute the command in that external container without adding Invowk ownership labels

#### Scenario: Missing CLI-only external target fails
- **WHEN** the user passes `--ivk-container-name <name>`, the runtime has no `persistent.create_if_missing` opt-in, and no container named `<name>` exists
- **THEN** Invowk SHALL fail with an actionable missing-target error

#### Scenario: Stopped external container fails
- **WHEN** the user passes `--ivk-container-name <name>` and `<name>` identifies an external container that is stopped
- **THEN** Invowk SHALL fail without starting the external container

#### Scenario: CLI override uses create-if-missing opt-in
- **WHEN** the user passes `--ivk-container-name <name>`, the runtime has `persistent.create_if_missing` set to true, and no container named `<name>` exists
- **THEN** Invowk SHALL create a managed persistent container using `<name>` as the container name

### Requirement: Persistent execution uses exec-time command state
Persistent container execution SHALL run command-specific state through container exec rather than container creation.

#### Scenario: Command runs through exec
- **WHEN** Invowk targets an existing persistent container
- **THEN** Invowk SHALL execute the command with the container engine's exec operation instead of starting a disposable `run --rm` container

#### Scenario: Dynamic environment is exec-only
- **WHEN** Invowk passes command flags, env-file values, env-var overrides, host SSH credentials, or TUI callback values to a persistent target
- **THEN** Invowk SHALL pass those values as exec-time environment and SHALL NOT store them in the persistent container's creation-time environment

#### Scenario: Workdir is exec-time
- **WHEN** a command uses the effective working directory or `--ivk-workdir`
- **THEN** Invowk SHALL apply that workdir to the exec invocation for persistent targets

#### Scenario: Host callback token is revoked after persistent execution
- **WHEN** a persistent container command uses host SSH access
- **THEN** Invowk SHALL revoke the per-execution host callback token after the command completes or preparation fails

#### Scenario: Capture execution uses the same target
- **WHEN** Invowk performs container-runtime dependency validation that captures output
- **THEN** the capture execution SHALL use the same persistent target selection rules as normal command execution

### Requirement: Persistent container creation is race-safe
Invowk SHALL make concurrent first use of the same managed persistent target idempotent.

#### Scenario: Concurrent create resolves to one target
- **WHEN** two Invowk processes concurrently target the same missing managed persistent container with `create_if_missing` enabled
- **THEN** at most one container SHALL be created and both processes SHALL execute only if the resulting managed container matches the expected fingerprint

#### Scenario: Name conflict is re-inspected
- **WHEN** container creation returns a name-conflict error after a prior missing-target inspect
- **THEN** Invowk SHALL re-inspect the target and proceed only if it is a matching Invowk-managed container

### Requirement: Persistent targeting is visible in dry-run output
Invowk dry-run output SHALL disclose persistent container targeting decisions without performing lifecycle operations.

#### Scenario: Dry-run reports disposable container
- **WHEN** `--ivk-dry-run` is used for a container command without persistent targeting
- **THEN** the dry-run plan SHALL identify the execution as disposable container execution

#### Scenario: Dry-run reports persistent target
- **WHEN** `--ivk-dry-run` is used for a command with persistent targeting
- **THEN** the dry-run plan SHALL include the effective target name, name source, and whether missing-target creation is allowed

#### Scenario: Dry-run does not create persistent container
- **WHEN** `--ivk-dry-run` is used and the effective persistent target does not exist
- **THEN** Invowk SHALL NOT create, start, or exec a container

### Requirement: Persistent container documentation
Invowk documentation SHALL describe persistent container targeting as opt-in stateful behavior.

#### Scenario: Invowkfile reference documents persistent fields
- **WHEN** users read the invowkfile runtime reference
- **THEN** it SHALL document `persistent.create_if_missing`, `persistent.name`, derived names, validation rules, and default disposable behavior

#### Scenario: CLI reference documents container-name override
- **WHEN** users read command-line reference documentation
- **THEN** it SHALL document `--ivk-container-name`, its precedence, and its pre-existing-container use case

#### Scenario: Runtime documentation explains external target expectations
- **WHEN** users read container runtime documentation
- **THEN** it SHALL explain that external targets must already be running and must provide the expected workspace mount or working directory for the command to succeed

#### Scenario: Runtime documentation explains statefulness
- **WHEN** users read persistent container documentation
- **THEN** it SHALL state that persistent containers retain state across invocations and do not provide a fresh execution environment
