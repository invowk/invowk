# Tasks

## 1. Schemas, Config & Sync

- [x] 1.1 Add `workspace_mount` (`enabled`, `root`, `readonly`, `selinux_label`) to `#RuntimeConfigContainerBase` in `pkg/invowkfile/invowkfile_schema.cue` and Go types in `pkg/invowkfile/runtime.go`; add `workspace_root` to `#IncludeEntry` and `selinux_labeling` to `#ContainerConfig` in `internal/config/config_schema.cue` + `internal/config/types.go`; verify with `go test -run TestSync ./pkg/invowkfile/ ./internal/config/`

## 2. Three-Tier Mount Policy, Volumes & Script Decoupling

- [x] 2.1 Plumb source trust tier (Layer 1/2/3), `<cwd>` root, `IncludeEntry.WorkspaceRoot`, and `WorkDirSource` (`pkg/invowkfile/workdir_source.go`) from `internal/discovery/` into `runtime.ExecutionContext`, implement the 3-tier containment evaluator + runtime blocking of absolute host bind-mounts in `*.invowkmod` (handling Windows drive letters vs named volumes cleanly); verify with `go test -run 'Test.*WorkspaceMount|Test.*VolumeContainment' ./internal/runtime/ ./internal/discovery/`
- [x] 2.2 Fix `getContainerWorkDir` (`internal/runtime/container_provision.go`), resolve relative `volumes` against `invowkDir`, fix `buildInterpreterCommand` `script.file` container path mapping for `root: "workdir"`/`enabled: false`, and decouple inline interpreter temp-script files (`os.MkdirTemp` + `:ro` mount, cleaned up after container stop/remove); verify with `go test ./internal/runtime/...`

## 3. SELinux Engine Options, Dry-Run & Audit

- [x] 3.1 Update `internal/container/docker.go`, `podman.go`, `engine_base.go`, and `internal/runtime/container_persistent.go` to honor per-mount `selinux_label` (`auto`/`shared`/`private`/`none`, keeping isolated `:ro` script mounts labeled when SELinux is enabled) and operator `container.selinux_labeling: "disabled"` (`--security-opt label=disable`); verify with `go test ./internal/container/... ./internal/runtime/...`
- [x] 3.2 Update `cmd/invowk/cmd_dryrun.go` to display resolved container workspace/volume mounts and SELinux labels (coordinated with `privilege-capabilities` `DryRunPlan` layout), and add container mount rules to `internal/audit/` (`checker.go` + `finding_codes.go`); verify with `go test ./cmd/invowk/... ./internal/audit/...`

## 4. Formal Models, Integration Tests & Docs

- [x] 4.1 Update `formal/alloy/ModulePathContainment.als`, `formal/alloy/VirtualPathHarness.als`, `formal/manifest.toml`, and golden/replay tests for the 3-tier container containment rules and mutants; verify with `make formal-golden && python3 scripts/formal.py correspondence && make formal && make mutation-formal-refresh && make mutation-formal`
- [x] 4.2 Add CLI/container tests (`tests/cli/`, using `debian:stable-slim` only) covering `workdir: ".."`, `readonly: true`, `script.file`, relative volumes, module absolute-volume rejection, and SELinux labels; update `website/` docs (`cd website && npm run build`)

## 5. Final Verification Gates

- [x] 5.1 Run all completion gates from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, `make check-goplint-consumer-routed`, `make formal`, and `make sonar-local`
