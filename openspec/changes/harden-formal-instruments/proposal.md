## Why

The formal setup fails closed everywhere except in two instruments, and both let a model look bound to code that nothing checks:
- **A correspondence row can name a binding that never runs its function.** The correspondence check only proves that the symbol is declared in the named file. The first formal-bindings mutation run (809 mutants, 419 baselined survivors) showed that every mutant of such a row escapes. A coverage prototype on the current tree (2026-11-14) runs each killer test alone under `-coverpkg` and finds **eight** bound rows whose binding executes none of the function's statements:
  - the six already known: `validateDestinationPath`, `LoadEnvFile`, `newVirtualPathResolverForFilesystem`, `standardVirtualAnchorsForOS`, `CustomCheckScript.ResolveWithFSAndModule`, `Invowkfile.GetEffectiveWorkDir`;
  - `VirtualFilesystemConfig.EffectiveAccess` (listed in `tasks/next/formal-binding-gaps.md`);
  - `Server.ValidateToken`, which is new. `TestHostCallbackToken_LifecycleMatchesModel` authenticates through `admitConn`, and `ValidateToken` has no production caller.
- **A platform-only leg can skip on the one platform it exists for.** `TestModulePathContainment_F16Junction` skipped on windows-latest (run 36202690168, job 108292507012) because `fstree.Probe().Junction` was false. `probeJunction` throws the creation error away, so the cause is unknown. Even where it runs, the leg only logs a verdict. No other Go test generates `KindJunction`, although `formal/README.md` says junctions are covered by "the witness commands, the rapid properties, and the Windows-only F16 leg". Only the Alloy witnesses exist. F16 has never been decided on any platform.

## What Changes

