## 0. Ground rules

- [ ] 0.1 Work only in the `feat/harden-formal-instruments` worktree. Do not run any `make mutation-*` target: its wrapper reverts uncommitted tracked files. Make no production Go code changes. Only these may change:
  - tests and `internal/testutil/{fstree,mpctree}`;
  - scripts, the Makefile, and the workflow;
  - the manifest, the correspondence tables, and docs
- [ ] 0.2 Before any change, record `python3 scripts/formal.py snapshot --out <scratch>/baseline.json` (outside `artifacts/`)
- [ ] 0.3 Stop rule: if a strengthened or new binding fails on unmodified code and no open finding (F13, F15, F17) explains it, first confirm in the model source that the model makes that claim for that operation kind. A transcription error in the replay is fixed, not recorded. If the claim is real, stop and report it, then record F18 through the finding procedure (design D9). Never weaken an assertion, and never edit product code

## 1. Reach gate: plan and subcommand

- [ ] 1.1 `scripts/formal_mutation.py`: record each row's own killers in `functions[].rows[].killers` and bump `PLAN_VERSION` to 2. Update the plan fixtures and golden plan in `scripts/test_formal_mutation.py`
- [ ] 1.2 Implement `reach [--out DIR]` (default `artifacts/formal/reach/`), as described in design D1:
  - run each killer alone with `-count=1 -json -covermode=set -coverpkg -coverprofile`, with the rapid determinism variables pinned, in parallel through `formal.parallel_jobs()`;
  - check exit status, outcome, and profile header, and check every (row, killer, function) block inside the function's range;
  - write `reach.json` and a text summary, and print a `timing` line
- [ ] 1.3 Add unit tests to `scripts/test_formal_mutation.py` with fixture profiles and a stub `GO_CMD`. Cover:
  - reached;
  - zero covered;
  - file not instrumented;
  - an empty range;
  - a missing profile;
  - a garbled profile;
  - killer fail, skip, and not-run;
  - a non-zero exit with passing JSON;
  - exempt unbound rows (all five reasons);
  - the per-row killer split;
  - loading `PLAN_VERSION` 2
- [ ] 1.4 `scripts/formal.py`: add the `[reach]` manifest table (`budget_seconds`, `budget_source`, `calibration`; reject unknown keys) to `validate_manifest` and to `manifest_budgets`, with `scripts/test_formal.py` cases
- [ ] 1.5 Add a `formal-reach` Make target and its help line, and add the new script entry points to `lint-scripts` if needed
- [ ] 1.6 **Calibration, before any rebinding.** Run `make formal-reach`. It must fail and name exactly the eight (row, killer) pairs in design.md Context, with no tooling errors. Record that output summary and the wall time in `[reach] calibration`, and set `budget_seconds = ceil(3 × local seconds)` with `budget_source = "local"`. If it names any other pair, stop and report it

## 2. Reach gate: CI wiring

- [ ] 2.1 Add a `Binding reach` step to `.github/workflows/formal-verification.yml` after the replay step, with `if: steps.affected.outputs.run != 'false'`, `timeout-minutes: 6`, and `run: make formal-reach`. Raise the job `timeout-minutes` to 52
- [ ] 2.2 Update `scripts/test_formal.py`: the expected step list, the job-timeout sum, and the `affected` condition for the new step
- [ ] 2.3 Add `artifacts/formal/reach/` to the upload step's paths, so failures leave profiles
- [ ] 2.4 Add `scripts/formal_mutation.py` and `scripts/test_formal_mutation.py` to `formal/manifest.toml` `[ci] paths`

## 3. Capability policy and skip guard

- [ ] 3.1 `internal/testutil/fstree`:
  - add `Capability` with the per-GOOS required policy (design D4);
  - make `Caps` keep `SymlinkErr` and `JunctionErr`, and make the probes return errors;
  - add `Require(t, cap)`, and make `Cache.Get` fail on a required-but-missing capability;
  - add `MustSymlink`/`MustJunction` for `Fataler`s;
  - add SPDX headers to new files
