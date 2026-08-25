## 1. Pre-Extraction Parameterization (invowk, one semantic-tier PR)

- [ ] 1.1 Thread an explicit repo-root seam through `cmd/soundness-profile`, `cmd/soundness-gate`, `cmd/docs-guard`, `cmd/clean-tree-evidence`, `cmd/check-clean-tree-evidence`, `cmd/repository-audit`, `cmd/race-repeat`, and `cmd/subgate-report` so callers own the root; keep every default equal to current behavior.
- [ ] 1.2 Parameterize docs-guard's Makefile anchor path and governed-document list; parameterize the consumer-smoke scan target and analyzer/baseline/exceptions paths.
- [ ] 1.3 Add or extend tests proving default-path behavior is byte-identical and that explicit roots resolve correctly from a foreign working directory.
- [ ] 1.4 Regenerate the retained v4 completion record in the same PR (`check-goplint-soundness-semantic` → `generate-goplint-clean-tree-evidence` → `check-goplint-clean-tree-evidence` → `check-goplint-soundness-complete`) and verify one `workflow_dispatch` of lint.yml (complete profile) is green on main after squash-merge.

## 2. New-Repo Bootstrap (invowk/goplint, zero invowk changes)

- [ ] 2.1 Extract history: fresh `git clone --no-local`, `git filter-repo --path tools/goplint/ --path-rename tools/goplint/:`; verify the tree builds at the extracted tip.
- [ ] 2.2 Rename the module to `github.com/invowk/goplint` in one signed commit (import rewrite + go.mod); `go build ./... && go test ./...` green; add repo-local `.gitignore` for test binaries.
- [ ] 2.3 Create the public `invowk/goplint` repository and push; add LICENSE (MPL-2.0), `AGENTS.md`, CODEOWNERS, dependabot (`gomod /`, `github-actions`).
- [ ] 2.4 Bindings PR: goplint Makefile (~30 harness targets rooted at `.`), ownership manifest v3 (goplint-repo-relative, three classes, fail-closed to semantic), clean-tree v5 plan + path selection, re-pointed subgate manifest, `.pre-commit-config.yaml`, go-mutesting tool pin, converted docs-guard-governed soundness specs, re-anchored docs-guard defaults.
- [ ] 2.5 CI PR: `lint.yml` (plan/worker/aggregate executor), `ci.yml` (module tests, windows cross-build, license-check, file-length, shellcheck, agent-docs), `goplint-fuzz.yml`, `mutation.yml`, `codeql.yml`, `reference-corpus-certification.yml` with `testdata/corpus/invowk.ref`; hold the `complete`-forcing schedule trigger back until evidence exists.
- [ ] 2.6 Evidence bootstrap PR: run the semantic profile, generate `clean-tree-run.v5.json`, verify with the v5 checker and complete profile, then enable the schedule trigger.
- [ ] 2.7 Enable SonarCloud automatic analysis; apply the Safety-equivalent ruleset (signatures, linear history, squash-only) plus required status check `goplint soundness aggregate`.
- [ ] 2.8 Tag `v0.1.0` (signed) and confirm the module is fetchable via the Go module proxy.

## 3. Invowk Cutover (single atomic squash PR)

- [ ] 3.1 Relocate `tools/goplint/baseline.toml` → `.goplint/baseline.toml`, `tools/goplint/exceptions.toml` → `.goplint/exceptions.toml` (adding `[test_home_env]`), and the consumer-smoke policy → `.goplint/consumer-smoke.github-ubuntu-x64-4cpu.toml`; author `.goplint/consumer-gate.v1.json` and `.goplint/ownership.v1.json`.
- [ ] 3.2 `git rm -r tools/goplint`; add `github.com/invowk/goplint` tool directives + `require` pin `v0.1.0` to root `go.mod`; `make tidy`.
- [ ] 3.3 Rewrite the Makefile: consumer targets re-pointed at the pinned tool and `.goplint/` configs; delete the ~30 harness targets; add `check-goplint-consumer-routed`; new `scripts/goplint.sh`, `scripts/goplint-consumer-smoke.sh`, `scripts/goplint-consumer-routed.sh`.
- [ ] 3.4 Rewrite `.pre-commit-config.yaml` (`goplint-behavior` → consumer routed entry, filter regexes drop `tools/goplint`, add `.goplint/`), `lint.yml` (plan/worker/aggregate → "goplint consumer gates" job, `go-version-file: go.mod`, pruned path filters); delete `goplint-fuzz.yml`; drop the goplint arm from `mutation-testing.yml` and `scripts/mutation.sh`; remove `tools/mutation/goplint-*`.
- [ ] 3.5 Prune `scripts/golangci-lint.sh` two-module dispatch (and its test), `scripts/check-windows-build.sh` steps 3–4, `scripts/check-agent-docs.sh` goplint doc entries, shellcheck list, sonar exclusions, `.gitignore` entries, `scripts/govulncheck-all.sh` test assertions.
- [ ] 3.6 Documentation sweep (~30 files): `.claude/CLAUDE.md`, `.agents/rules/{commands,checklist,version-pinning,testing}.md`, affected skills and `.agents/commands/improve-type-system.md`, `docs/goplint/**` → consumer pointer doc, README tree listing; `make check-agent-docs` green.
- [ ] 3.7 Full verification: `pre-commit run --all-files`; `make lint test check-types check-baseline check-goplint-exceptions check-agent-docs check-file-length test-cli`; post-merge lint.yml + ci.yml green on main; a no-op PR in invowk/goplint stays green.
- [ ] 3.8 Add the invowk required status check "goplint consumer gates" after its first green run on main.

## 4. Cleanup

- [ ] 4.1 Archive this change: sync spec deltas; remove `goplint-analysis-soundness` and `goplint-soundness-assurance` from invowk's spec set (authoritative copies now in invowk/goplint).
- [ ] 4.2 Accept the first dependabot goplint pin-bump PR in invowk as the pin-mechanism smoke test.
- [ ] 4.3 After one clean goplint release, remove the `GOPLINT_FORCE_SEMANTIC` escape hatch in the goplint repository.
- [ ] 4.4 Run `/learn` to capture migration learnings in agent docs and memory.
