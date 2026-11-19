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
| N5 | EVOLVE — security | `restrict-tui-server-bind` (S3, done: PR #173, merged as d7e4feb4) | M |
| N7 | EVOLVE — bug | SSH host-callback reachability (own change after S3, not yet proposed) | M |
| N8 | EVOLVE — security | follow-up to S4 (`basedir.TempDir`, not yet proposed) | M |
| N9 | DECIDE — product | follow-up to S4 (engine environment, not yet proposed) | M |
| N10 | EVOLVE — security | follow-up to S4 (per-URL git auth: auth order, Windows known_hosts default; not yet proposed) | M |
| N11 | EVOLVE — bug | `fix-tui-capture-output` (S5a, Part A); no-server `-i` runs gated in S5b | M |
| N12 | EVOLVE — bug | container auto-provision on non-Linux hosts (not yet proposed; not S5) | S |
| N13 | EVOLVE — tests | portable no-terminal test helper (before v0.17.0) | S |
| N14 | EVOLVE — bug | `tui spin` Ctrl+C race and dropped output (after v0.17.0; with N1/S2) | S |
| N15 | DECIDE — product | `pager` and non-selectable `table` with stdout redirected (after v0.17.0) | S |

Merge order: S1 → S2 → C2 → C1 → C3 → C4 (interleaved with the formal track).
S2, C1 and C2 all touch `cmd/invowk/root.go`, so they land in sequence.
v0.17.0 merge order: S1 → C1 → scripts-safety → F1 → S3 → S4, W2 after.

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
- **N5 — the TUI server binds every interface (S3, done: PR #173, merged as
  d7e4feb4).** Found 2026-11-14 while triaging Sonar go:S5332. `internal/tuiserver/server.go`
  listened on `0.0.0.0:0`, so during interactive commands the port was reachable from the
  LAN, guarded only by the per-session token. S3 (`openspec/changes/archive/2026-11-15-restrict-tui-server-bind`)
  binds loopback plus, at most, the one address a verified engine mode routes to the host,
  refuses `-i` runs in modes without a narrow route before start (exit 1, the error names the opt-in),
  adds the user-config opt-in `container.tui_server_bind: "all-interfaces"`, and requires
  the token on every request in constant time. Sonar S5332 there is resolved in SonarCloud
  as Accepted (real gap tracked as N5/S3; recorded as e13): plain HTTP over loopback or the
  bridge is by design.
  Docker Desktop (row 6) keeps its loopback-only plan (design decision 36). Verified
  2026-11-15 on native Windows only (`probe/row6-probe.ps1` on `win11-devbox`, Docker Desktop
  4.90.0, engine 29.7.2: loopback reachable, all-interfaces control reachable; design D12).
  **Not run**, and carried as release Warnings: the WSL2 variant (`probe/row6-probe.sh` in a
  WSL2 distribution; the VM has no user distribution, installing one is the maintainer's
  call), Docker Desktop on macOS and Linux, and one native Windows `invowk cmd <x> -i`
  container run (decision 27, ConPTY through `xpty`, supported by the code and unverified).
  invowk's own row-6 path (facts query, planner, adapter) has never run on a Docker Desktop
  host; only unit tests with constructed facts select row 6. Release-note wording for the
  row-6 Warning: on macOS and Windows hosts, in-container `invowk tui` also needs
  `container.auto_provision.binary_path` pointing at a Linux build of invowk (N12).
- **N6 — skipped-source diagnostics carry only a path (C1 follow-up).** C1 Decision 14
  exits 2 for a scoped miss in a source that loaded, but the CLI can only match a skipped
  source by its diagnostic path. Two cases can therefore wrongly exit 2: an `includes`
  alias for a module directory that the module scan also finds, and a provisioned module
  whose manifest keeps an alias or namespace (`ProvisionedModuleEntry.CommandNamespace`)
  and fails to load inside the container. Fix: have `internal/discovery` attach the
  intended source ID to load-failure diagnostics. Recorded in
  `openspec/changes/archive/2026-11-15-cli-exit-code-contract/design.md` (Decision 14).
- **N7 — the SSH host-callback server is unreachable from containers (separate change after
  S3).** Found 2026-11-14 while specifying S3. `internal/sshserver/server.go:124-127` and
  `internal/app/commandadapters/host_access.go:52` bind `127.0.0.1`, but containers dial
  `host.docker.internal`. One real run under rootful Docker 29.9.2 (`enable_host_ssh: true`,
  `debian:stable-slim`, bash `/dev/tcp` to `$INVOWK_SSH_HOST:$INVOWK_SSH_PORT`) got
  "Connection refused". The test is vacuous: `tests/cli/testdata/container_callback.txtar`
  only asserts the env vars. Fix by reusing S3's exposure plan (D1-D2) and add a real
  reachability test.
- **N8 — `TMPDIR`/`TMP`/`TEMP` are not validated (follow-up to S4).** Found 2026-11-14 in
  the `require-absolute-base-dirs` review (design decision 6). `os.TempDir()` returns a
  relative `TMPDIR` as given, and about ten sites use it (`os.TempDir`, `os.CreateTemp("",
  …)`, `os.MkdirTemp("", …)`, for example `internal/runtime/native.go:276`). On Windows
  the OS call derives it from `TMP`, `TEMP` and `USERPROFILE`. Add `basedir.TempDir()`
  with per-OS rules and route those sites through it; S4's temp fallbacks are safe
  without it (random `MkdirTemp` names, a non-truncating run lock).
- **N9 — Docker/Podman read their per-user config through a relative `HOME`/XDG
  environment (follow-up to S4, product decision).** Design decision 9 of S4: invowk
  passes the inherited environment to the engine CLIs, which read `~/.docker` and
  `$XDG_CONFIG_HOME/containers`. With a relative value they may read files under the
  current directory. Options: refuse to launch an engine, or rewrite its environment.
  Needs a maintainer decision.
- **N10 — git auth is chosen once, not per URL (follow-up to S4).** `setupAuth` in
  `internal/app/modulesync/git.go` runs in `NewGitFetcher`, before any URL is known, and
  tries SSH keys before HTTPS tokens: an `https://` fetch gets SSH public-key auth when
  `~/.ssh/id_*` exists, so `GITHUB_TOKEN`/`GITLAB_TOKEN`/`GIT_TOKEN` is ignored. Build the
  auth per URL from its scheme. The same per-URL construction closes a Windows gap found in
  S4's review: with `SSH_KNOWN_HOSTS` unset, go-git v5.19.2 (`getDefaultKnownHostsFiles`)
  also trusts `/etc/ssh/ssh_known_hosts`, which on Windows is root-relative and resolves on
  the current directory's drive. The fix passes go-git a host-key callback built from
  `<home>\.ssh\known_hosts` only, plus the host-key algorithms for that URL's host (go-git
  stops computing them when a callback is given), on both the key and the `Auth: nil`
  agent paths. Until then, set `SSH_KNOWN_HOSTS` to an absolute path on Windows.
