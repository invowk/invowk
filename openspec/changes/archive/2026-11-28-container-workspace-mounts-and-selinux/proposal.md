# Proposal: Container Workspace Mounts, Relative Volumes, and SELinux Policy

## Why

Project-local modules such as `cst.invowkmod/` (which lives at `<repo>/cst.invowkmod/` for module identity and `includes` referencing while setting `workdir: ".."`) cannot currently use Invowk's `container` runtime: `ContainerRuntime` unconditionally mounts `invowkDir` (`cst.invowkmod/`) at `/workspace`, passes the unmounted host checkout path directly as `-w`, does not resolve relative `volumes` against `invowkDir`, creates inline temp scripts inside `invowkDir`, and unconditionally appends `:z` to every mount on SELinux hosts (mutating `user_home_t` xattrs on developer checkouts).

## What Changes

- Endorse `<repo>/<m>.invowkmod/` + `workdir: ".."` under a hardened **3-tier trust model** with a **unified `<cwd>` ceiling**:
  - **Layer 1 (vendored `invowk_modules/`, remote, and `~/.invowk/cmds/` modules)**: strict module-root containment for `workspace_mount` and relative `volumes`.
  - **Layer 2 (`<cwd>/invowkfile.cue` standalone AND `<cwd>/<m>.invowkmod`)**: keep `/workspace` defaulting to `invowkDir` (non-breaking), allow `workspace_mount: { root: "workdir" }` and relative `volumes` only when resolved physically inside `<cwd>` (blocking escapes above `<cwd>`, including `workdir: ".."` in standalone root `invowkfile.cue` or `../..` in `<cwd>/<m>.invowkmod`).
  - **Layer 3 (`includes` in `config.cue`, outside `cwd`)**: disallow self-granted parent escape; require explicit operator-configured absolute `workspace_root` on `#IncludeEntry` in `internal/config/config_schema.cue`.
- **BREAKING**: Block absolute host bind-mounts in `volumes` at runtime for **all** `*.invowkmod` modules (portable/contained unit policy; named volumes like `cache:/cache` remain allowed), while preserving absolute host bind-mounts on standalone `<cwd>/invowkfile.cue` with `invowk audit` warnings/findings and `--ivk-dry-run` mount visibility.
- Add `workspace_mount?: { enabled?: bool, root?: "invowkfile" | "workdir", readonly?: bool, selinux_label?: "auto" | "shared" | "private" | "none" }` on `#RuntimeConfigContainerBase` (`pkg/invowkfile/invowkfile_schema.cue`).
- Fix `getContainerWorkDir` (`internal/runtime/container_provision.go`) to validate/map paths strictly against mounted container targets instead of passing unmounted host paths as `-w`, and decouple inline interpreter temp-script creation from `invowkDir` (mounting a temp file `:ro`) so `readonly: true` and `root: "workdir"` work without polluting the checkout.
- Resolve explicit relative host paths (`./...`, `../...`) in `volumes` against `invowkDir`, preserving Docker/Podman named volumes.
- Add operator-only SELinux confinement disablement (`--security-opt label=disable` via `config.cue` under `container`, never from `invowkfile.cue`), update `container-selinux-volume-labels`, add container mount `invowk audit` rules and `--ivk-dry-run` mount reporting, and update the formal Alloy models (`ModulePathContainment.als`, `VirtualPathHarness.als`, `module-path-containment-model` spec).

## Migration notes

- Any `*.invowkmod` module whose `container` runtime declared `volumes` with an absolute host path (e.g., `"/tmp:/tmp:ro"`) will now be rejected at container execution time. Use named volumes (`myvol:/data`), relative paths inside the module/workspace (`./data:/data`), or move host-specific absolute mounts into a standalone root `invowkfile.cue`.
- Standalone `<cwd>/invowkfile.cue` files that previously mounted directories **above** `<cwd>` via `workdir: ".."` or `volumes: ["../sibling:/mnt"]` will now be rejected by the unified Layer 2 `<cwd>` ceiling unless moved to an explicit CLI `--ivk-workdir` or an absolute host volume in standalone `invowkfile.cue`.

## Capabilities

### New Capabilities

- `container-workspace-mounts`: Defines `workspace_mount`, 3-tier containment for `workspace_mount` and `volumes`, runtime blocking of absolute host bind-mounts in `*.invowkmod`, relative volume path resolution, decoupled temp scripts for read-only/parent mounts, `--ivk-dry-run` mount display, and container mount audit findings.

### Modified Capabilities

- `container-selinux-volume-labels`: Adds per-mount `workspace_mount.selinux_label` (`auto` | `shared` | `private` | `none`) behavior and operator-config-only `--security-opt label=disable` support.
- `module-path-containment-model`: Updates `Allowed-location predicate set` and container workdir/mount containment requirements in `ModulePathContainment.als` / `VirtualPathHarness.als` to formalize the 3-tier trust boundaries and `<cwd>` ceiling.

## Impact

- Schemas & Types: `pkg/invowkfile/invowkfile_schema.cue`, `pkg/invowkfile/runtime.go`, `internal/config/config_schema.cue`, `internal/config/types.go`.
- Container & Runtime: `internal/runtime/container_exec.go`, `internal/runtime/container_provision.go`, `internal/container/engine_base.go`, `internal/container/engine_base_format.go`, `internal/container/docker.go`, `internal/container/podman.go`.
- CLI, Audit & Formal: `cmd/invowk/cmd_dryrun.go`, `internal/audit/` (new container mount checker), `formal/alloy/ModulePathContainment.als`, `formal/alloy/VirtualPathHarness.als`, `formal/manifest.toml`.
- Tests & Docs: `internal/runtime/*container*`, `internal/container/*`, `tests/cli/`, `make formal` golden/property updates, and `website/` docs.
