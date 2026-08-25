# Tasks: Upgrade Toolchain to Go 1.27

## 1. Mechanical Bump

- [x] 1.1 Bump `go 1.26.5` → latest `go 1.27.x` in root `go.mod` and `tools/goplint/go.mod` (lockstep)
- [x] 1.2 Bump `build/bencher/Dockerfile` base image `golang:1.26-bookworm` → `golang:1.27-bookworm` (same commit as 1.1; `GOTOOLCHAIN=local`)
- [x] 1.3 Upgrade `golang.org/x/tools` → v0.49.0 in both modules; upgrade golangci-lint tool directive → v2.13.1 (`go get -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1`) and go-mutesting → v2.8.2
- [x] 1.4 Update `scripts/golangci-lint.sh` `GOLANGCI_LINT_VERSION="v2.12.2"` → `"v2.13.1"`; update the five fixtures in `scripts/test_golangci_lint.sh` (v2.12.2 strings and the `go1.26.4` stub); run `make test-scripts` if present, else the script's own tests
- [x] 1.5 Update `.agents/rules/version-pinning.md` Current pinned versions (golangci-lint v2.13.1, go-mutesting v2.8.2) and the `golang:1.26` exemplar → `golang:1.27`
- [x] 1.6 Run `make tidy` on both modules; accept `go 1.27` require-block merging in the diff
- [x] 1.7 Run `make lint`; fix all new findings from golangci-lint v2.13.x and modernize's retarget to Go 1.27 idioms (both `.golangci.toml` files defer lang version to go.mod)
- [x] 1.8 Grep test suites for assertions on `encoding/json` error text (v1 is now json/v2-backed); fix any brittle exact-string assertions; run `make test` and triage failures
- [x] 1.9 Fork golua for Go 1.27 linkname removal (user-approved): create `github.com/invowk/golua` v0.3.0 from arnodel/golua v0.2.0 with `runtime/hash.go` on `hash/maphash.Comparable`, module path rewrite, REPL cmd dropped; switch invowk imports (`internal/runtime/lua.go`, `lua_io.go`) and `go.mod` to the fork

## 2. Goplint Artifact Regeneration

- [x] 2.1 Update `go_toolchain = "go1.26"` → `"go1.27"` and re-derive threshold values on the new toolchain in `tools/goplint/bench/thresholds.toml`, `thresholds.github-ubuntu-x64-4cpu.toml`, `consumer-smoke.github-ubuntu-x64-4cpu.toml`
- [x] 2.2 Regenerate the race-repeat timing census: `make update-goplint-race-repeat-timings` (writes `tools/goplint/spec/goplint-test-timings.v1.json` with exact `go1.27.x` toolchain)
- [x] 2.3 Update `go1.26.x` literals in test fixtures: `benchmarkpolicy/policy_test.go`, `racerepeat/model_test.go`, `soundnessgate/plan_test.go`, `soundnessgate/plan_model_test.go` (keep the deliberate-mismatch case a mismatch), `testdata/gates/clean-tree-run.v4.json` banners
- [x] 2.4 Run `make mutation-dry-run` and `make check-goplint-targeted-mutation` to verify go-mutesting v2.8.2 behavior on the new toolchain
- [x] 2.5 Run the goplint module test suite and executor parity: `make check-goplint-module-tests`, `make check-goplint-harness-parity`

## 3. Generic-Method Fixture (Soundness Obligation)

- [x] 3.1 Add goplint testdata: a value type declaring a Go 1.27 generic method (type-parameter list on a method); assert the analyzer's classification in a test
- [x] 3.2 If the analyzer hard-errors or silently misclassifies the fixture, stop and triage with the user (fix in goplint vs. defer) per the checklist's goplint-triage rule; the pinned outcome must be fail-closed
- [x] 3.3 Run `make check-semantic-spec` to confirm catalog/oracle integrity is unaffected

## 4. Feature Adoption: json/v2 Strict Decoding

- [x] 4.1 Convert goplint JSON record readers (clean-tree evidence, execution plans, timing census, baselines, subgate census, mutation contracts — the readers under `tools/goplint/internal/*` and `tools/goplint/goplint/*` using `json.Unmarshal`/`json.NewDecoder`) to `encoding/json/v2` with strict defaults (duplicate-key and invalid-UTF-8 rejection)
- [x] 4.2 Add round-trip tests per converted reader: records written by the corresponding producer decode successfully; field-name case-sensitivity covered; a fixture with a duplicate key is rejected with an actionable error
- [x] 4.3 Evaluate converting the interactive-protocol decode path (`internal/app/commandadapters/interactive_protocol.go`) to json/v2 for performance; convert only with casing verified against actual message producers and a round-trip test; leave `internal/auditllm`/`internal/agentcmd` LLM-response parsing on lenient v1 semantics (document why in code where converted paths sit adjacent)
- [x] 4.4 Re-run the full goplint gate suite after conversion to confirm freshly regenerated records decode under strict readers

