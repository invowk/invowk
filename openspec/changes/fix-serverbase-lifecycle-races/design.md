## Context

The model splits each transition into its CAS, `stateMu`, and `cancel()` steps. F1 needed a stop between the Starting CAS and the context store; F2 needed a lock-free CAS between a locked read and an unconditional Store.

## Decisions

- **F1: store under the same lock as the CAS.** `TransitionToStopping` still CASes lock-free, then takes `stateMu` to read `cancel`. Because the start holds `stateMu` across its CAS and store, the stopper either CASes first (the start's CAS then fails) or reads the stored `cancel`. *Alternative:* re-check the state after storing and cancel if a stop won; rejected as a second code path for the same guarantee.
- **F2: CAS from the observed state and re-read.** Giving up after a failed CAS would drop a legitimate failure after a Starting→Stopping change; re-reading lets `Failed` replace a non-terminal `Stopping` while never replacing a terminal state. The model was updated to this retry semantics before the code changed.
- **Defense in depth:** with F2 fixed only one caller wins a terminal transition, so the error channel's `sync.Once` is no longer load-bearing; the model's close-twice mutant now models a path that closes twice without it.

## Risks / Trade-offs

- F1 and F2 need interleavings inside one transition, so Go tests cannot replay them without a new seam; the TLA+ model and its pre-fix mutants are the regression guard.