1. **Reach gate.** A new `scripts/formal_mutation.py reach` subcommand and a `make formal-reach` target. They reuse the formal-bindings plan (each function's file, line range, rows, and killers) and run each killer test alone with `-coverpkg`/`-coverprofile`. The gate fails unless every killer of every bound row covers at least one statement inside its function's line range.
   - **Fail-closed cases:** a missing or unparsable profile, a killer that fails, skips, or is not reported, a function file absent from the profile, and a function range with no coverage blocks.
   - **Exempt rows:** unbound rows (`no-symbol`, `no-binding`, `type-symbol`, `trace-only`, `characterisation-only`) are reported, not gated.
   - **Wiring:** it runs as a new step of the formal CI lane, with budgets.
   - **Calibration:** on the current tree the gate fails and names exactly the eight rows above. It passes after item 4.
2. **No platform skip on its own platform.** `internal/testutil/fstree` gains a capability policy per GOOS: junctions are required on Windows, and symlinks are required on Linux and macOS (and on Windows under GitHub Actions).
   - `Probe` keeps each probe's error.
   - A required capability that is unavailable fails the test with that error, in the F16 leg, in `Cache.Get`, and in the rapid draws.
   - Binding-test files may skip for a capability only through the fstree helpers. `formal.py correspondence` rejects raw `Skip` calls there.
   - Junction creation is investigated in this order: surface the error, compare `writeMountPointReparse` with a known-good mount-point writer (Go's Windows `os` tests), then fall back to `mklink /J`.
3. **Junctions decide F16.** The `ModulePathContainment` rapid property draws `KindJunction` links on Windows. Junction escapes are judged by file identity or content, never by `filepath.EvalSymlinks` (Go 1.23+ may not evaluate mount points). The F16 leg asserts instead of logging, and covers Load admission, script read, both copies, and the module hash. The golden replay keeps junctions fixed off. Windows CI on the PR decides one of three outcomes, each stated in advance:
   - (a) **Creation still fails:** the leg and the gate fail, and investigation continues. The change does not merge with the leg skipped.
   - (b) **Escape:** F16 becomes a confirmed finding through the established procedure: a manifest `finding = "F16"` command next to a passing fix configuration, a README Findings row, a characterisation replay, and a property carve-out mirroring F13. It is not fixed here.
   - (c) **Contained:** the F16 hypothesis is refuted and recorded as such. The leg stays as a binding that asserts containment.
4. **Rebind the eight rows** without touching production code:
   - `GetEffectiveWorkDir` → `TestModulePathContainment_ContainerWorkDirGolden`, which already reaches it.
   - `CustomCheckScript.ResolveWithFSAndModule` and `LoadEnvFile` → real-function replays inside `TestModulePathContainment_GoldenVectors`, compared with the model's current-code verdicts. Those verdicts include the open findings F13 and F17.
   - `validateDestinationPath` → a replay over the materialised golden trees.
   - The three `VirtualPathHarness` rows → a new construction-layer binding.
   - `ValidateToken` → admissibility checks added to the lifecycle state machine.
   - The README coverage claim is corrected.

   A strengthened binding that fails on unmodified code is a defect. It gets F18 (the next free id) through the finding procedure. The implementer stops and reports, and never weakens the assertion or patches product code.

**Non-goals:** triaging the 419 baselined survivors, and refreshing the formal-bindings baseline or rerun evidence. Rebinding makes the stated reasons of about 100 survivors on these eight functions stale. Because killer-test files change, the rerun evidence of all 419 survivors goes stale by the digest convention. That work, and re-running `make mutation-formal`, belong to the follow-up survivor-triage change. Fixing F13, F14, F15, F17, or a confirmed F16 is also out of scope.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `formal-verification-toolchain` (introduced by the unarchived `adopt-formal-verification` change and extended by `mutation-test-formal-bindings`):
  - the binding-reach gate, with its fail-closed cases, CI wiring, and budget;
  - the per-GOOS capability policy and the ban on raw skips in binding-test files;
  - the rule that rebinding a row keeps its model, element, symbol, and file cells, so the triage ledger stays valid.
- `module-path-containment-model` (introduced by the unarchived `model-module-path-containment` change):
  - junction generation in the rapid property, with an identity oracle;
  - a decided F16 leg with three pre-stated outcomes;
  - real-function replays for the custom-check, env-file, and unpack rows;
  - a construction-layer binding for the virtual-harness rows.

  This narrows that change's "skip instances lacking a capability" scenario so that it applies only off the capability's required platforms.

## Impact

- **Scripts:**
  - `scripts/formal_mutation.py`: the `reach` subcommand, and per-row killers in the plan, which bumps `PLAN_VERSION` to 2;
  - `scripts/formal.py`: the raw-skip check in `correspondence`, and the `[reach]` manifest table;
  - `scripts/test_formal_mutation.py` and `scripts/test_formal.py`.
- **Build and CI:**
  - `Makefile` (`formal-reach` and its help line);
  - `.github/workflows/formal-verification.yml` (a `Binding reach` step, and a larger job timeout);
  - `formal/manifest.toml` (`[reach]` budget and calibration, `[ci] paths`, model calibration text, and a possible F16 or F18 `finding` command).
- **Test utilities:** `internal/testutil/fstree` (the policy, probe errors, and the junction writer) and possibly `internal/testutil/mpctree` (sentinel file content).
- **Tests:**
  - `internal/app/moduleops/module_path_containment_{golden,rapid,formal}_test.go`;
  - `internal/runtime/virtual_path_harness_formal_test.go`;
  - `internal/sshserver/token_formal_test.go`;
  - a new Windows-only `internal/testutil/fstree/links_windows_test.go`.
- **Formal sources:**
  - the correspondence tables in `formal/alloy/ModulePathContainment.als`, `formal/alloy/VirtualPathHarness.als`, and `formal/tla/HostCallbackToken.tla`;
  - regenerated `ModulePathContainment` and `VirtualPathHarness` goldens, which change only their fingerprints.
- **Docs:** `formal/README.md`, `.agents/skills/formal-verification/SKILL.md`, `.agents/rules/commands.md`, and `tasks/next/formal-binding-gaps.md`.
- **No production Go code changes.** There are no new dependencies: the gate uses only the Python standard library and `go test`.
- **Verification limits:** the Windows parts cannot run locally because the maintainer's Windows VM is down. They are compile-checked with `GOOS=windows`, and Windows CI on the PR verifies them at runtime.
