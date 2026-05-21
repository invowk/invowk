# script-scoped-interpreters Specification

## Purpose
Define script-scoped interpreter configuration, resolution, runtime behavior, custom-check execution, and clean-break removal of runtime-level interpreter support.
## Requirements
### Requirement: Script objects own explicit interpreter selection
Invowk SHALL expose explicit interpreter selection only as `script.interpreter` on implementation scripts and custom-check scripts.

#### Scenario: Inline implementation script declares an interpreter
- **WHEN** an `invowkfile.cue` implementation declares `script: {content: "print('ok')", interpreter: "python3"}` with a native runtime
- **THEN** Invowk SHALL parse the command successfully and execute the inline script with `python3`

#### Scenario: File implementation script declares an interpreter
- **WHEN** an invowkmod implementation declares `script: {file: "scripts/build.py", interpreter: "python3"}`
- **THEN** Invowk SHALL read `scripts/build.py` from inside the source invowkmod and execute it with `python3`

#### Scenario: Custom check declares an interpreter
- **WHEN** a custom dependency check declares `script: {content: "print('ok')", interpreter: "python3"}`
- **THEN** Invowk SHALL execute the custom check with `python3` in the check's target environment and validate `expected_code` and `expected_output` against the result

#### Scenario: Runtime-level interpreter is rejected
- **WHEN** an implementation declares `runtimes: [{name: "native", interpreter: "python3"}]`
- **THEN** CUE parsing SHALL reject `interpreter` as an unknown runtime field

#### Scenario: Container runtime-level interpreter is rejected
- **WHEN** an implementation declares `runtimes: [{name: "container", image: "debian:stable-slim", interpreter: "python3"}]`
- **THEN** CUE parsing SHALL reject `interpreter` as an unknown runtime field

#### Scenario: Generated invowkfile output uses script-level interpreter
- **WHEN** Invowk generates CUE for an implementation or custom check with an explicit interpreter
- **THEN** the generated CUE SHALL place `interpreter` inside `script` and SHALL NOT emit runtime-level `interpreter`

### Requirement: Script source variants remain exactly-one source plus optional interpreter
Invowk SHALL require each implementation and custom-check script to select exactly one source, `content` or `file`, while allowing optional `interpreter` with either source.

#### Scenario: Content and interpreter are accepted
- **WHEN** a script declares `content` and `interpreter` without `file`
- **THEN** CUE parsing and Go validation SHALL accept the script subject to script content and interpreter validation

#### Scenario: File and interpreter are accepted in modules
- **WHEN** a module-contained script declares `file` and `interpreter` without `content`
- **THEN** CUE parsing and Go validation SHALL accept the script subject to module file containment and interpreter validation

#### Scenario: Content and file remain mutually exclusive
- **WHEN** a script declares both `content` and `file`
- **THEN** CUE parsing or Go validation SHALL reject the script even when `interpreter` is valid

#### Scenario: Interpreter alone is not a source
- **WHEN** a script declares `interpreter` without `content` or `file`
- **THEN** CUE parsing or Go validation SHALL reject the script because no script source is selected

#### Scenario: Empty interpreter is rejected
- **WHEN** a script declares `interpreter: ""`
- **THEN** CUE parsing or Go validation SHALL reject the interpreter as invalid

#### Scenario: Whitespace-only interpreter is rejected
- **WHEN** a script declares `interpreter: "   "`
- **THEN** CUE parsing or Go validation SHALL reject the interpreter as invalid

#### Scenario: Unsafe interpreter is rejected
- **WHEN** a script declares an interpreter containing shell metacharacters or a non-allowlisted interpreter command
- **THEN** Go validation SHALL reject the interpreter before the script is executed

### Requirement: Interpreter resolution uses final script bytes
Invowk SHALL resolve script content before resolving the interpreter and SHALL use the final script bytes for shebang detection.

#### Scenario: Omitted interpreter uses inline shebang
- **WHEN** an inline script starts with `#!/usr/bin/env python3` and omits `script.interpreter`
- **THEN** Invowk SHALL detect `python3` from the shebang and execute the script with that interpreter in native or container runtimes

#### Scenario: Auto interpreter uses file shebang
- **WHEN** a module-contained `script.file` starts with `#!/usr/bin/env node` and declares `interpreter: "auto"`
- **THEN** Invowk SHALL detect `node` from the resolved file content and execute the script with that interpreter in native or container runtimes

