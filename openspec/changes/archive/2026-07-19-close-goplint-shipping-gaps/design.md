## Context

The goplint soundness consolidation is CI-blocking and architecturally sound (real IFDS/IDE, independent reference oracle, causal mutation kill contract, byte-determinism harness, retained exact-tree completion proof). Three gaps remain before the whole system is safely shippable:

1. `make check-goplint-soundness-complete` and `make check-goplint-clean-tree-evidence` consume the reviewed `tools/goplint/testdata/gates/clean-tree-v3.paths` and `clean-tree-v3.json` authorities plus the generated `clean-tree-run.v3.json` record. Record generation must invoke the `core` profile to avoid a recursive freshness check, but the generation command is undocumented. A reader sees only the verifier and cannot reproduce a claim.
2. The blocking mutation profile at `tools/goplint/testdata/mutation/profiles/blocking-v2.json` selects 27 mutant IDs from `testdata/mutation/soundness-mutants-v2.json`. There is no written contract that the selected mutants span every semantic category whose rule contract requires the `mutation` evidence layer. A mutation-required category with zero blocking mutants could regress silently.
3. `tools/goplint/goplint.test` is a 35 MB build artifact sitting in the worktree. Not tracked by git, but a single `git add .` would commit it.

The soundness spec explicitly forbids conversion of exhausted analysis into safety. This change extends the same discipline to the shipping surface: gate documentation and gate coverage must be verifiable from the documented workflow alone.

## Goals / Non-Goals

**Goals:**
- A reader following only `.agents/rules/commands.md` can generate a completion-proof bundle, verify it, and understand what the bundle proves.
- Every semantic category whose `spec/semantic-rules.v1.json` entry requires the `mutation` evidence layer is guaranteed to have at least one causal-kill mutant in the blocking profile on every PR.
- `tools/goplint/**/*.test` binaries cannot be accidentally committed.
- The new kernel-coverage subgate integrates into the existing manifest-driven `soundness-gate.v1.json` under the `core` profile so no new bespoke gate wiring is required.

**Non-Goals:**
- Rewriting or expanding the mutation profile beyond the minimum needed to satisfy mutation-required category coverage. Adding new mutants is not the goal; documenting and enforcing that the existing blocking kernel spans every mutation-required category is.
- Changing the analyzer, the IFDS/IDE machinery, or the reference oracle.
- Introducing a new evidence artifact format. The v3 clean-tree schema stays as-is; only the generation workflow becomes documented and repeatable.
- Making the `complete` profile part of ordinary CI. The design correctly separates the on-demand completion claim from routine PR gating.

## Decisions

### Kernel-coverage subgate

**Decision:** Add a new subgate `mutation-kernel-coverage` under `soundness-gate.v1.json:core` that reads `spec/semantic-rules.v1.json` to enumerate categories requiring the `mutation` evidence layer, reads the blocking mutation profile and mutant catalog to enumerate selected causal mutants and their category/stage/mismatch metadata, and asserts every mutation-required category has at least one covering mutant. It emits a structured observation for each category-to-mutant mapping so the aggregate report can validate a non-vacuous outcome.

**Alternatives considered:**
- Compute coverage at profile-load time in `cmd/targeted-mutation`. Rejected: hides the coverage signal inside mutation execution logs. A separate subgate makes coverage a first-class observation visible in the aggregate soundness report, and is checkable without running the mutations themselves (which are expensive).
- Require categories to be listed inline in `blocking-v2.json`. Rejected: duplicates data already in `soundness-mutants-v2.json`. The mutant declarations already carry `changed_stages`; the subgate reads that transitively.

**Rationale:** The subgate is fast (parses three JSON files, no analyzer or test run), fits the existing anti-vacuous-gate discipline, and cannot be baselined, excepted, or inline-ignored by the existing checklist rule that forbids suppressing proof uncertainty.

### Documented generation command

