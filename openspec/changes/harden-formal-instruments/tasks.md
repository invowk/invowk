## 0. Ground rules

- [ ] 0.1 Work only in the `feat/harden-formal-instruments` worktree, and observe these limits:
  - Do not run any `make mutation-*` target: its wrapper reverts uncommitted tracked files.
  - Make no production Go code changes. Only these may change:
    - tests and `internal/testutil/{fstree,mpctree}`;
    - scripts, the Makefile, and the workflow;
    - the manifest, the correspondence tables, and docs.
  - Run heavy commands under the shared heavy-command lock when one is in use: `make test`, `make lint`, `make formal`, `make formal-reach`, goplint targets, and commits.
- [ ] 0.2 Before any change, record `python3 scripts/formal.py snapshot --out <scratch>/baseline.json`, outside `artifacts/`.
- [ ] 0.3 Stop rule (design D9, "Defect rule"). Apply it when a strengthened or new binding fails on unmodified code and no open finding (F13, F15, F17) explains the failure:
  - First confirm in the model source that the model makes that claim for that operation kind. A transcription error in the replay is fixed, not recorded.
  - If the claim is real, stop and report it as candidate F18. Never weaken an assertion, and never edit product code.

  The same rule applies to an F16 outcome (c) with no modelled reason (design D7).
- [ ] 0.4 Check: `test -s <scratch>/baseline.json`.

## 1. Reach gate: plan and subcommand

- [ ] 1.1 `scripts/formal_mutation.py`:
  - record each row's own killers in `functions[].rows[].killers`, next to the per-function union;
  - bump `PLAN_VERSION` (line 58) to 2;
  - update the plan fixtures and the golden plan in `scripts/test_formal_mutation.py`.
- [ ] 1.2 Implement `reach [--out DIR]` (default `artifacts/formal/reach/`), as described in design D1:
  - run each killer alone with `-count=1 -json -covermode=set -coverpkg -coverprofile`, with `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME` pinned exactly as the mutation profile pins them, in parallel through `formal.parallel_jobs()`;
  - check exit status, outcome (`test_outcomes`), and profile header;
  - check every (row, killer, function) block inside the function's range;
  - write `reach.json` and a text summary that list unbound rows with their reason and abstraction cell, and print a `timing` line through `formal.report_timings`.
- [ ] 1.3 Add unit tests to `scripts/test_formal_mutation.py`, with fixture profiles and a stub `GO_CMD`. Cover:
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
  - loading `PLAN_VERSION` 2 and rejecting version 1;
  - a plan failure stopping the gate before any killer runs.
- [ ] 1.4 `scripts/formal.py`:
  - add the `[reach]` manifest table (`budget_seconds`, `budget_source`, `calibration`) to `validate_manifest` (line 277) and `manifest_budgets` (line 442), and reject unknown keys with `check_keys`;
  - add `scripts/test_formal.py` cases for a valid table, an unknown key, and a soft overrun that only warns.
- [ ] 1.5 Add a `formal-reach` Make target (`python3 scripts/formal_mutation.py reach`) and its help line, next to the other `formal-*` targets. `lint-scripts` lints shell scripts only, so it needs no change.
- [ ] 1.6 **Calibration, before any rebinding.** Run `make formal-reach`.
  - It must fail and name exactly the eight (row, killer) pairs in design Context 1, with no tooling errors. If it names any other pair, stop and report.
  - Record the output summary and the wall time in `[reach] calibration`.
  - Set `budget_seconds = ceil(3 × local seconds)` and `budget_source = "local"` (design D3).
- [ ] 1.7 Check: `python3 scripts/test_formal_mutation.py` and `python3 scripts/test_formal.py` pass.

## 2. Reach gate: CI wiring

