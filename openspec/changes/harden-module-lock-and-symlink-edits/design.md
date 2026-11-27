# Design: Harden Module Lock Identity (MA2) and Symlink-Safe Edits (MA5)

## Context

See `proposal.md` for motivation and capability scope. The current codebase exhibits two concrete gaps:

1. **MA2 (`darwin` lock directory canonicalization)**:
   - `internal/modulelock/physical.go:15-28` delegates canonicalization of `FileLocker.PhysicalDir` to a platform-specific `physicalDir(string) (string, error)` helper and validates the result as `types.FilesystemPath`.
   - `internal/modulelock/physical_windows.go:3,30-53` resolves real path, junctions, 8.3 names, and on-disk letter case via `windows.GetFinalPathNameByHandle`.
   - `internal/modulelock/physical_unix.go:3,13-23` currently applies `//go:build !windows` (covering both Linux and macOS) and runs only `filepath.Abs` + `filepath.EvalSymlinks`. On case-insensitive macOS APFS/HFS+ volumes, Go's `filepath.EvalSymlinks` preserves caller-supplied letter case rather than querying the filesystem's canonical directory spelling, leaving a documented limit in `openspec/specs/module-command-locking/spec.md:64-66,93-95,140`, `README.md:2215`, `website/docs/modules/dependencies/cli-commands.mdx:252`, and `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/modules/dependencies/cli-commands.mdx:243`.
   - `golang.org/x/sys` v0.50.0 (`unix` package, inspected for `darwin/arm64` and `darwin/amd64`) provides `unix.F_GETPATH` (`0x32`), `unix.SYS_FCNTL` (`92`), `unix.PathMax` (`0x400`), `unix.Syscall`, `unix.FcntlInt` (`fcntl_darwin.go:10`), and `unix.FcntlFlock` (`fcntl_darwin.go:15`), and does **not** expose a `unix.FcntlGetpath` wrapper.

2. **MA5 (Symlink behavior across module edit, authoring, and rollback paths)**:
   - `pkg/fspath/atomic.go:1-131` implements `AtomicWriteFile` by writing a temporary file inside `filepath.Dir(path)` and calling `os.Rename` onto `path`. When `path` is a symlink, `AtomicWriteFile` replaces the symlink entry itself with a regular file.
   - `pkg/invowkmod/invowkmod_edit.go:42-103` (`AddRequirement`) reads via `os.ReadFile` (`:47`), validates via `checkEditKeepsCUEValid` (`:99`), and writes back via `fspath.AtomicWriteFile(string(invowkmodPath), []byte(out), fspath.DefaultFilePerm)` (`:102`).
   - `pkg/invowkmod/invowkmod_edit.go:125-168` (`RemoveRequirementIfPresent`) reads via `os.ReadFile` (`:127`), treats any `os.IsNotExist(err)` — including a dangling symlink — as absent (`:128-130`, returning `(false, nil)`), validates (`:161`), and writes via `fspath.AtomicWriteFile` (`:164`).
   - `internal/agentcmd/module.go:527-567` (`loadExistingModule`) rejects symlinked module directories (`:539-541`), verifies the `.invowkmod` bundle via `invowkmod.Validate` (`:548`, which rejects symlinks inside packaged modules at `pkg/invowkmod/operations_validate.go:191-193`), and loads `invowkmod.cue`/`invowkfile.cue` through `readModuleFiles` (`:552`). Under the project lock (`writeChangedModuleFiles`, `:594-617`), it re-reads via `readModuleFiles` (`:609`) and writes via `writeModuleFiles` (`:616`, `:679-689`). Crucially, `readModuleFiles` (`:571-586`) falls back to `defaultInvowkfileContent` whenever `os.ReadFile(invowkfilePath)` returns `os.IsNotExist(err)` (`:580`), masking dangling `invowkfile.cue` symlinks, and `writeModuleFiles` passes un-resolved paths directly to `fspath.AtomicWriteFile` (`:681,685`).
   - `internal/app/modulesync/use_cases.go:289-323` (`AddModuleDependency`) and `:344-386` (`RemoveModuleDependency`) snapshot both lock and `invowkmod.cue` paths before editing (`:299-303`, `:351-355`) and call `rollBackDeclarationEdit` (`:316`, `:370`, `:392-398`) on failure. However, `readFileSnapshot` (`:458-489`) uses `os.Lstat` (`:460`) and its `os.ModeSymlink` branch (`:470-477`) captures only `os.Readlink` into `linkTarget` without snapshotting the target regular file's bytes or mode. `fileSnapshot.restore` (`:512-535`) delegates `snapshotSymlink` to `restoreLink` (`:540-557`), which exits early when `os.Readlink(path) == *s.linkTarget` (`:542-544`), leaving any target mutations un-restored (for instance when `RemoveModuleDependency` loops across multiple removals at `:365-383` and a later edit fails).

