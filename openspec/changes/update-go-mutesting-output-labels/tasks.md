## 1. Tool Pin Update

- [ ] 1.1 Re-check the latest `github.com/jonbaldie/go-mutesting/v2` release and confirm `v2.7.1` remains the selected target.
- [ ] 1.2 Update the root Go tool dependency for `github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting` to `v2.7.1`.
- [ ] 1.3 Run root `go mod tidy` and confirm `go mod tidy -diff` is clean.
- [ ] 1.4 Verify the resolved `go-mutesting` binary reports the expected module version.

## 2. Documentation and Guidance

- [ ] 2.1 Update `.agents/rules/version-pinning.md` and any wrapper/version expectation to list `go-mutesting v2.7.1`.
- [ ] 2.2 Search scripts, mutation docs, triage notes, OpenSpec specs, and agent guidance for old `PASS` / `FAIL` label assumptions.
- [ ] 2.3 Update current mutation guidance to use `KILLED` and `ESCAPED` for terminal labels.
- [ ] 2.4 Preserve historical v2.7.0 evidence by clearly marking old `PASS` / `FAIL` descriptions as historical and version-scoped.
- [ ] 2.5 Confirm automation continues to use machine-readable report fields or stable mutant IDs instead of adding terminal-label parsing.

## 3. Verification

- [ ] 3.1 Run `scripts/test_mutation.sh`.
- [ ] 3.2 Run the relevant Make mutation wrapper tests or targets that verify command construction and version checks.
- [ ] 3.3 Run a lightweight mutation dry-run or focused command path that does not require a full mutation scan.
- [ ] 3.4 Run `make lint`.
- [ ] 3.5 Run `make check-agent-docs` if `.agents/` guidance changed.

## 4. Review

- [ ] 4.1 Confirm mutation baselines and survivor counts were not recomputed as part of this label/tool update.
- [ ] 4.2 Record the final tool version, label semantics, and verification output in the implementation summary.