- [ ] 2.1 `.github/workflows/formal-verification.yml`:
  - add a `Binding reach` step after `Replay golden vectors and property tests`, with `if: steps.affected.outputs.run != 'false'`, `run: make formal-reach`, and `timeout-minutes` from design D3 (6 at 45 s local);
  - raise the job `timeout-minutes` from 46 to the D3 value (52 at a 6-minute step);
  - leave the upload step unchanged: it already uploads `artifacts/formal/`.
- [ ] 2.2 Update `scripts/test_formal.py`: the expected step list, the job-timeout sum, and the `affected` condition for the new step.
- [ ] 2.3 Add `scripts/formal_mutation.py` and `scripts/test_formal_mutation.py` to `[ci] paths` in `formal/manifest.toml`. Add a `scripts/test_formal.py` case in which a diff touching only `scripts/formal_mutation.py` classifies as `run=true`.
- [ ] 2.4 Check: `python3 scripts/test_formal.py` passes.

## 3. Capability policy and skip guard

- [ ] 3.1 `internal/testutil/fstree`, as described in design D4:
  - add `Capability` with the per-GOOS required policy, reading GOOS and `GITHUB_ACTIONS` through an injectable function;
  - make `Caps` keep `SymlinkErr` and `JunctionErr`, and make `probeSymlink` and `probeJunction` return errors (off Windows, `errNoJunctions`);
  - add `Require(t, cap)`, and make `Cache.Get` fail on a capability that is required but missing;
  - add `MustSymlink` and `MustJunction` for `Fataler`s;
  - add SPDX headers to new files.
- [ ] 3.2 Add a policy unit test in `fstree`, in a file without a binding-test suffix. Make it table-driven over `linux`, `darwin`, and `windows`, with `GITHUB_ACTIONS` set and unset and with injected probe results. It checks that a required-but-missing capability fails, that every other missing capability skips, and that each message carries the GOOS and the probe error.
- [ ] 3.3 Replace every raw skip in a binding-test file with the policy helpers. These are the 8 sites listed in design D4:
  - `internal/app/moduleops/module_path_containment_formal_test.go:23,108,118`;
  - `internal/app/moduleops/module_path_containment_rapid_test.go:27,72`;
  - `internal/runtime/virtual_path_harness_golden_test.go:87`;
  - `internal/runtime/virtual_path_harness_formal_test.go:25`;
  - `internal/runtime/virtual_path_harness_rapid_test.go:49`.
- [ ] 3.4 `scripts/formal.py correspondence`: fail on a `.Skip(`, `.Skipf(`, or `.SkipNow(` call in a `BINDING_TEST_SUFFIXES` file, naming the file and line. Add `scripts/test_formal.py` fixtures showing that a binding file fails and a plain `_test.go` passes.
- [ ] 3.5 Check: `go test ./internal/testutil/fstree/`, `python3 scripts/formal.py correspondence`, and `python3 scripts/test_formal.py` pass.

## 3b. Model-element check (BLOCKED: design Open question 1)

- [ ] 3b.1 Wait for the maintainer to choose option A, B, or C in design Open question 1. Do not start 3b.2 before that.
- [ ] 3b.2 Under option A (recommended), as described in design D11:
  - implement the leading-identifier check in `check_correspondence`, searching the comment-stripped model source and the model's manifest command `name` and `body` values;
  - add `[correspondence] unresolved_elements` to `formal/manifest.toml` with today's 34 `"<model>: <element>"` pairs (38 rows) from design Context 4 (the prototype's list, regenerated on the implementation tree), and validate the table in `validate_manifest`;
  - make a stale entry (it resolves, or names no row) fail.

  Under option B or C, follow the maintainer's wording and amend design D11 first.
- [ ] 3b.3 Add `scripts/test_formal.py` fixtures for a resolving row, an unresolved unlisted row, a listed row, a stale entry, and an entry with no row.
- [ ] 3b.4 Check: `python3 scripts/formal.py correspondence` and `python3 scripts/test_formal.py` pass.

## 4. Junction creation (Windows; compile-checked locally)