- **N11 — direct `invowk tui` rendering draws into captured stdout (S5,
  `fix-tui-capture-output`).** Found 2026-11-15 in S3's premise check. Without
  `INVOWK_TUI_ADDR`, every directly rendered component builds `tea.NewProgram(model)`
  without `tea.WithOutput` (`internal/tui/input.go:137`; same shape in `choose`, `filter`,
  `file`, `write`, `table`, `confirm`, `pager`, `spin`), and Bubble Tea defaults to stdout
  (`charm.land/bubbletea/v2@v2.0.9/tea.go:624-627`). So `name=$(invowk tui input)` draws
  nothing and `$name` holds control sequences in front of the answer, with exit 0, on
  `main` in a plain terminal. `spin` has the same defect for `out=$(invowk tui spin -- …)`.
  S5a (`fix-tui-capture-output`) ships Part A only: components draw on the controlling
  terminal (`/dev/tty`, `CONIN$`/`CONOUT$` with a console check), probed 2026-11-15
  against stderr (stderr loses the UI under `2>log`). S5a leaves S3's refusal in force.
  S5b, a separate later change (not yet proposed), holds the gate: the no-server `-i`
  path and a hardened tmux test of a real no-server `-i` session on row 10, asserting
  the screen state. Its earlier drafts are at 9d8483ca and 97d1e6a5. If the gate fails,
  S5b is shelved and the refusal stays in v0.17.0.
- **N12 — on Windows and macOS hosts, container auto-provision copies the host's own
  non-Linux binary into the Linux container (by code reading; not run on those hosts).**
  `applyHostProvisionDefaults` defaults `InvowkBinaryPath` to `os.Executable()` with no
  OS or architecture check (`internal/runtime/container_provision.go:237-240`). The
  provisioner copies it as-is (`internal/provision/layer_provisioner.go:307-313`), and
  the generated Dockerfile installs it as `/invowk/bin/invowk` on `PATH`
  (`internal/provision/dockerfile.go:25-30`, `:43-45`). Nothing in `internal/provision/`
  or the container runtime checks `GOOS` (grep on d7e4feb4), and the container runtime is
  available on any host whose engine is (`internal/runtime/container.go:186-188`). The
  docs say only "uses the running invowk binary" (`website/docs/configuration/options.mdx:245`,
  `website/docs/reference/config-schema.mdx:352`). On a Windows (PE) or macOS (Mach-O)
  host, the container run itself still works, because invowk is not the entrypoint
  (`internal/runtime/container_exec.go:181` uses `invowk` only as `$0`). But every nested
  `invowk` call inside the container fails with an exec-format error, including
  `invowk tui *` under `-i` on Docker Desktop (row 6) and in-container `invowk cmd`
  dependencies, unless `container.auto_provision.binary_path` points at a Linux build.
  Fix direction: provision a Linux binary that matches the image architecture, or warn
  and skip the binary on a non-Linux host, and document `binary_path`. This also qualifies
  S3's row-6 release Warning (N5): on macOS and Windows hosts, in-container `invowk tui`
  already needs `binary_path`, whatever the TUI server's reachability.
