# interactive-mode-terminal Specification

## Purpose
Defines the terminal that interactive mode (`invowk cmd … -i`) gives a command: which terminal the command owns, how signals reach it, how its output is drawn and kept, how its terminal queries and the user's keys are handled, and when the completion banner appears.

## Requirements

### Requirement: The viewer's terminal is the command's controlling terminal
On Unix, a command run with `-i` SHALL start in a new session whose controlling terminal is the viewer's pseudo-terminal, which is also its stdin, stdout and stderr. A process of the command that opens `/dev/tty` SHALL get that pseudo-terminal, never the terminal invowk itself runs in, and SHALL read no key from the outer terminal.

#### Scenario: Terminal identity inside the viewer
- **WHEN** a native command run with `-i` prints `tty` and `ps -o tty= -p $$`
- **THEN** both SHALL name the same pseudo-terminal device

#### Scenario: Components without a TUI server answer inside the viewer
- **GIVEN** a native command whose script unsets `INVOWK_TUI_ADDR` and `INVOWK_TUI_TOKEN`, then runs `c=$(invowk tui choose --title "Pick one" alpha beta gamma); echo "CHOSEN=[$c]"` and `n=$(invowk tui input --title "Your name"); echo "NAME=[$n]"`
- **WHEN** the user runs it with `-i` and presses Down, Enter, types `Ana` and presses Enter
- **THEN** the viewer SHALL show `CHOSEN=[beta]` and `NAME=[Ana]` on every run
- **AND** no key SHALL appear echoed as a control sequence such as `^[[B`

#### Scenario: Components with the TUI server are unchanged
- **WHEN** the same script runs with `-i` without unsetting the TUI server variables
- **THEN** the components SHALL render as overlays of the viewer, and the viewer SHALL show `CHOSEN=[beta]` and `NAME=[Ana]`

### Requirement: Ctrl+C in the viewer interrupts the command
On Unix, while a command runs, Ctrl+C typed in the viewer SHALL reach the command's terminal as the interrupt character, so the terminal's foreground job receives SIGINT, or reads the character when it has turned signal generation off. The viewer SHALL stay open and show the command's completion. Ctrl+Z SHALL not suspend a native command: the command's session has no job control, so the stop signal the terminal generates is discarded, and a raw-mode program reads the key instead.

#### Scenario: Interrupt a sleeping command
- **WHEN** a native command `sleep 30` runs with `-i` and the user presses Ctrl+C
- **THEN** the command SHALL end well before 30 seconds
- **AND** the viewer SHALL show a failed completion with a non-zero exit status and wait for the user to exit

#### Scenario: A raw-mode program receives the key
- **WHEN** a command run with `-i` runs `stty raw -echo; od -An -c -N1` and the user presses Ctrl+C
- **THEN** it SHALL print the byte 003 and end normally

#### Scenario: Ctrl+Z does not suspend the command
- **WHEN** a command run with `-i` runs `sleep 2; echo AFTER` and the user presses Ctrl+Z
- **THEN** the command SHALL keep running and print `AFTER`

