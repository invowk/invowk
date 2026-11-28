# Design: Container Workspace Mounts, Relative Volumes, and SELinux Policy

## Context

Current behavior in `main`:
- `internal/runtime/container_exec.go:87,153-157` sets `invowkDir := filepath.Dir(string(ctx.Invowkfile.FilePath))` and unconditionally appends `invowkDir + ":/workspace"` to `volumes`, ignoring `workdir`.
- `internal/runtime/container_provision.go:375-402` (`getContainerWorkDir`) maps paths inside `invowkDir` to `/workspace/...`, and for any host absolute `effectiveWorkDir` outside `invowkDir` returns `string(effectiveWorkDir)` verbatim (`container_provision.go:394`) — passing an unmounted host path directly to `-w`.
- `internal/runtime/container_provision.go:324-335` (`buildInterpreterCommand` file branch) computes `relPath := filepath.Rel(invowkDir, string(scriptPath))` and executes `/workspace/<relPath>`, assuming `invowkDir` is mounted at `/workspace`. `container_provision.go:340,362` creates inline interpreter temp scripts inside `invowkDir` (`os.CreateTemp(invowkDir, "invowk-script-*"+ext)`) and executes `/workspace/<basename>`, which fails if `/workspace` is mounted `readonly`, `enabled: false`, or rooted at `workdir: ".."`.
- `pkg/invowkfile/invowkfile_schema.cue:118-150` (`#RuntimeConfigContainerBase`) documents `volumes: ["./data:/data", "/tmp:/tmp:ro"]`, yet `internal/container/engine_base.go:305-307` forwards `opts.Volumes` verbatim to `-v` without resolving relative host paths against `invowkDir`, and `e.volumeFormatter(v)` (`engine_base.go:306`) receives only a raw `VolumeMountSpec` string without per-mount SELinux label options or persistent-create propagation (`internal/runtime/container_persistent.go`).
- `pkg/invowkfile/invowkfile.go:77-86` (`Invowkfile`) and `internal/runtime/` (`ExecutionContext`) do not currently carry the discovery trust tier (Layer 1/2/3), `<cwd>` ceiling, or `IncludeEntry.workspace_root` (though `pkg/invowkfile/workdir_source.go:98` already tracks `WorkDirSource`!).
- `internal/container/docker.go:43` and `internal/container/podman.go:50,159-192` unconditionally apply `addSELinuxLabelWithCheck` (`:z`) whenever SELinux is active and `:z`/`:Z` is absent, mutating host file xattrs even on read-only mounts.
- `internal/config/config_schema.cue:60-67` defines `#IncludeEntry` (`path`, `alias`) and `config_schema.cue:70-82` defines `#ContainerConfig` (`auto_provision`, `tui_server_bind`).
- `formal/alloy/ModulePathContainment.als:35-36,43-45` and `formal/alloy/VirtualPathHarness.als:85-87` formalize `allowedWorkspace`, `allowedTrustedWorkdir`, and `allowedWorkdirCwd`, with correspondence checks enforced by `make formal`.

## Goals / Non-Goals

**Goals:**
- Enable `<repo>/<m>.invowkmod/` (`workdir: ".."`) and standalone `invowkfile.cue` commands to run cleanly in containers with configurable `/workspace` mount root, read-only mode, relative `volumes`, and SELinux label control.
- Preserve and extend formal module path containment across a clear 3-tier trust model, blocking absolute host bind-mounts in all `*.invowkmod` modules.
- Prevent unprivileged manifests (`invowkfile.cue` / `*.invowkmod`) from disabling container SELinux confinement (`--security-opt label=disable` lives exclusively in operator `config.cue`).

**Non-Goals:**
- Changing default `/workspace` mount root for existing `invowkfile.cue` or `*.invowkmod` commands that omit `workspace_mount` (non-breaking default stays `invowkDir`).
- Allowing `*.invowkmod` modules to declare absolute host bind-mounts in `volumes` (may be added later via explicit `#IncludeEntry` allowlists if a real use case emerges).

## Decisions

### D1. Three-tier containment & unified `<cwd>` ceiling
Evaluate every container mount (`workspace_mount` and `volumes`) using **physical path resolution** (`filepath.EvalSymlinks` on deepest existing ancestor + missing suffix, matching `VirtualPathHarness.als`) against source tier:
1. **Layer 1 (`invowk_modules/` vendored/remote and `~/.invowk/cmds/` global)**:
   - Allowed container bind-mount ceiling = declaring module's physical root only.
   - `workspace_mount.root: "workdir"` fails closed if `effectiveWorkDir` resolves outside the module root (except explicit CLI `--ivk-workdir`, which only narrows/mounts if trusted by user).
   - Relative `volumes` escaping module root and **any** absolute host bind-mount in `volumes` fail closed at runtime. Named volumes (`myvol:/data`, no `/`, `.`, or `..` host path shape) remain allowed.
