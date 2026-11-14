## Context

The behaviour described here was verified on main `85f1eb7b` on 2026-11-14, using a local build, an isolated `HOME` and a scratch invowkfile. All `file:line` citations are against that commit. S2 and C2 land before C1 and also edit `cmd/invowk/root.go`, so re-check the citations on rebase.

Observed exit codes:

| Invocation | Today | Path |
|---|---|---|
| `invowk cmd test nonexistent` (group typo) | **0**, help on stdout | group `RunE` returns `cmd.Help()` (`cmd/invowk/cmd_discovery.go:85-111`) |
| `invowk module bogus`, `agent bogus`, `agent cmd bogus`, `internal bogus` | **0**, help on stdout | non-runnable parent: Cobra returns `flag.ErrHelp` before `ValidateArgs` (`cobra@v1.10.2/command.go:955-956`), and `ExecuteC` turns it into help and a nil error |
| `invowk config bogus`, `tui bogus` | **0**, help on stdout | runnable routing parent with `Args == nil` and `RunE: cmd.Help()` that ignores positionals (`cmd/invowk/config.go:60-62`, `cmd/invowk/tui.go:38-40`) |
| `invowk cmd deploy prod` where `deploy prod` is ambiguous across sources and a unique sibling `deploy dev` exists (so group node `deploy` is registered) | **0**, help | group `deploy` checks `ambiguousPrefixes["deploy"]`, not the full path (`cmd_discovery.go:84,98`) |
| `invowk cmd w2` where `w2` is in the invowkfile and `w2 sub` is in module `m` | **0**, help; the leaf never runs | registration walks commands sorted by `Name` (`internal/discovery/discovery_commands.go:285-287`) but builds nodes by `SimpleName`. `m w2 sub` sorts before `w2`, so a group node `w2` is created first, and `commandMap` then skips the leaf (`cmd_discovery.go:68-71`) |
| unknown flag (root or leaf), bad flag value | 1 | pflag error → `FlagErrorFunc` (none set; fang sets none: `fang@v1.0.0/fang.go:110-178`) → `Execute` default (`root.go:104`) |
| `invowk bogus` | 1 | Cobra `legacyArgs` (`cobra@v1.10.2/args.go:28-39`), called only when `root.Args == nil` (`command.go:775-776`) |
| `invowk init a b c`, `completion bogus`, `audit a b c` | 1 | Cobra `Args` validators |
| `invowk cmd nope`, `cmd e2` (ambiguous), `cmd @bogus x`, `--ivk-runtime native` (not allowed), missing or extra declared args | 1 | `renderAndWrapServiceError` (`cmd/invowk/app.go:263-332`) → `ServiceError` → default 1 |
| `--ivk-runtime bogus` | 1 | `parsedRuntimeMode` (`cmd/invowk/cmd.go:64`, `cmd_execute_helpers.go:113`) |
| `invowk cmd --ivk-watch nope` | 1 | `WatchCommandNotFoundError` (`cmd/invowk/cmd_watch.go:87`) |
| `invowk cmd w typo` (leaf `w` with child `w sub`) | 0, runs `w typo` | leaves use `cobra.ArbitraryArgs` (`cmd_discovery.go:178`) |
| script `exit 2` | 2 | `executeRequest` → `ExitError{Code}` (`cmd/invowk/cmd.go:343-345`) → `root.go:101-103` |
| `--ivk-env-var=NOEQ` | 0, warning | malformed value ignored with a warning |

Existing non-zero codes, gathered from all `ExitError`, `os.Exit` and exit-code producers:

