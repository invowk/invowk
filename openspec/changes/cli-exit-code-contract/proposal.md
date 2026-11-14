## Why

Invowk's exit status cannot tell a wrapper whether invowk rejected the command line or whether the work itself failed. A typo under a command group (`invowk cmd test nonexistent`) or a built-in group (`invowk config bogus`) exits **0** and prints help to stdout. Every other usage error (unknown flag or command, bad flag value, wrong argument count, unknown `@source`, ambiguous command) exits **1**, the same status as a config, discovery or dependency failure. `Execute` maps every error that is not an `ExitError` to 1 (`cmd/invowk/root.go:95-105`).

CST and other callers that wrap invowk in systemd units, shell wrappers or CI need a documented contract. This change is C1 in the CST adoption track (`.claude/issues/cst-adoption-gaps.md`, Issue 1). It ships in the bundled BREAKING release (target `v0.17.0`).

## What Changes

- **BREAKING**: Invowk exits with status **2** whenever it rejects the command line before running any user code. The cases:
  - unknown flag, a flag value that does not parse, a missing flag value;
  - unknown root command (`invowk bogus`);
  - an unmatched token under a discovered command group (`invowk cmd test nonexistent`) or a built-in group (`invowk config bogus`, `invowk module bogus`, `invowk tui bogus`, `invowk agent bogus`);
  - Cobra argument-arity errors on built-in commands (`invowk init a b c`, `invowk completion bogus`);
  - unknown command under `invowk cmd`, including in watch mode;
  - an ambiguous command name;
  - `@source` or `--ivk-from` naming a source that does not exist;
  - an invalid `--ivk-runtime` value, or a runtime the command does not allow;
  - declared-argument errors (`deps.ArgumentValidationError`) and invowkfile flag-validation errors (`deps.FlagValidationError`);
  - `invowk cmd @src` with no command name.
- **BREAKING**: Usage errors under a command group no longer print help to stdout with status 0. They print an `unknown command` error, `Did you mean` suggestions (Cobra `SuggestionsFor`) and the group's available subcommands, all on **stderr**, and exit 2. A bare group with no arguments (`invowk cmd test`, `invowk config`) still prints help to stdout and exits 0.
- A typed CLI-layer `UsageError`, plus one pure function that maps an error to an exit code. `Execute` uses that function, so it can be unit-tested.
- Discovered-command registration is fixed so that a command which is also the prefix of a command from another source (leaf `w2` in the invowkfile, `w2 sub` in a module) is always registered as a runnable leaf. On `85f1eb7b` it is shadowed by a help-only group node, and `invowk cmd w2` prints help instead of running. Without this fix, the new group rule would turn `invowk cmd w2 x` into exit 2 for a command that exists.
- A documented exit-code contract:

  | Code | Meaning |
  |---|---|
  | 0 | success |
  | N | the user command's own exit code, forwarded |
  | 2 | invowk rejected the command line |
  | 1 | invowk failed for another reason |

  The documentation lists the per-command exceptions (`audit`, `agent`, `tui confirm`, `validate`). It states that 2 can also be a script's own code, and that container-engine codes 125/126 are forwarded after retries.
- A testscript helper that asserts an exact exit code, because `! exec` passes for any non-zero status.
- Not in this change (deferred to C3 `command-contract-schema`): rejecting unmatched tokens after a leaf that also has children (`invowk cmd w typo` still runs `w` with `typo` as `$1`). See design.md, Decision 6.

### Migration notes (for the v0.17.0 release section)

> **Exit codes for usage errors changed (C1).** Invowk now exits with status `2` when it rejects the command line: unknown command, flag or `@source`; bad flag value; wrong number of arguments; ambiguous command name; or a runtime the command does not allow. These used to exit `1`. Typos under a command group used to print help and exit `0`. They now print an error with suggestions to stderr and exit `2`.
>
> Status `1` now means only that invowk failed for another reason, such as config, discovery, dependency checks, an unsupported platform or a missing container engine. A non-zero status from your own command is still forwarded unchanged.
>
> What to update:
> - Scripts that test `$? -eq 1` to detect a mistyped invocation should test for `2`, or for any non-zero status.
> - Scripts that ran `invowk cmd <group> <word>` and expected help on stdout should run `invowk cmd <group>` (still status 0) or `invowk cmd <group> --help`.
> - Your own commands can also exit with `2`. If you must tell the two apart, read stderr, or give your scripts codes other than 1 and 2.
> - `invowk audit` keeps `0` = clean, `1` = findings, `2` = scan error. Its argument errors now also exit `2` instead of `1`.

## Capabilities

### New Capabilities
- `cli-exit-codes`: the process exit-code contract of the `invowk` binary. It covers success, forwarding a user command's code, rejecting the command line (2), other invowk failures (1), per-command exceptions, and where usage-error output goes (stdout or stderr) for command groups.

### Modified Capabilities
<!-- None: no existing spec in openspec/specs/ states CLI exit-code requirements (checked 2026-11-14). -->

## Impact

- **Code** (`cmd/invowk/` only; domain packages unchanged):
  - `root.go`: the `Execute` mapping, a root `SetFlagErrorFunc`, and a root `Args` validator.
  - New `usage_error.go` and an exit-code mapper.
  - `cmd_discovery.go`: group nodes, plus the registration-order fix.
  - `app.go` (`renderAndWrapServiceError`), `cmd.go`, `cmd_execute_helpers.go` and `cmd_watch.go`: wrap the usage-class errors.
  - A guard for built-in group commands, applied when the tree is built.
- **Tests**:
  - Go unit tests for the mapper and for group/registration behaviour.
  - A new exact-exit-code command in the `tests/cli` testscript harness.
  - New txtar cases, with virtual/native mirrors for user-command cases. Existing txtar cases that relied on group help with extra tokens need triage.
- **Docs**:
  - `website/docs/reference/cli.mdx` (Exit Codes) and `website/docs/security/audit.mdx:184`, plus their pt-BR copies under `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`.
  - `README.md` (Exit Codes) and the CLI skill (`.agents/skills/cli/SKILL.md`, "Error Handling & Exit Codes").
  - `website/versioned_docs` is not edited.
- **Ordering**: lands after S1, S2 and C2 (merge order S1 → S2 → C2 → C1). S2 and C2 also edit `cmd/invowk/root.go`, and C2 may replace fang's error handler. Line citations are against main `85f1eb7b` and must be re-checked on rebase.
- **Completion**: shell-completion output must not change (`completion.txtar`, `__complete` paths).
