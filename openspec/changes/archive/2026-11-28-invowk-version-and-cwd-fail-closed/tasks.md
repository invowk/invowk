# Tasks

## 1. Schema, Build Version & Pre-Pass

- [x] 1.1 Add `invowk_version` (`SemVerConstraint`) to `#Invowkmod` (`pkg/invowkmod/invowkmod_schema.cue`), `#Invowkfile` (`pkg/invowkfile/invowkfile_schema.cue`), and Go structs, and verify with `go test -run TestSync ./pkg/invowkmod/ ./pkg/invowkfile/`
- [x] 1.2 Implement shared build-version resolution (ldflags + `debug.ReadBuildInfo().Main.Version` before `(devel)` fallback), schema-less CUE `invowk_version` pre-pass before `ParseAndDecodeCompiled`, and `field not allowed` upgrade hint in `pkg/invowkmod/parse.go` and `pkg/invowkfile/parse.go`; verify with `go test -run 'Test.*InvowkVersion|TestParse' ./pkg/invowkmod/ ./pkg/invowkfile/`

## 2. Scoped Fail-Closed CWD Discovery

- [x] 2.1 Tag current-directory parse/load diagnostics/errors in `internal/discovery/` (keeping `DiscoverCommandSet`/`DiscoverAndValidateCommandSet` non-aborting so `invowk validate` keeps reporting all workspace errors) and fail closed immediately in `invowk cmd` / `commandsvc` when `--ivk-no-cwd` is off (preventing global fallback and surfacing the real error with exit `1`), while keeping resilient skip + `SkippedSourcesError` for non-cwd sources and keeping `--ivk-no-cwd` skipping cwd completely; verify with `go test ./internal/discovery/... ./internal/app/commandsvc/... ./cmd/invowk/...`
- [x] 2.2 Migrate `tests/cli/testdata/{virtual_diagnostics_footer,virtual_verbose,native_verbose,virtual_not_found_skipped,native_not_found_skipped}.txtar` to place `broken.invowkmod` in non-cwd sources (`~/.invowk/cmds` / `includes`) where skipped-source handling applies, add new CLI testscript cases for `invowk_version` mismatch, `(devel)` warning, `field not allowed` hint, broken cwd vs global shadowing, and `--ivk-no-cwd` bypass, and update `website/` schema & CLI docs; verify with `go test ./tests/cli/... && cd website && npm run build`

## 3. Final Verification Gates

- [x] 3.1 Run all completion gates from `.agents/rules/checklist.md`: `make tidy`, `make license-check`, `make lint`, `make test`, `make check-baseline`, `make check-file-length`, `make check-goplint-consumer-routed`, and `make sonar-local`
