## 1. Repository hygiene

- [x] 1.1 Confirm `tools/goplint/goplint.test` is present in the worktree and not tracked (`git ls-files tools/goplint/goplint.test` returns empty; `ls -la tools/goplint/goplint.test` exists).
- [x] 1.2 Add `tools/goplint/**/*.test` to `.gitignore`, with a defensive explicit line for `tools/goplint/goplint.test` immediately below.
- [x] 1.3 Run `git check-ignore -v tools/goplint/goplint.test` and confirm the new pattern matches.
- [x] 1.4 Run `git status --short` and confirm no `*.test` files are listed under `tools/goplint/`.

## 2. Clean-tree evidence generation command

- [x] 2.1 Inspect `tools/goplint/cmd/clean-tree-evidence/main.go` to determine whether it already exposes a generation mode (subcommand or flag). Record the finding in this task before proceeding. **Finding:** generation is already the command's sole/default mode (not a subcommand or `-generate` flag), with the exact invocation shape `-root`, `-paths`, `-plan`, and `-evidence`.
- [x] 2.2 If a generation mode does not exist, add a `-generate` flag to `cmd/clean-tree-evidence` that produces the `paths`, `plan`, and `run` artifacts under `tools/goplint/testdata/gates/` using the reviewed synthetic-tree procedure documented in `docs/goplint/current-techniques-and-semantics.md:196-207`. If it does exist, adopt its actual invocation shape verbatim. **Finding:** the existing sole/default recorder mode is adopted verbatim; the reviewed `paths` and `plan` remain inputs and only the `run` record is generated.
- [x] 2.3 Add a new Make target `generate-goplint-clean-tree-evidence` that consumes the reviewed `tools/goplint/testdata/gates/clean-tree-v3.paths` and `clean-tree-v3.json` inputs and writes `tools/goplint/testdata/gates/clean-tree-run.v3.json` using the existing recorder invocation.
- [x] 2.4 Ensure missing or stale record verification names the failing artifact and points to `make generate-goplint-clean-tree-evidence`; confirm generation followed by `make check-goplint-clean-tree-evidence` preserves the caller's real Git index and all worktree content except the authorized run-record output. **Finding:** missing-record verification fails naming the artifact and the generation command; a stale record (post-generation tree edits) is rejected via the retained complete-diff SHA-256 census; generation aborts with "clean-tree capture mutated caller index or worktree" when the tree is concurrently modified, and succeeds on an undisturbed tree with verification passing afterwards.
- [x] 2.5 Confirm `make check-goplint-soundness-complete` succeeds end-to-end after a fresh generation. **Finding:** complete profile passed with 19 subgates and 88 observations after fresh generation.

## 3. Mutation kernel category coverage

- [x] 3.1 Read `tools/goplint/spec/semantic-rules.v1.json` and enumerate every registered semantic category into a working list. **Finding:** 42 categories are registered; eight protocol categories require the `mutation` evidence layer.
- [x] 3.2 Read `tools/goplint/testdata/mutation/profiles/blocking-v2.json` and enumerate the selected mutant IDs. **Finding:** the live blocking v2 profile contains 27 selected causal mutants, not the proposal's stale count of 20.
- [x] 3.3 For each selected mutant, read its `changed_stages` / `expected_mismatches` metadata in `tools/goplint/testdata/mutation/soundness-mutants-v2.json` and record the categories it covers.
- [x] 3.4 Cross-check coverage: build a `mutation-required category → [mutant_ids]` map and identify any mutation-required category with an empty list. **Finding:** all eight mutation-required categories have 11–17 covering selected mutants; none is empty.
- [x] 3.5 For every uncovered mutation-required category, promote a covering mutant from `soundness-mutants-v2.json` into `blocking-v2.json`; mutation-required categories cannot be exempted, baselined, excepted, or inline-ignored. **Finding:** no promotion is required because the uncovered set is empty.
- [x] 3.6 Re-verify coverage is complete after any promotions.