2. **Layer 2 (`<cwd>/invowkfile.cue` standalone AND `<cwd>/<m>.invowkmod`)**:
   - Default `/workspace` mount root remains `invowkDir` (`<cwd>` for standalone; `<cwd>/<m>.invowkmod` for module).
   - `workspace_mount: { root: "workdir" }` and relative `volumes` (`./...`, `../...`) are allowed **iff** the resolved physical host path lies inside `<cwd>` (blocks `..` escaping above `<cwd>`, e.g., `workdir: ".."` in root `invowkfile.cue` or `workdir: "../.."` in `<cwd>/<m>.invowkmod`, unless `--ivk-workdir` was passed explicitly on CLI).
   - Absolute host bind-mounts in `volumes`: **blocked at runtime** for `<cwd>/<m>.invowkmod`; **allowed** for standalone `<cwd>/invowkfile.cue` (preserving `/tmp:/tmp:ro`, etc.) and surfaced by `invowk audit` + `--ivk-dry-run`.
3. **Layer 3 (`includes` in `config.cue`, outside `cwd`, e.g., `--ivk-no-cwd`)**:
   - Module manifests cannot self-grant parent mounts.
   - Add optional `workspace_root?: string` (absolute path validated in Go via `os.IsAbs` + physical check enclosing the included module or explicit directory) on `#IncludeEntry` (`internal/config/config_schema.cue:60-67`, `internal/config/types.go:281-286`) instead of an imprecise `allow_parent_workdir` boolean.
   - `workspace_mount.root: "workdir"` and relative `volumes` in an included module may resolve up to `IncludeEntry.workspace_root` when set, otherwise strictly within the module root. Absolute host bind-mounts in `volumes` remain blocked at runtime.

### D2. `workspace_mount` schema, relative `volumes`, and `getContainerWorkDir` fix
- Add `workspace_mount` to `#RuntimeConfigContainerBase` (`pkg/invowkfile/invowkfile_schema.cue:118`):
  ```cue
  workspace_mount?: close({
      enabled?:       bool // default: true
      root?:          "invowkfile" | "workdir" // default: "invowkfile"
      readonly?:      bool // default: false
      selinux_label?: "auto" | "shared" | "private" | "none" // default: "auto"
  })
  ```
- **Plumbing across Discovery -> ExecutionContext**: carry the source trust tier (`Layer 1` / `Layer 2` / `Layer 3`), `<cwd>` root, and resolved `IncludeEntry.WorkspaceRoot` from discovery (`internal/discovery/`) into `commandsvc` / `execute` / `runtime.ExecutionContext`, alongside `Invowkfile.ResolveEffectiveWorkDir` (`pkg/invowkfile/workdir_source.go:98`) so `--ivk-workdir` (`WorkDirSourceCLIOverride`) remains distinguishable from manifest-declared `workdir`.
- Fix `getContainerWorkDir` (`internal/runtime/container_provision.go:375-402`):
  - Compute the mounted host workspace root (`mountHostRoot`: `invowkDir` when `root: "invowkfile"`, `effectiveWorkDir` when `root: "workdir"`, or empty when `enabled: false`).
  - When `enabled: true` and `effectiveWorkDir` lies physically inside `mountHostRoot`, map `-w` to `/workspace` or `/workspace/<rel>`.
  - When `enabled: false` (or `effectiveWorkDir` is outside `mountHostRoot`), NEVER pass a raw host path verbatim as `-w` — map through mounted `volumes` targets if covered, accept an explicit container-absolute path only when configured, or fail closed explaining that `workdir` is not mounted inside the container.
- **Fix `buildInterpreterCommand` for BOTH `script.file` and inline temp scripts (`container_provision.go:324-364`)**:
  - For `script.file`: if `enabled: true` and `scriptPath` is physically inside `mountHostRoot`, map to `/workspace/<rel-to-mountHostRoot>` (e.g., `/workspace/cst.invowkmod/scripts/run.py` when `root: "workdir"`); if `enabled: false` or outside `mountHostRoot`, bind-mount `scriptPath` read-only at a dedicated container mount path (e.g., `/.invowk/scripts/<name>:ro`).
  - For inline interpreter scripts: write to `os.MkdirTemp("", "invowk-container-script-*")` (cleaned up *after* container stop/remove per `termination-signal-cleanup` ordering so Windows never hits `ERROR_SHARING_VIOLATION`) and mount read-only at `/.invowk/tmp/<file>:ro`.
