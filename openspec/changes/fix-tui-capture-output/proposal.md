# Proposal

## Why

Rendered directly, the `invowk tui` components draw to stdout. So the documented capture form `name=$(invowk tui input)` gives the script Bubble Tea's terminal control sequences in front of the answer, draws nothing and exits 0. This happens on `main` in a plain terminal, not only under `-i`. S3 (`restrict-tui-server-bind`, PR #173) therefore refuses `-i` in engine modes without a narrow TUI route. With the defect fixed, those modes could run without a TUI server instead. The v0.17.0 tag waits for this change (maintainer decision 27).

## What Changes

The change has two parts with a gate between them (maintainer decision 29).

**Part A: components draw on the terminal, not stdout. Part A ships whatever the gate shows.**

- `input`, `choose`, `filter`, `file`, `write` and `table` draw their UI on the controlling terminal: `/dev/tty` on Unix, `CONIN$`/`CONOUT$` on Windows. stdout carries only the result. `confirm`, `spin` and `pager` draw there too. `format` and `style` are unchanged, because their stdout is their product.
- When no controlling terminal can be opened, invowk falls back to stdin and stderr, if both are terminals. Otherwise the drawing components fail with exit status 1 and an error saying the component needs a terminal. `spin` then runs its command without the spinner. `pager` then writes the content to stdout.
- stdout captured, stderr redirected and stdin piped no longer corrupt a captured value or hide the UI.

**Gate: an automated PTY test of a real no-server `-i` session.**

- A tmux test in `tests/cli` drives `invowk cmd <x> -i` against a persistent rootless Podman container (plan row 10), with the stage-3 no-server path applied on the branch. It asserts three things:
  - a captured `input` value is byte-exact;
  - the prompt is visible;
  - uncaptured `input` and `choose` return the right value.
- It must pass on the CI Podman leg, where it fails instead of skipping.

**Part B: only if the gate passes. If the gate fails, Part B is dropped and S3's refusal stays.**

- **BREAKING (reverses an unreleased S3 BREAKING):** in modes without a narrow route, `invowk cmd … -i` runs again. invowk opens no TUI server, injects no `INVOWK_TUI_*` variables and prints exactly one stderr warning. The command's `invowk tui` calls render directly on the container's terminal.
- Interactive mode that comes from the `ui.interactive` config default behaves like an explicit `-i`. It no longer falls back to a non-interactive run.
- Flatpak without host networking also runs without a server, and its warning still does not offer the opt-in.
- When `INVOWK_TUI_ADDR` is set but the server does not answer the authenticated health check within 2 s, a delegating `invowk tui` command prints one warning and renders directly. Before, it failed after 5 s. A `401` stays an error.
- Removed: the `tui_exposure_refused` issue kind and template, the refusal error, the refusal half of the interactive preflight, and `tests/cli/testdata/tui_exposure_refused.txtar`. The docs (README, website, pt-BR) are rewritten to match.
- Kept: an invalid bind plan still fails before any listener opens or any container starts.

## Capabilities

### New Capabilities
- `tui-component-terminal-io`: where the `invowk tui` components draw and read keys when they render directly, and what they do without a terminal.

### Modified Capabilities
- `tui-server-exposure` (Part B only): modes without a narrow route run without a server instead of refusing. The "no fallback" requirement is removed. The unreachable-server requirement gains a bounded fallback to direct rendering. Flatpak without network runs without a server.

## Impact

- Part A: `internal/tui` (every `tea.NewProgram` call site and the unused `Config.Output`), `cmd/invowk/tui_*.go` (spin and pager without a terminal), `tests/cli/tui_tmux_test.go`, and the `invowk tui` docs.
- Gate: a new tmux test, and `.github/workflows/ci.yml` (the Podman leg sets the gate's required flag).
- Part B:
  - `internal/app/commandadapters/interactive.go`;
  - `internal/app/commandsvc` (`dispatch.go`, `errors.go`, `ports.go`, `types.go`);
  - `internal/containerplan/tuiexposure.go`;
  - `internal/issue` and its template;
  - `internal/tuiclient` (the bound and the fallback);
  - `cmd/invowk/app.go`;
  - their tests;
  - `tests/cli/testdata/tui_exposure_refused.txtar`;
  - README, `website/docs/advanced/interactive-mode.mdx` and its pt-BR copy.

## Migration notes

- **Fixed:** `v=$(invowk tui input)` and the other value components capture exactly the answer, and the UI draws on the terminal even when stdout or stderr is redirected. Scripts that stripped escape sequences from captured values can stop. A component run with no terminal at all now fails with a clear error. Before, it failed with Bubble Tea's "could not open TTY" error. `invowk tui spin` and `invowk tui pager` instead degrade to plain output.
- **Supersedes S3's BREAKING refusal note (Part B, if shipped):** S3's notes said that `invowk cmd … -i` exits 1 before start in modes without a narrow TUI route: remote engines, rootful Podman, persistent rootless Podman, rootless Podman without pasta, rootless Docker, Snap and Flatpak without network. That refusal never ships. In those modes the run starts with one warning and no TUI server, and `invowk tui` calls draw in the command's own terminal. The `ui.interactive: true` config default now behaves like `-i` there, instead of running non-interactively. The opt-in `container.tui_server_bind: "all-interfaces"` is unchanged. Inside a container, an unreachable TUI server now leads to direct rendering after 2 s, with a warning, instead of an error after 5 s.
- **If the gate fails:** S3's refusal note stands, and only the "Fixed" note ships.
