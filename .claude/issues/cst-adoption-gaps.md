# Gaps found while evaluating invowk for CST (root/system tooling)

**Status: TRIAGED (2026-11-14) — tracker, not filing drafts**
**Source: evaluation of migrating CST (github.com/cominotti/cst) to invowk commands, 2026-11-13**
**Re-verified on: main `85f1eb7b` (v0.16.1-0.20261008185530-85f1eb7bc661), local build, isolated `HOME`**

## Context

CST is workstation setup tooling for a Fedora Silverblue host: interactive commands
(`cst btrfs plan|apply|status ...`), commands that re-exec themselves with `sudo`,
programs that systemd runs as root at boot and on timers, and user-session services.
It already uses invowk for its dev/CI layer (`cst.invowkmod`), and that works well.

The 2026-11-13 drafts were re-verified against main on 2026-11-14 and triaged item by
item. Several premises were wrong (Issues 4, 5, 7, 8); others were worse than described
(Issues 1, 3, 6). Four new gaps (N1–N4) came out of the review. Each item below records
the verdict, what was verified (with `file:line` on main `85f1eb7b`), the agreed
direction, and the change that owns it.

Decisions taken with the maintainer (2026-11-14):
- Remove the `./config.cue` fallback outright (no opt-in mode).
- Bundle the BREAKING changes (S1 config trust, C1 exit codes, and the later F12
  admission fix) into one release (target `v0.17.0`) with a single migration section.
- Built-in privilege elevation (`elevate: "sudo"`) is rejected.

## Summary

| # | Verdict | Owner change | Size |
|---|---|---|---|
| 1 | EVOLVE | `cli-exit-code-contract` (C1) | S/M |
| 2 | EVOLVE | `clean-noninteractive-output` (C2, not yet proposed) | M |
| 3a | EVOLVE — security | `harden-config-trust` (S1) | S–M |
| 3b | DEFER | revisit after the trust-sources model (F3a) and Issue 4 | M–L |
| 4 | RESHAPE | `command-contract-schema` (C3, not yet proposed) + docs (C4) | S–M |
| 5 | RESHAPE | C3 (`passthrough`) + docs now | S–M |
| 6 | EVOLVE | C3 (`invowk_version`, invalid-file handling) | S–M |
| 7 | RESHAPE | C4 (docs + `completion --alias`) | S |
| 8 | DISCARD | one docs sentence in C4 | — |
| N1 | EVOLVE | `forward-termination-signals` (S2, not yet proposed) | S |
| N2 | EVOLVE | C2 | S |
| N3 | EVOLVE | C3 | S |
| N4 | EVOLVE | C2 | S |
| N5 | EVOLVE — security | `restrict-tui-server-bind` (S3, not yet proposed) | M |

Merge order: S1 → S2 → C2 → C1 → C3 → C4 (interleaved with the formal track).
S2, C1 and C2 all touch `cmd/invowk/root.go`, so they land in sequence.

---

## Issue 1: Usage errors exit 0 or 1 instead of 2 — EVOLVE (C1)

Verified (re-run 2026-11-14):

| Invocation | Exit |
|---|---|
| `invowk cmd test nonexistent` (typo under a group) | **0**, prints group help |
| `invowk cmd test unit --bogus` | 1 |
| `invowk --bogus`, missing/extra declared args, unknown command, ambiguity | 1 |
| `invowk cmd w typo` where `w` is a leaf that also has child `w sub` | **0**, runs `w` with `typo` as `$1` |

- Group nodes return `cmd.Help()` and ignore `args` (`cmd/invowk/cmd_discovery.go:83-111`).
- `Execute` maps every non-`ExitError` to 1 (`cmd/invowk/root.go:95-105`).
- Leaves use `cobra.ArbitraryArgs` (`cmd_discovery.go:178`), which is why `w typo` runs `w`.
- A script's own exit code is already forwarded (`exit 2` → rc 2), so 2 is not exclusive.

Direction: a CLI-layer `UsageError` mapped to 2 in `Execute`. Sources: root
`SetFlagErrorFunc`, group nodes given unmatched args (help + `SuggestionsFor` on stderr),
`ArgumentValidationError`, unknown/ambiguous command. Document the contract
(0 ok; N the command's own code; 2 invowk rejected the command line; 1 invowk internal
failure) and that 2 can collide with a script's own code. BREAKING (scripts testing `== 1`).

Open design point: the leaf-with-children rule (`w typo`). Undeclared positionals are
today the only way to pass args to such a leaf (declared args + children are rejected,
`pkg/invowkfile/command_tree.go:89`), so making `w typo` an error needs the `passthrough`
opt-out from C3. The C1 spec decides whether that rule lands in C1 or moves to C3.

