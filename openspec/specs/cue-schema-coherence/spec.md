# cue-schema-coherence Specification

## Purpose
Define coherence requirements for CUE schemas, Go value validation, generated configuration, documentation, and tests across Invowk schema and validation contracts.
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
Invowk SHALL update tests and documentation for every schema or validation contract changed by this capability, and SHALL remove compatibility-only schema/model/docs leftovers when a clean-break field move is specified.

#### Scenario: Schema sync tests cover changed fields
- **WHEN** CUE schemas or Go JSON-tagged structs are changed
- **THEN** schema sync tests SHALL verify that CUE fields and Go fields remain aligned

#### Scenario: Behavioral tests cover parser parity
- **WHEN** CUE regexes or Go validators are tightened
- **THEN** behavioral tests SHALL cover accepted examples and rejected counterexamples for the affected fields

#### Scenario: Docs describe current validation behavior
- **WHEN** README, website docs, snippets, samples, or generated references mention changed fields
- **THEN** they SHALL describe the current validation behavior and no longer repeat stale schema comments

#### Scenario: Script interpreter field is schema and Go coherent
- **WHEN** `script.interpreter` is added to implementation scripts and custom-check scripts
- **THEN** CUE schema sync and Go behavioral tests SHALL verify the field exists on both script structs with the same JSON spelling, validation length, and interpreter safety semantics

#### Scenario: Virtual filesystem config is schema and Go coherent
- **WHEN** virtual filesystem config moves to `platforms[].virtual.filesystem`
- **THEN** CUE schema sync and Go behavioral tests SHALL verify platform config structs, JSON spelling, default access mode, access enum values, logical path name validation, path value validation, and generated CUE remain aligned

#### Scenario: Clean-break changes do not leave tombstones
- **WHEN** a field is removed as part of a clean-break schema change
- **THEN** user-facing CUE structs SHALL rely on closed-schema unknown-field rejection and SHALL NOT retain explicit legacy tombstones, aliases, ignored fields, or compatibility-only decode paths

### Requirement: Config schema represents the full effective config shape
The `internal/config/config_schema.cue` schema SHALL describe the complete effective `Config` shape, including default values, enum constraints, nested structs, and collection constraints.

#### Scenario: Empty config evaluates to defaults
- **WHEN** an empty config document is validated and decoded through the config CUE schema
- **THEN** the resulting `Config` SHALL equal the default configuration used when no config file exists

#### Scenario: Top-level defaults are schema-visible
- **WHEN** developers read `internal/config/config_schema.cue`
- **THEN** defaults for `container_engine`, `includes`, `default_runtime`, `virtual`, `ui`, `container`, and `llm` SHALL be represented in CUE rather than only in Go patch-merge code

#### Scenario: Nested defaults are schema-visible
- **WHEN** developers read nested config definitions such as `#VirtualConfig`, `#UIConfig`, and `#AutoProvisionConfig`
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
- **THEN** the output SHALL include all top-level and nested effective config fields, including default-valued fields such as `container_engine`, `default_runtime`, `virtual.utilities.enabled`, `ui.verbose`, and `container.auto_provision.inherit_includes`

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

### Requirement: Schema comments distinguish CUE and Invowk validation ownership
CUE schema comments SHALL clearly distinguish rules enforced by CUE from rules enforced by Invowk after decode.

#### Scenario: Container source invariants identify shared ownership
- **WHEN** developers read the container runtime schema comments for `containerfile` and `image`
- **THEN** the comments SHALL state that CUE models the exactly-one source shape when statically expressible and Invowk Go validation preserves the same invariant after decode for direct Go values, diagnostics, and defense-in-depth

#### Scenario: Argument and flag semantic rules identify validation ownership
- **WHEN** developers read argument or flag schema comments for descriptions, required/default conflicts, ordering, variadic position, default type compatibility, regex safety, or reserved system flag names
- **THEN** the comments SHALL identify descriptions as required by both CUE and Go value validation, and SHALL identify semantic rules outside CUE as Invowk post-decode validation rather than implying CUE alone enforces them

#### Scenario: Command hierarchy rules identify Go ownership
- **WHEN** developers read command schema comments for duplicate platform/runtime combinations or subcommands with positional args
- **THEN** the comments SHALL identify those rules as Invowk validation that requires command tree or implementation analysis

#### Scenario: Documentation warns that CUE validation is not full Invowk validation
- **WHEN** users read schema reference or command-authoring documentation
- **THEN** the docs SHALL explain that CUE validation checks shape and static constraints, while Invowk performs additional semantic validation after decode

### Requirement: Runtime variant mistakes have actionable diagnostics
Invowk SHALL report targeted validation diagnostics for the runtime-variant mistakes enumerated in this requirement instead of exposing only generic CUE disjunction errors.

