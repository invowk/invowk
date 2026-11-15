# Design

## Context

See proposal.md (Why). Current behaviour, cited against `main` at d7e4feb4:

- **Where direct rendering draws.** Every directly rendered component builds `tea.NewProgram(model)` with no options:
  - `input.go:137`, `choose.go:386` and `:481`, `filter.go:206`, `file.go:142`, `write.go:141`, `table.go:157`;
  - `confirm.go:189`, `pager.go:165`, `spin.go:468` (all in `internal/tui/`).
  Bubble Tea v2.0.9 then writes to `os.Stdout` (`charm.land/bubbletea/v2@v2.0.9/tea.go:624-627`). The command adapters print the result to the same stream (`cmd/invowk/tui_input.go:99`; `tui_choose.go:77-111`, `tui_filter.go:131`, `tui_file.go:110`, `tui_write.go:97`, `tui_table.go:106`). `spin` prints the wrapped command's output there (`cmd/invowk/tui_spin.go:115-118`).
- **Input today.** When stdin is not a terminal, Bubble Tea already opens the TTY for input (`tea.go:1017-1026`). On Unix that is `/dev/tty` (`github.com/charmbracelet/ultraviolet@v0.0.0-20260811164956-006e29f97886/tty_unix.go:15-21`); on Windows it is `CONIN$` and `CONOUT$` (`tty_windows.go:12-25`). Output never falls back that way. An output that is not a TTY gets no size (`tty_unix.go:30-32`, `tea.go:1051-1058`), and frames are written into it regardless.
- **Unused knob.** `tui.Config.Output` (`internal/tui/tui.go:77-78`, default `os.Stdout` at `:213`) is read by no program; only `tui_write_test.go:164` mentions it. `huh` is not a dependency (`go.mod`); every component is a plain Bubble Tea model.
- **Server path.** `tuiclient.NewClientFromEnv()` decides between delegating and direct rendering (`cmd/invowk/tui_input.go:70`). The reach timeout is 5 s (`internal/tuiclient/client.go:47-50`), enforced by `checkReachable` (`:521`). An unreachable server is `ErrTUIServerUnreachable` (`:61-66`), and its error names the opt-in (`:193`).
- **The `-i` viewer is not a terminal emulator.** `RunInteractiveSession` streams the PTY's bytes, minus OSC sequences, into a `strings.Builder` that a `viewport` shows (`internal/tui/interactive_session.go:56-69`, `interactive_model.go:42-56`, `interactive_helpers.go:27-33`). A child Bubble Tea program's cursor movements are therefore shown as text. This explains the "redraw artifacts" of S3's premise check (`openspec/changes/archive/2026-11-15-restrict-tui-server-bind/design.md:56`). Part A does not change it: inside a container, `/dev/tty` is the same PTY.
- **S3's refusal on main.**
  - `checkedTUIExposurePlan` returns `plan.RefusalError()` (`internal/app/commandadapters/interactive.go` on d7e4feb4).
  - `commandsvc.preflightInteractive` runs it before dependency validation, and turns a config-default request into a non-interactive run with a warning (`internal/app/commandsvc/dispatch.go`, the `!req.InteractiveSet` branch).
  - `classifyExecutionError` maps the refusal and an invalid plan to `ErrorKindTUIExposureRefused` (`internal/app/commandsvc/errors.go`), rendered by `internal/issue/templates/tui_exposure_refused.md`.
  - `tests/cli/testdata/tui_exposure_refused.txtar` pins the behaviour.
- **S3's stage-3 code.** Commit 3022dea3 (`git fetch origin refs/pull/173/head`) ran refused plans without a server: `plan.HasServer()` guarded the server start, the bridge and `Stop`, and printed `plan.Warning` once before the alternate screen. `git diff 3022dea3 d7e4feb4 -- internal/app/commandadapters/interactive.go` is the exact delta Part B reverses.
- **Test surfaces.**
  - `tests/cli/tui_tmux_test.go` drives the real binary in a 100x30 tmux pane. It skips when tmux is missing (`:103-112`), and it drives value components only with stdout uncaptured (`TestTUI_Input` at `:232`).
  - The container PTY test `internal/app/commandadapters/interactive_exposure_integration_test.go` replaces the viewer with a raw `creack/pty` session (`ptySession`, `:149-168`), so it never runs the real viewer.
  - CI runs `./tests/cli/...` in full mode on the Docker and Podman legs, and in a non-short CLI step on Windows and macOS (`.github/workflows/ci.yml:230-260`). The Podman leg is rootless with pasta (`:74-82`).

