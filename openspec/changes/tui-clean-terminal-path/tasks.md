# Tasks

The brief asks for one commit per item: groups 1-3 make the N14 commit, groups 4-6 the N15 commit. Calibration logs go under the stage's scratch directory, each with a `head=` line; cite them in the commit body.

## 1. N14: `SpinWithAction` waits for its action (design D2)

- [x] 1.1 Red first: add `TestSpinWithAction_CancelWaitsForAction` to `internal/tui/tui_terminal_test.go`, written against the current `func()` signature. It injects a resolver that returns a dummy terminal, a spinner that returns nil at once, and an action that sleeps 50 ms and then writes a shared variable, which the test reads after `spinWithAction` returns. Run `go test -race -count=1 -run TestSpinWithAction_CancelWaitsForAction ./internal/tui/` on the unfixed code, confirm it FAILS (a race report or the action not finished), and save the log.
- [x] 1.2 Change `SpinWithAction` and `spinWithAction` to take `ctx` and `action func(context.Context)`. Add `SpinCancelWait` (15 s), `ErrSpinCancelled` (wraps `context.Canceled`) and `ErrSpinActionStillRunning`. Implement the three spinner-return rules of D2, with an internal bound parameter so tests can use a short one. Verify with `go build ./...`.
- [x] 1.3 Make `spinActionModel` return `tea.Interrupt` on `ctrl+c`. Keep `SpinBuilder.Action(func())` working through a wrapper, with its context from `SpinBuilder.Context` or `context.Background()`, and update the doc comments. Verify with the updated `TestSpinBuilder_*` tests and a new `spinActionModel` Ctrl+C test.
- [x] 1.4 Adapt 1.1 to the new signature: the action waits for its context and then writes. Add tests for nil and `ErrInterrupted` returns (both cancel, then wait), for a bound that expires with an action ignoring cancellation (`ErrSpinActionStillRunning`, returned within the bound), for a spinner failure (waits without cancelling and returns the spinner error), and keep the existing no-terminal and resolver-error tests. Verify with `go test -race -count=1 -run 'TestSpinWithAction|TestSpinActionModel|TestSpinBuilder' ./internal/tui/`.

## 2. N14: the spin adapter reports every outcome (design D1, D3)

- [x] 2.1 In `cmd/invowk/tui_spin.go`, pass the result through a buffered channel, read it without blocking after `SpinWithAction` returns, and implement the D3 outcome table: output printed whenever the result was read; status 1 with `spin: command cancelled` on `ErrSpinCancelled`; status 1 with no read on `ErrSpinActionStillRunning`; the spinner error as a stderr warning while the command's status stands; a status outside 0-255 becomes status 1 with `spin: command ended by a signal`. Change `tuiSpinAction` to the new signature. Verify with `go build ./...`.
- [x] 2.2 Add or extend tests in `cmd/invowk/tui_spin_test.go`, one per D3 row, with a fake `tuiSpinAction`, plus ExitCode -1 mapped to 1, a cancelled context mapped to 1, and `tuiSpinCommandWaitDelay < tui.SpinCancelWait`. Verify with `go test -race -count=1 -run 'TestRunTUISpin|TestTUISpin' ./cmd/invowk/`.

## 3. N14: end-to-end tests and docs

- [x] 3.1 Add the subtest `ctrl_c_cancels_command` to `TestTUI_Spin` in `tests/cli/tui_tmux_test.go`: run `spin --title Wait -- sh -c 'echo $$ > pid; echo started; exec sleep 30'; echo INVOWK_EXIT:$?`, wait for `Wait`, send `C-c`, then expect `INVOWK_EXIT:1` well before 30 s, `started` in the pane, and the pid no longer alive. Calibrate on the base binary (it shows `INVOWK_EXIT:0`) and save the log. Verify with `go test -count=1 -run 'TestTUI_Spin' ./tests/cli/` on Linux.
- [x] 3.2 Add a Unix-only Go test in `tests/cli` (build tag `!windows`). It starts `bin/invowk tui spin -- sh -c 'echo started; exec sleep 30'` with no terminal (`Setsid`) and stdin from `/dev/null`, sends SIGINT to its group after `started` appears, and asserts exit 1 and stdout `started`. Calibrate on the base (it exits 255) and save the log. Verify with `go test -count=1 -run <its name> ./tests/cli/`.
- [x] 3.3 Docs: in `website/docs/tui/table-and-spin.mdx` ("Exit Code Handling"), describe Ctrl+C (the command is stopped, status 1, output kept) and the status 1 for a command killed by a signal; add the `spin` row in `website/docs/tui/overview.mdx` "Exit Codes" if it needs one; mirror both in `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/tui/`; check the `tui spin` line of the CLI reference exit-code list (`website/docs/reference/cli.mdx`) and of README.md (tui section only) and add "1 when cancelled" where they list spin's codes. Snippets in `website/src/components/Snippet/data/tui.ts` only if they need it. Verify with `cd website && npm run build`.
- [x] 3.4 Commit N14, signed, under the heavy lock, in the background with an `rc=` line, waiting with Monitor: `fix(tui): stop the wrapped command on Ctrl+C in tui spin`. The body lists the D1 finding (native gives 1) and the calibration logs. Verify the commit with `git log --show-signature -1`.

