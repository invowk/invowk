## ADDED Requirements

### Requirement: Virtual runtimes define a Go-native safety harness
Invowk SHALL define every `virtual-*` runtime as a Go-native safety harness, not as a kernel-level jail. Invowk SHALL enforce path validation and execution gating for VM-controlled operations and SHALL document that launched host binaries run as normal host processes.

#### Scenario: Host binary boundary is explicit
- **WHEN** a `virtual-sh` or `virtual-lua` runtime launches an explicitly allowed host binary
- **THEN** Invowk SHALL NOT claim that the launched process is filesystem-sandboxed by the virtual runtime
- **THEN** Invowk SHALL document that `container` is required for process-level isolation

#### Scenario: Go-native operations remain inside the harness
- **WHEN** a virtual script uses VM-controlled file I/O, shell redirection, Lua file I/O, or a u-root built-in
- **THEN** Invowk SHALL route the operation through the virtual runtime safety harness before touching the host filesystem

### Requirement: Virtual config namespace controls shared utility commands
Invowk SHALL expose shared virtual-family configuration under `virtual`. The `virtual` config namespace SHALL NOT make `virtual` a valid runtime selector.

#### Scenario: Family-level utility config defaults to enabled
- **WHEN** generated config or effective config includes the virtual-family settings
- **THEN** Invowk SHALL use `virtual.utilities.enabled: true` by default
- **THEN** Invowk SHALL NOT emit the legacy `virtual_shell` config namespace

#### Scenario: Legacy virtual shell config namespace is rejected
- **WHEN** config declares `virtual_shell`
- **THEN** config parsing or validation SHALL reject the field

#### Scenario: Virtual config namespace does not revive the legacy runtime name
- **WHEN** config declares `virtual.utilities.enabled`
- **THEN** `default_runtime: "virtual"` SHALL still be rejected as an invalid runtime selector

#### Scenario: Disabling utilities affects both virtual interpreters
- **WHEN** config sets `virtual.utilities.enabled: false`
- **THEN** `virtual-sh` SHALL NOT resolve Invowk-provided external-style utility commands from the shared virtual utility set
- **THEN** `virtual-lua` SHALL NOT expose those utility commands through `invowk.cmd` or `invowk.capture`
- **THEN** host binary execution SHALL remain governed by `allowed_binaries` and `binary_lookup_mode`

### Requirement: Path sanitization for virtual runtimes
All `virtual-*` runtimes SHALL enforce shared path sanitization for VM-controlled filesystem operations. In restricted mode, access SHALL be allowed only for implicit safe roots, standard allowed anchors, or selected-platform `virtual.filesystem.paths` mappings. In full mode, VM-controlled filesystem operations SHALL be allowed to access normalized host filesystem paths after resolver checks.

#### Scenario: Relative workdir access is allowed
- **WHEN** a `virtual-sh` or `virtual-lua` script reads `./manifest.json` inside the effective work directory
- **THEN** Invowk SHALL allow the operation after resolving and validating the final host path

#### Scenario: Module file access is allowed
- **WHEN** a virtual script loaded from an invowkmod reads a file inside the source module directory
- **THEN** Invowk SHALL allow the operation after resolving and validating the final host path

#### Scenario: Unauthorized absolute path is blocked
- **WHEN** a `virtual-sh` script attempts to read `/etc/passwd` in restricted mode without a matching anchor or selected-platform `virtual.filesystem.paths` mapping
- **THEN** Invowk SHALL fail the operation with a permission-denied diagnostic from the shared path validator

#### Scenario: Traversal out of an allowed root is blocked
- **WHEN** a virtual script uses `../`, symlinks, Windows drive syntax, UNC paths, or mixed separators to escape an allowed root
- **THEN** Invowk SHALL reject the resolved path before performing the filesystem operation

#### Scenario: Shell redirection is path checked
- **WHEN** a `virtual-sh` script redirects output to `/tmp/../etc/hosts`
- **THEN** Invowk SHALL validate the resolved target path and block the write if it escapes the allowed roots

#### Scenario: Lua file I/O is path checked
- **WHEN** a `virtual-lua` script calls `io.open` for a host path
- **THEN** Invowk SHALL validate the resolved path with the same path validator used by `virtual-sh`

