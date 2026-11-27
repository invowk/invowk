## Why

Finding F4 of `adopt-formal-verification`: discovery still accepts v1.0 lock files, whose entries record no content hash. `VerifyLockedVendoredModuleHash` returned nil for such an entry, and `module vendor` skipped verification, so a lock rewritten as v1.0 without hashes turned off tamper detection for vendored modules. The LockIntegrity TLA+ model reproduced it, and a Go replay confirmed it.

## What Changes

- **BREAKING:** discovery rejects a vendored module whose lock entry has no content hash, with `LockEntryWithoutContentHashError` telling the user to run `invowk module sync` to upgrade the lock to v2.0.
- **BREAKING:** `invowk module vendor` refuses to vendor through a hashless entry.
- Sync no longer trusts an existing cached copy when the locked version's entry has no hash; it replaces it with the verified-commit fetch, whose hash becomes the new baseline.
- The LockIntegrity model's configuration now mirrors the fixed code; the pre-fix behaviour is a regression mutant; the F4 replay test is inverted.

## Capabilities

### New Capabilities

- `hashless-lock-entries`: lock entries without a content hash never vouch for module content.

### Modified Capabilities

(none)

## Impact

- `pkg/invowkmod` (`verify.go`, `lock_integrity.go`), `internal/app/moduleops/vendor.go`, `internal/app/modulesync/resolver_deps.go`, their tests; lock-file docs (en, pt-BR) and README; `formal/`.
- Migration: projects with a v1.0 `invowkmod.lock.cue` and vendored modules run `invowk module sync` once.