#### Scenario: Container-only field on native runtime
- **WHEN** an invowkfile declares `persistent`, `image`, `containerfile`, `volumes`, `ports`, `enable_host_ssh`, or runtime-level `depends_on` on a runtime whose `name` is `native`
- **THEN** Invowk SHALL reject the file with an error identifying the offending field and stating that the field is only valid for `container` runtime

#### Scenario: Container-only field on virtual runtime
- **WHEN** an invowkfile declares `persistent`, `image`, `containerfile`, `volumes`, `ports`, `enable_host_ssh`, or runtime-level `depends_on` on a runtime whose `name` is `virtual`
- **THEN** Invowk SHALL reject the file with an error identifying the offending field and stating that the field is only valid for `container` runtime

#### Scenario: Virtual runtime interpreter is rejected clearly
- **WHEN** an invowkfile declares `interpreter` on a runtime whose `name` is `virtual`
- **THEN** Invowk SHALL reject the file with an error identifying `interpreter` and stating that the virtual runtime does not support interpreter overrides

#### Scenario: Container source is missing
- **WHEN** an invowkfile declares a runtime whose `name` is `container` without `image` or `containerfile`
- **THEN** Invowk SHALL reject the file with an error stating that container runtime requires either `image` or `containerfile`

#### Scenario: Container source is duplicated
- **WHEN** an invowkfile declares a runtime whose `name` is `container` with both `image` and `containerfile`
- **THEN** Invowk SHALL reject the file with an error stating that `image` and `containerfile` are mutually exclusive

### Requirement: Containerfile paths reject parent-directory segments only
The `containerfile` path policy SHALL reject parent-directory path segments while allowing ordinary names that merely contain consecutive dots.

#### Scenario: Simple relative containerfile path is accepted
- **WHEN** an invowkfile declares `containerfile: "Containerfile"` or `containerfile: "docker/Containerfile"`
- **THEN** CUE parsing and Invowk validation SHALL accept the path when the referenced file exists and all other runtime fields are valid

#### Scenario: Dot segments are accepted
- **WHEN** an invowkfile declares `containerfile: "./Containerfile"` or `containerfile: "docker/./Containerfile"`
- **THEN** Invowk SHALL accept the path subject to existing file existence and filename validation

#### Scenario: Consecutive dots inside a segment are accepted
- **WHEN** an invowkfile declares `containerfile: "Containerfile..backup"` or `containerfile: "docker/v1..2/Containerfile"`
- **THEN** CUE parsing and Invowk validation SHALL accept the value as a relative child path when all other path rules pass

#### Scenario: Parent segment at start is rejected
- **WHEN** an invowkfile declares `containerfile: "../Containerfile"`
- **THEN** Invowk SHALL reject the value because it contains a parent-directory segment

#### Scenario: Parent segment in middle is rejected
- **WHEN** an invowkfile declares `containerfile: "docker/../Containerfile"`
- **THEN** Invowk SHALL reject the value because it contains a parent-directory segment even though lexical cleaning would remain inside the invowkfile directory

#### Scenario: Backslash parent segment is rejected
- **WHEN** an invowkfile declares `containerfile: "docker\\..\\Containerfile"`
- **THEN** Invowk SHALL reject the value after normalizing backslashes to slash path separators

#### Scenario: Absolute path is rejected
- **WHEN** an invowkfile declares a Unix absolute path, Windows drive-qualified path, UNC path, or rooted Windows path as `containerfile`
- **THEN** Invowk SHALL reject the value before attempting to build the container image

#### Scenario: Path is resolved relative to invowkfile directory
- **WHEN** an invowkfile at `/repo/invowkfile.cue` declares `containerfile: "docker/Containerfile"`
- **THEN** Invowk SHALL look for `/repo/docker/Containerfile` and build with `/repo` as the build context

### Requirement: LLM defaults schema uses defaults-oriented naming
The config CUE schema SHALL model LLM common fields as backend defaults rather than "no backend" configuration.

#### Scenario: Defaults helper name replaces no-backend helper name
- **WHEN** developers inspect `internal/config/config_schema.cue`
- **THEN** the LLM config disjunction SHALL use `#LLMDefaultsConfig` and SHALL NOT define or reference `#LLMNoBackendConfig`

#### Scenario: Top-level model is documented as common default
- **WHEN** users read LLM config documentation or schema snippets
- **THEN** top-level `llm.model` SHALL be described as a common backend default rather than only a provider model override

#### Scenario: API model override precedence is documented
- **WHEN** users read LLM API config documentation
- **THEN** the docs SHALL state that `llm.api.model` overrides top-level `llm.model` for API-backed execution

