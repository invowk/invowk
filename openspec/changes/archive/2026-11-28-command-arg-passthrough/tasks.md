# Tasks

## 1. CUE Schema, Model & Tree Validation

- [x] 1.1 Add `passthrough?: bool` to `#Command` in `pkg/invowkfile/invowkfile_schema.cue`, `Passthrough bool` (`json:"passthrough,omitempty"`) to `pkg/invowkfile/command.go`, mutual-exclusivity checks (`flags`, `args`) in CUE/`Command.Validate()`, and subcommand conflict checks in `pkg/invowkfile/command_tree.go`; verify with `go test -run 'TestSync|TestValidateCommandTree|TestCommand' ./pkg/invowkfile/`

## 2. Hardened CLI Routing & Flag Promotion

- [x] 2.1 Promote `--ivk-workdir`, `--ivk-env-var`, `--ivk-env-file`, `--ivk-env-inherit-mode`, `--ivk-env-inherit-allow`, and `--ivk-env-inherit-deny` to `cmdCmd.PersistentFlags()` in `cmd/invowk/cmd.go` (removing duplicate leaf registration in `cmd/invowk/cmd_discovery.go`) and verify zero flag regressions with `go test ./cmd/invowk/...`
- [x] 2.2 Set `DisableFlagParsing: true` on passthrough Cobra leaves (`cmd/invowk/cmd_discovery.go`), add `[passthrough]` annotation in `listCommands`, and implement non-mutating pre-parse boundary splitting for `@source`, `--ivk-from`, module-qualified `<source> <cmd>`, and ambiguous group nodes in `cmd/invowk/cmd.go` so `--help` and unknown flags reach the script verbatim; verify with `go test -run 'Test.*Passthrough' ./cmd/invowk/`

## 3. Cross-Runtime Verification & Documentation

- [x] 3.1 Verify verbatim forwarding (`--help`, `-h`, `-v`, `--foo=bar`, `ARGn`/`ARGC`) across `native`, `virtual-sh`, `virtual-lua`, and `container` via `tests/cli/` testscript tests (plus native mirrors per `native-mirror` skill) and verify with `go test -run 'TestCLI/.*passthrough' ./tests/cli/`
- [x] 3.2 Document `passthrough: true`, pre-command `--ivk-*` rule, `invowk help cmd <name>`, Windows `cmd.exe` caveat, and `--help` execution semantics in `website/` docs (`cd website && npm run build`)

## 4. Final Verification Gates

- [x] 4.1 Run all completion gates from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, `make check-goplint-consumer-routed`, and `make sonar-local`