## Issue 2: ANSI on non-TTY output; `NO_COLOR` / `TERM=dumb` ignored — EVOLVE (C2)

Verified: `NO_COLOR=1 TERM=dumb invowk cmd nope 2>&1 | cat -v` emits 16 escape
sequences. Causes: Glamour with a fixed `"dark"` style (`cmd/invowk/service_error.go:61`,
`cmd_discovery.go:365,375`, `config.go:161` via `issue_render.go:39`); lipgloss strings
written with `fmt.Fprint` straight to `app.stderr` (`app.go:111-115,320`,
`cmd_render.go:45,66`) without downsampling. Flag errors are clean because fang renders them.

Direction: wrap `App` stdout/stderr in `colorprofile.NewWriter(w, os.Environ())`
(`github.com/charmbracelet/colorprofile` is already indirect, go.mod:89); Glamour `"notty"`
off-TTY; `--ivk-color=auto|always|never` with precedence flag > `NO_COLOR`/`CLICOLOR_FORCE`
> config > auto. Decide separately for `invowk tui style/format`, which exist to emit styling.

## Issue 3: Config and discovery depend on `$HOME` and the cwd — split

### 3a — EVOLVE, security (S1 `harden-config-trust`)

Verified:
- When no user config exists, `./config.cue` in the cwd is loaded
  (`internal/config/config.go:196-211`; documented in
  `website/docs/configuration/overview.mdx:21`). That is the normal state for root and
  for many users. The file controls `includes` (load modules from anywhere),
  `llm.api.base_url` + `api_key_env` (`internal/config/config_schema.cue:158-170` — send an
  API key from the environment to a repo-chosen host), `container.auto_provision.binary_path`
  (`:86`) and `virtual`. **`cd` into a cloned repository is enough to inject config.**
- The draft's "no env var" claim is half wrong: `XDG_CONFIG_HOME` is honoured
  (`config.go:109`). What is missing is a variable naming the config file. Only
  `~/.invowk/cmds` depends on `$HOME` directly (`config.go:132-138`).
- The draft's "cwd shadows commands" claim is wrong: duplicates fail closed
  ("Ambiguous command", "Module name collision", rc 1). The cwd can still add commands.

Direction (agreed): remove the fallback; add `INVOWK_CONFIG` (precedence `--ivk-config` >
`INVOWK_CONFIG` > user config > defaults); add `--ivk-no-cwd` / `INVOWK_NO_CWD=1` to
disable cwd invowkfile/module discovery. BREAKING, release-bundled.

### 3b — DEFER

`/etc/invowk/config.cue` and owner/permission checks under `euid==0`. If ever done, warn
by default (CI containers run as root over uid-1001 workspaces) with an opt-in strict
mode in the style of sshd `StrictModes`. Revisit after the formal trust-sources model
(F3a) and Issue 4.

## Issue 4: No privilege model — RESHAPE (C3 + C4)

Verified: no `Geteuid`/sudo handling in production code. But "a command cannot say it
needs root" is false in practice: `depends_on.custom_checks` with
`test "$(id -u)" = 0` fails before the script runs ("Dependencies not satisfied", rc 1;
schema `#CustomCheck`, `pkg/invowkfile/invowkfile_schema.cue:301-317`). What is missing
is a declarative, portable form, help output, and a clear message. The native runtime
picks the shell from `$SHELL` (`internal/runtime/native.go`), which also changes under
sudo/systemd.

Direction: capabilities `"elevated"` (Unix `euid==0`, Windows elevated token) and
`"unprivileged"` in `#CapabilityName` (`invowkfile_schema.cue:367`), via the existing
`deps` path (precedent: `"tty"`), shown in help and `--ivk-dry-run`; `invowk audit` flags
`elevated` commands whose files are writable by others. Document the supported pattern:
root-installed `/usr/local/bin/invowk`, `sudo /usr/local/bin/invowk -c /etc/x/invowk.cue
--ivk-no-cwd cmd ...`. Optional `INVOWK_INVOKING_USER` from `SUDO_USER`, informational only.

**Rejected:** built-in elevation (`elevate: "sudo"`). invowk usually lives in a
user-writable path; re-executing a user-writable binary/module as root turns any user
compromise into root, implicitly.

## Issue 5: No raw argument passthrough — RESHAPE (C3)

Verified: undeclared positionals are already forwarded (`cmd x a b` → `argc=2`), and
`--` forwards flags intact including `--ivk-*` (leaves use `cobra.ArbitraryArgs`). The
real gaps: an unknown flag before `--` exits 1, `--help` is captured by invowk, and the
forwarding is undocumented (`website/docs/flags-and-arguments/positional-arguments.mdx`).

