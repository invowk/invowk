# Spec Delta

## Purpose

Updates `cli-exit-codes` so current-directory parse, schema, or `invowk_version` failures fail closed immediately with status `1` and their underlying error, while non-cwd skipped sources retain `SkippedSourcesError` and scoped-miss exit status `2`.

## ADDED Requirements

### Requirement: Broken current-directory sources fail closed immediately with status 1
When `--ivk-no-cwd` is off and a current-directory source (`./invowkfile.cue` or `./*.invowkmod`) fails parse, schema, or `invowk_version` validation, `invowk cmd` SHALL abort immediately with exit status `1` and the source's load error, without falling back to global/included sources or emitting `command not found`.

#### Scenario: Broken cwd invowkfile blocks global fallback with status 1
- **WHEN** cwd `invowkfile.cue` has a schema error and `~/.invowk/cmds` defines `build`, and `invowk cmd build` runs without `--ivk-no-cwd`
- **THEN** stderr SHALL report the `invowkfile.cue` error directly, the global command SHALL NOT run, and `invowk` SHALL exit with status `1`

## MODIFIED Requirements

### Requirement: Command groups reject unmatched tokens
A command group is a command node that exists only to hold subcommands, such as the synthesized parent `test` of `test unit` or a built-in parent such as `config` or `module`. When a group gets one or more positional tokens that do not resolve to a subcommand, invowk SHALL exit with status 2 and SHALL write nothing to stdout. Exception: see "A command missing because a source failed to load exits with status 1".

#### Scenario: Typo under a discovered group
- **WHEN** commands `test unit` and `test other` exist and the user runs `invowk cmd test nonexistent`
- **THEN** invowk SHALL exit with status 2
- **AND** stdout SHALL be empty

#### Scenario: Typo under a discovered group while a module failed to load
- **WHEN** commands `test unit` and `test other` exist in the invowkfile, non-cwd module directory `broken.invowkmod` (e.g., under `~/.invowk/cmds`) fails to load, and the user runs `invowk cmd test typo`
- **THEN** invowk SHALL exit with status 1
- **AND** stdout SHALL be empty

#### Scenario: Typo under a built-in group
- **WHEN** the user runs `invowk config bogus`
- **THEN** invowk SHALL exit with status 2
- **AND** stdout SHALL be empty

#### Scenario: Typo under a non-runnable built-in group
- **WHEN** the user runs `invowk module bogus`
- **THEN** invowk SHALL exit with status 2
- **AND** stdout SHALL be empty

### Requirement: A command missing because a source failed to load exits with status 1
When `--ivk-no-cwd` is off and a current-directory source fails to parse or load, invowk SHALL fail closed immediately with status `1` and report that error. When the command or source named on the command line is not found, and discovery reported in the same invocation a non-cwd invowkfile or module that it skipped because it failed to parse or load, invowk SHALL exit with status 1 instead of 2. Policy skips, such as reserved module names, do not count. Ambiguity, argument and flag errors keep status 2 (unless a broken cwd source already aborted discovery).

#### Scenario: Command defined in an invowkfile that does not parse
- **WHEN** the cwd invowkfile defines `build` but has a field the schema does not allow, and the user runs `invowk cmd build`
- **THEN** stderr SHALL report the invowkfile's schema/parse error
- **AND** invowk SHALL exit with status 1

#### Scenario: Source filter on a skipped invowkfile
- **WHEN** the cwd invowkfile does not parse and the user runs `invowk cmd @invowkfile build`
- **THEN** invowk SHALL exit with status 1

#### Scenario: Source filter on the skipped module
- **WHEN** non-cwd module directory `broken.invowkmod` (e.g., under `~/.invowk/cmds`) fails to load and the user runs `invowk cmd @broken x`
- **THEN** invowk SHALL exit with status 1

### Requirement: A scoped miss in a source that loaded exits with status 2
For a request that names a source (`@source` or `--ivk-from`) while all current-directory sources loaded cleanly, only a skipped non-cwd module that may be that source SHALL count for the status-1 exception: a scanned module directory counts for the source of its name (residuals: design.md, Decision 14). Included and vendored modules, an invowkfile inside a module, and a diagnostic without a path SHALL always count. Otherwise the miss is certain and SHALL exit with status 2.

#### Scenario: Scoped miss in a source that loaded
- **WHEN** the invowkfile is valid, non-cwd module directory `broken.invowkmod` fails to load, and the user runs `invowk cmd @invowkfile nope`
- **THEN** invowk SHALL exit with status 2

#### Scenario: Unknown source that cannot be the skipped one
- **WHEN** non-cwd module directory `broken.invowkmod` fails to load and the user runs `invowk cmd @bogus x`
- **THEN** invowk SHALL exit with status 2

### Requirement: A skipped-source not-found error names the skipped sources
When a not-found error exits with status 1 because a non-cwd source failed to load, its message SHALL name each skipped source that counted by a short label (its file or module directory name, the module directory plus the file for an invowkfile inside a module, or `an unnamed source` when the diagnostic has no path), with the discovery diagnostic code (for example `broken.invowkmod (module_load_skipped)`), so a wrapper can tell why the status is 1.

#### Scenario: The error names the skipped source
- **WHEN** the invowkfile is valid, non-cwd module directory `broken.invowkmod` fails to load, and the user runs `invowk cmd nope`
- **THEN** invowk SHALL exit with status 1
- **AND** stderr SHALL contain `broken.invowkmod` and `module_load_skipped`
