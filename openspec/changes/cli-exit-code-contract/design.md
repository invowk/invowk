## Context

The behaviour described here was first verified on main `85f1eb7b` on 2026-11-14, using a local build, an isolated `HOME` and a scratch invowkfile. The branch was then rebased onto main `c0a000da`. `git diff --stat 85f1eb7b c0a000da -- cmd/ internal/ pkg/ tests/` is empty, so the observations still hold, and every `file:line` citation below was re-checked against `c0a000da`. The reserved-flag and `@invowkfile` rows were added during the stage-2 review and observed with a `c0a000da` build the same way; the agent row is derived from code and the existing `agent_*.txtar` tests. S1 (`harden-config-trust`) also edits `cmd/invowk/root.go` and merges first; this branch is rebased onto it afterwards, so re-check the `root.go` citations then.

Observed exit codes:

| Invocation | Today | Path |
|---|---|---|
| `invowk cmd test nonexistent` (group typo) | **0**, help on stdout | group `RunE` returns `cmd.Help()` (`cmd/invowk/cmd_discovery.go:86-112`, help at `:111`) |
| `invowk module bogus`, `agent bogus`, `agent cmd bogus`, `internal bogus` | **0**, help on stdout | non-runnable parent: Cobra returns `flag.ErrHelp` before `ValidateArgs` (`cobra@v1.10.2/command.go:955-956`, `ValidateArgs` at `:966`), and `ExecuteC` turns it into help and a nil error |
| `invowk config bogus`, `tui bogus` | **0**, help on stdout | runnable routing parent with `Args == nil` and `RunE: cmd.Help()` that ignores positionals (`cmd/invowk/config.go:60-62`, `cmd/invowk/tui.go:38-40`) |
| `invowk cmd deploy prod` where `deploy prod` is ambiguous across sources and a unique sibling `deploy dev` exists (so group node `deploy` is registered) | **0**, help | group `deploy` checks `ambiguousPrefixes["deploy"]`, not the full path (`cmd_discovery.go:82,98`) |
| `invowk cmd w2` where `w2` is in the invowkfile and `w2 sub` is in module `m` | **0**, help; the leaf never runs | registration walks commands sorted by `Name` (`internal/discovery/discovery_commands.go:285-287`) but builds nodes by `SimpleName` (`cmd_discovery.go:55-72`). `m w2 sub` sorts before `w2`, so a group node `w2` is created first, and `commandMap` then skips the leaf (`cmd_discovery.go:69-72`) |
| unknown flag (root or leaf), bad flag value | 1 | pflag error → `FlagErrorFunc` (none set, so the parent walk returns the error unchanged: `cobra@v1.10.2/command.go:547-558`; fang sets none: `fang@v1.0.0/fang.go:109-178`) → `Execute` default (`cmd/invowk/root.go:104`) |
| `invowk bogus` | 1 | Cobra `legacyArgs` (`cobra@v1.10.2/args.go:28-39`), called only when `Args == nil` (`command.go:774-776`) |
| `invowk init a b c`, `completion bogus`, `audit a b c` | 1 | Cobra `Args` validators |
| `invowk cmd nope`, `cmd e2` (ambiguous), `cmd @bogus x`, `--ivk-runtime native` (not allowed), missing or extra declared args | 1 | `renderAndWrapServiceError` (`cmd/invowk/app.go:264-333`) → `ServiceError` → default 1 |
| `--ivk-runtime bogus` | 1 | `parsedRuntimeMode` (`cmd/invowk/cmd.go:64-66`) called from `buildCommandExecuteRequest` (`cmd/invowk/cmd_execute_helpers.go:113`) |
| `--ivk-env-inherit-mode bogus` | 1, `Invalid env_inherit_mode "bogus"` | `ParseEnvInheritMode` (`cmd_execute_helpers.go:118-121`) |
| `--ivk-env-var=NOEQ`, `--ivk-env-var==v` | **0**, warning | `parseEnvVarFlags` skips entries whose `=` index is `<= 0` and logs `slog.Warn` (`cmd_execute_helpers.go:60-78`); the skip is pinned by `TestParseEnvVarFlags` (`cmd_execute_helpers_test.go:109`) |
| `--ivk-workdir ' '`, `--ivk-env-inherit-allow 1BAD`, `--ivk-container-name ../x` | 1, `Invalid request: 1 field error(s)` | passed through unvalidated (`cmd_execute_helpers.go:134-147`), rejected later by `commandsvc.Request.Validate` (`internal/app/commandsvc/types_validate.go:74-145`, called at `service.go:153`) as `InvalidRequestError` |
| `--ivk-env-file=` (empty path) | **0**, the command runs | the empty value is not rejected on this path |
| `invowk cmd @invowkfile` (no command name) | 1, `No command specified` | `errNoCommandSpecified` (`cmd/invowk/cmd.go:200`, `cmd_execute_helpers.go:233`, `cmd_watch.go:59`) |
| `invowk cmd --ivk-watch nope` | 1 | `WatchCommandNotFoundError` (`cmd/invowk/cmd_watch.go:87`) |
| `invowk cmd w typo` (leaf `w` with child `w sub`) | 0, runs `w typo` | leaves use `cobra.ArbitraryArgs` (`cmd_discovery.go:178`) |
| script `exit 2` | 2 | `executeRequest` → `ExitError{Code}` (`cmd/invowk/cmd.go:343-345`) → `root.go:101-103` |
| any `invowk agent …` failure, including a bad name or mutually exclusive flags | 2 | every site returns `ExitError{Code: auditExitError}` (`cmd/invowk/agent.go:72,105,150,186,194,233,266,346,367,377,382,409,419,424,450,460,485,495`; shared helpers `cmd/invowk/llm_flags.go:78,93,144,154,169,175,192,197`) |

