# Proposal

## Why

CI on PR #211 (run 45559206210, job `Test (ubuntu-latest / podman)`) failed ten `internal/sshserver` tests under `-race` with a data race between `listenerWg.Wait()` in upstream `charm.land/ssh` v0.4.3 `Shutdown` and `listenerWg.Add(1)` in its `Serve`. Two defects meet there:

- **Upstream (charm.land/ssh v0.4.3, https://github.com/charmbracelet/ssh/issues/61):** when `Shutdown` runs before `Serve` has registered its listener, `Shutdown` returns at once, and `Serve` then registers the listener, resets its done channel ("reuse after Shutdown") and serves on. The listener WaitGroup's `Add` and `Wait` race. This reproduces with the library alone.
- **Ours:** `serve()` reports readiness (`TransitionToRunning`) before it calls `srv.Serve`, so `Start` returns before the upstream server has registered its listener. A `Stop` right after `Start`, which every test with `defer testutil.MustStop` does, lands in that window. A `Stop` while `Start` is still running can also reach our own `WaitForShutdown` before `Start` calls `AddGoroutine`, the same WaitGroup contract violation in our code, and can leave a listener open with a serve goroutine nobody stops.
- **Exposed by the fix:** once readiness waits for the first `Accept`, a startup timeout or a cancelled `ctx` can fire while the serve goroutine is still before its serve loop. Today that path marks the server `Failed` but closes neither the listener nor the upstream server, so the server would keep serving and a later `Stop` would block in `WaitForShutdown`. Moving readiness makes the path reachable, so this change tears it down too.

This change closes the listener-registration race. It does not close the upstream connection-registration race (`connWg.Add` in `trackConn` against `connWg.Wait()` in `Shutdown`, https://github.com/gliderlabs/ssh/issues/117), which stays reachable in principle when an SSH handshake completes while `Stop` runs. Closing that one would mean dropping the graceful upstream `Shutdown`, which the F11 characterisation row and `TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen` require. It is a known upstream residual; the change promises nothing about races inside the library itself.

## What Changes

- `Start` returns `nil` only after it has observed readiness, and readiness means the upstream server has registered its listener. The observable point is the first `Accept` on a listener wrapper that `serve()` passes to `srv.Serve`.
- One teardown routine, run at most once and serialised by the mutex `Stop` holds, closes the listener first, waits for the serve goroutine to reach its first `Accept` or exit (at most `ShutdownTimeout`) and only then calls upstream `Shutdown`. On a timeout it skips upstream `Shutdown`, so no `WaitGroup` `Add` can race a `Wait`; the closed listener still ends the serve loop at its first `Accept`. `Stop` and every failure return of `Start` use that routine.
- Every failure return of `Start` (host key, listen, `NewServer`, startup timeout or cancelled `ctx`, an error or a closed error channel) goes through one helper that never returns `nil`. When a concurrent `Stop` already stopped the server and recorded no failure, the helper returns `errStoppedDuringStartup`. After a startup timeout or a cancelled `ctx`, `Start` returns an error wrapping the context error with the listener closed, and a later `Stop` and `Wait()` return. A `Start` that finds the server already stopped (a `Stop` on a never-started server, before or racing a `Start` whose `ctx` is already cancelled, which serverbase lets through without a transition) returns that error before it binds anything or launches a goroutine.
- `serve()` signals on every exit path, including the early return when the server or listener is missing and an upstream `Serve` that fails before its first `Accept`. It receives that signal as an argument, so a teardown that has already cleared the server's fields cannot leave it without one, and an ordinary Start/Stop overlap never waits for `ShutdownTimeout`.
- `Start` publishes the listener, the upstream server and its goroutines in one critical section that a teardown already begun refuses: a `Stop` during `Start` never reaches our `WaitForShutdown` before an `AddGoroutine`, and the listener a refused `Start` opened is closed.
- Concurrent `Stop` calls are serialised, so a `Stop` that loses the stop transition waits only after the winning one has returned, never on our WaitGroup while `Start` can still add to it.
- The godoc of the public `Wait()` states that it must not be called concurrently with `Start` (no caller does).
- A code comment at the workaround cites https://github.com/charmbracelet/ssh/issues/61 and says to remove the workaround once upstream fixes it.
- Regression tests: deterministic window-holding tests through an unexported struct field, whose behavioural negative control (run with `-race`) fails on every run on 36f85fd6; Stop-during-Start cases; startup-timeout and cancelled-context teardown; behavioural assertions that need no race detector; and a short bounded Start/Stop stress loop that asserts invariants, also run on the base as a recorded race-detector supplement.
- `.agents/skills/server/SKILL.md` records the readiness rule for servers whose upstream registers listeners asynchronously.

## Capabilities

### New Capabilities

- `host-callback-server-lifecycle`: start readiness, stop ordering, startup-failure teardown and stop-during-start behaviour of the host-callback SSH server in `internal/sshserver`.

### Modified Capabilities

None.

## Impact

- Code: `internal/sshserver/server_lifecycle.go` (`Start`, `Stop`, `doStop`, `serve`, `Wait` godoc, a listener wrapper, a serve gate, the teardown routine and the start-failure helper), `internal/sshserver/server.go` (`Server` fields), new tests in `internal/sshserver`.
- Not touched: `internal/core/serverbase`, `internal/tuiserver` (uses `net/http`, which handles Shutdown-before-Serve), `internal/discovery`, dependencies (`go.mod` unchanged).
- Behaviour: `Start` may return a few microseconds later (after upstream `Serve` reaches `Accept`). `Start` returns an error, not `nil`, when a concurrent `Stop` wins before readiness. A `Start` that fails on its startup timeout or a cancelled `ctx` may now take up to `ShutdownTimeout` longer, because it tears the server down before it returns.
- Callers of `Start` outside `internal/sshserver`: only `HostAccess.Ensure` (`internal/app/commandadapters/host_access.go:44-73`). It calls `Start` and `Stop` under its own mutex, so they never overlap, and it treats any non-nil result as a failure and keeps no instance. It does not rely on `nil` for a server stopped during startup and needs no change; the startup teardown also stops it leaking a listener when `Start` times out.
- Formal: `internal/sshserver/**` is a formal CI path (`formal/manifest.toml:57`, trace `HostCallbackToken` at `:2157`). One correspondence row names a touched symbol, `Server.Stop` (`formal/tla/HostCallbackToken.tla:33`), as characterisation only (F11: `Stop` closes the listener; open connections stay open), which this change keeps. No mutation plan covers it (`formal_mutation.py symbols` reports `Start`, `Stop`, `doStop`, `serve` as unplanned), so no ledger refresh is planned; `make formal` and the correspondence check still run.
- Not breaking.
