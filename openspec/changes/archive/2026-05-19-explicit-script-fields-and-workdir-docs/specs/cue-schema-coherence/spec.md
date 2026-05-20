## ADDED Requirements

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
