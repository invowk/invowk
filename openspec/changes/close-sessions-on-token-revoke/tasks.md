## 1. Fix

- [x] 1.1 Wrap accepted connections; bind them to their token at login atomically
- [x] 1.2 `RevokeToken` and `RevokeTokensForCommand` close bound connections, including those of expired tokens
- [x] 1.3 Fail closed on untracked connections

## 2. Tests and model

- [x] 2.1 Real SSH client test, revoke/login race test, and expired-token test (calibrated by disabling the close and splitting the critical section)
- [x] 2.2 Extend the rapid binding with sessions (detects revocation leaving connections open and expiry closing them)
- [x] 2.3 Flip the HostCallbackToken base configuration; keep the pre-fix behaviour as a regression mutant

## 3. Docs and verification

- [x] 3.1 Container runtime docs (en, pt-BR), README, `formal/README.md`
- [x] 3.2 `make formal`; package tests; lint; baseline; website build
