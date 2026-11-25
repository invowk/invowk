# Tasks

All paths are in the worktree `/var/home/danilo/Workspace/github/invowk/invowk-w2`. `make test`, `make lint`, `make test-scripts`, `make check-baseline`, `make formal`, mutation targets, `go get` and every `git commit` run under `flock "$(git -C <worktree> rev-parse --git-common-dir)/invowk-heavy.lock" ...`, backgrounded with an `rc=` line and waited for with Monitor (`.agents/rules/agent-workflows.md`). One signed commit per item group (groups 1-8).

## 1. A drawn table shows its data rows (design D1)

- [ ] 1.1 Add tmux subtests to `TestTUI_Table` (`tests/cli/tui_tmux_test.go`) for a display table and a `--selectable` table on `name,age,city` / `Alice,30,NYC` / `Bob,25,LA`: wait for `Alice` and `Bob` on separate pane lines before any key; for the selectable one send Down and Enter and check that the printed row is `Bob,25,LA`. Run them against the base binary and record that they fail (`go test -run 'TestTUI_Table' ./tests/cli/`)
- [ ] 1.2 Add a unit test in `internal/tui/tui_table_test.go` that the `View()` of `NewTableModel` with two rows contains both rows' cells, and one that a table whose columns exceed a `WindowSizeMsg` width renders each row on one line no wider than that width; see both fail on the base
- [ ] 1.3 In `internal/tui/table.go`, set the table width when the model is built (`opts.Width`, else the natural width of the columns) and handle `tea.WindowSizeMsg` in the standalone model as D1 says; keep `SetSize` for overlays
- [ ] 1.4 Verify: `go test -run 'TestNewTableModel|TestTableModel' ./internal/tui/` and `go test -run 'TestTUI_Table' ./tests/cli/` pass

## 2. Plain-text table cell text (design D2)

- [ ] 2.1 In `internal/tui/table_text.go`, write Cf runes, U+2028 and U+2029 as a space and document in the `WriteTableText` godoc that invalid UTF-8 becomes U+FFFD; add table rows to `internal/tui/tui_table_text_test.go` for U+202E, U+200B, U+2028, U+2029, U+200D and an invalid byte
- [ ] 2.2 Rename `errTUISpinNotRunMsg` to `tuiSpinNotRunErrMsg` in `cmd/invowk/tui_spin.go` and check with `rg errTUISpinNotRunMsg` that no reference is left
- [ ] 2.3 Add to `tests/cli/testdata/tui_no_terminal.txtar` a `--file` case with the quoted field `Alice,"line1` / `line2"` and a stdin case with U+202E and U+200B in a cell, each compared with an exact expected file
- [ ] 2.4 Update `website/docs/tui/table-and-spin.mdx` and `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/tui/table-and-spin.mdx` (format and separator characters become spaces; invalid UTF-8 becomes U+FFFD)
- [ ] 2.5 Verify: `go test -run 'TestWriteTableText' ./internal/tui/`, `go test -run 'TestCLI/tui_no_terminal' ./tests/cli/` and `cd website && npm run build` pass

## 3. A Start on a server that is not Created changes nothing (design D3)

- [ ] 3.1 Add serverbase tests: a cancelled-context `TransitionToStarting` on Starting, Running, Stopping, Stopped and Failed returns an error naming the state and leaves state, `LastError()` and the server context unchanged; on Created it still fails the server. Add sshserver and tuiserver tests for a second `Start` with a cancelled context on a running server (still serving, later `Stop` clean) and during a held first `Start` (spec `server-lifecycle-base`). See them fail on the base
- [ ] 3.2 Reorder `TransitionToStarting` in `internal/core/serverbase/base.go` as D3 says, keeping F1's single critical section and F2's terminal CAS
- [ ] 3.3 Update the comments in `internal/sshserver/server_lifecycle.go` (`Start` terminal check, `Stop` godoc), the server skill (`.agents/skills/server/SKILL.md` Start rule 1 caveat and test checklist) and the main-spec wording through this change's delta
- [ ] 3.4 Formal: add `FixStartGuard`, the history variable and `CancelledStartChangesOnlyCreated` to `formal/tla/Serverbase.tla`; add `cancelledStartGuardFixed` and `mutantPreFixCancelledStartFailsAny` to `formal/manifest.toml`, set the base constant, re-measure `distinct_states`; update `sequentialStep` in `base_formal_test.go`, the trace harness if it takes the cancelled branch, and the "Starting steps" correspondence row
- [ ] 3.5 Verify: `go test -race ./internal/core/serverbase/ ./internal/sshserver/ ./internal/tuiserver/`, `python3 scripts/formal.py correspondence`, and `make formal` (Java is needed; otherwise state that the required CI check runs it)

## 4. F11: Stop closes every connection (design D4)