## 4. N15: plain-text table writer (design D5)

- [x] 4.1 Add `WriteTableText(w io.Writer, headers []string, rows [][]string) error` in `internal/tui` (`table.go` or a new `table_text.go`, with the license header), implementing the spec's "Plain-text table format": `lipgloss.Width` for widths, two-space gaps, trailing spaces trimmed, `\n`, short rows padded, and every row printed. Verify with `go build ./...`.
- [x] 4.2 Add golden tests for the three spec scenarios ("Aligned rows", "Short rows and overridden titles", "Height does not cut rows" at the writer level), plus a wide-character row (CJK) that stays aligned, an empty cell, and no `\x1b` in the output. Verify with `go test -count=1 -run TestWriteTableText ./internal/tui/`.

## 5. N15: the stdout rule in the pager and table adapters (design D4)

- [x] 5.1 Add `stdoutIsTerminal(w io.Writer) bool` in `cmd/invowk`: true only for an `*os.File` that `term.IsTerminal` accepts. Use it in `runTuiPager`, after the content is read and before the server delegation, to write the content unchanged and return. Write to `cmd.OutOrStdout()`. Verify with `go build ./...`.
- [x] 5.2 Use it in `runTuiTable` when the table is not `--selectable`, after `splitHeadersAndData` and before `renderTableSelection`, to call `tui.WriteTableText` and return. `--selectable` keeps today's path. Verify with `go build ./...`.
- [x] 5.3 Add tests in `cmd/invowk`:
  - The pager with a buffer stdout gets the exact bytes, also with `--title` and `--line-numbers`.
  - The non-selectable table with a buffer stdout gets the spec's "Aligned rows" text.
  - With `INVOWK_TUI_ADDR` and `INVOWK_TUI_TOKEN` set to a test server (`startTableTestServer`, and a pager equivalent if needed), no request reaches the server for either component.
  - The selectable table with a server still sends its request (unchanged).

  Verify with `go test -race -count=1 -run 'TestRunTuiPager|TestRunTuiTable|TestRenderTUITable' ./cmd/invowk/`.

## 6. N15: end-to-end tests and docs

- [x] 6.1 In `tests/cli/testdata/tui_no_terminal.txtar` (notty, all three OSes):
  - The non-selectable table prints the spec's aligned rows and exits 0, both from `--file` and from stdin with `--separator '|'`.
  - The pager with `--title X --line-numbers` passes the bytes through.
  - Update the header comment.
  - Calibrate the table case on the base (it exits 1, `Table needs a terminal`) and save the log.

  Verify with `go test -count=1 -run 'TestCLI/tui_no_terminal' ./tests/cli/`.
- [x] 6.2 In `tests/cli/tui_tmux_test.go`, add two subtests, `TestTUI_Pager/stdout_redirected` (`pager f > out; echo INVOWK_EXIT:$?`) and `TestTUI_Table/stdout_piped` (`table --file f | cat > out; echo INVOWK_EXIT:$?`). Each expects `INVOWK_EXIT:0` with no key press and the expected file bytes. Calibrate on the base (both wait for a key until the timeout) and save the log. Verify with `go test -count=1 -run 'TestTUI_Pager|TestTUI_Table' ./tests/cli/` and `go test -run TestTUIExemptionTmuxCoverage ./cmd/invowk/`.
- [x] 6.3 Docs: in `website/docs/tui/pager.mdx`, state that a redirected stdout passes the content through like `less`; in `website/docs/tui/table-and-spin.mdx`, document the plain-text output of a non-selectable table with stdout redirected, with the example from the proposal; in `website/docs/tui/overview.mdx` ("Piping and Capture" and "Without a Terminal"), add the stdout rule and move `table` (non-selectable) out of the exit-1 list; mirror all three in pt-BR; update README.md's tui paragraph (line ~2823) only. Verify with `cd website && npm run build`.
- [ ] 6.4 Commit N15, signed, under the heavy lock, in the background with an `rc=` line, waiting with Monitor: `fix(tui): pass content through when pager or table stdout is redirected`. The body names the table format and the calibration logs. Verify the commit with `git log --show-signature -1`.

## 7. Completion gates (`.agents/rules/checklist.md`)

- [ ] 7.1 Run `make tidy` and `make license-check` (for the new Go files); both must be clean.
- [ ] 7.2 Run `make lint` under the heavy lock and fix every finding, including pre-existing ones. It must exit 0.
- [ ] 7.3 Run `make test` under the heavy lock, in the background with an `rc=` line; it must reach `rc=0`.
- [ ] 7.4 Run `make check-baseline` under the heavy lock and triage every new goplint finding; inconclusives block. It must exit 0.
- [ ] 7.5 Run `make check-file-length`: every touched Go file stays under 1000 lines.
- [ ] 7.6 Run `make sonar-local`, with no new code-fixable issues (go:S8209, S8193, S1192 outside tests, S107, S8239); S3776 is accepted under e5.
- [ ] 7.7 Conditional checks: `cd website && npm run build` (website changed). No `formal/`, `scripts/` or schema changes are expected: confirm with `git diff --stat origin/main`, and run `make check-agent-docs` only if `.agents/` changed.
- [ ] 7.8 Run `/learn` (`.agents/skills/learn/`) as the last step.