- [ ] 4.1 Add a Windows-only `internal/testutil/fstree/links_windows_test.go` with `TestCreateJunction`, as described in design D5 step 1. It requires creation and checks it functionally: the link is not `ModeSymlink`, and `os.SameFile` holds through the link. On failure it prints the probe error.
- [ ] 4.2 Compare `writeMountPointReparse` (`links_windows.go:76-99`) field by field with Go 1.27's `src/os/os_windows_test.go` (`createMountPoint` and `reparseData`). Compare the buffer layout, both NULs, `ReparseDataLength`, the input length, and the `CreateFile` flags. Fix every divergence, and note the comparison in a code comment. The missing print-name NUL is only a hypothesis.
- [ ] 4.3 Add the `cmd /c mklink /J` fallback, used only when the reparse write fails. When both fail, report both errors.
- [ ] 4.4 Check, locally:
  - `GOOS=windows go vet ./internal/testutil/fstree/ ./internal/app/moduleops/ ./internal/runtime/`;
  - `GOOS=windows go test -c -o /dev/null` for each of those three packages.

  Runtime verification is deferred to Windows CI (group 8).

## 5. Junctions in the property and the F16 leg

- [ ] 5.1 `TestModulePathContainment_Property`, as described in design D6:
  - draw symlink or junction where junctions are available;
  - make junctions target directories, with the script path `<rel>x/secret`;
  - judge escape for both kinds by `os.SameFile` against a unique outside sentinel, backed by content, and replace the `EvalSymlinks` oracle at line 94;
  - keep the F13 carve-out path-based (`invowk_modules/`).
- [ ] 5.2 Rewrite `TestModulePathContainment_F16Junction` to `Require` junctions and to assert containment, with no verdict that is only logged, as described in design D7. Assert:
  - `IsModule` and `Load`;
  - the script read;
  - both copies: the outside sentinel is absent from each copy;
  - `computeModuleHash`: the hash is unchanged when only the outside sentinel changes.
- [ ] 5.3 Reword the `probeF16` row's abstraction note: it is still `characterisation:` and asserts containment pending the Windows verdict. Run `make formal-golden` for `ModulePathContainment`, because its source text changed, then `make formal`.
- [ ] 5.4 Check, on Linux:
  - `go test -count=1 -run 'TestModulePathContainment_(Property|F16Junction)$' ./internal/app/moduleops/` passes: the property runs only the symlink branch, and the F16 leg skips under the policy;
  - the Windows compile checks from 4.4 pass.

## 6. Resolve the eight rows

- [ ] 6.1 In the `GetEffectiveWorkDir` row (`formal/alloy/ModulePathContainment.als:32`), set the binding to `TestModulePathContainment_ContainerWorkDirGolden`.
- [ ] 6.2 Custom-check replay. In `TestModulePathContainment_GoldenVectors`, replay `CustomCheckScript.ResolveWithFSAndModule` on every `ScriptRead` instance with a recording reader, mirroring `realScriptRead` (`module_path_containment_golden_test.go:133-148`). Require the model's accept and touched node, F13 instances included.
- [ ] 6.3 Env replay:
  - in `internal/testutil/mpctree/mpctree.go:97`, write regular files as `INVOWK_ATOM=content-<id>`; if that breaks another assertion, use an env-only override instead;
  - call `LoadEnvFile(env, path, moduleRoot)` on every `EnvRead` instance of an admitted module that `ValidateEnvFilePath` accepts;
  - require the model's accept and touched node, F17 instances included;
  - require an error for a non-file or missing target.
