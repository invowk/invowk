## Why

Two module-hardening gaps remain deferred from the module command locking and concurrency rollback work (`MA2` and `MA5` in the prior change backlog):

1. **MA2 — `darwin` case-insensitive project lock identity**: `internal/modulelock/physical_unix.go` currently runs under `//go:build !windows` and resolves `PhysicalDir` only via `filepath.Abs` + `filepath.EvalSymlinks`. On case-insensitive macOS APFS/HFS+ volumes, Go's `filepath.EvalSymlinks` preserves caller letter case instead of returning on-disk canonical case, so two concurrent commands targeting `/path/To/Module` and `/path/to/module` hash to different lock paths and fail to exclude each other.
2. **MA5 — Symlink-safe module manifest and authoring edits (Option A1 approved 2026-11-26)**: `pkg/invowkmod.AddRequirement`, `pkg/invowkmod.RemoveRequirementIfPresent`, and `internal/agentcmd/module.go` (`agent mod change`) currently read through symlinks via `os.ReadFile` (or mask dangling `invowkfile.cue` symlinks via `os.IsNotExist` fallback) and pass the original path directly to `fspath.AtomicWriteFile`, which atomically renames over the symlink entry and silently detaches shared/dotfile links. Meanwhile, `internal/app/modulesync`'s `fileSnapshot` only captures the symlink target string (`os.Readlink`) without snapshotting the target regular file's bytes and permissions, so rollback leaves target mutations un-restored when edits go through a symlink.

## What Changes

- **Darwin physical directory resolution (`MA2`)**:
  - Narrow `internal/modulelock/physical_unix.go` build constraint to `//go:build !windows && !darwin`.
  - Add `internal/modulelock/physical_darwin.go` (`//go:build darwin`) resolving `PhysicalDir` to its canonical on-disk path (including canonical letter case, resolved symlinks, and firmlinks) via Darwin `fcntl(..., F_GETPATH, ...)` (`unix.Syscall(unix.SYS_FCNTL, ...)` in `golang.org/x/sys/unix` v0.50.0, since `unix.FcntlGetpath` does not exist in v0.50.0).
  - Add Darwin case-variant tests in `internal/modulelock/physical_darwin_test.go`.
  - Remove the macOS letter-case known limit from `openspec/specs/module-command-locking/spec.md`, `README.md`, and the EN / pt-BR module dependency CLI docs under `website/`.
- **Regular-file symlink preservation with fail-closed validation (`MA5`, Option A1)**:
  - Introduce a shared regular-file write-target resolver helper in `pkg/fspath` that inspects `os.Lstat`, resolves symlinks with `filepath.EvalSymlinks`, fails closed on dangling symlinks, broken loops, or targets that are not regular files (directories, FIFOs, sockets, devices), and returns the canonical regular target path and permissions for `fspath.AtomicWriteFile`.
  - Update `pkg/invowkmod.AddRequirement` and `pkg/invowkmod.RemoveRequirementIfPresent` (`pkg/invowkmod/invowkmod_edit.go`) to resolve targets with this helper so symlinks to regular files stay intact (writes atomically update the resolved target file) and dangling/non-regular symlinks fail closed (preserving `fs.ErrNotExist` for `AddRequirement` while no longer treating a dangling `invowkmod.cue` symlink as absent in `RemoveRequirementIfPresent`).
  - Update `internal/agentcmd/module.go` (`readModuleFiles` and `writeModuleFiles`) so `agent mod change` applies the same Option A1 rules to `invowkmod.cue` and `invowkfile.cue`, and does not fall back to default `invowkfile.cue` bytes when `invowkfile.cue` is a dangling symlink.
  - Update `internal/app/modulesync/use_cases.go` (`fileSnapshot`, `readFileSnapshot`, `restore`) so symlinks pointing to regular files snapshot both the symlink target string and the resolved target file's path, bytes, and permissions, and rollback restores target content atomically plus restores/preserves the symlink entry. Dangling or directory symlinks continue to snapshot link-only.

## Capabilities

### New Capabilities

_(None — this change hardens existing module locking, persistence, and authoring capabilities.)_

### Modified Capabilities

- `module-command-locking`: Remove the macOS case-insensitive volume known limit from lock serialization and documentation; require macOS project lock normalization to match Windows across letter-case path variants of the same directory.
- `module-lock-persistence`: Require `invowkmod.cue` edit operations (`AddRequirement`, `RemoveRequirementIfPresent`) to follow symlinks to regular files, write the resolved target atomically while keeping the symlink intact, fail closed on dangling or non-regular symlinks, and update rollback snapshots so symlinks to regular files restore both the target content atomically and the symlink entry.
- `agent-authoring-crud`: Require `agent mod change` reads and writes of `invowkmod.cue` and `invowkfile.cue` to follow symlinks to regular files, keep symlinks intact via atomic writes to the resolved target, and fail closed on dangling or non-regular symlinks.

## Impact

- **Go packages**:
  - `internal/modulelock` (`physical_unix.go`, new `physical_darwin.go`, new `physical_darwin_test.go`)
  - `pkg/fspath` (new shared target-resolution helper and unit tests)
  - `pkg/invowkmod` (`invowkmod_edit.go`, unit and mutation tests)
  - `internal/agentcmd` (`module.go`, unit tests)
  - `internal/app/modulesync` (`use_cases.go`, rollback unit tests)
- **Specs & Docs**:
  - `openspec/specs/module-command-locking/spec.md`, `openspec/specs/module-lock-persistence/spec.md`, `openspec/specs/agent-authoring-crud/spec.md`
  - `README.md`, `website/docs/modules/dependencies/cli-commands.mdx`, `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/modules/dependencies/cli-commands.mdx`
- **Formal verification & guardrails**:
  - `formal/manifest.toml` `[ci] paths` includes `pkg/fspath/**`, `pkg/invowkmod/**`, `internal/app/modulesync/**`, and `internal/modulelock/**`, so `python3 scripts/formal.py affected` will trigger formal checks.
  - `pkg/invowkmod.RemoveRequirementIfPresent` and `internal/app/modulesync.fileSnapshot.restore` are bound symbols in `formal/tla/ConcurrentModuleEdits.tla` / `tools/mutation/triage/formal-bindings.toml` (`AddRequirement` is in `[triage] binding_gaps`), so correspondence, reachability, and scoped formal mutation baseline refresh apply if bound function bodies shift.
  - Helper naming in `pkg/fspath` and `internal/app/modulesync` must respect `tests/coverage/pathmatrix_coverage_test.go` (`TestPathMatrixSurfaces_AreCovered`).
- **Compatibility**: Non-breaking for CLI flags, schemas, and regular-file manifests; workflows with symlinked `invowkmod.cue` / `invowkfile.cue` files now preserve symlinks instead of overwriting them, and dangling symlinks fail closed rather than silently succeeding or falling back to default file contents.