## 4. Mutation-kernel-coverage subgate

- [x] 4.1 Add a subgate manifest `tools/goplint/testdata/subgates/mutation-kernel-coverage.v1.json` that binds the subgate to (a) the semantic-rules manifest, (b) the blocking mutation profile, and (c) the mutant catalog; the manifest MUST NOT permit exemptions for mutation-required categories.
- [x] 4.2 Implement subgate execution logic (either as a new `cmd/mutation-kernel-coverage` binary or as a new subcommand of an existing gate binary — follow the convention already used by peer subgates under `tools/goplint/cmd/`).
- [x] 4.3 The subgate MUST emit one structured observation per mutation-required registered category listing the covering mutant IDs, so the aggregate report can validate a non-vacuous outcome.
- [x] 4.4 The subgate MUST fail with a specific error identifying every uncovered category when coverage is incomplete.
- [x] 4.5 Register the new subgate under `tools/goplint/spec/soundness-gate.v1.json:core` alongside the other core subgates.
- [x] 4.6 Add a corresponding entry (target or gate reference) to the Makefile so the subgate is invocable directly for local debugging.

## 5. Documentation updates

- [x] 5.1 Add both `make generate-goplint-clean-tree-evidence` and the new mutation-kernel-coverage subgate command to the Quick Reference table in `.agents/rules/commands.md`, root `AGENTS.md`, and Make help adjacent to their existing peers.
- [x] 5.2 Update `.agents/rules/checklist.md` to show the exact generation command before verification, explain that record generation uses the `core` profile rather than `complete` to avoid recursive freshness verification, and document the blocking mutation-kernel contract.
- [x] 5.3 Update `tools/goplint/AGENTS.md` and `tools/goplint/README.md` to document (a) the generation command, (b) the mutation-kernel-coverage contract, and (c) the fact that uncovered kernel categories and missing/stale completion evidence cannot be baselined, excepted, or inline-ignored.
- [x] 5.4 Confirm the tracked `tools/goplint/CLAUDE.md` compatibility symlink exposes the updated `tools/goplint/AGENTS.md` Quick Reference entries for the generation target and kernel-coverage subgate.
- [x] 5.5 Run `make check-agent-docs` and confirm it passes.

## 6. Verification

- [x] 6.1 Run `make check-goplint-soundness-core` and confirm the new mutation-kernel-coverage subgate participates and passes.
- [x] 6.2 Run `make generate-goplint-clean-tree-evidence` followed by `make check-goplint-soundness-complete` on a clean tree and confirm end-to-end success. **Finding:** end-to-end success on an undisturbed tree; generation must be the last step because any subsequent tree edit (including this checklist update) staleness-invalidates the record, so the record is regenerated after final edits.
- [x] 6.3 Deliberately remove every covering mutant for one category from a temporary edit of `blocking-v2.json`, run `make check-goplint-soundness-core`, and confirm the new subgate fails first with a message identifying that uncovered category. Restore the profile afterwards. **Finding:** stripping the 11 mutants covering `use-before-validate-cross-block` fails the subgate with "uncovered mutation-required semantic categories: use-before-validate-cross-block"; profile restored byte-identical and the subgate passes again.
- [x] 6.4 Deliberately delete the generated `clean-tree-run.v3.json` record, run `make check-goplint-soundness-complete`, and confirm it fails with a message naming the missing record and pointing at the documented generation command. Regenerate the record afterwards. **Finding:** with the record removed, verification fails naming the missing path and instructing "regenerate the retained record with `make generate-goplint-clean-tree-evidence`"; record regenerated afterwards.
- [x] 6.5 Run `make lint`, `make test`, `make check-baseline`, and `make check-goplint-exceptions` and confirm no regressions.

## 7. OpenSpec validation

- [x] 7.1 Run `openspec validate close-goplint-shipping-gaps` and resolve any warnings.
- [x] 7.2 Confirm every spec delta scenario has at least one implementing task in this list.
