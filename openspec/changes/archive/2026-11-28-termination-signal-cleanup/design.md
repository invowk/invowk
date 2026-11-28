# Design: Termination Signal Forwarding and Resource Cleanup (`SIGTERM` / `SIGHUP`)

## Context

Current behavior in `main`:
- `cmd/invowk/root.go:103-108` calls `fang.Execute(context.Background(), rootCmd, ..., fang.WithNotifySignal(os.Interrupt))`. `SIGTERM` and `SIGHUP` are not trapped, causing immediate OS termination without running `defer` blocks.
- `internal/runtime/native.go:207,253` creates `exec.CommandContext(ctx.Context, ...)` with `cmd.WaitDelay = cmdWaitDelay` (`10 * time.Second`, `native.go:24`), without setting `SysProcAttr.Setpgid` or a custom `cmd.Cancel` hook that signals descendants before killing. Temp scripts are removed via `defer os.Remove(tempFile)` (`native.go:247`).
- `internal/runtime/container_exec.go:371` runs `defer prep.cleanup()`, and both `Execute` (`container_exec.go:391-406`), `ExecuteCapture` (`container_exec.go:455-470`), and interactive `PrepareCommand` (`internal/runtime/container_prepare.go:69-97`) construct ephemeral `container.RunOptions` with `Remove: true` and `Name: ""` (no deterministic container ID/name for ephemeral runs). `containerEngine` (`internal/runtime/container.go:66-77`) does not yet expose `Remove` (or `Stop`). Cancelling `ctx.Context` kills the CLI client (`docker run` / `podman run`), which can leave the container running in a Docker daemon.
- `cmd/invowk/watch_signals_unix.go:20-58` traps `SIGHUP` and `SIGTERM` separately for watch mode so `preserve.Store(true)` keeps OSC 7577 resume records on terminal shutdown (`select` against `sigCh` and `parent.Done()` at `watch_signals_unix.go:42-48`).
- `openspec/specs/cli-exit-codes/spec.md:281` classifies "command timeout or cancellation" under exit status `1`, and `cli-exit-codes/spec.md:357-360` keeps `invowk tui spin` at exit status `1` on Ctrl+C / signal (currently phrasing `"as a native invowk cmd does"`).

## Goals / Non-Goals

**Goals:**
- Ensure `SIGINT`, `SIGTERM`, and `SIGHUP` on Unix (`os.Interrupt` on Windows) trigger graceful context cancellation, temp-script removal, child process-group termination, and ephemeral container `Stop`/`Remove`.
- Prevent `SIGTTIN`/`SIGTTOU` hangs by only creating a new Unix process group (`Setpgid: true`) when the child is not attached directly to a controlling TTY.
- Return POSIX-standard `128 + signal` (`129`/`130`/`143`) when Invowk terminates due to an OS signal on Unix.

**Non-Goals:**
- Stopping or removing opt-in `persistent` containers (`runtime.persistent`) on command cancellation — persistent containers intentionally outlive individual invocations.
- Changing `invowk tui spin`'s existing exit-code-1 contract (`openspec/specs/cli-exit-codes/spec.md:357-360`).

## Decisions

### D1. Platform-split signal notification, watch coordination, and exit-code mapping
- Replace `fang.WithNotifySignal(os.Interrupt)` (`cmd/invowk/root.go:107`) with Invowk's own platform-split `signal.Notify` root context (trapping `os.Interrupt`, `syscall.SIGTERM`, `syscall.SIGHUP` on Unix; `os.Interrupt` on Windows) that records the exact received `os.Signal` (`fang.WithNotifySignal` discards the `os.Signal` and runs `fang`'s error handler before returning).
- Coordinate with `watchShutdownSignals` (`cmd/invowk/watch_signals_unix.go:20-58`): attach/read the root signal tracker on context so when root cancels on `SIGTERM`/`SIGHUP`, watch mode unambiguously treats it as `preserve.Store(true)` without racing `<-parent.Done()` vs `<-sigCh` at `watch_signals_unix.go:42-48`.
- Coordinate with `cli-color-and-quiet-exit-framing` (`fang.WithErrorHandler`): when termination is caused by a trapped signal, avoid printing a redundant `ERROR / context canceled` box (map to `&ExitError{Code: 128 + int(sig)}` on Unix; keep `invowk tui spin` returning exit status `1` per `cli-exit-codes/spec.md:357-360`). Explicit context deadlines (`context.DeadlineExceeded`) remain exit `1`.

### D2. Safe Unix process-group lifecycle (non-TTY only)
- In `internal/runtime/native_unix.go` (paired with `native_windows.go` stub), evaluate whether the child inherits a controlling TTY directly (e.g., `stdin`/`stdout` are TTYs and not running inside `-i` PTY isolation).
- **Non-TTY / pipe / CI / `systemd` / captured runs**: set `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` and wire `cmd.Cancel` to send the incoming termination signal (defaulting to `syscall.SIGTERM`) to `-pgid`, waiting up to a bounded grace window (e.g., 5s, bounded inside `cmd.WaitDelay`) before escalating to `SIGKILL` on `-pgid`.
- **Direct TTY runs**: leave the child in the foreground process group so terminal-driven job control, `sudo` password prompts, and `ssh` never stall on `SIGTTIN`/`SIGTTOU`. Note: `-i` interactive PTY runs (`internal/app/commandadapters/interactive.go`) already own a dedicated PTY session and close/signal the PTY child on cancellation.

### D3. Traceable ephemeral container cleanup on cancellation
- Expose `Remove`/`Stop` on `containerEngine` (`internal/runtime/container.go:66-77`) and assign a unique ephemeral container name (e.g., `invowk-ephem-<runid>`) across `ContainerRuntime.Execute` (`internal/runtime/container_exec.go:391-406`), `ExecuteCapture` (`container_exec.go:455-470`), and interactive `PrepareCommand` (`internal/runtime/container_prepare.go:69-97`).
- On `ctx.Err() != nil`, execute daemon cleanup (`Stop` with short timeout followed by forced `Remove`) using a bounded `context.WithoutCancel(ctx.Context)` (e.g., 5s–10s timeout) **before** `prep.cleanup()` removes any host-mounted temporary files/directories (preventing Windows sharing/busy errors if a temp script is bind-mounted `:ro` into the container).

## Risks / Trade-offs

- **Risk**: `Setpgid: true` applied indiscriminately breaks interactive CLI prompts on TTY.
  -> **Mitigation**: Explicitly gate `Setpgid: true` on non-TTY / non-foreground invocations and test both piped child-tree killing and direct/PTY interactive commands.
- **Risk**: Graceful `Stop` + `Remove` adds up to the grace window latency on `SIGTERM`.
  -> **Mitigation**: Keep grace period bounded (e.g., 3s–5s) and honor a second signal (`SIGINT`/`SIGTERM`) to abort waiting immediately if `fang`/signal handler supports force-exit.

## Maintainer decisions

- **2026-11-27**: Confirmed `os.Interrupt`/`SIGTERM`/`SIGHUP` on Unix (`os.Interrupt` on Windows), gating `Setpgid: true` to non-TTY native executions (avoiding `SIGTTIN`/`SIGTTOU`), adding daemon container cleanup via `context.WithoutCancel` + traceable IDs, and returning `128 + signal` on Unix with a spec delta to `cli-exit-codes`.

## Open questions

- Confirm exact grace-period duration (e.g., 5s total vs 3s term + `cmdWaitDelay`) so CI tests on slow runners stay deterministic without hanging `make test`.
