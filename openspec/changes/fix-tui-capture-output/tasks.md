# Tasks

Scope (maintainer decision 39): this change is Part A only. The no-server `-i` path, the gate test, the `tui-server-exposure` deltas, the config-default change and the client fallback belong to S5b, a later change; do not implement them here. Their drafts are recoverable from commits 9d8483ca and 97d1e6a5.

## 1. Terminal resolution in `internal/tui`

- [x] 1.1 Add the D1 resolver to `internal/tui`: `tea.OpenTTY`, then stdin+stderr when both are terminals, otherwise `ErrNoTerminal`, a sentinel with a typed wrapper that names the component. Inject the opener, the terminal checks and the console probe so they can be unit-tested. Verify with `go test -run TestResolveComponentTerminal ./internal/tui/`, which covers each D1 table row, including a failing opener with terminal stdio and with non-terminal stdio, and a successful opener whose console probe fails.
- [x] 1.2 Windows console check (D1 step 1): in a `_windows.go` file, probe both opened handles with `windows.GetConsoleMode`; on failure close both and treat the open as failed. Close `CONIN$` when the open fails halfway. Add a Windows-only test asserting that a regular temp file fails the real probe. Verify with `make check-windows-build` and, on the Windows CI leg, `go test -run 'TestResolveComponentTerminal|TestConsoleProbe' ./internal/tui/`.
- [x] 1.3 Pass the resolved input and output to every directly rendered `tea.NewProgram`, and close the terminal after `Run`. The call sites are `input.go:137`, `choose.go:386`/`:481`, `filter.go:206`, `file.go:142`, `write.go:141`, `table.go:157`, `confirm.go:189`, `pager.go:165` and `spin.go:468`. Leave `interactive_session.go:33` unchanged. Verify that `rg -n 'tea.NewProgram\(' internal/tui` shows only resolver-backed calls plus the viewer.
- [x] 1.4 Remove the unused `tui.Config.Output` field and its default, and update the two tests that read it: `internal/tui/tui_test.go:140` (`TestDefaultConfig`) and `internal/tui/tui_write_test.go:164`. Verify with `rg -n 'Config.Output|cfg.Output|\.Output:' internal/tui` (no hits for the field) and `go build ./... && go test ./internal/tui/`.
- [x] 1.5 Spin and pager without a terminal (D3). `SpinWithAction` resolves the terminal before it starts the action goroutine; on `ErrNoTerminal` it runs the action synchronously once; if the spinner fails after the action started, it waits on `doneCh` before returning. `Pager` writes the content to stdout. Verify with unit tests that inject a failing resolver: the spin test counts action calls and asserts exactly one; the pager test asserts the content on stdout. Then `go test -run 'TestSpinWithAction|TestPager' ./internal/tui/` and `go test -run 'TestTuiSpin|TestTuiPager' ./cmd/invowk/`.
- [x] 1.6 Make the adapters surface `ErrNoTerminal` as a one-line error with exit 1 and empty stdout. Verify with a testscript `tests/cli/testdata/tui_no_terminal.txtar` that runs each case under `setsid` with stdio redirected, so that a local run cannot open the developer's `/dev/tty`: `input` exits 1 with `needs a terminal` on stderr and empty stdout; `spin -- sh -c 'echo ok; exit 3'` prints `ok` and exits 3; a spin that appends to a file leaves exactly one line; `printf 'x\n' | invowk tui pager` prints `x` and exits 0. Gate it with `[!exec:setsid] skip` (Unix only). It is not a virtual test, so it needs no mirror.
- [x] 1.7 Run `make lint` and `go test ./internal/tui/ ./cmd/invowk/`.

## 2. Capture tests, CI and docs

- [x] 2.1 Add `requireTmux` to `tests/cli/tui_tmux_test.go` and use it in `newTmuxSession` in place of the bare skip: with `INVOWK_TEST_REQUIRE_TMUX=1` a missing tmux or a failed session start fails the test; otherwise it skips as today. Set `INVOWK_TEST_REQUIRE_TMUX: "1"` in `.github/workflows/ci.yml` on the "Run CLI integration tests (full)" step only (the Linux Docker and Podman legs). Verify locally with `INVOWK_TEST_REQUIRE_TMUX=1 PATH=<path without tmux> go test -run TestTUI_Input ./tests/cli/` failing, and passing with tmux.
- [x] 2.2 Extend `tests/cli/tui_tmux_test.go` with capture variants. Each assertion compares `od -An -tx1` hex of the captured value with the expected bytes, and checks that the title was visible before keys were sent. Cover:
  - `v=$(invowk tui input)`, `choose`, `filter` (options piped), `write`, `file` and `table --selectable`;
  - `out=$(invowk tui spin -- echo done)`;
  - `input` with `2>err.log` (assert `err.log` holds no ESC byte);
  - `input </dev/null`.
  These run on Linux only (D4). Keep `TestTUIExemptionTmuxCoverage` green. Verify with `go test -run 'TestTUI_' ./tests/cli/` under tmux.
- [x] 2.3 Calibrate: temporarily drop the `tea.WithOutput` option at the input call site, confirm the captured-input test fails with control bytes in the hex, then restore it. Record the result in the PR description.
- [x] 2.4 Update the `invowk tui` docs: README, `website/docs` TUI pages, and their pt-BR copies. Say that the UI draws on the terminal and stdout carries only the result, and give the no-terminal behaviour per component (D3). Do not change the interactive-mode refusal text (S3 stands). Verify with `cd website && npm run build`.
- [ ] 2.5 Run `make test-cli`. After the push, check the CI job logs of both Linux full legs: every `TestTUI_*` capture test ran and passed, none skipped. Record the run and job ids in the PR description.

## 3. Tracker and completion gates

- [x] 3.1 Keep `.claude/issues/cst-adoption-gaps.md` current: N11 says S5a ships Part A and S5b holds the gate.
- [ ] 3.2 Run the completion gates of `.agents/rules/checklist.md`:
  - `/simplify`;
  - `make tidy` and `make license-check` (new Go files);
  - `make lint`;
  - `make test`;
  - `make check-baseline` (triage every goplint finding);
  - `make check-file-length`;
  - `make sonar-local`;
  - the conditional checks: `make check-agent-docs` (the tracker changed), `make test-scripts` if `scripts/` changed, and `cd website && npm run build`.

## Workflow follow-up

- Archive this change after merge, adding the new `tui-component-terminal-io` spec. `tui-server-exposure` is not synced by this change.
- Propose S5b after this change merges (decision 39).
- `/learn` after the implementation lands.