- [ ] 4.1 Invert the replay as `TestHostCallbackToken_StopClosesAuthenticatedConnection` in `internal/sshserver/token_formal_test.go`; add a Stop-during-handshake test (a test hook in the password handler holds the login while `Stop` runs) and a revocation-racing-Stop test; see the replay fail on the base
- [ ] 4.2 Implement D4 in `internal/sshserver` (server-owned connection set and WaitGroup in the `ConnCallback` wrapper, refusal once a teardown began, `tokenConn.Close` bookkeeping, `teardown` closes every connection and calls the library's `Close`); keep the serve gate, `stopMu` and the `tornDown` publish rule of #213
- [ ] 4.3 Formal: `HostCallbackToken` base constant `StopClosesSessions = "TRUE"`; rename `findingF11StopKeepsSessions` to `mutantPreFixF11StopKeepsSessions` (no `finding` key, `mutant_of = "noSessionAfterStopFixed"`); update the calibration text, `distinct_states`, the "Stop" correspondence row, the model comment at `HostCallbackToken.tla:18-21`, the trace harness's Stop check, and the F11 row of `formal/README.md` (Fixed, `(shipped)`)
- [ ] 4.4 Rewrite the Purpose of `openspec/specs/host-callback-server-lifecycle/spec.md` (#117 no longer applies; Stop closes connections) and the server skill's Stop section
- [ ] 4.5 Verify: `go test -race -count=3 ./internal/sshserver/` (every #213 lifecycle test included), `python3 scripts/formal.py correspondence`, `make formal-traces` and `make formal` when Java is available

## 5. TestWatcherDebounce on Windows (design D5)

- [ ] 5.1 Add a deterministic coalescing test on `fakeWatcherBackend` in `internal/watch/watcher_test.go`: three events in one window give exactly one callback with all three paths and no second callback
- [ ] 5.2 Change `TestWatcherDebounce` to the assertions of D5 (first callback in time, only the three paths, union of all three until two quiet debounce periods), with a comment citing run 45517294042 and the late Windows change notification
- [ ] 5.3 Verify: `go test -race -count=20 -run 'TestWatcher' ./internal/watch/` passes locally; the fix stage names the passing "Test (windows)" job of the PR head in its result

## 6. acp-go-sdk 0.20.0 (design D6)

- [ ] 6.1 Under the heavy lock, `go get github.com/coder/acp-go-sdk@v0.20.0` and `go mod tidy`; check with `git diff go.mod` that only that line moved and that the version is exact
- [ ] 6.2 Migrate `internal/acpclient` per D6 (elicitation methods with method-not-found, `Patch` reads, `int64` fields with clamping, `TerminalId`); grep the guide's silent-change list and record the result in the commit body
- [ ] 6.3 Add a test that an elicitation request from a fake agent gets method-not-found, the capability is not advertised, and the session still completes a prompt turn; existing acpclient tests pass
- [ ] 6.4 Dep-audit: `make vulncheck`, licence check of v0.20.0 (Apache-2.0), changelog review 0.13.5 -> 0.20.0 noted in the commit body
- [ ] 6.5 Verify: `go test -race ./internal/acpclient/` and `go build ./...` pass

## 7. Provisioned binary is the checked copy (design D7)

- [ ] 7.1 Add tests in `internal/provision` with a seam between resolution and build that replaces the file at `binary_path` with a Mach-O stub, a script and a different Linux ELF (after the key, and after the check before the key); the fake engine records the build context; assert the named error and no build, and see them fail on the base. Add a test that an unchanged binary yields the same tag and build context as before
- [ ] 7.2 Implement D7: `calculateCacheKey` returns the binary hash, `binaryResolution` carries the checked architecture, `prepareBuildContext` copies once (open, regular-file check, hash while writing), checks and compares the copy, and fails with an error naming the path
- [ ] 7.3 Run `python3 scripts/formal_mutation.py plan --out <scratch>` and `symbols` on the changed provision functions and record the labels
- [ ] 7.4 Verify: `go test -race ./internal/provision/` passes, including #214's cache-key and drift tests

## 8. Unix -i hang-up for a command that never reads (design D8)

- [ ] 8.1 Write the probe test in `internal/app/commandadapters/interactive_session_unix_test.go` (command that never reads, input filled, session ended, SIGHUP observed within a bound while the test process runs) and see it fail on the base
- [ ] 8.2 Implement the non-blocking duplicate of the master and the terminal adapter of D8; or, if the probe shows it needs an xpty or creack/pty change, revert the item's code, delete `specs/interactive-mode-terminal/spec.md` from this change, mark 8.2-8.3 with a note, and record the finding for the result
- [ ] 8.3 Verify: `go test -race -run 'Interactive' ./internal/app/commandadapters/ ./internal/tui/` and `go test -run 'TestInteractiveViewer' ./tests/cli/` pass on Linux; the macOS CI job passes the probe

## 9. Formal-bindings refresh and integration

- [ ] 9.1 After the review fixes and after W1a has merged and this branch is rebased: `python3 scripts/formal_mutation.py plan --out <scratch>` and `symbols` for every rewritten bound symbol (D9); on `-> refresh`, one report-only `make mutation-formal` in a throwaway worktree, `make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=...`, triage of every added entry, the reruns it names, and `python3 scripts/formal_mutation.py triage --check`
- [ ] 9.2 Verify: `python3 scripts/formal.py correspondence`, `make formal-reach`, `make test-scripts` and `make formal` (when Java is available) pass

## 10. Completion gates (.agents/rules/checklist.md)

- [ ] 10.1 `make tidy`; `make license-check` for new Go files
- [ ] 10.2 `make lint`, fixing every issue
- [ ] 10.3 `make test`
- [ ] 10.4 `make check-baseline`, triaging every new goplint finding
- [ ] 10.5 `make check-file-length`
- [ ] 10.6 `make sonar-local`
- [ ] 10.7 Conditional checks: `make test-scripts` (formal changed), `python3 scripts/formal.py correspondence` and `make formal`, `make check-agent-docs` (server skill changed), `cd website && npm run build` (docs changed)
- [ ] 10.8 `/learn`

## Workflow follow-up

- Archive the change after review (`openspec-archive-change`); the archive applies the deltas, and the Purpose edit of task 4.4 is already in the main spec.
- In the result: PR #190 is closed already; the drawn-table cell sanitising and the double-Start finding number are open questions for the maintainer; the provision copy now installs a symlinked `binary_path` (D7).
