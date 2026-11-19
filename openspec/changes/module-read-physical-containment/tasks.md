# Tasks

Commit boundaries follow design D7. Groups 1-4 form the F13 commit, group 5 the F17 commit, and group 7 the post-evidence F16 commit. Each commit must leave `make formal` green, and must keep its own pre-fix mutant reproducing alone. Formal artefacts are edited in the implement stage only (decision 46 item 7).

## 1. F13: contained script reads (code)

- [ ] 1.1 Add the containment helper in `pkg/invowkfile` (new file, with license header). It opens an `os.Root` on the module path and runs `Stat` on the path relative to it. It returns one of four results (design D2):
  - contained;
  - absent: `OpenRoot` not-exist, or `Stat` and `Lstat` both not-exist;
  - escaping: a non-OS error, or `Stat` not-exist while `Lstat` succeeds. It wraps `ErrScriptPathTraversal`;
  - denied: any other OS error, which becomes a read error.

  Verify with a table test over:
  - a regular file;
  - relative in-module links to a file, to a directory and to the root (`..`);
  - an absolute in-module link (denied);
  - relative and absolute escaping links;
  - a dangling relative link;
  - a missing file and a missing root;
  - a path below an escaping link;
  - a directory without search permission, off Windows.

  Each branch needs a test that kills its plausible mutants, because the helper becomes a formal-bindings target.
- [ ] 1.2 Call the helper from `Implementation.ResolveScriptWithFSAndModule` and `CustomCheckScript.ResolveWithFSAndModule`, after `validateScriptPathContainment` and before the injected reader. Leave `validateScriptPathContainment` unchanged. Verify:
  - new resolver tests prove a vendored escaping link is denied and the reader is never called;
  - the existing fake-reader tests pass unchanged: `go test ./pkg/invowkfile/ ./internal/app/deps/`.
- [ ] 1.3 Add a cross-platform test, run on all OSes, in which the module path is given through an alias of the temporary directory (on macOS, the unresolved `os.TempDir()` under `/var`, and on the other OSes a symlinked parent where available). On case-insensitive filesystems, also give the module or script path in a different case. A contained script must be read in each case. Verify with `go test -run <name> ./pkg/invowkfile/`, and note the skip reason per OS where a variant cannot run.
- [ ] 1.4 Delete `Module.ValidateScriptPath` and `checkSymlinkSafety` (`pkg/invowkmod/invowkmod.go`), and with them their mutation tests (`invowkmod_mutation_test.go`), the goplint exception (`.goplint/exceptions.toml`) and the note in `docs/next/cross-platform-windows-improvements.md`. Keep `ContainsPath`. Verify with `go build ./... && go test ./pkg/invowkmod/ && make check-baseline`.

## 2. F13: acceptance through invowk cmd

- [ ] 2.1 Check how testscript can create symlinks: the built-in `symlink` command, and the conditions `commonCondition` offers in `tests/cli/cmd_test.go:88`. No existing txtar creates one. Add a condition if one is needed, and record the platform gating it implies.
- [ ] 2.2 Add `virtual_module_read_containment.txtar` and its native mirror `native_module_read_containment.txtar`. Each holds a module whose `script.file` is `invowk_modules/x`, where `x` is a symlink to an outside file carrying a sentinel. `invowk cmd` must fail with the containment error, and the sentinel must not appear in stdout or stderr. Verify with `make test-cli` (the `TestShRuntimeMirrorCoverage` and `TestVirtualNativeCommandPathAlignment` checks included).

## 3. F13: formal artefacts

- [ ] 3.1 Add the opt-in relative-target option to `internal/testutil/fstree`. It creates each link after its target, with the target relative to the link's directory. Use it only in the `ModulePathContainment` golden replay. Verify with a fstree unit test, and check that `TestVirtualPathHarness_GoldenVectors` is unchanged.
- [ ] 3.2 Model: `accepts` gains `physContains` for `ScriptRead`. Update the header comment and the correspondence table:
  - add a `physContains` row bound to the helper;
  - mark the `readOp` rows of both resolvers "code stricter than model: absolute link target denied inside the module", naming the 3.5 test;
  - turn `findingF13` into a binding row for the renamed replay;
  - remove the `ValidateScriptPath` and `checkSymlinkSafety` rows.

  Then run `make formal-golden`. Verify that the golden fingerprint changed, and that `TestModulePathContainment_GoldenVectors` passes with `dAccepts` equal to the real script and custom-check decisions on every `ScriptRead` instance.
