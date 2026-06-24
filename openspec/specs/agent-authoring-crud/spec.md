# agent-authoring-crud Specification

## Purpose
TBD - created by archiving change expand-agent-authoring-crud. Update Purpose after archive.
## Requirements
### Requirement: Explicit agent authoring command tree
Invowk SHALL expose explicit command and module authoring operations under `invowk agent cmd` and `invowk agent mod`.

#### Scenario: Command authoring verbs are available
- **WHEN** a user runs `invowk agent cmd --help`
- **THEN** the help output SHALL list `create`, `change`, `remove`, and `prompt`

#### Scenario: Module authoring verbs are available
- **WHEN** a user runs `invowk agent mod --help`
- **THEN** the help output SHALL list `create`, `change`, `remove`, and `prompt`

#### Scenario: CRUD is not coalesced into one public command
- **WHEN** this change is implemented
- **THEN** Invowk SHALL NOT expose a single public `agent cmd crud`, `agent cmd apply`, `agent mod crud`, or `agent mod apply` command as the primary user-facing interface for these operations

### Requirement: Command create requires caller supplied identity
Invowk SHALL require `invowk agent cmd create <name> [description...]` and SHALL use `<name>` as the generated command's durable identity.

#### Scenario: Legacy description-only create is rejected
- **WHEN** a user runs `invowk agent cmd create 'add a lint command'`
- **THEN** Invowk SHALL treat `add a lint command` as the required `<name>` argument rather than a description-only legacy form
- **AND** if no separate description or `--from-file` content is provided, Invowk SHALL fail with an error explaining that a command description is required

#### Scenario: Create writes command with requested name
- **WHEN** a user runs `invowk agent cmd create lint 'run golangci-lint'`
- **THEN** Invowk SHALL request one generated command object from the configured LLM provider
- **AND** the generated command object SHALL be accepted only if its `name` field is exactly `lint`
- **AND** Invowk SHALL add the command to the target invowkfile when no command named `lint` exists

#### Scenario: Create rejects existing command
- **WHEN** the target invowkfile already contains command `lint`
- **AND** a user runs `invowk agent cmd create lint 'run golangci-lint'`
- **THEN** Invowk SHALL fail without modifying the file
- **AND** the error SHALL direct the user to `invowk agent cmd change lint`

#### Scenario: Create rejects model identity mismatch
- **WHEN** a user runs `invowk agent cmd create lint 'run golangci-lint'`
- **AND** the LLM response contains a command object with `name: "test"`
- **THEN** Invowk SHALL reject the response and use validation feedback for the bounded repair attempt
- **AND** Invowk SHALL fail without writing if the final response still does not use `lint`

#### Scenario: Replace flag is removed
- **WHEN** a user runs `invowk agent cmd create --replace lint 'change lint'`
- **THEN** Invowk SHALL fail because `--replace` is not a valid flag for `agent cmd create`

### Requirement: Command change replaces existing command
Invowk SHALL provide `invowk agent cmd change <name> [description...]` for LLM-assisted replacement of exactly one existing command in a target invowkfile.

#### Scenario: Change updates requested command only
- **WHEN** the target invowkfile contains commands `lint` and `test`
- **AND** a user runs `invowk agent cmd change lint 'make lint stricter'`
- **THEN** Invowk SHALL send the current target invowkfile content and the existing `lint` command context to the LLM provider
- **AND** Invowk SHALL replace only the `lint` command object after validation succeeds
- **AND** command `test` SHALL remain unchanged

#### Scenario: Change rejects missing command
- **WHEN** the target invowkfile does not contain command `lint`
- **AND** a user runs `invowk agent cmd change lint 'make lint stricter'`
- **THEN** Invowk SHALL fail without modifying the file
- **AND** the error SHALL direct the user to `invowk agent cmd create lint`

#### Scenario: Change preserves requested identity
- **WHEN** a user runs `invowk agent cmd change lint 'make lint stricter'`
- **AND** the LLM response contains a command object with a name other than `lint`
- **THEN** Invowk SHALL reject the response and SHALL NOT replace any command unless a bounded repair returns a command named `lint`

### Requirement: Command remove is deterministic
Invowk SHALL provide `invowk agent cmd remove <name>` for deterministic removal of exactly one command from a target invowkfile.

#### Scenario: Remove deletes existing command
- **WHEN** the target invowkfile contains command `lint`
- **AND** a user runs `invowk agent cmd remove lint`
- **THEN** Invowk SHALL remove the `lint` command object without calling an LLM provider
- **AND** Invowk SHALL validate the resulting invowkfile before writing it

#### Scenario: Remove rejects missing command
- **WHEN** the target invowkfile does not contain command `lint`
- **AND** a user runs `invowk agent cmd remove lint`
- **THEN** Invowk SHALL fail without modifying the file

