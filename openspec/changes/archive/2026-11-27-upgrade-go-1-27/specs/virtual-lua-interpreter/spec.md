# virtual-lua-interpreter Specification (Delta)

## MODIFIED Requirements

### Requirement: Built-in Lua runtime
Invowk SHALL provide `virtual-lua` as a built-in virtual runtime using `github.com/invowk/golua` (the maintained fork of `arnodel/golua`, patched for Go 1.27's `//go:linkname` allowlist removal via `hash/maphash.Comparable`). The runtime SHALL execute Lua scripts in-process with a fresh Lua VM for each command execution.

#### Scenario: Running Lua through virtual-lua
- **WHEN** a command implementation includes a runtime config with `name: "virtual-lua"`
- **THEN** Invowk SHALL execute the script with the embedded Lua runtime

#### Scenario: Fresh VM per command
- **WHEN** a command dependency invokes another Lua-backed command before the selected command
- **THEN** each command execution SHALL receive a separate Lua VM with no shared Lua globals

#### Scenario: Simple dependency checks do not create command VMs
- **WHEN** `depends_on.tools`, capabilities, files, directories, or environment checks run without invoking a command or custom check script
- **THEN** Invowk SHALL NOT create a Lua VM for those simple checks
