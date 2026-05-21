## ADDED Requirements

### Requirement: Lua audit discovery
Invowk audit SHALL discover Lua content associated with invowk modules and `virtual-lua` command execution.

#### Scenario: Inline Lua script is audited
- **WHEN** an invowkfile contains an inline `virtual-lua` implementation script
- **THEN** `invowk audit` SHALL include that Lua source in deterministic audit analysis

#### Scenario: Lua script file is audited
- **WHEN** an invowkmod implementation uses `script.file` with `virtual-lua`
- **THEN** `invowk audit` SHALL include the resolved module-contained Lua file in deterministic audit analysis

#### Scenario: Required Lua file is audited
- **WHEN** a `virtual-lua` script can load a module-local Lua file through `require`
- **THEN** `invowk audit` SHALL include that required Lua file in deterministic audit analysis

#### Scenario: Self-contained module Lua file is audited
- **WHEN** an invowkmod contains a `.lua` file that is not directly referenced by an implementation but is self-contained module content
- **THEN** `invowk audit` SHALL include the file or report why it was excluded

### Requirement: Deterministic Lua risk checks
Invowk audit SHALL perform deterministic Lua checks for APIs and patterns that can weaken the virtual safety harness or expose sensitive data.

#### Scenario: Host process escape patterns are flagged
- **WHEN** Lua code references disabled or risky APIs such as `os.execute`, `io.popen`, `package.loadlib`, `debug`, or dynamic Go package import
- **THEN** `invowk audit` SHALL flag the finding with file/source location when available

#### Scenario: Broad host binary opt-out is flagged
- **WHEN** a `virtual-lua` runtime declares `allowed_binaries: ["*"]`
- **THEN** `invowk audit` SHALL flag that the command opts out of host-binary gating

#### Scenario: Sensitive environment reads are flagged
- **WHEN** Lua code reads environment variables whose names indicate credentials, tokens, passwords, or cloud secrets
- **THEN** `invowk audit` SHALL flag the access for review

#### Scenario: Network-capable allowed binaries are correlated
- **WHEN** Lua code can launch allowed host binaries such as `curl`, `wget`, `ssh`, `scp`, `git`, or language package managers
- **THEN** `invowk audit` SHALL correlate those binaries with environment and path access findings

### Requirement: LLM audit guidance covers Lua
Invowk's LLM-assisted audit instructions SHALL include Lua-specific review guidance and SHALL direct agents to use focused subagents when possible for non-trivial Lua module trees.

#### Scenario: Lua subagent guidance is present
- **WHEN** LLM-assisted audit reviews a module with multiple Lua files or complex Lua bridge usage
- **THEN** the audit instructions SHALL ask for focused Lua analysis and subagent delegation when available

#### Scenario: Lua audit explains the safety boundary
- **WHEN** LLM-assisted audit reports virtual Lua risks
- **THEN** the report SHALL distinguish Go-native sandboxed operations from explicitly allowed host binaries that run outside the Go-level path sandbox

#### Scenario: Lua audit includes bridge semantics
- **WHEN** LLM-assisted audit reviews Lua code using `invowk.path`, `invowk.env`, `invowk.cmd`, or `invowk.capture`
- **THEN** the instructions SHALL tell the reviewer how those bridge APIs map to anchors, environment, host binary gating, and capture/stream behavior
