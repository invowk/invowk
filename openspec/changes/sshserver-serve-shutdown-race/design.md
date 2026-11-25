# Design

## Context

Motivation: proposal.md, "Why". Current behaviour, at base 36f85fd6:

- `serve()` calls `s.base.TransitionToRunning()` as its first step (`internal/sshserver/server_lifecycle.go:172`), before it reads `srv` and `listener` (`:175-178`) and before `srv.Serve(listener)` (`:184`). `Start` returns `nil` as soon as `StartedChannel()` closes (`:98-102`), so it returns before upstream `Serve` has run.
- Upstream `Serve` (`charm.land/ssh@v0.4.3/server.go:294-339`) runs `ensureHandlers`, `ensureHostSigner` (`:304`), then `trackListener(l, true)` (`:312`), which resets `doneChan` when no listener or connection is tracked (`:592-596`) and calls `listenerWg.Add(1)` (`:598`), and only then its first `l.Accept()` (`:316`), in the same goroutine.
- Upstream `Shutdown` (`server.go:268-287`) closes the tracked listeners and `doneChan` under `srv.mu`, then waits on `listenerWg` in a new goroutine (`:275-279`). A `Shutdown` that runs before `trackListener` finds nothing to wait for; `Serve` then registers the listener, resets `doneChan` and serves on, and its `Add` races the `Wait` (CI: `server.go:275` against `:312`).
- `doStop` closes our listener before `Shutdown` (`server_lifecycle.go:141-145`). That is why `Serve` does not serve on in invowk today: its first `Accept` fails on the closed listener and, with `doneChan` reset, returns the `net.ErrClosed` error, which `serve()` ignores (`:188-190`). The race on the WaitGroup remains.
- `Start` publishes `s.listener` and `s.addr` (`:62-65`) and `s.srv` (`:85-87`) in two separate critical sections and calls `AddGoroutine` outside any lock (`:90`, `:94`). `TransitionToStopping` accepts `StateStarting` (`internal/core/serverbase/base.go:174`), so a `Stop` during `Start` runs `doStop` at once. When it runs before `:90`, `doStop` takes nil resources, `WaitForShutdown` (`:158`, `base.go:218-220`) returns on a zero counter, and `Start` then calls `AddGoroutine`: an `Add` racing a `Wait` in our own WaitGroup. When it runs before `:62`, the listener `Start` stores later is never closed and the serve goroutine serves on it with nobody to stop it.
- In that case `TransitionToRunning` fails its CAS (`base.go:124-128`), `startedCh` never closes, and `Start` reads the closed error channel (`:104-106`): it receives `nil`, `TransitionToFailed(nil)` finds a terminal state and returns the recorded `nil` (`base.go:134-140`), so `Start` returns `nil` for a server that is stopped.
- `TestServerServeFailureTransitionsToFailed` (`internal/sshserver/server_test.go:409-433`) builds a `Server` literal with only `base`, `srv` and `listener` and calls `serve()` directly, without `Start`.
- The production caller (`internal/app/commandadapters/host_access.go:63-67`, `:85-95`) calls `Start` and later `Stop` from one goroutine.
- `TestHostCallbackToken_TraceHarness` runs with a `ShutdownTimeout` of at most 100 ms and tolerates `context.DeadlineExceeded` from `Stop` (`internal/sshserver/token_trace_test.go:245`).

## Goals / Non-Goals

**Goals:**

- Readiness and stop ordering that keep every upstream `listenerWg.Add` ordered before any `listenerWg.Wait`, and every `AddGoroutine` of ours ordered before our `WaitForShutdown`, for any interleaving of `Start` and `Stop`.
- A regression test that fails on 36f85fd6 under `-race`, and assertions that hold without the race detector.

**Non-Goals:**

- Changes to `internal/core/serverbase`, `internal/tuiserver`, `go.mod` or the upstream module (no fork, no replace).
- The `Start` failure paths that leave a serve goroutine running (see Open questions).