#### Scenario: Provider and API backends remain exclusive
- **WHEN** config declares both `llm.provider` and a semantically configured `llm.api`
- **THEN** CUE parsing or Invowk post-decode validation SHALL reject the config with an error explaining that provider and API backends are mutually exclusive

### Requirement: Clean-break documentation removes stale schema guidance
Documentation SHALL describe only the new schema and validation contracts.

#### Scenario: Old command dependency syntax is absent
- **WHEN** README, website docs, snippet data, samples, or schema references describe command dependencies
- **THEN** they SHALL NOT show `tools lint`, `com.company.tools lint`, or any other source-prefix-with-space form as source-qualified dependency syntax

#### Scenario: New command dependency syntax is documented
- **WHEN** docs describe command dependencies across sources
- **THEN** they SHALL show explicit `@source command` examples such as `@tools lint` and `@com.company.tools lint`

#### Scenario: Containerfile path policy is documented
- **WHEN** docs describe `containerfile`
- **THEN** they SHALL state that the path is relative to the invowkfile directory, rejects parent-directory segments, and allows ordinary filenames containing consecutive dots

#### Scenario: LLM helper terminology is removed from docs
- **WHEN** docs or generated snippets include the config schema
- **THEN** they SHALL use the new LLM defaults helper name and SHALL NOT include `#LLMNoBackendConfig`

#### Scenario: Agent-facing guidance is current
- **WHEN** `.agents` rules, skills, commands, or AGENTS indexes mention affected schema behavior
- **THEN** they SHALL be updated or left untouched only if they do not contain stale behavior

### Requirement: Schema coherence changes are verified across code and docs
Invowk SHALL cover schema contract changes with focused parser, validation, integration, and documentation checks.

#### Scenario: Behavioral sync tests cover command dependency refs
- **WHEN** the command dependency reference grammar changes
- **THEN** tests SHALL verify CUE and Go accept/reject the same bare and source-qualified dependency reference examples

#### Scenario: Behavioral sync tests cover containerfile policy
- **WHEN** the containerfile path policy changes
- **THEN** tests SHALL cover accepted names containing `..`, rejected parent-directory segments, slash and backslash traversal forms, and existing valid relative examples

#### Scenario: Runtime diagnostics are tested
- **WHEN** runtime variant diagnostics are improved
- **THEN** tests SHALL assert the actionable error message for at least one native, one virtual, and one container source invariant failure

#### Scenario: LLM schema rename is tested
- **WHEN** the LLM schema helper is renamed
- **THEN** schema sync or documentation tests SHALL fail if current config snippets still reference the old helper name

#### Scenario: Documentation integrity checks run
- **WHEN** docs, snippets, AGENTS indexes, rules, or skills are changed
- **THEN** the implementation SHALL run the repository's relevant documentation integrity checks, including `make check-agent-docs` when agent docs are touched

### Requirement: Environment allowlists require allow mode
Runtime environment allowlists SHALL be valid only when the same runtime environment inheritance configuration explicitly selects allow mode.

#### Scenario: Allowlist with allow mode is accepted
- **WHEN** an invowkfile runtime declares `env_inherit_mode: "allow"` and `env_inherit_allow: ["PATH", "TERM"]`
- **THEN** CUE parsing and Invowk validation SHALL accept the runtime environment inheritance configuration when all variable names are valid

#### Scenario: Allowlist with omitted mode is rejected
- **WHEN** an invowkfile runtime declares `env_inherit_allow: ["PATH"]` without declaring `env_inherit_mode`
- **THEN** Invowk validation SHALL reject the runtime with an error explaining that `env_inherit_allow` requires `env_inherit_mode: "allow"`

#### Scenario: Allowlist with none mode is rejected
- **WHEN** an invowkfile runtime declares `env_inherit_mode: "none"` and `env_inherit_allow: ["PATH"]`
- **THEN** Invowk validation SHALL reject the runtime with an error explaining that `env_inherit_allow` requires `env_inherit_mode: "allow"`

#### Scenario: Allowlist with all mode is rejected
- **WHEN** an invowkfile runtime declares `env_inherit_mode: "all"` and `env_inherit_allow: ["PATH"]`
- **THEN** Invowk validation SHALL reject the runtime with an error explaining that `env_inherit_allow` requires `env_inherit_mode: "allow"`

#### Scenario: Non-allow modes without allowlists remain valid
- **WHEN** an invowkfile runtime declares `env_inherit_mode: "none"` or `env_inherit_mode: "all"` without `env_inherit_allow`
- **THEN** CUE parsing and Invowk validation SHALL continue to accept the runtime environment inheritance configuration