### Output probe (2026-11-15, Fedora 45 host, tmux 80x20)

A throwaway Bubble Tea v2.0.9 program in the session scratchpad ran in five configurations. Its model shows `PROMPT> <typed>`, and the keys `hey` + Enter came from `tmux send-keys`. "Captured" means `v=$(…)`, and the value was written with `printf %s` and dumped with `od`.

| Configuration | Prompt visible | Captured value |
|---|---|---|
| default (`tea.NewProgram(m)`), captured | no | `1b5b3f32303236247…` (control sequences) then `hey` |
| `WithOutput(os.Stderr)`, captured | yes | `hey` |
| `WithOutput(os.Stderr)`, captured, `2>err.log` | **no**; `err.log` got the frames | `hey` |
| `OpenTTY()` in + out, captured | yes | `hey` |
| `OpenTTY()`, captured, `2>err.log </dev/null` | yes | `hey` |
| `OpenTTY()` under `setsid` (no controlling terminal) | n/a | exit 1, `open /dev/tty: no such device or address` |
| default under `setsid` | n/a | exit 1, `bubbletea: error opening TTY` |

## Goals / Non-Goals

**Goals:**
- Part A: one terminal-selection rule for every directly rendered component, with a defined outcome for each I/O situation.
- Gate: one automated, CI-enforced decision on whether no-server `-i` sessions are usable.
- Part B (if the gate passes): restore stage 3's no-server runs with S3's later hardening intact (plan validation, the authenticated health check, `Proxy = nil`).

**Non-Goals:**
- Making the `-i` viewer a terminal emulator. Redraw artifacts of child programs stay (Risks).
- Changing `format` or `style`, or the `-i` viewer's own output (`interactive_session.go:33`), which is the parent's terminal UI.
- Server-delegated rendering, the wire protocol, and the bind plans of narrow rows.
- N12 (non-Linux host binary provisioned into Linux containers): tracker item only.

## Maintainer decisions (2026-11-15)

From the program log, Round 8:

1. *(27)* The v0.17.0 tag waits for this change. The release still needs the maintainer.
2. *(28)* S3 merged as d7e4feb4. Its stage-3 no-server code stays reachable at 3022dea3.
3. *(29)* The change has a gate. Part A ships either way. The gate is an automated PTY test of a real no-server `-i` session on row 10: a byte-exact captured value, and usable uncaptured `input` and `choose`. Part B ships only if the gate passes. It covers:
   - a no-server run with one warning, written as MODIFIED and REMOVED deltas against `tui-server-exposure`;
   - config-default interactive mode treated like `-i`;
   - the connect-failure fallback with a shorter bound, where a `401` stays an error;
   - migration notes that supersede S3's refusal note.
   If the gate fails, v0.17.0 keeps the refusal, and the maintainer is told.
4. *(30)* Tracker: N11 is this defect, and N12 is the provisioning question, verified by reading code. N12 is not S5 scope.
5. *(31)* S4 merges first, and S5 rebases onto it.

## Decisions

### D1. Draw on the controlling terminal, not on stderr

A new helper in `internal/tui` resolves a `componentTerminal` (input reader, output writer, close function). Every directly rendered `tea.NewProgram` call passes it through `tea.WithInput` and `tea.WithOutput`, and closes it after `Run`. Resolution order:

1. `tea.OpenTTY()`: `/dev/tty` on Unix, `CONIN$` and `CONOUT$` on Windows.
2. If that fails, and both stdin and stderr are terminals, use stdin and stderr.
3. Otherwise fail with `ErrNoTerminal`, which wraps the open error and names the component.