Existing non-zero codes, gathered from all `ExitError`, `os.Exit` and exit-code producers:

| Code | Producer | Notes |
|---|---|---|
| N | the user command, forwarded: native `extractExitCode` (`internal/runtime/native_helpers.go:59-89`), virtual-sh (`internal/runtime/sh.go:232,258`), virtual-lua (`internal/runtime/lua.go:646-650`), container (`internal/container/engine_base.go:532`), interactive mode (`internal/app/commandadapters/interactive_session.go:46-61`), `tui spin` | Any value in 0–255 (`pkg/types/exit_code.go:36-41`) |
| 125, 126 | container engine or OCI runtime failure | Retried as transient (`internal/runtime/container_exec.go:296`, `internal/container/availability.go:22-24`). After the retries run out, the code is forwarded as if it were the command's own |
| 1 | `Execute` default (`root.go:104`), `NewApp` failure (`root.go:90`) | |
| 1 | runtime failures that are not exit statuses: shell not found, start failure | `native_helpers.go:86-88`, `interactive.go:44-97`, `commandsvc/dispatch.go:122`, `commandsvc/ports.go:127-129` |
| 1 | a child killed by a signal | `ExitCode()` is -1, which fails validation, so 1 is used (`native_helpers.go:76-79`, `interactive_session.go:55-61`). No 128+N convention; S2 owns signals |
| 1 | `invowk validate` found errors (`cmd/invowk/validate.go:139,193,282,306,400`) | |
| 1 | `invowk tui confirm` declined (`cmd/invowk/tui_confirm.go:90`) | Documented in `website/docs/tui/choose-and-confirm.mdx:60-62` |
| 1 | hidden `invowk internal check-cmd` (`cmd/invowk/internal_check_cmd.go:25-47`), `internal exec-virtual-lua` (`internal_exec_lua.go:127`) | `check-cmd` is consumed only as zero or non-zero by container dependency checks (`internal/app/commandadapters/dependency_runtime.go:169-178`) |
| 1 / 2 | `invowk audit`: 1 findings, 2 scan or config error (`cmd/invowk/audit.go:23-25`) | Documented in `website/docs/security/audit.mdx:179-185`, `website/docs/reference/cli.mdx:568` and `README.md:3024-3030`. `audit.mdx:184` says that argument errors use status 1 |
| 2 | every `invowk agent …` failure (see the first table) | Reuses `auditExitError`; undocumented; no intrinsic reason (maintainer decision 3) |
| 1 (after S1) | S1's fatal explicit-config error: `--ivk-config` or `INVOWK_CONFIG` names a missing or invalid file | Not on this branch: today `invowk --ivk-config /nonexistent cmd …` warns and exits 0 (`tests/cli/testdata/config_override_flag.txtar`, Test 4). S1 makes it fail with 1 (maintainer decision 7) |

