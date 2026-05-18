## ADDED Requirements

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