### Requirement: Flag and argument descriptions are mandatory everywhere
Command flag and positional argument descriptions SHALL be mandatory in CUE schemas, full invowkfile validation, generated examples, and direct Go value validation.

#### Scenario: Flag with description is accepted
- **WHEN** an invowkfile declares a flag with both `name` and a non-whitespace `description`
- **THEN** CUE parsing, full invowkfile validation, and `Flag.Validate()` SHALL accept the flag when all other flag fields are valid

#### Scenario: Flag missing description is rejected
- **WHEN** an invowkfile or direct Go value declares a flag with `name: "verbose"` and no description
- **THEN** CUE parsing or `Flag.Validate()` SHALL reject the flag because `description` is required

#### Scenario: Flag whitespace description is rejected
- **WHEN** an invowkfile or direct Go value declares a flag whose `description` contains only whitespace
- **THEN** CUE parsing, full invowkfile validation, or `Flag.Validate()` SHALL reject the flag because `description` must contain non-whitespace text

#### Scenario: Argument with description is accepted
- **WHEN** an invowkfile declares a positional argument with both `name` and a non-whitespace `description`
- **THEN** CUE parsing, full invowkfile validation, and `Argument.Validate()` SHALL accept the argument when all other argument fields are valid

#### Scenario: Argument missing description is rejected
- **WHEN** an invowkfile or direct Go value declares an argument with `name: "file"` and no description
- **THEN** CUE parsing or `Argument.Validate()` SHALL reject the argument because `description` is required

#### Scenario: Argument whitespace description is rejected
- **WHEN** an invowkfile or direct Go value declares an argument whose `description` contains only whitespace
- **THEN** CUE parsing, full invowkfile validation, or `Argument.Validate()` SHALL reject the argument because `description` must contain non-whitespace text

### Requirement: Container runtime source selection is schema and Go coherent
The container runtime configuration SHALL require exactly one source field, `image` or `containerfile`, and the same invariant SHALL be represented by CUE validation and Go validation.

#### Scenario: Container runtime with image is accepted
- **WHEN** an invowkfile runtime declares `name: "container"` and `image: "debian:stable-slim"` without `containerfile`
- **THEN** CUE parsing and Invowk validation SHALL accept the runtime source configuration

#### Scenario: Container runtime with containerfile is accepted
- **WHEN** an invowkfile runtime declares `name: "container"` and `containerfile: "Containerfile"` without `image`
- **THEN** CUE parsing and Invowk validation SHALL accept the runtime source configuration when the containerfile path is otherwise valid

#### Scenario: Container runtime missing source is rejected
- **WHEN** an invowkfile runtime declares `name: "container"` without `image` or `containerfile`
- **THEN** CUE parsing or Invowk validation SHALL reject the runtime with an actionable error explaining that container runtime requires either `image` or `containerfile`

#### Scenario: Container runtime with duplicated source is rejected
- **WHEN** an invowkfile runtime declares `name: "container"` with both `image` and `containerfile`
- **THEN** CUE parsing or Invowk validation SHALL reject the runtime with an actionable error explaining that `image` and `containerfile` are mutually exclusive

#### Scenario: Direct Go runtime validation preserves the source invariant
- **WHEN** a `RuntimeConfig` value is constructed directly in Go with `Name` set to container and with zero or two source fields
- **THEN** `RuntimeConfig.Validate()` SHALL reject the value with the same source invariant used for parsed invowkfiles

### Requirement: Module requirement paths use CUE as a portable prefilter
The `invowkmod` CUE schema SHALL validate `requires.path` only as a portable shape prefilter, while Go validation SHALL remain responsible for cross-platform traversal and absolute-path rejection.

#### Scenario: Consecutive dots inside a path segment are accepted
- **WHEN** an `invowkmod.cue` requirement declares `path: "modules/foo..bar"` or `path: "modules/v1..2/tools"`
- **THEN** CUE parsing and `SubdirectoryPath.Validate()` SHALL accept the path when all other path rules pass

#### Scenario: Parent segment at start is rejected
- **WHEN** an `invowkmod.cue` requirement declares `path: "../tools"`
- **THEN** Invowk post-decode validation SHALL reject the path because parent-directory traversal is not allowed

#### Scenario: Parent segment after normalization is rejected
- **WHEN** an `invowkmod.cue` requirement declares `path: "modules/../../secret"` or `path: "modules\\..\\secret"`
- **THEN** Invowk post-decode validation SHALL reject the path after separator normalization

#### Scenario: Absolute and drive-qualified paths are rejected
- **WHEN** an `invowkmod.cue` requirement declares a Unix absolute path, Windows drive-qualified path, UNC path, or rooted Windows path
- **THEN** Invowk post-decode validation SHALL reject the path before it is used for module source resolution

