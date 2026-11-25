# Design

## Context

Motivation: proposal.md, Why. Requirements: `specs/interactive-mode-terminal/spec.md`. Current state on base 7b16cd50:

- The adapter creates the pty at the full outer-terminal size (`internal/app/commandadapters/interactive_session.go:19-27`) and starts the command with `pty.Start(cmd)` (`:34`). `xpty.UnixPty.Start` (xpty v0.1.4 `pty.go:90-104`) only fills `Stdin`/`Stdout`/`Stderr` with the slave when they are nil and calls `cmd.Start()`: no `Setsid`, no `Setctty`. Nothing in `cmd/` or `internal/` sets `SysProcAttr`. The command's controlling terminal is therefore invowk's own terminal.
- The prepared commands never set stdio: native builds them with `exec.CommandContext` and no stdio (`internal/runtime/native.go:431`); virtual-sh, virtual-lua and container commands are child processes prepared the same way (`internal/runtime/sh.go:290`, `lua_interactive.go:137`, `container_prepare.go:28`).
- Since #175 the components open `/dev/tty` when they render directly (`internal/tui/terminal_unix.go:24`), so without a TUI server they read the real terminal and race the viewer.
- The viewer: `RunInteractiveSession` (`internal/tui/interactive_session.go:18-54`) starts two independent goroutines, the output reader and the waiter (`:38-41`); their `Send`s are unordered, which is why trailing output can land below the banner. The reader strips OSC sequences and sends text (`:56-70`, `interactive_helpers.go:21-34`); the model appends it to a `strings.Builder` (`interactive.go:100`, `interactive_model.go:37-47`) shown in a bubbles viewport; `doneMsg` appends the banner to the same builder (`interactive_model.go:49-57`, `:428-471`).
- Keys: `forwardKeyToPty` (`interactive_model.go:330-391`) maps a fixed table; Ctrl+letter other than c/d/z is dropped (`:337-346`); no application-cursor mode, no paste.
- Resize: the pty is resized to width x (height - 3) only in `stateExecuting` (`interactive_model.go:394-425`, header 2 rows and footer 1 at `:396-397`).
- Server-delivered styled text (`ComponentTypeWrite`) is appended to the builder (`interactive_model.go:238-272`).
- The sshserver pty uses `creack/pty.Start` (`internal/sshserver/pty_unix.go:18-20`), which already sets `Setsid` and `Setctty` (creack/pty v1.1.24 `start.go:22-23`); its Windows path uses pipes (`pty_windows.go`). No other place starts a child on a pty.

## Goals / Non-Goals

**Goals:**
- Commit 1 (controlling terminal): the Unix session and controlling-terminal setup, its signal consequences, and its tmux tests.
- Commit 2 (terminal emulation): screen emulation, history, query answering, key encoding, the completion ordering fix, and their tests and docs.

**Non-Goals:**
- Lifting the refusal of `invowk tui` inside containers without the TUI server (S3): a maintainer decision.
- Mouse forwarding to the command, and scrolling the history while the command runs (the live screen is shown until completion; history browsing is a completed-state feature, as the keys go to the command while it runs). Today the mouse wheel scrolls the log while the command runs (`interactive_model.go:80-84`, mouse mode in `View` at `:155`); that goes away and is listed as a user-visible change (proposal Impact, docs task 3.8) and as a maintainer ask.
- Reporting the outer terminal's real colors to the command: color queries get the emulator's defaults.
- `internal/tui` value/display components (spin, pager, table: N14/N15) are not edited.
- Kitty keyboard protocol or modifyOtherKeys toward the command: the command gets legacy encodings only.

## Decisions

### D1. New session with the pty slave as controlling terminal (Unix only)