#### Scenario: Explicit interpreter takes precedence over shebang
- **WHEN** a script declares `interpreter: "python3"` and the script content starts with `#!/bin/sh`
- **THEN** Invowk SHALL execute the script with `python3` and SHALL ignore the shebang for interpreter selection

#### Scenario: No interpreter falls back to default shell behavior
- **WHEN** a script omits `interpreter` and has no shebang
- **THEN** Invowk SHALL use the default shell behavior for the selected runtime or custom-check target environment

#### Scenario: File extension does not select interpreter
- **WHEN** a module-contained script file has a recognized extension but no shebang and no `script.interpreter`
- **THEN** Invowk SHALL NOT infer the interpreter from the file extension

### Requirement: Virtual-sh runtime only accepts shell-compatible script interpreters
Invowk SHALL execute `virtual-sh` runtime scripts with the embedded mvdan/sh shell and SHALL reject non-shell interpreter selections for `virtual-sh` implementations.

#### Scenario: Virtual-sh script without interpreter uses mvdan/sh
- **WHEN** a `virtual-sh` implementation script omits `interpreter` and has no shebang
- **THEN** Invowk SHALL execute it with the embedded mvdan/sh runtime

#### Scenario: Virtual-sh script with shell shebang uses mvdan/sh
- **WHEN** a `virtual-sh` implementation script starts with a shell-compatible shebang such as `#!/bin/sh`
- **THEN** Invowk SHALL execute it with the embedded mvdan/sh runtime and SHALL NOT require `/bin/sh` to exist on the host

#### Scenario: Virtual-sh script with explicit shell interpreter uses mvdan/sh
- **WHEN** a `virtual-sh` implementation script declares `script.interpreter: "sh"`
- **THEN** Invowk SHALL accept the script as shell-compatible and execute it with the embedded mvdan/sh runtime

#### Scenario: Virtual-sh script rejects non-shell interpreter
- **WHEN** a `virtual-sh` implementation script declares `script.interpreter: "python3"` or uses a Python shebang
- **THEN** Invowk SHALL reject the implementation with `ErrInterpreterNotAllowed` or an equivalent validation error before execution

### Requirement: Host custom checks default to portable virtual-shell execution
Invowk SHALL run host custom checks with the embedded virtual shell when the check is shell-compatible, so custom checks do not depend on fixed host shell paths.

#### Scenario: Host custom check without interpreter runs on virtual shell
- **WHEN** a root, command, or implementation-level custom check declares `script.content: "echo ok"` and omits `interpreter`
- **THEN** Invowk SHALL execute the check through the embedded mvdan/sh shell on Linux, macOS, and Windows

#### Scenario: Host custom check does not require fixed Unix shell paths
- **WHEN** a host custom check runs on a system without `/bin/sh` or `/usr/bin/sh`
- **THEN** Invowk SHALL still execute shell-compatible custom checks through the embedded mvdan/sh shell

#### Scenario: Host custom check with shell interpreter runs on virtual shell
- **WHEN** a host custom check declares `script.interpreter: "sh"` or a shell-compatible shebang
- **THEN** Invowk SHALL execute the check through the embedded mvdan/sh shell

#### Scenario: Host custom check with non-shell interpreter runs host interpreter
- **WHEN** a host custom check declares `script.interpreter: "python3"`
- **THEN** Invowk SHALL execute the check with the allowlisted `python3` interpreter resolved in the host environment

#### Scenario: Host custom check reports missing explicit interpreter
- **WHEN** a host custom check declares an allowlisted non-shell interpreter that is not available on the host
- **THEN** Invowk SHALL fail the dependency check with a diagnostic that identifies the custom check and missing interpreter

### Requirement: Container custom checks honor script interpreters inside the container
Invowk SHALL apply script-level interpreter semantics to custom checks declared inside selected container runtime dependency blocks.

#### Scenario: Container custom check without interpreter uses container shell
- **WHEN** a selected container runtime declares a custom check with no `script.interpreter` and no shebang
- **THEN** Invowk SHALL execute the check using the container runtime's default shell behavior

#### Scenario: Container custom check with shebang uses container interpreter
- **WHEN** a selected container runtime declares a custom check whose resolved script starts with `#!/usr/bin/env python3`
- **THEN** Invowk SHALL execute the check with `python3` inside the container

#### Scenario: Container custom check with explicit interpreter uses container interpreter
- **WHEN** a selected container runtime declares a custom check with `script.interpreter: "node"`
- **THEN** Invowk SHALL execute the check with `node` inside the container

