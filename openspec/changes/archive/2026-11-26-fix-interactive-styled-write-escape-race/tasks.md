# Tasks

## 1. ANSI/VT escape-sequence boundary buffer (`internal/tui`)

- [x] 1.1 Implement `ansiStreamBuffer` in `internal/tui/interactive_ansi_buffer.go` covering `GroundState`, lone `ESC`, 2-byte and intermediate `ESC` sequences, `CSI` (`ESC [` and `0x9B`), `OSC` (`ESC ]` and `0x9D` with `BEL`, `0x9C`, and split `ESC \` `ST`), `DCS`/`SOS`/`PM`/`APC` (`ESC P`/`X`/`^`/`_` and 8-bit equivalents, including `DCSEntry` + `0x1B` passthrough), `SS2`/`SS3` (`ESC N`/`ESC O` and `0x8E`/`0x8F`), multi-byte UTF-8 (`0xC2..0xF4`), `maxEscapeHoldBytes` (`64 KiB`) overflow flush, and `emuState` tracking, verifying that complete leading prefixes flush immediately at `lastSafeEnd` (`GroundState`).
- [x] 1.2 Add comprehensive unit tests in `internal/tui/tui_interactive_ansi_buffer_test.go` covering byte-by-byte and chunk-boundary splits across every sequence family (lone `ESC`, CSI, OSC with `BEL` and split `ST`, DCS/SOS/PM/APC, `SS2`/`SS3`, intermediate `ESC`, 2/3/4-byte UTF-8, UTF-8 inside OSC titles, interrupted sequences, `maxEscapeHoldBytes` overflow, and `emuState` transitions).
- [x] 1.3 Narrowest check: run `go test -race -run TestANSIStreamBuffer ./internal/tui/...` and verify every test passes.

## 2. Interactive viewer ground-state guard, bounded flush, and SGR isolation (`internal/tui`)

- [x] 2.1 Wire `ansiStreamBuffer` into `interactiveModel` (`internal/tui/interactive.go`, `internal/tui/interactive_model.go`) and `streamInteractiveOutput` (`internal/tui/interactive_session.go`): schedule generation-gated `escapeFlushMsg{seq}` (`interactiveEscapeFlushDelay = 25 * time.Millisecond`, injectable for deterministic/short tests), send `outputEndMsg{}` before `streamInteractiveOutput` exits on read error, flush pending bytes on `escapeFlushMsg`, `outputEndMsg`, and `doneMsg`, and apply `ensureEmulatorGroundState()` before `leaveAltScreen`.
- [x] 2.2 Update `handleStyledTextRequest` (`internal/tui/interactive_model.go`) so it preserves any incomplete command sequence still held in `ansiStreamBuffer`, calls `ensureEmulatorGroundState()` before writing, wraps styled output with `\x1b[0m` SGR resets (plus `CAN` ground-state cleanup before the trailing reset if `opts.Text` ends mid-sequence), and preserves synchronous `cmd == nil` behavior in `stateExecuting`.
- [x] 2.3 Add/update unit and concurrent session tests in `internal/tui/tui_interactive_model_test.go`, `internal/tui/tui_interactive_screen_test.go`, and `internal/tui/tui_interactive_session_test.go` proving split-sequence interleaving with `TUIComponentMsg{ComponentTypeWrite}`, bidirectional SGR attribute isolation, bounded idle/`outputEndMsg`/`doneMsg` flush of a lone `\x1b`, dirty-parser recovery before `handleStyledTextRequest` and `leaveAltScreen`, and race-free concurrent `RunInteractiveSession` execution.
- [x] 2.4 Narrowest check: run `go test -race -run TestInteractive ./internal/tui/...` and verify all interactive model, screen, key, and session tests pass.

## 3. Hardened tmux TUI readiness and output assertions (`tests/cli/tui_tmux_test.go`)

- [x] 3.1 Harden `TestTUI_Confirm`, `TestTUI_Choose/navigate_and_select`, `TestTUI_Input/type_and_submit` (fixing `--header` to `--title`), `TestTUI_Write/multiline_submit`, `TestTUI_Filter/dismiss_with_escape`, `TestTUI_File/cancel_with_escape`, `TestTUI_Table` (`select_row`, `display_on_terminal`), `TestTUI_Spin/run_command`, and `TestTUI_Pager/dismiss_with_q` in `tests/cli/tui_tmux_test.go` per `design.md` D5 using TUI-rendered chrome, `tuiInputSettleDelay`, line-focused assertions, and dynamically expanded markers while preserving `TestTUIExemptionTmuxCoverage` AST call shapes.
- [x] 3.2 Narrowest check: run `go test -v -run 'TestTUI_' ./tests/cli/...` (verifying zero `SKIP`s) and `go test -v -run TestTUIExemptionTmuxCoverage ./cmd/invowk/...`.

## 4. Integration verification and `.agents/rules/checklist.md` completion gates

- [x] 4.1 Verify long-`binaryPath` (>= 85 chars) tmux stability by running `TestTUI_` in `tests/cli/` against an `invowk` binary placed at a long path forcing column-100 command-line wraps in `100x30` panes and confirming zero failures or `SKIP`s.
- [x] 4.2 Run race-enabled suite across interactive TUI, adapter, server, and client packages: `go test -race ./internal/tui/... ./internal/app/commandadapters/... ./internal/tuiserver/... ./internal/tuiclient/...`.
- [x] 4.3 Execute `.agents/rules/checklist.md` completion gates from the worktree root: review changed code for reuse/simplification, `make tidy`, `make license-check`, `make lint`, `make check-baseline`, `make check-file-length`, `make check-windows-build`, `make sonar-local`, and full `make test` (verifying zero skipped/failed tmux or CLI tests).

## Workflow follow-up

- Run `/learn` (`.agents/skills/learn/SKILL.md`) as the final step after implementation and gates pass.
- [x] Archive the completed change via `openspec-archive-change` (synced to `openspec/specs/interactive-mode-terminal/spec.md` and archived).
