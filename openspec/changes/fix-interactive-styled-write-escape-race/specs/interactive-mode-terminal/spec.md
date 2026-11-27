# Spec Delta

## ADDED Requirements

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