- [ ] 3.2 Add a policy unit test in `fstree`, table-driven over GOOS values with injected caps and environment. It checks that a required-but-missing capability fails and that others skip. Keep the test file free of the binding-test suffixes
- [ ] 3.3 Replace every raw skip in a binding-test file with the policy helpers. That is the 8 sites in `internal/app/moduleops/module_path_containment_{formal,rapid}_test.go` and `internal/runtime/virtual_path_harness_{formal,golden,rapid}_test.go`
- [ ] 3.4 `scripts/formal.py correspondence`: fail on a `.Skip(`, `.Skipf(`, or `.SkipNow(` call in a binding-test file, naming the file and line. Add `scripts/test_formal.py` fixtures that check a binding file fails and a plain `_test.go` passes

## 4. Junction creation (Windows; compile-checked locally)

- [ ] 4.1 Add a Windows-only `internal/testutil/fstree/links_windows_test.go` `TestCreateJunction`. It requires creation and checks it functionally: not `ModeSymlink`, and `os.SameFile` through the link. On failure it prints the probe error
- [ ] 4.2 Compare `writeMountPointReparse` field by field with Go 1.27's `src/os/os_windows_test.go` `createMountPoint`/`reparseData`: buffer layout, both NULs, `ReparseDataLength`, the input length, and the `CreateFile` flags. Fix every divergence and note the comparison in a code comment. The hypothesis (missing print-name NUL) is only a hypothesis
- [ ] 4.3 Add the `cmd /c mklink /J` fallback, used only when the reparse write fails. Report both errors when both fail
- [ ] 4.4 Verify locally with `GOOS=windows go vet ./internal/testutil/fstree/ ./internal/app/moduleops/` and `GOOS=windows go test -c -o /dev/null` for `./internal/testutil/fstree/` and `./internal/app/moduleops/`. Runtime verification is deferred to Windows CI on the PR

## 5. Junctions in the property and the F16 leg

- [ ] 5.1 `TestModulePathContainment_Property` (design D6):
  - draw symlink or junction where junctions are available; junctions target directories, and the script path is `<rel>x/secret`;
  - judge escape by `os.SameFile` against a unique outside sentinel, backed by content, for both kinds;
  - keep the F13 carve-out path-based (`invowk_modules/`)
- [ ] 5.2 Rewrite `TestModulePathContainment_F16Junction` to `Require` junctions and to assert containment (no logging-only verdict) for:
  - `IsModule`/`Load`;
  - the script read;
  - both copies (sentinel present in the copy);
  - `computeModuleHash` (the hash changes when only the outside sentinel changes)
- [ ] 5.3 Update the `probeF16` row's abstraction note (still `characterisation:`). Run `make formal-golden` for `ModulePathContainment` because its table changed, and run `make formal`
- [ ] 5.4 Run the Linux tests: the property runs only the symlink branch, and the F16 leg skips under the policy. Then run the Windows compile checks from 4.4
- [ ] 5.5 On the PR's Windows CI run, read the verdicts of `TestCreateJunction`, the property's junction branch, and the F16 leg. Apply exactly one outcome from design D7:
  - (a) creation fails: iterate on section 4;
  - (b) escape: record F16 through the finding procedure (manifest `finding = "F16"` with a passing fix configuration, a README Findings row, the leg flipped to assert the escape, and an F16 carve-out in the property), and run `make formal-golden` if the `.als` changed;
  - (c) contained: record F16 as refuted in the README.

  In every case, write the outcome and the CI run id into `ModulePathContainment`'s calibration

## 6. Rebind the eight rows

- [ ] 6.1 `GetEffectiveWorkDir` row (`ModulePathContainment.als`): set the binding to `TestModulePathContainment_ContainerWorkDirGolden`
- [ ] 6.2 Custom-check replay: in `TestModulePathContainment_GoldenVectors`, replay `CustomCheckScript.ResolveWithFSAndModule` on every `ScriptRead` instance with a recording reader. Require the model's accept and touched node, F13 instances included
- [ ] 6.3 Env replay:
  - give regular files a dotenv sentinel (`INVOWK_ATOM=<id>`, keeping the `content-<id>` substring) in `mpctree`, or an env-only override if that breaks another assertion;
  - call `LoadEnvFile` on every `EnvRead` instance of an admitted module that `ValidateEnvFilePath` accepts;
  - require the model's accept and touched node, F17 instances included. A non-file or missing target must return an error