- [ ] 6.4 Unbind the `unpackRejectsEscape` row (`ModulePathContainment.als:39`). Set the binding to `-`, and rewrite the abstraction cell as described in design D9, "The two unbound rows", ending with the follow-up "bind `unpackInside` to `normalizeZIPPath` (survivor-triage)". Leave the element, symbol, and file cells unchanged.
- [ ] 6.5 Add `TestVirtualPathHarness_RootConstruction` to `internal/runtime/virtual_path_harness_formal_test.go`, as described in design D9, "The construction replay". It covers:
  - resolvers for access omitted, `restricted`, and `full`, each with and without a declared path;
  - acceptance of probe paths per location kind (`InReadRoot`, `InWorkdir`, `OutsideAll`), compared with `H.access = Full or locIsAllowedRoot[l, True]` (F15);
  - `EffectiveAccess` for omitted, `restricted`, and `full`.

  Bind the `allowedRoots` and `access` rows (`formal/alloy/VirtualPathHarness.als:30,32`) to it, then run `make formal-golden` for `VirtualPathHarness`.
- [ ] 6.5b (BLOCKED: design Open question 2) The `anchors` row (`VirtualPathHarness.als:31`):
  - under option A (recommended), set its binding to `-` and write the abstraction cell described in design D9, "The unbound rows";
  - under option B, add the per-OS anchor comparison to `TestVirtualPathHarness_RootConstruction` against the documented anchor layout, and bind the row to it.
- [ ] 6.6 Unbind the `Auth` row (`formal/tla/HostCallbackToken.tla:28`). Set the binding to `-`, and extend the abstraction cell as described in design D9: keep "TTL expiry is a nondeterministic action", and add the no-production-caller reason, the pointer to `Server.admitConn` (row `Auth/End atomicity`), and the survivor-triage follow-up. Do not add a `ValidateToken` call to any test.
- [ ] 6.7 Seed one defect at a time for 6.1, 6.2, 6.3, and 6.5 (and 6.5b under option B), confirm that the new binding fails, and revert each one. Append the results to each model's `calibration`. The defects:
  - drop the `validateScriptPathContainment` call in `CustomCheckScript.ResolveWithFSAndModule`;
  - make `LoadEnvFile` ignore `basePath`;
  - make `GetEffectiveWorkDir` ignore the implementation-level `workdir`;
  - drop the declared paths from the roots in `newVirtualPathResolverForFilesystem`;
  - make `EffectiveAccess` return `full` for an omitted access mode;
  - under option B only: drop the XDG override in `standardVirtualAnchorsForOS`.
- [ ] 6.8 Run `make formal-reach`:
  - it must pass, with every bound pair reached and every row unbound by 6.4, 6.5b, and 6.6 listed as `no-binding`;
  - seed a reach regression by temporarily removing the `LoadEnvFile` call: exactly that pair must fail, then revert it;
  - update `[reach] calibration` and the measured budget, and recompute the step and job timeouts if the measured time exceeds 45 s (design D3).
- [ ] 6.9 Check:
  - `python3 scripts/formal_mutation.py triage --check` passes with no ledger edits;
  - `python3 scripts/formal.py snapshot --compare <scratch>/baseline.json` shows every verdict and golden instance digest identical, except entries added by a recorded F16 or F18 outcome.

## 7. Documentation

- [ ] 7.1 `formal/README.md`:
  - correct the junction coverage sentence at line 125 (design D8), and the matching calibration text in `formal/manifest.toml`;
  - replace the F16 paragraph (lines 201-206) with "decided on Windows CI; outcome pending" until group 8 records the outcome;
  - add `make formal-reach` to Running, and a "Binding reach" bullet to "How a model earns trust";
  - rewrite the Mutation testing paragraph at lines 352-356, covering the maintainer's cross-cutting decision:
    - the reach gate found eight rows, not seven, including `ValidateToken`, which has no production caller;
    - how many are rebound: five, or six under option B of Open question 2;
    - which are unbound (`Auth`, `unpackRejectsEscape`, and `anchors` under option A), with their reasons;
  - add the reach step to the CI budgets table, and a pending Windows replay budget note (design D12).