### Requirement: Size changes reach the command
The command's terminal size SHALL equal the viewer's command area (the viewer's width, and its height minus the header and footer rows) from the start of the run, and SHALL follow every resize of the outer terminal while the command runs. On Unix the resize SHALL deliver SIGWINCH to the terminal's foreground job.

#### Scenario: Size at start
- **WHEN** a command run with `-i` in a 100x30 terminal prints `stty size` as its first action
- **THEN** it SHALL print the command area's rows and columns, `27 100`

#### Scenario: Resize while running
- **WHEN** the outer terminal is resized while a command run with `-i` waits in `trap 'stty size' WINCH; while :; do sleep 1; done`
- **THEN** the command SHALL print the new command-area size

### Requirement: Leaving the viewer early hangs up the command
On Unix, when the viewer ends before the command does, by Ctrl+\ or by invowk being cancelled, the command's terminal SHALL be hung up, so its session receives SIGHUP, and the viewer SHALL return to the outer terminal without waiting for the command. The hang-up SHALL happen when the viewer ends, also when the command never read the input the viewer sent it, and not only when invowk exits. For a container command the session is the engine CLI's: the hang-up SHALL reach the engine CLI, and what then happens to the processes inside the container is the engine's behavior.

#### Scenario: Force quit
- **WHEN** a native command `sleep 3011 & echo SLEEP_PID=$!; wait` runs with `-i` and the user presses Ctrl+\
- **THEN** invowk SHALL exit, and the process with the printed `SLEEP_PID` SHALL no longer be running shortly after

#### Scenario: Force quit of a command that never reads its input
- **GIVEN** an interactive session whose command runs `stty raw -echo; trap 'echo HUP > hup.txt; exit 1' HUP; while :; do sleep 1; done`, which never reads its input
- **WHEN** enough keys and terminal replies are sent to fill the command's input, the viewer is ended by Ctrl+\, and the process that hosted the viewer keeps running
- **THEN** the session SHALL return, `hup.txt` SHALL appear and the command SHALL exit shortly after, while that process is still running

### Requirement: Background jobs end with the command
On Unix, when the command's main process exits, the command's terminal SHALL send SIGHUP to its foreground job, so a job the command started in the background without detaching it (for example with `&` in a shell script) SHALL end with the command unless it ignores SIGHUP or left the session (`nohup`, `setsid`), as when a terminal window closes. A run without `-i` is not affected.

#### Scenario: A background job is hung up
- **WHEN** a native command `sleep 3017 & echo BG_PID=$!` runs with `-i` and ends
- **THEN** the process with the printed `BG_PID` SHALL no longer be running shortly after the completion banner appears

#### Scenario: A detached job survives
- **WHEN** a native command `nohup sleep 3019 >/dev/null 2>&1 & echo BG_PID=$!` runs with `-i` and ends
- **THEN** the process with the printed `BG_PID` SHALL still be running after the completion banner appears

### Requirement: Windows keeps its console behavior
On Windows, the command SHALL keep running on the pseudo-console invowk creates for it, as before this change; no session or controlling-terminal setup applies there.

#### Scenario: Windows interactive run
- **WHEN** a command runs with `-i` on Windows
- **THEN** it SHALL start on its pseudo-console as before, and the screen, query, key and completion requirements of this capability SHALL apply to its output

### Requirement: Output is drawn as a terminal screen
The viewer SHALL apply the command's output to a terminal screen the size of the command area, honoring cursor movement, erasing, insertion and deletion, scrolling regions, line wrap, tabs and character attributes and colors, and SHALL show that screen while the command runs. Overwritten text SHALL not remain visible.

#### Scenario: A repainting prompt shows one frame
- **WHEN** the no-server `choose` script of the controlling-terminal requirement runs with `-i` and the user presses Down
- **THEN** the viewer SHALL show exactly one `Pick one` frame, with `beta` marked as the current choice in place
- **AND** the component's help line `↑/k up • ↓/j down • enter submit • esc cancel` SHALL appear exactly once
- **AND** the viewer's header and footer SHALL stay intact

#### Scenario: A typed answer shows on the prompt row
- **WHEN** the no-server `input` script of the controlling-terminal requirement runs with `-i` and the user types `Ana`
- **THEN** the prompt row SHALL show `> Ana`
- **AND** the component's help line SHALL read exactly `enter submit • esc cancel`, and no other help line SHALL be shown
- **AND** the viewer's header and footer SHALL stay intact

#### Scenario: Carriage-return progress
- **WHEN** a command run with `-i` prints `printf 'step 1\rstep 2\rdone  \n'`
- **THEN** the screen SHALL show `done` on that row and neither `step 1` nor `step 2`

### Requirement: Full-screen programs render in the viewer
When the command switches to the alternate screen, the viewer SHALL show that screen; when the command switches back, the viewer SHALL show the main screen as it was before the switch. Alternate-screen content SHALL not be added to the history.

#### Scenario: less
- **WHEN** a command run with `-i` runs `seq 1 200 | less` and the user presses `q`
- **THEN** while `less` runs, the command area SHALL show its first page and status line
- **AND** after `q`, the main screen SHALL be shown again and the history SHALL hold no `less` page

### Requirement: Output history is kept and browsable after completion
Lines that scroll off the top of the main screen SHALL be kept as history, up to the most recent 10,000 lines. After the command ends, the viewer SHALL show the history followed by the final main screen, without trailing blank rows, and then the completion banner, browsable with the existing completed-state keys.

#### Scenario: Long output scrolls
- **WHEN** a command run with `-i` prints `seq 1 500` and ends
- **THEN** the viewer SHALL show the completion banner, and pressing Home (or `g`) SHALL show the line `1` at the top
- **AND** pressing End (or `G`) SHALL show the last lines `500` and the banner

#### Scenario: History limit
- **WHEN** a command run with `-i` prints `seq 1 20000` and ends
- **THEN** at most 10,000 history lines SHALL be browsable above the final screen, line `1` SHALL no longer be browsable, and line `20000` SHALL be shown above the banner

### Requirement: Terminal queries are answered inside the viewer
The viewer SHALL answer the command's terminal queries that its terminal supports (at least primary device attributes, cursor position report and mode reports) by writing the replies to the command's input, and SHALL drop the others. Only the viewer's rendering of the emulated screen (its text, character attributes and hyperlinks) SHALL reach the outer terminal; no query, mode change or other control sequence the command writes SHALL be passed to it.

#### Scenario: Device attributes
- **WHEN** a command run with `-i` puts its terminal in raw mode, writes `ESC [ c` and reads its input for one second
- **THEN** it SHALL read a reply that starts with `ESC [ ?` and ends with `c`

#### Scenario: Cursor position
- **WHEN** a command run with `-i` writes `ESC [ 6 n` from row 1, column 1 of a cleared screen and reads its input in raw mode
- **THEN** it SHALL read `ESC [ 1 ; 1 R`

### Requirement: The viewer stays responsive when the command does not read its input
Answering queries and forwarding keys SHALL never block the viewer. When the command does not read its input, the viewer SHALL keep drawing output and SHALL keep handling Ctrl+\; replies and keys that cannot be delivered because the command's input is full MAY be dropped.

#### Scenario: Query flood without reads
- **WHEN** a command run with `-i` runs `stty raw -echo; while :; do printf '\033[c'; done`, which never reads its input, and the user presses Ctrl+\ after a few seconds
- **THEN** the viewer SHALL force quit within a few seconds

### Requirement: Keys are encoded for the command's terminal modes
While the command runs, the viewer SHALL forward every key except Ctrl+\, encoded as a terminal in the command's current modes would send it: every Ctrl+letter (with or without Shift) as its control character, Alt as an `ESC` prefix (Alt+Shift+letter as `ESC` and the uppercase letter), cursor keys in application form after the command enables application cursor keys, keypad keys in application form after the command enables application keypad keys. Pasted text SHALL be forwarded, bracketed when the command enabled bracketed paste, and SHALL be delivered or dropped whole, never in part. Under a server component overlay, pasted text SHALL go to the component.

#### Scenario: Ctrl+letter
- **WHEN** a command run with `-i` runs `stty raw -echo; od -An -c -N1` and the user presses Ctrl+A
- **THEN** it SHALL print the byte 001

#### Scenario: Application cursor keys
- **WHEN** a command run with `-i` enables application cursor keys (`printf '\033[?1h'`), reads three bytes in raw mode and the user presses Up
- **THEN** it SHALL read `ESC O A`

#### Scenario: Normal cursor keys
- **WHEN** a command run with `-i` reads three bytes in raw mode without enabling application cursor keys and the user presses Up
- **THEN** it SHALL read `ESC [ A`

#### Scenario: Ctrl+\ stays with the viewer
- **WHEN** the user presses Ctrl+\ while a command runs with `-i`
- **THEN** the viewer SHALL force quit and the key SHALL not be sent to the command

### Requirement: Lock and unencodable modifiers do not lose keys
Lock modifiers (Caps Lock, Num Lock, Scroll Lock) SHALL not change the bytes a key sends. A special key held with a modifier that the legacy encoding has no form for (such as Ctrl+Left or Shift+Up) SHALL be sent as the plain key, never dropped.

#### Scenario: Lock modifiers do not change keys
- **WHEN** the outer terminal reports Caps Lock or Num Lock held with a key, and the user presses Ctrl+A, Enter or Up while a command reads raw input with `-i`
- **THEN** the command SHALL read `001`, `\r` or `ESC [ A`, the same bytes as without the lock

#### Scenario: Modified special keys
- **WHEN** the user presses Ctrl+Left or Shift+Up while a command reads raw input with `-i` in normal cursor mode
- **THEN** the command SHALL read `ESC [ D` or `ESC [ A`

### Requirement: The completion banner follows all of the command's output
The completion banner SHALL be shown only after the viewer has drawn the output it read before the command ended, and it SHALL stay the last thing in the completed view: output that reaches the viewer after the banner appeared (from processes the command left running, or flushed late by the Windows pseudo-console) SHALL be drawn above the banner, never below it, and SHALL not be dropped.

#### Scenario: Trailing output stays above the banner
- **WHEN** a command run with `-i` prints `seq 1 50; echo LAST-LINE` and ends at once
- **THEN** on every run `LAST-LINE` SHALL appear above the completion banner, and nothing SHALL appear below the banner except its own lines

#### Scenario: Late output is drawn above the banner
- **GIVEN** a command whose terminal delivers its last output only after the command ended and the banner is shown
- **WHEN** that output arrives
- **THEN** the completed view SHALL show it above the banner, and the banner SHALL remain the last lines of the view

### Requirement: Incomplete escape and UTF-8 sequences are buffered across output chunks
When command output ends mid-sequence—a lone ESC, an unterminated CSI, OSC, DCS, SOS, PM, or APC sequence, an SS2 or SS3 shift awaiting its following character, an intermediate ESC sequence, or an incomplete UTF-8 rune—the viewer SHALL hold that trailing fragment until completed by subsequent output, a bounded idle timeout, or stream end. Complete preceding text and sequences SHALL flush immediately.

#### Scenario: Split CSI or OSC sequence across chunks
- **WHEN** command output delivers `\x1b[3` in one read and `1mRED\x1b[0m` in the next, or splits an OSC sequence across reads
- **THEN** the preceding complete text SHALL appear immediately, and the viewer SHALL apply the completed sequence once the remaining bytes arrive without drawing raw escape fragments

#### Scenario: Split multi-byte UTF-8 rune across chunks
- **WHEN** command output splits a multi-byte UTF-8 character across two reads
- **THEN** the viewer SHALL hold the leading bytes until the continuation bytes arrive and render the single decoded character intact

#### Scenario: Legitimate lone ESC flushes within bounded time
- **WHEN** a command writes a standalone `\x1b` (or an incomplete sequence tail) followed by no further output
- **THEN** preceding complete text SHALL appear at once, and the trailing fragment SHALL flush within the bounded idle period or at stream end without stalling the viewer

### Requirement: Delegated styled text writes at a clean ground state with isolated attributes
When a child process delivers styled text through the TUI server while the command runs, the viewer SHALL write that styled output to the command screen only at a clean parser ground state, without disturbing any incomplete command sequence held at a chunk boundary, and SHALL isolate SGR attributes around the styled text so attributes do not bleed across streams.

#### Scenario: Styled text arrives during a split command escape sequence
- **WHEN** command output ends with an incomplete `\x1b[3` (or lone `\x1b`, unclosed OSC/DCS/SOS/PM/APC, `ESC N`/`ESC O`, or partial UTF-8 rune) and a TUI-server styled write arrives before the remainder `1mOK`
- **THEN** the styled text SHALL render intact on the command screen, and once `1mOK` arrives the command's `\x1b[31mOK` SHALL apply cleanly with neither literal fragment text nor swallowed characters

#### Scenario: SGR attributes do not bleed between command output and styled writes
- **WHEN** the command enables a foreground or background color before a TUI-server styled write, and continues writing plain text after the styled write finishes
- **THEN** the styled write SHALL render with its own configured style rather than inheriting the command's color, and subsequent command text SHALL not inherit the styled write's attributes

#### Scenario: Styled text or alt-screen exit after a forced idle or EOF flush
- **WHEN** a prior truncated sequence from the command flushed on idle timeout or stream end, leaving the emulator outside ground state, and a styled write or alt-screen exit is applied next
- **THEN** the viewer SHALL restore parser ground state before applying the styled text or alt-screen exit so those sequences are never swallowed