| Code | Producer | Notes |
|---|---|---|
| N | the user command, forwarded: native `extractExitCode` (`internal/runtime/native_helpers.go:59-89`), virtual-sh (`internal/runtime/sh.go:232,258`), virtual-lua (`internal/runtime/lua.go:648`), container (`internal/container/engine_base.go:532`), interactive mode (`internal/app/commandadapters/interactive_session.go:46-61`), `tui spin` | Any value in 0–255 (`pkg/types/exit_code.go:36-41`) |
| 125, 126 | container engine or OCI runtime failure | Retried as transient (`internal/runtime/container_exec.go:296`, `internal/container/availability.go:22`). After the retries run out, the code is forwarded as if it were the command's own |
| 1 | `Execute` default (`root.go:104`), `NewApp` failure (`root.go:91`) | |
| 1 | runtime failures that are not exit statuses: shell not found, start failure | `native_helpers.go:86-88`, `interactive.go:44-97`, `commandsvc/dispatch.go:122`, `commandsvc/ports.go:127-129` |
| 1 | a child killed by a signal | `ExitCode()` is -1, which fails validation, so 1 is used (`native_helpers.go:76-79`, `interactive_session.go:55-61`). No 128+N convention; S2 owns signals |
| 1 | `invowk validate` found errors (`cmd/invowk/validate.go:139,193,282,306,400`) | |
| 1 | `invowk tui confirm` declined (`cmd/invowk/tui_confirm.go:90`) | Documented in `website/docs/tui/choose-and-confirm.mdx:61-62` |
| 1 | hidden `invowk internal check-cmd` (`cmd/invowk/internal_check_cmd.go:25-47`), `internal exec-virtual-lua` (`internal_exec_lua.go:127`) | `check-cmd` is consumed only as zero or non-zero by container dependency checks (`internal/app/commandadapters/dependency_runtime.go:169-178`) |
| 1 / 2 | `invowk audit`: 1 findings, 2 scan or config error (`cmd/invowk/audit.go:22-24`) | Documented in `website/docs/security/audit.mdx:179-185`, `website/docs/reference/cli.mdx:568` and README §Exit Codes. `audit.mdx:184` says that argument errors use status 1 |
| 2 | every `invowk agent …` failure (`cmd/invowk/agent.go:72-495`, `llm_flags.go:78-154`) | Reuses `auditExitError`; undocumented |

Constraints:
- Hexagonal rule: domain packages (`internal/app/*`, `pkg/*`) return typed errors and `cmd/invowk` renders. This change does not touch domain packages.
- fang's `DefaultErrorHandler` prints `err.Error()` and adds the "Try --help for usage." hint by matching message prefixes (`fang@v1.0.0/help.go:111-149`). It ignores `SilenceErrors`, so every returned error is printed once by fang.

## Goals / Non-Goals

**Goals:**
- A four-class, documented exit-code contract: 0 / N / 2 / 1.
- Every command-line rejection exits 2, through one typed error and one pure mapping function.
- Group typos fail loudly on stderr, for both discovered and built-in groups.
- Fix the two registration bugs found while investigating: a cross-source prefix leaf is shadowed, and an ambiguous nested name reached through a group shows help. Both are on the code path that the group rule changes.
- Exact exit-code assertions in the CLI tests.

