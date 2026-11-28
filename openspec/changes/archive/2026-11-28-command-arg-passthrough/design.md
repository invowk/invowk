# Design: Raw Command Argument Passthrough (`passthrough: true`)

## Context

Current behavior in `main`:
- `pkg/invowkfile/invowkfile_schema.cue:537-593` defines `#Command` with `flags` and `args`, without a `passthrough` option.
- `pkg/invowkfile/command_tree.go:77-111` validates that commands declaring `args` cannot also have child subcommands (`ArgsSubcommandConflictError`).
- `cmd/invowk/cmd_discovery.go:185-233` builds leaf Cobra commands with `Args: cobra.ArbitraryArgs` (`cmd_discovery.go:232`), so Cobra parses flags everywhere on the command line (rejecting unknown flags with exit `2` and intercepting `--help`/`-h` unless preceded by `--`), and registers leaf-only `--ivk-*` flags (`--ivk-workdir`, `--ivk-env-var`, `--ivk-env-file`, `--ivk-env-inherit-*`) at `cmd_discovery.go:236-241` instead of on `cmdCmd`.
- `cmd/invowk/cmd.go:73-147` defines `cmdCmd` with only 6 persistent flags (`--ivk-runtime`, `--ivk-from`, `--ivk-force-rebuild`, `--ivk-container-name`, `--ivk-dry-run`, `--ivk-watch`). Ambiguous commands are excluded from Cobra leaf registration (`cmd/invowk/cmd_discovery.go:70-72`) and routed through `cmdCmd.RunE` (`cmd/invowk/cmd.go:103-137`), where Cobra flag parsing is enabled.
- `internal/app/execute/orchestrator.go:323-328` projects `opts.Args` into `ARG1..ARGn` and `ARGC`; `internal/runtime/sh.go:476-479` prepends `"--"` before `ctx.PositionalArgs` for `mvdan/sh`; `internal/runtime/lua.go:211` passes `ctx.PositionalArgs`; `internal/runtime/native.go:410-411` ignores positional args on `cmd.exe` (`native.go:397`).

## Goals / Non-Goals

**Goals:**
- Allow wrapper commands (`passthrough: true`) to forward all post-command tokens verbatim (`--help`, `-h`, `-v`, `--json`, `--`, etc.) across POSIX `native`, `virtual-sh`, `virtual-lua`, and `container` runtimes.
- Keep pre-command `--ivk-*` control consistent across both direct leaves and `@source`/`--ivk-from` disambiguated paths.
- Keep `invowk help cmd <name>` available for Invowk-level command metadata and signal `[passthrough]` in `invowk cmd` listings.