- [ ] 3.3 Manifest:
  - rename `findingF13VendoredSymlink` to `mutantPreFixF13LexicalScriptContainment` (drop `finding`, keep `mutant_of` and `expect = "counterexample"`);
  - rename or remove the reach entry `ModulePathContainment: findingF13`;
  - remove the `ValidateScriptPath` and `checkSymlinkSafety` reach and binding entries;
  - update the calibration text.

  Verify with `python3 scripts/formal.py correspondence` and `make formal-reach`.
- [ ] 3.4 Invert `TestModulePathContainment_F13VendoredSymlink` and rename it, for example to `TestModulePathContainment_VendoredSymlinkScriptReadDenied`. It must assert that both the implementation read and the custom-check read are denied and return no sentinel. Update the golden comment ("F13 instances included"). Verify with `go test -run 'TestModulePathContainment' ./internal/app/moduleops/`.
- [ ] 3.5 Add `TestModulePathContainment_AbsoluteInModuleLinkDenied`, which tests the script read here and gains its env arm in 5.4, and name it in the `readOp` rows. Verify with `go test -run TestModulePathContainment_AbsoluteInModuleLinkDenied ./internal/app/moduleops/`.
- [ ] 3.6 Rapid property:
  - drop the `invowk_modules/` exemption;
  - judge accept by the resolver's result;
  - generate relative as well as absolute link targets;
  - keep the F16 junction exemption only while F16 remains recorded for the script read.

  Verify with `make formal-rapid-deep` at the budget recorded in the manifest.
- [ ] 3.7 F16 leg: change the script-read arm of `TestModulePathContainment_F16Junction` to assert denial and no sentinel, keep the other arms as recorded, and add a note to the model's F16 calibration text. Do NOT edit `scriptReadScanGuarded` or `findingF16JunctionScan` in this commit, because `make formal` would fail there; that edit belongs to group 7. Verify that `make formal` stays green, and that the leg compiles on Windows with `GOOS=windows go vet ./internal/app/moduleops/`.
- [ ] 3.8 Seed one plausible defect in the new helper, for example treating an escape error as absent. Check that the golden replay or the 3.5 test fails, revert the defect, and record the result in the manifest calibration text. Add the `physContains` reach entry. Verify with `make formal-reach`.
- [ ] 3.9 `formal/README.md`: mark the F13 row "Fixed", naming `mutantPreFixF13LexicalScriptContainment`; record the code-stricter-than-model case; describe junction coverage as it now is. Verify with `make formal` and `make check-agent-docs`.

## 4. F13: docs and commit

- [ ] 4.1 Docs:
  - `website/docs/modules/validating.mdx`: script and custom-check reads are physically contained, `invowk_modules/` included, and links with absolute targets are denied;
  - the same text in its pt-BR copy;
  - `.agents/skills/module-security/SKILL.md` and its check catalog (SC-01).

  Verify with `cd website && npm run build`.
- [ ] 4.2 Add a test where `collectInvowkfileInterpreterDiagnostics` (`cmd/invowk/validate_interpreter_diagnostics.go`) runs on a module whose `script.file` and custom-check file escape through an `invowk_modules/` link. A recording reader must never be called for the outside path. Also add a custom-check test where the escaping file makes the check fail with the traversal error (`internal/app/deps`). Verify with `go test ./cmd/invowk/ ./internal/app/deps/`.
- [ ] 4.3 Run `make tidy`, `make license-check` and `make lint`, then commit F13 (signed, under the heavy-command lock, with a 600000 ms timeout). Verify with `git log --show-signature -1`.

## 5. F17: contained module env files (one commit)

- [ ] 5.1 Give `runtime.LoadEnvFile` a `moduleRoot` parameter and add `ErrEnvFileOutsideModule` (design D3).
  - With `moduleRoot == ""`, behaviour is unchanged.
  - Otherwise it denies an absolute path, runs `Stat` through `os.Root`, then reads through it.
  - An optional `?` file is skipped only when `Stat` and `Lstat` both report not-exist.
  - A dangling or escaping link fails.
  - OS errors stay read errors.

  Update every caller and test caller. Verify with unit tests for each of these cases, including an absolute path built directly in Go: `go test ./internal/runtime/`.