## 5. Feature Adoption: Goroutine-Leak Assertions

- [x] 5.1 Add zero-leak assertions to serverbase state-machine lifecycle tests using the GA `goroutineleak` profile (assert no goroutines attributable to the server remain after terminal stop; avoid `t.Parallel()` where attribution is ambiguous)
- [x] 5.2 Extend sshserver and tuiserver lifecycle tests with the same assertion pattern
- [x] 5.3 Fix or triage with the user any pre-existing leaks the assertions surface (no exclusion lists, no baselines)

## 6. Feature Adoption: 1.27 Idiom Pass

- [x] 6.1 Review the ~11 production `strings.LastIndex`/`bytes.LastIndex` slicing sites; rewrite with `CutLast` where it simplifies; accept modernize's automatic 1.27 rewrites

## 7. Performance Re-Certification

- [x] 7.1 Regenerate PGO profile on Go 1.27: `make pgo-profile-parse-discovery` (or full `make pgo-profile`); commit `default.pgo`; run `make pgo-audit`
- [x] 7.2 Run `make bench-report` and compare against prior snapshot; review Bencher thresholds for shifts from size-specialized malloc and json/v2 decoding
- [x] 7.3 Run `make check-goplint-benchmarks` (five-sample certification) against the re-derived threshold manifests

## 8. Documentation Sweep

- [x] 8.1 Update Go prerequisite prose: `README.md` (lines ~92, ~3286), `website/docs/getting-started/installation.mdx` + `website/i18n/pt-BR/.../installation.mdx` (parity), `AGENTS.md` + `.claude/CLAUDE.md` ("Go 1.26+" → "Go 1.27+"), `.agents/rules/commands.md` prerequisites, `.github/ISSUE_TEMPLATE/bug_report.yml` placeholder
- [x] 8.2 Update version-bearing examples: `website/src/components/Snippet/data/dependencies.ts` (grep patterns, `go-1.26`/`go-1.27` alternative names, numeric `-ge 26` threshold, "keep it simple" example, stale `[6-9]` regex comments), `README.md` regex comment (~line 781)
- [x] 8.3 Update `pkg/invowkfile/invowkfile_schema.cue` example comment `golang:1.26` → `golang:1.27` together with its paired fixture in `pkg/invowkfile/sync_runtime_behavioral_test.go`; run `/schema-sync-check` behavioral sync tests
- [x] 8.4 Update `golang:1.26` exemplars in agent docs: `AGENTS.md`/`.claude/CLAUDE.md` container policy, `.agents/skills/review-docs/SKILL.md` + its references, `.agents/skills/go/SKILL.md` modernize note ("Go 1.26 forms" → "Go 1.27 forms"); update `docs/goplint/soundness-gate-performance.md` toolchain table
- [x] 8.5 Update golua references to `github.com/invowk/golua` fork: `README.md` (~line 3318 key-deps list), `docs/architecture/c4-component-runtime.md`, `website/docs/architecture/c4-component-runtime.mdx` + pt-BR current i18n counterpart (NOT frozen versioned docs), and any `.agents/skills/virtual-lua` module-path mentions
- [x] 8.6 Run parity gates: `node scripts/validate-docs-parity.mjs`, `make check-agent-docs`, `cd website && npm run build`

## 9. Verification & Completion

- [x] 9.1 Full pre-completion sequence: `make tidy`, `make license-check`, `make lint`, `make test`, `make test-cli`, `make check-baseline`, `make check-file-length`, `make sonar-local`
- [x] 9.2 Triage any newly surfaced goplint findings with the user (fix location: production code vs. goplint) before closing
- [x] 9.3 Soundness completion sequence: `make check-goplint-soundness-semantic` → `make generate-goplint-clean-tree-evidence` (full regeneration, not rebind) → `make check-goplint-clean-tree-evidence` → `make check-goplint-soundness-complete`
- [x] 9.4 Run `/learn` to capture toolchain-upgrade learnings (new pin locations, gate behavior on toolchain moves)