Direction: `passthrough: true` on a command (leaf `DisableFlagParsing`; `--help` goes to
the script; schema rejects it with `args`, `flags`, or children); document `--` forwarding now.

## Issue 6: No minimum invowk version per module — EVOLVE (C3)

Verified: `#Invowkmod` is `close({...})` (`pkg/invowkmod/invowkmod_schema.cue:41`) and
`ParseInvowkmodBytes` validates and decodes in one pass (`pkg/invowkmod/parse.go:36-45`),
so an old binary rejects the new field itself. Worse than drafted: an invowkfile with an
invalid field is skipped with a warning and the user sees "command not found" (see N3).
`SemVerConstraint`/`Constraint.Matches` exist (`pkg/invowkmod/semver.go:129,155`).

Direction: `invowk_version: ">=0.17.0"` in `invowkmod.cue` and at the root of
`invowkfile.cue`, read by a schema-less pre-pass before `ParseAndDecode`, with a
dedicated error and exit code. Dev builds skip with a verbose warning; pseudo-versions
compare by base version. Immediate win regardless of the field: on CUE
`field not allowed`, hint "this file may need a newer invowk". Only effective going forward.

## Issue 7: Expose a module as its own command (`cst ...`) — RESHAPE (C4)

Verified: `invowk mod install` does not exist (`module` has list/archive/import/vendor/
tidy/create/add/remove/sync/update/deps; `import` installs into `~/.invowk/cmds`,
`cmd/invowk/module_package.go:50-67`).

Direction: no `argv[0]` multi-call (a symlink resolved by discovery inherits the cwd/$HOME
fragility). Document the wrapper `exec /usr/local/bin/invowk -c <cfg> --ivk-no-cwd cmd cst "$@"`
and add `invowk completion <shell> --alias cst --prefix "cmd cst"`. Depends on 1, 2, 3a, 5.

## Issue 8: Inline scripts go to the default temp dir — DISCARD

Verified wrong: `os.CreateTemp("", …)` (`internal/runtime/native.go:276`) honours
`$TMPDIR`; the file is passed to the interpreter as an argument, so `noexec` is
irrelevant; it is created O_EXCL 0600 then 0700, and `PrivateTmp=` keeps `/tmp` writable.
`XDG_RUNTIME_DIR` would be wrong for system units. Action: one sentence in the unit
guide ("set `TMPDIR` if needed").

---

## New gaps from the review

- **N1 — SIGTERM is not handled (S2).** Only `os.Interrupt` is notified
  (`cmd/invowk/root.go:99`). On `systemctl stop`/`kill` invowk dies without cancelling the
  context: no temp-script cleanup, no container cleanup, no signal forwarded to the child
  (which is orphaned outside the cgroup). Notify `SIGTERM`/`SIGHUP` on Unix and forward.
- **N2 — fang's `ERROR / Exit status N.` banner (C2).** Appended to stderr when the user's
  own command fails; the App writer is not an `*os.File`, so fang's non-TTY detection
  (`fang@v1.0.0/help.go:112-118`) never triggers. Noise for wrappers and the journal.
- **N3 — an invalid invowkfile is skipped with a warning (C3).** The user sees "command
  not found". Decide whether an invalid cwd invowkfile is fatal.
- **N4 — `ui.color_scheme` is a dead setting (C2).** `config_schema.cue:116`; only read and
  written by `config show/set` (`cmd/invowk/config.go:32,149,330`).
- **N5 — the TUI server binds every interface (S3, not yet proposed).** Found 2026-11-14
  while triaging Sonar go:S5332. `internal/tuiserver/server.go:108` listens on
  `0.0.0.0:0`, so during interactive commands the port is reachable from the LAN, guarded
  only by the per-session token. Containers reach it through `host.docker.internal`, which
  is probably why it binds wide. Restrict the bind to loopback plus the engine's bridge
  gateway, covering Docker and Podman, rootful and rootless, and Flatpak, with container
  tests. Until then, Sonar S5332 there is resolved in SonarCloud as a false positive
  (recorded as e13): plain HTTP over loopback or the bridge is by design.

## Why CST keeps its production paths out of invowk anyway

Even with all of the above fixed, CST keeps systemd `ExecStart=` targets, root-owned
copies in `/usr/local/sbin`, initramfs/degraded-boot files, GameMode hooks, the polkit
rule and user-session daemons as plain executables: those paths must not depend on an
extra runtime that can change underneath them. The realistic adoption path is invowk for
dev/CI (works today) and, after S1, C1–C4, an optional user-facing layer that forwards to
the existing programs.