#### Scenario: u-root built-ins are path checked
- **WHEN** a virtual script invokes a u-root built-in such as `cp`, `rm`, `cat`, or `tee`
- **THEN** Invowk SHALL ensure the built-in uses path-validated filesystem operations

### Requirement: Host binary execution is denied by default
All `virtual-*` runtimes SHALL deny host binary execution unless the selected runtime configuration explicitly allows it through `allowed_binaries`.

#### Scenario: Empty allowed binaries denies host binary
- **WHEN** a virtual runtime omits `allowed_binaries` or sets `allowed_binaries: []`
- **THEN** Invowk SHALL deny attempts to execute host binaries such as `git`, `curl`, `python`, or `node`

#### Scenario: Named allowed binary may execute
- **WHEN** a virtual runtime declares `allowed_binaries: ["git"]`
- **THEN** Invowk SHALL allow `git` to be resolved according to `binary_lookup_mode`
- **THEN** Invowk SHALL continue to deny non-listed host binaries

#### Scenario: Wildcard explicitly allows all host binaries
- **WHEN** a virtual runtime declares `allowed_binaries: ["*"]`
- **THEN** Invowk SHALL allow all host binaries to resolve according to `binary_lookup_mode`
- **THEN** dry-run and documentation SHALL identify that this opts out of host-binary gating

#### Scenario: u-root built-ins do not require allowlisting
- **WHEN** a virtual script invokes a u-root built-in
- **THEN** Invowk SHALL allow the built-in even when `allowed_binaries` is empty
- **THEN** the built-in SHALL remain subject to path validation

#### Scenario: Absolute executable path is exact
- **WHEN** `allowed_binaries` contains an absolute executable path
- **THEN** Invowk SHALL allow only that exact executable path
- **THEN** `binary_lookup_mode` SHALL NOT search alternate directories for that entry

### Requirement: Binary lookup mode controls allowed binary resolution
Invowk SHALL support `binary_lookup_mode` for virtual runtimes with allowed values `"host"` and `"strict"`. The default SHALL be `"host"`.

#### Scenario: Host lookup mode uses full effective PATH
- **WHEN** a virtual runtime allows `git` and omits `binary_lookup_mode`
- **THEN** Invowk SHALL use `"host"` lookup mode
- **THEN** Invowk SHALL resolve `git` from the effective host `PATH` visible to the runtime

#### Scenario: Strict lookup mode uses hardcoded system paths
- **WHEN** a virtual runtime declares `binary_lookup_mode: "strict"`
- **THEN** Invowk SHALL resolve bare allowed binary names only from hardcoded platform system paths
- **THEN** Invowk SHALL NOT use user-writable PATH entries such as home-bin directories or Windows AppData shims

#### Scenario: Invalid lookup mode is rejected by schema or validation
- **WHEN** a virtual runtime declares `binary_lookup_mode: "custom"`
- **THEN** Invowk SHALL reject the configuration before execution

#### Scenario: Resolved host binary path is exposed
- **WHEN** a virtual runtime resolves and launches an allowed host binary
- **THEN** `virtual-sh` SHALL expose the resolved executable path as `INVOWK_STATE_BIN_PATH`
- **THEN** `virtual-lua` SHALL expose the resolved executable path as `invowk.state.bin_path`

### Requirement: Virtual runtime environment is controlled
Virtual runtimes SHALL pass host binaries the same effective environment constructed for the virtual runtime after Invowk environment inheritance, filtering, and internal variable injection. User-provided environment values SHALL NOT override Invowk-reserved `INVOWK_` state variables.

#### Scenario: Inherited environment obeys existing env policy
- **WHEN** a virtual runtime uses `env_inherit_mode: "allow"`
- **THEN** host binaries launched from that runtime SHALL only receive the allowed host variables plus Invowk-injected variables and configured command environment

#### Scenario: Reserved state variables cannot be spoofed
- **WHEN** user configuration attempts to set `INVOWK_STATE_BIN_PATH`
- **THEN** Invowk SHALL keep its own generated state value authoritative
