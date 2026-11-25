# Spec Delta

## Purpose

Defines the terminal that interactive mode (`invowk cmd … -i`) gives a command: which terminal the command owns, how signals reach it, how its output is drawn and kept, how its terminal queries and the user's keys are handled, and when the completion banner appears.

## ADDED Requirements

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
On Unix, while a command runs, Ctrl+C typed in the viewer SHALL reach the command's terminal as the interrupt character, so the terminal's foreground job receives SIGINT, or reads the character when it has turned signal generation off. The viewer SHALL stay open and show the command's completion.

#### Scenario: Interrupt a sleeping command
- **WHEN** a native command `sleep 30` runs with `-i` and the user presses Ctrl+C
- **THEN** the command SHALL end well before 30 seconds
- **AND** the viewer SHALL show a failed completion with a non-zero exit status and wait for the user to exit

#### Scenario: A raw-mode program receives the key
- **WHEN** a command that put its terminal in raw mode, such as a running `invowk tui choose` without a TUI server, runs with `-i` and the user presses Ctrl+C
- **THEN** that program SHALL receive Ctrl+C as a key

### Requirement: Size changes reach the command
The command's terminal size SHALL equal the viewer's command area (the viewer's width, and its height minus the header and footer rows) from the start of the run, and SHALL follow every resize of the outer terminal while the command runs. On Unix the resize SHALL deliver SIGWINCH to the terminal's foreground job.

#### Scenario: Size at start
- **WHEN** a command run with `-i` in a 100x30 terminal prints `stty size` as its first action
- **THEN** it SHALL print the command area's rows and columns, `27 100`

#### Scenario: Resize while running
- **WHEN** the outer terminal is resized while a command run with `-i` waits in `trap 'stty size' WINCH; while :; do sleep 1; done`
- **THEN** the command SHALL print the new command-area size

### Requirement: Leaving the viewer early hangs up the command
On Unix, when the viewer ends before the command does, by Ctrl+\ or by invowk being cancelled, the command's terminal SHALL be hung up, so its session receives SIGHUP, and the viewer SHALL return to the outer terminal without waiting for the command.

#### Scenario: Force quit
- **WHEN** a native command `sleep 300` runs with `-i` and the user presses Ctrl+\
- **THEN** invowk SHALL exit, and the `sleep` process SHALL no longer be running shortly after

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
- **AND** the component's help line SHALL read exactly `enter submit • esc cancel`
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
The viewer SHALL answer the command's terminal queries that its terminal supports (at least primary device attributes, cursor position report and mode reports) by writing the replies to the command's input, and SHALL drop the others. No byte the command writes SHALL be passed to the outer terminal.

#### Scenario: Device attributes
- **WHEN** a command run with `-i` puts its terminal in raw mode, writes `ESC [ c` and reads its input for one second
- **THEN** it SHALL read a reply that starts with `ESC [ ?` and ends with `c`

#### Scenario: Cursor position
- **WHEN** a command run with `-i` writes `ESC [ 6 n` from row 1, column 1 of a cleared screen and reads its input in raw mode
- **THEN** it SHALL read `ESC [ 1 ; 1 R`

### Requirement: Keys are encoded for the command's terminal modes
While the command runs, the viewer SHALL forward every key except Ctrl+\, encoded as a terminal in the command's current modes would send it: every Ctrl+letter as its control character, Alt as an `ESC` prefix, cursor keys in application form after the command enables application cursor keys. Pasted text SHALL be forwarded, bracketed when the command enabled bracketed paste.

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

### Requirement: The completion banner follows all of the command's output
The completion banner SHALL be shown only after the viewer has drawn the output the command wrote before it ended, and no such output SHALL appear below the banner. Output that processes left running by the command write after the banner SHALL not be drawn.

#### Scenario: Trailing output stays above the banner
- **WHEN** a command run with `-i` prints `seq 1 50; echo LAST-LINE` and ends at once
- **THEN** on every run `LAST-LINE` SHALL appear above the completion banner, and nothing SHALL appear below the banner except its own lines
