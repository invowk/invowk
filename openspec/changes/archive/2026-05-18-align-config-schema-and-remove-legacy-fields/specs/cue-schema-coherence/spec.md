## ADDED Requirements

### Requirement: Config schema represents the full effective config shape
The `internal/config/config_schema.cue` schema SHALL describe the complete effective `Config` shape, including default values, enum constraints, nested structs, and collection constraints.

#### Scenario: Empty config evaluates to defaults
- **WHEN** an empty config document is validated and decoded through the config CUE schema
- **THEN** the resulting `Config` SHALL equal the default configuration used when no config file exists

#### Scenario: Top-level defaults are schema-visible
- **WHEN** developers read `internal/config/config_schema.cue`
- **THEN** defaults for `container_engine`, `includes`, `default_runtime`, `virtual_shell`, `ui`, `container`, and `llm` SHALL be represented in CUE rather than only in Go patch-merge code

#### Scenario: Nested defaults are schema-visible
- **WHEN** developers read nested config definitions such as `#VirtualShellConfig`, `#UIConfig`, and `#AutoProvisionConfig`
- **THEN** their effective default values SHALL be represented in CUE with constraints on allowed overrides

#### Scenario: User overrides remain accepted
- **WHEN** a config file overrides a valid subset of defaulted fields
- **THEN** CUE evaluation SHALL combine those overrides with schema defaults and decode a complete valid `Config`

#### Scenario: Invalid overrides are rejected
- **WHEN** a config file sets an invalid enum value, invalid duration, invalid include entry, or invalid typed field
- **THEN** CUE parsing or post-decode Go validation SHALL reject the config before it is used

### Requirement: DefaultConfig is derived from CUE defaults
Invowk SHALL derive `DefaultConfig()` from CUE schema evaluation so `internal/config/config_schema.cue` is the single source of truth for config default values.

#### Scenario: DefaultConfig uses CUE evaluation
- **WHEN** `DefaultConfig()` is called
- **THEN** it SHALL evaluate an empty config document against `#Config` and decode the resulting effective config

#### Scenario: No independent Go default copy exists
- **WHEN** developers inspect the default config implementation
- **THEN** default values SHALL NOT be maintained as a separate hand-written Go struct that duplicates CUE defaults

#### Scenario: Empty config and DefaultConfig share behavior
- **WHEN** tests decode an empty config file and call `DefaultConfig()`
- **THEN** both paths SHALL produce the same effective `Config`

### Requirement: Config variants are modeled as closed CUE disjunctions
Config sections whose valid shape depends on mutually exclusive alternatives SHALL use closed CUE disjunctions when the alternatives are statically expressible.

#### Scenario: LLM provider config is accepted
- **WHEN** config selects a supported LLM provider backend without configuring an API backend
- **THEN** CUE parsing SHALL accept the provider-backed config shape

#### Scenario: LLM API config is accepted
- **WHEN** config selects an OpenAI-compatible API backend without configuring a provider backend
- **THEN** CUE parsing SHALL accept the API-backed config shape

#### Scenario: Empty LLM config defaults to no backend
- **WHEN** config omits LLM backend settings
- **THEN** CUE evaluation SHALL produce the effective no-backend default without requiring Go patch semantics

#### Scenario: Mixed LLM backend shapes are rejected
- **WHEN** config sets both provider-backed and API-backed LLM fields in the same backend config
- **THEN** CUE parsing SHALL reject the config shape

### Requirement: Generated config output includes the full effective shape
Generated config output SHALL include every effective config field, including fields whose value equals the CUE default.

#### Scenario: Default generated config includes default-valued fields
- **WHEN** Invowk generates the default config file
- **THEN** the output SHALL include all top-level and nested effective config fields, including default-valued fields such as `container_engine`, `default_runtime`, `virtual_shell.enable_uroot_utils`, `ui.verbose`, and `container.auto_provision.inherit_includes`

#### Scenario: Generated config round-trips
- **WHEN** generated config output is parsed through the config CUE schema
- **THEN** it SHALL decode to the same effective `Config` that was used to generate it

#### Scenario: Generated config is not a minimal patch
- **WHEN** a config field has the same value as its CUE default
- **THEN** generated config output SHALL still include that field rather than relying on omission and schema defaulting

### Requirement: User-facing CUE structs are closed without legacy tombstones
Every user-facing CUE struct in `invowkfile`, `invowkmod`, and config schemas SHALL be closed, and schemas SHALL NOT list removed or legacy user fields solely to reject them.

#### Scenario: Unknown config field is rejected by closure
- **WHEN** a config file declares a field that is not part of the current config schema
- **THEN** CUE parsing SHALL reject it because the containing struct is closed

#### Scenario: Unknown invowkfile field is rejected by closure
- **WHEN** an `invowkfile.cue` declares a field that is not part of the current invowkfile schema
- **THEN** CUE parsing SHALL reject it because `#Invowkfile` or the containing struct is closed

#### Scenario: Schema contains only current fields
- **WHEN** developers inspect user-facing schema definitions
- **THEN** the definitions SHALL contain current supported fields only, not explicit `_|_` tombstones for removed names

### Requirement: Legacy invowkfile field declarations are removed
The `pkg/invowkfile/invowkfile_schema.cue` schema SHALL remove explicit declarations for legacy fields that are no longer supported.

#### Scenario: Legacy commands field is not modeled
- **WHEN** developers inspect `#Invowkfile`
- **THEN** it SHALL NOT declare `commands` as a special unsupported field

#### Scenario: Legacy module metadata fields are not modeled
- **WHEN** developers inspect `#Invowkfile`
- **THEN** it SHALL NOT declare old module metadata fields such as `module`, `version`, `description`, or `requires`

#### Scenario: Legacy fields still fail validation
- **WHEN** an `invowkfile.cue` uses old fields such as `commands`, `module`, `version`, `description`, or `requires`
- **THEN** CUE parsing SHALL reject them as unknown fields in a closed struct

### Requirement: Schema sync covers full config defaults and closures
Schema sync and behavioral tests SHALL cover full-shape config defaults, closed structs, and the absence of legacy invowkfile tombstones.

#### Scenario: Config full-shape sync is tested
- **WHEN** config schema or Go config structs change
- **THEN** tests SHALL verify CUE fields, Go JSON tags, default values, and decoded effective config remain aligned

#### Scenario: Closure behavior is tested
- **WHEN** unsupported fields are supplied in config or invowkfile test fixtures
- **THEN** tests SHALL verify they are rejected by closed schema definitions

#### Scenario: Legacy tombstone removal is tested
- **WHEN** invowkfile schema tests inspect `#Invowkfile`
- **THEN** tests SHALL verify removed legacy field names are not declared as schema fields

#### Scenario: Documentation reflects the new schema model
- **WHEN** docs, samples, or generated config references describe config or invowkfile schema behavior
- **THEN** they SHALL describe config as a default-bearing full schema and SHALL NOT document old invowkfile field tombstones as supported migration behavior
