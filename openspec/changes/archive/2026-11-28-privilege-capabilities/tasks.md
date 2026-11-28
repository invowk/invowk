# Tasks

## 1. Schema, Capability Enum & Sync

- [x] 1.1 Add `"elevated"` and `"unprivileged"` to `#CapabilityName` and update the capability list doc comment in `pkg/invowkfile/invowkfile_schema.cue` and `pkg/invowkfile/capabilities.go` (updating `Validate()`, `ValidCapabilityNames()`, and error messages); verify with `go test -run 'Test.*Capability|TestSync' ./pkg/invowkfile/`

## 2. Host & Container Capability Evaluators

- [x] 2.1 Implement platform-split `"elevated"`/`"unprivileged"` checks (`os.Geteuid() == 0` on Unix; Windows process token elevation check) in `internal/app/commandadapters/dependency_host.go` (+ `_unix.go`/`_windows.go`) and `id -u` probes in `internal/app/commandadapters/dependency_runtime.go`; verify with `go test ./internal/app/commandadapters/... ./internal/app/deps/...`

## 3. Help, Dry-Run & Security Audit

- [x] 3.1 Carry merged host (root+command+impl) and runtime `capabilities` in `DryRunPlan` (`internal/app/commandsvc/`) and render them uniformly in `cmd/invowk/cmd_discovery.go` (`--help`) and `cmd/invowk/cmd_dryrun.go` (`--ivk-dry-run`); verify with `go test -run 'TestDryRun|Test.*Help' ./cmd/invowk/ ./internal/app/commandsvc/`
- [x] 3.2 Add the `elevated` file-permission checker across root+command+impl capability scopes in `internal/audit/` (registering in `internal/audit/checker.go` + `finding_codes.go`, handling `0o002` world-writable `HIGH` vs UPG-aware group-writable) and verify with `go test ./internal/audit/...`
- [x] 3.3 Add CLI testscript tests (`tests/cli/`) covering `"unprivileged"` pass, `"elevated"` fail, `--ivk-dry-run`, and `invowk audit` findings; update `website/` schema and audit docs (`cd website && npm run build`)

## 4. Final Verification Gates

- [x] 4.1 Run all completion gates from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, `make check-goplint-consumer-routed`, and `make sonar-local`
