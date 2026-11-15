# Tasks

Gate rule (maintainer decision 29): groups 1-2 (Part A) ship whatever the gate shows. Group 3 is the gate. Groups 4-5 (Part B) run only if task 3.4 records a pass. **If the gate fails, Part B tasks are dropped and S3's refusal stays.** In that case, carry out the FAIL procedure under group 3: revert the gate's no-server commit, delete `specs/tui-server-exposure/`, delete the unchecked tasks of groups 4-5 (or turn them into plain bullets under a "Dropped by the gate" note), because unchecked boxes would keep the change incomplete, re-run `openspec validate fix-tui-capture-output --strict`, and tell the maintainer. Then continue with group 6.

## 1. Part A: terminal resolution in `internal/tui`

- [ ] 1.1 Add the D1 resolver to `internal/tui`: `tea.OpenTTY`, then stdin+stderr when both are terminals, otherwise `ErrNoTerminal`, a sentinel with a typed wrapper that names the component. Inject the opener and the terminal checks so they can be unit-tested. Verify with `go test -run TestResolveComponentTerminal ./internal/tui/`, which covers each D1 table row, including a failing opener with terminal stdio and with non-terminal stdio.
- [ ] 1.2 Pass the resolved input and output to every directly rendered `tea.NewProgram`, and close the terminal after `Run`. The call sites are `input.go:137`, `choose.go:386`/`:481`, `filter.go:206`, `file.go:142`, `write.go:141`, `table.go:157`, `confirm.go:189`, `pager.go:165` and `spin.go:468`. Leave `interactive_session.go:33` unchanged. Verify that `rg -n 'tea.NewProgram\(' internal/tui` shows only resolver-backed calls plus the viewer.
- [ ] 1.3 Remove the unused `tui.Config.Output` field and its default, and update `tui_write_test.go:164`. Verify with `go build ./... && go test ./internal/tui/`.
- [ ] 1.4 Without a terminal, `SpinWithAction` runs the action with no program, and `Pager` writes the content to stdout (D3). Verify with unit tests for both that inject a failing resolver, and with `go test -run 'TestTuiSpin|TestTuiPager' ./cmd/invowk/`.
- [ ] 1.5 Make the adapters surface `ErrNoTerminal` as a one-line error with exit 1 and empty stdout. Verify with a testscript `tests/cli/testdata/tui_no_terminal.txtar` that runs each case under `setsid` with stdio redirected, so that a local run cannot open the developer's `/dev/tty`: `input` exits 1 with `needs a terminal` on stderr, and `spin -- sh -c 'echo ok; exit 3'` prints `ok` and exits 3. Gate it with `[!exec:setsid] skip`. It is not a virtual test, so it needs no mirror.
- [ ] 1.6 Run `make lint` and `go test ./internal/tui/ ./cmd/invowk/`.

## 2. Part A: capture tests and docs

- [ ] 2.1 Extend `tests/cli/tui_tmux_test.go` with capture variants. Each assertion compares `od -An -tx1` hex of the captured value with the expected bytes, and checks that the title was visible before keys were sent. Cover:
  - `v=$(invowk tui input)`, `choose`, `filter` (options piped), `write`, `file` and `table --selectable`;
  - `out=$(invowk tui spin -- echo done)`;
  - `input` with `2>err.log` (assert `err.log` holds no ESC byte);
  - `input </dev/null`.
  Keep `TestTUIExemptionTmuxCoverage` green. Verify with `go test -run 'TestTUI_' ./tests/cli/` under tmux.
- [ ] 2.2 Calibrate: temporarily drop the `tea.WithOutput` option at one call site (input), confirm the capture test fails with control bytes in the hex, then restore it. Record the result in the PR description.
- [ ] 2.3 Update the `invowk tui` docs: README, `website/docs` TUI pages, and their pt-BR copies. Say that the UI draws on the terminal and stdout carries only the result, and give the no-terminal behaviour per component (D3). Verify with `cd website && npm run build`.
- [ ] 2.4 Run `make test-cli`.

## 3. Gate: no-server `-i` session on row 10

- [ ] 3.1 Check that tmux is available on the CI Podman leg. Read the last CI log for `TestTUI_*`: did they run or skip? Record the answer. If tmux is missing, choose the D4 fallback (a `creack/pty` + terminal-emulator screen) before writing 3.3, and record why.
- [ ] 3.2 In its own commit, apply the minimal no-server adapter path from 3022dea3 (`git diff 3022dea3 d7e4feb4 -- internal/app/commandadapters/interactive.go`, reversed): for a refused plan, no server, no `ctx.TUI` and one warning. Keep `plan.Validate()`. Also bypass the preflight refusal for that plan. Nothing else changes in this commit. Verify that `go test ./internal/app/commandadapters/` builds and runs. The refusal tests are expected to fail until group 4.
- [ ] 3.3 Write `TestTUI_NoServerInteractiveSession` in `tests/cli/tui_noserver_tmux_test.go` (D4):
  - a persistent rootless Podman `debian:stable-slim` target, created and removed by the test;
  - a captured `input` compared as hex, uncaptured `input` and `choose`, the title visible before keys, the warning exactly once, and no `INVOWK_TUI_` in `env`.
  With `INVOWK_TEST_REQUIRE_TUI_GATE=1` it fails instead of skipping. Add that variable to the Podman leg in `.github/workflows/ci.yml`. Verify locally on rootless Podman with `go test -run TestTUI_NoServerInteractiveSession ./tests/cli/`.
