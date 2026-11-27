# Tasks

## 1. Watch Resume Reporter & Guardrails

- [x] 1.1 Implement `watchResumeReporter` (using `ansi.SetProgramResume` and `ansi.ClearProgramResume` from `x/ansi v0.12.0`), handling `OSC7577_TOKEN` presence, TTY-only output (`/dev/tty` or terminal `stdout`), `os.Executable()` + `os.Getwd()`, `INVOWK_CONFIG`/`INVOWK_NO_CWD` flag materialization, `--ivk-env-var` suppression, and protocol validity/size checks
- [x] 1.2 Add deterministic unit tests for `watchResumeReporter` (fake writer, fake clock/PID, valid MAC verification, `--ivk-env-var` skip, control-char/oversize skip, non-TTY no-op) and verify with `go test ./cmd/invowk -run TestWatchResume`

## 2. Child Execution Token Stripping Propagation

- [x] 2.1 Thread `StripProgramResumeToken` from `cmd.ExecuteRequest` -> `commandsvc.ExecuteRequest` -> `execute.BuildExecutionContextOptions` -> `runtime.EnvContext` so `DefaultEnvBuilder.Build` strips `OSC7577_TOKEN` when requested (covering non-interactive `native` children)
- [x] 2.2 Add unit tests in `internal/runtime/` and `internal/app/commandsvc/` confirming `StripProgramResumeToken: true` removes `OSC7577_TOKEN` while leaving non-watch `native` execution unaffected, and verify with `go test ./internal/runtime/... ./internal/app/commandsvc/... ./internal/app/execute/...`

## 3. Watch Mode Lifecycle & Signal Handling (`SIGINT` vs `SIGHUP`/`SIGTERM`)

- [x] 3.1 Add platform-split shutdown signal helper (`watch_signals_unix.go` / `watch_signals_other.go`) distinguishing `SIGHUP`/`SIGTERM` (preserve record) from `SIGINT`/error return (emit `clear=1`)
- [x] 3.2 Integrate `watchResumeReporter`, signal handling, and `StripProgramResumeToken: true` into `runWatchMode` (`cmd/invowk/cmd_watch.go`), including clean interaction with `--ivk-watch -i`
- [x] 3.3 Add unit and PTY/CLI tests covering watch-mode OSC 7577 emission, `SIGINT` clear, `SIGHUP`/`SIGTERM` preservation, and child `OSC7577_TOKEN` stripping, and verify with `go test ./cmd/invowk/... ./tests/cli -run TestWatch`

## 4. Documentation

- [x] 4.1 Update watch-mode documentation in `website/` to describe Program Resume Protocol (OSC 7577) support, the `--ivk-env-var` guardrail, and `SIGINT` vs terminal-restart behavior, and verify `cd website && npm run build`

## 5. Final Verification Gates

- [x] 5.1 Run full completion checklist from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, and `make sonar-local`