#### Scenario: Schema comment identifies validation ownership
- **WHEN** developers inspect `pkg/invowkmod/invowkmod_schema.cue`
- **THEN** the `requires.path` comment SHALL describe CUE as a portable prefilter and SHALL identify cross-platform traversal and absolute-path checks as Go-owned validation

### Requirement: Tightened schema contracts are covered by tests and docs
Invowk SHALL cover the tightened schema contracts with behavioral tests, direct value validation tests, parser diagnostics tests, and user-facing documentation updates.

#### Scenario: Environment allowlist tests cover all modes
- **WHEN** runtime environment inheritance validation changes
- **THEN** tests SHALL cover allowlists with omitted, `none`, `all`, and `allow` modes

#### Scenario: Flag and argument tests cover value-level descriptions
- **WHEN** flag or argument validation changes
- **THEN** tests SHALL cover missing, whitespace-only, and valid descriptions through both parsed CUE fixtures and direct Go value validation

#### Scenario: Runtime source tests cover schema and diagnostics
- **WHEN** container runtime source validation changes
- **THEN** tests SHALL cover accepted `image`, accepted `containerfile`, missing source, duplicated source, and actionable diagnostic text

#### Scenario: Module path tests cover relaxed CUE and Go security
- **WHEN** module requirement path validation changes
- **THEN** tests SHALL cover safe consecutive-dot segments accepted by CUE and Go, plus traversal and absolute-path examples rejected by Go

#### Scenario: Documentation reflects mandatory descriptions and validation ownership
- **WHEN** README, website docs, snippets, schema references, samples, or agent guidance describe affected fields
- **THEN** they SHALL state that flag and argument descriptions are required, `env_inherit_allow` requires allow mode, container runtime needs exactly one source, and module requirement path traversal is Go-owned validation

### Requirement: Executable scripts use explicit content or file variants
The `invowkfile` implementation `script` field and custom-check `script` field SHALL be closed objects that select exactly one script source variant: `script.content` for inline script content or `script.file` for a module-contained script-file reference. Custom checks SHALL use `script` and SHALL NOT retain `check_script`.

#### Scenario: Inline script content is accepted
- **WHEN** an `invowkfile.cue` implementation declares `script: {content: "echo hello"}`
- **THEN** CUE parsing and Invowk validation SHALL accept the implementation when all other implementation fields are valid

#### Scenario: Multi-line inline script content is accepted
- **WHEN** an `invowkfile.cue` implementation declares `script: {content: """\necho hello\n"""}`
- **THEN** CUE parsing and Invowk validation SHALL accept the implementation when all other implementation fields are valid

#### Scenario: Implementation script file reference is accepted in a module
- **WHEN** an invowkmod `invowkfile.cue` implementation declares `script: {file: "scripts/build.sh"}` and the resolved file is contained in the same invowkmod
- **THEN** CUE parsing and Invowk validation SHALL accept the implementation when all other implementation fields are valid

#### Scenario: Extensionless script file reference is accepted
- **WHEN** an invowkmod `invowkfile.cue` implementation declares `script: {file: "scripts/build"}` and the resolved file is contained in the same invowkmod
- **THEN** CUE parsing and Invowk validation SHALL accept the implementation without requiring a `./` prefix or known script-file extension

#### Scenario: Custom check inline script content is accepted
- **WHEN** an `invowkfile.cue` custom check declares `script: {content: "docker info >/dev/null 2>&1"}`
- **THEN** CUE parsing and Invowk validation SHALL accept the custom check when all other custom-check fields are valid

#### Scenario: Custom check file reference is accepted in a module
- **WHEN** an invowkmod `invowkfile.cue` custom check declares `script: {file: "scripts/check-docker.sh"}` and the resolved file is contained in the same invowkmod
- **THEN** CUE parsing and Invowk validation SHALL accept the custom check when all other custom-check fields are valid

#### Scenario: Non-module script file reference is rejected
- **WHEN** a non-module `invowkfile.cue` implementation or custom check declares `script.file`
- **THEN** Invowk validation SHALL reject the file-backed script source because `script.file` is allowed only for invowkfiles loaded from an invowkmod

#### Scenario: Old implementation script string is rejected
- **WHEN** an `invowkfile.cue` implementation declares `script: "echo hello"`
- **THEN** CUE parsing or Invowk validation SHALL reject the implementation because implementation scripts must use `script.content` or `script.file`

#### Scenario: Old custom check_script field is rejected
- **WHEN** an `invowkfile.cue` custom check declares `check_script: "echo hello"`
- **THEN** CUE parsing or Invowk validation SHALL reject the custom check because custom checks must use `script.content` or `script.file`

