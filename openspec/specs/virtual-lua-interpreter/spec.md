# virtual-lua-interpreter Specification

## Purpose
TBD - created by archiving change rename-virtual-and-add-lua. Update Purpose after archive.
## Requirements
### Requirement: Built-in Lua runtime
Invowk SHALL provide `virtual-lua` as a built-in virtual runtime using `github.com/arnodel/golua`. The runtime SHALL execute Lua scripts in-process with a fresh Lua VM for each command execution.

#### Scenario: Running Lua through virtual-lua
- **WHEN** a command implementation includes a runtime config with `name: "virtual-lua"`
- **THEN** Invowk SHALL execute the script with the embedded Lua runtime

#### Scenario: Fresh VM per command
- **WHEN** a command dependency invokes another Lua-backed command before the selected command
- **THEN** each command execution SHALL receive a separate Lua VM with no shared Lua globals

#### Scenario: Simple dependency checks do not create command VMs
- **WHEN** `depends_on.tools`, capabilities, files, directories, or environment checks run without invoking a command or custom check script
- **THEN** Invowk SHALL NOT create a Lua VM for those simple checks

### Requirement: Lua interpreter validation
Invowk SHALL accept Lua-compatible interpreter intent for `virtual-lua` and SHALL reject non-Lua interpreter intent before execution.

#### Scenario: Omitted Lua interpreter uses embedded VM
- **WHEN** a `virtual-lua` implementation omits `script.interpreter` and has no shebang
- **THEN** Invowk SHALL execute it with the embedded Lua VM

#### Scenario: Lua shebang uses embedded VM
- **WHEN** a `virtual-lua` script starts with a Lua-compatible shebang
- **THEN** Invowk SHALL execute it with the embedded Lua VM and SHALL NOT require a host `lua` binary

#### Scenario: Explicit Lua interpreter uses embedded VM
- **WHEN** a `virtual-lua` implementation declares `script.interpreter: "lua"`
- **THEN** Invowk SHALL accept the implementation and execute it with the embedded Lua VM

#### Scenario: Non-Lua interpreter is rejected
- **WHEN** a `virtual-lua` implementation declares `script.interpreter: "python3"` or uses a Python shebang
- **THEN** Invowk SHALL reject the implementation before execution

### Requirement: Lua bridge API
Invowk SHALL inject a read-only global `invowk` table into every `virtual-lua` VM. The bridge SHALL expose path resolution, environment, execution state, and command helpers.

#### Scenario: Lua resolves an anchor
- **WHEN** a Lua script calls `invowk.path("@config")`
- **THEN** Invowk SHALL return the resolved and sanitized `@config` path for the active platform

#### Scenario: Lua reads environment
- **WHEN** a Lua script reads `invowk.env.PATH`
- **THEN** Invowk SHALL return the effective runtime environment value or nil when absent
- **THEN** the Lua script SHALL NOT be able to mutate the underlying runtime environment through `invowk.env`

#### Scenario: Lua streams command output
- **WHEN** a Lua script calls `invowk.cmd.ls("-la")`
- **THEN** Invowk SHALL run the u-root `ls` implementation or an explicitly allowed host binary named `ls`
- **THEN** stdout and stderr SHALL stream to the command execution streams
- **THEN** the Lua call SHALL return an exit code or Lua error according to the bridge contract

#### Scenario: Lua captures command output
- **WHEN** a Lua script calls `invowk.capture.ls("-la")`
- **THEN** Invowk SHALL capture stdout, stderr, and exit code and return them to Lua

#### Scenario: Lua state exposes binary metadata
- **WHEN** a Lua script launches an allowed host binary through the bridge
- **THEN** Invowk SHALL set `invowk.state.bin_path` to the resolved executable path

#### Scenario: Disabled virtual utilities are hidden from Lua command helpers
- **WHEN** config sets `virtual.utilities.enabled: false`
- **THEN** `invowk.cmd` and `invowk.capture` SHALL NOT expose Invowk-provided utility commands
- **THEN** explicitly allowed host binaries SHALL remain callable through the bridge according to `allowed_binaries`

### Requirement: Lua arguments and I/O streams
Invowk SHALL expose command positional arguments and execution streams to Lua in a deterministic way.

#### Scenario: Positional arguments are available
- **WHEN** a `virtual-lua` command runs with positional arguments
- **THEN** Invowk SHALL populate Lua's `arg` table and chunk varargs with the command arguments in order

#### Scenario: Standard input and output are connected
- **WHEN** a Lua script reads from stdin or writes to stdout/stderr
- **THEN** Invowk SHALL use the `ExecutionContext` streams for the selected command

#### Scenario: Interactive mode attaches streams
- **WHEN** a `virtual-lua` command runs with interactive execution enabled
- **THEN** Invowk SHALL attach the script's stdin/stdout/stderr to the interactive execution streams
- **THEN** Invowk SHALL NOT open an implicit Lua REPL

### Requirement: Lua standard library is sandbox-shaped
Invowk SHALL expose useful Lua standard library functionality while disabling escape-oriented APIs.

#### Scenario: Safe pure libraries are available
- **WHEN** a Lua script uses `string`, `table`, `math`, or `utf8`
- **THEN** Invowk SHALL make those libraries available

#### Scenario: File I/O is path validated
- **WHEN** a Lua script uses `io.open` or equivalent file APIs
- **THEN** Invowk SHALL route file access through the shared virtual path validator

#### Scenario: Environment lookup is controlled
- **WHEN** a Lua script calls `os.getenv`
- **THEN** Invowk SHALL read from the same effective environment exposed by `invowk.env`

#### Scenario: Escape APIs are unavailable
- **WHEN** a Lua script tries to use `os.execute`, `io.popen`, unrestricted `package.loadlib`, `debug`, or dynamic Go package import through `golib`
- **THEN** Invowk SHALL reject or omit that API in `virtual-lua`

### Requirement: Lua require is module-contained
Invowk SHALL support `require` for Lua files inside the inline script context or the source invowkmod while preventing module escape and native shared-library loading.

#### Scenario: Module-local require succeeds
- **WHEN** a `virtual-lua` script in an invowkmod calls `require("helpers.format")`
- **THEN** Invowk SHALL load the matching Lua source from inside the same invowkmod when present

#### Scenario: Require cannot escape module
- **WHEN** a Lua script tries to require a file outside the source invowkmod using relative traversal or absolute paths
- **THEN** Invowk SHALL reject the load before reading or executing the target

#### Scenario: Native Lua module loading is blocked
- **WHEN** a Lua script tries to load a native shared library through `require` or `package.loadlib`
- **THEN** Invowk SHALL reject the operation in `virtual-lua`

### Requirement: Lua resource controls
Invowk SHALL support optional Lua CPU and memory limits for `virtual-lua` runtime configs.

#### Scenario: CPU limit is enforced when configured
- **WHEN** a `virtual-lua` runtime declares `cpu_limit`
- **THEN** Invowk SHALL configure golua CPU accounting for that command execution
- **THEN** Invowk SHALL terminate the Lua execution with an actionable diagnostic when the limit is exceeded

#### Scenario: Memory limit is enforced when configured
- **WHEN** a `virtual-lua` runtime declares `memory_limit`
- **THEN** Invowk SHALL configure golua memory accounting for that command execution
- **THEN** Invowk SHALL terminate the Lua execution with an actionable diagnostic when the limit is exceeded

#### Scenario: Omitted limits do not add hidden caps
- **WHEN** a `virtual-lua` runtime omits `cpu_limit` and `memory_limit`
- **THEN** Invowk SHALL NOT impose undocumented Lua resource caps beyond normal process limits and command cancellation

