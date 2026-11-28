# Spec Delta

## Purpose

Defines OSC 7501 Program Status Protocol reporting across Invowk's interactive viewer (`-i`), standalone `invowk tui` components, and watch mode (`--ivk-watch`), including state mapping, lifecycle clearing, payload sanitization, and non-interactive pass-through silence.

## ADDED Requirements

### Requirement: Interactive mode reports execution and completion status via OSC 7501
When running `invowk cmd -i`, the viewer SHALL report OSC 7501 status with `app="invowk"` for `stateExecuting` (`working`), `stateTUI` (`blocked`, `idle`, or `working`), and `stateCompleted` (`done` on success, `error` on failure), and SHALL emit `clear` on dismiss, `Ctrl+\` force-quit, or context cancellation.

#### Scenario: Interactive command reports working, done, and clear
- **WHEN** a command runs with `-i`, exits with code 0 to enter `stateCompleted`, and the user then presses Enter or `q` to quit
- **THEN** the viewer SHALL report `state=working` while running, `state=done` in `stateCompleted`, and `state=clear` when dismissed

#### Scenario: Interactive command failure or cancellation clears cleanly
- **WHEN** a command run with `-i` exits with a non-zero exit code (or fails) and is dismissed, or `RunInteractiveSession` ends via `tea.ErrProgramKilled`
- **THEN** the viewer SHALL report `state=error` while showing the completion banner, and SHALL emit `state=clear` on dismiss, `Ctrl+\`, or cancellation

### Requirement: Interactive TUI overlays and standalone components report prompt or working status
While active, `invowk cmd -i` overlays and standalone `invowk tui` components SHALL map `confirm` to `blocked` (`permission`), password `input` to `blocked` (`auth`), other interactive inputs/lists/selectable `table` to `blocked` (`question`), `pager` and non-selectable `table` to `idle`, `spin` to `working`, and emit `clear` on exit.

#### Scenario: Password input reports blocked auth and clears on submit or signal
- **WHEN** standalone `invowk tui input --password` runs on a terminal and finishes, is cancelled, or is terminated by `SIGTERM`
- **THEN** it SHALL report `state=blocked:kind=auth` on the terminal while waiting, emit `state=clear` on exit, and write no OSC 7501 bytes to captured stdout

#### Scenario: Confirm, selectable table, and pager overlays map accurately
- **WHEN** `confirm`, selectable `table`, non-selectable `table`, or `pager` runs standalone or inside `-i` (`stateTUI`)
- **THEN** `confirm` SHALL report `blocked:kind=permission`, selectable `table` SHALL report `blocked:kind=question`, and `pager`/non-selectable `table` SHALL report `idle`, reverting to `working` on return to `stateExecuting`

#### Scenario: Delegated modal components do not overwrite viewer status independently
- **WHEN** any embeddable TUI component (`confirm`, `input`, `write`, `choose`, `filter`, `file`, `table`, `pager`, `spin`) renders as a modal overlay (`forModal: true`) inside `-i`
- **THEN** the embedded component model SHALL omit `ProgramStatus` so `interactiveModel` remains the sole OSC 7501 owner

#### Scenario: Spin and non-terminal fallbacks preserve clean stdout
- **WHEN** `invowk tui spin` runs on a terminal (until completion or `Ctrl+C`) or `invowk tui pager` / non-selectable `table` passes through on a non-terminal stdout
- **THEN** `spin` SHALL report `working` on the terminal and `state=clear` on finish or `Ctrl+C`, while non-terminal pass-through SHALL emit zero OSC 7501 bytes

### Requirement: Watch mode reports continuous status on supporting TTYs
When `invowk cmd --ivk-watch` runs with a terminal attached, Invowk SHALL emit OSC 7501 `working` during command execution, `idle` while awaiting changes after a passing run, `error` while awaiting changes after a failing run, and `clear` on exit, without writing OSC 7501 to redirected non-TTY stdout.

#### Scenario: Watch mode transitions working, error, working, idle, and clear
- **WHEN** `invowk cmd --ivk-watch` runs on a terminal, fails its initial run, succeeds on a file-change re-run, and then exits
- **THEN** Invowk SHALL emit `working` during each run, `error` after the failed run, `idle` after the passing run, and `clear` on exit

#### Scenario: Watch mode with no attached terminal or redirected stdout
- **WHEN** `invowk cmd --ivk-watch` runs with no terminal attached (such as a CI pipe) or with stdout redirected to a file while `/dev/tty` is open
- **THEN** redirected stdout SHALL receive zero OSC 7501 bytes, and no OSC 7501 bytes SHALL be emitted anywhere when no terminal is attached

### Requirement: Status payloads are sanitized, bounded, and properly attributed
All non-clear OSC 7501 reports emitted by Invowk SHALL carry `app="invowk"` (while `state=clear` omits `app` per OSC 7501) and sanitize `Title` and `Message` by replacing invalid UTF-8 and control characters (`U+0000`–`U+001F`, `U+007F`, `U+0080`–`U+009F`) with spaces and truncating to `title <= 192` and `msg <= 2048` bytes.

#### Scenario: Control characters and long UTF-8 in prompt title stay valid
- **WHEN** a command name or prompt title contains newlines, tabs, C1 control bytes, or exceeds byte limits
- **THEN** Invowk SHALL sanitize, trim, and truncate on valid UTF-8 boundaries so the emitted OSC 7501 sequence stays valid

### Requirement: Non-interactive command execution emits no OSC 7501
When `invowk cmd` runs without `-i` and without `--ivk-watch`, Invowk SHALL NOT emit OSC 7501 sequences of its own, leaving child tool reports and piped output undisturbed.

#### Scenario: Non-interactive native command passes through cleanly
- **WHEN** `invowk cmd build` runs without `-i` or `--ivk-watch`
- **THEN** Invowk SHALL NOT emit any OSC 7501 sequence before, during, or after the child process
