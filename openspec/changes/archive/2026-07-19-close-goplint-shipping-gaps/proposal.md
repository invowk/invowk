## Why

The current goplint soundness consolidation is architecturally sound and CI-blocking, but three shipping gaps remain: (1) `make check-goplint-soundness-complete` and `make check-goplint-clean-tree-evidence` consume the reviewed `tools/goplint/testdata/gates/clean-tree-v3.paths` and `clean-tree-v3.json` authorities plus the generated `clean-tree-run.v3.json` record, yet the *record-generation* command is undocumented — readers see only the verification target and cannot reproduce a claim; (2) the blocking mutation profile `testdata/mutation/profiles/blocking-v2.json` selects 27 declared causal mutants, but there is no written guarantee that their category metadata spans every semantic category whose rule contract requires the `mutation` evidence layer, so a whole mutation-required category could regress silently; (3) the 35 MB build artifact `tools/goplint/goplint.test` sits in the worktree, one `git add .` away from being committed.

## What Changes

- Document the clean-tree evidence generation command in `.agents/rules/commands.md`, `tools/goplint/AGENTS.md`, and `tools/goplint/README.md` alongside the existing verification command, so a maintainer can produce and verify a completion-proof bundle without reading source.
- Add a written coverage contract for the blocking mutation kernel: `testdata/mutation/profiles/blocking-v2.json` MUST include at least one mutant per semantic category whose `spec/semantic-rules.v1.json` entry requires the `mutation` evidence layer, verified by a new subgate that census-counts kernel mutants by category and fails when any mutation-required category is uncovered.
- Add `tools/goplint/goplint.test` (and the analogous per-directory `*.test` binaries) to `.gitignore`, then confirm no such artifact is tracked.
- Update the `lint-tooling-quality-gates` capability to require the command-documentation surface to include both generation and verification of completion-proof evidence, and to require the mutation-kernel coverage contract to be documented as a blocking gate.
- Update the `goplint-analysis-soundness` capability to codify the mutation-kernel category-coverage requirement and the retained-evidence generation protocol as first-class scenarios.

## Capabilities

### New Capabilities
<!-- None. All work extends existing capabilities. -->

### Modified Capabilities
- `goplint-analysis-soundness`: Add a scenario requiring the blocking mutation kernel to span every mutation-required semantic category, and a scenario requiring the completion-proof record-generation procedure to be documented alongside verification.
- `lint-tooling-quality-gates`: Extend the "Command documentation lists complete lint workflow" scenario so contributors see both generation and verification commands for the completion-proof bundle. Add a scenario requiring the mutation-kernel category-coverage contract to be a blocking subgate documented in the same surface as other blocking gates.

## Impact

- **Docs**: `.agents/rules/commands.md`, `tools/goplint/AGENTS.md`, `tools/goplint/README.md`, `tools/goplint/CLAUDE.md`.
- **Repo hygiene**: `.gitignore` gets `tools/goplint/goplint.test` and `tools/goplint/**/*.test`.
- **Mutation gate**: `tools/goplint/testdata/mutation/profiles/blocking-v2.json` may need additions to cover any uncovered mutation-required category. A new census subgate (in `tools/goplint/testdata/subgates/`) audits kernel coverage.
- **Soundness gate manifest**: `tools/goplint/spec/soundness-gate.v1.json` gains the mutation-kernel-coverage subgate under the `core` profile so kernel drift is caught on every PR.
- **No production code changes**: this is a documentation, gate, and hygiene change. No changes to `cmd/`, `internal/`, or `pkg/`.
- **Backwards compatibility**: adding subgates strengthens the gate; existing PRs will only fail if the mutation kernel is genuinely uncovered.