## Goals / Non-Goals

**Goals:**
- Resolve canonical on-disk letter case, symlinks, and firmlinks for `FileLocker.PhysicalDir` on `darwin` (`arm64` and `amd64`) using kernel `F_GETPATH`, achieving lock-identity parity with `windows` and updating specs/docs accordingly.
- Implement Option A1 consistently across `pkg/invowkmod` (`AddRequirement`, `RemoveRequirementIfPresent`), `internal/agentcmd` (`readModuleFiles`, `writeModuleFiles` for `invowkmod.cue` and `invowkfile.cue`), and `internal/app/modulesync` (`fileSnapshot`, `readFileSnapshot`, `restore`): follow symlinks to regular files, atomically replace resolved regular targets while preserving target permissions and symlink entries, and fail closed on dangling or non-regular symlinks.
- Keep all formal correspondence rows (`RemoveRequirementIfPresent`, `fileSnapshot.restore` in `ConcurrentModuleEdits.tla`) and `tests/coverage/pathmatrix_coverage_test.go` guardrails satisfied.

**Non-Goals:**
- Changing symlink rejection policy inside `.invowkmod` packaging validation (`pkg/invowkmod/operations_validate.go:191-193`) or `loadExistingModule` symlinked module directory rejection (`internal/agentcmd/module.go:539-541`).
- Changing non-module `agent cmd` (`internal/agentcmd/create.go`) plain `invowkfile.cue` writes or `invowk.mod.lock` lock-file write semantics (`SaveLock` continues replacing `invowk.mod.lock` via `fspath.AtomicWriteFile`, matching `openspec/specs/module-lock-persistence/spec.md:159-161`).
- Expanding `invowk module add`/`remove` to accept arbitrary symlinked `invowkmod.cue` pointing to directories, FIFOs, sockets, or devices (explicitly forbidden).

## Decisions

### Decision 1 (MA2): Darwin `F_GETPATH` via `unix.Syscall(unix.SYS_FCNTL, ...)` on a raw descriptor

In `internal/modulelock/physical_darwin.go` (`//go:build darwin`), `physicalDir(dir string) (string, error)`:
1. Normalizes `dir` with `filepath.Abs(dir)` so empty/relative path handling matches `physical_unix.go:14-16`.
2. Opens `abs` with `unix.Open(abs, unix.O_RDONLY|unix.O_CLOEXEC, 0)` retrying on `unix.EINTR`, and closes the fd via `defer`. We intentionally omit `unix.O_DIRECTORY` to match `physical_unix.go` and `physical_windows.go` behavior when tests pass existing filesystem paths (`PhysicalDir` is called on module/project directories in production).
3. Allocates `var buf [unix.PathMax]byte` (`1024` bytes) and calls `unix.Syscall(unix.SYS_FCNTL, uintptr(fd), unix.F_GETPATH, uintptr(unsafe.Pointer(&buf[0])))`, retrying on `unix.EINTR` and failing explicitly on non-zero `errno` (no silent fallback to `EvalSymlinks`).
4. Slices up to the first NUL byte via `bytes.IndexByte(buf[:], 0)`; fails explicitly if no NUL terminator is found or the resulting path is empty.

