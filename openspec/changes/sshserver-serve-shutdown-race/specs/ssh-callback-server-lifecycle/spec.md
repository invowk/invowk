# Spec Delta

## Purpose

Defines when the host-callback SSH server counts as started, how it stops, and what happens when a stop arrives while a start is still in progress, so that start, stop and their overlap never leak a listener or a goroutine and never race inside the SSH library.

## ADDED Requirements

### Requirement: Start reports ready only once the SSH library serves the listener
`Start` SHALL return `nil` only after the SSH library's serve loop has registered the server's listener and is about to accept its first connection. Readiness SHALL NOT be reported from setup steps that precede the serve loop.

#### Scenario: Stop right after Start
- **WHEN** `Start` returns `nil` and the caller calls `Stop` immediately
- **THEN** `Stop` SHALL return after the serve loop has exited, with no data race reported under `-race`

#### Scenario: Serve loop delayed after setup
- **GIVEN** the serve goroutine is held after reading the server and listener and before entering the SSH library's serve loop
- **WHEN** `Start` is waiting for readiness
- **THEN** `Start` SHALL NOT return `nil` while the serve goroutine is held

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
- **WHEN** the SSH library's serve call returns an error before it accepts any connection, or the serve goroutine returns early because the server or listener is missing
- **THEN** a concurrent or later `Stop` SHALL NOT wait for the shutdown timeout on its account

### Requirement: Stop during Start leaves nothing running
A `Stop` that runs at any point of a concurrent `Start` SHALL leave no listener open and no server goroutine running once both calls have returned, and SHALL wait for every goroutine that `Start` launched before it returns.

#### Scenario: Stop before Start has launched its goroutines
- **WHEN** `Stop` completes its shutdown while `Start` is still configuring the host key, binding the listener or creating the SSH server
- **THEN** `Start` SHALL launch no server goroutine, SHALL close the listener it bound, and SHALL return a non-nil error

#### Scenario: Stop after Start has launched its goroutines but before readiness
- **WHEN** `Stop` runs after `Start` has launched the serve goroutine and before readiness is reported
- **THEN** `Stop` SHALL return only after that goroutine has exited, and `Start` SHALL return a non-nil error

#### Scenario: Port after Stop
- **WHEN** `Stop` has returned, whether or not a `Start` overlapped it
- **THEN** a TCP connection to the server's bound port SHALL be refused, with no race detector needed to observe it

### Requirement: Repeated start and stop stays clean
Short-lived servers that are started and stopped back to back SHALL each stop cleanly, without a data race, a leaked goroutine or a `Stop` that waits for the shutdown timeout.

#### Scenario: Bounded start/stop loop
- **WHEN** a test starts and immediately stops a fresh server a bounded number of times under `-race`
- **THEN** every `Start` SHALL return `nil`, every `Stop` SHALL return `nil` well inside `ShutdownTimeout`, and no data race SHALL be reported
