# Tasks

## 1. Completion Flags, Validation & Escaping

- [x] 1.1 Add `--alias` and `--prefix` flags to `cmd/invowk/completion.go`, switch `os.Stdout` to `cmd.OutOrStdout()`, upgrade Bash generation to `GenBashCompletionV2` (updating `tests/cli/testdata/completion.txtar`), and implement strict validation (`UsageError` exit `2` on invalid `--alias`, missing `--alias` with `--prefix`, NUL/newline/unclosed-quote `--prefix`); verify with `go test -run TestCompletionValidation ./cmd/invowk/ && go test -run 'TestCLI/completion' ./tests/cli/`

## 2. Per-Shell Script Rewriting & Dynamic Probe Verification

- [x] 2.1 Implement namespace-safe script rewriting and shell-specific token escaping across `bash`, `zsh`, `fish`, and `powershell` in `cmd/invowk/completion.go`, and verify `invowk __complete -c <cfg> --ivk-no-cwd cmd <source> ""` behavior in `cmd/invowk/`; verify with `go test -run TestCompletionAlias ./cmd/invowk/`
- [x] 2.2 Add CLI testscript tests (`tests/cli/`) covering all 4 shells with `--alias` + `--prefix` (including `-c` and `--ivk-no-cwd` scope flags) and live shell syntax/completion checks where available; document the wrapper + completion pattern alongside `--ivk-color` and `passthrough: true` in `website/` (`cd website && npm run build`)

## 3. Final Verification Gates

- [x] 3.1 Run all completion gates from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, and `make sonar-local`