Constraints:
- Hexagonal rule: domain packages (`internal/app/*`, `internal/agentcmd`, `pkg/*`) return typed errors and `cmd/invowk` renders. This change does not touch domain packages.
- fang's `DefaultErrorHandler` prints `err.Error()` and adds the "Try --help for usage." hint by matching message prefixes (`fang@v1.0.0/help.go:111-149`). It ignores `SilenceErrors`, so every returned error is printed once by fang.
- `cmd/invowk/root.go` is also edited by S1. C1 keeps its `root.go` footprint to two edits: the body of `Execute` and one `installUsageGuards(rootCmd)` call at the end of `NewRootCommand`. Everything else lives in new files.

## Goals / Non-Goals

**Goals:**
- A four-class, documented exit-code contract: 0 / N / 2 / 1.
- Every command-line rejection exits 2, through one typed error and one pure mapping function. That includes invalid values of invowk's reserved `--ivk-*` flags.
- Group typos fail loudly on stderr, for both discovered and built-in groups.
- `invowk agent` follows the contract: usage errors 2, other failures 1.
- Fix the two registration bugs found while investigating: a cross-source prefix leaf is shadowed, and an ambiguous nested name reached through a group shows help. Both are on the code path that the group rule changes.
- Exact exit-code assertions in the CLI tests.

