## 0. Ground rules

- [x] 0.1 Work only in the `feat/harden-formal-instruments` worktree, and observe these limits:
  - Do not run any `make mutation-*` target: its wrapper reverts uncommitted tracked files.
  - Make no production Go code changes. Only these may change:
    - tests and `internal/testutil/{fstree,mpctree}`;
    - scripts, the Makefile, and the workflow;
    - the manifest, the correspondence tables, and docs.
  - Run heavy commands under the shared heavy-command lock when one is in use: `make test`, `make lint`, `make formal`, `make formal-reach`, goplint targets, and commits.
- [x] 0.2 Before any change, record `python3 scripts/formal.py snapshot --out <scratch>/baseline.json`, outside `artifacts/`.
- [x] 0.3 Stop rule (design D9, "Defect rule"). Apply it when a strengthened or new binding fails on unmodified code and no open finding (F13, F15, F17) explains the failure:
  - First confirm in the model source that the model makes that claim for that operation kind. A transcription error in the replay is fixed, not recorded.
  - If the claim is real, stop and report it as candidate F18. Never weaken an assertion, and never edit product code.

  The same rule applies to an F16 outcome (c) with no modelled reason (design D7).
  It also applies when a recorded F18 would add a `no-binding` row: that row would grow `[triage] binding_gaps`, which the base check rejects (design Risks, decision 15). Report it instead of adding the entry. A new characterisation-only row is by design and passes.
- [x] 0.4 Check: `test -s <scratch>/baseline.json`.

## 1. Reach gate: plan and subcommand

- [x] 1.1 `scripts/formal_mutation.py`:
  - record each row's own killers in `functions[].rows[].killers`, next to the per-function union;
  - bump `PLAN_VERSION` (line 58) to 2;
  - update the plan fixtures and the golden plan in `scripts/test_formal_mutation.py`.
- [x] 1.2 Implement `reach [--out DIR]` (default `artifacts/formal/reach/`), as described in design D1:
  - run each killer alone with `-count=1 -json -covermode=set -coverpkg -coverprofile`, with `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME` pinned exactly as the mutation profile pins them, in parallel through `formal.parallel_jobs()`;
  - check exit status, outcome (`test_outcomes`), and profile header;
  - check every (row, killer, function) block inside the function's range;
  - write `reach.json` and a text summary that list unbound rows with their reason and abstraction cell, and print a `timing` line through `formal.report_timings`.
- [x] 1.3 Add unit tests to `scripts/test_formal_mutation.py`, with fixture profiles and a stub `GO_CMD`. Cover:
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
- [x] 1.4 `scripts/formal.py`:
  - add the `[reach]` manifest table (`budget_seconds`, `budget_source`, `calibration`) to `validate_manifest` (line 277) and `manifest_budgets` (line 442), and reject unknown keys with `check_keys`;
  - add `scripts/test_formal.py` cases for a valid table, an unknown key, and a soft overrun that only warns.
- [x] 1.5 Add a `formal-reach` Make target (`python3 scripts/formal_mutation.py reach`) and its help line, next to the other `formal-*` targets. `lint-scripts` lints shell scripts only, so it needs no change.
- [x] 1.6 **Calibration, before any rebinding.** Run `make formal-reach`.
  - It must fail and name exactly the eight (row, killer) pairs in design Context 1, with no tooling errors. If it names any other pair, stop and report.
  - Record the output summary and the wall time in `[reach] calibration`.
  - Set `budget_seconds = ceil(3 × local seconds)` and `budget_source = "local"` (design D3).
- [x] 1.7 Check: `python3 scripts/test_formal_mutation.py` and `python3 scripts/test_formal.py` pass.

## 2. Reach gate: CI wiring

- [x] 2.1 `.github/workflows/formal-verification.yml`:
  - add a `Binding reach` step after `Replay golden vectors and property tests`, with `if: steps.affected.outputs.run != 'false'`, `run: make formal-reach`, and `timeout-minutes` from design D3 (6 at 45 s local);
  - raise the job `timeout-minutes` from 46 to the D3 value (52 at a 6-minute step);
  - leave the upload step unchanged: it already uploads `artifacts/formal/`.
- [x] 2.2 Update `scripts/test_formal.py`: the expected step list, the job-timeout sum, and the `affected` condition for the new step.
- [x] 2.3 Add `scripts/formal_mutation.py` and `scripts/test_formal_mutation.py` to `[ci] paths` in `formal/manifest.toml`. Add a `scripts/test_formal.py` case in which a diff touching only `scripts/formal_mutation.py` classifies as `run=true`.
- [x] 2.3b (2026-11-15: the base wiring is superseded by 8b.4. 2026-11-14: `Runner self-tests` runs `scripts/test_formal_mutation.py`; a `Triage lists` step runs `triage --check --base "${BASE_SHA:-$GITHUB_SHA}"` after it; `[ci] paths` gained the triage ledger directory and the formal-bindings baseline; `test_workflow_runs_the_triage_checks` and `test_ci_paths_cover_the_triage_inputs` check the wiring.) (decision 15) Wire the triage checks into `.github/workflows/formal-verification.yml` (design D13, "CI"):
  - add `python3 scripts/test_formal_mutation.py` to `Runner self-tests`;
  - add a `Triage lists` step after it, under the `affected` condition, running `python3 scripts/formal_mutation.py triage --check --base "$BASE"`, with `$BASE` the PR base SHA on `pull_request` and `$GITHUB_SHA` otherwise; keep the classify step's depth-1 fetch of the base;
  - add `tools/mutation/triage/**` and `tools/mutation/baselines/formal-bindings-baseline.json` to `[ci] paths`;
  - extend `scripts/test_formal.py`: the step list, both commands, the `affected` condition, the `--base` wiring, and the new `[ci] paths`.
