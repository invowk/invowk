# Proposal

## Why

CI on PR #211 (run 45559206210, job `Test (ubuntu-latest / podman)`) failed ten `internal/sshserver` tests under `-race` with a data race between `listenerWg.Wait()` in upstream `charm.land/ssh` v0.4.3 `Shutdown` and `listenerWg.Add(1)` in its `Serve`. Two defects meet there:

- **Upstream (charm.land/ssh v0.4.3):** when `Shutdown` runs before `Serve` has registered its listener, `Shutdown` returns at once, and `Serve` then registers the listener, resets its done channel ("reuse after Shutdown") and serves on. The WaitGroup `Add` and `Wait` race. This reproduces with the library alone.
- **Ours:** `serve()` reports readiness (`TransitionToRunning`) before it calls `srv.Serve`, so `Start` returns before the upstream server has registered its listener. A `Stop` right after `Start`, which every test with `defer testutil.MustStop` does, lands in that window. A `Stop` while `Start` is still running can also reach our own `WaitForShutdown` before `Start` calls `AddGoroutine`, the same WaitGroup contract violation in our code, and can leave a listener open with a serve goroutine nobody stops.

## What Changes

- `Start` returns ready only after the upstream server has registered its listener. The observable point is the first `Accept` on a listener wrapper that `serve()` passes to `srv.Serve`.
- `Stop` closes the listener first (as today), then calls upstream `Shutdown` only after the serve goroutine has reached that point or has exited, waiting at most `ShutdownTimeout`. On a timeout it skips upstream `Shutdown`, so no `WaitGroup` `Add` can race a `Wait`; the closed listener still ends the serve loop at its first `Accept`.
- `serve()` signals on every exit path, including the early return when the server or listener is missing and an upstream `Serve` that fails before its first `Accept`.
- `Start` publishes the listener, the upstream server and its goroutines in one critical section that a stop already in progress refuses: a `Stop` during `Start` never reaches our `WaitForShutdown` before an `AddGoroutine`, and the listener a refused `Start` opened is closed.
- A `Stop` that wins before readiness makes `Start` return an error instead of `nil`.
- A code comment names the upstream defect by its behaviour, so the workaround can be removed once upstream fixes it (the maintainer adds the issue URL later).
- Regression tests: a deterministic window-widening test through an unexported struct field (negative control recorded on 36f85fd6), a Stop-during-Start case, behavioural assertions that need no race detector, and a short bounded Start/Stop stress loop.
- `.agents/skills/server/SKILL.md` records the readiness rule for servers whose upstream registers listeners asynchronously.

## Capabilities

### New Capabilities

- `ssh-callback-server-lifecycle`: start readiness, stop ordering and stop-during-start behaviour of the host-callback SSH server in `internal/sshserver`.

### Modified Capabilities

None.

## Impact

- Code: `internal/sshserver/server_lifecycle.go` (`Start`, `doStop`, `serve`, a listener wrapper and readiness gate), `internal/sshserver/server.go` (`Server` fields), new tests in `internal/sshserver`.
- Not touched: `internal/core/serverbase`, `internal/tuiserver` (uses `net/http`, which handles Shutdown-before-Serve), `internal/discovery`, dependencies (`go.mod` unchanged).
- Behaviour: `Start` may return a few microseconds later (after upstream `Serve` reaches `Accept`); `Start` returns an error, not `nil`, when a concurrent `Stop` wins before readiness. The only production caller, `internal/app/commandadapters/host_access.go:63-67`, calls `Start` and `Stop` from one goroutine and is unaffected.
- Formal: `internal/sshserver/**` is a formal CI path (`formal/manifest.toml:57`, trace `HostCallbackToken` at `:2157`), but the touched symbols are not bound by any correspondence row (`formal_mutation.py symbols` reports `Start`, `Stop`, `doStop`, `serve` as unplanned), so no ledger refresh is planned.
- Not breaking.
