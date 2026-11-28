# Tasks

## 1. CLI Color Profile & Flag Plumbing

- [x] 1.1 Promote `github.com/charmbracelet/colorprofile` in `go.mod`, add `--ivk-color` (`auto|always|never`) with typed validation (returning `UsageError` exit `2` on invalid values) in `cmd/invowk/root.go`, and update `firstTopLevelToken` (`cmd/invowk/cmd.go`) to skip `--ivk-color <val>` / `--ivk-color=<val>` before `cmd` and `__complete`; verify with `go test -run 'TestColorFlag|TestShouldRegisterDiscoveredCommands' ./cmd/invowk/`
- [x] 1.2 Add dedicated `uiOut`/`uiErr` `colorprofile.Writer` streams on `App` (`cmd/invowk/app.go`) honoring `--ivk-color` > `NO_COLOR`/`CLICOLOR_FORCE`/`TERM=dumb` > TTY detection and wire `fang` help rendering to respect `--ivk-color` (stripping pre-rendered `rootCmd.Long` ANSI when color is off), while leaving raw `App.stdout`/`App.stderr` unwrapped for child `ExecutionContext.IO`, `WatchTerminalOpener`, `clear_screen`, and opt-in `invowk tui style`/`tui format` (unless `--ivk-color=never`); verify with `go test -run 'TestColorProfileWriter|TestWatch' ./cmd/invowk/`

## 2. Glamour Theme Wiring & Quiet Exit Framing

- [x] 2.1 Wire `ui.color_scheme` (`auto|dark|light`) and `"notty"` fallback into `cmd/invowk/service_error.go` and `cmd/invowk/issue_render.go` (replacing hardcoded `"dark"`), and verify with `go test -run TestIssueRender ./cmd/invowk/`
- [x] 2.2 Install a custom `fang.WithErrorHandler` in `cmd/invowk/root.go` that suppresses `ERROR / Exit status N.` unconditionally for bare `*ExitError{Err: nil}` (emitting only an optional single line under `--ivk-verbose`) while keeping `*UsageError` and `*ServiceError` rendering intact; update `tests/cli/testdata/audit_findings.txtar` and verify with `go test -run 'TestCLI/(audit_findings|exit_codes)' ./tests/cli/`

## 3. End-to-End Tests & Documentation

- [x] 3.1 Add testscript and Go unit tests covering non-TTY output, `NO_COLOR=1`, `TERM=dumb`, `--ivk-color=always|never|auto`, `ui.color_scheme`, `tui style` pipe behavior, and quiet non-zero command forwarding; verify with `go test ./cmd/invowk/... ./tests/cli/...`
- [x] 3.2 Document `--ivk-color` and `NO_COLOR`/`ui.color_scheme` interaction in `website/` docs and verify `cd website && npm run build`

## 4. Final Verification Gates

- [x] 4.1 Run all completion gates from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, and `make sonar-local`