- [x] 2.4 Check: `python3 scripts/test_formal.py` passes.

## 3. Capability policy and skip guard

- [x] 3.1 `internal/testutil/fstree`, as described in design D4:
  - add `Capability` with the per-GOOS required policy, reading GOOS and `GITHUB_ACTIONS` through an injectable function;
  - make `Caps` keep `SymlinkErr` and `JunctionErr`, and make `probeSymlink` and `probeJunction` return errors (off Windows, `errNoJunctions`);
  - add `Require(t, cap)`, and make `Cache.Get` fail on a capability that is required but missing;
  - add `MustSymlink` and `MustJunction` for `Fataler`s;
  - add SPDX headers to new files.
- [x] 3.2 Add a policy unit test in `fstree`, in a file without a binding-test suffix. Make it table-driven over `linux`, `darwin`, and `windows`, with `GITHUB_ACTIONS` set and unset and with injected probe results. It checks that a required-but-missing capability fails, that every other missing capability skips, and that each message carries the GOOS and the probe error.
- [x] 3.3 Replace every raw skip in a binding-test file with the policy helpers. These are the 8 sites listed in design D4:
  - `internal/app/moduleops/module_path_containment_formal_test.go:23,108,118`;
  - `internal/app/moduleops/module_path_containment_rapid_test.go:27,72`;
  - `internal/runtime/virtual_path_harness_golden_test.go:87`;
  - `internal/runtime/virtual_path_harness_formal_test.go:25`;
  - `internal/runtime/virtual_path_harness_rapid_test.go:49`.
- [x] 3.4 `scripts/formal.py correspondence`: fail on a `.Skip(`, `.Skipf(`, or `.SkipNow(` call in a `BINDING_TEST_SUFFIXES` file, naming the file and line. Add `scripts/test_formal.py` fixtures showing that a binding file fails and a plain `_test.go` passes.
- [x] 3.5 Check: `go test ./internal/testutil/fstree/`, `python3 scripts/formal.py correspondence`, and `python3 scripts/test_formal.py` pass.

## 3b. Model-element check (design D11; decisions 7 and 10)

- [x] 3b.1 Implement the leading-identifier check in `check_correspondence`. It is a whole-word, case-sensitive match against the comment-stripped model source and the model's manifest command `name` and `body` values. A row that does not resolve fails as `<model>: row <element> names a model element that is not in <file> or its manifest commands`.
- [x] 3b.2 Add `[correspondence] unresolved_elements` to `formal/manifest.toml`:
  - regenerate the `"<model>: <element>"` pairs on the implementation tree; expect today's 34 pairs (38 rows, design Context 4), and stop and report if the set differs;
  - validate the table in `validate_manifest`, rejecting other keys with `check_keys`;
  - report a listed pair instead of failing it;
  - fail a stale entry: one that resolves, or that names no row.
- [x] 3b.3 Add `scripts/test_formal.py` fixtures for a resolving row, a row resolved only by a manifest command `body`, a case-only mismatch that fails, an unresolved unlisted row, a listed row, a stale entry, an entry with no row, and an unknown `[correspondence]` key.
- [x] 3b.4 Check: `python3 scripts/formal.py correspondence` and `python3 scripts/test_formal.py` pass.

## 4. Junction creation (Windows; compile-checked locally)