- [ ] 5.2 `DefaultEnvBuilder.Build` passes `ctx.Invowkfile.ModulePath` as `moduleRoot` for root, command and implementation `env.files`. Verify with:
  - a test where a plain invowkfile's symlinked `.env` that points outside still loads;
  - a module-context test where an escaping link fails before the command runs.
- [ ] 5.3 Extend the `/private`-alias and case test from 1.3 to the module env read. Verify with `go test ./internal/runtime/`.
- [ ] 5.4 Formal:
  - model `accepts` gains `physContains` for `EnvRead`, then `make formal-golden`;
  - `replayEnvRead` calls `LoadEnvFile` with the module root and expects `admit && validates && physContains`, and its comment is updated;
  - the `readOp | LoadEnvFile` row keeps its binding, and its abstraction cell is marked code-stricter;
  - seed one defect in the module branch (for example, skip the `Stat` check), check that its binding fails, revert it, and record it in the calibration text;
  - rename `findingF17EnvVendoredSymlink` to `mutantPreFixF17EnvNoPhysicalContainment`, with its reach entry;
  - invert `TestModulePathContainment_F17EnvVendoredSymlink` and rename it, for example to `TestModulePathContainment_VendoredSymlinkEnvReadDenied`;
  - add the env arm of 3.5;
  - mark the README F17 row "Fixed".

  Verify with `python3 scripts/formal.py correspondence` and `make formal`, which must leave `envReadContainedPhysical` passing and the F17 pre-fix mutant producing a counterexample on its own.
- [ ] 5.5 Docs: the env-files section of `website/docs/modules/creating-modules.mdx` and the path-resolution section of `website/docs/environment/env-files.mdx` (both with their pt-BR copies) state that module env files must resolve inside the module, and when `?` skips a file. Also add the env sentence to `validating.mdx`, update the module-security SC-01 text for env files, and grep `README.md` for `env.files` resolution text. Verify with `cd website && npm run build`.
- [ ] 5.6 Run `make tidy` and `make lint`, then commit F17 (signed, under the lock, with a 600000 ms timeout). Verify with `git log --show-signature -1`.

## 6. Integration gates and evidence

- [ ] 6.1 Run the checklist gates on the final tree:
  - `make test`, `make check-baseline` (triage any new goplint finding), `make check-file-length`, `make sonar-local`;
  - `make test-scripts` and `make check-agent-docs`;
  - `go test -run Sync ./pkg/invowkfile/ ./pkg/invowkmod/ ./internal/config/`;
  - `for m in samples/invowkmods/*.invowkmod; do go run . validate "$m"; done`.

  All must pass.
- [ ] 6.2 Push the branch and open the draft PR. Its body carries the release-note line from proposal.md (a security fix, no `!`) and states the `pkg/invowkmod` API removal.
- [ ] 6.3 From the PR's windows-latest job, quote the run id, the job id and the JUnit or test-output lines. They must show that the inverted F13 and F17 replays and `TestModulePathContainment_F16Junction` RAN and passed, not skipped. Report each F16 arm's outcome. The macOS job must show the alias and case tests from 1.3 and 5.3 running.

## 7. F16 reclassification (post-evidence commit)

- [ ] 7.1 From the 6.3 outcomes, reclassify F16 under the modified requirement "F16 is decided by one of three stated outcomes":
  - **Outcome (d), every arm contained:** `findingF16JunctionScan` becomes `mutantPreFixF16…`, `scriptReadScanGuarded` uses `physContains`, and the rapid F16 carve-out goes.
  - **Otherwise:** narrow the finding to the arms that still escape.

  Then update the manifest, the correspondence and the README row (with run and job ids), and the calibration text. Verify with `make formal-golden` if the `.als` changed, then `python3 scripts/formal.py correspondence`, `make formal` and `make formal-reach`. Commit it signed, under the lock.

## Workflow follow-up

- Fix stage: `make mutation-formal` is expected to run, because `ResolveScriptWithFSAndModule`, `ResolveWithFSAndModule` and `LoadEnvFile` are bound and rewritten. Check the plan first. Then run the partial refresh `make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=...`, covering the rewritten symbols and the removal-only `pkg/invowkmod/invowkmod.go:ValidateScriptPath` and `pkg/invowkmod/invowkmod.go:checkSymlinkSafety`, and triage only `equivalent` or deferred-gap classes.
- Tracker: record the residual native and container re-open race (design Risks), the `Module.ContainsPath` question and the `invowk validate` physical-escape question (design Open questions).
- Archive the change once the review requirements are met.
