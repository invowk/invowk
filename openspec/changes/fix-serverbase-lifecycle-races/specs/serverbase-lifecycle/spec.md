## ADDED Requirements

### Requirement: Terminal states are absorbing
Once a `serverbase.Base` reaches `Stopped` or `Failed`, no transition SHALL change its state, including transitions racing with the lock-free `TransitionToStopping`.

#### Scenario: Fail races a stop of a never-started server
- **WHEN** `TransitionToFailed` observes `Created` under `stateMu` while `TransitionToStopping` compare-and-swaps `Created` to `Stopped`
- **THEN** `TransitionToFailed` SHALL re-read the state, find it terminal, and return the recorded error without changing the state

#### Scenario: Model check
- **WHEN** `make formal` runs
- **THEN** `Serverbase.terminalAbsorbingFixed` SHALL pass and `Serverbase.mutantPreFixF2BlindStore` SHALL produce a counterexample

### Requirement: A winning stop cancels the server context
If `TransitionToStopping` returns true, the server context created by `TransitionToStarting` SHALL be cancelled once every caller has returned.

#### Scenario: Stop races start
- **WHEN** `TransitionToStopping` compare-and-swaps `Starting` to `Stopping` while `TransitionToStarting` is still running
- **THEN** it SHALL read the stored cancel function under `stateMu` and call it

#### Scenario: Model check
- **WHEN** `make formal` runs
- **THEN** `Serverbase.stopCancelsContextFixed` SHALL pass and `Serverbase.mutantPreFixF1UnlockedStart` SHALL produce a counterexample