#### Scenario: Empty script object is rejected
- **WHEN** an `invowkfile.cue` implementation or custom check declares `script: {}`
- **THEN** CUE parsing or Invowk validation SHALL reject the entry because exactly one script variant is required

#### Scenario: Duplicated script variants are rejected
- **WHEN** an `invowkfile.cue` implementation or custom check declares both `script.content` and `script.file`
- **THEN** CUE parsing or Invowk validation SHALL reject the entry because the script source variants are mutually exclusive

#### Scenario: Empty inline content is rejected
- **WHEN** an `invowkfile.cue` implementation or custom check declares `script: {content: ""}`
- **THEN** CUE parsing or Invowk validation SHALL reject the entry because script content must be non-empty

#### Scenario: Empty file reference is rejected
- **WHEN** an `invowkfile.cue` implementation or custom check declares `script: {file: ""}`
- **THEN** CUE parsing or Invowk validation SHALL reject the entry because script file paths must be non-empty

### Requirement: Script resolution no longer uses file-detection heuristics
Invowk SHALL determine executable script mode from the explicit `script.content` or `script.file` field and SHALL NOT infer file mode from path prefixes, Windows drive-letter shape, or known script-file extensions.

#### Scenario: File-like inline content remains content
- **WHEN** an implementation declares `script: {content: "scripts/build.sh"}`
- **THEN** Invowk SHALL treat the value as inline script content and SHALL NOT attempt to read `scripts/build.sh` as a file

#### Scenario: File-like custom check content remains content
- **WHEN** a custom check declares `script: {content: "scripts/check.sh"}`
- **THEN** Invowk SHALL treat the value as inline custom-check script content and SHALL NOT attempt to read `scripts/check.sh` as a file

#### Scenario: Relative file reference resolves as a file even without extension
- **WHEN** an implementation or custom check declares `script: {file: "scripts/build"}` in an invowkmod invowkfile
- **THEN** Invowk SHALL resolve the script file path relative to the module root because the file variant was selected explicitly

#### Scenario: Dot-prefixed file reference remains valid
- **WHEN** an implementation or custom check declares `script: {file: "./scripts/build.sh"}` in an invowkmod invowkfile
- **THEN** Invowk SHALL resolve the script file path using the same module-relative semantics as other script file references

#### Scenario: Windows-style file reference remains explicit
- **WHEN** an implementation or custom check declares `script: {file: "C:\\tools\\build.ps1"}`
- **THEN** Invowk SHALL treat the value as a script file reference because it is declared in `script.file`, and SHALL reject it unless the resolved target is contained in the source invowkmod

### Requirement: Old executable-script wiring is removed
Invowk SHALL remove old implementation-script string wiring and custom-check `check_script` wiring and SHALL NOT keep fallback parsing, dual-shape decoding, automatic conversion, aliases, or heuristic mode selection for executable script sources.

#### Scenario: Implementation model no longer stores scripts as a plain string
- **WHEN** developers inspect the Go implementation model
- **THEN** implementation scripts SHALL be represented by an explicit content-or-file model rather than by a plain `ScriptContent` string field

#### Scenario: Custom check model no longer stores scripts as check_script
- **WHEN** developers inspect the Go custom-check dependency model
- **THEN** custom-check scripts SHALL be represented by the explicit `script` content-or-file model rather than by a `CheckScript ScriptContent` field or `check_script` JSON tag

#### Scenario: Heuristic extension list is removed from executable script mode selection
- **WHEN** developers inspect script-mode selection logic
- **THEN** there SHALL NOT be an executable-script extension list that decides file mode from suffixes such as `.sh`, `.py`, or `.ps1`

#### Scenario: Heuristic path-prefix detection is removed from executable script mode selection
- **WHEN** developers inspect script-mode selection logic
- **THEN** there SHALL NOT be executable-script logic that decides file mode from `./`, `../`, `/`, or Windows drive-letter prefixes

#### Scenario: Old helper methods do not preserve string inference
- **WHEN** helper methods for selected script content or selected script file paths remain after the change
- **THEN** those helpers SHALL derive behavior only from explicit `script.content` or `script.file` fields

### Requirement: Script file behavior is module-contained
`script.file` SHALL be allowed only when the source `invowkfile.cue` is loaded from an invowkmod. Invowk SHALL resolve file-backed implementation scripts and custom-check scripts against that invowkmod and SHALL reject any resolved target outside the same invowkmod before reading file content.

#### Scenario: Module implementation script file resolves from module root
- **WHEN** a module command declares `script: {file: "scripts/build.sh"}`
- **THEN** Invowk SHALL resolve the script file relative to the module root before reading it

