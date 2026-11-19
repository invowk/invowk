# Design

## Context

See proposal.md (Why). Current behaviour, cited against `main` at d7e4feb4 (unchanged on 562f6842 for these files):

- **Where direct rendering draws.** Every directly rendered component builds `tea.NewProgram(model)` with no options:
  - `input.go:137`, `choose.go:386` and `:481`, `filter.go:206`, `file.go:142`, `write.go:141`, `table.go:157`;
  - `confirm.go:189`, `pager.go:165`, `spin.go:468` (all in `internal/tui/`).
  Bubble Tea v2.0.9 then writes to `os.Stdout` (`charm.land/bubbletea/v2@v2.0.9/tea.go:624-627`). The command adapters print the result to the same stream (`cmd/invowk/tui_input.go:99`; `tui_choose.go:77-111`, `tui_filter.go:131`, `tui_file.go:110`, `tui_write.go:97`, `tui_table.go:106`). `spin` prints the wrapped command's output there (`cmd/invowk/tui_spin.go:115-118`).
- **Input today.** When stdin is not a terminal, Bubble Tea already opens the TTY for input (`tea.go:1017-1026`). On Unix that is `/dev/tty` (`github.com/charmbracelet/ultraviolet@v0.0.0-20260811164956-006e29f97886/tty_unix.go:15-21`); on Windows it is `CONIN$` and `CONOUT$` (`tty_windows.go:12-25`). Output never falls back that way. An output that is not a TTY gets no size (`tty_unix.go:30-32`, `tea.go:1051-1058`), and frames are written into it regardless.
- **Unused knob.** `tui.Config.Output` (`internal/tui/tui.go:77-78`, default `os.Stdout` at `:213`) is read by no program. Two tests read it: `internal/tui/tui_test.go:140` (`TestDefaultConfig` asserts it is `os.Stdout`) and `internal/tui/tui_write_test.go:164` (the builder's expected options copy it). `huh` is not a dependency (`go.mod`); every component is a plain Bubble Tea model.
- **Server path.** `tuiclient.NewClientFromEnv()` decides between delegating and direct rendering (`cmd/invowk/tui_input.go:70`). Delegated rendering draws in the server's TUI and is not touched by this change.
- **Windows console opening.** Bubble Tea's `OpenTTY` on Windows opens `CONIN$` and then `CONOUT$` with `os.OpenFile` and checks nothing else (`ultraviolet .../tty_windows.go:12-25`). It does not check that a usable console is attached, and it leaks the `CONIN$` handle when `CONOUT$` fails. `golang.org/x/sys` (v0.49.0) is already a direct dependency (`go.mod:32`).
- **Spin today.** `SpinWithAction` starts the action goroutine before `runActionSpinner` builds the program (`internal/tui/spin.go:300-306`). A resolver error raised inside `runActionSpinner` would therefore come after the action had already started, and a naive "run the action without a spinner" fallback would run it a second time.
- **The `-i` viewer is not a terminal emulator.** `RunInteractiveSession` streams the PTY's bytes, minus OSC sequences, into a `strings.Builder` that a `viewport` shows (`internal/tui/interactive_session.go:56-69`, `interactive_model.go:42-56`, `interactive_helpers.go:27-33`). This change does not touch it: inside a container, `/dev/tty` is the same PTY. Whether no-server `-i` runs are usable through that viewer is S5b's question (Non-Goals).
- **Test surfaces.**
  - `tests/cli/tui_tmux_test.go` drives the real binary in a 100x30 tmux pane. It skips when tmux cannot start a session (`newTmuxSession`, `:54-57`), and it drives value components only with stdout uncaptured (`TestTUI_Input`).
  - tmux is present on the Linux CI legs: in run 43748272128 (main at d7e4feb4), job 132916300070 `Test (ubuntu-latest / podman)` ran and passed `TestTUI_Choose`, `TestTUI_Confirm`, `TestTUI_File` and the rest. On macOS (job 132916299843) every `TestTUI_*` skipped, because tmux is missing; Windows has no tmux either. Nothing today makes a missing tmux on Linux fail.
  - CI runs `./tests/cli/...` in full mode on the Docker and Podman legs, and in a non-short CLI step on Windows and macOS (`.github/workflows/ci.yml`, "Run CLI integration tests (full)" and "Run CLI integration tests").

### Output probe (2026-11-15, Fedora 45 host, tmux 80x20)

A throwaway Bubble Tea v2.0.9 program in the session scratchpad ran in seven configurations. Its model shows `PROMPT> <typed>`, and the keys `hey` + Enter came from `tmux send-keys`. "Captured" means `v=$(…)`, and the value was written with `printf %s` and dumped with `od`.

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
- One terminal-selection rule for every directly rendered component, with a defined outcome for each I/O situation, on Unix and Windows.
- Capture tests that cannot skip silently on the Linux CI legs.

**Non-Goals:**
- No-server `-i` runs, the gate test of a real no-server `-i` session, the `tui-server-exposure` deltas, the config-default change and the connect-failure fallback. They move to S5b (maintainer decision 39), recoverable from 9d8483ca and 97d1e6a5. S3's refusal is unchanged here.
- Making the `-i` viewer a terminal emulator.
- Changing `format` or `style`, or the `-i` viewer's own output (`interactive_session.go:33`), which is the parent's terminal UI.
- Server-delegated rendering, the wire protocol, and the bind plans.
- N12 (non-Linux host binary provisioned into Linux containers): tracker item only.

## Maintainer decisions

From the program log:

1. *(27)* The v0.17.0 tag waits for this change. The release still needs the maintainer.
2. *(39, replacing the mechanics of 29)* S5 splits in two. S5a, this change, is Part A only: components stop drawing to stdout. S5b, a new change after S5a merges, brings the no-server path and the hardened gate (33). If S5b's gate fails, S5b is shelved and S3's refusal ships in v0.17.0 (34), with nothing to revert here.
3. *(30)* Tracker: N11 is this defect, and N12 is the provisioning question. N12 is not S5 scope.

## Decisions

### D1. Draw on the controlling terminal, not on stderr

A new helper in `internal/tui` resolves a `componentTerminal` (input reader, output writer, close function). Every directly rendered `tea.NewProgram` call passes it through `tea.WithInput` and `tea.WithOutput`, and closes it after `Run`. Resolution order:

1. `tea.OpenTTY()`: `/dev/tty` on Unix, `CONIN$` and `CONOUT$` on Windows. On Windows the resolver then probes both handles with `windows.GetConsoleMode` (`golang.org/x/sys/windows`, already a dependency). If either probe fails, no usable console is attached: it closes both handles and treats step 1 as failed. It also closes `CONIN$` itself when the open fails halfway (Context). The probe is injected, so the rule is unit-tested on every OS, and a Windows-only test asserts that a regular file fails the real probe.
2. If step 1 fails, and both stdin and stderr are terminals, use stdin and stderr.
3. Otherwise fail with `ErrNoTerminal`, which wraps the open error and names the component.

| Situation | Outcome |
|---|---|
| stdout captured | UI on the terminal; stdout holds only the result (probe row 4) |
| stderr redirected | UI still on the terminal (probe row 5); stderr gets only errors |
| stdin a pipe (`filter`, `table` data, `</dev/null`) | keys come from the terminal; the adapter reads stdin data before the program starts (`cmd/invowk/tui_filter.go:70`, `tui_table.go:191`) |
| no controlling terminal, stdin and stderr terminals | step 2 |
| no terminal at all (CI) | value components, `confirm` and `table`: exit 1 with `ErrNoTerminal`, stdout empty; `spin` and `pager`: D3 |
| Windows console | `CONIN$`/`CONOUT$` after the console-mode probe; Bubble Tea enables VT processing on them (bubbletea `tty_windows.go:13-50`) |
| Windows with no usable console (detached, or handles that are not consoles) | the probe fails, then step 2 or 3 |
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
- `SpinWithAction` resolves the terminal **before** it starts the action goroutine (Context: today the goroutine starts first, `spin.go:300-306`). On `ErrNoTerminal` it runs the action synchronously, once, and returns nil. On success it starts the goroutine and the spinner as today. If the spinner program itself fails after the action started, `SpinWithAction` waits on `doneCh` before it returns the error, so the action is never run twice and never abandoned mid-run. A unit test with an injected failing resolver asserts the action runs exactly once.
- `pager` without a terminal writes the content to stdout, as `less` does.
- The other components need an answer, so they fail. `confirm` failing exits 1, which reads as "No". That is today's behaviour too, and the stderr message tells the two apart (Risks).

### D4. Test coverage per platform

- **Resolver (D1):** unit tests with an injected opener, terminal checks and console probe, run on every CI leg (Linux, macOS, Windows). The Windows-only probe test runs on the Windows leg.
- **Capture tests:** real-binary tmux tests in `tests/cli/tui_tmux_test.go`. tmux exists only on the Linux legs (Context: run 43748272128, job 132916300070), so these tests run on Linux only. macOS and Windows have no tmux and skip them; their coverage is the resolver unit tests and `tui_no_terminal.txtar` (Unix, `setsid`).
- **No silent skip on Linux:** a `requireTmux` helper replaces the bare skip in `newTmuxSession`. When `INVOWK_TEST_REQUIRE_TMUX=1` it fails the test instead of skipping. `.github/workflows/ci.yml` sets that variable on the CLI full step of the two Linux full legs (Docker and Podman) only. Local runs without the variable keep skipping.
- **Calibration:** the captured-input test is checked once against a build with the `tea.WithOutput` option removed at the input call site; it must fail on the hex check.
- Windows `CONIN$`/`CONOUT$` in a real console is not exercised end to end in CI. A manual check on the `win11-devbox` VM is optional and the maintainer's call.

## Risks / Trade-offs

- [`confirm` without a terminal exits 1, like "No"] → Unchanged from today. The stderr error names the cause. A distinct exit code would need a `cli-exit-codes` change, outside this change.
- [Opening `/dev/tty` affects processes that redirect all three streams and expect no UI] → They get the UI on their terminal, which is where an interactive prompt belongs. Without a terminal they get `ErrNoTerminal` instead of Bubble Tea's error.
- [Windows terminals that are not consoles (mintty without ConPTY)] → `CONIN$` may open but fail the console-mode probe. Step 2 then uses stdin/stderr if they are consoles, otherwise `ErrNoTerminal`. Not covered end to end by CI; the probe and the fallback are unit-tested.
- [Capture tests skip on macOS and Windows] → No tmux there. The resolver rule is unit-tested on those legs, and the Linux legs cannot skip the capture tests (D4).
- [`-i` sessions in no-route modes] → Still refused by S3 in this change. S5b decides whether they can run without a server.

## Migration Plan

Ship in v0.17.0, on top of S4 (562f6842). To roll back, revert the change: it has no persisted state, schema or config change.

## Open Questions

- With a server, `invowk tui spin` sends `SpinContext` and then still runs its own local spinner (`cmd/invowk/tui_spin.go:90-110`, `internal/tuiclient/client.go:472-478`). This change keeps that behaviour and only moves the local spinner off stdout. Whether a server-delegated spin should skip the local spinner is a separate question, and it does not change this change's specs.