- [x] 4.1 Add a Windows-only `internal/testutil/fstree/links_windows_test.go` with `TestCreateJunction`, as described in design D5 step 1. It requires creation and checks it functionally: the link is not `ModeSymlink`, and `os.SameFile` holds through the link. On failure it prints the probe error.
- [x] 4.2 Compare `writeMountPointReparse` (`links_windows.go:76-99`) field by field with Go 1.27's `src/os/os_windows_test.go` (`createMountPoint` and `reparseData`). Compare the buffer layout, both NULs, `ReparseDataLength`, the input length, and the `CreateFile` flags. Fix every divergence, and note the comparison in a code comment. The missing print-name NUL is only a hypothesis.
- [x] 4.3 Add the `cmd /c mklink /J` fallback, used only when the reparse write fails. When both fail, report both errors. (2026-11-14: superseded by decision 12; removed in 4.3b.)
- [x] 4.3b (2026-11-14: `createJunctionVia`, its method constants, and the `context`/`os/exec`/`strings` imports are gone; `createJunction` writes the reparse point only; `TestCreateJunction` calls it and keeps the functional check; `GOOS=windows` vet and `go test -c` pass for fstree, moduleops, and runtime.) (decision 12) Remove the `mklink /J` fallback: `createJunctionVia`, its method constants, and the `exec`/`context`/`strings` imports it alone used, so `createJunction` and `probeJunction` call the reparse writer only. `TestCreateJunction` keeps requiring the reparse writer and checks it functionally. Remove the fallback from the code comments, design D5, the `module-path-containment-model` delta, and the skill.
- [x] 4.4 Check, locally:
  - `GOOS=windows go vet ./internal/testutil/fstree/ ./internal/app/moduleops/ ./internal/runtime/`;
  - `GOOS=windows go test -c -o /dev/null` for each of those three packages.

  Runtime verification is deferred to Windows CI (group 8).

## 5. Junctions in the property and the F16 leg

- [x] 5.1 `TestModulePathContainment_Property`, as described in design D6:
  - draw symlink or junction where junctions are available;
  - make junctions target directories, with the script path `<rel>x/secret`;
  - judge escape for both kinds by `os.SameFile` against a unique outside sentinel, backed by content, and replace the `EvalSymlinks` oracle at line 94;
  - keep the F13 carve-out path-based (`invowk_modules/`).
- [x] 5.2 Rewrite `TestModulePathContainment_F16Junction` to `Require` junctions and to assert containment, with no verdict that is only logged, as described in design D7. Assert:
  - `IsModule` and `Load`;
  - the script read;
  - both copies: the outside sentinel is absent from each copy;
  - `computeModuleHash`: the hash is unchanged when only the outside sentinel changes.
- [x] 5.3 Reword the `probeF16` row's abstraction note: it is still `characterisation:` and asserts containment pending the Windows verdict. Run `make formal-golden` for `ModulePathContainment`, because its source text changed, then `make formal`.
- [x] 5.4 Check, on Linux:
  - `go test -count=1 -run 'TestModulePathContainment_(Property|F16Junction)$' ./internal/app/moduleops/` passes: the property runs only the symlink branch, and the F16 leg skips under the policy;
  - the Windows compile checks from 4.4 pass.

## 6. Resolve the eight rows

- [x] 6.1 In the `GetEffectiveWorkDir` row (`formal/alloy/ModulePathContainment.als:32`), set the binding to `TestModulePathContainment_ContainerWorkDirGolden`.
- [x] 6.2 Custom-check replay. In `TestModulePathContainment_GoldenVectors`, replay `CustomCheckScript.ResolveWithFSAndModule` on every `ScriptRead` instance with a recording reader, mirroring `realScriptRead` (`module_path_containment_golden_test.go:133-148`). Require the model's accept and touched node, F13 instances included.
- [x] 6.3 Env replay:
  - in `internal/testutil/mpctree/mpctree.go:97`, write regular files as `INVOWK_ATOM=content-<id>`; if that breaks another assertion, use an env-only override instead;
  - call `LoadEnvFile(env, path, moduleRoot)` on every `EnvRead` instance of an admitted module that `ValidateEnvFilePath` accepts;
  - require the model's accept and touched node, F17 instances included;
  - require an error for a non-file or missing target.
- [x] 6.4 Unbind the `unpackRejectsEscape` row (`ModulePathContainment.als:39`). Set the binding to `-`, and rewrite the abstraction cell as described in design D9, "The unbound rows", ending with the follow-up "bind `unpackInside` to `normalizeZIPPath` (survivor-triage)". Leave the element, symbol, and file cells unchanged.
- [x] 6.5 Add `TestVirtualPathHarness_RootConstruction` to `internal/runtime/virtual_path_harness_formal_test.go`, as described in design D9, "The construction replay". It covers:
  - resolvers for access omitted, `restricted`, and `full`, each with and without a declared path;
  - acceptance of probe paths per location kind (`InReadRoot`, `InWorkdir`, `OutsideAll`), compared with `H.access = Full or locIsAllowedRoot[l, True]` (F15);
  - `EffectiveAccess` for omitted, `restricted`, and `full`.

  Bind the `allowedRoots` and `access` rows (`formal/alloy/VirtualPathHarness.als:30,32`) to it, then run `make formal-golden` for `VirtualPathHarness`.
