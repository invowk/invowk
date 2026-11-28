# Tasks

## 1. Root Signal Notification & Exit Code Mapping

- [x] 1.1 Replace `fang.WithNotifySignal` with Invowk's platform-split signal-recording context (`os.Interrupt`, `syscall.SIGTERM`, `syscall.SIGHUP` on Unix; `os.Interrupt` on Windows) in `cmd/invowk/root.go`, coordinate `SIGHUP`/`SIGTERM` with `watchShutdownSignals` (`cmd/invowk/watch_signals_unix.go`) so OSC 7577 resume preservation never races, and map Unix signal termination to `128 + signal` while keeping `tui spin` at `1`; verify with `go test -run 'TestSignalExitCode|TestWatchShutdown' ./cmd/invowk/`

## 2. Native Process-Group & Temp-Script Cleanup

- [x] 2.1 Implement Unix `Setpgid: true` and `-pgid` grace-period termination (`SIGTERM` then `SIGKILL`) gated strictly to non-TTY/non-foreground executions in `internal/runtime/native_unix.go` (+ Windows stub), verifying temp-script cleanup and interactive TTY safety with `go test -run TestNativeRuntime_Signal ./internal/runtime/`

## 3. Container Daemon Cleanup on Cancellation

- [x] 3.1 Expose `Remove`/`Stop` on `containerEngine` (`internal/runtime/container.go`), assign traceable ephemeral container identifiers across `ContainerRuntime.Execute`, `ExecuteCapture` (`internal/runtime/container_exec.go`), and interactive `PrepareCommand` (`internal/runtime/container_prepare.go`), and run bounded `context.WithoutCancel` daemon cleanup before `prep.cleanup()` removes temp files; verify with `go test -run TestContainerRuntime_CancelCleanup ./internal/runtime/`
- [x] 3.2 Add CLI/integration tests for `SIGTERM`/`SIGHUP` forwarding, orphan prevention, and `143`/`129`/`130` exit codes (`tests/cli/`), and update `website/` CLI reference Exit Codes docs; verify with `go test -run 'Test.*Signal' ./tests/cli/ && cd website && npm run build`

## 4. Final Verification Gates

- [x] 4.1 Run all completion gates from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, and `make sonar-local`
