## 1. Fix

- [x] 1.1 `LockEntryWithoutContentHashError`; `VerifyLockedVendoredModuleHash` returns it for hashless entries
- [x] 1.2 `VendorModules` rejects hashless entries
- [x] 1.3 Sync drops a cached copy when the locked version's entry has no hash

## 2. Tests and model

- [x] 2.1 Invert the F4 replay; add a sync test with a v1.0 lock and a tampered cache (calibrated by disabling the refetch)
- [x] 2.2 Give vendoring test fixtures real content hashes
- [x] 2.3 Flip the LockIntegrity configuration; keep the pre-fix behaviour as a regression mutant

## 3. Docs and verification

- [x] 3.1 Lock-file docs (en, pt-BR) and README
- [x] 3.2 `make formal`; affected package tests; lint; baseline; website build
