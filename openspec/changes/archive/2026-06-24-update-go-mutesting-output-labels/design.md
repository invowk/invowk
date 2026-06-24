## Context

Invowk pins `go-mutesting` as a root Go tool dependency and has a mutation-testing contract covering exact tool versions, profiles, baselines, reports, and wrapper validation. The available `go-mutesting v2.7.1` release changes per-mutant terminal output labels: killed mutants are now labeled `KILLED`, and surviving mutants are now labeled `ESCAPED`. This replaces the confusing `v2.7.0` terminal convention where `PASS` represented killed mutants and `FAIL` represented escaped mutants.

Repository scripts appear to consume machine-readable report files rather than parsing those terminal labels directly, but `tools/mutation/triage/current-full-scan.md` and related guidance explicitly document the old labels. The update should keep historical evidence understandable while making current guidance match the new tool output.

## Goals / Non-Goals

**Goals:**
- Update the pinned `go-mutesting` tool dependency to `v2.7.1`.
- Keep root module tool pinning and version-pinning documentation synchronized.
- Update mutation triage and agent-facing guidance for `KILLED` / `ESCAPED` terminal labels.
- Verify wrapper tests and lightweight command paths without running a full mutation scan.

**Non-Goals:**
- Recompute mutation baselines or survivor counts.
- Change mutation profile selection, changed-line filtering, report file formats, or CI scheduling.
- Interpret old v2.7.0 terminal logs as if they used v2.7.1 labels.
- Make mutation testing part of the regular `make test` or root CI matrix.

## Decisions

### Update the Go tool pin through root module tooling

Use the repository's Go tool dependency policy to update `github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting` to `v2.7.1`. The version-pinning rule and any wrapper expectations should name the same version.

Alternative considered: leave `v2.7.0` pinned to avoid documentation churn. That preserves the confusing label behavior and leaves future triage vulnerable to the same interpretation mistake already documented in the repo.

### Preserve historical notes but label them as historical

Update mutation triage documentation so old `PASS`/`FAIL` explanations are clearly scoped to prior `go-mutesting v2.7.0` runs. Current instructions should use `KILLED` and `ESCAPED`, and any examples of focused reruns should reflect the new output labels.

Alternative considered: rewrite historical notes to use new labels. That would make old evidence easier to read but less faithful to the actual output captured at the time.

### Prefer machine-readable reports over terminal label parsing

Keep scripts and CI gates based on JSON/report artifacts where possible. Terminal labels are useful for humans, but gates should continue using stable report fields and mutant IDs rather than scraping status words from log text.

Alternative considered: add terminal output parsing tests for `KILLED` and `ESCAPED`. That would increase coupling to log formatting without improving baseline or report correctness.

## Risks / Trade-offs

- Historical triage notes may become ambiguous -> Mitigate by adding explicit version context around old `PASS`/`FAIL` language.
- Wrapper tests might accidentally rely on the old label words -> Mitigate by searching scripts and updating only real expectations.
- The tool bump may change report details beyond terminal labels -> Mitigate by running wrapper tests and a lightweight dry-run or focused profile before completion.

## Migration Plan

1. Update the root Go tool dependency to `go-mutesting v2.7.1` and tidy the root module.
2. Update `.agents/rules/version-pinning.md` and any mutation tool version checks.
3. Search mutation scripts, docs, triage notes, and agent guidance for old label assumptions.
4. Update current guidance to `KILLED` / `ESCAPED` while preserving historical v2.7.0 evidence as version-scoped.
5. Run `scripts/test_mutation.sh`, relevant Make mutation wrapper tests, `make lint`, and a lightweight mutation dry-run or focused command that does not require a full scan.

Rollback is to restore the `v2.7.0` tool pin and corresponding documentation if validation shows report or wrapper incompatibility.

## Open Questions

None. Implementation should avoid refreshing mutation baselines unless a separate baseline-maintenance change requests it.