| Situation | Outcome |
|---|---|
| stdout captured | UI on the terminal; stdout holds only the result (probe row 4) |
| stderr redirected | UI still on the terminal (probe row 5); stderr gets only errors |
| stdin a pipe (`filter`, `table` data, `</dev/null`) | keys come from the terminal; the adapter reads stdin data before the program starts (`cmd/invowk/tui_filter.go:70`, `tui_table.go:191`) |
| no controlling terminal, stdin and stderr terminals | step 2 |
| no terminal at all (CI) | value components, `confirm` and `table`: exit 1 with `ErrNoTerminal`, stdout empty; `spin` and `pager`: D3 |
| Windows console | `CONIN$`/`CONOUT$`; Bubble Tea enables VT processing on them (bubbletea `tty_windows.go:13-50`) |
| inside a `-i` container (`-t`) | `/dev/tty` is the container's PTY, which the viewer shows |

- *Alternative: stderr.* Gum's choice. Rejected by the probe: `2>log` hides the UI and writes frames into the log (probe row 3).
- *Alternative: use stdout when it is a terminal.* Rejected: one rule is simpler to test, and the result is the same terminal.
- *Alternative: fall back to stdout.* Rejected: that is the defect.
- Colour detection follows the output (`tea.go:1091-1093`). Captured runs therefore keep their colours on the terminal, and the captured value has none.

### D2. Which components are affected

- Value components: `input`, `choose`, `filter`, `file`, `write`, and `table` (including the non-selectable display).
- Status and display components: `confirm`, `spin`, `pager`.
- Unchanged: `format` and `style` (lipgloss strings written to stdout, `cmd/invowk/tui_format.go:93`, `tui_style.go:116`), the server-side embedded components, and the `-i` viewer.
- `Config.Output` is removed, because nothing reads it. The builder API keeps compiling, since no caller sets the field.

### D3. `spin` and `pager` degrade instead of failing

- `spin` decorates a command. Today `invowk tui spin -- make build` in CI fails before it runs the command, because Bubble Tea cannot open a TTY (probe row 7). With `ErrNoTerminal`, `SpinWithAction` runs the action without a program, and the adapter keeps its output and exit-code handling (`cmd/invowk/tui_spin.go:104-130`).
- `pager` without a terminal writes the content to stdout, as `less` does.
- The other components need an answer, so they fail. `confirm` failing exits 1, which reads as "No". That is today's behaviour too, and the stderr message tells the two apart (Risks).

### D4. Gate surface: the tmux harness in `tests/cli`

The gate is `TestTUI_NoServerInteractiveSession` in a new `tests/cli/tui_noserver_tmux_test.go`. It runs the real binary: `invowk cmd gate -i` in a tmux pane against a persistent rootless Podman `debian:stable-slim` target (`persistent: {create_if_missing: true}`), which the real planner puts on row 10. The script:
1. runs `name=$(invowk tui input --title GATE-IN)`;
2. prints `GATE-HEX:` followed by `printf %s "$name" | od -An -tx1 | tr -d ' \n'`;
3. runs uncaptured `invowk tui input --title GATE-RAW` and `invowk tui choose red green blue`, and prints their values the same way.

The test asserts four things:
- the `GATE-IN` title is visible before typing;
- the hex equals the typed text exactly. Hex survives the viewer and `capture-pane`, so the comparison is byte-exact;
- the uncaptured `input` value is right, and `choose` returns `green` after Down and Enter;
- exactly one warning appears, and no `INVOWK_TUI_` variable is in `env`.

Why tmux and not a Go PTY test:
- The adapters' PTY test bypasses the real viewer (`ptySession`), and the viewer is where S3 saw artifacts.
- A raw PTY stream of the cursed renderer is diff-encoded, while tmux gives the emulated screen.

"Usable" means the prompt title is visible and keys produce the right values. Redraw artifacts from the viewer (Context) are tolerated and documented, not asserted against.

Enforcement:
- The test skips unless the engine is rootless Podman.
- The CI Podman leg sets `INVOWK_TEST_REQUIRE_TUI_GATE=1`. Then a missing tmux, a missing engine or a plan row other than 10 fails the test instead of skipping it.
- If `ubuntu-latest` has no tmux, the first CI run shows it, and the fallback is the same flow driven through `creack/pty` with a terminal-emulator screen such as `charmbracelet/x/vt`, which would be a new pinned test dependency (it is not in `go.mod` today). That fallback is decided in task 3.1, not at random.
- macOS and Windows have no container engine in CI, so they do not run the gate. They run the Part A unit tests of the resolution rule (D1), and macOS runs the Part A tmux capture tests when tmux is present. Windows `CONIN$`/`CONOUT$` is covered by unit tests with an injected opener only. A manual check on the `win11-devbox` VM is optional and the maintainer's call.