- [ ] 7.2 `.agents/skills/formal-verification/SKILL.md`:
  - add the reach gate to the workflow and Verification sections;
  - replace "skipped and counted" (lines 93-96) with the capability policy;
  - state that binding tests skip only through `fstree` helpers;
  - add the junction oracle pitfall: never use `EvalSymlinks` for mount points;
  - add the rule that rows whose function does not carry the model's claim are unbound, not given a direct-call test.
- [ ] 7.3 `.agents/rules/commands.md`: add `formal-reach` to the Formal row of the targets table (line 11).
- [ ] 7.4 `tasks/next/formal-binding-gaps.md`:
  - mark each of the eight rows rebound or unbound, with the reason for each unbound row;
  - flag their baselined survivors for retriage;
  - note that the rerun evidence of all 419 survivors is stale by the digest convention;
  - add the survivor-triage follow-up as future work: the 419 survivors, a baseline refresh, re-pointing `Auth` to `Server.admitConn`, a `normalizeZIPPath` row for `unpackInside`, per-OS anchors in `VirtualPathHarness.als` if option A of Open question 2 was chosen, the Alloy-only "`validateDestinationPath` omitted" calibration claim (design Risks), and renaming the unresolved model elements if option A of Open question 1 was chosen.
- [ ] 7.5 Check: `make check-agent-docs` passes.

## 8. Windows CI evidence (open until the runs exist)

- [ ] 8.1 Push. On the PR's windows-latest run, read the verdicts of `TestCreateJunction`, the property's junction branch, and the F16 leg, and apply exactly one outcome from design D7:
  - (a) creation fails: iterate on group 4;
  - (b) escape: record F16 through the finding procedure, with a manifest `finding = "F16"` command next to a passing fix configuration, a README Findings row, the leg flipped to assert the escape, and an F16 carve-out in the property. Run `make formal-golden` if the `.als` changed;
  - (c) contained: record F16 as refuted in the README, naming each operation's modelled fact and its dropping mutant; stop if no modelled fact explains an operation.

  In every case, write the outcome and the CI run id into `ModulePathContainment`'s calibration, and record in design D5 which step fixed creation.
- [ ] 8.2 After the first three windows-latest runs of PR #163 that include the group 6 replays:
  - set the Windows golden-replay budget to `ceil(1.5 × max)` of their `ModulePathContainment` golden times;
  - record the value, the three run ids, and the note that nothing enforces it in `formal/README.md` (design D12).
- [ ] 8.3 Confirm that the formal lane's `Binding reach` step passes on the PR, and record its measured CI seconds for the later `budget_source = "ci"` promotion.
- [ ] 8.4 Check: the PR's windows-latest job shows `TestCreateJunction`, `TestModulePathContainment_Property`, and `TestModulePathContainment_F16Junction` as run, not skipped.

## 9. Completion gates (`.agents/rules/checklist.md`)

- [ ] 9.1 Review the changed code for reuse, simplification, and efficiency (`/simplify`), and fix what you find.
- [ ] 9.2 `make tidy`, then `make license-check` for the new Go files.
- [ ] 9.3 `make lint`, fixing all issues.
- [ ] 9.4 `make test`, the full suite, not short.
- [ ] 9.5 `make check-baseline`. Triage every new goplint finding, and ask the user where a fix in Invowk and a fix in the analyzer are both plausible.
- [ ] 9.6 `make check-file-length`.
- [ ] 9.7 `make sonar-local`. Suppress false positives in both `sonar-project.properties` and `.sonarcloud.properties`.
- [ ] 9.8 Conditional checks:
  - `make test-scripts` (scripts changed);
  - `python3 scripts/formal.py correspondence` and `make formal` (formal-bound code and tables changed);
  - `make check-agent-docs` (skill and rules changed);
  - the `GOOS=windows` vet and compile checks from 4.4 on the final tree.
- [ ] 9.9 `make formal-reach` on the final tree.
- [ ] 9.10 `/learn`.
