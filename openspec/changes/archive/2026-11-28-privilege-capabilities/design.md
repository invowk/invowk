# Design: Declarative Privilege Capabilities (`elevated` / `unprivileged`)

## Context

Current behavior in `main`:
- `pkg/invowkfile/invowkfile_schema.cue:370` defines `#CapabilityName: "local-area-network" | "internet" | "containers" | "tty"`, mirrored by `pkg/invowkfile/capabilities.go:11-20,68-85` with `//goplint:nonzero,enum-cue=#CapabilityName` (`capabilities.go:31`).
- `internal/app/commandadapters/dependency_host.go:214-234` switches on `invowkfile.CapabilityName` for host `depends_on.capabilities` checks.
- `internal/app/commandadapters/dependency_runtime.go:152-165` maps `invowkfile.CapabilityName` to POSIX shell probes for container `runtime.depends_on.capabilities` checks.
- `cmd/invowk/cmd_dryrun.go:92-94` notes that dependency validation is skipped in dry-run mode without displaying declared `capabilities` (or `depends_on` summary), and `cmd/invowk/cmd_discovery.go:188,243-245` builds leaf `Long` help without listing declared capabilities.
- `internal/audit/finding_codes.go:5-68` defines all audit finding codes; no rule inspects privilege requirements vs file permissions today.

## Goals / Non-Goals

**Goals:**
- Support cross-platform `"elevated"` and `"unprivileged"` capability checks through the existing `depends_on.capabilities` pipeline (no new schema section).
- Flag insecure non-owner writable files (`invowkfile.cue`, `invowkmod.cue`, referenced `script.file` and `env.files`) on commands requiring `"elevated"` via `invowk audit`.
- Display declared `capabilities` uniformly in leaf `--help` and `--ivk-dry-run`.

**Non-Goals:**
- Automatic privilege elevation (e.g., `elevate: "sudo"`), which remains explicitly rejected because `invowk` often resides in user-writable locations.
- Synthesizing `INVOWK_INVOKING_USER` from `SUDO_USER` (non-portable across `doas`/`run0`/`pkexec`/Windows and untrustworthy as an authorization signal; scripts needing `SUDO_USER` already read it via standard env inheritance).

## Decisions

### D1. Cross-platform privilege detection
- **Unix (host)**: `"elevated"` passes iff `os.Geteuid() == 0`; `"unprivileged"` passes iff `os.Geteuid() != 0`.
- **Windows (host)**: split into `_windows.go` / `_unix.go` in `internal/app/commandadapters/`: use `golang.org/x/sys/windows` (`Token.IsElevated()`) on the current process token (`windows.GetCurrentProcessToken()`); `"unprivileged"` is its boolean negation.
- **Container runtime (`dependency_runtime.go:152-165`)**: `"elevated"` maps to `test "$(id -u)" -eq 0`; `"unprivileged"` maps to `test "$(id -u)" -ne 0`. Note in docs that container images often run as UID `0` by default unless `USER` or rootless mapping dictates otherwise, so `runtime.depends_on.capabilities` accurately probes the in-container UID.

### D2. Audit rule for elevated command definitions (`internal/audit/`)
- Add an audit checker in `internal/audit/` (registered in `DefaultCheckers()` in `internal/audit/checker.go` with codes following `<domain>-<category>-<description>` in `internal/audit/finding_codes.go`) inspecting all host capability scopes (root `Invowkfile.DependsOn`, command `Command.DependsOn`, and implementation `Implementation.DependsOn`).
- When any scope declares `"elevated"` and `invowkfile.cue`, `invowkmod.cue`, or any referenced `script.file` or `env.files` target is writable by non-owners on Unix:
  - World-writable (`mode & 0o002 != 0`): emit `HIGH` severity.
  - Group-writable (`mode & 0o020 != 0`): check whether the file's group is a single-user private group (UPG: e.g., file GID matches owner UID / single member in group) or shared group; emit `MEDIUM` for shared-group writable files and avoid noisy `HIGH` false positives on standard Fedora/Silverblue `umask 0002` UPG checkouts.
- Provide a clean no-op / platform helper on Windows where POSIX `0o022` mode bits do not apply.

### D3. Uniform capability visibility in `--help` and `--ivk-dry-run`
- Extend `DryRunPlan` (`internal/app/commandsvc/types.go` / `service.go:226-268`) to carry merged declared host `depends_on.capabilities` (root + command + implementation) and container runtime `depends_on.capabilities`.
- Render capabilities uniformly in `buildLeafCommand` (`cmd/invowk/cmd_discovery.go:188,243-245`) and `renderDryRun` (`cmd/invowk/cmd_dryrun.go:23-96`, coordinated cleanly with G6's container mount display), and update the capability list doc comment in `pkg/invowkfile/invowkfile_schema.cue:376-380`.

## Risks / Trade-offs

- **Risk**: Windows CI vs Unix permission semantics differ for the audit file-mode check.
  -> **Mitigation**: Gate POSIX `0o022` mode bits cleanly on Unix or provide an explicit cross-platform helper that skips POSIX-mode checks cleanly on Windows without false positives.
- **Risk**: `goplint` enum-cue sync requires `pkg/invowkfile/capabilities.go` and `invowkfile_schema.cue` to match identically.
  -> **Mitigation**: Update `ValidCapabilityNames()`, `CapabilityName.Validate()`, and `InvalidCapabilityNameError.Error()`, and verify with `Sync*` and `goplint` checks.

## Maintainer decisions

- **2026-11-27**: Confirmed adding `"elevated"` and `"unprivileged"` to `#CapabilityName`, implementing the `invowk audit` writable-file check for elevated commands, rendering declared `capabilities` uniformly in `--help` and `--ivk-dry-run`, and **rejecting** `INVOWK_INVOKING_USER` synthesis.

## Open questions

- None blocking; confirm exact `finding_codes.go` constant names so they align with G6's container mount audit checker and `internal/audit/triage_test.go` / `scanner_test.go`.
