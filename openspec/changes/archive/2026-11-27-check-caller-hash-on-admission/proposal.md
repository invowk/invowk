## Why

Finding F6 of `adopt-formal-verification`: command-scope admission (`IsDeclaredLockedCommandSource`) matched the caller's lock entry by module identity and command namespace only. When the discovered copy of a dependency is a sibling module's vendored version, it is verified only against the sibling's lock. So a different version than the caller locked was admitted under the caller's lock. The LockIntegrity TLA+ model reproduced it, and a Go replay confirmed it.

## What Changes

- **BREAKING:** when the caller's lock entry records a content hash, admission also requires the directory the dependency's commands were discovered in to hash to it. A mismatching copy is a forbidden command dependency.
- The denial message explains that a declared dependency can fail because the discovered copy is another version.
- The LockIntegrity model's configuration already mirrors the fix. The pre-fix behaviour becomes the regression mutant `mutantPreFixF6AdmitByIdentity`, and the Go replay test is inverted.
- Hashless (v1.0) entries keep admitting by identity; rejecting them is the separate `reject-hashless-lock-entries` change (F4).

## Capabilities

### New Capabilities

- `command-scope-content-admission`: direct-dependency admission compares the caller's locked content hash.

### Modified Capabilities

(none)

## Impact

- `pkg/invowkmod/vendored_policy.go` (new `modulePath` parameter), `internal/app/deps/deps.go`, tests; command dependency docs (en, pt-BR) and README; `formal/`.
- Modules that call a dependency whose discovered copy is not the version they locked now fail `depends_on.cmds` validation until the versions agree.