### D5. Gate mechanics: land the no-server path first, revert it on failure

On main, row 10 refuses before start, so the gate needs the no-server path. Task group 3 therefore applies the minimal stage-3 adapter delta (D6, first bullet) as its own commit, then runs the gate locally and in CI.
- **Pass:** group 4 completes Part B on top.
- **Fail:** that commit is reverted, `specs/tui-server-exposure/` is deleted from this change, groups 4-5 are marked dropped, `openspec validate --strict` is re-run, and the maintainer is told. The refusal ships in v0.17.0, together with Part A.

### D6. Part B shape

- **Adapter.** Restore stage 3's `HasServer()` branch: no server, no bridge, no `ctx.TUI`, and one warning before the alternate screen. Keep `plan.Validate()` as a hard error.
- **Plan.** `TUIPlanRefused` becomes `TUIPlanNoServer`. `RefusalError()` and `ErrTUIExposureRefused` go, and `Warning()` carries the text. Row 2's warning names `--share=network` and not the opt-in.
- **Preflight.** `InteractivePreflight` stays, but only reports an invalid plan, so that an invalid plan still fails before container-scoped `depends_on` probes start containers. The warning is printed once, by the adapter.
- **Config default.** The `!req.InteractiveSet` branch in `preflightInteractive` goes. A config-default request runs like `-i`. `InteractiveSet` is removed if nothing else reads it.
- **Errors.** `ErrorKindTUIExposureRefused`, `TUIExposureRefusedId` and `tui_exposure_refused.md` go. An invalid plan becomes a generic execution failure (exit 1), because the validated constructors leave only the zero value as an invalid plan.
- **Client.** The reach bound goes from 5 s to 2 s. On `ErrTUIServerUnreachable`, the adapters print one warning that names the address and the opt-in, then render directly through D1. A `401` and the caller's own cancellation stay errors. A health check on loopback or a bridge answers in milliseconds, so 2 s leaves margin for loaded runners. Each unreachable call costs at most 2 s.
- **Tests and docs.** `tui_exposure_refused.txtar` becomes `tui_exposure_noserver.txtar`: an explicit `-i` and a config default each run with one warning, and without `-i` there is no warning. README and `interactive-mode.mdx` (en and pt-BR) describe no-server runs.

## Risks / Trade-offs

- [The `-i` viewer shows child redraws as text] → Prompts stay readable and values correct, which is what the gate asserts. A viewer with a terminal emulator is a separate change (Open questions).
- [`confirm` without a terminal exits 1, like "No"] → Unchanged from today. The stderr error names the cause. A distinct exit code would need a `cli-exit-codes` change, outside S5.
- [Opening `/dev/tty` affects processes that redirect all three streams and expect no UI] → They get the UI on their terminal, which is where an interactive prompt belongs. Without a terminal they get `ErrNoTerminal` instead of Bubble Tea's error.
- [Windows terminals that are not consoles (mintty without ConPTY)] → `CONIN$` may fail there. Step 2 then uses stdin/stderr if they are consoles, otherwise `ErrNoTerminal`. Not covered by CI.
- [Gate passes on Podman 5.7 in CI but behaves differently elsewhere] → The gate covers row 10 only. The other no-server rows share the adapter path and the in-container rendering, and their documentation says they are unverified on real engines.
- [2 s bound on a slow but working bridge] → The fallback renders directly, which works after Part A, so a false "unreachable" costs a warning, not a failure.

## Migration Plan

Ship in v0.17.0. S4 merges first, and this branch rebases onto it (decision 31). To roll back, revert the change. Part A has no persisted state. Part B removes an issue kind, which is not part of any config or schema.

## Open Questions

- Should the `-i` viewer emulate a terminal (for example with `charmbracelet/x/vt`), so that child programs draw without artifacts? This is deferrable: it changes neither this change's specs nor the gate's definition of usable.
