## ADDED Requirements

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

## MODIFIED Requirements

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