**Non-Goals:**
- Rejecting unmatched tokens after a leaf that has children (`w typo`). Deferred to C3; see Decision 6.
- Fang's `ERROR / Exit status N.` banner and ANSI cleanup (C2, Issue 2 / N2).
- Signal forwarding and 128+N codes (S2, N1).
- Changing `audit` codes, and a dedicated code for "needs a newer invowk" (C3, Issue 6).
- `invowk help bogus`, which exits 0 with "Unknown help topic" (Cobra's built-in help command). Follow-up (maintainer decision 4).
- Implementing S1's explicit-config failure. C1 only records its status (1) in the contract.
- Remapping container-engine codes 125/126.

## Maintainer decisions (2026-11-14)

Approved by the maintainer as "the recommendations, as amended by the advisor review". They replace the former open questions.

1. **Both registration fixes stay in C1**: the prefix leaf hidden by another source's child (Decision 7) and the nested ambiguous name that printed help (Decision 3, step 3). Tests prove that the `invowk cmd` listing order and the shell-completion output do not change.
2. **Usage status is 2.** `EX_USAGE` (64) is a rejected alternative, not a fallback (Decision 4).
3. **`invowk agent` is aligned to the contract now**: usage errors 2, every other failure 1. It exited 2 on every failure only because it reuses `auditExitError`. BREAKING; listed in the migration notes. `audit`'s own 0/1/2 stays as documented (Decision 8).
4. **`invowk help bogus`** is a follow-up, out of scope.
5. **A malformed `--ivk-env-var`** (today exit 0 plus a warning) is in C1: invalid flag values exit 2 (Decision 9).
6. **Migration notes have no shared file.** Each change keeps its text in its own `proposal.md` "Migration notes" section, and the release assembles them.
7. **S1's fatal explicit-config error exits 1** and is listed in the contract table, although S1's code is not on this branch.

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

An exported constant `ExitCodeUsage types.ExitCode = 2` and the constructor `newUsageError(err error) error` sit next to it. `exitCodeForError` lives in the same file. The body of `Execute` (`root.go:95-105`) becomes `os.Exit(int(exitCodeForError(fang.Execute(...))))`; the `NewApp` failure at `root.go:88-91` keeps its direct `os.Exit(1)`. The rules, in order:
1. `nil` → 0.
2. `errors.AsType[*ExitError]` → its `Code`.
3. `errors.AsType[*UsageError]` → 2.
4. Anything else → 1.

Invariant: a `UsageError` never wraps an `ExitError`. `newUsageError` returns its argument unchanged when it already contains an `*ExitError` (so the explicit code wins), and returns `nil` for `nil`. A unit test enforces both. An `ExitError` MAY wrap a `UsageError`; rule 2 then gives the explicit code.

`Error()` returns the wrapped text unchanged. That keeps existing stderr assertions and fang's prefix-based "Try --help" hint working. It also leaves C2 free to switch to `errors.As(err, &UsageError{})` in its own error handler.

Alternatives considered:
- **Return `ExitError{Code: 2}` everywhere.** Rejected as the general mechanism. `ExitError` means "already rendered, stay silent" to invowk's own code (`silenceOnExitError`, `cmd/invowk/cmd.go:229-236`), and `ExitError.Error()` falls back to `exit status N` when `Err` is nil (`cmd/invowk/exit_error.go:18-23`). That couples "which status" to "how it is rendered", which is the exact split C2 needs to control. `agent` and `audit` keep `ExitError` only because they already render their own messages (Decision 8).
- **Classify domain error types centrally in `exitCodeForError`**, with `errors.As` on `deps.ArgumentValidationError`, `commandsvc.ClassifiedError` and the others. Rejected. `renderAndWrapServiceError` wraps `classified.Err` and drops `ClassifiedError.Kind` (`app.go:330`), so command-not-found cannot be recognised downstream. The domain-to-CLI mapping already lives in one adapter function, which is the right place to decide "usage".
- **An `issue.ActionableError`.** Rejected. That type carries operation, resource and suggestions for rendering, not process status. A `UsageError` can wrap one if needed.

### 2. Where `UsageError` is produced

`installUsageGuards(root *cobra.Command)` in `usage_error.go` does all the Cobra-tree wiring. `NewRootCommand` calls it once, after the last `AddCommand` (`root.go:78`). Discovered `invowk cmd` nodes are registered inside `newCmdCommand` (`cmd/invowk/cmd.go:143-148`), so they already exist when the walk runs; they carry an explicit `Args: cobra.ArbitraryArgs`, which never errors, so wrapping it changes nothing.

| Source | Hook |
|---|---|
| pflag parse errors at any level | `installUsageGuards` calls `root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return newUsageError(err) })`. Children inherit it through Cobra's parent walk (`command.go:547-558`) |
| unknown root command | `installUsageGuards` sets `root.Args` to a validator that returns a `UsageError` with Cobra's exact message: `unknown command %q for %q` plus the `SuggestionsFor` suffix. Setting it also bypasses `legacyArgs`. It sets `root.RunE` to show help, because a non-runnable command never reaches `ValidateArgs` (`command.go:955-966`). Bare `invowk` still prints help and exits 0 |
| built-in groups (`config`, `module`, `tui`, `agent`, `agent cmd`, `agent mod`, `internal`, …) | The same tree walk. For every non-root command that has subcommands and `Args == nil`, it sets `Args` to the group guard (Decision 3, rule 4). If the command is not runnable, it also adds `RunE: help`. This covers both non-runnable parents (`module`, `agent`, `internal`) and runnable help-fallback parents (`config`, `tui`). Rule: a parent that really accepts positionals MUST declare `Args` explicitly. `invowk cmd` (`cmd/invowk/cmd.go:72-131`) and the discovered group nodes therefore get an explicit `Args: cobra.ArbitraryArgs`, because they do their own checks in `RunE` (Decision 3); otherwise the guard would reject `invowk cmd nope` before the service runs |
| Cobra `Args` validators on built-ins | The same walk wraps each non-nil `Args` so that its errors become `UsageError` |
| discovered group nodes | Their own `RunE` (Decision 3) |
| `ArgumentValidationError`, `FlagValidationError`, `AmbiguousCommandError`, `SourceNotFoundError`, `RuntimeNotAllowedError`, and `ClassifiedError` with kind `CommandNotFound` or `CommandAmbiguous` | `renderAndWrapServiceError` (`app.go:264-333`) returns `newUsageError(newServiceError(...))`. `errors.AsType[*ServiceError]` still finds the inner value, so `renderServiceError` and the existing tests are unchanged |
| `errNoCommandSpecified` (`app_validate.go:22-23`), `WatchCommandNotFoundError` (`cmd_watch.go:87`), the CLI `AmbiguousCommandError` from `checkAmbiguousCommand` | Wrapped where they are returned. `errors.Is(err, errNoCommandSpecified)` keeps working through `Unwrap` (`cmd_watch_test.go:147`) |
| reserved `--ivk-*` flag values | `buildCommandExecuteRequest` (`cmd_execute_helpers.go:108-149`); see Decision 9 |
| `invowk agent` argv checks | See Decision 8 |

`UnsupportedPlatformError`, `DependencyError`, timeouts, cancellations, engine-not-found, `commandsvc.InvalidRequestError` and every other `ClassifiedError` kind stay at 1. Their cause is the environment or an internal bug, not the argv. S1's explicit-config failure also stays 1, although it comes from a flag value: the path itself is accepted, and what fails is the file's existence or content (maintainer decision 7).

Fang adds its hidden `man` leaf inside `fang.Execute`, after the walk, so its `NoArgs` error stays 1. That is accepted, because the command is hidden.

### 3. Group behaviour

A group's `RunE` applies these rules in order:
1. **No args.** Show help on stdout and exit 0. This is the bare-group decision. A bare group is an explicit request to browse, as with bare `invowk cmd`, which lists commands and exits 0. Cobra's own default for non-runnable parents, `docker network` and `kubectl config` behave the same way. Changing it would break discovery without making scripts any safer.
2. **`--ivk-from` set.** Run the existing disambiguated path (`cmd_discovery.go:90-96`).
3. **Discovered groups only.** Always run `checkAmbiguousCommand(prefix + args)`, whether or not the prefix itself is ambiguous, and drop the `isAmbiguous` gate (`cmd_discovery.go:82,98`). `checkAmbiguousCommand` returns a non-nil error only for an ambiguous path; for a path that is not found, and for a discovery failure, it renders any diagnostics and returns nil (`cmd/invowk/cmd_execute_helpers.go:307-331`), so a plain typo falls through to rule 4. If the full path is ambiguous, render the ambiguous-command message as today (`cmd_discovery.go:99-107`) and return `newUsageError(err)` (exit 2). This fixes `invowk cmd deploy prod` showing help. The resolution reuses the per-request discovery cache, and it only runs when the invocation is already wrong.
4. **Otherwise.** Write the group's available subcommands (name and short description, visible commands only, in Cobra's sorted order) to stderr. Return `newUsageError(fmt.Errorf("unknown command %q for %q%s", args[0], cmd.CommandPath(), suggestions))`, where `suggestions` uses Cobra's `\n\nDid you mean this?\n\t%s\n` format and comes from `cmd.SuggestionsFor(args[0])`. The top-level error handler, fang's today or C2's later, prints the error line. Nothing goes to stdout.

