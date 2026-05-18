# cue-schema-coherence Specification

## Purpose
TBD - created by archiving change harden-invowkmod-identity-and-schema-coherence. Update Purpose after archive.
## Requirements
### Requirement: Module requirement version constraints are schema and Go coherent
The `invowkmod` CUE schema SHALL accept the same declared version-constraint syntax that Go validation accepts and SHALL reject malformed strings without relying on prefix-only matching.

#### Scenario: Two-character comparison operators are accepted
- **WHEN** an `invowkmod.cue` requirement declares `version: ">=1.0.0"` or `version: "<=2.0.0"`
- **THEN** CUE parsing and Go validation SHALL accept the version constraint

#### Scenario: Supported single-character operators are accepted
- **WHEN** an `invowkmod.cue` requirement declares a version using `^`, `~`, `>`, `<`, or `=` followed by a supported semantic version
- **THEN** CUE parsing and Go validation SHALL accept the version constraint

#### Scenario: Bare semantic versions are accepted
- **WHEN** an `invowkmod.cue` requirement declares `version: "1.2.3"`
- **THEN** CUE parsing and Go validation SHALL accept the version constraint

#### Scenario: Trailing junk is rejected
- **WHEN** an `invowkmod.cue` requirement declares `version: "1.2.3junk"`
- **THEN** CUE parsing SHALL reject the value

#### Scenario: Unsupported operator is rejected
- **WHEN** an `invowkmod.cue` requirement declares `version: ">>1.0.0"`
- **THEN** CUE parsing or Go validation SHALL reject the value

#### Scenario: V-prefixed version is rejected
- **WHEN** an `invowkmod.cue` requirement declares `version: "^v1.2.3"`
- **THEN** CUE parsing or Go validation SHALL reject the value

### Requirement: Invowkmod parsing runs full metadata validation
Invowk SHALL run full Go metadata validation after decoding `invowkmod.cue` through CUE.

#### Scenario: Invalid Git URL is rejected after decode
- **WHEN** an `invowkmod.cue` requirement has an unsupported or malformed `git_url`
- **THEN** `ParseInvowkmodBytes` SHALL return a validation error

#### Scenario: Invalid alias is rejected after decode
- **WHEN** an `invowkmod.cue` requirement has an alias that fails the Go `ModuleAlias` validator
- **THEN** `ParseInvowkmodBytes` SHALL return a validation error

#### Scenario: Invalid path is rejected after decode
- **WHEN** an `invowkmod.cue` requirement has a path that fails cross-platform path validation
- **THEN** `ParseInvowkmodBytes` SHALL return a validation error that identifies the offending requirement path

#### Scenario: Invalid module metadata is rejected after decode
- **WHEN** top-level module metadata passes CUE syntax but fails a Go value-type validator
- **THEN** `ParseInvowkmodBytes` SHALL return a validation error before the metadata is used by module sync or discovery

### Requirement: Git URL validation does not encode module directory naming
The `invowkmod` CUE schema and Go validators SHALL validate Git URL scheme and length without requiring the repository basename to end in `.invowkmod`.

#### Scenario: Ordinary Git URL is accepted
- **WHEN** a requirement declares `git_url: "https://example.com/acme/tools.git"`
- **THEN** invowkmod parsing SHALL accept the URL when all other fields are valid

#### Scenario: Unsupported URL scheme is rejected
- **WHEN** a requirement declares a Git URL with an unsupported scheme such as `file://` or `ftp://`
- **THEN** invowkmod parsing SHALL reject the URL

#### Scenario: Schema comment matches validation
- **WHEN** developers read `pkg/invowkmod/invowkmod_schema.cue`
- **THEN** the `git_url` comment SHALL describe supported schemes and SHALL NOT state that the repository name must end in `.invowkmod`

### Requirement: Root invowkfile workdir rejects whitespace-only strings
The root-level `workdir` field in `invowkfile.cue` SHALL require at least one non-whitespace rune when present.

#### Scenario: Whitespace-only root workdir is rejected
- **WHEN** an `invowkfile.cue` declares `workdir: "   "`
- **THEN** invowkfile parsing SHALL fail during schema or parse-level validation

#### Scenario: Non-empty root workdir is accepted
- **WHEN** an `invowkfile.cue` declares a relative or absolute root `workdir` containing at least one non-whitespace rune
- **THEN** invowkfile parsing SHALL accept the field subject to existing path handling

#### Scenario: Omitted root workdir remains valid
- **WHEN** an `invowkfile.cue` omits root `workdir`
- **THEN** invowkfile parsing SHALL continue to accept the file

### Requirement: Go-only validation boundaries are documented
CUE schema comments and Go validators SHALL clearly identify validations that intentionally remain in Go.

#### Scenario: Runtime image and containerfile exclusivity is documented
- **WHEN** developers read the container runtime schema or validator
- **THEN** the mutual exclusivity between `image` and `containerfile` SHALL be documented as Go-only validation when it is not enforced by CUE

#### Scenario: LLM provider and API exclusivity is documented
- **WHEN** developers read config schema or validators for LLM backend selection
- **THEN** any mutual exclusivity between provider and API settings SHALL be documented as Go-only validation when it is not enforced by CUE

#### Scenario: Path traversal stays Go-only
- **WHEN** developers read module requirement path validation
- **THEN** comments SHALL explain that cross-platform traversal and normalization checks are enforced in Go

### Requirement: Duration schema helper names reflect their limits
CUE duration helper definitions with the same name SHALL have the same maximum length; duration helpers with different maximum lengths SHALL use distinct names.

#### Scenario: Config LLM timeout helper is renamed
- **WHEN** `internal/config/config_schema.cue` keeps the existing 64-rune LLM timeout limit
- **THEN** the helper SHALL use a specific name such as `#LLMTimeoutDurationString` instead of `#DurationString`

#### Scenario: Invowkfile duration helper keeps its limit
- **WHEN** `pkg/invowkfile/invowkfile_schema.cue` defines duration strings for implementation timeout and watch debounce
- **THEN** `#DurationString` SHALL continue to match the Go `invowkfile.DurationString` 32-rune limit

#### Scenario: Same helper name cannot hide different limits
- **WHEN** two CUE schema helpers use the same duration helper name
- **THEN** their maximum rune limits SHALL be identical

#### Scenario: Behavioral sync tests use the specific helper
- **WHEN** config schema sync tests validate `llm.timeout`
- **THEN** tests SHALL exercise the renamed config helper or the field using the 64-rune `LLMTimeout` Go validator

### Requirement: Schema coherence changes are covered by tests and docs
Invowk SHALL update tests and documentation for every schema or validation contract changed by this capability.

#### Scenario: Schema sync tests cover changed fields
- **WHEN** CUE schemas or Go JSON-tagged structs are changed
- **THEN** schema sync tests SHALL verify that CUE fields and Go fields remain aligned

#### Scenario: Behavioral tests cover parser parity
- **WHEN** CUE regexes or Go validators are tightened
- **THEN** behavioral tests SHALL cover accepted examples and rejected counterexamples for the affected fields

#### Scenario: Docs describe current validation behavior
- **WHEN** README, website docs, snippets, samples, or generated references mention changed fields
- **THEN** they SHALL describe the current validation behavior and no longer repeat stale schema comments

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

