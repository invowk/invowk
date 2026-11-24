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
| N11 | EVOLVE — bug | `fix-tui-capture-output` (S5a, Part A, shipped #175); no-server `-i` runs not proposed (spike failed) | M |
| N12 | EVOLVE — bug | container auto-provision on non-Linux hosts (not S5; scheduled: group D) | S |
| N13 | EVOLVE — tests | portable no-terminal test helper (`notty`, before v0.17.0) | S |
| N14 | EVOLVE — bug | `tui spin` Ctrl+C race and dropped output (after v0.17.0; with N1/S2; scheduled: group C) | S |
| N15 | DECIDE — product | `pager` and non-selectable `table` with stdout redirected (after v0.17.0; scheduled: group C) | S |
| N16 | EVOLVE — security | F18: Windows junction admitted by the virtual-runtime path policy (scheduled with F14: group B2) | M |
| N17 | EVOLVE — bug | terminal-emulating `-i` viewer, needed for no-server `-i` runs (N11 spike; not yet proposed) | M |
| N18 | EVOLVE — bug | `modulesync` Sync rewrites an unchanged lock with a new `generated:` timestamp (the `TestSyncFreshCacheReproducesLockedHash` "flake"; FIXED by PR #186, 768fa8c1) | S |
| N19 | EVOLVE — CI | `ci.yml`/`release.yml` test-leg parity check (fixed: `scripts/check-ci-release-parity.py`, PR `chore/ci-parity-and-template-refresh`) | S |
| N20 | EVOLVE — tests | formal-bindings plan-inputs digest leaves out the killer data files (PR #184 follow-up; not yet proposed) | S |
| N21 | EVOLVE — tests | insertion-only mutant after a blank line has an empty anchor `context` (PR #184 follow-up; not yet proposed) | S |
| N22 | EVOLVE — bug | `module sync/update/add/remove` print "Lock file updated" when `LockFile.Save` skipped the write (#186 follow-up; scheduled before the next tag) | S |
| N23 | DECIDE — product | lock-format questions: UTC `generated:`, and unknown CUE fields a no-op save no longer normalises (#186 follow-up; deferred) | S |
| N24 | EVOLVE — security | native and container runtimes re-open `SelectedScriptFilePath()` after the contained read (TOCTOU residual; outside B1; not yet proposed) | S |
| N25 | EVOLVE — CI | Dependabot reactivated 2026-11-19 (security updates enabled, version updates running again); merge gate added in `decide-only` mode (`ci/dependabot-gate`) | S |
| N26 | DONE — supply chain | website npm residuals `tinypool`, `serialize-javascript`, `uuid`: fixed by exact `overrides` (#205, 2026-11-24); remove them when the parents allow patched ranges | S |
| N27 | EVOLVE — deps | `github.com/coder/acp-go-sdk` 0.20 migration (Dependabot PR #190 broke `internal/acpclient`; not yet proposed) | S–M |
| N28 | EVOLVE — CI | Dependabot gate flip to `merge` mode, after it has judged real pull requests correctly (pending) | S |

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
  S5a merged as #175 (`d21e56f9`). S5b (no-server `-i` runs) is not proposed: a spike on
  2026-11-19 failed it, so the refusal stays in v0.17.0 (option (a)). Spike: S5a binary,
  native `invowk cmd pick -i` under tmux (100x30), script unsets `INVOWK_TUI_ADDR` and
  `INVOWK_TUI_TOKEN` and runs `invowk tui choose --title "Pick one" alpha beta gamma`,
  then `invowk tui input --title "Your name"`, so the child draws directly through the
  `-i` viewer. `choose` renders and Down moves the cursor, but Enter does not submit it
  (only Ctrl+J does). In `input`, typing `Ana` overdraws the help line
  (`An er submit • esc cancel`) and neither Enter nor Ctrl+J submits. Control (the same
  harness without the `unset`, so the TUI server renders): both prompts submit on Enter
  and print `CHOSEN=[beta]` and `NAME=[Ana]`. A raw reader under `-i` receives Enter as
  `\r` (and `\n` in cooked mode), exactly as without `-i`, so key forwarding is
  correct; the gap is the viewer's terminal emulation for a Bubble Tea child. No-server
  `-i` runs need a terminal-emulating viewer first, a separate change after v0.17.0.
  Earlier S5b drafts: 9d8483ca and 97d1e6a5.
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
  with no console on Windows. Then the txtar runs on all three systems. Fixed by the
  `notty` and `tuiaction` helpers in `tests/cli/notty_test.go`, installed through
  `testscript.Main`; calibrated 2026-11-19: without the new session, under a real tty,
  the test hangs on `/dev/tty` (30 s timeout); with it, it passes. That calibration ran on
  Linux only: the txtar passes on the Windows and macOS CI legs, but nobody has seen it
  fail there with the detach removed, so its failure mode on those systems is uncalibrated.
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
- **N16 — F18: on Windows the virtual-runtime path policy admits a path through a junction
  inside an allowed root, even at depth 0.** Found 2026-11-19 by the Windows probe, run
  44523520503, job 135557989975. Through a junction, `filepath.EvalSymlinks` returns
  `ErrNotExist`, and the policy admits the path although the junction leads outside the
  root; `os.Root` rejects the same path. Junctions are not `ModeSymlink` on Windows (as in
  F16). Planned with F14 (deepest-existing-ancestor evaluation) in group B2.
- **N17 — terminal-emulating `-i` viewer (N11 follow-up).** No-server `-i` runs (S5b)
  need a viewer that emulates a terminal for a Bubble Tea child; the N11 spike shows Enter
  not submitting `choose` and `input` overdrawing its help line. Diagnosis lead: the
  components emit `CSI > 4;2 m` (xterm modifyOtherKeys, level 2) and `CSI > 1 u` (kitty
  keyboard protocol, flag 1), seen in a v0.16.0 capture. Unverified: the child may then
  expect key encodings the viewer never produces. Separate change after v0.17.0.
- **N18 — `modulesync` Sync rewrites an unchanged lock file (product bug, was a flake;
  FIXED by PR #186, 768fa8c1).** See "Flaky tests" below:
  `TestSyncFreshCacheReproducesLockedHash` failed deterministically when more than 1 s
  passed between its two syncs, because Sync wrote the lock again with a new `generated:`
  timestamp although nothing else changed. `LockFile.Save` (`pkg/invowkmod/lockfile.go`)
  now skips the write when the valid lock on disk has equal content, so the file stays
  byte-identical. Follow-ups: N22, N23.
- **N19 — `release.yml` test legs drift from `ci.yml` (fixed).** The first v0.17.0 tag run
  failed because #173 added `tui-plan-row` and `INVOWK_TEST_TUI_PLAN_ROW` to `ci.yml` only
  (#183 fixed `release.yml`; `INVOWK_TEST_REQUIRE_TMUX` was missing there too).
  `scripts/check-ci-release-parity.py` now compares the `test` jobs' matrix legs and the
  test steps' `INVOWK_TEST_*` env, with an explicit exception table; its calibration test
  (`scripts/test_check_ci_release_parity.py`, fixtures under
  `scripts/testdata/ci-release-parity/`) fails on `release.yml` at 567e7b00, and it runs in
  `make test-scripts` on every PR.
- **N20 — formal-bindings evidence ignores the killer data files (PR #184 follow-up).** The
  plan-inputs digest covers the target files and the files declaring the killer tests, but
  not the data files those tests read (golden vectors, helpers). A golden change therefore
  does not stale the baseline evidence or the refresh's run match; today only the skill's
  "rerun after `make formal-golden`" note guards it. Fix: fold the killer tests' data
  files into the digest.
- **N21 — insertion-only mutant after a blank line has an empty anchor (PR #184
  follow-up).** An insertion-only survivor is anchored by `context`, the unchanged line
  before its first added line; when that line is blank the context is empty and the
  refresh cannot anchor it, so the entry must be anchored by hand. Fix: use the nearest
  non-blank line before the insertion (or several context lines).
- **N22 — `module sync/update/add/remove` print "Lock file updated" even when
  `LockFile.Save` skipped the write (#186 follow-up; scheduled before the next tag).**
  Since #186, `Save` writes nothing when the valid lock on disk has equal content, but
  `Save` returns only an error, so the CLI cannot tell a skip from a write and always
  reports an update. Sites: `cmd/invowk/module_deps.go` :198 (`runModuleAdd`), :237-239
  (`runModuleRemove`), :284 (`runModuleSync`) and :364 (`runModuleUpdate`); the four
  `lock.Save` calls in `internal/app/modulesync/resolver.go` (`Add`, `Remove`, `Update`,
  `Sync`). Fix: `Save` reports whether it wrote, the resolver passes that through in its
  results, and the CLI prints "unchanged" on a skip.
- **N23 — lock-format questions (#186 follow-up; deferred).** Two questions the skipped
  write raises, not yet decided: (a) whether `generated:` should be written in UTC, so the
  stamp does not carry the writer's zone offset; (b) the open decode keeps unknown CUE
  fields in the lock file, and a no-op save no longer rewrites (and so no longer
  normalises) such a file. Decide whether unknown fields should count as a content
  difference, be rejected, or stay as they are.
- **N24 — the native and container runtimes re-open `SelectedScriptFilePath()` after the
  contained read (TOCTOU residual; outside B1's scope).** The script content is read
  through the containment check, but for interpreter scripts `internal/runtime/native.go`
  (:240, :454) passes the path itself to the interpreter and
  `internal/runtime/container_provision.go` (:318) maps it into `/workspace`, so the
  interpreter re-opens the file: one swapped between the read and the re-open runs
  unchecked content. Residual risk; fix by running from the already-read content (a temp
  file, as for inline scripts) or an open handle.
- **N25 — Dependabot version updates have not run since 2026-06-23.** `.github/dependabot.yml`
  has not changed since 2026-01-16. The last Dependabot PRs merged were #68 and #89
  (2026-03-17); every one opened after that was closed unmerged, the last (#124, #125)
  opened 2026-06-22 and closed 2026-06-23. This looks like GitHub's pause of version
  updates on inactivity (inferred; the pause state of version updates is not readable
  through the API). Dependabot security updates are disabled:
  `GET /repos/invowk/invowk/automated-security-fixes` returns
  `{"enabled":false,"paused":true}` (2026-11-19). Unpausing (and enabling security updates) is a repository setting for the
  maintainer; agents do not change GitHub settings.

  **Reactivated 2026-11-19.** The maintainer enabled security updates, and version updates
  run again: on 2026-11-23 Dependabot opened #189 (npm), #190 (Go, a group of 10) and #191
  (GitHub Actions). The old configuration put every update of an ecosystem in one group, so
  one major (`typescript` 6 -> 7 in #189) or one broken dependency (`acp-go-sdk` in #190,
  N27) held the whole pull request. PR `ci/dependabot-gate` changes that:
  - `.github/dependabot.yml`: gomod and npm group only patch and minor updates. Majors, the
    sync-pair dependencies and the two fast-moving SDKs (`github.com/coder/acp-go-sdk`,
    `github.com/openai/openai-go/v3`) arrive as their own pull requests.
  - `.github/workflows/dependabot-gate.yml` with `scripts/dependabot-gate.py`: a merge gate
    that runs main's copy on `workflow_run` and decides `merge` or `hold` (rules:
    `.agents/rules/ci-workflows.md`, "Dependabot merge gate"). It ships in `decide-only`
    mode, which writes the verdict to the job summary and merges nothing. Its verdicts on
    the three open pull requests: all `hold` (#189 the `typescript` major, #190 failing
    checks and the `acp-go-sdk` 0.x minor, #191 a GitHub Actions update).
  - Merges will use the dedicated `invowk-dependabot-merge` GitHub App (no ruleset bypass;
    `DEPENDABOT_MERGE_APP_ID` variable and `DEPENDABOT_MERGE_APP_PRIVATE_KEY` secret,
    created by the maintainer). The flip to `merge` mode is N28.
- **N26 — website npm residuals with no fix inside Docusaurus 3 (2026-11-19 audit).**
  After `npm audit fix` (PR `chore/deps-security-refresh`, run twice: the second run
  bumped `express` 4.22.2 to 4.22.3, whose `qs ~6.16.0` cleared the top-level `qs` 6.15.4
  moderates GHSA-4mjr-xmp4-gh2g and GHSA-6pq8-jhm2-2vcw and removed the nested
  `body-parser` copy), `npm audit` reports 22 vulnerable packages (15 critical, 1 high,
  6 moderate) that all come from three, each pinned under `@docusaurus/core` 3.10.2, the
  latest 3.x release:
  - `tinypool` 1.1.1 (critical, GHSA-5gmw-xhrv-c9v3 and GHSA-85c8-ppgw-ccpr, fixed in
    2.1.2; `@docusaurus/core` requires `^1.0.2`);
  - `serialize-javascript` 6.0.2 (high, GHSA-5c6j-r48x-rmvq, GHSA-qj8w-gfj5-8c6v and
    GHSA-m5mm-p8h7-m29v, fixed in 7.1.3; `copy-webpack-plugin` 11.0.0 requires `^6.0.0`
    and `css-minimizer-webpack-plugin` 5.0.1 `^6.0.1`, both under `@docusaurus/bundler`);
  - `uuid` 8.3.2 (moderate, GHSA-w5hq-g745-h8pq, fixed in 11.1.1), through `sockjs`
    0.3.24 under `webpack-dev-server`, which `@docusaurus/core` requires.

  None of the fixes is semver-compatible with its parent's range; npm's only offer is
  `--force`, which would install `@docusaurus/core` 3.7.0.
  All three are build-time or dev-server only (the site is static). The other 19 flagged
  packages (the 14 critical Docusaurus packages, `@docusaurus/bundler`, the two webpack
  plugins, `sockjs`, `webpack-dev-server`) are flagged only through these three. Fix: the
  Docusaurus release that moves off them (4.x), or a later 3.x.

  **Resolved 2026-11-24 (#205), reversing the "no overrides" note above.** `website/package.json`
  `overrides` pins `tinypool` 2.1.2, `serialize-javascript` 7.1.3 and `uuid` 11.1.1, which clears
  Dependabot alerts #17, #47, #48, #125, #126 and #152 (`npm audit`: 22 to 0). Each crosses its
  parent's major, so the call sites were checked at the installed versions: `@docusaurus/core`
  loads `tinypool` 2 through `import()` with options v2 still supports (the SSG pool runs in the
  build); `sockjs` calls `require('uuid').v4`, which uuid 11's CommonJS path keeps (12 drops it);
  the two webpack plugins call `serialize()` with the same API, and a round trip of
  css-minimizer's serialized options through `new Function` returned the functions intact. With
  `future.v4` on, Rspack and the `ws` dev server are used, so only the `tinypool` override is on
  a live path today; the other two are a fallback if those flags are turned off.
  - **Do not remove the overrides** while a parent still requires the vulnerable range.
  - **Remove each one** when its parent allows a patched release (`@docusaurus/core` for
    `tinypool`, `sockjs` or `webpack-dev-server` for `uuid`, `copy-webpack-plugin` and
    `css-minimizer-webpack-plugin` for `serialize-javascript`, or Docusaurus 4), then re-run
    `npm audit`. The exact pins freeze these packages until then (`tinypool` 2.2.0 and
    `serialize-javascript` 7.1.4 exist).
  - A Dependabot update of one of these packages has to edit `overrides`, so the Dependabot gate
    holds it for a human: expected.
- **N27 — `github.com/coder/acp-go-sdk` 0.20 migration.** Dependabot PR #190 (2026-11-23)
  moved the SDK from 0.13.5 to 0.20.0 inside the Go group; Dependabot labels the step
  semver-minor, but the 0.x API changed and `internal/acpclient` no longer builds (the four
  CI test legs, `govulncheck`, golangci-lint and the goplint consumer gates fail on #190).
  The gate treats a minor bump to a 0.x version as breaking, and `.github/dependabot.yml`
  now keeps the SDK out of the Go group. Pending: migrate `internal/acpclient` to the 0.20
  API in its own change; the other nine bumps of #190 do not depend on it.
- **N28 — Dependabot gate flip to `merge` mode.** The gate (N25) runs in `decide-only`
  mode. Flip it once it has judged real Dependabot pull requests correctly: compare the
  verdicts in the `Dependabot Gate` job summaries with what a maintainer would have
  decided, across at least one patch or minor pull request that should merge and the holds
  already seen. The flip is one reviewed PR that changes `GATE_MODE` in
  `.github/workflows/dependabot-gate.yml` to `merge` (and the pinned value in
  `scripts/test_dependabot_gate.py`); it is never a repository variable.
  Known before the flip (security review of the gate, 2026-11-24):
  - The gate holds a head that is behind `main` and looks again right before it merges, but
    the ruleset does not require up-to-date branches, so two merges a second apart are not
    excluded by GitHub itself. `strict_required_status_checks_policy: true` on the `main`
    ruleset would close that; it is a maintainer setting.
  - Socket and SonarCloud report on their own schedule and start no workflow. When one of
    them reports after the last workflow of a pull request has completed, nothing re-runs
    the gate; re-running any job of that pull request does.
  - `create-github-app-token@v3` deprecates `app-id` for `client-id`. The workflow keeps
    `app-id` (it still works); moving needs a new repository variable with the App's client
    id.
  - Grouped Go updates will mostly hold: their `go.mod` side effects usually include a 0.x
    minor (all of `golang.org/x/*`), a pseudo-version or a new indirect module (#190 has
    nine such changes besides `acp-go-sdk`).

## Why CST keeps its production paths out of invowk anyway

Even with all of the above fixed, CST keeps systemd `ExecStart=` targets, root-owned
copies in `/usr/local/sbin`, initramfs/degraded-boot files, GameMode hooks, the polkit
rule and user-session daemons as plain executables: those paths must not depend on an
extra runtime that can change underneath them. The realistic adoption path is invowk for
dev/CI (works today) and, after S1, C1–C4, an optional user-facing layer that forwards to
the existing programs.

## Flaky tests

Recorded, not fixed. Each passed on a rerun with no code change. One turned out to be a
product bug (`TestSyncFreshCacheReproducesLockedHash`, N18).

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
its hash; once, on Windows, it did not.

**Diagnosed 2026-11-19: a product bug, not a flake (N18).** With more than 1 s between
the two syncs the test fails deterministically: Sync rewrites the unchanged lock with a
new `generated:` timestamp. Every sighting in this section is a run whose two syncs
happened to straddle a second boundary. Fixed by PR #186 (768fa8c1).

Seen again 2026-11-15 on Linux (Fedora, local `make test` of S4 at `3670d1e5`): same
assertion in gotestsum's first pass, passed in its re-run. 200 isolated runs passed on S4 and on `main` (`d7e4feb4`). Hypothesis
at the time: the lock file carries an RFC 3339 `generated` timestamp, so a rewrite that
crosses a second boundary changes its bytes (since confirmed, see above).

Third sighting, 2026-11-19 on Windows: CI run 44475510192, job 135394971934 (PR #175 at
`e993c5af`, which does not touch `internal/app/modulesync`). Same assertion; the next head
passed. Diagnose after v0.17.0.