**Non-Goals:**
- Rejecting unmatched tokens after a leaf that has children (`w typo`). Deferred to C3; see Decision 6.
- Fang's `ERROR / Exit status N.` banner and ANSI cleanup (C2, Issue 2 / N2).
- Signal forwarding and 128+N codes (S2, N1).
- Changing `agent` or `audit` codes. A dedicated code for "needs a newer invowk" (C3, Issue 6).
- `invowk help bogus`, which exits 0 with "Unknown help topic" (Cobra's built-in help command). Listed as an open question.
- Turning the malformed `--ivk-env-var` warning into an error. Listed as an open question.

## Decisions

### 1. A CLI-layer `UsageError` plus a pure `exitCodeForError`

New `cmd/invowk/usage_error.go`:

```go
// UsageError marks an error caused by the shape of the command line
// (unknown command/flag/source, bad flag value, arity, ambiguity).
type UsageError struct{ Err error }
func (e *UsageError) Error() string { return e.Err.Error() } // message unchanged
func (e *UsageError) Unwrap() error { return e.Err }
```

An exported constant `ExitCodeUsage types.ExitCode = 2` sits next to it. `Execute` is reduced to `os.Exit(int(exitCodeForError(err)))`. The rules, in order:
1. `nil` → 0.
2. `errors.AsType[*ExitError]` → its `Code`.
3. `errors.AsType[*UsageError]` → 2.
4. Anything else → 1.

Invariant: a `UsageError` never wraps an `ExitError`. Constructors and a unit test enforce it, so rule order is never ambiguous.

`Error()` returns the wrapped text unchanged. That keeps existing stderr assertions and fang's prefix-based "Try --help" hint working. It also leaves C2 free to switch to `errors.As(err, &UsageError{})` in its own error handler.

Alternatives considered:
- **Return `ExitError{Code: 2}` directly.** Rejected. `ExitError` means "already rendered, stay silent" to invowk's own code (`silenceOnExitError`, `cmd/invowk/cmd.go:229-235`), and `ExitError.Error()` falls back to `exit status N` when `Err` is nil. That couples "which status" to "how it is rendered", which is the exact split C2 needs to control, and it mixes up classification with exit signalling. A separate type keeps the original error and lets any handler choose how to render it.
- **Classify domain error types centrally in `exitCodeForError`**, with `errors.As` on `deps.ArgumentValidationError`, `commandsvc.ClassifiedError` and the others. Rejected. `renderAndWrapServiceError` wraps `classified.Err` and drops `ClassifiedError.Kind` (`app.go:330`), so command-not-found cannot be recognised downstream. The domain-to-CLI mapping already lives in one adapter function, which is the right place to decide "usage".
- **An `issue.ActionableError`.** Rejected. That type carries operation, resource and suggestions for rendering, not process status. A `UsageError` can wrap one if needed.

### 2. Where `UsageError` is produced

| Source | Hook |
|---|---|
| pflag parse errors at any level | `rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &UsageError{Err: err} })`. Children inherit it through Cobra's parent walk (`command.go:547-558`) |
| unknown root command | Set `rootCmd.Args` to a validator that returns a `UsageError` with Cobra's exact message: `unknown command %q for %q` plus the `SuggestionsFor` suffix. Setting it also bypasses `legacyArgs`. The root also gets `RunE: help`, because a non-runnable command never reaches `ValidateArgs` (`command.go:955-968`). Bare `invowk` still prints help and exits 0 |
| built-in groups (`config`, `module`, `tui`, `agent`, `agent cmd`, `agent mod`, `internal`, …) | A single `installUsageGuards(root)` tree walk at the end of `NewRootCommand`. For every non-root command that has subcommands and `Args == nil`, it sets `Args` to the group guard (Decision 3). If the command is not runnable, it also adds `RunE: help`. This covers both non-runnable parents (`module`, `agent`, `internal`) and runnable help-fallback parents (`config`, `tui`). Rule: a parent that really accepts positionals MUST declare `Args` explicitly. `invowk cmd` (`cmd/invowk/cmd.go:72`) and the discovered group nodes therefore get an explicit `Args: cobra.ArbitraryArgs`, because they do their own checks in `RunE` (Decision 3); otherwise the guard would reject `invowk cmd nope` before the service runs |
| Cobra `Args` validators on built-ins | The same walk wraps each non-nil `Args` so that its errors become `UsageError`. `cobra.ArbitraryArgs` never errors, so discovered leaves are unaffected |
| discovered group nodes | Their own `RunE` (Decision 3) |
| `ArgumentValidationError`, `FlagValidationError`, `AmbiguousCommandError`, `SourceNotFoundError`, `RuntimeNotAllowedError`, and `ClassifiedError` with kind `CommandNotFound` or `CommandAmbiguous` | `renderAndWrapServiceError` returns `&UsageError{Err: newServiceError(...)}`. `errors.AsType[*ServiceError]` still finds the inner value, so `renderServiceError` and the existing tests are unchanged |
| `errNoCommandSpecified` (`app_validate.go:22-23`), invalid `--ivk-runtime` value (`cmd_execute_helpers.go:113`), `WatchCommandNotFoundError` (`cmd_watch.go:87`), the CLI `AmbiguousCommandError` from `checkAmbiguousCommand` | Wrapped where they are returned. `errors.Is(err, errNoCommandSpecified)` keeps working through `Unwrap` (`cmd_watch_test.go:147`) |

`UnsupportedPlatformError`, `DependencyError`, timeouts, cancellations, engine-not-found and every other `ClassifiedError` kind stay at 1. Their cause is the environment, not the argv.

Fang adds its hidden `man` leaf inside `fang.Execute`, after the walk, so its `NoArgs` error stays 1. That is accepted, because the command is hidden.

### 3. Group behaviour

A group's `RunE` applies these rules in order:
1. **No args.** Show help on stdout and exit 0. This is the bare-group decision. A bare group is an explicit request to browse, as with bare `invowk cmd`, which lists commands and exits 0. Cobra's own default for non-runnable parents, `docker network` and `kubectl config` behave the same way. Changing it would break discovery without making scripts any safer.
2. **`--ivk-from` set.** Run the existing disambiguated path (`cmd_discovery.go:89-95`).
3. **Discovered groups only.** Always run `checkAmbiguousCommand(prefix + args)`, whether or not the prefix itself is ambiguous. If the full path is ambiguous, render the ambiguous-command message and return `UsageError` (exit 2). This fixes `invowk cmd deploy prod` showing help. The resolution reuses the per-request discovery cache, and it only runs when the invocation is already wrong.
4. **Otherwise.** Write the group's available subcommands (name and short description, visible commands only) to stderr. Return `&UsageError{Err: fmt.Errorf("unknown command %q for %q%s", args[0], cmd.CommandPath(), suggestions)}`, where `suggestions` uses Cobra's `Did you mean this?` format and comes from `cmd.SuggestionsFor(args[0])`. The top-level error handler, fang's today or C2's later, prints the error line. Nothing goes to stdout.

Alternative: put the listing inside the error message. Rejected. Fang styles the error text as a single width-limited block and appends a period, which mangles multi-line lists. Write the listing first and let the handler print the error line last.

### 4. Usage status is 2, not sysexits `EX_USAGE` (64)

Most argument parsers already use 2 for usage errors:
- Go's `flag` package with `ExitOnError`;
- POSIX utilities and Bash builtins ("misuse");
- Python argparse and click;
- Rust clap;
- GNU `grep` and `diff`.

Users and wrappers already read 2 that way.

Alternative: `EX_USAGE=64` from BSD `sysexits.h`. It collides less with scripts, which seldom exit 64. Rejected as the default:
- Few CLIs use it, and the Go ecosystem does not.
- It still collides with any script that uses sysexits.
- It would make invowk the odd one out among the tools it is compared with.

Recorded as the fallback if the maintainer weighs collision avoidance higher.

The collision is documented either way. A forwarded `N == 2` looks the same as a usage error, and so does a nested `invowk` inside a user script whose usage error the script propagates. A wrapper that needs certainty reads stderr. The contract is only exact for commands whose scripts avoid codes 1 and 2.

### 5. Unknown and ambiguous command names both exit 2

Both are rejections of the argv. Nothing ran, and changing the command line fixes them: correct the name, or add `@source` / `--ivk-from`. A distinct code for ambiguity (for example 3) was considered and rejected. Every extra reserved code narrows the space left for user scripts, and no consumer has asked to tell the two apart. Stderr already says which one it was.

### 6. Leaf-with-children (`w typo`) is out of C1 and deferred to C3

The problem: `invowk cmd w typo` runs `w` with `$1=typo` and exits with the script's code, so a typo of `sub` runs the wrong command. Leaves accept `cobra.ArbitraryArgs` (`cmd_discovery.go:178`). A leaf that has children cannot declare `args`, because `ValidateCommandTree` rejects that (`pkg/invowkfile/command_tree.go:89`). Undeclared positionals are therefore the only way to pass arguments to such a leaf. Issue 5 confirms they are relied on, and `--` forwarding depends on them.

Options looked at:
- (a) Reject any undeclared positional on a leaf with children. Breaks legitimate use with no opt-out.
- (b) Reject only when the token is within edit distance of a child name. Heuristic: a legitimate value such as `sbu` would be refused, depending on what children happen to exist. Rejected.
- (c) Warn on stderr instead of failing. Adds noise, which C2 is removing, and machines ignore it anyway. Rejected.
- (d) Make the rule strict once C3 adds `passthrough: true` (and possibly lets declared `args` coexist with children). That gives a safe opt-out. **Chosen.**

C1 pins today's behaviour in a spec requirement and a test, so that C3's change is visible.

### 7. Registration order fix for prefix leaves

Before the node-building loop in `registerDiscoveredCommands`, iterate over a copy of `commandSet.Commands` sorted by `SimpleName`. A string sorts before any string that extends it with `" …"`, so the leaf `w2` is always built before `w2 sub` and becomes the parent. The children then attach to the leaf through `commandMap`.

Alternative: when a leaf finds an existing group node, upgrade that node in place. Rejected. It needs Cobra node surgery (copying children and flags), while a sort is enough.

This is unforeseen scope. general-rules says to ask before folding it in. It is included here because the C1 group rule would otherwise turn `invowk cmd w2 x` into exit 2 for a command that exists. The maintainer may split it out (see Open Questions).

### 8. Testing approach

- **Pure unit tests:**
  - `exitCodeForError` table: nil, `ExitError` 0/2/42, `UsageError`, `UsageError` wrapping `ServiceError`, plain error;
  - `renderAndWrapServiceError` usage classification, extending `cmd/invowk/cmd_execute_error_classifier_test.go`;
  - the root `Args` validator, group `RunE` and `installUsageGuards`, on a synthetic Cobra tree;
  - registration order, with sources in both orders.
- **End-to-end:** `! exec` passes for any non-zero status, and `tests/cli/cmd_test.go` defines no custom commands. Add `Cmds: {"exitcode": …}` to `testscript.Params`. Its syntax is `exitcode <want> invowk <args…>`. It runs the built binary in the script's cwd with the script's environment, copies stdout and stderr into the testscript buffers so `stdout`/`stderr` assertions keep working, and fails unless the status equals `<want>`. It must not support `!` negation, because the expected code is explicit. `exec` keeps working everywhere else.
- Built-in command cases are exempt from native mirrors (`.agents/rules/testing.md`, built-in exemption). User-command cases, such as group typos, declared-arg errors and forwarded codes, need `virtual_*` and `native_*` pairs with platform-split CUE.

## Risks / Trade-offs

- [Scripts that check `== 1` for usage errors break] → BREAKING note and migration text in proposal.md; released in the bundled v0.17.0.
- [Exit 2 collides with a script's own `exit 2`, a nested invowk, and audit's scan-error 2] → Documented in the contract. Wrappers read stderr. 64 is recorded as an alternative.
- [Making the root and built-in groups runnable changes help rendering] → Fang's usage line depends on `Use` and subcommands, not on `Runnable` (`fang@v1.0.0/help.go:163-185`). Verify with `version_help.txtar` and a before/after diff of `invowk --help`, `invowk config --help` and `invowk cmd --help`. `TestBuiltinCommandTxtarCoverage` already skips runnable routing nodes (`cmd/invowk/cmd_coverage_test.go:345-351`).
- [Completion regressions] → Cobra's `__complete` does not run `Args` or `RunE`. Add a txtar check that `__complete config ""` and `__complete cmd test ""` keep listing subcommands, and keep `completion.txtar` unchanged.
- [Engine codes 125/126 forwarded as N] → Documented as a known limitation. Remapping them would need container-runtime changes and is out of scope.
- [Merge conflicts with S2 and C2 in `root.go`] → C1 lands last of the three. Keep the `Execute` edit to the single `exitCodeForError` call.
- [Hidden txtar dependence on group help with extra tokens] → Run `make test-cli` early (task 1.1) and triage every failure against the spec.

## Migration Plan

Ship in v0.17.0 with the migration text from proposal.md in the single BREAKING section, alongside S1 and F12. To roll back, revert the commit. No data or schema migration is involved.

## Open Questions

1. Should `invowk agent` failures move from 2 to 1 to match the global contract, since this release is breaking anyway? Proposed answer: no. Keep them out of C1, document them as an exception, and revisit with agent docs.
2. Should `invowk help <unknown-topic>` exit 2? It needs a custom help command. Proposed answer: leave it to a follow-up.
3. Should a malformed `--ivk-env-var` value (no `=`) be a usage error (2) instead of a warning (0)? Proposed answer: yes, in C3 or C2, not here.
4. Where does the v0.17.0 migration text live? The bundled-release tracker or release-notes draft is not named yet.
5. `invowk --ivk-config /nonexistent cmd test unit` exits **0**: the config load error becomes a warning in `resolveUIFlags` (`cmd/invowk/cmd.go`) and execution goes on. Is an unreadable explicit `--ivk-config` a usage error (2), an internal error (1), or acceptable? Proposed answer: hand it to S1 (`harden-config-trust`), which owns config resolution. C1 does not specify it.
6. Should the registration-order fix (Decision 7) and the ambiguous nested-name fix (Decision 3, step 3) stay in C1 or ship separately before it? Proposed answer: keep them in C1, because the group rule depends on both.
7. Is 2 or 64 final? Proposed answer: 2 (Decision 4).