Rule 4 is one helper, `rejectUnmatchedGroupToken(cmd, args) error`, shared by the built-in group guard (Decision 2) and the discovered group `RunE`.

Alternative: put the listing inside the error message. Rejected. Fang styles the error text as a single width-limited block and appends a period, which mangles multi-line lists. Write the listing first and let the handler print the error line last.

### 4. Usage status is 2, not sysexits `EX_USAGE` (64)

Most argument parsers already use 2 for usage errors:
- Go's `flag` package with `ExitOnError`;
- POSIX utilities and Bash builtins ("misuse");
- Python argparse and click;
- Rust clap;
- GNU `grep` and `diff`.

Users and wrappers already read 2 that way.

Rejected alternative: `EX_USAGE=64` from BSD `sysexits.h`. It collides less with scripts, which seldom exit 64, but:
- Few CLIs use it, and the Go ecosystem does not.
- It still collides with any script that uses sysexits.
- It would make invowk the odd one out among the tools it is compared with.

The maintainer chose 2 (decision 2).

The collision is documented. A forwarded `N == 2` looks the same as a usage error, and so does a nested `invowk` inside a user script whose usage error the script propagates. A wrapper that needs certainty reads stderr. The contract is only exact for commands whose scripts avoid codes 1 and 2.

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