#### Scenario: Remove supports dry-run
- **WHEN** a user runs `invowk agent cmd remove lint --dry-run`
- **THEN** Invowk SHALL print the planned file diff
- **AND** Invowk SHALL NOT modify the target invowkfile

### Requirement: Command authoring modes remain safe
Invowk SHALL support explicit non-writing and verification modes for command create and change operations.

#### Scenario: Command dry-run prints patch only
- **WHEN** a user runs `invowk agent cmd create lint --dry-run 'run lint'`
- **THEN** Invowk SHALL validate the generated command and print the planned patch
- **AND** Invowk SHALL NOT write the target invowkfile

#### Scenario: Command print emits generated command only
- **WHEN** a user runs `invowk agent cmd create lint --print 'run lint'`
- **THEN** Invowk SHALL validate the generated command and print the generated command CUE object
- **AND** Invowk SHALL NOT write the target invowkfile

#### Scenario: Command verify checks written command
- **WHEN** a user runs `invowk agent cmd create lint --verify 'run lint'`
- **THEN** Invowk SHALL write the command after validation succeeds
- **AND** Invowk SHALL verify the written command with a dry-run execution plan

#### Scenario: Command mode flags remain mutually exclusive
- **WHEN** a user combines `--dry-run`, `--print`, or `--verify` in an invalid mode combination
- **THEN** Invowk SHALL fail before calling an LLM provider
- **AND** Invowk SHALL explain the invalid mode combination

### Requirement: Command prompts cover operations
Invowk SHALL render command-authoring prompts and schemas that describe create, change, and remove operation contracts.

#### Scenario: Command prompt supports operation selection
- **WHEN** a user runs `invowk agent cmd prompt create`, `invowk agent cmd prompt change`, or `invowk agent cmd prompt remove`
- **THEN** Invowk SHALL print prompt guidance for the requested command operation

#### Scenario: Command prompt without operation lists supported operations
- **WHEN** a user runs `invowk agent cmd prompt`
- **THEN** Invowk SHALL print command-authoring prompt guidance that describes the supported create, change, and remove operation contracts

#### Scenario: Command prompt JSON is structured
- **WHEN** a user runs `invowk agent cmd prompt change --format json`
- **THEN** Invowk SHALL print JSON containing the operation name, system prompt text, relevant CUE schemas, and the expected response schema when the operation uses an LLM response

### Requirement: Module create requires module identity
Invowk SHALL provide `invowk agent mod create <module-id> [description...]` for LLM-assisted local module scaffold creation.

#### Scenario: Module create writes scaffold with requested identity
- **WHEN** a user runs `invowk agent mod create com.example.tools 'commands for tool automation'`
- **THEN** Invowk SHALL generate a local module directory named `com.example.tools.invowkmod`
- **AND** the generated `invowkmod.cue` SHALL contain `module: "com.example.tools"`
- **AND** the generated module SHALL include a valid `invowkfile.cue`

#### Scenario: Module create rejects existing module directory
- **WHEN** `com.example.tools.invowkmod` already exists
- **AND** a user runs `invowk agent mod create com.example.tools 'commands for tool automation'`
- **THEN** Invowk SHALL fail without modifying the existing module directory
- **AND** the error SHALL direct the user to `invowk agent mod change com.example.tools`

#### Scenario: Module create rejects generated identity mismatch
- **WHEN** a user runs `invowk agent mod create com.example.tools 'commands for tool automation'`
- **AND** the LLM response contains `invowkmod.cue` with a different module ID
- **THEN** Invowk SHALL reject the response and SHALL NOT write any module files unless a bounded repair returns `module: "com.example.tools"`

#### Scenario: Module create can scaffold scripts directory
- **WHEN** a user runs `invowk agent mod create com.example.tools --scripts 'commands for tool automation'`
- **THEN** Invowk SHALL include the scripts directory scaffold in the planned module layout
- **AND** Invowk SHALL NOT create arbitrary generated script files in this change

### Requirement: Module change updates existing local module
Invowk SHALL provide `invowk agent mod change <module-id-or-path> [description...]` for LLM-assisted updates to an existing local module's CUE files.

#### Scenario: Module change updates module CUE files
- **WHEN** a local module `com.example.tools.invowkmod` exists
- **AND** a user runs `invowk agent mod change com.example.tools 'add a lint command'`
- **THEN** Invowk SHALL send the current `invowkmod.cue` and `invowkfile.cue` content to the LLM provider
- **AND** Invowk SHALL write validated updates only to those module CUE files
- **AND** the module ID SHALL remain `com.example.tools`

#### Scenario: Module change rejects missing module
- **WHEN** no local module can be resolved from `com.example.tools`
- **AND** a user runs `invowk agent mod change com.example.tools 'add a lint command'`
- **THEN** Invowk SHALL fail without writing files
- **AND** the error SHALL direct the user to `invowk agent mod create com.example.tools`