**Alternatives considered:**
- **`unix.FcntlGetpath`**: Does not exist in `golang.org/x/sys/unix` v0.50.0.
- **`unix.FcntlInt(fd, unix.F_GETPATH, int(uintptr(...)))`** (`fcntl_darwin.go:10`): Converts a pointer through `uintptr` to `int` outside a direct syscall expression, violating Go `unsafe.Pointer` rules (`go vet` `unsafeptr` and `-d=checkptr`). Rejected.
- **`unix.FcntlFlock(fd, unix.F_GETPATH, (*unix.Flock_t)(unsafe.Pointer(&buf[0])))`** (`fcntl_darwin.go:15`): Relies on pointer-type punning between `[1024]byte` (1-byte aligned) and `*unix.Flock_t` (8-byte aligned due to `Start int64`), which risks alignment errors under `-d=checkptr` unless manually backed by `uint64` storage. Rejected in favor of direct `unix.Syscall` matching the repository's `//nolint:gosec // G103:` precedent in `internal/sshserver/pty_unix.go:25-29` (`CGO_ENABLED=0`-safe, `checkptr`-safe on both `darwin/amd64` and `darwin/arm64`).
- **`os.Open` + `f.Fd()`**: `os.File.Fd()` disables non-blocking mode and risks premature finalizer close unless guarded by `runtime.KeepAlive(f)`; `unix.Open` returns a plain `int` fd cleanly.

### Decision 2 (MA2): Build-tag split, Darwin test strategy, and doc sync

- Narrow `internal/modulelock/physical_unix.go:3` from `//go:build !windows` to `//go:build !windows && !darwin`.
- Add `internal/modulelock/physical_darwin.go` (`//go:build darwin`) and `internal/modulelock/physical_darwin_test.go` (`//go:build darwin`).
- Because Linux/Windows developer machines cannot execute Darwin binaries, cross-verify `GOOS=darwin GOARCH=arm64` and `GOOS=darwin GOARCH=amd64` compilation/vet (`go vet` and `go test -c`) locally, while `physical_darwin_test.go` tests uppercase/lowercase/mixed-case directory lookups returning identical `PhysicalDir` matching on-disk canonical case (skipping cleanly only if the test volume is case-sensitive, detected when swapped-case `os.Stat` returns `fs.ErrNotExist`).
- Update `README.md:2215`, `website/docs/modules/dependencies/cli-commands.mdx:252`, and `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/modules/dependencies/cli-commands.mdx:243` to remove the macOS letter-case known limit and state macOS canonicalization parity alongside Windows.

### Decision 3 (MA5): Shared regular-file write-target inspection helper in `pkg/fspath`

To prevent duplicated symlink/regular-file logic across `pkg/invowkmod`, `internal/agentcmd`, and `internal/app/modulesync`, add a cohesive helper in `pkg/fspath/target.go`:
- `var ErrNonRegularTarget = errors.New("target is not a regular file")`
- `type FileTarget struct { Path string; Perm os.FileMode; Exists bool; WasSymlink bool }`
- `func InspectWritableTarget(path string) (FileTarget, error)`:
  1. `info, err := os.Lstat(path)`: if `errors.Is(err, fs.ErrNotExist)`, return `FileTarget{Path: path, Exists: false}, nil`; on any other `os.Lstat` error, return wrapped error.
  2. If `info.Mode()&os.ModeSymlink == 0`: if `!info.Mode().IsRegular()`, return `fmt.Errorf("%w: %s (%s)", ErrNonRegularTarget, path, info.Mode().Type())`; otherwise return `FileTarget{Path: path, Perm: info.Mode().Perm(), Exists: true, WasSymlink: false}, nil`.
  3. If `info.Mode()&os.ModeSymlink != 0`: resolve `resolved, err := filepath.EvalSymlinks(path)` (wrapping `%w` so dangling links preserve `errors.Is(err, fs.ErrNotExist)`), call `targetInfo, err := os.Lstat(resolved)` and fail closed unless `targetInfo.Mode().IsRegular()`; return `FileTarget{Path: resolved, Perm: targetInfo.Mode().Perm(), Exists: true, WasSymlink: true}, nil`.

**Design & guardrail rationale:**
- Naming `InspectWritableTarget` / `FileTarget` avoids `"path"` and `"workdir"` substrings so it accurately reflects file-target semantics without triggering `tests/coverage/pathmatrix_coverage_test.go`'s dialect-neutral path-validator check (`TestPathMatrixSurfaces_AreCovered`).
- Checking `os.Lstat` before `filepath.EvalSymlinks` ensures absent paths (`Exists: false`) are distinguished cleanly from dangling symlinks (where `Lstat` succeeds with `ModeSymlink` and `EvalSymlinks` fails closed with wrapped `fs.ErrNotExist`).
- Preserving `targetInfo.Mode().Perm()` keeps existing file permissions (for example `0600` on private manifests or `0644` default) rather than silently widening permissions to `fspath.DefaultFilePerm`.

