# Proposal: Termination Signal Forwarding and Resource Cleanup (`SIGTERM` / `SIGHUP`)

## Why

`cmd/invowk/root.go` currently notifies `fang.Execute` only on `os.Interrupt` (`SIGINT`). When Invowk receives `SIGTERM` or `SIGHUP` (from `systemd`, `timeout`, CI job cancellation, or a closing terminal), the OS terminates the Go process immediately without cancelling `context.Context`, signalling child processes, stopping ephemeral containers, or removing temporary scripts.

## What Changes

- Expand signal notification in `cmd/invowk/root.go` using a platform-split helper (`os.Interrupt`, `syscall.SIGTERM`, `syscall.SIGHUP` on Unix; `os.Interrupt` on Windows) so `context.Context` cancellation triggers cleanup paths across all runtimes.
- Isolate non-interactive native child commands in their own process group on Unix (`Setpgid: true`) and forward termination signals to `-pgid` with a bounded grace period before `SIGKILL` — explicitly keeping foreground TTY executions in the terminal's process group so interactive commands (`sudo`, `ssh`, prompts) do not suspend on `SIGTTIN`/`SIGTTOU`.
- Track ephemeral container invocations (`ContainerRuntime.Execute` in `internal/runtime/container_exec.go`) with a unique container name/ID so cancellation runs `Stop`/`Remove` on a bounded `context.WithoutCancel` cleanup context instead of leaving orphaned daemon containers.
- **BREAKING**: Return `128 + signal` (`129` `SIGHUP`, `130` `SIGINT`, `143` `SIGTERM`) on Unix when Invowk terminates due to an incoming signal (previously `SIGINT` returned `1` and `SIGTERM`/`SIGHUP` terminated via OS default), updating the `cli-exit-codes` specification.

## Migration notes

- Scripts or wrappers checking for exit status `1` when interrupting `invowk cmd` with `SIGINT` on Unix will now observe `130` (and `143` on `SIGTERM`, `129` on `SIGHUP`), matching standard POSIX shells. Explicit command timeouts (`timeout`) and `invowk tui spin` cancellations continue exiting with status `1`.

## Capabilities

### New Capabilities

- `signal-propagation-and-cleanup`: Defines `SIGTERM`/`SIGHUP`/`SIGINT` propagation, safe Unix process-group signaling (non-TTY vs direct TTY), temp-script cleanup, and bounded daemon container teardown on cancellation.

### Modified Capabilities

- `cli-exit-codes`: Updates `Non-usage failures exit with status 1` and adds signal exit status rules (`128 + signal` on Unix for `SIGINT`, `SIGTERM`, `SIGHUP`) while preserving `tui spin`'s documented per-command exception.

## Impact

- CLI & Signal Handling: `cmd/invowk/root.go`, new platform-split signal helpers (`*_unix.go` / `*_windows.go` in `cmd/invowk/` and `internal/runtime/`).
- Runtimes: `internal/runtime/native.go`, `internal/runtime/container_exec.go`, `internal/container/engine_base.go`.
- Tests & Docs: Unix signal/process-group/container-cleanup integration tests, `cli-exit-codes` spec update, and `website/` CLI reference docs.
