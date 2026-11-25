# Proposal

## Why

Module commands that run at the same time lose each other's work. A `module sync` or `module update` saves a lock built from files it read before its fetch, so it drops a concurrent `module add` or brings back a concurrent `module remove`, and a duplicate `module add` rolls back the other one (formal finding F8). Two projects that share the module cache can lock one version's commit with another version's content hash (F9). The rollback after a failed `invowkmod.cue` edit truncates the files in place, so a concurrent reader can see an empty lock (F10). The maintainer approved fixing all three on 2026-11-25 (decisions 125 and 127), together with the rollback follow-ups left open by `module-cli-messages` (OQ1, OQ2, OQ4, the `os.Stat` snapshot bug and the exit status after a rollback).

## What Changes

- Every module command that writes `invowkmod.cue` or the lock (`module add`, `remove`, `sync`, `update`, `tidy`, `vendor`) holds an exclusive per-project lock from before its first read of either file until after its last write or rollback. A command that finds the lock busy prints `Waiting for another invowk module command on this project (lock: <path>)` once on stderr and waits up to 5 minutes; Ctrl+C ends the wait. Fixes F8 and the stale-vendor case.
- Fetching a Git source holds an exclusive per-source lock from checkout until the cache copy is verified, always taken after the project lock. A busy source prints `Waiting for another invowk module command on module source <url> (lock: <path>)`. Fixes F9.
- Lock files are empty files under `<module cache>/locks/`, outside every project, module, cache worktree and cache copy. They are never deleted. The OS releases a lock when its holder exits, so no lock goes stale. Read-only consumers (discovery, command runs, `validate`, `audit`, `module deps`) take no lock.
- The rollback restores each file with an atomic replace (temporary file and rename), so a concurrent reader sees the old or the new file, never a partial one. Fixes F10.
- The rollback snapshot no longer follows symbolic links: a dangling `invowkmod.cue` link survives a rollback instead of being deleted, and a linked file is restored as the link it was.
- **BREAKING** (exit status): `module add` and `module remove` exit with status 1 when they rolled back after a failed `invowkmod.cue` edit. Until now they exited 0.
- `module remove` no longer prints `Removed <namespace>` lines when the removal was rolled back.
- The use case reports the rollback outcome as a typed value (none, complete, incomplete). The CLI no longer infers it from the shape of a joined error.
- `module add` and `module remove` recognise `requires` entries written on one line (`{git_url: "…", version: "…"}`): add reports them as duplicates, and remove deletes them.
- Formal: F8, F9 and F10 are recorded as fixed in `formal/README.md`. The `ConcurrentModuleEdits` base configuration mirrors the fixed code, each finding becomes a `mutantPreFix…` command, and the Go replays are inverted.

## Migration notes

- Scripts that call `invowk module add` or `invowk module remove` and treat exit status 0 as success now see status 1 when the command rolled back its lock change. In that case the dependency was not added or removed. Before this release such a run exited 0 and only stdout said so.
- Module commands on the same project, or on the same Git source across projects, now run one at a time. A command that has to wait says so on stderr. If another invowk process keeps the lock for more than 5 minutes, the waiting command fails and names the lock file.
- The module cache directory gets a `locks/` subdirectory (mode 0700). Do not delete lock files while a module command runs; deleting them is harmless otherwise.

## Capabilities

### New Capabilities
- `module-command-locking`: which module commands serialise on a project or Git source, where the lock files live, how a waiting command reports and ends its wait, the lock order, and which consumers take no lock.

### Modified Capabilities
- `module-lock-persistence`: a failed `invowkmod.cue` edit makes add and remove exit 1, remove prints no `Removed` lines after a rollback, and the rollback restores each file atomically and without following links; requires entries written on one line are matched.
- `concurrent-module-edit-model`: F8, F9 and F10 move from open findings to fixed. The base configuration mirrors the locked, atomic code, the pre-fix behaviour becomes regression mutants, the replays assert the fixed behaviour, and the rollback reader is a read-only consumer that takes no lock.

## Impact

- Code: `internal/app/modulesync` (lock port, project lock in the use cases, source lock in `resolveOne`, typed rollback outcome, snapshot and restore), a new adapter package `internal/modulelock` (flock on Unix, `LockFileEx` on Windows, through the existing `golang.org/x/sys`), `internal/app/moduleops` (vendor under the project lock), `pkg/invowkmod/invowkmod_edit.go` (one-line entries), `cmd/invowk/module_deps.go`, `module_tidy.go`, `module_package.go` (wiring, wait line, exit status, rollback lines).
- Tests: the inverted `TestConcurrentEdits_*` replays in `internal/app/modulesync` and `internal/app/moduleops`, new adapter tests (including a cross-process test), use-case and editor tests, and the testscripts `module_remove_messages.txtar` and `module_add_remove.txtar`.
- Formal: `formal/tla/ConcurrentModuleEdits.tla`, `formal/manifest.toml`, `formal/README.md`, and in the fix stage the formal-bindings ledger.
- Docs: `README.md`, `website/docs/modules/dependencies/cli-commands.mdx` and its pt-BR copy, and the PR release note.
- No `go.mod` change.