- [ ] 6.4 Unpack replay: on every `Unpack` instance, call `validateDestinationPath(root, filepath.Join(root, filepath.FromSlash(p)), p)` and compare it with the decision the golden records for `Unpack`. First confirm in the `.als` (`unpackInside[destCheck]`, `accepts`, `lexContains`, refShape facts) what the model claims for each refShape. State the `AbsPath` call-shape difference (`Join` places an absolute name under the root) in the row's abstraction note. Update the row's abstraction note to say it is a Go-side replay of the destination check
- [ ] 6.5 Add `TestVirtualPathHarness_RootConstruction` to `internal/runtime/virtual_path_harness_formal_test.go`, as described in design D9:
  - anchors for linux, darwin, and windows, with and without overrides, against an independent table;
  - resolver tagged roots for access unset, `restricted`, and `full`, with and without declared paths, with the workdir included (F15);
  - the access mode.

  Bind the `allowedRoots`, `anchors`, and `access` rows (`VirtualPathHarness.als`) to it. Run `make formal-golden` for `VirtualPathHarness`
- [ ] 6.6 `ValidateToken`: in `TestHostCallbackToken_LifecycleMatchesModel`, after every step, require `ValidateToken(tok).ok` to equal the Auth step's own expectation (`model[e] == tokenValid` and not past the TTL on the fake clock) for every generated token. The `Auth` row in `HostCallbackToken.tla` keeps its symbol
- [ ] 6.7 For each of 6.2–6.6, seed one plausible defect in the real function locally, confirm the binding fails, and revert it. Examples:
  - drop the containment check in the custom-check resolver;
  - make `LoadEnvFile` ignore `basePath`;
  - drop the `..` check in `validateDestinationPath`;
  - drop the XDG override in `standardVirtualAnchorsForOS`;
  - skip the expiry check in `ValidateToken`.

  Append the results to each model's `calibration`
- [ ] 6.8 Run `make formal-reach`. It must pass with every bound pair reached. Seed a reach regression by temporarily removing the `LoadEnvFile` call: exactly that pair must fail. Revert it. Update `[reach] calibration` and the measured budget
- [ ] 6.9 `python3 scripts/formal_mutation.py triage --check` must pass with no ledger edits, because the row identity cells are unchanged
- [ ] 6.10 Run `python3 scripts/formal.py snapshot --compare <scratch>/baseline.json`. Every verdict and golden instance digest must be identical, except entries that a recorded F16 or F18 outcome added

## 7. Documentation

- [ ] 7.1 `formal/README.md`:
  - correct the junction coverage sentence (design D8);
  - replace the F16 paragraph with the recorded outcome;
  - add `make formal-reach` to Running and a "Binding reach" bullet to "How a model earns trust";
  - update the Mutation testing paragraph that lists the seven rows (now eight, and rebound);
  - add the reach step to the CI budgets table
- [ ] 7.2 `.agents/skills/formal-verification/SKILL.md`:
  - the reach gate in the workflow and Verification sections;
  - the capability policy replacing "skipped and counted" (lines 76–80);
  - binding tests skip only through `fstree` helpers;
  - the junction oracle pitfall: never `EvalSymlinks` for mount points
- [ ] 7.3 `.agents/rules/commands.md`: add `make formal-reach` to the Quick Reference and the CI/CD workflow row
- [ ] 7.4 `tasks/next/formal-binding-gaps.md`: mark the eight rows rebound, and flag their baselined survivors for retriage, and note that the rerun evidence of all 419 survivors is stale by the digest convention, for the survivor-triage follow-up. Add the follow-up as future work, covering the 419 survivors, a baseline refresh, and open questions 1–2 from design.md

## 8. Verification

- [ ] 8.1 `make lint`
- [ ] 8.2 `make test-scripts` (includes `test_formal.py`, `test_formal_mutation.py`, and `triage --check`)
- [ ] 8.3 `make formal` and `make formal-reach`
- [ ] 8.4 `make test` (the full suite, not short)
- [ ] 8.5 `make check-file-length`, and `make license-check` for new Go files
- [ ] 8.6 `make check-agent-docs`, since the skill and rules changed
- [ ] 8.7 `make check-baseline` (goplint consumer gate); triage any new finding with the user
- [ ] 8.8 Run the `GOOS=windows` vet and compile checks again on the final tree (4.4)
- [ ] 8.9 Push, then confirm on the PR that windows-latest runs `TestCreateJunction`, the property's junction branch, and the F16 leg without a skip, and that the formal lane's `Binding reach` step passes. Record the measured CI seconds for a later `budget_source = "ci"` promotion
