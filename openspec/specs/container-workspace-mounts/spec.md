# container-workspace-mounts Specification

## Purpose

Defines `workspace_mount` configuration for container runtimes, 3-tier workspace and volume mount containment, runtime blocking of absolute host bind-mounts in `*.invowkmod`, relative `volumes` resolution, `getContainerWorkDir` path mapping, decoupled `script.file`/temp scripts, dry-run mount visibility, and container mount audit rules.

## Requirements

### Requirement: #RuntimeConfigContainerBase supports declarative workspace_mount
`#RuntimeConfigContainerBase` SHALL admit an optional `workspace_mount` block (`enabled?: bool`, `root?: "invowkfile" | "workdir"`, `readonly?: bool`, `selinux_label?: "auto" | "shared" | "private" | "none"`), defaulting `/workspace` to `invowkDir` (`root: "invowkfile"`).

#### Scenario: Module with workdir ".." mounts repo root at /workspace when opted in
- **WHEN** `<repo>/cst.invowkmod/invowkfile.cue` sets `workdir: ".."` and `workspace_mount: { root: "workdir", readonly: true }`
- **THEN** container execution SHALL mount `<repo>` read-only at `/workspace` and pass `-w /workspace`

#### Scenario: Workspace mount can be disabled completely
- **WHEN** a container runtime sets `workspace_mount: { enabled: false }`
- **THEN** `/workspace` SHALL NOT be bind-mounted automatically

### Requirement: Layer 1 and Layer 2 enforce module and cwd mount ceilings
Vendored, remote, and global (`~/.invowk/cmds/`) modules (Layer 1) SHALL NOT mount outside the declaring module's physical root. Local `cwd` sources (Layer 2: `<cwd>/invowkfile.cue` and `<cwd>/<m>.invowkmod`) SHALL allow `workspace_mount.root: "workdir"` and relative `volumes` only inside `<cwd>` (except CLI `--ivk-workdir`).

#### Scenario: Module cannot escape cwd via workdir "../.."
- **WHEN** `<cwd>/cst.invowkmod` declares `workdir: "../.."` and `workspace_mount: { root: "workdir" }`
- **THEN** container execution SHALL fail closed before starting the container

#### Scenario: Standalone invowkfile cannot escape cwd via workspace_mount
- **WHEN** `<cwd>/invowkfile.cue` declares `workdir: ".."` and `workspace_mount: { root: "workdir" }` without CLI `--ivk-workdir`
- **THEN** container execution SHALL fail closed before starting the container

### Requirement: Layer 3 included modules require explicit operator workspace_root
Modules loaded via `includes` (`config.cue`) outside `cwd` SHALL NOT mount outside the module root unless the operator configures an explicit absolute `workspace_root` on that `#IncludeEntry`, which bounds both `workspace_mount.root: "workdir"` and relative `volumes`.

#### Scenario: Included module mounts parent directory only with operator workspace_root
- **WHEN** `cst.invowkmod` is loaded via `includes` with `--ivk-no-cwd` and `workspace_root` is omitted
- **THEN** `workspace_mount.root: "workdir"` pointing to `..` SHALL fail closed; when the operator sets `workspace_root: "/abs/repo"` in `config.cue`, it SHALL succeed

### Requirement: *.invowkmod modules cannot declare absolute host bind-mounts in volumes
All `*.invowkmod` modules SHALL be rejected at container execution time if any `volumes` entry specifies an absolute host bind-mount. Named volumes (`name:/target`) and tier-contained relative mounts SHALL remain allowed. Standalone `<cwd>/invowkfile.cue` files MAY keep declaring absolute host bind-mounts.

#### Scenario: Absolute host volume in module is rejected at runtime
- **WHEN** a command in `cst.invowkmod` declares `volumes: ["/etc:/host-etc:ro"]`
- **THEN** container execution SHALL fail before invoking the container engine

#### Scenario: Named volume in module remains allowed
- **WHEN** a command in `cst.invowkmod` declares `volumes: ["build-cache:/cache"]`
- **THEN** container execution SHALL pass the named volume to the container engine

### Requirement: Relative volumes resolve against invowkDir and container scripts decouple from /workspace
Explicit relative host paths in `volumes` (`./...`, `../...`) SHALL resolve against `invowkDir` (preserving named volumes). `getContainerWorkDir` SHALL never forward an unmounted host path verbatim as `-w`. Container `script.file` and inline interpreter scripts SHALL resolve against `mountHostRoot` or bind-mount read-only from an isolated host path rather than writing inside `invowkDir`.

#### Scenario: Read-only workspace mount runs inline Python and module script.file cleanly
- **WHEN** a container command uses `#!/usr/bin/env python3` inline or `script: { file: "scripts/run.py" }` with `workspace_mount: { root: "workdir", readonly: true }`
- **THEN** Invowk SHALL create no temporary file in the repository checkout and SHALL execute the script cleanly inside `/workspace` or an isolated `:ro` mount

### Requirement: Dry-run and invowk audit surface container mount configuration
`--ivk-dry-run` SHALL display resolved container workspace mount settings and `volumes`, and `invowk audit` SHALL report findings for parent workspace mounts, sensitive host mounts in standalone invowkfiles, and SELinux relabel opt-outs.

#### Scenario: Dry-run shows workspace_mount and volumes
- **WHEN** `invowk cmd --ivk-dry-run` inspects a container command with `workspace_mount: { root: "workdir", readonly: true, selinux_label: "none" }`
- **THEN** dry-run output SHALL display the resolved `/workspace` host root, `readonly` mode, and `selinux_label: none`
