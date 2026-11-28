# Proposal: Declarative Privilege Capabilities (`elevated` / `unprivileged`)

## Why

Projects like CST strictly separate unprivileged commands (checks, diagnostics, planning) from root-privileged commands (`install`, `apply`), but `#CapabilityName` currently only supports `"local-area-network" | "internet" | "containers" | "tty"`. Authors must resort to non-portable `custom_checks` (`test "$(id -u)" = 0`) that are invisible to command `--help`, `--ivk-dry-run`, and `invowk audit`.

## What Changes

- Add `"elevated"` (`os.Geteuid() == 0` on Unix; elevated process token on Windows) and `"unprivileged"` (negation of `elevated`) to `#CapabilityName` in `pkg/invowkfile/invowkfile_schema.cue` and `pkg/invowkfile/capabilities.go`.
- Evaluate `"elevated"` and `"unprivileged"` in both the host capability checker (`internal/app/commandadapters/dependency_host.go`) and the container runtime dependency probe (`internal/app/commandadapters/dependency_runtime.go`).
- Add an `invowk audit` checker rule in `internal/audit/` that flags commands declaring `"elevated"` whose `invowkfile.cue`, `invowkmod.cue`, or referenced script/env files are group- or world-writable (`mode & 0o022 != 0`).
- Surface declared `capabilities` consistently in command `--help` and `--ivk-dry-run` output (`cmd/invowk/cmd_dryrun.go`, `cmd/invowk/cmd_discovery.go`).
- Explicitly reject built-in `sudo` elevation and reject synthesizing an `INVOWK_INVOKING_USER` variable from `SUDO_USER`.

## Capabilities

### New Capabilities

- `privilege-capabilities`: Defines declarative `"elevated"` and `"unprivileged"` capability checks across host and container environments, help/dry-run capability visibility, and `invowk audit` file-permission checks for elevated commands.

### Modified Capabilities

(none)

## Impact

- Schema & Types: `pkg/invowkfile/invowkfile_schema.cue`, `pkg/invowkfile/capabilities.go`, schema-sync tests in `pkg/invowkfile/`.
- Dependency Checkers: `internal/app/commandadapters/dependency_host.go` (+ `_unix.go` / `_windows.go`), `internal/app/commandadapters/dependency_runtime.go`.
- CLI & Audit: `cmd/invowk/cmd_dryrun.go`, `cmd/invowk/cmd_discovery.go`, `internal/audit/` (new check + `finding_codes.go`).
- Tests & Docs: unit/testscript/audit tests and `website/` schema & audit docs.
