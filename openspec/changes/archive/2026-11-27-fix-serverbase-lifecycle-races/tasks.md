## 1. Fix

- [x] 1.1 `TransitionToStarting`: CAS and context store in one `stateMu` critical section
- [x] 1.2 `casToTerminalLocked` for `TransitionToFailed` and `TransitionToStopped`

## 2. Model

- [x] 2.1 Model the retrying CAS in `Serverbase.tla` and confirm it passes before changing code
- [x] 2.2 Flip the base configuration to the fixed code; keep the pre-fix code as regression mutants; declare `StartStore` dead
- [x] 2.3 Give the close-twice mutant a path two callers can reach, since the fix made `sync.Once` defense in depth

## 3. Verification

- [x] 3.1 `make formal`, `make formal-traces`, serverbase, sshserver, and tuiserver tests
- [x] 3.2 Update `.agents/skills/server/SKILL.md` and `formal/README.md`
