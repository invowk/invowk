## RENAMED Requirements

### Requirement: Virtual runtime only accepts shell-compatible script interpreters
FROM: Virtual runtime only accepts shell-compatible script interpreters
TO: Virtual-sh runtime only accepts shell-compatible script interpreters

## MODIFIED Requirements

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

## ADDED Requirements

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
