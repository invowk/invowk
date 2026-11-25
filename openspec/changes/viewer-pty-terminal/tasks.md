# Tasks

## 1. Controlling terminal (commit 1)

- [ ] 1.1 Add the platform hook of design D1 (`interactive_session_unix.go` with `//go:build unix`, `interactive_session_other.go` with `//go:build !unix`, both in `internal/app/commandadapters/`): on Unix set `Setsid`, `Setctty`, `Ctty: 0` and stdio to the pty slave, and refuse a prepared command that already set `Stdin` or `SysProcAttr`; call it from `runInteractiveCmd` before `pty.Start`. Verify with a unit test of the refusal and `make check-windows-build`.
- [ ] 1.2 Create the pty at the command-area size (width x (height - 3)) through a helper exported by `internal/tui` that shares the header/footer constants with `handleWindowSize`, and resize the pty in every running state, `stateTUI` included (design D2). Verify with a model unit test that a `WindowSizeMsg` in `stateTUI` resizes the terminal fake.
- [ ] 1.3 Unix integration test in `internal/app/commandadapters` (skipped on Windows, per the windows-testing skill): run `sh -c 'tty; ps -o tty= -p $$'` through `runInteractiveCmd`'s start path with a test `InteractiveTerminal` reader and assert the two names match; and a `sleep 30` child that `0x03` written to the master ends with a signal well before 30 s. Verify with `go test -race -run 'TestInteractive.*Ctty|TestInteractive.*Interrupt' ./internal/app/commandadapters/`.
- [ ] 1.4 tmux test file `tests/cli/interactive_viewer_tmux_test.go` (follow `.agents/skills/tmux-testing/SKILL.md`; 100x30 pane; a temp invowkfile with a native `pick` command running the diagnosis recipe): no-server run passes 5 of 5 (`CHOSEN=[beta]`, `NAME=[Ana]`, no `^[[B`), the server control run passes, terminal identity (`tty` equals `ps -o tty=`), `stty size` prints `27 100`, Ctrl+C ends `sleep 30` with a failed banner, Ctrl+\ on `sleep 300` leaves no `sleep` process. Verify with `go test -run 'TestInteractiveViewer' ./tests/cli/` on Linux with tmux.
- [ ] 1.5 Docs for commit 1: README "Interactive Mode" and `website/docs/advanced/interactive-mode.mdx` (+ pt-BR copy under `website/i18n/pt-BR/`) say that the command gets the viewer as its terminal (`/dev/tty`, `sudo`/`ssh` prompts inside the viewer), and that Ctrl+C, resize and Ctrl+\ reach it as SIGINT, SIGWINCH and SIGHUP on Unix; update the `interactive/key-bindings-executing` snippet. Verify with `cd website && npm run build`.
- [ ] 1.6 Commit 1 (signed, under the heavy lock, backgrounded with an `rc=` line): `fix(tui): give -i commands the viewer pty as controlling terminal`. Verify `git log -1 --show-signature`.

## 2. Dependency

- [ ] 2.1 `go get github.com/charmbracelet/x/vt@v0.1.0` under the heavy lock, then `make tidy`; confirm the `go.mod` diff adds only `github.com/charmbracelet/x/vt v0.1.0` (direct) and `github.com/charmbracelet/x/exp/ordered v0.1.0` (indirect) and changes no other version (design D4). Verify with `git diff go.mod` and `make vulncheck`.

## 3. Terminal emulation (commit 2)

