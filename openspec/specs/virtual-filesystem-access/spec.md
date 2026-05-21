# virtual-filesystem-access Specification

## Purpose
TBD - created by archiving change reshape-virtual-filesystem-config. Update Purpose after archive.
## Requirements
### Requirement: Platform virtual filesystem config
Invowk SHALL expose virtual-runtime filesystem configuration under each implementation platform at `platforms[].virtual.filesystem`.

#### Scenario: Platform declares restricted virtual filesystem paths
- **WHEN** an implementation platform declares `virtual: {filesystem: {access: "restricted", paths: {DATA: "@data/tool"}}}`
- **THEN** Invowk SHALL parse the platform successfully
- **THEN** the selected virtual runtime SHALL expose `DATA` as a logical virtual filesystem path for that platform

#### Scenario: Omitted virtual filesystem config is restricted
- **WHEN** an implementation platform omits `virtual` or omits `virtual.filesystem.access`
- **THEN** Invowk SHALL treat the selected virtual runtime filesystem access as `"restricted"`
- **THEN** no additional named paths SHALL be exposed beyond implicit anchors

#### Scenario: Virtual namespace is not a runtime selector
- **WHEN** an implementation platform declares `virtual.filesystem`
- **THEN** Invowk SHALL NOT make `"virtual"` a valid `runtimes[].name` value

### Requirement: Virtual filesystem access modes
Invowk SHALL support exactly two virtual filesystem access modes: `"restricted"` and `"full"`.

#### Scenario: Restricted access limits virtual file I/O
- **WHEN** the selected platform uses `virtual.filesystem.access: "restricted"`
- **THEN** VM-controlled virtual filesystem operations SHALL be allowed only under implicit safe roots and `virtual.filesystem.paths` roots

#### Scenario: Full access allows host filesystem access
- **WHEN** the selected platform uses `virtual.filesystem.access: "full"`
- **THEN** VM-controlled virtual filesystem operations SHALL be allowed to access host filesystem paths after normal path normalization and resolver checks
- **THEN** Invowk SHALL NOT describe this mode as kernel-level isolation

#### Scenario: Invalid access mode is rejected
- **WHEN** an implementation platform declares `virtual.filesystem.access: "custom"`
- **THEN** CUE parsing or Go validation SHALL reject the configuration before execution

#### Scenario: Full access is visible in dry run
- **WHEN** a dry-run plan selects a virtual runtime whose selected platform uses `virtual.filesystem.access: "full"`
- **THEN** dry-run output SHALL show that virtual filesystem access is full
- **THEN** dry-run output SHALL continue to show selected runtime host-binary policy separately

### Requirement: Virtual filesystem paths are named bridge handles
Invowk SHALL expose `virtual.filesystem.paths` as a map of logical path names to platform-local path strings. These paths SHALL be named bridge handles for scripts and SHALL define allowed roots only when filesystem access is `"restricted"`.

#### Scenario: Path names become shell environment handles
- **WHEN** a selected platform declares `virtual.filesystem.paths: {DATA: "@data/tool"}`
- **THEN** `virtual-sh` SHALL inject `INVOWK_PATH_DATA` with the resolved platform path

#### Scenario: Path names become Lua bridge handles
- **WHEN** a selected platform declares `virtual.filesystem.paths: {DATA: "@data/tool"}`
- **THEN** `virtual-lua` SHALL allow `invowk.path("DATA/out.json")` to resolve under the `DATA` path

#### Scenario: Path names must be safe environment suffixes
- **WHEN** a `virtual.filesystem.paths` key contains lowercase letters, spaces, punctuation, or starts with a digit
- **THEN** Invowk SHALL reject the key before execution

#### Scenario: Path values are platform-local strings
- **WHEN** a platform declares a `virtual.filesystem.paths` entry
- **THEN** the entry value SHALL be a non-empty string path for that platform
- **THEN** the value SHALL NOT be a nested `linux`/`macos`/`windows` platform-keyed object

#### Scenario: Paths are handles in full access mode
- **WHEN** a selected platform uses `virtual.filesystem.access: "full"` and declares `virtual.filesystem.paths`
- **THEN** the declared paths SHALL be exposed through `INVOWK_PATH_*` and `invowk.path`
- **THEN** the declared paths SHALL NOT be treated as the complete filesystem permission boundary

### Requirement: Generated CUE uses virtual filesystem paths
Invowk SHALL generate virtual filesystem path handles under the selected platform's `virtual.filesystem.paths` block.

#### Scenario: Generated CUE uses virtual filesystem paths
- **WHEN** Invowk generates CUE for an implementation with virtual filesystem path handles
- **THEN** generated CUE SHALL emit `platforms[].virtual.filesystem.paths`

### Requirement: Host binary policy remains runtime scoped
Invowk SHALL keep virtual host binary execution policy on selected virtual runtime configs through `runtimes[].allowed_binaries` and `runtimes[].binary_lookup_mode`.

#### Scenario: Runtime declares host binary policy
- **WHEN** an implementation declares `runtimes: [{name: "virtual-lua", allowed_binaries: ["git"], binary_lookup_mode: "strict"}]`
- **THEN** the selected virtual runtime SHALL allow only configured host binaries according to the configured lookup mode

#### Scenario: Platform virtual filesystem does not grant host binaries
- **WHEN** a selected platform declares `virtual.filesystem.access: "full"`
- **THEN** host binary execution SHALL still be denied unless the selected runtime config explicitly allows it through `allowed_binaries`

#### Scenario: Binary policy is not accepted under platform virtual filesystem
- **WHEN** a platform declares `virtual.filesystem.allowed_binaries` or `virtual.allowed_binaries`
- **THEN** CUE parsing SHALL reject the field as unknown