- **Relative `volumes` resolution**: parse volume specs accounting for Windows drive letters (`C:\...`, `C:/...` vs `host:container[:opts]`) so Docker/Podman named volumes (`^[a-zA-Z0-9][a-zA-Z0-9._-]+$` without path separators or leading `.`) stay distinct from host filesystem paths. Resolve explicit relative host paths against `invowkDir`, apply Layer 1/2/3 ceiling checks, and reject invalid escapes before invoking the engine.

### D3. SELinux label policy split (`invowkfile` vs `config.cue`)
- Extend `VolumeMountSpec` / `container.RunOptions` (and `container_persistent.go` `CreateArgs`) so per-mount SELinux intent reaches `BaseCLIEngine` (`internal/container/engine_base.go:305-307`) and `addSELinuxLabelWithCheck` (`internal/container/podman.go:159`, `docker.go:43`):
  - `workspace_mount.selinux_label`: `"auto"` -> `:z` when SELinux active; `"shared"` -> `:z`; `"private"` -> `:Z`; `"none"` -> omit automatic `:z`/`:Z` **on `/workspace` only** (decoupled `:ro` temp/script mounts under `/.invowk/` still receive `:z`/`:Z` when SELinux labeling is active so confined containers can execute them!).
  - Preserve explicit `:z`/`:Z` on user `volumes` entries when configured.
- Operator-only `--security-opt label=disable`: add `selinux_labeling: *"enabled" | "disabled"` under `#ContainerConfig` in `internal/config/config_schema.cue:70-82` (`internal/config/types.go:341-346`) — NEVER in `invowkfile.cue`. When set by the operator in `config.cue`, pass `--security-opt label=disable` to `docker run` / `podman run` / persistent `create` and skip automatic `:z` labeling across mounts.

### D4. Dry-run, `invowk audit`, and Alloy formal correspondence
- `--ivk-dry-run` (`cmd/invowk/cmd_dryrun.go`): display resolved workspace mount status (`enabled`/`disabled`, host root, container target, `ro`/`rw`, SELinux label) and resolved `volumes` mounts.
- `internal/audit/`: add container mount findings (parent/workspace mount outside module directory, sensitive host absolute paths in standalone `invowkfile.cue` such as `/`, `/etc`, `/var/run/docker.sock`, `~/.ssh`, and `workspace_mount.selinux_label: "none"` notes).
- Formal models (`formal/alloy/ModulePathContainment.als`, `formal/alloy/VirtualPathHarness.als`, `formal/manifest.toml`): model the Layer 1/2/3 container mount ceiling, relative `volumes` resolution, and absolute-bind rejection for modules, keeping `scripts/formal.py correspondence` and `make formal` passing with updated golden vectors.

## Risks / Trade-offs

- **Risk**: `workspace_mount.enabled: false` + `auto_provision` or `script.file` inside a module still needs module scripts reachable inside the container.
  -> **Mitigation**: `script.file` scripts are read into memory or copied/mounted read-only via provisioning/temp-file mount; document clearly that `workspace_mount.enabled: false` omits `/workspace` checkout mounting while keeping script execution functional.
- **Risk**: Updating `ModulePathContainment.als` and `getContainerWorkDir` touches `formal-bindings` correspondence tables (`.agents/rules/formal-bindings.md`).
  -> **Mitigation**: Dedicated task group in `tasks.md` updates correspondence rows, mutants, and regenerates golden vectors (`make formal-golden` + `make formal`).

## Maintainer decisions

- **2026-11-27**: Confirmed official endorsement of `<repo>/<m>.invowkmod/` + `workdir: ".."` under the 3-tier trust model; unified `<cwd>` ceiling for both `<cwd>/invowkfile.cue` standalone and `<cwd>/<m>.invowkmod`; **runtime blocking of absolute host bind-mounts in `volumes` across all `*.invowkmod` modules** (keeping named volumes allowed, and keeping standalone `<cwd>/invowkfile.cue` absolute volumes allowed with `invowk audit` + `--ivk-dry-run`); explicit absolute `workspace_root` on `#IncludeEntry` (instead of `allow_parent_workdir: true`); `--security-opt label=disable` restricted to operator `config.cue`; and decoupling inline temp scripts from `invowkDir`.

## Open questions

- Confirm syntax for opting out of `:z` on individual user `volumes` entries (if needed alongside `workspace_mount.selinux_label` on `/workspace` and operator `container.selinux_labeling: "disabled"`).
