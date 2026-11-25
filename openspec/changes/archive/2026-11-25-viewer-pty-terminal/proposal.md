## Why

The `-i` viewer starts the command on a pty but never makes that pty the command's controlling terminal, so every `invowk tui` component a script runs without the TUI server (they open `/dev/tty` since #175) reads keys from the real terminal and races the viewer for them: in a measured run 0 of 5 key scripts produced both answers. The viewer also appends the command's bytes to a text log and ignores cursor movement, so any program that repaints (an inline Bubble Tea prompt, a progress bar, `less`) is drawn as a pile of stale frames. Diagnosis: `program-notes/viewer-diagnosis.md` (tracker N17; the keyboard-protocol lead was wrong).

## What Changes

- On Unix, the command starts in a new session whose controlling terminal is the viewer's pty. `/dev/tty`, `tty` and `ps -o tty=` all name the viewer's pty, so prompts that open the terminal (`invowk tui *` without a server, `sudo`, `ssh`) draw and read inside the viewer.
- Consequences, now part of the contract: Ctrl+C typed in the viewer interrupts the command's foreground job (SIGINT through the pty); resizing reaches the command as a window-size change (SIGWINCH); leaving the viewer early (Ctrl+\) or invowk exiting hangs up the command's session (SIGHUP); when the command's main process exits, background jobs it started without detaching (`cmd &` in a script) are hung up too, as when a terminal window closes (`nohup`/`setsid` keep them); Ctrl+Z does not suspend the command (no job control). For container commands the hang-up reaches the engine CLI. Windows ConPTY is unchanged.
- The viewer interprets the command's output as a terminal screen: cursor movement, erase, scrolling regions, colors and the alternate screen are applied, so a repainting program shows one current frame and a full-screen program renders.
- Lines that scroll off the top are kept as history, up to 10,000 lines (the current viewer keeps all output); after completion the history plus the final screen is browsable with the existing keys. Full-screen (alternate-screen) content is not added to history.
- Terminal queries from the command (device attributes, cursor position, colors, mode reports) are answered by the viewer's terminal; no query or other control sequence from the command reaches the outer terminal, only the viewer's rendering of the emulated screen. Answering never blocks the viewer: when the command does not read its input, undeliverable replies and keys are dropped and Ctrl+\ still quits.
- Keys are encoded for the command's current terminal modes (application cursor keys, Alt prefix); every Ctrl+letter is forwarded (today only Ctrl+C, D and Z are), lock modifiers (Caps Lock, Num Lock) do not change a key, modified special keys the legacy encoding cannot express are sent as the plain key (as today), and pasted text is forwarded, bracketed when the command asked for it. Ctrl+\ stays the viewer's force quit.
- The completion banner always comes after the command's last output (fixes the observed race where trailing output landed below it); output that arrives after the banner (a late Windows pseudo-console flush, a process the command left running) is drawn above it.
- New Go dependency `github.com/charmbracelet/x/vt` v0.1.0 (MIT), pinned exactly.

## Capabilities

### New Capabilities

- `interactive-mode-terminal`: the terminal that `-i` gives a command: its controlling-terminal identity, signal delivery, screen emulation, history, query answering, key encoding and completion ordering.

### Modified Capabilities

(none: `tui-component-terminal-io` already says a directly rendered component uses the controlling terminal; this change makes the viewer's pty that terminal, and its "Server-delegated rendering is unchanged" requirement keeps holding.)

## Impact

- Code: `internal/app/commandadapters/interactive_session.go` (plus a Unix/Windows split for the child's session setup), `internal/tui/interactive.go`, `interactive_model.go`, `interactive_session.go`, `interactive_helpers.go` and their tests; a new tmux test file under `tests/cli/`.
- Dependencies: `go.mod`/`go.sum` gain `github.com/charmbracelet/x/vt` and its one new transitive module `github.com/charmbracelet/x/exp/ordered`; no existing module changes version (`github.com/charmbracelet/ultraviolet` would move from the indirect to the direct block at the same version only if the code imported it directly, which the design avoids).
- User-visible: interactive output history is capped at 10,000 lines; the mouse wheel no longer scrolls the history while the command runs (history is browsable after completion, as before); Ctrl+C, resize and early exit now reach the command as signals; background jobs end with the command's main process; Ctrl+Z does not suspend the command. Docs: README "Interactive Mode", `website/docs/advanced/interactive-mode.mdx` and its pt-BR copy, and the key-binding snippets.
- Not affected: non-interactive runs, the TUI server protocol and overlays, `invowk tui` components, the sshserver pty (it already uses `creack/pty.Start`, which sets a new session and controlling terminal).
