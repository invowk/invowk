## Why

`pkg/fspath.AtomicWriteFile`, which `LockFile.Save` uses for `invowkmod.lock.cue`, wrote a temp file and renamed it without any fsync. Readers never saw partial content, but after power loss the renamed file could be empty or torn, because the data and the directory entry were never forced to disk. The AtomicWrite TLA+ model (finding F3 of `adopt-formal-verification`) showed that fsync of the file restores atomicity across power loss and fsync of the directory makes a returned success durable.

## What Changes

- The temp file is fsynced before it is closed and renamed.
- The parent directory is fsynced after the rename. On Windows this step is skipped, because directories cannot be opened for fsync and NTFS journals the rename.
- A failed directory fsync returns an error that states the new content is in place but its durability is unknown; the deferred cleanup no longer tries to remove a temp name that the rename already consumed.
- The AtomicWrite model gains that post-rename failure, and its base configuration now mirrors the fixed code; the pre-fix configurations stay as regression mutants.

## Capabilities

### New Capabilities

- `atomic-write-durability`: durability guarantees of `AtomicWriteFile` across power loss.

### Modified Capabilities

(none)

## Impact

- `pkg/fspath/atomic.go` and its tests; `formal/tla/AtomicWrite.tla`, `formal/manifest.toml`, `formal/README.md`.
- Two extra syscalls per lock-file save.
