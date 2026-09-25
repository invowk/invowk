## 1. Fix

- [x] 1.1 fsync the temp file before close and rename
- [x] 1.2 fsync the parent directory after the rename (skipped on Windows); report a post-rename failure without removing the consumed temp name

## 2. Model and tests

- [x] 2.1 Add the post-rename directory-fsync failure to `AtomicWrite.tla`; flip the base configuration; keep pre-fix configurations as regression mutants
- [x] 2.2 Inject failures at the new sync steps in the real-filesystem binding and the trace harness
- [x] 2.3 `make formal`, `make formal-traces`, `go test ./pkg/fspath/ ./pkg/invowkmod/`, Windows and macOS cross-vet