## Decisions

- **D1. Readiness at the first `Accept` of a wrapped listener.** `serve()` passes `srv.Serve` a small wrapper around the listener whose `Accept` runs a one-shot callback before it delegates. The callback calls `s.base.TransitionToRunning()` and opens the serve gate (D2). Upstream calls `trackListener` before its first `Accept` in the same goroutine (`server.go:312`, `:316`), so `Add` happens before readiness, readiness before `Start` returns, and `Start` before any `Stop` that follows it. The wrapper's `Close` and `Addr` delegate; double `Close` (ours in `doStop`, upstream's in `Shutdown` and its deferred close) stays best-effort as today.
  - *Alternative:* poll upstream state (`srv.started` is unexported; no public "serving" signal exists in v0.4.3). Rejected: nothing to poll.
  - *Alternative:* keep readiness early and only reorder `Stop`. Rejected: `Start` would still report ready before the server accepts, and the brief requires readiness to mean the listener is registered.
  - *Alternative:* serialise `Serve` and `Shutdown` with a mutex of ours. Rejected: `Serve` blocks for the server's whole life.
- **D2. A serve gate, opened on every exit path.** A small nil-safe type (a `sync.Once` and a channel) is created by `Start` and published with the server. It opens at the first `Accept` (D1) and, through a `defer` at the top of `serve()`, on every return: the early `srv == nil || listener == nil` return, an upstream `Serve` that fails before `Accept` (for example in `ensureHostSigner`), and the normal return. Its methods accept a nil receiver, so the literal construction in `server_test.go:409-433` keeps working with no gate.
- **D3. `doStop` waits for the gate before upstream `Shutdown`.** `doStop` keeps closing the listener first, takes the gate together with `srv` and `listener`, then waits for the gate with the existing `shutdownCtx` (`ShutdownTimeout`). Gate open: call `Shutdown(shutdownCtx)` as today. Timeout: log it, skip `Shutdown` and return an error wrapping `context.DeadlineExceeded`; the closed listener ends the serve loop at its first `Accept`, and `WaitForShutdown` then waits for it. A nil gate (no server published) skips the wait.
  - *Alternative:* call upstream `Close` on a timeout. Rejected: it adds nothing once the listener is closed (no connection exists before the first `Accept`, and `trackListener` resets `doneChan` anyway).
- **D4. One publish point that a started stop refuses.** `doStop` sets a `stopBegun` flag in the critical section where it takes the resources. `Start` builds the listener and the upstream server as today but publishes `listener`, `addr`, `srv`, the gate and both `AddGoroutine` calls in one `srvMu` critical section, after checking `stopBegun`. If a stop has begun, `Start` closes the listener it bound, launches nothing and returns an unexported sentinel error (`errStoppedDuringStartup`), leaving the state to `Stop`. Either the publish precedes `doStop`'s critical section, so every `Add` happens before its `Wait`, or `Start` launches nothing.
  - *Alternative:* check `base.State()` under `srvMu`. Rejected: `TransitionToStopping` changes the state lock-free (`base.go:160-191`), so a flag written by `doStop` under the same lock is the simpler argument.
  - *Alternative:* a serverbase change to make `AddGoroutine` refuse after a stop. Rejected by scope (brief, requirement 6) and not needed.