#### Scenario: Module change rejects arbitrary file writes
- **WHEN** the LLM response attempts to create or modify files other than `invowkmod.cue` and `invowkfile.cue`
- **THEN** Invowk SHALL reject those file writes for this change
- **AND** Invowk SHALL NOT create or modify arbitrary script files

### Requirement: Module authoring modes remain safe
Invowk SHALL support explicit non-writing and verification modes for module create and change operations.

#### Scenario: Module dry-run prints planned file changes
- **WHEN** a user runs `invowk agent mod create com.example.tools --dry-run 'commands for tool automation'`
- **THEN** Invowk SHALL validate the generated module files and print the planned file additions or diffs
- **AND** Invowk SHALL NOT create the module directory or write module files

#### Scenario: Module print emits generated file bundle
- **WHEN** a user runs `invowk agent mod create com.example.tools --print 'commands for tool automation'`
- **THEN** Invowk SHALL validate the generated module files and print a structured representation of the generated file bundle
- **AND** Invowk SHALL NOT create the module directory or write module files

#### Scenario: Module verify validates written module
- **WHEN** a user runs `invowk agent mod create com.example.tools --verify 'commands for tool automation'`
- **THEN** Invowk SHALL write the generated module after validation succeeds
- **AND** Invowk SHALL verify the written module with the module validation path

#### Scenario: Module mode flags remain mutually exclusive
- **WHEN** a user combines `--dry-run`, `--print`, or `--verify` in an invalid mode combination
- **THEN** Invowk SHALL fail before calling an LLM provider
- **AND** Invowk SHALL explain the invalid mode combination

### Requirement: Module remove is guarded local deletion
Invowk SHALL provide `invowk agent mod remove <module-id-or-path>` for guarded removal of a local module directory and SHALL NOT use it for dependency removal.

#### Scenario: Module remove previews deletion
- **WHEN** a user runs `invowk agent mod remove com.example.tools --dry-run`
- **THEN** Invowk SHALL resolve the matching local module directory
- **AND** Invowk SHALL print the planned deletion without deleting files

#### Scenario: Module remove requires force for deletion
- **WHEN** a user runs `invowk agent mod remove com.example.tools` without `--force`
- **THEN** Invowk SHALL fail before deleting files
- **AND** the error SHALL explain that `--force` is required for destructive module removal

#### Scenario: Module remove deletes exact validated module
- **WHEN** a user runs `invowk agent mod remove com.example.tools --force`
- **AND** `com.example.tools.invowkmod` is a valid local module directory
- **THEN** Invowk SHALL delete that exact module directory
- **AND** Invowk SHALL NOT modify `invowkmod.cue` dependency declarations or lock files

#### Scenario: Module remove rejects symlink targets
- **WHEN** the resolved module path is a symlink or does not validate as a local `.invowkmod` directory
- **THEN** Invowk SHALL fail without deleting files

### Requirement: Module prompts cover operations
Invowk SHALL render module-authoring prompts and schemas that describe create, change, and remove operation contracts.

#### Scenario: Module prompt supports operation selection
- **WHEN** a user runs `invowk agent mod prompt create`, `invowk agent mod prompt change`, or `invowk agent mod prompt remove`
- **THEN** Invowk SHALL print prompt guidance for the requested module operation

#### Scenario: Module prompt without operation lists supported operations
- **WHEN** a user runs `invowk agent mod prompt`
- **THEN** Invowk SHALL print module-authoring prompt guidance that describes the supported create, change, and remove operation contracts

#### Scenario: Module prompt JSON includes module schemas
- **WHEN** a user runs `invowk agent mod prompt create --format json`
- **THEN** Invowk SHALL print JSON containing the operation name, system prompt text, `invowkmod.cue` schema, `invowkfile.cue` schema, and the expected response schema when the operation uses an LLM response

### Requirement: Documentation and tests reflect clean break
Invowk SHALL update user documentation, agent-facing review references, diagrams, and automated tests to match the new authoring contract.

#### Scenario: Tests cover new CLI contract
- **WHEN** this change is implemented
- **THEN** CLI tests SHALL cover command create, change, remove, prompt, module create, module change, module remove, and module prompt
- **AND** tests SHALL assert that legacy `agent cmd create` usage without a separate description and `--replace` fail

#### Scenario: Documentation surfaces are synchronized
- **WHEN** README, website docs, snippets, diagrams, or `.agents` review-doc references mention LLM-assisted authoring
- **THEN** they SHALL describe `agent cmd` and `agent mod` create, change, remove, and prompt behavior accurately
- **AND** documentation checks including `make check-agent-docs` SHALL pass when agent guidance changes are part of the implementation