- **N13 — `tui_no_terminal.txtar` skips on macOS and Windows (S5a follow-up, before
  v0.17.0).** The test drops the controlling terminal with `setsid`, guarded by
  `[!exec:setsid] skip`. That guard is not in `.agents/rules/testing.md`'s closed list of
  allowed platform skips, and `setsid` exists only on the Linux CI legs (skipped in jobs
  135403243980 and 135403243939). Fix: a testscript command implemented in Go that starts
  the binary without a controlling terminal: a new session on Unix, a detached process
  with no console on Windows. Then the txtar runs on all three systems.
- **N14 — `invowk tui spin`: Ctrl+C abandons the running command, and a spinner failure
  drops its output (S5a review; pre-existing on `main`).** `spinActionModel` returns
  `tea.Quit` on Ctrl+C, so `runActionSpinner` returns nil. `spinWithAction` then returns
  without waiting for the action goroutine, and `runTuiSpinWithRunner`
  (`cmd/invowk/tui_spin.go`) reads `runResult` while the goroutine may still write it.
  That is a data race: the run exits 0 with empty output and the child keeps running.
  Separately, when the spinner fails after the command started, the adapter returns the
  spinner error and drops the command's output and exit code. Fix: cancel the command's
  context on quit and exit 130; print the output and map the exit code whenever the
  action ran. Belongs with N1 (S2, termination signals).
- **N15 — `pager` and non-selectable `table` when only stdout is redirected (S5a review,
  product decision).** S5a's `pager` opens the terminal and writes nothing to stdout when
  only stdout is a pipe (unlike `less`, which passes content through), and a non-selectable
  `table` is a value component, so with no terminal it exits 1. v0.17.0 ships this
  reviewed behaviour. Decide whether either should pass content through to stdout
  whenever stdout is not a terminal.

## Why CST keeps its production paths out of invowk anyway

Even with all of the above fixed, CST keeps systemd `ExecStart=` targets, root-owned
copies in `/usr/local/sbin`, initramfs/degraded-boot files, GameMode hooks, the polkit
rule and user-session daemons as plain executables: those paths must not depend on an
extra runtime that can change underneath them. The realistic adoption path is invowk for
dev/CI (works today) and, after S1, C1–C4, an optional user-facing layer that forwards to
the existing programs.

## Flaky tests

Recorded, not fixed. Each passed on a rerun with no code change.

### `TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen` (`internal/sshserver`)

Found 2026-11-15 on PR #174. Lane "Models, golden vectors, and correspondence", step
"Replay golden vectors and property tests" of `formal-verification.yml`, run
[43670064960](https://github.com/invowk/invowk/actions/runs/43670064960), attempt 1, head
`dac3acf7`, ubuntu. Attempt 2 of the job passed. PR #174 changes no Go code.

```
INFO ssh-server: SSH server started address=127.0.0.1:41749
INFO ssh-server: SSH server started address=127.0.0.1:34379
INFO ssh-server: SSH server stopped
INFO ssh-server: SSH server stopped
--- FAIL: TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen (0.04s)
    token_formal_test.go:152: Stop() error = <nil>, want context.DeadlineExceeded while a connection is open
FAIL
FAIL	github.com/invowk/invowk/internal/sshserver	0.053s
```

`Stop()` returned nil although the test expects the open authenticated connection to hold
it until `context.DeadlineExceeded`. The cause is not investigated; the 0.04 s run suggests
the connection was not yet (or no longer) tracked when `Stop()` ran.

### `TestSyncFreshCacheReproducesLockedHash` (`internal/app/modulesync`)

Found 2026-11-15 on PR #163. Job "Test (windows)", step "Run tests (short)" of `ci.yml`,
run [43658445060](https://github.com/invowk/invowk/actions/runs/43658445060), attempt 1,
head `d6846a2b`, windows-latest. It failed in the first gotestsum pass and passed in
gotestsum's re-run in the same step (the job failed for other, real failures:
`TestModulePathContainment_F16Junction` and `TestModulePathContainment_Property`).

```
=== FAIL: internal/app/modulesync TestSyncFreshCacheReproducesLockedHash (0.26s)
    resolver_integrity_test.go:122: lock file changed although the locked version reproduced its content
...
 ✖ Sync fresh cache reproduces locked hash (0.26s)
 ✓ Sync fresh cache reproduces locked hash (0.09s)
```

The test expects the lock file to stay byte-identical when the locked version reproduces
its hash; once, on Windows, it did not. The cause is not investigated.

Seen again 2026-11-15 on Linux (Fedora, local `make test` of S4 at `3670d1e5`): same
assertion in gotestsum's first pass, passed in its re-run. 200 isolated runs passed on S4 and on `main` (`d7e4feb4`). Unverified
hypothesis: the lock file carries an RFC 3339 `generated` timestamp, so a rewrite that
crosses a second boundary changes its bytes.

Third sighting, 2026-11-19 on Windows: CI run 44475510192, job 135394971934 (PR #175 at
`e993c5af`, which does not touch `internal/app/modulesync`). Same assertion; the next head
passed. Diagnose after v0.17.0.
