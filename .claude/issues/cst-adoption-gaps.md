# Gaps blocking broader CST adoption of invowk

**Status: REFRESHED (2026-11-27) — open gaps only**
**Source: evaluation of CST (`github.com/cominotti/cst`) against `invowk` `v0.18.0` (`a4e3b2b8`) and `main` (`7a282c1b`)**

## Context

CST is workstation setup tooling for a Fedora Silverblue host: interactive commands (`cst btrfs plan|apply|status ...`), commands that re-exec themselves with `sudo`, programs that `systemd` runs as root at boot and on timers, user-session services, and a project-local dev/CI task suite in `cst.invowkmod/` (`workdir: ".."`).

`v0.17.0`–`v0.18.0` closed several major blockers for CST:

- **`C1` (#162):** usage errors now exit `2` instead of `0`/`1`, preventing silent typo passes in CI and scripts.
- **`S1` (#161) & `S4` (#172):** removed `./config.cue` cwd fallback, added `INVOWK_CONFIG` and `--ivk-no-cwd`/`INVOWK_NO_CWD=1`, and enforced absolute base directories everywhere.
- **`S3` (#173), `S5a` (#175), `-i` viewer & TUI fixes (#207, #215, #216, #227):** restricted the TUI server bind, moved direct TUI output to `/dev/tty`, fixed `-i` PTY/ANSI handling (`sudo`/`ssh`/`invowk tui *` work cleanly), and fixed `spin` Ctrl+C and `pager`/`table` pipe behavior.
- **SELinux Docker volume labeling (#141):** Docker volumes receive `:z` when the daemon enforces SELinux.
- **Module locking, containment, and CUE `v0.18.0` (#142, #188, #217, #223, #228, #231):** deterministic lock files and safe concurrent module operations.

Only the gaps that **do not exist yet** in `v0.18.0` (nor on `main` `7a282c1b`) and matter for CST remain below.

## Summary

| ID | Area | Priority for CST | Track |
|---|---|---|---|
| **G1** | Clean non-TTY / `NO_COLOR` output & quiet error framing | High | `C2` |
| **G2** | Termination signal forwarding & cleanup (`SIGTERM` / `SIGHUP`) | High | `S2` |
| **G3** | Declarative privilege capabilities (`elevated` / `unprivileged`) | Medium | `C3` |
| **G4** | Raw argument passthrough (`passthrough: true`) | High | `C3` |
| **G5** | Minimum `invowk_version` constraint & fail-closed invalid `invowkfile.cue` | Medium | `C3` |
| **G6** | Container runtime for project-local modules (`workdir: ".."` & mount options) | High | New |
| **G7** | Standalone CLI alias completion (`completion --alias`) | Low–Medium | `C4` |
| **G8** | System-wide config `/etc/invowk/config.cue` & root ownership checks | Deferred | Future |

---

## G1 — Clean non-TTY / `NO_COLOR` output & quiet error framing (`C2`)

**Current state in `v0.18.0`:**
- `cmd/invowk/service_error.go:45-64` (`renderServiceError`) and `cmd/invowk/issue_render.go:34-46` (`renderIssueCatalogEntry`) still invoke `glamour.Render(..., "dark")` unconditionally regardless of TTY or `NO_COLOR`.
- Styled lipgloss strings are written straight to `stderr`/`stdout` without `colorprofile` downsampling, and `ui.color_scheme` (`internal/config/config_schema.cue:135`, `cmd/invowk/config.go:27,35,176`) is stored and shown by `config show`/`set` without affecting any renderer.
- `fang.Execute` (`cmd/invowk/root.go:103-109`) prints an `ERROR / Exit status N.` frame on stderr whenever a command returns a non-zero exit code (because `App` wraps stdout/stderr rather than passing `*os.File` directly, fang's non-TTY check does not suppress it).

**Why it matters for CST:**
- CI job logs, `systemd` journal units, shell wrappers, and redirected pipelines (`cst doctor`, `cst perf`) must emit clean, parseable plain text when `stdout`/`stderr` are not a TTY or `NO_COLOR=1` / `TERM=dumb` is set.
- `cst doctor` and `cst perf` intentionally use exit status `1` for normal non-healthy verdicts (`WARN`/`ERROR` or `ATTENTION`/`BOTTLENECK`). An extra `ERROR / Exit status 1.` banner injected by invowk misleads users and breaks clean wrapper output.

**Design suggestions:**
- Wrap `App` stdout and stderr writers with `github.com/charmbracelet/colorprofile.NewWriter(w, os.Environ())` (honoring `NO_COLOR`, `CLICOLOR_FORCE`, and `TERM=dumb`).
- Add a persistent `--ivk-color=auto|always|never` flag (precedence: `--ivk-color` > `NO_COLOR`/`CLICOLOR_FORCE` > `ui.color_scheme` > `auto`), and select Glamour's `"notty"` style when color/TTY is off. Keep `invowk tui style` and `invowk tui format` explicitly opt-in to styling.
- Suppress fang's exit-status banner when forwarding a command's own non-zero exit status (or expose `--ivk-quiet` / proper TTY detection on the stderr fd so wrappers get raw stderr).

---

## G2 — Termination signal forwarding & cleanup (`SIGTERM` / `SIGHUP`) (`S2`)

**Current state in `v0.18.0`:**
- `cmd/invowk/root.go:107` passes only `fang.WithNotifySignal(os.Interrupt)` (`SIGINT`).
- `SIGTERM` and `SIGHUP` use the OS default action: immediate process termination without cancelling `context.Context`, cleaning up temp scripts/containers, or signalling child processes.

**Why it matters for CST:**
- Long-running dev/CI tasks (`invowk cmd test e2e tier1`, `invowk cmd test quint`, `invowk cmd test mutation`, `invowk cmd ci fast-gates`) or any command invoked under `systemd`, `timeout`, or a closing terminal receive `SIGTERM` or `SIGHUP`.
- Immediate termination leaves orphaned Docker containers, `bwrap`/`qemu`/Java processes, and temporary script files behind.

**Design suggestions:**
- Use a platform-split signal helper: notify `os.Interrupt`, `syscall.SIGTERM`, and `syscall.SIGHUP` on Unix (`os.Interrupt` on Windows).
- Run native child commands in their own process group (`SysProcAttr{Setpgid: true}` on Unix) and forward termination signals to `-pgid`, followed by container stop/remove and temp-script cleanup within a bounded grace period before forcing `SIGKILL`.
- Return `128 + signal` (e.g., `143` for `SIGTERM`, `130` for `SIGINT`) to match POSIX shell expectations.

---

## G3 — Declarative privilege capabilities (`elevated` / `unprivileged`) (`C3`)

**Current state in `v0.18.0`:**
- `#CapabilityName` (`pkg/invowkfile/invowkfile_schema.cue:370`, `pkg/invowkfile/capabilities.go:31`) only admits `"local-area-network" | "internet" | "containers" | "tty"`.
- Built-in privilege elevation (`elevate: "sudo"`) remains explicitly rejected: `invowk` commonly resides in user-writable directories, so re-execing it as root would turn user compromise into privilege escalation.

**Why it matters for CST:**
- CST strictly separates unprivileged commands (`doctor`, `perf`, `toolchains`, `plan`) from root operations (`install`, `apply`).
- Today, checking privilege inside `depends_on.custom_checks` (`test "$(id -u)" = 0`) is non-portable, does not show up cleanly in `--help` or `--ivk-dry-run`, and cannot be inspected by `invowk audit`.

**Design suggestions:**
- Add `"elevated"` (`os.Geteuid() == 0` on Unix; elevated token on Windows) and `"unprivileged"` to `#CapabilityName` and evaluate them in the existing `deps` pipeline (`internal/app/deps`).
- Render privilege requirements in command help and `--ivk-dry-run` output.
- Add an `invowk audit` rule flagging commands with `"elevated"` whose `invowkfile.cue`, `invowkmod.cue`, or referenced script/env files are writable by non-owners (group/world-writable).
- Optionally expose informational `INVOWK_INVOKING_USER` (read from `SUDO_USER`, never trusted for authorization).

---

## G4 — Raw argument passthrough (`passthrough: true`) (`C3`)

**Current state in `v0.18.0`:**
- Leaves use `cobra.ArbitraryArgs` (`cmd/invowk/cmd_discovery.go:232`), so `--` forwards flags intact (`invowk cmd x -- --help`), while unknown flags before `--` fail with exit `2` and `--help` is intercepted by invowk.
- `pkg/invowkfile/command_tree.go:78-112` forbids `args` on commands that also have child subcommands.

**Why it matters for CST:**
- Wrapping CST CLIs (`cst perf`, `cst sync`, `cst btrfs ...`) without `passthrough` forces duplicating every downstream flag in `cst.invowkmod/invowkfile.cue` and manually translating `INVOWK_FLAG_*` back into CLI options in bash.
- Every new option added to a Python CLI requires synchronized edits to `invowkfile.cue` and task unit tests.

**Design suggestions:**
- Add an opt-in `passthrough?: bool` field on `#Command` (mutually exclusive with `flags`, `args`, and child subcommands in both CUE schema and `ValidateCommandTree`).
- On the generated Cobra leaf (`DisableFlagParsing: true`), forward every token after the command path verbatim (`$@` / `ARGn` / `ARGC`), including `--help`, `-h`, and unknown flags, while still honoring `--ivk-*` global flags positioned before the command path.

---

## G5 — Minimum `invowk_version` constraint & fail-closed invalid `invowkfile.cue` (`C3`)

**Current state in `v0.18.0`:**
- `#Invowkmod` and `#Invowkfile` are `close({...})` schemas decoded in a single pass (`pkg/invowkmod/parse.go:40-50`, `pkg/invowkfile/parse.go`), so an older `invowk` binary rejects a new schema field as `field not allowed`.
- Discovery skips an invalid cwd `invowkfile.cue` with a warning, leaving `invowk cmd <name>` to fail with a generic "command not found" instead of surfacing the parse/version error.

**Why it matters for CST:**
- CST pins `invowk` in CI (`.github/actions/setup-invowk/action.yml`) while local workstations or toolboxes may have an older binary installed.
- When `cst.invowkmod/invowkfile.cue` adopts a newer invowk feature or has a syntax/schema error, users and agents need an immediate, unambiguous version mismatch or parse failure rather than a misleading "command not found".

**Design suggestions:**
- Add an optional `invowk_version` SemVer constraint field to `invowkmod.cue` and top-level `invowkfile.cue`.
- Read `invowk_version` in a lightweight schema-less CUE pre-pass before `ParseAndDecodeCompiled` and evaluate it with `pkg/invowkmod`'s existing `SemVerConstraint` (dev builds emit a verbose warning and continue); exit `1` with a clear upgrade message when unsatisfied.
- On CUE `field not allowed` errors, append a hint (`this file may require a newer version of invowk`).
- Fail closed (exit `1`) when a cwd `invowkfile.cue` or `*.invowkmod` fails schema/parse validation instead of silently dropping its commands.

---

## G6 — Container runtime for project-local modules (`workdir: ".."` & mount options)

**Current state in `v0.18.0`:**
- `internal/runtime/container_exec.go:154` always bind-mounts `invowkDir` (for `cst.invowkmod/`, the module directory itself) to `/workspace`, ignoring root `workdir: ".."`.
- `getContainerWorkDir` (`internal/runtime/container_provision.go:375-402`) sees `effectiveWorkDir` (`..`) is outside `invowkDir`, so it passes the **host** absolute checkout path directly as `-w <host_path>` inside the container, where that directory does not exist.
- `ContainerRuntime` `volumes` (`internal/container/engine_base.go:305-307`) are forwarded verbatim without resolving relative host paths against `invowkDir` (`..:/repo:ro` is rejected by Docker).
- Both Docker and Podman engines unconditionally append `:z` (`internal/container/docker.go:43`, `internal/container/podman.go:159`) whenever SELinux is enabled, mutating host file xattrs (`user_home_t` -> `container_file_t`) even when a read-only mount + `--security-opt label=disable` (or `:ro` without relabeling) is desired.

**Why it matters for CST:**
- CST places its tasks in `cst.invowkmod/` with `workdir: ".."` and enforces zero hardcoded checkout paths (`tests/test_repo_paths.py`).
- Running CI-parity `fast-gates` inside `registry.fedoraproject.org/fedora:45` should ideally use invowk's native `container` runtime rather than shelling out to `docker run` from a `native` task. Today it cannot: the container cannot see the repository root at `/workspace`, `-w` points to an unmounted host path, and `:z` relabels the developer's `~/Workspace` checkout on Silverblue.

**Design suggestions:**
- Allow `#RuntimeConfigContainerBase` to configure how the workspace is mounted (e.g. `workspace_mount?: {root?: "invowkfile" | "workdir", readonly?: bool, selinux_label?: "auto" | "shared" | "private" | "none"}`), so modules with `workdir: ".."` can mount the project root at `/workspace` (and `getContainerWorkDir` maps relative paths inside it cleanly).
- Support resolving relative host paths in `volumes` against the invowkfile/module base directory (subject to explicit opt-in or path-containment rules).
- Allow disabling `:z` auto-relabeling per container config or user config (`--security-opt label=disable` or `selinux_label: "none"` for read-only CI/inspection containers on SELinux hosts).

---

## G7 — Standalone CLI alias completion (`completion --alias`) (`C4`)

**Current state in `v0.18.0`:**
- `cmd/invowk/cmd_discovery.go:365-385` supports completion under `invowk cmd ...`, with no `--alias` or `--prefix` rewriting. Multi-call `argv[0]` dispatch remains rejected to avoid symlink/$HOME/cwd fragility.

**Why it matters for CST:**
- If CST later exposes an optional user-facing invowk wrapper (`exec /usr/local/bin/invowk -c <cfg> --ivk-no-cwd cmd cst "$@"`), shell tab-completion must complete `cst <subcommand>` rather than `invowk cmd cst <subcommand>`.

**Design suggestions:**
- Add `invowk completion <shell> --alias <name> --prefix "<command prefix>"` (e.g. `--alias cst --prefix "cmd cst"`) so generated bash/zsh/fish completion scripts bind to `<name>` and prepend `<prefix>` when querying `__complete`.
- Document the wrapper + completion pattern alongside G1 (`C2`) and G4 (`passthrough`).

---

## G8 (Deferred) — System-wide config `/etc/invowk/config.cue` & root ownership checks

- Revisit only if `invowk` is adopted for multi-user root tooling.
- Any root ownership/permission check on config or module files (`euid == 0`) must **warn by default** (CI containers routinely run as `root` over non-root UID bind mounts) with an opt-in strict mode analogous to OpenSSH `StrictModes`.

---

## Scope boundary: why CST production paths stay out of invowk

Even after G1–G7 land, CST keeps its production paths (`systemd` `ExecStart=` targets, root-owned copies in `/usr/local/sbin`, initramfs/degraded-boot helpers, GameMode hooks, Polkit rules, and user-session daemons) as plain standard-library Python / shell executables: core system recovery and root automation must not depend on an extra tool runtime that can change underneath them. `invowk` remains CST's dev/CI orchestration layer (already working well) and a candidate for an optional user-facing CLI wrapper once G1, G4, and G7 are available.
