## Why

The formal models are trusted only as far as their bindings (golden replays, rapid properties, and failure-injection tests) constrain the Go code named in each model's correspondence table. Today that is measured only by hand: each model's `calibration` record lists a few seeded defects. The existing mutation profiles cannot measure it:
- `tools/mutation/root-packages.txt` omits `internal/app/modulesync` and `internal/sshserver`;
- the full profile runs every package test under `-short`, so an unrelated unit test can kill a mutant the bindings would miss;
- go-mutesting's built-in executor runs only the mutated package's tests, so cross-package bindings (for example `CommandScope.CanCallTarget` in `pkg/invowkmod`, bound by `TestScopeConstructionGoldenVectors` in `internal/app/deps`) never get credit.

A mutant that survives only its function's binding tests marks code the model claims to describe but the bindings do not pin down. That is the drift the adopt-formal-verification design is meant to control.

## What Changes

- **Landing order.** This change lands after `formal-infra-refinements` and after the three model changes (`trace-validate-token-and-lock`, `model-concurrent-lock-writes`, `model-module-path-containment`), and before `promote-formal-ci-gate`. Its targets are whatever the correspondence tables name at that point, including the bindings those changes add. It archives after `adopt-formal-verification`.
- **Target set and Make targets.** A new `formal-bindings` mutation target set in `scripts/mutation.sh` (`--target-set`, `MUTATION_TARGET_SET`) is available on the existing `dry-run`, `full`, `baseline-update`, and `rerun` profiles; `pr` rejects it. Make targets: `mutation-formal-dry-run`, `mutation-formal`, `mutation-formal-baseline-update`, and `mutation-formal-rerun`.
- **Generated plan.** A new `scripts/formal_mutation.py plan` builds the plan at run time from the correspondence tables and `formal/manifest.toml`, so no hand-kept list can drift. For each named function the plan lists its line range, its rows, its killer tests, and the packages that own them.
  - Trace harnesses and characterisation (finding-replay) tests are never killers.
  - Rows without a killer are reported as unbound, with a reason.
  - The plan fails closed on a match collision, an unlocatable test, or an empty plan.
- **Clean-code pre-flight.** go-mutesting ignores `--noop` with `--exec`. The wrapper therefore runs every file's killer tests on clean code first. Any failure, skip, or timeout fails the run. Per-file exec timeouts are derived from the measured clean time.
- **Per-function execution.** A custom exec script (`scripts/mutation-formal-exec.sh`) finds the function that contains the mutated line and applies the mutant through `go test -overlay`, so the worktree is never rewritten. It runs only that function's killer tests, across packages, with `-count=1` and no `-short`, so golden vectors replay in full. Trace validation and TLC are excluded.
- **Determinism.** The exec script refuses to run unless `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME` are set. Focused reruns append evidence to a tracked JSONL file, and a survivor needs two recorded escapes before it can be baselined.
- **Baseline and triage.** Accepted survivors go in a separate baseline, `tools/mutation/baselines/formal-bindings-baseline.json`. A triage ledger, `tools/mutation/triage/formal-bindings.toml`, classifies each by model and element as `equivalent`, `abstraction`, `binding-gap`, `model-gap`, or `defect`. `formal_mutation.py triage --check` runs in `make test-scripts` and during baseline updates.
- **Feedback loop.** A closed gap leads to a stronger binding or a new model property, with the mutant ID recorded in the model's calibration. A deferred gap needs a follow-up. A defect is recorded as a finding under the next free id, with no product fix in this change.
- **Correspondence extensions** in `scripts/formal.py`:
  - multi-test binding cells;
  - a completeness guard over `*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go`, and `*_trace_test.go`;
  - a characterisation-test marking convention.
- **Advisory status.** The profile stays manual and advisory. The dispatch-only workflow gains a `target_set` choice. The profile is not a PR gate and is not part of `make test`. Sharding is deferred.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `mutation-testing`: new requirements for the formal-bindings target set:
  - generated targets;
  - a clean-code pre-flight;
  - per-function binding-only cross-package execution;
  - determinism with rerun evidence;
  - a separate baseline with a triage ledger;
  - advisory-only status.
- `formal-verification-toolchain` (introduced by the unarchived `adopt-formal-verification` change; this change archives after it): new requirements for multi-test binding cells (which supersede the singular "binding test" column wording), the completeness guard, characterisation-test marking, and survivor feedback into bindings, models, and findings.

## Impact

- **Scripts:**
  - new: `scripts/formal_mutation.py`, `scripts/test_formal_mutation.py`, `scripts/mutation-formal-exec.sh`;
  - changed: `scripts/mutation.sh` (target set, usage text) and `scripts/test_mutation.sh`;
  - changed: `scripts/formal.py` and `scripts/test_formal.py` (the correspondence extensions only).
- **Build and CI:** `Makefile` (targets, help and env-var lines, `test-scripts`, `lint-scripts`) and `.github/workflows/mutation-testing.yml`.
- **Formal sources:** the correspondence tables in `formal/alloy/*.als` and `formal/tla/*.tla`, and `formal/manifest.toml` (calibration text; for model gaps and defects, new property, mutant, and `finding` commands).
- **Tests:** binding test files (`*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go`) when gaps are closed. These are test-only changes; no production Go code changes.
- **New tools files:** `tools/mutation/baselines/formal-bindings-baseline.json`, `tools/mutation/triage/formal-bindings.toml`, and `tools/mutation/triage/formal-bindings-reruns.jsonl`.
- **Docs:** `.agents/rules/commands.md`, `.agents/skills/formal-verification/SKILL.md`, and `formal/README.md`.
- **Dependencies:** no new Go, Python, or npm dependencies; go-mutesting stays pinned at v2.8.3.
- **Runtime:** at HEAD 4831f23d there are 519 candidate mutants in 14 files, about 10–25 min on the one worker `--exec` allows, plus timeout-bound mutants. The sibling changes add rows and costlier golden replays, so the figures are re-measured before the first run (design.md, Risks).
- **Overlap with the five sibling changes:**
  - `formal-infra-refinements` rewrites how `scripts/formal.py` handles Alloy sources, the golden format, and the fingerprint;
  - `trace-validate-token-and-lock` adds trace-harness bindings and rows;
  - `model-concurrent-lock-writes` and `model-module-path-containment` add models, bound rows, and characterisation tests (F8–F10, F13 and later);
  - `promote-formal-ci-gate` adds `scripts/formal.py` subcommands and `[ci] paths`. This change keeps its triage check out of `make formal`, so those paths need no additions.
