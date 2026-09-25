## Why

The TLA+ model of `internal/core/serverbase` (`formal/tla/Serverbase.tla`, from `adopt-formal-verification`) produced two counterexamples of the current code:

- **F1:** `TransitionToStarting` CASed Created→Starting before storing `ctx`/`cancel` under `stateMu`. A concurrent `TransitionToStopping` in that window read a nil `cancel`, returned true without cancelling, and the context created afterwards was never cancelled by the stop.
- **F2:** `TransitionToFailed` and `TransitionToStopped` checked for a terminal state under `stateMu` and then `Store`d unconditionally. A lock-free Created→Stopped CAS by `TransitionToStopping` in between was overwritten: Stopped became Failed, contradicting the package contract that terminal states are irreversible.

Current callers serialise Start and Stop (`HostAccess.Ensure`, the interactive TUI path), so neither is reachable today, but both are contract bugs any new caller could hit.

## What Changes

- `TransitionToStarting` performs the CAS and the context store in one `stateMu` critical section.
- `TransitionToFailed` and `TransitionToStopped` move to a terminal state through `casToTerminalLocked`, which compare-and-swaps from the observed state and re-reads on failure, never overwriting a terminal state.
- The Serverbase model's base configuration now mirrors the fixed code; the pre-fix code is kept as two regression mutants. The server skill states the guarantee.

## Capabilities

### New Capabilities

- `serverbase-lifecycle`: terminal states are absorbing and a winning stop always cancels the server context, under concurrent transitions.

### Modified Capabilities

(none)

## Impact

- `internal/core/serverbase/base.go`; `formal/tla/Serverbase.tla`, `formal/manifest.toml`, `formal/README.md`; `.agents/skills/server/SKILL.md`.
- No behaviour change for serialised callers.