- [x] 6.5b (2026-11-14: superseded by decision 14; see 6.5c.) Unbind the `anchors` row (`VirtualPathHarness.als:31`, decision 8). Set its binding to `-` and write the abstraction cell described in design D9, "The unbound rows": the model abstracts anchors into `InReadRoot`, the construction replay checks only that anchor roots are read roots, and per-OS anchors in the model are the follow-up. Leave the element, symbol, and file cells unchanged. Add no anchor-layout comparison to any test.
- [x] 6.5c (2026-11-14: the `anchors` row binds `TestVirtualPathHarness_RootConstruction`; the replay accepts `@work` with the other anchors and denies `@home/.ssh/invowk-home-probe-<tag>` under restricted and omitted access; `VirtualPathHarness` golden regenerated, 14580 instances; the reach gate shows `standardVirtualAnchorsForOS` reached, 10/21 statements.) (decision 14) Bind the `anchors` row (`VirtualPathHarness.als:31`) to `TestVirtualPathHarness_RootConstruction` with the narrow claim "anchors are explicit allowed roots":
  - add `@work` to the anchor probes that must be accepted;
  - add the `@home` negative from `cross-platform-path-anchors` "Anchor allow roots are explicit": a probe at `@home/.ssh/<tag>` that no other anchor encloses, rejected under restricted and omitted access and accepted under full; fail if `@home` is empty (an anchor enclosing the probe is a defect the comparison reports, as the seeded `@config` = home defect shows);
  - rewrite the row's abstraction cell: anchor paths are inputs from the production resolver, not verified; per-OS anchors in the model are the follow-up;
  - run `make formal-golden` for `VirtualPathHarness` (its source text changes).
