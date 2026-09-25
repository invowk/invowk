## 1. Pre-flight

- [x] 1.1 Confirm that `formal-infra-refinements` has landed (manifest-generated Alloy commands, golden format 2, unknown-key rejection), and branch this work from it in its own worktree
- [x] 1.2 Re-verify every enforcement point, line number, and the `Load`-caller classification (design D8.4) against the current `main`, and update design.md if code has moved
- [x] 1.3 Write throwaway Go reproductions for the suspected findings: F13 (vendored-subtree symlink), F14 (virtual deep `mkdir` into a writable outside dir), F15 (`workdir` widening under `restricted`), and D8.8 (junctions, on Windows). Keep a confirmed reproduction as the finding's replay. For one that does not reproduce, note that its id stays unused

## 2. ModulePathContainment model (predicates only; no commands in the .als)

- [x] 2.1 Create `formal/alloy/ModulePathContainment.als` (SPDX header) with the tree (`Dir`, `File`, `Symlink`, `Junction`), a symlinked module root, the `invowk_modules` subtree, the reserved `HarnessFile` atom, references (steps, `..`, absolute dialects), and the `lexAt` and `physAt` relations. Add the facts `chainsWithinTwo` (a cycle is an error) and `foldDistinctSiblings`, plus the `caseFold` flag
- [x] 2.2 Add per-layer implementation predicates (`validates`, `contains`, `accepts`, `touches`) for every operation in design D5.2, including the root-following copy of a symlinked source root
- [x] 2.3 Add the discovery-fact predicates `isModuleRejectsLinkedRoot`, `loadScansNonLinkedRoot`, `vendorSkipExact`, `discoverySkipsLinkedChildren`, and `copySkipsInnerLinks`, and their conjunction `discoveryFacts`
- [x] 2.4 Add the independent policy `contained[op]` and the allowed-location predicates `allowedWorkspace`, `allowedVendoredSelf`, `allowedVirtualRoot`, and `allowedWorkdirCwd`, with the `invowk_modules` rule that the target must resolve physically inside
- [x] 2.5 Add the predicates for the safety properties, their antecedents, and the mutant variants listed in the spec. Add the F13 current-code predicate (raw-module-path containment) and its fixed variant (physical containment)
- [x] 2.6 Add the witness predicates: in-module link, escaping link, chain of two, dangling link, two-link cycle, symlinked ancestor, symlinked module root, junction, case-fold alias, vendored child, and Windows-drive reference
- [x] 2.7 Add the correspondence table: every predicate, fact, and allowed-location row, the dead validators marked as having no production caller, and binding cells naming every golden, rapid, and formal test
- [x] 2.8 Add the `noJunk` predicate and the golden predicate recording every layer in the `Decisions` `one sig`

## 3. VirtualPathHarness model (predicates only)

- [x] 3.1 Create `formal/alloy/VirtualPathHarness.als` with the root-list construction (workdir, script base, anchors, temporary roots, declared paths), `normalizeExistingOrParent` (full, parent, lexical), `pathWithin`, the `restricted` and `full` access modes, and runtime `ln` links with literal targets
- [x] 3.2 Add the policy that transcribes `virtual-runtime-sandbox` "Traversal out of an allowed root is blocked" and `virtual-filesystem-access` "Restricted access limits virtual file I/O", with `allowedWorkdirCwd`
- [x] 3.3 Add the safety, antecedent, mutant (unevaluated comparison, dropped parent evaluation), witness, and F14/F15 current-code and fixed predicates, plus the correspondence table, `noJunk`, and the golden predicate

## 4. Manifest commands and generation

- [x] 4.1 Add both `[[model]]` entries to `formal/manifest.toml`, each with a default `scope`, `calibration`, and `[model.golden]`. Add every `[[model.command]]` with `body`, optional `scope`, `expect`, `property`, `antecedent`, `mutant_of`, `witness`, and `finding`. Use only keys the runner accepts
- [x] 4.2 Measure the golden instance count, the decoded size, and the Alloy time with a capped `-r` for candidate scopes, then pick the smallest scope that meets section 7
- [x] 4.3 Run `make formal-alloy` until every verdict matches, and run `python3 scripts/test_formal.py` (including the golden-command digest check)
- [x] 4.4 Run `make formal-golden` to generate the format-2 golden files `internal/app/moduleops/testdata/formal/module_path_containment_golden.json.gz` and `internal/runtime/testdata/formal/virtual_path_harness_golden.json.gz`

## 5. Test helper and golden replays

