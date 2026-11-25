# Spec Delta

## ADDED Requirements

### Requirement: Stop closes every connection
`Stop` SHALL close every connection the server accepted, whether or not it has authenticated, and SHALL accept no new connection once it has begun; a login that completes after `Stop` closed its connection SHALL NOT be admitted. When `Stop` returns, no SSH client SHALL still be able to use a connection to the server.

#### Scenario: Authenticated connection open at Stop
- **GIVEN** a running server and an SSH client that logged in with a valid token and keeps its connection open
- **WHEN** `Stop` is called
- **THEN** `Stop` SHALL return `nil` well inside `ShutdownTimeout`, the client SHALL see its connection closed, and opening a new session on it SHALL fail

#### Scenario: Revocation and Stop together
- **GIVEN** a running server and an authenticated client
- **WHEN** the client's token is revoked and `Stop` runs concurrently
- **THEN** the connection SHALL be closed exactly once, both calls SHALL return, and `-race` SHALL report no data race

### Requirement: Stop waits for connection handling, bounded by the shutdown timeout
`Stop` SHALL return only after the server's handling of every accepted connection has ended, bounded by `ShutdownTimeout`. A connection that a client keeps open SHALL NOT make `Stop` wait, since `Stop` closes it; `Stop` SHALL then return `nil`. When the handling of a closed connection has not ended at `ShutdownTimeout`, for example because a login check has not returned, `Stop` SHALL return an error.

#### Scenario: Connection still authenticating at Stop
- **GIVEN** a running server and an SSH client whose authentication is held inside the server's password check
- **WHEN** `Stop` runs, and the hold is released while `Stop` is still waiting, after `Stop` has closed the connections
- **THEN** the client's login SHALL fail, the server SHALL track no connection for it, `Stop` SHALL return `nil` well inside `ShutdownTimeout`, and `-race` SHALL report no data race

#### Scenario: Login check that does not return before the shutdown timeout
- **GIVEN** a running server and an SSH client whose authentication is held inside the server's password check
- **WHEN** `Stop` runs and the hold is released only after `ShutdownTimeout`
- **THEN** `Stop` SHALL return an error at `ShutdownTimeout`, the client's connection SHALL be closed, and after the release the login SHALL fail

## MODIFIED Requirements

### Requirement: Stop shuts the SSH library down only after its serve loop has registered the listener or exited
`Stop` SHALL close the listener first and SHALL shut the SSH library's server down only after the serve loop has registered the listener or the serve goroutine has exited. That wait SHALL be bounded by the configured `ShutdownTimeout`; when it expires, `Stop` SHALL skip the library's shutdown and SHALL return an error. The library's shutdown SHALL be one that does not wait for the library's own connection tracking, so no handshake that completes during `Stop` races it.

#### Scenario: Stop while the serve loop has not started
- **GIVEN** the serve goroutine is held before the SSH library's serve loop
- **WHEN** a concurrent `Stop` runs and the hold is then released
- **THEN** `Stop` SHALL return only after the serve goroutine has exited, the serve loop SHALL NOT keep serving, and `-race` SHALL report no data race

#### Scenario: Serve loop never reached within the shutdown timeout
- **GIVEN** the serve goroutine is held before the serve loop for longer than `ShutdownTimeout`
- **WHEN** `Stop` runs
- **THEN** `Stop` SHALL stop waiting at `ShutdownTimeout`, SHALL NOT call the library's shutdown, and SHALL return an error after the serve goroutine has exited

#### Scenario: Serve loop fails before accepting
- **WHEN** the SSH library's serve call returns an error before it accepts any connection, or the serve goroutine returns early because the server or listener is missing, including when a concurrent `Stop` took them before the serve goroutine read them
- **THEN** a concurrent or later `Stop` SHALL NOT wait for the shutdown timeout on its account

### Requirement: Stop during Start leaves nothing running
A `Stop` that runs at any point of a concurrent `Start` SHALL leave no listener open and no server goroutine running once both calls have returned. Every `Stop` call, including one that loses to a concurrent `Stop`, SHALL return only after every goroutine that `Start` launched has exited. These guarantees SHALL hold whatever further `Start` calls reach the server, since a `Start` on a server that is not in Created changes nothing (capability `server-lifecycle-base`). A `Start` that a concurrent `Stop` overtook SHALL return a non-nil error.

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

#### Scenario: Second Start with a cancelled context during the first Start
- **GIVEN** a `Start` is still in progress
- **WHEN** a second `Start` with an already-cancelled context returns, and `Stop` then runs
- **THEN** the second `Start` SHALL return a non-nil error without binding or tearing anything down, and `Stop` SHALL return only after every goroutine that the first `Start` launched has exited, leaving no listener open

#### Scenario: Port after Stop
- **WHEN** `Stop` has returned and so has any `Start` that overlapped it
- **THEN** a TCP connection to the server's bound port SHALL be refused, with no race detector needed to observe it