#### Scenario: Container custom check reports missing explicit interpreter
- **WHEN** a selected container runtime custom check declares an interpreter that is not available inside the container
- **THEN** Invowk SHALL fail the dependency check with a diagnostic that identifies the custom check and container validation failure

### Requirement: Script files remain module-only and module-contained
Invowk SHALL preserve the existing `script.file` security boundary for implementation scripts and custom-check scripts when interpreter metadata is present.

#### Scenario: Root invowkfile cannot use script file with interpreter
- **WHEN** a root or project invowkfile declares `script: {file: "scripts/check.py", interpreter: "python3"}`
- **THEN** Invowk SHALL reject the script file because `script.file` is only allowed for invowkfiles loaded from an invowkmod

#### Scenario: Module script file cannot escape module with interpreter
- **WHEN** an invowkmod script declares `script: {file: "../outside.py", interpreter: "python3"}`
- **THEN** Invowk SHALL reject the file reference because the resolved target escapes the source invowkmod

#### Scenario: Module custom-check file cannot escape module with interpreter
- **WHEN** an invowkmod custom check declares `script: {file: "../outside-check.py", interpreter: "python3"}`
- **THEN** Invowk SHALL reject the file reference because the resolved target escapes the source invowkmod

#### Scenario: Module script file shebang is read only after containment validation
- **WHEN** an invowkmod uses `script.file` with omitted or auto interpreter
- **THEN** Invowk SHALL validate module containment before reading the file for shebang detection

### Requirement: Clean-break implementation removes runtime interpreter leftovers
Invowk SHALL remove old runtime-level interpreter implementation surfaces rather than retaining compatibility-only code.

#### Scenario: RuntimeConfig has no Interpreter field
- **WHEN** developers inspect the Go `RuntimeConfig` type
- **THEN** it SHALL NOT contain an `Interpreter` field

#### Scenario: Runtime interpreter helper APIs are removed or relocated
- **WHEN** developers inspect runtime interpreter helper methods
- **THEN** helpers that exist only to read `RuntimeConfig.Interpreter` SHALL be removed or replaced with script-level equivalents

#### Scenario: Runtime generator cannot emit interpreter
- **WHEN** developers inspect CUE generation code for runtime configs
- **THEN** it SHALL have no branch that writes `interpreter` inside a runtime block

#### Scenario: Legacy tests are replaced
- **WHEN** tests refer to runtime-level interpreter examples
- **THEN** they SHALL be removed or rewritten to assert the new script-level behavior

#### Scenario: Benchmark fixture uses current script shape only
- **WHEN** `scripts/bench-bmf.mjs` generates benchmark invowkfiles
- **THEN** every generated script SHALL use explicit object script sources and SHALL NOT generate runtime-level interpreter fields or legacy script string shapes

### Requirement: Documentation teaches only script-scoped interpreters
Invowk SHALL update user-facing and authoring documentation so interpreter examples and references describe only `script.interpreter`.

#### Scenario: README examples use script-level interpreter
- **WHEN** README examples explain explicit interpreters
- **THEN** they SHALL place `interpreter` inside `script` and SHALL NOT show runtime-level interpreter fields

#### Scenario: Website current docs use script-level interpreter
- **WHEN** current website docs explain native, virtual, container, custom checks, invowkfile schema, or advanced interpreters
- **THEN** they SHALL document `script.interpreter` and SHALL NOT document runtime-level interpreter fields as valid

#### Scenario: Portuguese current docs match current behavior
- **WHEN** Portuguese current i18n docs mention interpreters, custom checks, or invowkfile schema
- **THEN** they SHALL match the new script-level interpreter behavior

#### Scenario: Snippets use script-level interpreter
- **WHEN** website snippet definitions include interpreter examples
- **THEN** they SHALL place `interpreter` inside `script`

#### Scenario: Generated reference docs include script interpreter
- **WHEN** generated or maintained invowkfile schema reference docs list script fields
- **THEN** they SHALL include `script.interpreter` for implementations and custom checks and SHALL NOT list runtime-level `interpreter`

#### Scenario: LLM authoring guidance uses script-level interpreter
- **WHEN** agent or LLM authoring prompts, templates, docs, or generated command examples mention interpreters
- **THEN** they SHALL instruct authors to use `script.interpreter`