#### Scenario: Module custom-check script file resolves from module root
- **WHEN** a custom check in a module invowkfile declares `script: {file: "scripts/check.sh"}`
- **THEN** Invowk SHALL resolve the custom-check script file relative to the module root before reading it

#### Scenario: Non-module implementation script file is rejected
- **WHEN** a non-module invowkfile command declares `script: {file: "scripts/build.sh"}`
- **THEN** Invowk SHALL reject the implementation because file-backed scripts require a source invowkfile inside an invowkmod

#### Scenario: Non-module custom-check script file is rejected
- **WHEN** a custom check in a non-module invowkfile declares `script: {file: "scripts/check.sh"}`
- **THEN** Invowk SHALL reject the custom check because file-backed scripts require a source invowkfile inside an invowkmod

#### Scenario: Module implementation script file traversal is rejected
- **WHEN** a module command declares a `script.file` value that resolves outside the module root
- **THEN** Invowk SHALL reject script resolution before reading outside the module boundary

#### Scenario: Module custom-check script file traversal is rejected
- **WHEN** a custom check in a module invowkfile declares a `script.file` value that resolves outside the module root
- **THEN** Invowk SHALL reject script resolution before reading outside the module boundary

#### Scenario: Absolute file reference outside the module is rejected
- **WHEN** an implementation or custom check in a module invowkfile declares an absolute, rooted, drive-qualified, or UNC `script.file` value whose resolved target is outside the source module
- **THEN** Invowk SHALL reject script resolution before reading outside the module boundary

#### Scenario: Missing script file reports the selected path
- **WHEN** an implementation or custom check declares `script.file` and the resolved module-contained file cannot be read
- **THEN** Invowk SHALL fail execution with an actionable error that includes the resolved script file path

#### Scenario: Resolved script file content is validated
- **WHEN** `script.file` points to a file whose content is empty or otherwise invalid as script content
- **THEN** Invowk SHALL reject the resolved script content before execution

#### Scenario: Container custom-check file runs as resolved content
- **WHEN** a runtime-level custom check inside a container runtime block declares `script.file` in a module invowkfile
- **THEN** Invowk SHALL read and validate the module-contained file on the host/module side and execute the resolved script content inside the selected container dependency probe

### Requirement: First-party examples and documentation use explicit script variants
All first-party command implementation examples, custom-check examples, and fixtures SHALL use `script.content` or `script.file`. Non-module/root invowkfile examples SHALL use `script.content`; file-backed examples SHALL live in invowkmod examples or fixtures where the referenced file is contained in the module.

#### Scenario: Root invowkfile command examples are migrated
- **WHEN** developers inspect the repository root `invowkfile.cue`
- **THEN** every implementation script example SHALL use `script.content` because root invowkfiles cannot use `script.file`

#### Scenario: Sample modules are migrated
- **WHEN** developers inspect safe sample modules and audit fixture modules under `samples/invowkmods/`
- **THEN** every implementation script and custom-check script SHALL use the explicit script object shape, with `script.file` references contained in the owning module

#### Scenario: CLI fixtures are migrated
- **WHEN** CLI testscript fixtures define invowkfile command implementations
- **THEN** those implementation scripts and custom-check scripts SHALL use the explicit script object shape

#### Scenario: README examples are migrated
- **WHEN** users read README command or custom-check examples
- **THEN** executable scripts SHALL be shown with `script.content` or module-contained `script.file` and SHALL NOT teach old implementation `script: "..."` or custom-check `check_script: "..."` usage

#### Scenario: Website and localized docs are migrated
- **WHEN** users read current, versioned, or localized website documentation that shows implementation scripts or custom checks
- **THEN** those examples SHALL use the explicit script object shape and SHALL NOT retain old implementation `script: "..."` or custom-check `check_script: "..."` examples

#### Scenario: Generated schema references are migrated
- **WHEN** generated schema/reference documentation describes implementation scripts or custom-check scripts
- **THEN** it SHALL document `script.content`, `script.file`, their mutual exclusivity, module-only file semantics, and clean-break examples that replace the old string shapes

#### Scenario: Dependency custom-check examples are migrated
- **WHEN** examples use dependency custom checks
- **THEN** those custom checks SHALL use `script.content` or module-contained `script.file` and SHALL NOT use `check_script`

### Requirement: Workdir documentation reflects existing execution semantics
Invowk SHALL document `workdir` as runtime-neutral execution configuration whose schema placement remains root-, command-, and implementation-level, with the existing effective precedence and path-resolution semantics.

#### Scenario: Workdir precedence is documented
- **WHEN** users read schema comments, README content, website docs, generated references, or examples for `workdir`
- **THEN** the documentation SHALL state that effective workdir precedence is CLI override, implementation, command, root, then default invowkfile or module directory

