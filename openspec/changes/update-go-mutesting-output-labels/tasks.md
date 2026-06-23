## 1. Tool Pin Update

- [x] 1.1 Re-check the latest `github.com/jonbaldie/go-mutesting/v2` release and confirm `v2.7.1` remains the selected target.
- [x] 1.2 Update the root Go tool dependency for `github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting` to `v2.7.1`.
- [x] 1.3 Run root `go mod tidy` and confirm `go mod tidy -diff` is clean.
- [x] 1.4 Verify the resolved `go-mutesting` binary reports the expected module version.

## 2. Documentation and Guidance

- [x] 2.1 Update `.agents/rules/version-pinning.md` and any wrapper/version expectation to list `go-mutesting v2.7.1`.
- [x] 2.2 Search scripts, mutation docs, triage notes, OpenSpec specs, and agent guidance for old `PASS` / `FAIL` label assumptions.
- [x] 2.3 Update current mutation guidance to use `KILLED` and `ESCAPED` for terminal labels.
- [x] 2.4 Preserve historical v2.7.0 evidence by clearly marking old `PASS` / `FAIL` descriptions as historical and version-scoped.
- [x] 2.5 Confirm automation continues to use machine-readable report fields or stable mutant IDs instead of adding terminal-label parsing.

## 3. Verification

- [x] 3.1 Run `scripts/test_mutation.sh`.
- [x] 3.2 Run the relevant Make mutation wrapper tests or targets that verify command construction and version checks.
- [x] 3.3 Run a lightweight mutation dry-run or focused command path that does not require a full mutation scan.
- [x] 3.4 Run `make lint`.
- [x] 3.5 Run `make check-agent-docs` if `.agents/` guidance changed.

## 4. Review

- [x] 4.1 Confirm mutation baselines and survivor counts were not recomputed as part of this label/tool update.
- [x] 4.2 Record the final tool version, label semantics, and verification output in the implementation summary.
