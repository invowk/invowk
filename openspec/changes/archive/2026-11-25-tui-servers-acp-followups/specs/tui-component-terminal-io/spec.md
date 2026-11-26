# Spec Delta

## ADDED Requirements

### Requirement: A drawn table shows its data rows
When `invowk tui table` draws on the terminal, with or without `--selectable`, it SHALL show the column titles, the header rule and the data rows that fit its height, each row on its own line with its cells under their titles. A selectable table SHALL mark the selected row. The rows SHALL be visible before any key is pressed.

#### Scenario: Display table on a terminal
- **WHEN** a user in a 120x30 terminal runs `invowk tui table --file rows.csv`, and `rows.csv` holds `name,age,city`, `Alice,30,NYC` and `Bob,25,LA`
- **THEN** the terminal SHALL show `Alice`, `30` and `NYC` on one line and `Bob`, `25` and `LA` on the next, below the titles, before any key is pressed

#### Scenario: Selectable table on a terminal
- **WHEN** a user in a 120x30 terminal runs `row=$(invowk tui table --file rows.csv --selectable)` with the same file and presses Down and then Enter
- **THEN** both data rows SHALL be visible before the first key press, and `$row` SHALL be `Bob,25,LA`

#### Scenario: Table wider than the terminal
- **WHEN** the table's columns are wider in total than the terminal
- **THEN** every visible row SHALL still be drawn on one terminal line each, cut at the terminal's width

## MODIFIED Requirements

### Requirement: Plain-text table format
The plain-text `table` output SHALL start with a line of column titles (the header row, or `--columns`), followed by one line per data row in input order. Each cell SHALL be left-aligned and padded with spaces to the widest cell of its column, measured in terminal cells, with two spaces between columns, trailing spaces trimmed, and `\n` line endings. Each control character in a cell (a newline, a tab, an escape), each format character (Unicode category Cf, such as a bidirectional override or a zero-width character) and each line or paragraph separator (U+2028, U+2029) SHALL be written as a space, so every row stays on one line and its text reads in the order it is stored. Each byte sequence in a cell that is not valid UTF-8 SHALL be written as U+FFFD. It SHALL hold no control sequences, borders or header rule. Every row SHALL be printed, whatever `--height` and `--widths` say.

#### Scenario: Aligned rows
- **WHEN** `invowk tui table --separator '|'` runs with stdout not a terminal and stdin `name|age|city`, `Alice|30|NYC`, `Bob|25|LA`
- **THEN** stdout SHALL be exactly `name   age  city`, `Alice  30   NYC`, `Bob    25   LA`, one per line

#### Scenario: Short rows and overridden titles
- **WHEN** `invowk tui table --columns a,b,c` runs with stdout not a terminal and stdin `x,y,z` and `1`
- **THEN** stdout SHALL be exactly `a  b  c`, `x  y  z`, `1`, one per line

#### Scenario: Control characters in a cell
- **WHEN** `invowk tui table --file rows.csv` runs with stdout not a terminal and `rows.csv` holds `name,notes` and `Alice,"line1` / `line2"` (a quoted field with a newline)
- **THEN** stdout SHALL be exactly `name   notes`, `Alice  line1 line2`, one per line

#### Scenario: Format and separator characters in a cell
- **WHEN** `invowk tui table --file rows.csv` runs with stdout not a terminal and a cell of `rows.csv` holds `a`, U+202E (right-to-left override), `b`, U+200B (zero-width space), `c`, U+2028 (line separator) and `d`
- **THEN** that cell SHALL be written as `a b c d`, and no U+202E, U+200B or U+2028 SHALL appear in stdout

#### Scenario: Invalid UTF-8 in a cell
- **WHEN** `invowk tui table` runs with stdout not a terminal and stdin `name` and a line holding `a`, the byte `0xff` and `b`
- **THEN** that cell SHALL be written as `a`, U+FFFD, `b`

#### Scenario: Height does not cut rows
- **WHEN** `invowk tui table --height 2 --file rows.csv` runs with stdout not a terminal and `rows.csv` holds 30 data rows
- **THEN** stdout SHALL hold the title line and all 30 rows
