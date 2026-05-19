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