**Decision:** Add a `make generate-goplint-clean-tree-evidence` target that invokes the existing `cmd/clean-tree-evidence` binary with the reviewed paths file. Document it in `.agents/rules/commands.md`'s Quick Reference table adjacent to `make check-goplint-clean-tree-evidence`, and add a paragraph to the checklist explaining that generation must precede verification for completion claims.

**Alternatives considered:**
- Ship the v3 bundle in-tree at HEAD. Rejected: violates the recursive-freshness constraint documented in `docs/goplint/current-techniques-and-semantics.md:196-207`. The bundle is bound to a specific tree and staleness is the failure mode the verifier exists to prevent.
- Generate on demand inside `check-goplint-soundness-complete`. Rejected: same recursion concern — `complete` invoking generation would make the freshness check depend on its own output.

**Rationale:** A separate documented generation command preserves the design's separation between "produce an artifact bound to this tree" (generation) and "verify an existing artifact matches this tree" (verification), while making both discoverable from the same command reference.

### `.gitignore` scope

**Decision:** Add both a broad pattern `tools/goplint/**/*.test` and, defensively, the exact file `tools/goplint/goplint.test`. Verify by running `git check-ignore` on both.

**Alternatives considered:**
- Rely on a top-level `*.test` pattern. Rejected: the repo does have other `_test.go` outputs in nested modules and a repo-wide `*.test` would silently ignore artifacts a maintainer might want to inspect. Scope the ignore to `tools/goplint`.

## Risks / Trade-offs

- **[Risk]** Kernel-coverage subgate could produce false negatives if category or `changed_stages` metadata on a mutant is incomplete or wrong. **Mitigation:** The subgate emits per-category observations that include the covering mutant IDs and rejects selected mutants without causal stage/mismatch metadata, so aggregate validation fails on missing coverage and human review can catch mislabeled metadata. `soundness-mutants-v2.json` already binds each mutant to source anchors, hashes, and assertion IDs, giving reviewers a concrete cross-check.
- **[Risk]** The documented generation command drifts from the actual `cmd/clean-tree-evidence` invocation. **Mitigation:** The command is a single Make target with a stable script. Any signature change is a visible diff to the Makefile and to the commands doc, caught by `make check-agent-docs`.
- **[Trade-off]** Adding the kernel-coverage subgate to the `core` profile adds one JSON-parse subgate to every PR. Cost is negligible (single-digit milliseconds) compared with the safety improvement.
- **[Risk]** Broadening `.gitignore` to `tools/goplint/**/*.test` could mask a legitimate `*.test` file a future contributor wants to track. **Mitigation:** The convention that `*.test` files are Go build artifacts is stable across the Go toolchain. If a real need arises to track a `*.test` file, a per-path negation entry can be added later.

## Migration Plan

No migration required. This change is additive:
- New Make target for generation.
- New subgate registered under the existing `core` profile.
- Documentation updates only touch surfaces that already describe adjacent gates.
- `.gitignore` addition is a no-op for anyone whose `goplint.test` was already untracked.

Rollback is deletion of the new subgate manifest entry, the new Make target, the new doc sections, and the new `.gitignore` lines. The retained-evidence workflow returns to the pre-change state where generation is implicit and undocumented.

## Open Questions

- The existing `cmd/clean-tree-evidence` command is the recorder's sole generation mode and accepts `-root`, `-paths`, `-plan`, and `-evidence`; the Make target adopts that invocation without adding a redundant mode flag.
- Should the kernel-coverage subgate be added to the `complete` profile as well, or only `core`? Design assumes `core` is sufficient because `complete` = `core` + `clean-tree-freshness`, so kernel coverage flows in transitively. Confirm during implementation.
- Categories that do not register the `mutation` evidence layer are outside this subgate's population by contract; they remain blocking through their declared rule-contract, owner-route, oracle, and artifact-parity layers. Mutation-required categories cannot be exempted.