Before the node-building loop in `registerDiscoveredCommands` (`cmd_discovery.go:55`), iterate over a copy of `commandSet.Commands` (`slices.Clone`) sorted by `SimpleName` with `slices.SortStableFunc`. A string sorts before any string that extends it with `" …"`, so the leaf `w2` is always built before `w2 sub` and becomes the parent. The children then attach to the leaf through `commandMap`. `commandSet.Commands` itself is not reordered, so `listCommands` (`cmd_discovery.go:362`) and `completeCommands` (`cmd_discovery.go:302`), which read discovery results and not the Cobra tree, keep their output. Cobra sorts subcommands by name for help, so help order does not depend on insertion order either.

Alternative: when a leaf finds an existing group node, upgrade that node in place. Rejected. It needs Cobra node surgery (copying children and flags), while a sort is enough.

This was unforeseen scope; the maintainer kept it in C1 (decision 1), because the C1 group rule would otherwise turn `invowk cmd w2 x` into exit 2 for a command that exists.

### 8. `invowk agent` follows the contract

One rule for every agent site: an error the CLI detects from argv alone, before any file is read or any LLM is called, is a usage error (2). Everything else is a failure (1).

| Site | Error | New code |
|---|---|---|
| `agent.go:72`, `:233` | `agentcmd.RenderCommandPrompt` / `RenderModulePrompt`. `renderPrompt` (`internal/agentcmd/prompt.go:187-219`) fails on an unknown operation (`promptOperations`, `:367-380`), an unknown `--format` (`:217-218`), or JSON encoding of a static struct (`:212-214`) | 2 for every error. The first two are argv errors. `agentcmd` exports no sentinels to tell them apart, and the encoding of a fixed in-memory document cannot fail in practice, so the CLI does not split them; a comment at the site records this |
| `agent.go:105`, `:150`, `:186`, `:266` | `CommandName` / `ModuleID` `Validate()` of `args[0]` | 2 |
| `agent.go:367`, `:409`, `:450`, `:485` | `validateAgent*Options`: mutually exclusive `--dry-run`/`--print`/`--verify` (`agent.go:654-662`), missing description or description plus `--from-file` (`:678-686`), an empty module target, `validateLLMFlagSelection` (`llm_flags.go:53-65`) | 2 |
| `agent.go:194`, `:346` | `agentcmd.RemoveCommand` / `RemoveModule` (not found, `requires --force`, I/O) | 1 |
| `agent.go:377`, `:419`, `:460`, `:495` | `agentcmd.Create*` / `Change*` (already exists, does not exist, LLM output invalid, I/O) | 1 |
| `agent.go:382`, `:424` | `verifyGeneratedCommand` | 1 |
| `llm_flags.go:78` | `llmConfigFlagValues` (`llm_flags.go:98-130`): an invalid `--llm-provider`, `--llm-url`, `--llm-model` or `--llm-concurrency` value | 2 |
| `llm_flags.go:93`, `:144`, `:154`, `:169`, `:175`, `:192`, `:197` | config resolution (which also merges the config file), "LLM is not enabled", API-config validation, provider detection, model verification, client construction | 1. These can involve config-file values, so they are not argv-only. `--llm-timeout` and `--llm-api-key` are not checked at `:78`; a bad value surfaces here and exits 1 |