#### Scenario: Workdir runtime neutrality is documented
- **WHEN** users read `workdir` documentation
- **THEN** the documentation SHALL state that `workdir` applies across native, virtual, and container execution rather than belonging to one runtime-specific config shape

#### Scenario: Workdir placement remains unchanged
- **WHEN** an invowkfile declares root-level, command-level, or implementation-level `workdir`
- **THEN** CUE parsing and Invowk validation SHALL continue to accept the field at the existing locations when the value is otherwise valid

#### Scenario: Container workdir behavior is documented
- **WHEN** users read container runtime or workdir documentation
- **THEN** the documentation SHALL describe the existing container workdir mapping behavior for relative and absolute workdir values

#### Scenario: Workdir examples use the new script shape
- **WHEN** workdir examples include command implementations
- **THEN** those examples SHALL use `script.content` or `script.file` while preserving the existing workdir behavior being demonstrated

### Requirement: Invowkfile validation ownership is documented for script and workdir fields
Invowk SHALL document which script and workdir checks are owned by CUE structural validation and which checks remain Go/runtime semantic validation.

#### Scenario: CUE-owned script checks are documented
- **WHEN** developers read invowkfile schema comments or generated schema references for implementation scripts or custom-check scripts
- **THEN** they SHALL identify CUE as responsible for the closed `script` object shape, required variant selection, non-empty values, and maximum lengths

#### Scenario: Go-owned script checks are documented
- **WHEN** developers read invowkfile schema comments, Go validators, or generated references for script file behavior
- **THEN** they SHALL identify Go/runtime validation as responsible for module-context checks, script-file path resolution, module containment, filesystem reads, and resolved script content validation

#### Scenario: Workdir semantic checks are documented
- **WHEN** developers read schema comments, Go validators, or documentation for `workdir`
- **THEN** they SHALL identify CUE as responsible for local string shape and Go/runtime logic as responsible for effective precedence, path resolution, container mapping, and execution-time directory validation

#### Scenario: Contextual command validation remains Go-owned
- **WHEN** documentation discusses validations that require command discovery, selected implementation, module context, or runtime context
- **THEN** it SHALL describe those validations as Go-owned semantic checks rather than pure CUE schema checks

### Requirement: Explicit script clean break is covered by tests and verification
Invowk SHALL cover the explicit script clean break with parser tests, value validation tests, script-resolution tests, runtime tests, generated-output tests, fixture coverage, and documentation checks.

#### Scenario: Parser tests cover script variants
- **WHEN** invowkfile parser tests run
- **THEN** they SHALL cover accepted `script.content`, accepted module-contained `script.file`, rejected non-module `script.file`, rejected old implementation string scripts, rejected old custom-check `check_script`, rejected missing variants, rejected duplicated variants, and rejected empty values

#### Scenario: No old executable-script wiring remains
- **WHEN** clean-break verification searches implementation code
- **THEN** it SHALL find no remaining fallback decoder, automatic converter, file-extension heuristic, path-prefix heuristic, plain-string implementation script field, custom-check `CheckScript` field, or `check_script` JSON/CUE field

#### Scenario: Script resolution tests cover explicit mode selection
- **WHEN** script resolution tests run
- **THEN** they SHALL prove that `script.content` never triggers file reads, that module-contained `script.file` triggers file resolution regardless of extension or path prefix, and that non-module `script.file` is rejected

#### Scenario: Runtime tests cover resolved scripts
- **WHEN** native, virtual, or container runtime tests execute selected implementations
- **THEN** they SHALL continue to execute resolved inline and file-backed scripts through the selected runtime

#### Scenario: Custom-check tests cover resolved scripts
- **WHEN** host and container dependency custom-check tests run
- **THEN** they SHALL cover inline custom-check content, module-contained custom-check files, non-module file rejection, missing custom-check files, resolved-content validation, and expected-code/output behavior after script resolution

#### Scenario: Generated CUE round-trips
- **WHEN** Invowk generates CUE for commands with inline or file-backed implementation scripts or custom-check scripts
- **THEN** the generated CUE SHALL use the explicit script object shape and SHALL parse back to an equivalent invowkfile model

#### Scenario: Documentation checks cover migrated examples
- **WHEN** documentation integrity, snippet validation, website build, website typecheck, or i18n parity checks are required by touched docs
- **THEN** those checks SHALL pass with examples that use the explicit script object shape and no `check_script` examples

#### Scenario: OpenSpec validation passes
- **WHEN** the change artifacts are complete
- **THEN** `openspec validate explicit-script-fields-and-workdir-docs --strict` SHALL pass