- [x] 5.1 Add `internal/testutil/fstree` (SPDX header). It materialises trees under `filepath.EvalSymlinks(t.TempDir())`, names module roots `m<i>.invowkmod`, and writes a fixed `invowkmod.cue` and `invowkfile.cue` template mapped to `HarnessFile`. It creates typed symlinks on Windows (dangling links as file links) and junctions on Windows only. It maps paths back to atoms with `os.SameFile`, and probes symlink, junction, and case-fold capability
- [x] 5.2 Add `pkg/invowkfile/export_test.go` exposing `validateScriptPathContainment` to the package's external tests. It is test-only, with no production change
- [x] 5.3 Add `TestModulePathContainment_GoldenVectors` in `internal/app/moduleops/module_path_containment_golden_test.go` (external package). It replays every operation in design D5.2, compares every layer decision and touched node, asserts the Go fact and policy transcriptions on every instance, logs the audit verdicts as advisory, and logs skip counts per capability
- [x] 5.4 Add `TestModulePathContainment_ContainmentLayerGolden` in `pkg/invowkfile/module_path_containment_golden_test.go` and `TestModulePathContainment_ContainerWorkDirGolden` in `internal/runtime/module_path_containment_golden_test.go`. Both read the shared golden file
- [x] 5.5 Add `TestVirtualPathHarness_GoldenVectors` in `internal/runtime/virtual_path_harness_golden_test.go`. It binds root construction through `standardVirtualAnchorsForOS`, and validation through a directly built `virtualPathResolver` over materialised roots. It fails if no instance has a node outside every root
- [x] 5.6 Keep every new Go file under 1000 lines, and split by operation kind if needed

## 6. Rapid properties

- [x] 6.1 Add `TestModulePathContainment_Property` in `internal/app/moduleops/module_path_containment_rapid_test.go`
- [x] 6.2 Add `TestVirtualPathHarness_Property` in `internal/runtime/virtual_path_harness_rapid_test.go`, covering runtime-created links and deep non-existent paths
- [x] 6.3 Name both in their correspondence binding cells, and verify them under `make formal-rapid-deep` (after task 9.1), with only recorded findings excluded

## 7. Cost budgets

- [x] 7.1 Measure the golden replay time in `make test` and the deep-rapid time on Linux, macOS, and Windows CI legs
- [x] 7.2 Confirm a replay time of 30 s or less per OS and a deep-rapid time of 60 s or less. Otherwise narrow the scope or generator, record the reason, and re-run calibration
- [x] 7.3 Record the measured values in design.md for `promote-formal-ci-gate`'s `budget_seconds`

## 8. Calibration and findings

- [x] 8.1 Seed each real-code defect listed in the spec one at a time. Confirm that a layer comparison, golden replay, or rapid property fails, and revert each seed before the next
- [x] 8.2 Widen the scope, or add a layer binding, for any surviving defect. Record the results in both `calibration` fields
- [x] 8.3 For each confirmed finding (F13, F14, F15 if confirmed, F16 onward), add the `finding` command, the passing fixed configuration, and a Go replay test (`TestModulePathContainment_F13…` in `module_path_containment_formal_test.go`, `TestVirtualPathHarness_F14…` and `…_F15…` in `virtual_path_harness_formal_test.go`), each named in a binding cell
- [x] 8.4 Record the refuted hypotheses (for example, the symlinked hash root, the env-file runtime read, and any finding id left unused) with the fact that refutes each and the mutant that exhibits it
- [x] 8.5 Do not fix any finding. Report the findings to the maintainer for separate fix changes

## 9. Wiring and documentation

- [x] 9.1 Add `./internal/app/moduleops/ ./internal/runtime/ ./pkg/invowkfile/` to `FORMAL_RAPID_PACKAGES` in `Makefile`
- [x] 9.2 Add those packages, and `ModulePathContainment|VirtualPathHarness` to the `-run` pattern, in the "Replay golden vectors and property tests" step of `.github/workflows/formal-verification.yml`. Extend its `paths` filter with the bound files listed in design D10, or add them to `[ci] paths` if `promote-formal-ci-gate` has landed
- [x] 9.3 Update `formal/README.md`: the Models table (with instance counts), the Findings rows F13 onward, and the refuted hypotheses and unused ids
- [x] 9.4 Document the fstree replay pattern, per-instance platform gating and skip counts, per-layer golden decisions, and the filesystem-replay budget in `.agents/skills/formal-verification/SKILL.md` (step 6 and Pitfalls)
- [x] 9.5 Fix `.agents/agents/supply-chain-reviewer.md:13,17,67,72` and `docs/next/cross-platform-windows-improvements.md:86`. Point the SC-01/SC-05 rows in `supply-chain-reviewer.md` and `.agents/skills/module-security/SKILL.md:348,352` at the new models. Run `make check-agent-docs`
- [x] 9.6 Run `scripts/formal.py correspondence`, `make formal`, `make test-scripts`, `make lint`, `make license-check`, `make check-file-length`, `make check-baseline`, and `make test`