- [ ] 3.4 Run the gate locally and in CI on the Podman leg, then record PASS or FAIL in this file with the CI run ID. Calibrate by pointing the captured case at a build without Part A (a binary built from d7e4feb4, swapped in for the test's `binaryPath`): it must fail on the hex check.
FAIL procedure (plain bullet, not tracked; only on a FAIL in 3.4):

- Revert 3.2's commit and delete `specs/tui-server-exposure/`.
- Keep 3.3's test only if it can assert the refusal instead; otherwise delete it.
- Delete or un-box groups 4-5, run `openspec validate fix-tui-capture-output --strict`, and tell the maintainer that v0.17.0 keeps the refusal.

## 4. Part B: no-server runs (only after a gate PASS)

- [ ] 4.1 Change `internal/containerplan/tuiexposure.go`: `TUIPlanRefused` becomes `TUIPlanNoServer`, and `RefusalError()` and `ErrTUIExposureRefused` are removed. Make `Warning()` give each no-server row its text, with row 2 naming `--share=network` and not the opt-in. Keep `Validate`, `NewTUIExposurePlanRefused` renamed, and the row invariants. Verify with `go test ./internal/containerplan/`, including `TestSelectTUIExposureAlwaysValid`.
- [ ] 4.2 Finish the adapter: one warning before the alternate screen, `HasServer()` guarding server, bridge and `Stop`, and an invalid plan still exit 1 before `PrepareInteractive`. Rework `interactive_exposure_test.go` and `TestInteractivePersistentRootlessPodmanRefused`, which becomes `…NoServer`: it asserts no listener and no `INVOWK_TUI_*`. Verify with `go test ./internal/app/commandadapters/`.
- [ ] 4.3 Narrow `commandsvc.preflightInteractive` to invalid plans, and drop the `!req.InteractiveSet` downgrade, so that config-default runs behave like `-i`. Remove `InteractiveSet` if unused. Update `dispatch_preflight_test.go` so that an invalid plan still fails before dependency validation and a no-server plan passes. Verify with `go test ./internal/app/commandsvc/`.
- [ ] 4.4 Remove `ErrorKindTUIExposureRefused`, `TUIExposureRefusedId`, `internal/issue/templates/tui_exposure_refused.md` and their wiring (`cmd/invowk/app.go`, `errors.go`, `types.go`, the issue tests and `cmd_execute_error_classifier_test.go`). Verify that `rg -n 'TUIExposureRefused|tui_exposure_refused' --glob '!openspec/**'` is empty and `go test ./internal/issue/ ./cmd/invowk/` passes.
- [ ] 4.5 Replace `tests/cli/testdata/tui_exposure_refused.txtar` with `tui_exposure_noserver.txtar`, using the same fake docker on row 4. It covers: an explicit `-i` runs (exit 0) with exactly one warning naming the opt-in; the config default behaves the same; and without `-i` there is no warning and no facts query. Verify with `go test -run 'TestCLI/tui_exposure_noserver' ./tests/cli/`.
- [ ] 4.6 Edit the Purpose of `openspec/specs/tui-server-exposure/spec.md`, which says invowk refuses interactive runs, to describe no-server runs. Verify with `openspec validate --specs --strict`.

## 5. Part B: client fallback and docs (only after a gate PASS)

- [ ] 5.1 In `internal/tuiclient/client.go`, set `reachTimeout` to 2 s. On `ErrTUIServerUnreachable`, the `cmd/invowk/tui_*.go` adapters print one warning (address plus `container.tui_server_bind`) and render through the Part A resolver. A `401` and cancellation stay errors. Factor this into one shared helper, not nine copies. Verify with `go test ./internal/tuiclient/ ./cmd/invowk/` and update `client_unreachable_test.go`: refused, reset, silent and closed fall back within 2 s plus slack, and a 401 errors.
- [ ] 5.2 Update `TestTUIClientUnreachableFromContainer`, which now expects a fallback, and re-run the gate test on top of all of Part B. Verify with `go test -run 'TestTUIClientUnreachableFromContainer|TestTUI_NoServerInteractiveSession' ./internal/app/commandadapters/ ./tests/cli/` on the Podman leg.
- [ ] 5.3 Rewrite the no-route paragraphs in `website/docs/advanced/interactive-mode.mdx` (`:141-149`), its pt-BR copy and the README. Cover no-server runs, the single warning, the config default, the 2 s fallback, and the viewer's redraw artifacts. Add the proposal's migration notes to the v0.17.0 release-notes draft, superseding S3's refusal note. Verify with `cd website && npm run build`.

## 6. Tracker and completion gates

- [ ] 6.1 Keep `.claude/issues/cst-adoption-gaps.md` current: N11 points to this change, and its state reflects the gate result.
- [ ] 6.2 Run the completion gates of `.agents/rules/checklist.md`:
  - `/simplify`;
  - `make tidy` and `make license-check` (new Go files);
  - `make lint`;
  - `make test`;
  - `make check-baseline` (triage every goplint finding);
  - `make check-file-length`;
  - `make sonar-local`;
  - the conditional checks: `make check-agent-docs` (the tracker changed) and `cd website && npm run build`.

## Workflow follow-up

- Rebase onto S4 after it merges (decision 31). Then archive this change, and if Part B shipped, sync `tui-server-exposure` and add the new `tui-component-terminal-io` spec.
- `/learn` after the implementation lands.