- **D5. `Start` reports a stop that wins before readiness.** When `Start` reads the closed error channel (`ok == false`) it returns `LastError()` if one is recorded, `nil` if `StartedChannel()` is already closed (the server was ready, then stopped), and `errStoppedDuringStartup` otherwise, instead of calling `TransitionToFailed(nil)`. The state stays as `Stop` set it.
- **D6. Test seam: one unexported struct field.** `Server` gets `hooks *lifecycleHooks`, nil in production, with `beforePublish func(net.Addr)` (called in `Start` after the listener is bound and before the publish point, with the bound address) and `beforeServe func()` (called in `serve()` after it has read `srv`, `listener` and the gate, before `srv.Serve`). Tests set it on a `Server` from `mustNew` before `Start`; the `go` statement orders the write before the reads. No package-level variable.
- **D7. Regression test shape.** `beforeServe` signals that it was reached, waits until `State()` is `Stopping` (the stop has begun), then sleeps about 20 ms so that upstream `Shutdown` runs first on the old code. `Start` runs in a goroutine; the test calls `Stop` once the hook was reached. On 36f85fd6 (with the one-line hook call added in a throwaway worktree) `-race` reports the `server.go:275` / `:312` race; on the fix the gate orders `Add` before `Wait` whatever the timing. The sleep only widens the window for the negative control; the fixed code's result does not depend on it.
- **D8. Upstream reference.** One comment, at the readiness wrapper, names the defect by behaviour (charm.land/ssh v0.4.3: a `Shutdown` before `Serve` registers its listener returns at once and `Serve` serves on, racing its listener WaitGroup) and says that D1-D3 can be simplified back once upstream fixes it. No TODO marker; the maintainer adds the issue URL in a later revise.
- **D9. Skill doc.** `.agents/skills/server/SKILL.md`, Start rule 4, gains: when a library's `Serve` registers its listener itself, readiness means the library has registered it (signal from the first `Accept` of a wrapped listener), and `Stop` must not call the library's `Shutdown` before that point or the serve goroutine's exit. The generic example stays (it fits `net/http`).
- **D10. Formal.** `formal_mutation.py symbols` against a fresh plan reports `Start`, `Stop`, `doStop` and `serve` as unplanned (no bound correspondence row); the bound files in this package are `server_auth.go` and `server_conns.go`, which this change does not touch. No ledger refresh is planned; the implement stage re-runs `symbols` on every touched symbol, and `make formal-reach` plus the formal CI path (`formal/manifest.toml:57`) still run.

## Risks / Trade-offs

- [Upstream reorders `trackListener` and `Accept` in a later version] → D1 then signals too early. Mitigation: the D8 comment names v0.4.3 and the ordering it relies on; the regression test runs under `-race` in CI and fails again if the ordering breaks.
- [`Start` returns slightly later] → it now waits for `ensureHostSigner` and `trackListener`, microseconds with a configured host signer; `StartupTimeout` (default 5 s) bounds it.
- [Behaviour change: `Start` returns an error instead of `nil` when a concurrent `Stop` wins] → no production caller overlaps the two (`host_access.go:63-95`); the error is truthful.
- [The regression test's window depends on a sleep] → only the negative control depends on it; the fixed code's outcome does not. The negative control runs with `-count` high enough to show the failure reliably, and its output is recorded in tasks.md.
- [A `beforeServe` hold longer than `ShutdownTimeout`] → `Stop` returns a timeout error after the serve goroutine exits; covered by a test.

## Migration Plan

None: internal package, no config or schema change. Rollback is a revert.

## Maintainer decisions

- 2026-11-25 (brief `program-notes/briefs/sshserver-race-brief.md`, decision 117): readiness = upstream has registered its listener, observed at the first `Accept`; `Stop` never calls upstream `Shutdown` before that point or the serve goroutine's exit, bounded by `ShutdownTimeout`; scope frozen to `internal/sshserver` (plus the server skill doc); the upstream issue URL is added by the maintainer session later.

## Open questions

- **Startup timeout and cancelled `ctx` leave the server serving.** When `startupCtx` ends (`server_lifecycle.go:108-110`), `Start` calls `TransitionToFailed`, which cancels the base context but closes neither the listener nor the upstream server; the serve goroutine keeps serving, and a later `Stop` sees `Failed`, skips `doStop` and blocks in `WaitForShutdown` (`:118-121`). Pre-existing and outside the frozen scope; it does not change this change's specs or tasks. Listed for the maintainer: fix it in a follow-up (tear down through the same gate) or here in a revise.