**Alternative considered:** Calling `os.Stat` everywhere and skipping `Lstat`: rejected because `os.Stat` conflates a missing file with a dangling symlink (causing `RemoveRequirementIfPresent` and `agentcmd.readModuleFiles` to misclassify dangling links as absent) and does not provide the resolved destination path for `fspath.AtomicWriteFile`.

### Decision 4 (MA5): Applying Option A1 in `pkg/invowkmod` and `internal/agentcmd`

- **`pkg/invowkmod/invowkmod_edit.go`**:
  - `AddRequirement` (`:42-103`): call `fspath.InspectWritableTarget(string(invowkmodPath))`. Return any inspection error directly; if `!target.Exists`, return `fmt.Errorf("read %s: %w", invowkmodPath, fs.ErrNotExist)` so `errors.Is(err, fs.ErrNotExist)` contracts in `cmd/invowk` and `modulesync` remain unchanged. Read `target.Path`, preserve `checkEditKeepsCUEValid` diagnostics (`invowkmodPath`), and write atomically via `fspath.AtomicWriteFile(target.Path, []byte(out), target.Perm)`.
  - `RemoveRequirementIfPresent` (`:125-168`): call `fspath.InspectWritableTarget(pathStr)`. On error (including dangling symlink or non-regular symlink target), return `false, err` — failing closed instead of masking dangling symlinks as absent. Only when `!target.Exists` (truly absent file) return `false, nil`. Read `target.Path` and write atomically via `fspath.AtomicWriteFile(target.Path, []byte(out), target.Perm)`.
- **`internal/agentcmd/module.go`**:
  - Update `readModuleFiles` (`:571-586`): inspect `invowkmodPath` and `invowkfilePath` via `fspath.InspectWritableTarget`. For `invowkmodPath`, require `target.Exists` and read `target.Path`. For `invowkfilePath`, propagate any `InspectWritableTarget` error (failing closed on dangling/non-regular symlinks rather than hitting `:580` fallback); fall back to `defaultInvowkfileContent` only when `!target.Exists`.
  - Update `writeModuleFiles` (`:679-689`, called by `writeChangedModuleFiles` under the project lock at `:599-616` and by bundle creation at `:647,:670`): inspect each target with `fspath.InspectWritableTarget` immediately before `fspath.AtomicWriteFile`. Fail closed on any inspection error (protecting against TOCTOU replacement with a dangling or non-regular symlink between read and write); when `target.Exists`, write `target.Path` with `target.Perm`; when `!target.Exists` (new `invowkfile.cue` or staging temp dirs), write the direct path with `moduleFilePerm`.

### Decision 5 (MA5): Target-aware symlink snapshot and rollback in `internal/app/modulesync`

Update `fileSnapshot`, `readFileSnapshot`, and `fileSnapshot.restore` in `internal/app/modulesync/use_cases.go`:
- Add `targetPath *types.FilesystemPath` to `fileSnapshot` (`:61-72`), allowed only when `kind == snapshotSymlink`, and validate it in `fileSnapshot.Validate`.
- In `readFileSnapshot` (`:458-489`), inside `case info.Mode()&os.ModeSymlink != 0:` (`:470-477`):
  - Always record `kind: snapshotSymlink` and `linkTarget` (`os.Readlink`) as today.
  - Also probe `target, err := fspath.InspectWritableTarget(string(path))`:
    - If `err == nil && target.Exists`, read `target.Path` via `os.ReadFile` (failing `readFileSnapshot` if a readable regular target cannot be read) and store validated `targetPath`, `data: fileSnapshotData(targetData)`, and `mode: target.Perm`.
    - If `err != nil` (dangling symlink, loop, or non-regular target like directory/FIFO), leave `targetPath: nil` so snapshot still succeeds link-only. That preserves existing rollback tests where `invowkmod.cue` starts as a dangling link (`AddRequirement` fails closed and rollback keeps the dangling link intact).