- [ ] 3.1 Replace `content strings.Builder` and the live viewport in `interactiveModel` with a `*vt.Emulator` sized to the command area (design D3): `outputMsg` carries bytes and is written in `Update`; `View` renders header, `Render()`, footer, the overlay on top and the child's visible cursor; resize keeps emulator and pty equal; remove `stripOSCSequences` and its test; write server styled text into the emulator with `\n` -> `\r\n`. Verify with model unit tests: a cursor-up repaint leaves one frame, `\r` progress leaves `done`, an OSC split across two `outputMsg`s draws nothing, styled text appears on the screen.
- [ ] 3.2 Reply pump (design D5): one goroutine copies `Emulator.Read` to the terminal's `Write`, stopped by closing the input pipe writer at session end, never `Emulator.Close`. Verify with `go test -race` tests that a DA1 query gets `ESC [ ?…c`, a CPR query from 1;1 gets `ESC [ 1 ; 1 R`, a flood of queries with a terminal fake that never reads does not block `Update`, and the pump goroutine ends with the session.
- [ ] 3.3 Key encoding (design D6): convert `tea.KeyPressMsg` to `uv.KeyPressEvent` (clear `IsRepeat`, `BaseCode`, `ShiftedCode`, and `Text` under Ctrl), drop special keys vt does not encode, send through `Emulator.SendKey`; forward `tea.PasteMsg` through `Emulator.Paste`; keep Ctrl+\ as force quit; remove `forwardKeyToPty`. Verify with a table test over Ctrl+A..Z, Alt+x, Enter, Tab, Shift+Tab, Backspace, Esc, arrows in normal and application-cursor mode, Home/End/PgUp/PgDn/Delete, F1, shifted and non-ASCII text, a repeated key, and bracketed and plain paste.
- [ ] 3.4 History and completed view (design D7): scrollback size 10,000 set explicitly; on `doneMsg` leave a stuck alternate screen; build the completed viewport from styled scrollback lines, the main screen without trailing blank rows, then the banner; keep the completed-state keys and scroll percentage. Verify with unit tests: `seq 1 500` output browsable from `1` to the banner, `seq 1 20000` keeps at most 10,000 history lines with `20000` above the banner, alternate-screen content absent from history.
- [ ] 3.5 Completion ordering (design D8): close the parent's slave copy after `Start` on Unix (ignore `os.ErrClosed` on the later `Close`), sequence reader end and waiter with the bounded grace period, ignore `outputMsg` after `doneMsg`. Verify with a `-race` unit test using a terminal fake that delivers its last chunk after `wait` returns (the chunk precedes the banner), one whose reader never ends (banner after the grace period), and a run of `seq 1 50; echo LAST-LINE` through the real adapter 20 times with `LAST-LINE` above the banner each time.
- [ ] 3.6 Update the existing tests of `internal/tui/tui_interactive*_test.go` to the emulator model (content assertions read the rendered screen or completed viewport) and keep `TestInteractiveModel_ConcurrentOutputWrites` meaningful or replace it with an event-loop ordering test. Verify with `go test -race ./internal/tui/ ./internal/app/commandadapters/`.
- [ ] 3.7 Extend `tests/cli/interactive_viewer_tmux_test.go`: one `Pick one` frame with `beta` marked in place, help line exactly `enter submit • esc cancel`, header and footer intact; `> Ana` on the prompt row; `seq 1 200 | less` shows its first page and `q` returns to the main screen; `seq 1 500` scrolls to `1` after completion; Ctrl+A reaches `od` as 001; `LAST-LINE` above the banner on 5 of 5 runs. Verify with `go test -run 'TestInteractiveViewer' ./tests/cli/`.
- [ ] 3.8 Docs for commit 2: README and `website/docs/advanced/interactive-mode.mdx` (+ pt-BR) describe screen emulation (repainting and full-screen programs render), the 10,000-line history and that it is browsable after completion, that queries are answered inside the viewer, and the key forwarding (all Ctrl+letters, paste); update the `interactive/key-bindings-*` snippets; check `docs/architecture/` for the viewer (no diagram change expected). Verify with `cd website && npm run build`.
- [ ] 3.9 Commit 2 (signed, under the heavy lock, backgrounded with an `rc=` line): `fix(tui): emulate a terminal screen in the -i viewer`. Verify `git log -1 --show-signature`.

## 4. Completion gates (`.agents/rules/checklist.md`)

- [ ] 4.1 `make tidy` and `make license-check` (new Go files).
- [ ] 4.2 `make lint` (all issues, pre-existing included).
- [ ] 4.3 `make test` (full suite).
- [ ] 4.4 `make check-baseline` and triage every new goplint finding.
- [ ] 4.5 `make check-file-length`.
- [ ] 4.6 `make sonar-local`: no new code-fixable issues (go:S8209, S8193, S1192, S107, S8239).
- [ ] 4.7 Conditional checks: `cd website && npm run build`; `make check-agent-docs` only if agent docs changed; formal correspondence is not affected (no bound symbol).
- [ ] 4.8 `/learn` (minimal).

## Workflow follow-up

- Maintainer asks for the result: lifting S3 (in-container `invowk tui` without the TUI server) with this change's evidence; a manual Windows ConPTY check of `-i`; whether the history cap should be configurable.
- Archive the change after review (fix stage).
