# Tasks

## 1. Calibrating tests (written first, must fail on the pre-fix tree)

- [x] 1.1 In `internal/app/modulesync/resolver_integrity_test.go`, make `TestSyncFreshCacheReproducesLockedHash` deterministic: after the first sync, rewrite the lock with `generated:` set to a fixed past instant (design D3) and compare the second sync's lock against those backdated bytes; no sleeps. Verify it fails on the unmodified tree: `go test -run TestSyncFreshCacheReproducesLockedHash -count=3 ./internal/app/modulesync/` fails 3 of 3.
- [x] 1.2 Add `tests/cli/testdata/module_update_lock_unchanged.txtar`: a valid but non-canonical empty lock (old `generated:` with `+00:00`, different spacing), a saved copy, `exec invowk module update`, `cmp` the lock with the copy (design D3). Verify it fails on the unmodified tree: `go test -run 'TestCLI/module_update_lock_unchanged' ./tests/cli/` (use the CLI test runner's actual name filter).
- [x] 1.3 Record both pre-fix failures (command and failing assertion) in the stage result.

## 2. Writer fix in `LockFile.Save`

- [x] 2.1 In `pkg/invowkmod/lockfile.go`, add an unexported semantic-equality helper (same `Version`, same keys, equal `LockedModule` per key) and make `Save` inspect the existing file with `InspectLockFile`: when it is present, parsed and equal, return nil without writing and set `l.Generated` from the file; otherwise set `l.Generated` to the write time and write as today (design D1, D2). Stat, size or parse failures fall through to the write, so `lockfile_mutation_test.go:182` keeps its mkdir error. Update the `Save` doc comment. Verify 1.1 and 1.2 now pass.
- [x] 2.2 Add `pkg/invowkmod` unit tests for `Save`: an equivalent non-canonical file stays byte-identical; a change to each single `LockedModule` field, an added key, a removed key, and a version change each write a newer `generated:`; a missing file is created; an unparseable file is replaced. Verify `go test -run 'TestLockFile' ./pkg/invowkmod/` passes.
- [x] 2.3 Adjust existing tests that pin an exact written `generated:` from a new lock (check `pkg/invowkmod/lockfile_test.go` around :244-260 and :349-400, `lockfile_cue_test.go`, and `internal/app/modulesync` tests that read the lock), listing each one touched; use an unexported `saveAt` seam only if an exact time is needed (design D2). Verify `go test ./pkg/invowkmod/ ./internal/app/modulesync/` passes.

## 3. Content change advances `generated:`

- [x] 3.1 In `internal/app/modulesync/resolver_integrity_test.go` (or a sibling file), add a test that backdates the lock, then changes content through the resolver (an `Update` to a newly listed version via the fake fetcher, and an `Add` of a second module), and asserts the written `generated:` parses later than the backdated instant (spec: Changed lock gets a fresh generation time). Verify `go test -run 'TestSync|TestUpdate|TestAdd' ./internal/app/modulesync/` passes.

## 4. Formal correspondence

- [x] 4.1 Add a note to the `LockFile.Save` rows of `formal/tla/AtomicWrite.tla:27` and `formal/tla/ConcurrentModuleEdits.tla:57` (and the A1 comment at `ConcurrentModuleEdits.tla:12` if it needs it) that a skipped write equals an atomic replace with identical content (design D4); do not change model logic. Verify `python3 scripts/formal.py correspondence` and `make formal-reach` pass.
- [x] 4.2 Confirm `TestConcurrentEdits_FindingF8_SyncOverwritesConcurrentAdd` still reproduces F8: `go test -run TestConcurrentEdits_FindingF8 ./internal/app/modulesync/` behaves as before the change. Run `make formal` when Java is available; otherwise say so.
- [x] 4.3 Because `formal/` changed, run `make test-scripts` (under the heavy-command lock) and verify it passes. If the template's fix stage requires it for bound code, run `make mutation-formal` and the partial refresh for any bound symbol whose body changed (`Resolver.Sync` must stay unchanged; `LockFile.Save` is a `binding_gaps` entry).

## 5. Docs

- [x] 5.1 Check the website and README for statements about when `invowkmod.lock.cue` is written or what `generated:` means (`rg -n 'generated|lock file' website/docs README.md`), and update any that the new behavior contradicts, including i18n copies per `.agents/skills/docs/SKILL.md`. Verify `cd website && npm run build` passes when docs change; otherwise record that no doc needed a change.
- [x] 5.2 List the open question about the "Lock file updated" message (`cmd/invowk/module_deps.go:284`, `:364`) in the result's `maintainer_asks`.

## 6. Completion gates (`.agents/rules/checklist.md`)

- [x] 6.1 `make tidy`; `make license-check` if new Go files were added.
- [x] 6.2 `make lint` passes with no issues.
- [x] 6.3 `make test` passes (full suite, under the heavy-command lock).
- [x] 6.4 `make check-baseline` passes; triage any new goplint finding.
- [x] 6.5 `make check-file-length` passes.
- [x] 6.6 `make sonar-local`: no new real issues.
- [x] 6.7 Conditional checks: `make test-scripts` (formal changed) and `python3 scripts/formal.py correspondence` pass.
- [x] 6.8 `/learn` (`.agents/skills/learn/`) last.
