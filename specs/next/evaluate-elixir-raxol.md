# Evaluation: Elixir + Raxol as Invowk Runtime

**Status**: Research complete — not recommended
**Date**: 2026-03-08

## Context

Evaluate whether Elixir (with the Raxol TUI library) would be a good fit for
Invowk's architecture, assuming CUE would run via WASM.

## Raxol Overview

Raxol is an Elixir TUI framework ("The Next.js of Terminal UIs") offering
React-style components, Elm Architecture state management, 60 FPS animations,
and Phoenix LiveView integration. Modular since v2.0: `raxol_core`, `raxol_liveview`,
`raxol_plugin`.

**Maturity**: Early-stage. 35 GitHub stars, 0 downstream dependents, single author,
~6 months old, 9 downloads/week on Hex. Claims of enterprise features (SAML/OIDC,
CRDT collaboration) from a solo project in that timeframe warrant skepticism.

## Where BEAM/Elixir Would Genuinely Excel

### 1. The TUI Server Bridge Is a Hand-Rolled Actor System

The `tuiserver` → `BridgeTUIRequests` → `interactiveModel.Update` pipeline
(HTTP handler → channel send → goroutine → `p.Send()` → Bubbletea mailbox →
Update dispatch → component lifecycle → ResponseCh reply) is literally an actor
receiving a message and sending a reply. In OTP, this entire flow becomes a single
`gen_server:call/2`. The `shutdownCh` + `sync.Once` + `mu.Lock()` + `responseCh` +
`select` pattern disappears.

### 2. Server State Machines Are Solved in OTP

`serverbase.Base` (233 lines) implements lifecycle management with `atomic.Int32`,
`sync.Mutex`, CAS-retry loops, `sync.WaitGroup`, `chan struct{}` for ready signaling,
and `chan error` for async errors. Both `sshserver` and `tuiserver` embed this. In OTP,
this is `gen_statem` or a `GenServer` with state — lifecycle, supervision, error
propagation, and graceful shutdown are framework-provided.

### 3. Supervision Trees Eliminate Shutdown Choreography

The shutdown ordering (`defer tuiServer.Stop()` interacting with `BridgeTUIRequests`
goroutine lifetime interacting with HTTP handlers blocked on `ResponseCh`) is the
hardest code to write correctly. OTP supervision trees with `one_for_all` strategy
handle child shutdown ordering inherently.

### 4. Watch System Debounce Becomes Trivial

The `time.AfterFunc` + `wg.Add(1)` TOCTOU avoidance + `timer.Reset()` return checks +
`running` atomic skip-if-busy pattern becomes `Process.send_after/3` with
`Process.cancel_timer/1`. Single-process mailbox ordering eliminates the races.

### 5. Error Isolation via Process Boundaries

A panic in the PTY reader goroutine crashes the entire process. In BEAM, each actor
is a separate process — a crash in one doesn't take down others. Supervisors restart
just the failed component.

## Where BEAM/Elixir Breaks Down

### 1. Startup Time (Dealbreaker)

BEAM boot: ~150ms minimum. Go with PGO: ~5-15ms. Invowk is a CLI tool that runs on
every developer keystroke. This is structural and unsolvable.

### 2. Single Binary Distribution

Go: `CGO_ENABLED=0`, one file, 10-15MB (UPX compressed). Elixir: Burrito/Bakeware
release, 20-50MB, slower startup, not a single copyable file. Breaks the container
provisioning model (ephemeral layer with `COPY` of binary).

### 3. No Virtual Shell Equivalent

`mvdan/sh` (pure-Go POSIX shell interpreter running in-process) has no Elixir
equivalent. Entire virtual runtime tier would be lost.

### 4. PTY Management Is Immature on BEAM

Invowk does deep PTY work: allocation, I/O bridging, window resize forwarding,
platform-specific handling. Go ecosystem (`creack/pty`, `xpty`) is mature. Erlang
ports are coarser-grained.

### 5. CUE via WASM Adds Latency

CUE parsing is already the dominant startup cost as a native Go library. WASM bridge
adds serialization overhead, cold module instantiation (~10-50ms), and loss of type-safe
CUE Value API.

### 6. Windows Support Degrades

Raxol falls back to "pure Elixir driver" on Windows at 10x worse frame time (500us vs
50us). Elixir's Windows story is weaker overall.

## Concurrency Architecture Map

Interactive execution spawns 7+ concurrent actors:

- Bubbletea event loop (main)
- PTY reader goroutine → outputMsg
- Command wait goroutine → doneMsg
- TUI bridge goroutine (BridgeTUIRequests) → TUIComponentMsg
- TUI HTTP server goroutine
- SSH server goroutine (container runtime)
- Token cleanup goroutine (SSH)
- Watch system goroutine (--ivk-watch)

Synchronization primitives: 5 mutexes, 3 atomics, 6+ channels, 2 WaitGroups,
2 sync.Once, 1 flock.

**Approximately 2,000 lines of server/bridge code out of ~30K+ total** would benefit
from OTP. The remaining 93% (CLI, discovery, CUE parsing, runtime dispatch, container
provisioning) is CLI-shaped, not server-shaped.

## Recommendation

**Not recommended** as a full rewrite target. The BEAM concurrency advantages are real
but localized to ~7% of the codebase. Trading Go's structural CLI advantages (startup
time, single binary, PTY ecosystem, CUE native library, virtual shell) for OTP's
concurrency model would be optimizing the minority at the cost of the majority.

### Alternative Worth Exploring

A **hybrid architecture** where the interactive/server subsystem is a separate Elixir
process that the Go CLI spawns and communicates with via a Unix socket or stdio protocol.
This would get OTP supervision for the concurrent parts and Go's CLI advantages for
everything else. Similar to how `rust-analyzer` separates its server from editor clients.

### If Exploring Language Alternatives

Rust (already has `/rust-alt` skill) addresses the same performance/distribution
requirements while offering richer type-level guarantees. For TUI, Ratatui is far more
mature than Raxol (or any Elixir TUI library).