### Requirement: Script-scoped interpreter behavior is test covered
Invowk SHALL include automated tests that cover schema, Go model, runtime execution, dependency checks, documentation fixtures, and generated output for script-scoped interpreters.

#### Scenario: Schema sync covers script interpreter
- **WHEN** CUE schema or Go script structs change
- **THEN** schema sync tests SHALL verify `script.interpreter` is present on implementation and custom-check script structs and absent from runtime configs

#### Scenario: Parser tests cover accepted and rejected shapes
- **WHEN** invowkfile parsing tests run
- **THEN** they SHALL cover valid script-level interpreters and rejected runtime-level interpreters for native, virtual, and container runtimes

#### Scenario: Runtime tests cover native and container interpreters
- **WHEN** runtime tests run for native and container execution
- **THEN** they SHALL cover explicit script interpreters, shebang interpreters, file-backed scripts, and no-interpreter shell fallback

#### Scenario: Virtual tests cover allowed and rejected interpreters
- **WHEN** virtual runtime tests run
- **THEN** they SHALL cover default mvdan/sh execution, shell-compatible script interpreters, and rejected non-shell interpreters

#### Scenario: Dependency tests cover custom check interpreters
- **WHEN** dependency tests run
- **THEN** they SHALL cover host and container custom checks with omitted, shell-compatible, shebang, and explicit non-shell interpreters

#### Scenario: CLI tests cover cross-platform custom checks
- **WHEN** CLI testscript suites run on Linux, macOS, and Windows
- **THEN** custom-check scenarios SHALL avoid fixed host shell path assumptions and SHALL pass with the embedded virtual shell default

#### Scenario: Documentation checks cover stale runtime interpreter examples
- **WHEN** documentation verification runs
- **THEN** it SHALL fail if current docs, README, snippets, or generated references still contain valid-looking runtime-level interpreter examples

### Requirement: Virtual-lua runtime only accepts Lua script interpreters
Invowk SHALL execute `virtual-lua` runtime scripts with the embedded Lua VM and SHALL reject non-Lua interpreter selections for `virtual-lua` implementations.

#### Scenario: Virtual-lua script without interpreter uses embedded Lua VM
- **WHEN** a `virtual-lua` implementation script omits `interpreter` and has no shebang
- **THEN** Invowk SHALL execute it with the embedded Lua VM

#### Scenario: Virtual-lua script with Lua shebang uses embedded Lua VM
- **WHEN** a `virtual-lua` implementation script starts with a Lua-compatible shebang
- **THEN** Invowk SHALL execute it with the embedded Lua VM and SHALL NOT require a host Lua executable

#### Scenario: Virtual-lua script with explicit Lua interpreter uses embedded Lua VM
- **WHEN** a `virtual-lua` implementation script declares `script.interpreter: "lua"`
- **THEN** Invowk SHALL accept the script as Lua-compatible and execute it with the embedded Lua VM

#### Scenario: Virtual-lua script rejects non-Lua interpreter
- **WHEN** a `virtual-lua` implementation script declares `script.interpreter: "python3"` or uses a Python shebang
- **THEN** Invowk SHALL reject the implementation before execution

### Requirement: Runtime selectors use explicit virtual-family names
Invowk SHALL use `virtual-sh` and `virtual-lua` as user-authored runtime selectors. The family-level `virtual` config namespace SHALL NOT be a runtime selector.

#### Scenario: Config virtual namespace is not a runtime selector
- **WHEN** config declares `virtual.utilities.enabled`
- **THEN** `virtual` SHALL be treated only as the virtual-family config namespace

#### Scenario: Generated output uses explicit virtual-family names
- **WHEN** Invowk generates CUE, dry-run output, list output, docs snippets, or example invowkfiles
- **THEN** it SHALL use `virtual-sh` or `virtual-lua`

### Requirement: Runtime lists remain the command shape
Invowk SHALL keep the existing `runtimes` and `platforms` list model for implementations. This change SHALL NOT introduce singular `runtime` or `platform` fields.

#### Scenario: Multiple virtual runtimes remain selectable
- **WHEN** an implementation declares both `virtual-sh` and `virtual-lua` in its runtime list
- **THEN** Invowk SHALL preserve existing runtime-selection semantics for list-ordered runtime configs

#### Scenario: Singular runtime field is not introduced
- **WHEN** a user declares `runtime: "virtual-lua"` instead of `runtimes`
- **THEN** Invowk SHALL reject the unsupported field according to the current schema model
