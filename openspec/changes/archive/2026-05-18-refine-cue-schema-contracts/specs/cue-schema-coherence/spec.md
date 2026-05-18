## ADDED Requirements

### Requirement: Schema comments distinguish CUE and Invowk validation ownership
CUE schema comments SHALL clearly distinguish rules enforced by CUE from rules enforced by Invowk after decode.

#### Scenario: Container source invariants identify Go ownership
- **WHEN** developers read the container runtime schema comments for `containerfile` and `image`
- **THEN** the comments SHALL state that CUE validates field shape and Invowk rejects both-fields and missing-source invariants after decode

#### Scenario: Argument and flag semantic rules identify Go ownership
- **WHEN** developers read argument or flag schema comments for required/default conflicts, ordering, variadic position, default type compatibility, regex safety, or reserved system flag names
- **THEN** the comments SHALL identify those rules as Invowk post-decode validation rather than implying CUE alone enforces them

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