- In `fileSnapshot.restore` (`:512-535`):
  - For `case snapshotSymlink:`, if `s.targetPath != nil`, restore target regular-file bytes and mode atomically via `s.fileWriter()(string(*s.targetPath), []byte(s.data), s.mode.Perm())`, and restore/preserve the symlink via `s.restoreLink()`, joining any errors (`errors.Join`) so both target content restoration failure and link recreation failure are surfaced accurately.

**Alternative considered:** Checking `os.ReadFile(*s.targetPath)` during rollback and skipping `fileWriter` if unchanged. Rejected in favor of uniform unconditional `fileWriter` restoration whenever `s.targetPath != nil`, which matches `case snapshotFile:` atomicity/replace guarantees, avoids stat-compare-write TOCTOU races, and triggers injected `fileWriter` fault tests cleanly.

### Decision 6: Formal verification and mutation baseline maintenance

Because `pkg/invowkmod.RemoveRequirementIfPresent` and `internal/app/modulesync.fileSnapshot.restore` are bound symbols in `formal/tla/ConcurrentModuleEdits.tla` / `tools/mutation/triage/formal-bindings.toml` (and `AddRequirement` is tracked under `formal/manifest.toml` `[triage] binding_gaps`), changes to those functions require running:
- `python3 scripts/formal.py affected --event pull_request`
- `python3 scripts/formal.py correspondence`
- `make formal-reach` (and `make formal` where TLC/Java is available)
- Scoped `make mutation-formal MUTATION_FORMAL_SYMBOL=...` and `make mutation-formal-refresh` if anchored mutant line numbers shift or new mutants arise in those bound symbols.

## Risks / Trade-offs

- **[Risk] Darwin-only `F_GETPATH` cannot execute on Linux/Windows hosts** → Mitigation: Cross-compile and `go vet` `internal/modulelock` with `GOOS=darwin` for both `amd64` and `arm64`, write `physical_darwin_test.go` without fragile environment assumptions, and verify execution on macOS CI runner.
- **[Risk] `RemoveRequirementIfPresent` and `agentcmd` `readModuleFiles` failing closed on dangling symlinks changes silent no-op/fallback behavior** → Mitigation: Only broken/dangling or non-regular symlinks are rejected (truly absent files preserve `(false, nil)` and default `invowkfile.cue` fallback); test both cases explicitly.
- **[Risk] Rollback restoration of `s.targetPath` assumes `*s.targetPath`'s parent directory is writable when `s.targetPath != nil`** → Mitigation: If `invowkmod.cue` is edited through a symlink, its target directory must already be writable; for `invowk.mod.lock` symlinks in test setups, both lock and target reside in writable test temp dirs, and any write error is surfaced explicitly via `errors.Join`.
- **[Risk] Touching bound symbols shifts `formal-bindings.toml` line anchors or introduces mutation survivors** → Mitigation: Include `formal.py correspondence`, `make formal-reach`, `pkg/invowkmod` mutation tests, and scoped `mutation-formal` / `mutation-formal-refresh` checks in `tasks.md`.

## Maintainer decisions

- **2026-11-26 — MA5 (Symlink-safe module edits: Option A1 approved)**: Approved Option A1 across `pkg/invowkmod` (`AddRequirement`, `RemoveRequirementIfPresent`), `internal/agentcmd` (`agent mod change`), and `internal/app/modulesync` (`fileSnapshot` snapshot & rollback): resolve symlinks to regular files, atomically write resolved target paths so symlink entries remain intact and target permissions are preserved, fail closed on dangling or non-regular symlinks, and snapshot/restore regular-file symlink targets atomically during rollback.
- **2026-11-26 — MA2 (Darwin project lock normalization via `F_GETPATH`)**: Approved resolving `FileLocker.PhysicalDir` on `darwin` using kernel `F_GETPATH` via `unix.Syscall(unix.SYS_FCNTL, ...)` (`unix.FcntlGetpath` does not exist in `golang.org/x/sys/unix` v0.50.0), splitting `physical_unix.go` (`!windows && !darwin`), adding Darwin tests, and removing the macOS case-insensitive limit from specs and EN/pt-BR documentation.

## Open questions

None.