`requires --force` stays 1 because the domain decides it after inspecting the module directory, not from argv alone.

Mechanism: in `agent.go`, each site keeps returning `&ExitError{Code, Err}`, whose `Error()` delegates to `Err` (`exit_error.go:18-23`), so fang prints the same message as today; only the code changes. The sites use `ExitCodeUsage` or a new `agentExitFailure types.ExitCode = 1` instead of `auditExitError`. The shared helpers `resolveLLMForCommand` and `buildLLMCompleter` (`llm_flags.go:67-197`), also used by `audit` (`audit.go:240,298`), take an `llmExitCodes{usage, failure types.ExitCode}` parameter and pass it down to `buildProviderCompleter` and `buildManualCompleter`. `audit` passes `{auditExitError, auditExitError}`, so all of its codes are unchanged. `agent` passes `{ExitCodeUsage, agentExitFailure}` through `resolveAgentAuthoringCompleter` (`agent.go:515-533`).

Alternatives considered:
- Return plain errors and `UsageError` from agent and let `exitCodeForError` decide. Rejected: the shared `llm_flags.go` helpers print their own message before returning (`llm_flags.go:77`, `:92`) and must keep returning audit's explicit 2, so an explicit code per site is the smaller, uniform change. C2 can revisit rendering.
- Keep agent at 2 and document it as an exception. Rejected by the maintainer (decision 3).

### 9. Invalid reserved `--ivk-*` flag values exit 2

`buildCommandExecuteRequest` (`cmd_execute_helpers.go:108-149`) is the single place that turns reserved flags into an `ExecuteRequest`. Every value it rejects returns `newUsageError(err)`:
- `--ivk-runtime`: the `parsedRuntimeMode` error (`:113-116`).
- `--ivk-env-inherit-mode`: the `ParseEnvInheritMode` error (`:118-121`).
- `--ivk-env-var`: `parseEnvVarFlags` (`:60-78`) returns `(map[string]string, error)`. An entry whose first `=` is missing or at index 0 (`NOEQ`, `=v`) is an error, `invalid --ivk-env-var value %q: expected KEY=VALUE`, instead of a `slog.Warn` and a skip. `KEY=` (empty value) stays valid.
- `--ivk-workdir`, `--ivk-env-file` (an empty path, which today is not rejected at all), `--ivk-env-inherit-allow`, `--ivk-env-inherit-deny` and `--ivk-container-name`: the CLI calls the same `Validate()` that `commandsvc.Request.Validate` (`types_validate.go:74-145`) would call, on each typed value, before building the request (`ContainerName.Validate` is `pkg/containerargs/container_name.go:39`). These checks are syntactic only (for example `WorkDir.Validate` rejects whitespace-only values, `pkg/invowkfile/workdir.go:40-48`; `EnvVarName.Validate` applies the name regex, `pkg/invowkfile/env.go:71-76`); a missing env file stays a runtime failure (1).

`commandsvc.InvalidRequestError` stays 1. After this change it is reachable only for fields the CLI does not validate first (for example `ResolvedCommand`), which indicates a bug, not a usage error. `opts.Validate()` at `:109-111` validates internal options and also stays 1.

### 10. Testing approach