`runInteractiveCmd` calls a platform hook before `pty.Start`. On Unix (`//go:build unix`, e.g. `interactive_session_unix.go`) it sets `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}` and sets `cmd.Stdin`, `cmd.Stdout` and `cmd.Stderr` to the slave explicitly (the `*xpty.UnixPty`'s `Slave()`), so `Ctty: 0` (a child descriptor number since Go 1.15) always names the slave. If a prepared command already has any of `Stdin`, `Stdout` or `Stderr` set, or a `SysProcAttr`, the hook returns an error and the run fails before start, rather than silently giving the command a different controlling terminal or discarding a stream the runtime chose; no runtime sets any of them today (Context). On other platforms (`//go:build !unix`) the hook does nothing: ConPTY attaches the process to the pseudo-console, and `CONIN$`/`CONOUT$` already open it.

Alternatives: calling `creack/pty.StartWithAttrs` (would replace xpty and its ConPTY support); `Setsid` without `Setctty` (the first terminal the child opens would become controlling, but `/dev/tty` would fail before that and the shell would have no job control).

### D2. Signal consequences (decided)

- Ctrl+C: the viewer keeps forwarding it as `0x03`; with a controlling terminal the line discipline now turns it into SIGINT for the foreground process group (before, the pty belonged to no session, so it had no foreground group to signal; inferred from the line discipline, not measured). Raw-mode programs still read it as a key. Ctrl+\ stays reserved for the viewer's force quit, so SIGQUIT cannot be typed, as today.
- SIGWINCH: `Resize` on the master now signals the foreground group. The pty is created at the command-area size (width x (height - 3)) instead of the full terminal size, so the command does not see a size change at its first frame; the viewer resizes the pty and the emulator on every `WindowSizeMsg` while the command runs, in `stateTUI` too (today only in `stateExecuting`, `interactive_model.go:420`).
- SIGHUP on early exit: when the viewer ends early (Ctrl+\, context cancellation), the deferred `pty.Close` closes the master and the kernel hangs up the session. Commands no longer linger on a dead pty.
- SIGHUP on leader exit (decided; a consequence of D1 that cannot be avoided while the command's main process is the controlling process): when the command's main process (the session leader) exits, the kernel sends SIGHUP (and SIGCONT) to the terminal's foreground process group. A shell script runs without job control, so a job it starts with `&` stays in the shell's process group, which is the foreground group: it ends with the script unless it ignores SIGHUP or detached itself (`nohup`, `setsid`). Before this change the pty had no session and such jobs kept running; a run without `-i` keeps them too. This is what closing a terminal window does, so it is specified (spec "Background jobs end with the command") and documented (task 1.5), and the maintainer is asked whether it is wanted. The kernel behaviour is inferred from the line discipline; task 1.3 measures it with a recorded pid.
- Ctrl+Z and Ctrl+D: the viewer forwards `0x1a` and `0x04`. Ctrl+D is end-of-file in canonical mode, as in any terminal. Ctrl+Z has no effect on the command: its process group has no parent outside the new session (invowk is in another session), so it is orphaned and Linux discards the SIGTSTP the terminal generates; a raw-mode program still reads `0x1a` as a key. The docs must not imply job control.
- `sudo`, `ssh` and other `/dev/tty` prompts now draw inside the viewer.
- Container runs: the engine CLI (`docker`/`podman run -it`) becomes the session leader on the pty; it already treats its stdin as the terminal, and now also receives SIGWINCH. The processes inside the container are not in the pty's session: a hang-up reaches the engine CLI only, and whether the container's processes end then is the engine's behaviour. The spec and docs promise nothing about them.

### D3. Screen emulation with `github.com/charmbracelet/x/vt` v0.1.0

The model replaces `content strings.Builder` and the live viewport with a `*vt.Emulator` sized to the command area. All emulator calls (`Write`, `Resize`, `SendKey`, `Paste`, `Render`, scrollback reads) happen on the Bubble Tea event loop: output arrives as `outputMsg` bytes and `Update` writes them to the emulator, so the plain `Emulator` is used, not `SafeEmulator`. `View` renders the header, `Render()` of the screen (styled), and the footer; the server overlay keeps drawing on top (`RenderOverlay`). The child's cursor is shown at its emulated position (offset by the header) when it is visible, using the Bubble Tea view cursor. OSC stripping (`stripOSCSequences`) is removed: the emulator's parser keeps state across writes, so fragmented sequences are parsed, not drawn. Window title and other callbacks are ignored.

Server-delivered styled text (`ComponentTypeWrite`) is written into the emulator with `\n` translated to `\r\n`, so it appears where the command's output is, as today.

Alternatives considered: `github.com/hinshun/vt10x` (last version a 2022 pseudo-version, repository idle since 2023-12) and `github.com/ActiveState/vt10x` (v1.3.1 of 2020-08, idle since 2023-12): both unmaintained; a hand-written emulator is excluded by the brief.

### D4. Dependency evaluation (dep-audit)

- Version: `v0.1.0`, tag `vt/v0.1.0`, commit 43427fdb of `charmbracelet/x`, published 2026-10-12 (older than the two-week floor); a release, not a pre-release, and the only tagged version. Pinned exactly in `go.mod`.
- Licence: MIT (`LICENSE`, Charmbracelet, Inc.), compatible with MPL-2.0; its one new transitive module `github.com/charmbracelet/x/exp/ordered` v0.1.0 is MIT too.
- Graph: in a scratch copy of this `go.mod`, `go get github.com/charmbracelet/x/vt@v0.1.0` added only `vt` and `x/exp/ordered`; every other requirement (ultraviolet 6b8d4baf91b4, ansi v0.11.9, and so on) is already newer than vt asks and stays unchanged. `govulncheck ./...` on that probe: "No vulnerabilities found". The implement stage reruns `make vulncheck` on the real tree.
- Maintenance: `charmbracelet/x` is active (last push 2026-11-22, not archived); vt commits through 2026-10-12, used by Charm's own products (commit 67fa755d "fixes for interactive bash in crush"); open upstream PRs on vt (#1034 scrolled-off sink, #1038/#1029 scrollback size, #1039/#1044 grapheme split across writes).
- Probe behaviour (scratch program, not committed): a cursor-up repaint left only the new frame in the scrollback; DA1 (`CSI c`) was answered with `ESC [ ? 62 ; 1 ; 6 ; 22 c`; alternate-screen lines did not enter the scrollback and the main screen returned on `?1049l`.

### D5. Query answering: a reply pump and a pty writer

vt v0.1.0 writes every reply (DA1, DSR/CPR, DECRQM, OSC 10/11 colors, the in-band resize report of `Resize` at `emulator.go:255-259`) and every key and paste (`SendKey` at `key.go:306`, `Paste` at `emulator.go:315-322`) synchronously into one unbuffered `io.Pipe` (`emulator.go:101`, `Read` at `:264`). The `Write`, `Resize`, `SendKey` or `Paste` call that produces the bytes, which runs in `Update`, returns only after a reader of that pipe has taken them. A reader that then blocks on the pty therefore blocks `Update`, and with it the force quit: Bubble Tea's input goroutine only queues Ctrl+\, and `Update` has to process it. The spec review's scratch probe showed exactly this with a single pump that writes to the pty: after one DA1 reply the pump blocked in its pty write, and the second `ESC [ c` blocked the caller. So reading the pipe and writing the pty are separate:

- The pump goroutine reads `Emulator.Read` in a loop and hands each chunk to a bounded queue without blocking (a buffered channel; at most 64 KiB queued). When a chunk does not fit, the pump drops it and counts the drop. The pump never waits on anything but `Read`, so every emulator call in `Update` returns as soon as the pump has copied its bytes.
- The writer goroutine takes chunks from the queue and writes them to the pty master. It is the only party that can block when the command does not read its input.

Trade-off: when the command leaves its input unread long enough to fill the pty input buffer and then the queue, further replies and keys are dropped instead of stalling the viewer (a hardware terminal would stall). Ctrl+\ never goes through the queue, so it always works (spec "The viewer stays responsive when the command does not read its input").

Both goroutines are created together with the emulator by one constructor (for example `newEmulatedScreen(width, height, sink io.Writer)`), which the session and the model tests both use, so no test drives an emulator whose replies have no reader. At session end the session closes the emulator's input pipe writer (`InputPipe()`, an `*io.PipeWriter`, used as an `io.Closer`): the pump's `Read` returns EOF and the pump closes the queue; later replies fail inside vt with `io.ErrClosedPipe`, which vt ignores, so they do not block either. The writer ends when the queue is closed and drained, or when its write fails because the adapter closed the pty; the session does not wait for the writer, which may be blocked in a write the command never reads. The pump is never stopped with `Emulator.Close`: `Close` writes the unguarded `closed` field that `Read` reads (upstream reverted its fix, charmbracelet/x #850/#851), which `-race` would flag. Queries the emulator does not support get no reply (Kitty keyboard query, XTVERSION), which the command treats as a terminal without the feature. No query, mode change or other control sequence from the command reaches the outer terminal: the viewer never writes the command's bytes to its own output; only its rendering of the emulated screen (text, SGR attributes, hyperlinks, through `Render`) does.

### D6. Key encoding through the emulator

`tea.KeyPressMsg` converts to vt's `KeyPressEvent` (an alias of `uv.KeyPressEvent`) by field copy (`tea.KeyMod` is `uv.KeyMod`; key codes are the `uv` constants), normalized before `SendKey`, because vt matches whole structs and clears only `BaseCode` and `ShiftedCode` itself (vt `key.go:25-306`):

1. `IsRepeat`, `BaseCode` and `ShiftedCode` are cleared.
2. Lock modifiers are masked: `ModCapsLock`, `ModNumLock` and `ModScrollLock` are removed from `Mod`. uv keeps Caps Lock in `Mod` on Windows (`decoder.go:2034-2051`) and both Caps Lock and Num Lock for Kitty-protocol terminals (`decoder.go:1371-1376`), which the viewer's own Bubble Tea program requests (`cursed_renderer.go:932-933`); without the mask, Ctrl+letter, Enter, arrows and Backspace would match no binding and send nothing.
3. Ctrl+Shift+letter is sent as Ctrl+letter (Shift removed), as xterm does.
4. `Text` is cleared when Ctrl is held.
5. Fallback for modifiers vt has no form for: vt encodes only Ctrl+Space, Ctrl+letter, Ctrl+`[ \ ] ^ _` and Shift+Tab with a modifier (Alt is handled first, as an `ESC` prefix). A key without text that carries Shift, Ctrl or Meta outside those bindings (Ctrl+Left, Shift+Up, Shift+Enter, Ctrl+Backspace) has Shift, Ctrl and Meta removed and is sent as the plain key, Alt prefix kept, which is what the viewer does today (`forwardKeyToPty` switches on the code alone, `interactive_model.go:350-382`). A key with text and Ctrl that has no control character (Ctrl+1) is dropped, as today.
6. Keys whose code is a special key vt does not encode (its default branch would write `string(code)`, a private-use rune, `key.go:293-303`) are dropped.

The code uses vt's aliases (`vt.KeyPressEvent`, `vt.ModCtrl`, …) and Bubble Tea's modifier constants (`tea.ModCapsLock`, …), so it does not import `ultraviolet` directly (see task 2.1). `tea.PasteMsg` goes to `Emulator.Paste` (bracketed when the command enabled it). Ctrl+\ is still intercepted first and never reaches `SendKey`; the check ignores lock modifiers, so Caps Lock + Ctrl+\ still force quits. `forwardKeyToPty` is removed.

### D7. History and the completed view

History is the emulator's scrollback, size 10,000 lines (vt `DefaultScrollbackSize`, set explicitly). On `doneMsg`, if the emulator is still on the alternate screen (a command that crashed in a full-screen program), the model writes `ESC [ ? 1049 l` to return to the main screen. The completed view builds a viewport whose content is: every scrollback line rendered with its styles (`uv.Line.Render`), then the main screen's rows without trailing blank rows, then the banner (`appendCompletionMessage`, now writing to that content). The existing completed-state keys and the scroll percentage keep working on that viewport. Memory: scrollback lines are trimmed of trailing blank cells by vt (`scrollback.go:31-56`).

### D8. Completion ordering

On Unix the adapter closes its own copy of the slave right after `Start` succeeds, so the master read returns EIO once every process of the command has closed the slave and the buffered output has been read. The session sequences the two goroutines: the reader sends its `outputMsg`s and then signals its end; the waiter, after `wait` returns, waits for that end or for a bounded grace period (about 250 ms, for processes the command left running that keep the slave open, and for ConPTY, whose output pipe does not end with the process, so on Windows the timer is the normal path), then sends `doneMsg`. Program messages from one goroutine are delivered in order, and the reader's last `Send` happens before its end signal, so all output read before the end precedes `doneMsg`.

Output that arrives after `doneMsg` is not dropped: a tail that conhost flushes after the grace period on Windows, or output of a process the command left running (one that ignores SIGHUP, D2). The model keeps writing such `outputMsg`s to the emulator, marks the completed view stale, and rebuilds its content (history, main screen, banner last; D7) at most once per frame. The rebuild keeps the user's scroll offset, or stays at the bottom when the user was at the bottom. The banner is therefore always the last thing in the completed view, on every platform, and the spec's completion requirement holds on Windows too. The reader keeps draining until it ends or the session ends. `UnixPty.Close` closes the slave again; the adapter ignores `os.ErrClosed` from that second close so the deferred close does not turn a successful run into an error.

## Risks / Trade-offs

- [History is capped at 10,000 lines, the old viewer kept everything] -> documented in README and website; the cap matches common terminal defaults; raising it is one constant.
- [Grapheme clusters split across two pty reads can render wrong (upstream #1039)] -> cosmetic, limited to multi-codepoint clusters at a 4096-byte boundary; revisit when upstream merges #1044.
- [Signal behaviour changes (Ctrl+C now interrupts, SIGHUP on early exit)] -> intended and specified; listed in the release notes.
- [`Ctty: 0` depends on stdin being the slave] -> D1 sets stdio explicitly and refuses a prepared command that set it.
- [Late output after the grace period, normal on Windows ConPTY] -> written to the emulator and drawn above the banner by a rebuild of the completed view (D8); nothing is dropped and the banner stays last.
- [A command that never reads its input loses replies and keys once the pty input buffer and the 64 KiB queue are full] -> the alternative is a frozen viewer (D5); Ctrl+\ is never queued, so the user can always quit.
- [Background jobs started with `&` end when the command's main process exits (SIGHUP)] -> matches a terminal window closing; specified, documented, and a maintainer ask (D2).
- [The mouse wheel no longer scrolls while the command runs] -> listed as user-visible; history stays browsable after completion; a maintainer ask.
- [Windows not exercised by tmux tests] -> the Unix hook is a no-op there; the model unit tests (emulator, keys, lock modifiers, late output above the banner) run on every OS; a manual ConPTY check is a maintainer ask.
- [vt is v0.x] -> pinned exactly; the API used is small (NewEmulator, Write, Read, Resize, Render, SendKey, Paste, Scrollback, IsAltScreen, CursorPosition).

## Migration Plan

No configuration or schema change. Rollback is reverting the two commits; `go.mod` loses the two modules.

## Maintainer decisions

- 2026-11-25 (brief, group E): two fixes in two commits; evaluate `charmbracelet/x/vt` first and pin it; forward every Ctrl+letter; fix the banner race; S3 stays out of scope.

## Open questions

- Whether the history cap should become configurable (no requirement asks for it now).
- Whether losing live wheel scrolling (Non-Goals) and ending background jobs on leader exit (D2) are acceptable to the maintainer; both are specified as decided here and asked in the result.