- [x] 6.6 Unbind the `Auth` row (`formal/tla/HostCallbackToken.tla:28`). Set the binding to `-`, and extend the abstraction cell as described in design D9: keep "TTL expiry is a nondeterministic action", and add the no-production-caller reason, the pointer to `Server.admitConn` (row `Auth/End atomicity`), and the survivor-triage follow-up. Do not add a `ValidateToken` call to any test.
- [x] 6.7 (2026-11-14: four of five planned defects fail their binding and are recorded in the calibrations; dropping the `validateScriptPathContainment` call survives, masked by `ScriptFilePath.Validate`; decision 11 swaps it. Round 5, same day: the module's-parent resolution is the recorded custom-check defect and the masked one is recorded as redundant at that entry point in the `ModulePathContainment` calibration and `tasks/next/formal-binding-gaps.md`; `@home` added to the roots and the `@config` anchor set to home each fail the construction replay with 4 disagreements, recorded in the `VirtualPathHarness` calibration; every seed reverted, `git status` clean for non-test files.) Seed one defect at a time for 6.1, 6.2, 6.3, 6.5, and 6.5c, confirm that the new binding fails, and revert each one. Append the results to each model's `calibration`. The defects:
  - resolve the custom-check script against the module's parent in `CustomCheckScript.ResolveWithFSAndModule` (decision 11); record the planned defect (drop its `validateScriptPathContainment` call) as "redundant at this entry point: fully masked by `ScriptFilePath.Validate`" in the `ModulePathContainment` calibration and in `tasks/next/formal-binding-gaps.md`, not in the ledger;
  - make `LoadEnvFile` ignore `basePath`;
  - make `GetEffectiveWorkDir` ignore the implementation-level `workdir`;
  - drop the declared paths from the roots in `newVirtualPathResolverForFilesystem`;
  - add `@home` to the allowed roots in `newVirtualPathResolverForFilesystem` (decision 14);
  - set the `@config` anchor to the home directory itself in `standardVirtualAnchorsForOS` (decision 14);
  - make `EffectiveAccess` return `full` for an omitted access mode.

  After each revert, `git status --short -- cmd internal pkg` must list no non-test file.
- [x] 6.8 Run `make formal-reach`:
  - it must pass, with every bound pair reached and every row unbound by 6.4, 6.5b, and 6.6 listed as `no-binding`;
  - seed a reach regression by temporarily removing the `LoadEnvFile` call: exactly that pair must fail, then revert it;
  - update `[reach] calibration` and the measured budget, and recompute the step and job timeouts if the measured time exceeds 45 s (design D3).
- [x] 6.9 Unplanned-row and orphan list (design D13, decision 9), after 6.4, 6.5b, and 6.6:
  - factor the row classification out of `build_plan` into a helper that `triage --check` also calls, with no `go list`;
  - add the `[triage]` table (`unbound_rows`, `orphaned_survivors`) to `validate_manifest`, rejecting other keys;
  - in `check_triage`, fail an unlisted unbound row or orphan, fail a stale entry, and print the listed entries;
  - seed the list from the plan on this tree; expect 54 rows (the 51 of D13 plus `Auth`, `unpackRejectsEscape`, `anchors`) and 42 orphans (`Auth` 8, `unpackRejectsEscape` 12, `anchors` 22), and stop and report if either set differs;
  - add the `scripts/test_formal_mutation.py` fixtures listed in D13.
- [x] 6.9b (2026-11-14: four rows moved to `type-symbol` (`Serverbase` `State`, `ScopeConstruction` `globalChildren` and `RootSrc`, `Watch` `backend`); reseeded with 19 binding gaps and 20 orphans, 34 by-design rows reported; `--base` implemented with an injectable reader; `GrowthTests` and the new `UnplannedTests` cases pass.) (decision 15) Narrow and guard the `[triage]` lists (design D13):
  - in `classify_rows`, classify a symbol that names no function as `type-symbol` before the binding check; expect `Serverbase` `State`, `ScopeConstruction` `globalChildren` and `RootSrc`, and `Watch` `backend` to move;
  - rename `unbound_rows` to `binding_gaps` and list only `no-binding` rows; derive and report the by-design rows; a listed row that is not `no-binding` is stale;
  - add `triage --check --base <sha>`: read `git show <sha>:formal/manifest.toml` and fail on any `unresolved_elements`, `binding_gaps`, or `orphaned_survivors` entry absent at the base; default to `git merge-base HEAD origin/main`; pass with a notice when the readable base lacks a list; fail closed, naming the base and `--base`, when the base cannot be determined or read; inject the reader so unit tests never call git;
  - reseed: expect 19 gaps and 20 orphans (`Auth` 8, `unpackRejectsEscape` 12) and 34 by-design rows; stop and report if the sets differ;
  - add `scripts/test_formal_mutation.py` fixtures: a new by-design row passes, the reclassified type symbol, a listed by-design row is stale, growth at the base fails for each list, a shrink passes, a base without the lists passes as seeding, an unreadable base fails closed.
- [x] 6.10 (2026-11-14, Round 5: `triage --check` and `--base HEAD` pass, removing a listed entry fails; `scripts/test_formal_mutation.py` passes, 73 tests; correspondence passes; `make formal-reach` passes, 54 of 54 triples, 5.6 s; `snapshot --compare` differs only by the three F16 commands. Live check of the growth gate through `main()`: unbinding the `Serverbase` `Starting steps` row and listing it and its 6 survivors makes `triage --check --base HEAD` exit 1 with exactly 7 "not listed at the base" failures; reverted. On PR CI run 43667156165 the `Triage lists` step passed against base 17e364ed with the three seeding notices.) Check:
  - `python3 scripts/formal_mutation.py triage --check` passes with no `[[survivor]]` or `[[closed]]` edits, and fails when one listed entry is removed; `triage --check --base HEAD` passes;
  - `python3 scripts/formal.py correspondence` and `make formal-reach` pass, with `anchors` reached by `TestVirtualPathHarness_RootConstruction`;
  - `python3 scripts/test_formal_mutation.py` passes;
  - `python3 scripts/formal.py snapshot --compare <scratch>/baseline.json` shows every verdict and golden instance digest identical, except entries added by a recorded F16 or F18 outcome.

## 7. Documentation

- [x] 7.1 `formal/README.md`:
  - correct the junction coverage sentence at line 125 (design D8), and the matching calibration text in `formal/manifest.toml`;
  - replace the F16 paragraph (lines 201-206) with "decided on Windows CI; outcome pending" until group 8 records the outcome;
  - add `make formal-reach` to Running, and a "Binding reach" bullet to "How a model earns trust";
  - rewrite the Mutation testing paragraph at lines 352-356, covering the maintainer's cross-cutting decision:
    - the reach gate found eight rows, not seven, including `ValidateToken`, which has no production caller;
    - how many are rebound: five (decision 8);
    - which are unbound (`Auth`, `unpackRejectsEscape`, and `anchors`), with their reasons;
  - describe the `[correspondence] unresolved_elements` and `[triage]` lists, and that both only shrink;
  - add the reach step to the CI budgets table, and a pending Windows replay budget note (design D12).
- [x] 7.2 `.agents/skills/formal-verification/SKILL.md`:
  - add the reach gate to the workflow and Verification sections;
  - replace "skipped and counted" (lines 93-96) with the capability policy;
  - state that binding tests skip only through `fstree` helpers;
  - add the junction oracle pitfall: never use `EvalSymlinks` for mount points;
  - add the rule that rows whose function does not carry the model's claim are unbound, not given a direct-call test;
  - add the model-element check and the two shrink-only lists, and how to clear an entry.
- [x] 7.3 `.agents/rules/commands.md`: add `formal-reach` to the Formal row of the targets table (line 11).
- [x] 7.4 `tasks/next/formal-binding-gaps.md`:
  - mark each of the eight rows rebound or unbound, with the reason for each unbound row;
  - flag their baselined survivors for retriage;
  - note that the rerun evidence of all 419 survivors is stale by the digest convention;
  - add the survivor-triage follow-up as future work: the 419 survivors, a baseline refresh, re-pointing `Auth` to `Server.admitConn`, a `normalizeZIPPath` row for `unpackInside`, per-OS anchors in `VirtualPathHarness.als` (decision 8), the Alloy-only "`validateDestinationPath` omitted" calibration claim (design Risks), renaming the 34 unresolved model elements and emptying `unresolved_elements` (decision 7), and emptying `[triage] orphaned_survivors` (decision 9).
- [x] 7.4b (2026-11-14: README triage and mutation paragraphs, the Windows budget set to pending, the skill's narrow-claim, triage, masked-defect, and no-fallback guidance, and the gaps document updated.) (decisions 11-15) Update `formal/README.md`, `.agents/skills/formal-verification/SKILL.md`, and `tasks/next/formal-binding-gaps.md`:
  - six rows rebound and two unbound, with `anchors` bound under its narrow claim, and 20 orphaned survivors;
  - the `[triage]` lists: binding gaps only, by-design rows derived, the type-symbol rule, the base growth check and how to run it locally;
  - no `mklink /J` fallback;
  - the masked custom-check defect, recorded as redundant at that entry point;
  - the Windows replay budget pending three runs of the final tree, and no CI step enforcing the figure.
- [x] 7.5 Check: `make check-agent-docs` passes. (2026-11-14, Round 5: passes.)

## 8. Windows CI evidence (open until the runs exist)

- [x] 8.1 (2026-11-14: outcome (b), escape, applied from windows-latest run 43657442894, job 132639016048: `TestCreateJunction` passed; IsModule and Load admitted the module, the script read of `j/secret` returned the outside sentinel, both copies and the hash stayed contained; the property's junction draw escaped too. Recorded as `findingF16JunctionScan` with fix `scriptReadScanAllKinds`, the leg asserts the escape, the property carries the F16 carve-out, README and calibration updated. D5 attribution from run 43660058330: `created via reparse point`, so step 2 (the Go-matching writer) fixed creation; recorded in design D5.) Push. On the PR's windows-latest run, read the verdicts of `TestCreateJunction`, the property's junction branch, and the F16 leg, and apply exactly one outcome from design D7:
  - (a) creation fails: iterate on group 4;
  - (b) escape: record F16 through the finding procedure, with a manifest `finding = "F16"` command next to a passing fix configuration, a README Findings row, the leg flipped to assert the escape, and an F16 carve-out in the property. Run `make formal-golden` if the `.als` changed;
  - (c) contained: record F16 as refuted in the README, naming each operation's modelled fact and its dropping mutant; stop if no modelled fact explains an operation.

  In every case, write the outcome and the CI run id into `ModulePathContainment`'s calibration, and record in design D5 which step fixed creation.
- [x] 8.2 (2026-11-15, stage 4: the 3575b947 figure is withdrawn, because the stage-4 changes regenerate the `ModulePathContainment` golden; 8c.9 re-measures under the decision-32 head rule. 2026-11-14: the figure first recorded, 64 s from runs 43657442894, 43658445060, and 43660058330, came from earlier trees and is withdrawn by decision 13. 2026-11-15: three green windows-latest attempts of head 3575b947, CI run 43667156106 attempts 1-3 (jobs 132668974832, 132671916514, 132674591827), measured the `ModulePathContainment` golden at 37.17 s, 36.97 s, and 29.23 s; budget `ceil(1.5 x 37.17)` = 56 s, recorded in `formal/README.md` with the note that nothing enforces it.) After the last code push, get three windows-latest runs of that same head (re-run the Windows job with `gh run rerun <id> --job <job>` as needed; do not push while they run):
  - read each attempt's `ModulePathContainment` golden time (`gh run view <id> --attempt N --log`);
  - set the Windows golden-replay budget to `ceil(1.5 × max)`;
  - record the value, the measured head SHA, the run ids and attempts, and the note that nothing enforces it in `formal/README.md` (design D12), in a docs-only commit;
  - if three runs of one head cannot be obtained, leave this task open with a dated note.
- [x] 8.3 (2026-11-14: run 43657442830 passed `Binding reach`: 53 of 53 triples, `timing reach 37.1s`, a soft overrun warning against the local 36 s budget; well inside the 6-minute step limit. First CI sample for the `budget_source = "ci"` promotion.) Confirm that the formal lane's `Binding reach` step passes on the PR, and record its measured CI seconds for the later `budget_source = "ci"` promotion.
- [x] 8.4 (2026-11-14: run 43660058330 shows `Create junction`, `Create junction created via reparse point`, `ModulePathContainment F16 junction`, and `ModulePathContainment property` as run and passing.) Check: the PR's windows-latest job shows `TestCreateJunction`, `TestModulePathContainment_Property`, and `TestModulePathContainment_F16Junction` as run, not skipped.

## 8b. Round 6: rebase, triage base, and anchors cell (decisions 17-22)

- [x] 8b.1 (2026-11-15: rebased onto 3760a4ba without conflicts; F1 never changed the old `test-scripts` recipe, and `scripts/script-tests.sh` already lists `triage --check`. CI on the pre-rebase head caa44a4d: Formal Verification, Lint, CI, and CodeQL green; Benchmarks skipped.) Rebase onto `main` (S1, C1, and the script-test safety gate). Any F1 addition to the old `test-scripts` recipe lands as an entry in `scripts/script-tests.sh`. Check CI on the pre-rebase head once.
- [x] 8b.2 (2026-11-15: strict validate passes; the trial archive validates 32 specs, 0 failed, 0 warnings; copy deleted. The calibration scenario is split into three.) Spec first: design decisions 17-22, the `mutation-testing` base and seeding requirements, the split "Calibration on the pre-change tree" scenario, and the `anchors` cell scenario. Check: `openspec validate harden-formal-instruments --strict` and a trial archive whose `openspec validate --specs --strict` reports 0 failures and 0 warnings.
- [x] 8b.3 (2026-11-15: `scripts/test_formal_mutation.py` passes, 77 tests; `triage --check` passes without `--base`, with `--base origin/main`, and with `INVOWK_TRIAGE_BASE` empty or set to `HEAD`.) `scripts/formal_mutation.py` (decisions 18, 19, 22):
  - seed only from a base with none of the three lists; fail for each list a partial base lacks;
  - print why each entry left a list against the base;
  - take the base from `--base`, else a non-empty `INVOWK_TRIAGE_BASE`, else the merge-base, and name the variable in every fail-closed message.

  Add `scripts/test_formal_mutation.py` cases for each, and one that pins a `Type.Method` symbol as a function. Unit tests of the merge-base path scrub the variable. Check: `python3 scripts/test_formal_mutation.py`.
- [x] 8b.4 (2026-11-15: the gate self-test passes, 101 checks, including a control in which the unfetched base does not resolve; `make test-scripts` passes, 16 entries and 43 canaries, its `triage --check` using the merge-base inside the throwaway repository; a depth-1 pull-request checkout passes with the helper's base, 3760a4ba, and fails closed naming `INVOWK_TRIAGE_BASE` without it; the release tag checkout passes with the base equal to HEAD.) `scripts/ci-triage-base.sh` (decision 22): the full base SHA on stdout, notices on stderr, non-zero on a failed fetch. Wire it into the formal lane's `Triage lists` step, `lint.yml`'s `script tests (worktree-safety gate)` job, and `release.yml`'s `make test-scripts` step, each assigning and exporting `INVOWK_TRIAGE_BASE` in the step that runs the check. Add the helper to `[ci] paths`, update the `scripts/test_formal.py` wiring assertions, and add a gate self-test case in `scripts/test_check_script_tests_worktree_safety.sh` for a base that no ref reaches. Check: `make test-scripts`; the pull-request and release checkout simulations with the variable set as the workflows set it.
- [x] 8b.5 (2026-11-15: the regenerated `VirtualPathHarness` golden differs from the previous one only in its fingerprint; the 0.2 snapshot file is no longer in the scratch directory, so the golden contents were compared directly. Correspondence passes; `make formal-reach` passes, `timing reach 8.7s`.) The `anchors` row's abstraction cell lists what the replay catches (decision 17); regenerate the `VirtualPathHarness` golden. `tasks/next/formal-binding-gaps.md` records "bound is not covered" for the row's 22 survivors and the three re-point candidates (decision 18). Check: `python3 scripts/formal.py correspondence`, `make formal-reach`, and `python3 scripts/formal.py snapshot --compare` against the 0.2 baseline.

## 8c. Stage 4: decision 16 and the independent review (decisions 16, 27-33)

- [x] 8c.1 (2026-11-15: `openspec validate harden-formal-instruments --strict` valid.) Spec first: rename the F16 leg requirement to "The F16 leg asserts each operation's verdict" (decision 16); add "Copy and hash containment is modelled for junction modules", "Receiver-qualified symbols name a declared method", "Rows bound at the base stay bound or leave visibly", and "Unresolved rows cannot grow against the base"; narrow the `anchors` cell scenario; reword the Windows budget scenarios and the reach timeout scenario. Check: `openspec validate harden-formal-instruments --strict`.
- [x] 8c.2 (2026-11-15: the reviewer's `CallbackServer.RevokeToken` edit fails correspondence, the plan, and `triage --check`; the real tree reports 34 by-design rows; `scripts/test_formal.py` 113 tests and `scripts/test_formal_mutation.py` 85 tests pass.) Receiver check (decision 27): `scripts/formal.py correspondence` fails a `Recv.name` row whose file declares `name` as a function with no `Recv` receiver; `classify_rows` reports it as a problem and `triage --check` fails on it. Add `scripts/test_formal.py` and `scripts/test_formal_mutation.py` cases. Check: the reviewer's `CallbackServer.RevokeToken` edit fails correspondence, `triage --check`, and the plan; the real tree still reports 34 by-design rows.
- [x] 8c.3 (2026-11-15: checked live with a `characterisation:` demotion and a new `readOp | IsModule` row; against base 3760a4ba, which has none of the lists, the checks print notices only; the gate self-test (`make test-scripts`), the depth-1 pull-request simulation (base 3760a4ba), and the release simulation (base HEAD) pass, 419 survivors, 19 closed.) Base classification (decision 28): `triage --check` classifies the base tables through the git reader and fails a demoted row and a new row under a listed unresolved pair; deletions and re-points are notices; a base without the lists reports notices only. Add `scripts/test_formal_mutation.py` cases. Check: `triage --check` against `origin/main`; the gate self-test, the depth-1 pull-request checkout, and the release simulation.
- [x] 8c.4 (2026-11-15: goldens regenerated, `command_count` 180; `snapshot --compare` against the stage-4 baseline differs only by the three new verdicts; `make formal` passes on 6ce407b0.) Model (decision 29): add `copyHashScanInside`, `copyHashJunctionScan`, `anteCopyHashJunction`, and `mutantCopyFollowsJunction`; `make formal-golden` for `ModulePathContainment`. Check: `make formal`, and `snapshot --compare` against the stage-4 snapshot shows only the three new commands and the regenerated fingerprints.
- [x] 8c.5 (2026-11-15: `GOOS=windows go vet` and `go test -c` pass; `ModulePathContainment F16 junction` passes on windows-latest in CI run 43685015559 attempts 1-3, so every copy and hash succeeded.) F16 leg (decision 30): each copy and both hash computations must succeed. Check: `GOOS=windows go vet` and `go test -c` for `internal/app/moduleops`; the leg passes on windows-latest.
- [x] 8c.6 (2026-11-15: `go test ./internal/testutil/fstree/` passes.) `fstree` cache policy: give `Cache` an injectable host and probe result, and unit-test that a required missing capability fails `Get` with the GOOS and probe error and that one not required is counted as a skip. Check: `go test ./internal/testutil/fstree/`.
- [x] 8c.7 (2026-11-15: correspondence passes; `make formal-reach` passes, `timing reach 6.0s`; the `@tmp` mask confirmed: the replay passes with TMPDIR unset and fails with TMPDIR=/var/tmp.) `anchors` cell, D9, and `tasks/next/formal-binding-gaps.md` (decision 31); regenerate the `VirtualPathHarness` golden. Check: `python3 scripts/formal.py correspondence`, `make formal-reach`.
- [x] 8c.8 (2026-11-15: `scripts/test_formal.py` passes; CI run 43685015525 on 6ce407b0 adds a second reach sample, 36.3 s, a soft overrun of the 36 s local budget inside the 3-minute step.) Reach timeout and docs (decisions 16 and 33): step 3 minutes, job 49; `formal/README.md` Binding reach row, budget table, and `[reach] calibration` record CI run 43657442830 (37.1 s). Check: `python3 scripts/test_formal.py`.
- [x] 8c.9 (2026-11-15: head 6ce407b0, CI run 43685015559 attempts 1-3, `Test (windows)` jobs 132723566961, 132726133020, 132728380605: `ModulePathContainment` golden 23.39 s, 23.40 s, 39.39 s under `gotestsum -- -race -short`; budget `ceil(1.5 x 39.39)` = 60 s; `Test (macos)` job 132723566989: 5.47 s. Recorded in `formal/README.md`. `git diff 6ce407b0 HEAD -- internal pkg formal/alloy formal/tla formal/manifest.toml` is empty.) Windows and macOS budgets (decision 32): three windows-latest attempts and the `Test (macos)` job of one head whose golden-replay inputs equal the archived head's; record the figures, the head, the run ids, the flags, and the empty input diff in `formal/README.md`. Check: `git diff <measured head> HEAD -- <replay inputs>` is empty.

## 9. Completion gates (`.agents/rules/checklist.md`)

Owned by stage 4 (the completion stage), after groups 0-8. If any step changes code, restart from 9.1.

- [x] 9.1 (2026-11-15: one `/simplify` pass in stage 4; fixes applied.) Review the changed code for reuse, simplification, and efficiency (`/simplify`), and fix what you find.
- [x] 9.2 (2026-11-15: `make tidy` and `make license-check` ok.) `make tidy`, then `make license-check` for the new Go files.
- [x] 9.3 (2026-11-15: `make lint` 0 issues.) `make lint`, fixing all issues.
- [x] 9.4 (2026-11-15: full `make test` passes on 6ce407b0 plus docs.) `make test`, the full suite, not short.
- [x] 9.5 (2026-11-15: `make check-baseline` passes; no new goplint findings.) `make check-baseline`. Triage every new goplint finding, and ask the user where a fix in Invowk and a fix in the analyzer are both plausible.
- [x] 9.6 (2026-11-15: all Go files under 1000 lines.) `make check-file-length`.
- [x] 9.7 (2026-11-15: `sonar-local` passes in pre-commit; PR #163 SonarCloud Code Analysis passes.) `make sonar-local`. Suppress false positives in both `sonar-project.properties` and `.sonarcloud.properties`.
- [x] 9.8 (2026-11-15: `make test-scripts`, correspondence, `make formal`, `make check-agent-docs`, and the windows vet and compile checks pass.) Conditional checks:
  - `make test-scripts` (scripts changed);
  - `python3 scripts/formal.py correspondence` and `make formal` (formal-bound code and tables changed);
  - `make check-agent-docs` (skill and rules changed);
  - the `GOOS=windows` vet and compile checks from 4.4 on the final tree.
- [x] 9.9 (2026-11-15: `make formal-reach` passes, 6.0 s.) `make formal-reach` on the final tree.
- [x] 9.10 (2026-11-15: `/learn` run.) `/learn`.
