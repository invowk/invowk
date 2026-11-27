## 1. Fix

- [x] 1.1 `IsDeclaredLockedCommandSource` takes the discovered module directory and compares the locked hash
- [x] 1.2 `buildCommandScope` passes the directory and memoizes per module
- [x] 1.3 Denial message covers the version-mismatch case

## 2. Tests and model

- [x] 2.1 Invert the F6 replay; add a `CheckCommandDependenciesExist` test with real directories (both calibrated by disabling the comparison)
- [x] 2.2 Drop placeholder hashes from identity-only fixtures
- [x] 2.3 Rename the finding command to the regression mutant `mutantPreFixF6AdmitByIdentity`

## 3. Docs and verification

- [x] 3.1 Command dependency docs (en, pt-BR), README, `formal/README.md`
- [x] 3.2 `make formal`; affected package and CLI tests; lint; baseline; website build
