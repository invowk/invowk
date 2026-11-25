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
- Mouse forwarding to the command, and scrolling the history while the command runs (the live screen is shown until completion; history browsing is a completed-state feature, as the keys go to the command while it runs).
- Reporting the outer terminal's real colors to the command: color queries get the emulator's defaults.
- `internal/tui` value/display components (spin, pager, table: N14/N15) are not edited.
- Kitty keyboard protocol or modifyOtherKeys toward the command: the command gets legacy encodings only.

## Decisions

### D1. New session with the pty slave as controlling terminal (Unix only)

`runInteractiveCmd` calls a platform hook before `pty.Start`. On Unix (`//go:build unix`, e.g. `interactive_session_unix.go`) it sets `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}` and sets `cmd.Stdin`, `cmd.Stdout` and `cmd.Stderr` to the slave explicitly (the `*xpty.UnixPty`'s `Slave()`), so `Ctty: 0` (a child descriptor number since Go 1.15) always names the slave. If a prepared command already has `Stdin` set or a `SysProcAttr`, the hook returns an error and the run fails before start, rather than silently giving the command a different controlling terminal; no runtime does this today (Context). On other platforms (`//go:build !unix`) the hook does nothing: ConPTY attaches the process to the pseudo-console, and `CONIN$`/`CONOUT$` already open it.

Alternatives: calling `creack/pty.StartWithAttrs` (would replace xpty and its ConPTY support); `Setsid` without `Setctty` (the first terminal the child opens would become controlling, but `/dev/tty` would fail before that and the shell would have no job control).

### D2. Signal consequences (decided)

- Ctrl+C: the viewer keeps forwarding it as `0x03`; with a controlling terminal the line discipline now turns it into SIGINT for the foreground process group (before, the pty belonged to no session, so it had no foreground group to signal; inferred from the line discipline, not measured). Raw-mode programs still read it as a key. Ctrl+\ stays reserved for the viewer's force quit, so SIGQUIT cannot be typed, as today.
- SIGWINCH: `Resize` on the master now signals the foreground group. The pty is created at the command-area size (width x (height - 3)) instead of the full terminal size, so the command does not see a size change at its first frame; the viewer resizes the pty and the emulator on every `WindowSizeMsg` while the command runs, in `stateTUI` too (today only in `stateExecuting`, `interactive_model.go:420`).
- SIGHUP: when the viewer ends early (Ctrl+\, context cancellation), the deferred `pty.Close` closes the master and the kernel hangs up the session; when the session leader exits, its foreground group gets SIGHUP. Commands no longer linger on a dead pty.
- `sudo`, `ssh` and other `/dev/tty` prompts now draw inside the viewer. Container runs: the engine CLI (`docker`/`podman run -it`) becomes the session leader on the pty; it already treats its stdin as the terminal, and now also receives SIGWINCH.

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

### D5. Query answering: a reply pump

The emulator writes its replies (DA1, DSR/CPR, DECRQM, OSC 10/11 colors, in-band resize when asked) to an `io.Pipe` (vt `emulator.go:101`, `Read` at `:264`); a write to that pipe blocks until it is read, and `SendKey`/`Paste` use the same pipe. So the session runs one pump goroutine: `Read` from the emulator, write to the pty master. Keys therefore travel model -> emulator encoding -> pump -> pty. The pump stops by closing the emulator's input pipe writer (`InputPipe()` as an `io.Closer`) when the session ends, never with `Emulator.Close`: `Close` writes the unguarded `closed` field that `Read` reads (upstream reverted its fix, charmbracelet/x #850/#851), which `-race` would flag. Queries the emulator does not support get no reply (Kitty keyboard query, XTVERSION), which the command treats as a terminal without the feature. Nothing goes to the outer terminal: the viewer never writes the command's bytes to its own output.

Risk handled: if the command never reads its input, a full pty input buffer could block the pump; the pump write is the only blocked party (the event loop's pipe write unblocks when the pump reads), and Ctrl+\ still quits because Bubble Tea reads keys on its own goroutine. Implement stage confirms with a test that floods replies.

### D6. Key encoding through the emulator

`tea.KeyPressMsg` converts to `uv.KeyPressEvent` by field copy (`tea.KeyMod` is `uv.KeyMod`; key codes are the `uv` constants), with `IsRepeat`, `BaseCode` and `ShiftedCode` cleared and `Text` cleared when Ctrl is held, because vt matches whole structs (vt `key.go:25-306`). Keys whose code is a special key vt does not encode (its default branch would write `string(code)`, a private-use rune, `key.go:293-303`) are dropped before `SendKey`. `tea.PasteMsg` goes to `Emulator.Paste` (bracketed when the command enabled it). Ctrl+\ is still intercepted first. `forwardKeyToPty` is removed.

### D7. History and the completed view

History is the emulator's scrollback, size 10,000 lines (vt `DefaultScrollbackSize`, set explicitly). On `doneMsg`, if the emulator is still on the alternate screen (a command that crashed in a full-screen program), the model writes `ESC [ ? 1049 l` to return to the main screen. The completed view builds a viewport whose content is: every scrollback line rendered with its styles (`uv.Line.Render`), then the main screen's rows without trailing blank rows, then the banner (`appendCompletionMessage`, now writing to that content). The existing completed-state keys and the scroll percentage keep working on that viewport. Memory: scrollback lines are trimmed of trailing blank cells by vt (`scrollback.go:31-56`).

### D8. Completion ordering

On Unix the adapter closes its own copy of the slave right after `Start` succeeds, so the master read returns EIO once every process of the command has closed the slave and the buffered output has been read. The session sequences the two goroutines: the reader sends its `outputMsg`s and then signals its end; the waiter, after `wait` returns, waits for that end or for a bounded grace period (about 250 ms, for processes the command left running that keep the slave open, and for ConPTY, whose output pipe does not end with the process), then sends `doneMsg`. Program messages from one goroutine are delivered in order, and the reader's last `Send` happens before its end signal, so all output read before the end precedes `doneMsg`. After `doneMsg` the model ignores `outputMsg` (the reader keeps draining so a left-running process does not block). `UnixPty.Close` closes the slave again; the adapter ignores `os.ErrClosed` from that second close so the deferred close does not turn a successful run into an error.

## Risks / Trade-offs

- [History is capped at 10,000 lines, the old viewer kept everything] -> documented in README and website; the cap matches common terminal defaults; raising it is one constant.
- [Grapheme clusters split across two pty reads can render wrong (upstream #1039)] -> cosmetic, limited to multi-codepoint clusters at a 4096-byte boundary; revisit when upstream merges #1044.
- [Signal behaviour changes (Ctrl+C now interrupts, SIGHUP on early exit)] -> intended and specified; listed in the release notes.
- [`Ctty: 0` depends on stdin being the slave] -> D1 sets stdio explicitly and refuses a prepared command that set it.
- [Grace period could cut output of a slow left-running process] -> such output was never ordered before the banner; the spec only promises output written before the command ended.
- [Windows not exercised by tmux tests] -> the Unix hook is a no-op there; Windows unit tests for the model (emulator, keys, ordering) run on every OS; a manual ConPTY check is a maintainer ask.
- [vt is v0.x] -> pinned exactly; the API used is small (NewEmulator, Write, Read, Resize, Render, SendKey, Paste, Scrollback, IsAltScreen, CursorPosition).

## Migration Plan

No configuration or schema change. Rollback is reverting the two commits; `go.mod` loses the two modules.

## Maintainer decisions

- 2026-11-25 (brief, group E): two fixes in two commits; evaluate `charmbracelet/x/vt` first and pin it; forward every Ctrl+letter; fix the banner race; S3 stays out of scope.

## Open questions

- Whether the history cap should become configurable (no requirement asks for it now).
