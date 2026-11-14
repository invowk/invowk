## Why

The formal setup fails closed everywhere except in two instruments, and both let a model look bound to code that nothing checks.

**A correspondence row can name a binding that never runs its function.**
- The correspondence check only proves that the symbol is declared in the named file. The first formal-bindings mutation run (809 mutants, 419 baselined survivors) showed that every mutant of such a row escapes.
- A coverage prototype, re-run on the rebased tree (base `c0a000da`, 2026-11-14), runs each killer test alone under `-coverpkg`. It finds **eight** bound rows whose binding executes none of the function's statements:
  - the six already known: `validateDestinationPath`, `LoadEnvFile`, `newVirtualPathResolverForFilesystem`, `standardVirtualAnchorsForOS`, `CustomCheckScript.ResolveWithFSAndModule`, and `Invowkfile.GetEffectiveWorkDir`;
  - `VirtualFilesystemConfig.EffectiveAccess`, listed in `tasks/next/formal-binding-gaps.md`;
  - `Server.ValidateToken`, which is new. `TestHostCallbackToken_LifecycleMatchesModel` authenticates through `admitConn`, and `ValidateToken` has no production caller.
- Two of these rows name a function that does not carry the model's claim, so no honest binding can reach them:
  - `ValidateToken` has no caller, and the model's `Auth` action is `admitConn`;
  - the model's unpack check is `normalizeZIPPath` followed by `validateDestinationPath`, and `validateDestinationPath` alone disagrees with the model on absolute names.
- Related: 34 of the 102 rows name a "model element" that does not exist in the model, among them `unpackRejectsEscape`.

**A platform-only leg can skip on the one platform it exists for.**
- `TestModulePathContainment_F16Junction` skipped on windows-latest (run 36202690168, job 108292507012) because `fstree.Probe().Junction` was false. `probeJunction` throws the creation error away, so the cause is unknown.
- Even where it runs, the leg only logs a verdict.
- No other Go test generates `KindJunction`, although `formal/README.md` says junctions are covered by "the witness commands, the rapid properties, and the Windows-only F16 leg". Only the Alloy witnesses exist.
- F16 has never been decided on any platform.

## What Changes

1. **Reach gate.** A new `scripts/formal_mutation.py reach` subcommand and a `make formal-reach` target reuse the formal-bindings plan: each function's file, line range, rows, and per-row killers. They run each killer test alone with `-coverpkg`/`-coverprofile`. The gate fails unless every killer of every bound row covers at least one statement inside its function's line range.
   - **Fail-closed cases:**
     - a missing or unparsable profile;
     - a killer that fails, skips, or is not reported;
     - a function file absent from the profile;
     - a function range with no coverage blocks.
   - **Exempt rows:** unbound rows (`no-symbol`, `no-binding`, `type-symbol`, `trace-only`, `characterisation-only`) are reported with their abstraction note, not gated.
   - **Wiring:** a new `Binding reach` step in the formal CI lane, with a soft budget in `[reach]` and a hard step timeout.
   - **Calibration:** on the pre-change tree the gate fails and names exactly the eight rows above. It passes after item 4.
2. **No platform skip on its own platform.** `internal/testutil/fstree` gains a capability policy per GOOS:
   - junctions are required on Windows;
   - symlinks are required on Linux and macOS, and on Windows under GitHub Actions only (`GITHUB_ACTIONS=true`).

   The policy works as follows:
   - `Probe` keeps each probe's error;
   - a required capability that is unavailable fails the test with that error, in the F16 leg, in `Cache.Get`, and in the rapid draws;
   - binding-test files may skip for a capability only through the fstree helpers, and `formal.py correspondence` rejects raw `Skip` calls there;
   - junction creation failures are investigated in a fixed order: surface the error, compare `writeMountPointReparse` with Go's own Windows mount-point test helper, then fall back to `mklink /J`.
3. **Junctions decide F16.** The `ModulePathContainment` rapid property draws `KindJunction` links on Windows.
   - Junction escapes are judged by file identity or content, never by `filepath.EvalSymlinks`, which Go 1.23+ may not apply to mount points.
   - The F16 leg asserts instead of logging. It covers `Load` admission, the script read, both copies, and the module hash.
   - The golden replay keeps junctions fixed off.

   Windows CI on the PR decides one of three outcomes, each stated in advance:
   - (a) **Creation still fails:** the leg and the job fail, and investigation continues. The change does not merge while the leg is skipped.
   - (b) **Escape:** F16 becomes a confirmed finding through the established procedure. That means a manifest `finding = "F16"` command next to a passing fix configuration, a README Findings row, a characterisation replay, and a property carve-out that mirrors F13. It is not fixed here.
   - (c) **Contained:** the F16 hypothesis is refuted. README records it, naming the modelled fact that keeps each operation contained and the mutant that drops it. The leg stays and asserts containment.
