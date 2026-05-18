## ADDED Requirements

### Requirement: Command dependency alternatives use dependency references
Invowk SHALL parse `depends_on.cmds[].alternatives` as command dependency references rather than raw command names.

#### Scenario: Bare local command reference is accepted
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["build"]}]`
- **THEN** CUE parsing and Go validation SHALL accept the alternative as a bare local command reference

#### Scenario: Bare subcommand reference is accepted
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["test unit"]}]`
- **THEN** CUE parsing and Go validation SHALL accept the alternative as a bare local command reference whose command name contains a space

#### Scenario: Source-qualified command reference is accepted
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["@tools lint"]}]`
- **THEN** CUE parsing and Go validation SHALL accept the alternative as command `lint` from source `tools`

#### Scenario: Dotted source-qualified command reference is accepted
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["@com.company.tools lint"]}]`
- **THEN** CUE parsing and Go validation SHALL accept the alternative as command `lint` from source `com.company.tools`

#### Scenario: Source-qualified subcommand reference is accepted
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["@tools test unit"]}]`
- **THEN** CUE parsing and Go validation SHALL accept the alternative as command `test unit` from source `tools`

#### Scenario: Missing command part is rejected
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["@tools"]}]`
- **THEN** CUE parsing or Go validation SHALL reject the alternative because the source-qualified reference has no command name

#### Scenario: Invalid source ID is rejected
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["@9tools lint"]}]`
- **THEN** CUE parsing or Go validation SHALL reject the alternative because the source ID is invalid

#### Scenario: Invalid command part is rejected
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["@tools 9lint"]}]`
- **THEN** CUE parsing or Go validation SHALL reject the alternative because the command name is invalid

### Requirement: Source-prefix-with-space syntax is removed
Invowk SHALL NOT treat dependency alternatives without an `@source` prefix as source-qualified references.

#### Scenario: Old alias prefix does not resolve as source-qualified
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["tools lint"]}]`
- **THEN** Invowk SHALL interpret the value only as a bare local command named `tools lint`

#### Scenario: Old dotted prefix is rejected or treated as invalid bare command
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["com.company.tools lint"]}]`
- **THEN** CUE parsing or Go validation SHALL reject the value as an invalid bare command reference and SHALL NOT treat `com.company.tools` as a source ID

#### Scenario: Missing old local command fails as missing local command
- **WHEN** a command declares `depends_on.cmds: [{alternatives: ["tools lint"]}]` and the caller's own source does not define command `tools lint`
- **THEN** dependency validation SHALL fail as a missing local command dependency, not as a missing source-qualified dependency

#### Scenario: Documentation contains no old syntax
- **WHEN** users read command dependency documentation or snippets
- **THEN** examples SHALL use `@tools lint` rather than `tools lint` for cross-source dependencies

### Requirement: Bare command dependency references resolve locally
Bare command dependency references SHALL resolve only against the command source that owns the declaring command.

#### Scenario: Root invowkfile bare reference resolves to root source
- **WHEN** a root invowkfile command declares a bare dependency reference `build`
- **THEN** dependency validation SHALL search for command `build` from source `invowkfile`

#### Scenario: Module bare reference resolves to same module source
- **WHEN** a module command declares a bare dependency reference `build`
- **THEN** dependency validation SHALL search for command `build` from the same module source as the declaring command

#### Scenario: Bare reference does not bind external source
- **WHEN** a module command declares bare dependency reference `build`, the same module does not define `build`, and another visible source defines `build`
- **THEN** dependency validation SHALL fail rather than binding the external command

#### Scenario: Cross-source dependency requires explicit source
- **WHEN** a command needs command `lint` from source `tools`
- **THEN** the dependency SHALL be declared as `@tools lint`

### Requirement: Source-qualified command dependency references resolve through scope policy
Source-qualified command dependency references SHALL resolve by source ID and command name, then apply command-scope authorization.

#### Scenario: Direct dependency source is allowed
- **WHEN** a module command declares `@tools lint` and `tools` is the effective source ID of a direct dependency allowed by the module lock scope
- **THEN** dependency validation SHALL accept the reference when command `lint` exists in source `tools`

#### Scenario: Same source is allowed
- **WHEN** a command declares `@tools build` and the declaring command also belongs to source `tools`
- **THEN** dependency validation SHALL accept the reference when command `build` exists in the same source

#### Scenario: Global source is allowed when in scope
- **WHEN** a command declares `@global-tools fmt` and `global-tools` is a globally installed command source visible to the caller's scope
- **THEN** dependency validation SHALL accept the reference when command `fmt` exists in source `global-tools`

#### Scenario: Out-of-scope source is forbidden
- **WHEN** a module command declares `@other-tools lint`, command `lint` exists in source `other-tools`, and `other-tools` is not same-source, global, or a direct dependency source allowed by scope
- **THEN** dependency validation SHALL fail with a forbidden-command dependency diagnostic naming the inaccessible source

#### Scenario: Unknown source is missing
- **WHEN** a command declares `@missing-tools lint` and no visible command source has source ID `missing-tools`
- **THEN** dependency validation SHALL fail with a missing command dependency diagnostic that includes `@missing-tools lint`

#### Scenario: Unknown command in known source is missing
- **WHEN** a command declares `@tools lint` and source `tools` exists but does not define command `lint`
- **THEN** dependency validation SHALL fail with a missing command dependency diagnostic that includes `@tools lint`

### Requirement: Command dependency reference diagnostics preserve user input
Validation and dependency errors SHALL report dependency references in the same syntax users write.

#### Scenario: Missing qualified reference is reported with at-prefix
- **WHEN** dependency validation fails for `@tools lint`
- **THEN** the diagnostic SHALL include `@tools lint`

#### Scenario: Forbidden qualified reference names source and command
- **WHEN** dependency validation rejects `@other-tools lint` because the source is out of scope
- **THEN** the diagnostic SHALL identify source `other-tools` and command `lint`

#### Scenario: Invalid reference reports reference grammar
- **WHEN** parsing rejects `@tools` or `@9tools lint`
- **THEN** the diagnostic SHALL explain the expected dependency reference format

### Requirement: Command dependency reference docs are complete
Invowk documentation SHALL describe command dependency references with local and source-qualified examples.

#### Scenario: Reference docs define grammar
- **WHEN** users read the invowkfile dependency reference
- **THEN** it SHALL define bare local references and explicit `@source command` references

#### Scenario: Module docs show direct dependency source reference
- **WHEN** users read module dependency documentation
- **THEN** it SHALL show a direct dependency with an alias and a command dependency such as `@tools lint`

#### Scenario: Ambiguity with command names containing spaces is explained
- **WHEN** users read command dependency documentation
- **THEN** it SHALL explain that `tools lint` is a single bare command name, while `@tools lint` is command `lint` from source `tools`

#### Scenario: CLI disambiguation docs remain consistent
- **WHEN** users read CLI source-disambiguation docs
- **THEN** examples using `invowk cmd @tools deploy` SHALL remain consistent with dependency reference examples using `@tools deploy`
