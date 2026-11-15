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
- A third, `standardVirtualAnchorsForOS`, does more than the model states. The model has no per-OS anchor layout, so a replay of it would need a non-model oracle. The maintainer first unbound it (design decision 8), then kept it bound with the narrow, normative claim that anchors are explicit allowed roots and `@home` is not (design decision 14).
- Related: 38 of the 102 rows (34 distinct model/element pairs) name a "model element" that does not exist in the model or its manifest commands, among them `unpackRejectsEscape`. They were verified as real naming drift, not matcher artifacts (design decision 10).
- Unbinding rows leaves survivors in the ledger whose function is no longer planned, and nothing reports them: 20 after this change (`Auth` 8, `unpackRejectsEscape` 12).

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
   - junction creation failures are investigated in a fixed order: surface the error, then compare `writeMountPointReparse` with Go's own Windows mount-point test helper. There is no `mklink /J` fallback (design decision 12): the probe shares the creation path, so a fallback would hide a broken writer.
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
   - `newVirtualPathResolverForFilesystem` and `EffectiveAccess` → a new construction-layer binding, `TestVirtualPathHarness_RootConstruction`, whose expectations come from the model's `locIsAllowedRoot` and access modes.
   - `standardVirtualAnchorsForOS` → `TestVirtualPathHarness_RootConstruction` with the narrow claim "anchors are explicit allowed roots" (design decision 14): `@config`, `@data`, `@cache`, `@state`, `@tmp`, and `@work` are accepted, and a `@home` path is rejected under restricted access. The anchor paths are inputs from the production resolver and are not verified; per-OS anchors in the model are a follow-up.
   - `Auth` / `ValidateToken` and `unpackRejectsEscape` / `validateDestinationPath` → also explicitly unbound (binding `-`), each with the reason in its abstraction cell. Re-pointing them (`Auth` to `admitConn`, `unpackInside` to `normalizeZIPPath`) belongs to the survivor-triage follow-up.
   - The result is six rows rebound and two unbound (design decision 14, which supersedes decision 8 and the "seven" of decision 1).
   - The README junction coverage claim is corrected.

   A strengthened binding that fails on unmodified code, unless an open finding explains it, is a defect. It gets F18, the next free id, through the finding procedure. The implementer stops and reports, and never weakens the assertion or patches product code.
5. **Model elements must exist.** `formal.py correspondence` checks that each row's model element is named in the model source or in the model's manifest commands. Today's 34 unresolved pairs (38 rows) go on a fail-closed, shrink-only `[correspondence] unresolved_elements` list in the manifest. No new or edited row may enter it, and the survivor-triage follow-up empties it (design decision 7). The match is case-sensitive.
6. **Binding gaps and orphaned survivors are reported, and the lists cannot grow.** `scripts/formal_mutation.py triage --check` lists every `no-binding` row (19 after this change) and every baselined survivor whose row is unbound (20) through a `[triage]` table in the manifest. By-design rows (no-symbol, type-symbol, trace-only, characterisation-only; 34) are derived and reported, not listed. A new gap or orphan fails, and a listed entry that no longer applies fails as stale (design decisions 9 and 15, D13).
   - `triage --check --base <sha>` fails when `[correspondence] unresolved_elements`, `[triage] binding_gaps`, or `[triage] orphaned_survivors` lists an entry that the base commit's manifest does not. In CI the base is the PR base SHA; locally it defaults to the merge-base with `origin/main`. An unreadable base fails closed.
   - The formal lane runs `scripts/test_formal_mutation.py` and this check on every affected PR.
7. **Windows replay budget.** The 30 s budget is not enforced on Windows. Its value is derived from three windows-latest runs of the final F1 tree and recorded with the run ids (design decision 13). No CI step enforces it.

**Non-goals:**
- Triaging the 419 baselined survivors, refreshing the formal-bindings baseline or its rerun evidence, and re-running `make mutation-formal`. All of this belongs to the follow-up survivor-triage change.
  - Rebinding makes the stated reasons of 81 survivors on the six rebound functions stale, and 20 survivors on the two unbound rows describe functions that are no longer planned. Item 6 lists those 20; the follow-up empties that list.
  - Because killer-test files change, the rerun evidence of all 419 survivors goes stale by the digest convention.
- Fixing F13, F14, F15, F17, a confirmed F16, or an F18.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `formal-verification-toolchain`:
  - the binding-reach gate, its per-row plan, its fail-closed cases, and its budget;
  - the per-GOOS capability policy and the ban on raw skips in binding-test files;
  - the model-element check and its shrink-only list of unresolved elements;
  - the rule that rebinding a row keeps its model, element, symbol, and file cells, so the triage ledger stays valid;
  - unbinding rows whose function does not carry the model's claim.
- `module-path-containment-model`:
  - junction creation and junction generation in the rapid property, with an identity oracle;
  - a decided F16 leg with three outcomes stated in advance;
  - real-function replays for the custom-check and env-file rows, and an unbound unpack row;
  - a construction-layer binding for the virtual-harness rows, including the narrow `anchors` claim.

  It MODIFIES "Golden replay tests", so instances skip only off a capability's required platforms. It MODIFIES "Cost budgets", so the Windows budget is derived from three CI runs.
- `mutation-testing`: `triage --check` reports binding gaps and orphaned survivors through lists that cannot grow against the PR base.
- `formal-ci-gate`: MODIFIES "CI time budgets" to add the `Binding reach` step to the heavy steps with a hard budget.

## Impact

- **Scripts:**
  - `scripts/formal_mutation.py`: the `reach` subcommand; per-row killers in the plan, which bumps `PLAN_VERSION` to 2; and the `[triage]` list check and the `--base` growth check in `triage --check`;
  - `scripts/formal.py`: the raw-skip check and the element check in `correspondence`, and the `[reach]`, `[correspondence]`, and `[triage]` manifest tables;
  - `scripts/test_formal_mutation.py` and `scripts/test_formal.py`.
- **Build and CI:**
  - `Makefile`: `formal-reach` and its help line;
  - `.github/workflows/formal-verification.yml`: a `Binding reach` step, a `Triage lists` step, `scripts/test_formal_mutation.py` in `Runner self-tests`, and a job timeout raised from 46 to 52;
  - `formal/manifest.toml`: `[reach]` budget and calibration, `[ci] paths` (including the triage ledger and baseline), model calibration text, `[correspondence] unresolved_elements`, `[triage]`, and a possible F16 or F18 `finding` command.
- **Test utilities:** `internal/testutil/fstree` (the policy, probe errors, and the junction writer, with no `mklink` fallback) and `internal/testutil/mpctree` (the dotenv sentinel content).
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