- **Pure unit tests:**
  - `exitCodeForError` table: nil, `ExitError` 0/1/2/42, `UsageError`, `UsageError` wrapping `ServiceError`, `ExitError` wrapping `UsageError`, plain error;
  - `newUsageError` invariant (nil in, nil out; an `ExitError` inside is returned unchanged);
  - `renderAndWrapServiceError` usage classification, extending `cmd/invowk/cmd_execute_error_classifier_test.go`;
  - the root `Args` validator, group `RunE` and `installUsageGuards`, on a synthetic Cobra tree;
  - registration order, with sources in both orders, plus unchanged `commandSet.Commands` order;
  - `parseEnvVarFlags` and `buildCommandExecuteRequest` reserved-flag errors;
  - agent and LLM-helper codes per Decision 8.
- **End-to-end:** `! exec` passes for any non-zero status, and `tests/cli/cmd_test.go` defines no custom commands (`testscript.Params` at `cmd_test.go:199-203`). Add `Cmds: {"exitcode": …}` to `testscript.Params`. Its syntax is `exitcode <want> <cmd> <args…>`. It calls `ts.Exec` (`go-internal@v1.16.0/testscript/testscript.go:1211-1214`), which runs the command in the script's cwd with the script's environment and stores stdout and stderr so `stdout`/`stderr` assertions keep working, and it fails unless the status equals `<want>`. It rejects `!` negation, because the expected code is explicit. `exec` keeps working everywhere else. The container runner (`tests/cli/cmd_container_test.go:167`) gets the same `Cmds` entry.
- Built-in command cases are exempt from native mirrors (`.agents/rules/testing.md`, built-in exemption). User-command cases, such as group typos, declared-arg errors, reserved-flag errors and forwarded codes, need `virtual_*` and `native_*` pairs with platform-split CUE.

## Risks / Trade-offs

- [Scripts that check `== 1` for usage errors, or `== 2` for agent failures, break] → BREAKING notes and migration text in proposal.md; released in the bundled v0.17.0.
- [Exit 2 collides with a script's own `exit 2`, a nested invowk, and audit's scan-error 2] → Documented in the contract. Wrappers read stderr.
- [Scripts that passed a malformed `--ivk-env-var` and relied on it being skipped now fail] → Migration note; the old behaviour silently dropped the variable, which hid the mistake.
- [Making the root and built-in groups runnable changes help rendering] → Fang's usage line depends on `Use` and subcommands, not on `Runnable` (`fang@v1.0.0/help.go:163-185`). Verify with `version_help.txtar` and a before/after diff of `invowk --help`, `invowk config --help`, `invowk module --help` and `invowk cmd --help`. `TestBuiltinCommandTxtarCoverage` already skips runnable routing nodes (`cmd/invowk/cmd_coverage_test.go:346-352`).
- [Completion or listing regressions] → Cobra's `__complete` does not run `Args` or `RunE`, and the registration sort works on a copy (Decision 7). Add txtar checks that `__complete config ""`, `__complete cmd ""` and `__complete cmd test ""` keep their candidates, and that the `invowk cmd` listing keeps its order; keep `completion.txtar`'s existing assertions unchanged.
- [Engine codes 125/126 forwarded as N] → Documented as a known limitation. Remapping them would need container-runtime changes and is out of scope.
- [Merge conflicts with S1 in `root.go`] → Two small, separated edits (Constraints). Rebase onto S1 after it merges and re-run the `root.go` unit tests.
- [Hidden txtar dependence on group help with extra tokens, or on agent exit 2] → Run `make test-cli` early (task 1.3) and triage every failure against the spec.

## Migration Plan

Ship in v0.17.0. The text in proposal.md "Migration notes" goes into the release notes; per maintainer decision 6, the release assembles each change's own notes, and there is no shared file. To roll back, revert the commit. No data or schema migration is involved.

## Open Questions

None. Every former question is resolved under "Maintainer decisions (2026-11-14)". Follow-ups outside C1: `invowk help <unknown-topic>` (decision 4), strict tokens after a leaf with children (C3, Decision 6), and 125/126 remapping.