4. **Resolve the eight rows** without touching production code:
   - `GetEffectiveWorkDir` → `TestModulePathContainment_ContainerWorkDirGolden`, which already reaches it.
   - `CustomCheckScript.ResolveWithFSAndModule` and `LoadEnvFile` → real-function replays inside `TestModulePathContainment_GoldenVectors`, compared with the model's current-code verdicts. Those verdicts include the open findings F13 and F17.
   - The three `VirtualPathHarness` rows → a new construction-layer binding, `TestVirtualPathHarness_RootConstruction`.
   - `Auth` / `ValidateToken` and `unpackRejectsEscape` / `validateDestinationPath` → explicitly unbound (binding `-`), each with the reason in its abstraction cell. Re-pointing them (`Auth` to `admitConn`, `unpackInside` to `normalizeZIPPath`) belongs to the survivor-triage follow-up.
   - The README junction coverage claim is corrected.

   A strengthened binding that fails on unmodified code, unless an open finding explains it, is a defect. It gets F18, the next free id, through the finding procedure. The implementer stops and reports, and never weakens the assertion or patches product code.
5. **Model elements must exist.** `formal.py correspondence` checks that each row's model element is named in the model source or in the model's manifest commands. How today's 34 unresolved rows are handled is pending a maintainer decision (design Open question 1). The recommendation is a fail-closed, shrink-only list in the manifest.
6. **Windows replay budget.** The 30 s budget is not enforced on Windows. Its value is derived from the first three Windows CI runs of PR #163 and recorded with the run ids.

**Non-goals:**
- Triaging the 419 baselined survivors, refreshing the formal-bindings baseline or its rerun evidence, and re-running `make mutation-formal`. All of this belongs to the follow-up survivor-triage change.
  - Rebinding makes the stated reasons of about 80 survivors on the six rebound functions stale, and 20 survivors on the two unbound rows describe functions that are no longer planned.
  - Because killer-test files change, the rerun evidence of all 419 survivors goes stale by the digest convention.
- Fixing F13, F14, F15, F17, a confirmed F16, or an F18.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `formal-verification-toolchain`:
  - the binding-reach gate, its per-row plan, its fail-closed cases, and its budget;
  - the per-GOOS capability policy and the ban on raw skips in binding-test files;
  - the model-element check;
  - the rule that rebinding a row keeps its model, element, symbol, and file cells, so the triage ledger stays valid;
  - unbinding rows whose function does not carry the model's claim.
- `module-path-containment-model`:
  - junction creation and junction generation in the rapid property, with an identity oracle;
  - a decided F16 leg with three outcomes stated in advance;
  - real-function replays for the custom-check and env-file rows, and an unbound unpack row;
  - a construction-layer binding for the virtual-harness rows.

  It MODIFIES "Golden replay tests", so instances skip only off a capability's required platforms. It MODIFIES "Cost budgets", so the Windows budget is derived from three CI runs.
- `formal-ci-gate`: MODIFIES "CI time budgets" to add the `Binding reach` step to the heavy steps with a hard budget.

## Impact

- **Scripts:**
  - `scripts/formal_mutation.py`: the `reach` subcommand, and per-row killers in the plan, which bumps `PLAN_VERSION` to 2;
  - `scripts/formal.py`: the raw-skip check and the element check in `correspondence`, and the `[reach]` and `[correspondence]` manifest tables;
  - `scripts/test_formal_mutation.py` and `scripts/test_formal.py`.
- **Build and CI:**
  - `Makefile`: `formal-reach` and its help line;
  - `.github/workflows/formal-verification.yml`: a `Binding reach` step, and a job timeout raised from 46 to 52;
  - `formal/manifest.toml`: `[reach]` budget and calibration, `[ci] paths`, model calibration text, possibly `[correspondence]`, and a possible F16 or F18 `finding` command.
- **Test utilities:** `internal/testutil/fstree` (the policy, probe errors, and the junction writer) and `internal/testutil/mpctree` (the dotenv sentinel content).
- **Tests:**
  - `internal/app/moduleops/module_path_containment_{golden,rapid,formal}_test.go`;
  - `internal/runtime/virtual_path_harness_{formal,golden,rapid}_test.go`;
  - a new policy unit test in `internal/testutil/fstree`;
  - a new Windows-only `internal/testutil/fstree/links_windows_test.go`.
- **Formal sources:**
  - the correspondence tables in `formal/alloy/ModulePathContainment.als`, `formal/alloy/VirtualPathHarness.als`, and `formal/tla/HostCallbackToken.tla`;
  - regenerated `ModulePathContainment` and `VirtualPathHarness` goldens, whose fingerprints change because their source text changes.
- **Docs:** `formal/README.md`, `.agents/skills/formal-verification/SKILL.md`, `.agents/rules/commands.md`, and `tasks/next/formal-binding-gaps.md`.
- **No production Go code changes.** There are no new dependencies: the gate uses only the Python standard library and `go test`.
- **Not breaking for users.** Only formal tooling, test utilities, CI, and docs change, so no migration notes are needed. Contributors see two new failure modes: a raw `t.Skip` in a binding-test file, and a required capability missing on its platform. Both are documented in the skill.
- **Verification limits:** the Windows parts cannot run locally, because the maintainer's Windows VM is down. They are compile-checked with `GOOS=windows`, and Windows CI on PR #163 verifies them at runtime.
