# Tasks

## 1. Shared OSC 7501 Status Helpers (`internal/tui`)

- [x] 1.1 Create `internal/tui/program_status.go` with `app="invowk"` builders, `sanitizeProgramStatusText` (UTF-8, control-char normalization, byte limits `192`/`2048`), `programStatusMeta` (`title`, `password`, `selectable`), and `statusForTUIComponent` mapping
- [x] 1.2 Add unit tests in `internal/tui/program_status_test.go` covering every component/state/kind mapping, control-character sanitization, multi-byte UTF-8 boundary truncation, and verify with `go test ./internal/tui -run TestProgramStatus`

## 2. Interactive Viewer (`-i`) OSC 7501 Lifecycle

- [x] 2.1 Update `internal/tui/interactive_model.go` to track `quitting` and `activeComponentMeta`, populate `View.ProgramStatus` across `!m.ready`, `stateExecuting`, `stateTUI`, `stateCompleted`, and `quitting`, and update `RunInteractiveSession` (`internal/tui/interactive_session.go`) to emit `ansi.ClearProgramStatus` on `tea.ErrProgramKilled`
- [x] 2.2 Add unit tests in `internal/tui/` verifying `View().ProgramStatus` transitions (including `!m.ready` force-quit, overlay metadata, completion `done`/`error`, and dismiss/cancel clearing) and verify with `go test ./internal/tui -run TestInteractive`

## 3. Standalone `invowk tui *` Component OSC 7501 Support

- [x] 3.1 Add `forModal` (`NewSpinModelForModal` in `spin.go`/`embeddable.go`), `title`/`selectable` tracking on `chooseModel`/`filterModel`/`tableModel`, `l.DisableQuitKeybindings()` on list models (`choose_builders.go`, `filter.go`), and `!done && !forModal` `View.ProgramStatus` across `tui.go`, `confirm.go`, `choose.go`, `filter.go`, `table.go`, `pager.go`, and `spin.go` (`spinActionModel` + `spinModel`)
- [x] 3.2 Update `runComponentProgramWith` (`internal/tui/terminal.go`) and `runSpinnerProgram` (`internal/tui/spin.go`) to emit `ansi.ClearProgramStatus` to `terminal.out` on `tea.ErrInterrupted`, `tea.ErrProgramKilled`, or `!IsDone()` exits
- [x] 3.3 Add unit tests for standalone vs modal (`forModal: true`) `ProgramStatus`, `Ctrl+C`/`SIGTERM` clearing, and non-terminal stdout isolation, and verify with `go test ./internal/tui/...`

## 4. Watch Mode (`--ivk-watch`) OSC 7501 Reporting

- [x] 4.1 Implement mutex-guarded `watchStatusReporter` using `ansi.SetProgramStatus` / `ansi.ClearProgramStatus` on TTY targets with an injectable `App`/`Dependencies` seam, and wire it into `runWatchMode` (`cmd/invowk/cmd_watch.go`) across `working`, `idle`, `error`, and `clear`
- [x] 4.2 Add unit tests verifying watch-mode OSC 7501 state sequences, fatal-error exit skip of `error` flash before `clear`, non-TTY silence, and mutex safety, and verify with `go test ./cmd/invowk -run TestWatch`

## 5. Documentation & Agent Guidance

- [x] 5.1 Update EN + pt-BR docs (`website/docs/advanced/interactive-mode.mdx`, `website/docs/tui/overview.mdx`, `website/docs/reference/cli.mdx` and pt-BR mirrors) documenting OSC 7501 Program Status Protocol support, and verify `cd website && npm run docs:parity && npm run build`
- [x] 5.2 Apply durable learnings across `.agents/skills/` and `.agents/rules/` (`.agents/skills/cli/SKILL.md`, `.agents/skills/tmux-testing/SKILL.md`, `.agents/skills/testing/SKILL.md`, `.agents/skills/module-security/SKILL.md`, `.agents/skills/container/SKILL.md`) covering OSC 7577 and OSC 7501 boundaries, and verify `make check-agent-docs`

## 6. Final Verification Gates

- [x] 6.1 Run full completion checklist from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, `make check-windows-build`, `make check-agent-docs`, and `make sonar-local`
