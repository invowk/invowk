# Spec Delta

## Purpose

Defines when the host-callback SSH server counts as started, how it stops, how a failed start is torn down, and what happens when a stop arrives while a start is still in progress. Start, stop, a failed start and their overlap never leak a listener or a goroutine, and the server never shuts the SSH library down before the library has registered its listener (the listener-registration race of charm.land/ssh v0.4.3, https://github.com/charmbracelet/ssh/issues/61). Races inside the SSH library itself are out of scope: the connection-registration race between a handshake that completes while `Stop` runs and the library's graceful shutdown (https://github.com/gliderlabs/ssh/issues/117) stays a known upstream residual, because closing it would drop the graceful shutdown that keeps authenticated connections open.

## ADDED Requirements

### Requirement: Start reports ready only once the SSH library serves the listener
`Start` SHALL return `nil` only when it has observed readiness, and readiness SHALL mean that the SSH library's serve loop has registered the server's listener and is about to accept its first connection. Readiness SHALL NOT be reported from setup steps that precede the serve loop. Every other return of `Start` SHALL be a non-nil error.

#### Scenario: Stop right after Start
- **WHEN** `Start` returns `nil` and the caller calls `Stop` immediately
- **THEN** `Stop` SHALL return after the serve loop has exited, with no data race reported under `-race`

#### Scenario: Serve loop delayed after setup
- **GIVEN** the serve goroutine is held after reading the server and listener and before entering the SSH library's serve loop
- **WHEN** `Start` is waiting for readiness
- **THEN** `Start` SHALL NOT return while the serve goroutine is held, and SHALL return `nil` once it is released and the serve loop is reached

### Requirement: Stop shuts the SSH library down only after its serve loop has registered the listener or exited
`Stop` SHALL close the listener first and SHALL ask the SSH library for its graceful shutdown only after the serve loop has registered the listener or the serve goroutine has exited. That wait SHALL be bounded by the configured `ShutdownTimeout`; when it expires, `Stop` SHALL skip the graceful shutdown and SHALL return an error.

#### Scenario: Stop while the serve loop has not started
- **GIVEN** the serve goroutine is held before the SSH library's serve loop
- **WHEN** a concurrent `Stop` runs and the hold is then released
- **THEN** `Stop` SHALL return only after the serve goroutine has exited, the serve loop SHALL NOT keep serving, and `-race` SHALL report no data race

#### Scenario: Serve loop never reached within the shutdown timeout
- **GIVEN** the serve goroutine is held before the serve loop for longer than `ShutdownTimeout`
- **WHEN** `Stop` runs
- **THEN** `Stop` SHALL stop waiting at `ShutdownTimeout`, SHALL NOT call the graceful shutdown, and SHALL return an error after the serve goroutine has exited

#### Scenario: Serve loop fails before accepting
- **WHEN** the SSH library's serve call returns an error before it accepts any connection, or the serve goroutine returns early because the server or listener is missing, including when a concurrent `Stop` took them before the serve goroutine read them
- **THEN** a concurrent or later `Stop` SHALL NOT wait for the shutdown timeout on its account

### Requirement: A failed Start leaves nothing serving
When `Start` fails after it has bound a listener, on its startup timeout, a cancelled context or any other path, it SHALL close the listener and tear the server down through the same shutdown ordering as `Stop` before it returns, and the teardown SHALL run at most once for a server whatever mix of `Start` failures and `Stop` calls reaches it. A later `Stop` and `Wait` SHALL return.

#### Scenario: Startup timeout while the serve loop is held
- **GIVEN** the serve goroutine is held before the serve loop for longer than `StartupTimeout`
- **WHEN** `Start` runs
- **THEN** `Start` SHALL return an error wrapping the context deadline error, a TCP connection to the bound port SHALL be refused, and a later `Stop` and `Wait` SHALL each return within a bounded time

#### Scenario: Caller cancels the context while the serve loop is held
- **GIVEN** the serve goroutine is held before the serve loop
- **WHEN** the caller cancels the context passed to `Start`
- **THEN** `Start` SHALL return an error wrapping the context cancellation error, a TCP connection to the bound port SHALL be refused, and a later `Stop` and `Wait` SHALL each return within a bounded time

### Requirement: Stop during Start leaves nothing running
A `Stop` that runs at any point of a concurrent `Start` SHALL leave no listener open and no server goroutine running once both calls have returned. Every `Stop` call, including one that loses to a concurrent `Stop`, SHALL return only after every goroutine that `Start` launched has exited. These guarantees cover a server started at most once; a second `Start` with an already-cancelled context is outside them. A `Start` that a concurrent `Stop` overtook SHALL return a non-nil error.

#### Scenario: Stop before Start has launched its goroutines
- **WHEN** `Stop` begins its shutdown while `Start` is still configuring the host key, binding the listener or creating the SSH server
- **THEN** `Start` SHALL launch no server goroutine, SHALL close the listener it bound, and SHALL return a non-nil error

#### Scenario: Stop after Start has launched its goroutines but before readiness
- **WHEN** `Stop` runs after `Start` has launched the serve goroutine and before readiness is reported
- **THEN** `Stop` SHALL return only after that goroutine has exited, and `Start` SHALL return a non-nil error

#### Scenario: Two Stop calls during Start
- **WHEN** two `Stop` calls run concurrently while `Start` is still in progress
- **THEN** both SHALL return only after every goroutine that `Start` launched has exited, and neither SHALL wait on the server's goroutines while `Start` can still launch one

#### Scenario: Start with a cancelled context on a server a Stop already stopped
- **GIVEN** `Stop` runs on a server that was never started, before or concurrently with a `Start` whose context is already cancelled
- **WHEN** `Start` finds the server already stopped
- **THEN** `Start` SHALL bind no listener and launch no server goroutine, SHALL return a non-nil error, and `Stop` and a later `Wait` SHALL each return within a bounded time

#### Scenario: Port after Stop
- **WHEN** `Stop` has returned and so has any `Start` that overlapped it
- **THEN** a TCP connection to the server's bound port SHALL be refused, with no race detector needed to observe it

### Requirement: Wait is called only after Start has returned
The documented contract of `Wait` SHALL state that it is called only after `Start` has returned, or without any `Start`. Within that contract `Wait` SHALL return once every goroutine that `Start` launched has exited.

#### Scenario: Wait after a Stop during Start
- **WHEN** `Stop` and an overlapping `Start` have both returned and `Wait` is then called
- **THEN** `Wait` SHALL return within a bounded time

### Requirement: Repeated start and stop stays clean
Short-lived servers that are started and stopped back to back, or stopped while starting, SHALL each stop cleanly: without a data race, with `Wait` returning once `Stop` has, and without a `Stop` that waits for the shutdown timeout.

#### Scenario: Bounded start/stop loop
- **WHEN** a test starts and immediately stops a fresh server a bounded number of times under `-race`
- **THEN** every `Start` SHALL return `nil`, every `Stop` SHALL return `nil` well inside `ShutdownTimeout`, and no data race SHALL be reported

#### Scenario: Bounded loop of Stop racing Start
- **WHEN** a test starts a fresh server in a goroutine and stops it concurrently, a bounded number of times under `-race`
- **THEN** after both calls return, `Start` SHALL have returned `nil` only if the server reached the running state, any error SHALL be acceptable when it never did, every `Stop` SHALL return `nil` well inside `ShutdownTimeout`, `Wait` SHALL return within a bounded time, no server goroutine SHALL be left, a TCP connection to the port SHALL be refused whenever a listener was bound, and no data race SHALL be reported
