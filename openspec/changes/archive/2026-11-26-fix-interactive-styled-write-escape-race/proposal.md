# Proposal

## Why

In interactive mode (`invowk cmd ... -i`), command PTY output (`outputMsg`) and TUI-server styled text requests (`ComponentTypeWrite` / `tuiclient.Client.Write` -> `handleStyledTextRequest`) are delivered over separate transport channels (`streamInteractiveOutput` and `bridgeTUIRequests`) and written sequentially into a single stateful `vt.Emulator` without escape-sequence boundary coordination or SGR attribute isolation. When a PTY read splits an ANSI/VT escape sequence or multi-byte UTF-8 rune across chunks and a `ComponentTypeWrite` (or `doneMsg`) write interleaves mid-sequence, the injected bytes are swallowed by the incomplete parser state, corrupting both the escape sequence and the rendered styled text; independently, multiple `TestTUI_*` tests in `tests/cli/tui_tmux_test.go` poll for strings already visible in the echoed shell command line (failing under long `binaryPath` column-100 wrapping or passing vacuously without verifying TUI rendering).

## What Changes

- Add an ANSI/VT and UTF-8 sequence-boundary buffer (`ansiStreamBuffer`) for PTY `outputMsg` streams in `internal/tui/` that flushes complete text and complete sequences immediately while holding any incomplete trailing sequence (`ESC` alone, `ESC [` CSI, `ESC ]` OSC without `BEL`/`ST`, `ESC P`/`X`/`^`/`_` DCS/SOS/PM/APC without `ST`, `ESC N`/`O` and C1 `SS2`/`SS3`, intermediate `ESC` sequences, and incomplete multi-byte UTF-8) across read boundaries.
- Add bounded flush triggers on a short idle timer (`escapeFlushMsg` with sequence generation gating), buffer-cap overflow, stream end (`outputEndMsg`), and completion (`doneMsg`) so a legitimate lone `ESC` or truncated tail never stalls indefinitely.
- Guard `handleStyledTextRequest` (and `leaveAltScreen` on `doneMsg`) so writes to `m.screen.emu` occur only at a clean `GroundState` (neutralizing any parser state left non-ground by a prior forced idle/EOF flush, without disturbing any still-buffered incomplete command sequence) and isolate SGR attributes (`\x1b[0m` before and after rendered styled text, plus ground-state cleanup if user-supplied text ends mid-sequence).
- Harden `tests/cli/tui_tmux_test.go` across `TestTUI_*` (`TestTUI_Confirm`, `TestTUI_Choose`, `TestTUI_Input`, `TestTUI_Write`, `TestTUI_Filter`, `TestTUI_File`, `TestTUI_Table`, `TestTUI_Spin`, `TestTUI_Pager`) so readiness and completion assertions match unambiguous TUI-rendered chrome, line-level markers, and dynamically expanded output markers rather than substrings of the echoed shell command line (fixing `TestTUI_Input`'s invalid `--header` flag to `--title` and preventing column-100 line-wrap flakes under long `binaryPath` values while preserving `TestTUIExemptionTmuxCoverage` AST patterns).

## Capabilities

### New Capabilities

<!-- None: this change hardens existing interactive-mode-terminal behavior and tmux test assertions. -->

### Modified Capabilities

- `interactive-mode-terminal`: Clarify and enforce escape-sequence/UTF-8 chunk-boundary preservation with bounded idle/EOF flush on command output, and require `ComponentTypeWrite` styled output to write only at a clean emulator ground state with SGR attribute isolation.

## Impact

- `internal/tui/`: `interactive.go`, `interactive_model.go`, `interactive_session.go`, `interactive_screen.go` (or a new `interactive_ansi_buffer.go`), plus unit and race tests in `tui_interactive_model_test.go`, `tui_interactive_screen_test.go`, and `tui_interactive_session_test.go`.
- `internal/app/commandadapters/`: no behavioral changes required (the PTY-vs-HTTP transport concurrency is documented; protocol adapters remain unchanged).
- `tests/cli/`: `tui_tmux_test.go` (preserving `TestTUIExemptionTmuxCoverage` AST recognition in `cmd/invowk/cmd_coverage_test.go`).
- No CLI flags, CUE schemas, `go.mod` dependencies, or public APIs change; non-breaking.