**Non-Goals:**
- Allowing `passthrough: true` alongside declared `flags`, `args`, or subcommands (mutually exclusive by design so Invowk's declarative typed validation model stays unambiguous).
- Supporting inline positional forwarding on Windows `cmd.exe` (document PowerShell/`pwsh` or `virtual-sh` for Windows passthrough commands, matching `native.go:397,410-411`).

## Decisions

### D1. Schema & command-tree mutual exclusivity
- Add `passthrough?: bool` to `#Command` (`pkg/invowkfile/invowkfile_schema.cue:537`) and `Passthrough bool` (`json:"passthrough,omitempty"`) to `invowkfile.Command` (`pkg/invowkfile/command.go:69`).
- Enforce mutual exclusivity in CUE (using variants or explicit CUE constraints so `passthrough: true` rejects non-empty `flags` or `args`, plus Go validation in `Command.Validate()`) and in `ValidateCommandTree` (`pkg/invowkfile/command_tree.go:77`) so a `passthrough: true` command `X` cannot have child subcommands `X Y`—either in the same source or across sources, matching `args` + subcommand rules.

### D2. Hardened CLI boundary: pre-command `--ivk-*` vs post-command passthrough
- **Leaf registration (`cmd/invowk/cmd_discovery.go:185`)**: when `cmdInfo.Command.Passthrough` is true, set `DisableFlagParsing: true` on the Cobra leaf so Cobra delivers every token after the leaf path verbatim to `RunE`'s `args` (including `--help`, `-h`, `--ivk-*` intended for a nested invowk, and `--` if present).
- **Leaf-only `--ivk-*` flags (`cmd/invowk/cmd_discovery.go:236-241`)**: promote `--ivk-workdir`, `--ivk-env-var`, `--ivk-env-file`, `--ivk-env-inherit-mode`, `--ivk-env-inherit-allow`, and `--ivk-env-inherit-deny` to `cmdCmd.PersistentFlags()` (`cmd/invowk/cmd.go:141-147`), ensuring `invowk cmd --ivk-workdir /tmp --ivk-env-var FOO=1 perf --help` parses all `--ivk-*` flags before reaching the passthrough leaf while still supporting normal non-passthrough leaves via persistent inheritance.
- **Module-qualified `<source> <cmd>`, `@source`, `--ivk-from`, and ambiguous group nodes**: module commands register in Cobra under `SimpleName` (`cmd_discovery.go:74`), NOT `Name` (`cst doctor`), so invocations like `invowk cmd cst doctor --help --json` (including G7 wrapper `exec invowk -c <cfg> --ivk-no-cwd cmd cst "$@"`) or ambiguous group-node calls like `invowk cmd --ivk-from cst cst perf --help` fall into `cmdCmd.RunE` or `buildDiscoveredGroupCommand` (`cmd_discovery.go:115-160`), where Cobra flag parsing is active. Therefore, a pre-parse passthrough detector (before Cobra parses `cmd` flags, without mutating `os.Args` which `watch_resume` and `classifyUnknownRootCommand` at `cmd/invowk/root.go:109` rely on) must:
  1. Skip all value-taking root flags (`-c`, `--ivk-color` from G1) and promoted `cmdCmd` persistent flags (`-r`, `-f`, `-w`, `-e`, `-E`, `--ivk-container-name`, `--ivk-env-inherit-*`).
  2. Resolve the longest-matching command target across direct `SimpleName`, explicit `@source` / `--ivk-from`, AND source-qualified `<source> <simpleName>` (`cmdInfo.Name`).
  3. When that resolved target has `Passthrough == true`, split argv at the command path boundary so Cobra parses only the pre-command Invowk flags and forwards every post-command token verbatim (never letting `cmdCmd` or a synthesized group node intercept `--help` or unknown flags).
- **Note on `--ivk-watch`**: verify how `--ivk-watch` interacts with passthrough `args` so watch re-executions pass the same verbatim post-command tokens.

### D3. Inspection, listing, and documentation safeguards
- Because `invowk cmd <passthrough-cmd> --help` executes the command (triggering `depends_on` and container provisioning if configured!), document that:
  - `invowk help cmd <name>` remains Invowk's non-executing metadata/help view (`DisableFlagParsing` on a leaf does not disable Cobra's `invowk help ...` subcommand routing).
  - `invowk cmd` listing (`cmd/invowk/cmd_discovery.go:506-528`) marks passthrough commands clearly (e.g., `[passthrough]`).
  - Docs warn authors to use `passthrough: true` only for targets/scripts that handle `--help`/`-h` safely themselves.

## Risks / Trade-offs

- **Risk**: `invowk cmd <passthrough> --help` executes `depends_on` and container runtime instead of printing Invowk help.
  -> **Mitigation**: Highlighted in `invowk cmd` listing (`[passthrough]`), `invowk help cmd <name>`, and schema documentation; `--ivk-dry-run` before the command name (`invowk cmd --ivk-dry-run <name> --foo`) inspects without executing.
- **Risk**: Short-flag collisions (`-w`, `-e`, `-E`) when promoting leaf `--ivk-*` flags to `cmdCmd.PersistentFlags()`.
  -> **Mitigation**: Audit every existing flag under `invowk cmd` (none exist on `cmdCmd` today except `-r`, `-f`, `-W`; verify root flags `-v`, `-c`, `-i` do not collide — `-w`, `-e`, `-E` have zero collisions with root or `cmdCmd`!).

## Maintainer decisions

- **2026-11-27**: Confirmed opt-in `passthrough?: bool` mutually exclusive with `flags`, `args`, and subcommands; promoting `--ivk-*` flags so everything before the command name is Invowk's and everything after is forwarded verbatim; and ensuring `@source`/`--ivk-from` passthrough invocations work identically to direct leaves.

## Open questions

- Confirm whether `--` immediately after a passthrough command name (`invowk cmd perf -- --help`) should be passed through verbatim (consistent with `DisableFlagParsing: true` and `cobra` behavior on `run`/`ssh`-style wrappers) or stripped. Recommendation: forward verbatim with `DisableFlagParsing: true` (standard wrapper semantics; e.g., `kubectl exec pod -- cmd`), and verify `ARG1`/`ARGC` match `$1`/`$#`.
