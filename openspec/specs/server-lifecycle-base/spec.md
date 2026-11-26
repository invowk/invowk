# server-lifecycle-base Specification

## Purpose
Defines the lifecycle rules that the shared server base gives every long-running invowk server (the SSH host-callback server and the TUI server), in particular which `Start` calls may change a server's state.

## Requirements

### Requirement: Only a Start on a never-started server changes its state
A `Start` SHALL change a server's lifecycle state only when the server is in Created, the state of a server that was never started or stopped. A `Start` on a server in any other state (Starting, Running, Stopping, Stopped or Failed) SHALL return a non-nil error that names the current state and SHALL leave the state, the recorded failure and the server's context unchanged, whether or not its own context is already cancelled.

#### Scenario: Second Start with a cancelled context on a running server
- **GIVEN** a server whose first `Start` returned `nil`, so it is Running
- **WHEN** a second `Start` is called with an already-cancelled context
- **THEN** that `Start` SHALL return a non-nil error naming the Running state, the server SHALL still be Running and serving, and a later `Stop` SHALL stop it as it would any running server

#### Scenario: Second Start with a cancelled context while the first is still starting
- **GIVEN** a server whose first `Start` is still in progress, so it is Starting
- **WHEN** a second `Start` is called with an already-cancelled context
- **THEN** that `Start` SHALL return a non-nil error, the server SHALL NOT be marked Failed by it, and the first `Start` SHALL finish as if the second had not been called

#### Scenario: Start with a cancelled context on a stopped server
- **GIVEN** a server that `Stop` moved from Created to Stopped
- **WHEN** `Start` is called with an already-cancelled context
- **THEN** `Start` SHALL return a non-nil error, never `nil`, and the server SHALL stay Stopped

#### Scenario: Start with a cancelled context racing a Stop of a never-started server
- **GIVEN** a server in Created
- **WHEN** `Start` with an already-cancelled context and `Stop` run concurrently
- **THEN** the server SHALL end Failed or Stopped and SHALL NOT move from one to the other, `Start` SHALL return a non-nil error either way, and it SHALL bind no listener and launch no server goroutine

#### Scenario: First Start with a cancelled context
- **GIVEN** a server in Created
- **WHEN** `Start` is called with an already-cancelled context
- **THEN** `Start` SHALL return an error wrapping the context error and the server SHALL be Failed, as before this change

#### Scenario: Both servers follow the rule
- **WHEN** the scenarios above run against the SSH host-callback server and against the TUI server
- **THEN** both SHALL behave as described
