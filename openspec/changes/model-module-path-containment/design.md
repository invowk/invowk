## Context

**Depends on `formal-infra-refinements` (lands first).** This change is written against that change's machinery:
- Alloy commands are generated from `formal/manifest.toml` (`[[model.command]] body`, optional `scope`, and a default model `scope`); a `.als` that declares a command fails the run;
- golden files use format 2, and the fingerprint also covers the rendered golden command;
- the runner rejects unknown manifest keys.

The change is implemented after that one, in parallel with `trace-validate-token-and-lock` and `model-concurrent-lock-writes`, in its own worktree.

Module path containment is enforced by several independent layers. None of them checks the physical target of a path at the point of use. The table below lists every enforcement point this change binds, with line numbers at `main` 4831f23d.

| Operation a module can trigger | Enforcement | Location | Kind |
|---|---|---|---|
| `script.file` syntax | `ScriptFilePath.Validate`: rejects empty and NUL paths, all four absolute dialects, and `..` after `\`→`/` normalisation | `pkg/invowkfile/script_file_path.go:38` | lexical |
| `script.file` resolution | `ScriptFilePath.ResolveFromModule`: keeps absolute inputs raw, so containment fails closed | `pkg/invowkfile/script_file_path.go:62` | lexical |
| Script read (all runtimes) | `Script.Validate()` first, then `validateScriptPathContainment` (`filepath.Rel` against the raw module path), then `os.ReadFile`, which follows links | `pkg/invowkfile/implementation.go:445`, `:508`; `internal/runtime/script_resolver.go:46` | lexical |
| Script execution (native, container) | the interpreter gets `SelectedScriptFilePath()`, the same path, re-opened after the read | `internal/runtime/native.go:240`, `:454`; `internal/runtime/container_provision.go:315` | none (relies on the prior read) |
| Parse-time script check | `validateModuleScriptFileSelection` | `pkg/invowkfile/validation_structure_command.go:181`, `validation_structure_deps.go:205` | lexical |
| Custom-check script read | `s.Validate()`, then the same containment check | `pkg/invowkfile/dependency.go:327` | lexical |
| Env files | `ValidateEnvFilePath` at parse time; `LoadEnvFile` joins and reads with no runtime check | `pkg/invowkfile/validation_filesystem.go:111`, `validation_structure_deps.go:276`; `internal/runtime/dotenv.go:17` | lexical, parse time only |
| Containerfile | `ValidateContainerfilePath` | `pkg/invowkfile/validation_filesystem.go:62`, `validation_structure_command.go:205` | lexical |
| `workdir` | `WorkDir.Validate` checks only for whitespace; `GetEffectiveWorkDir` accepts absolute paths and `..` | `pkg/invowkfile/workdir.go:40`, `invowkfile.go:171`; `internal/runtime/container_provision.go:364` | none |
| Module root admission | `invowkmod.IsModule`: an Lstat check rejects a symlinked root; gates every discovery `Load` | `pkg/invowkmod/operations.go:53`; `internal/discovery/discovery_files.go:235`, `:385`, `:458`, `:572`; `internal/provision/helpers.go:165` | Lstat (discovery fact) |
| Module tree admission | `invowkmod.Load` → `ensureModuleDirectory` (`os.Stat`, which follows links) → `scanModuleTree` (`WalkDir`, which does not descend into a symlinked root) → `inspectModuleEntry`: any symlink invalidates the module; `invowk_modules` is skipped by exact name | `pkg/invowkmod/operations_validate.go:77`, `:171`, `:184`, `:188`, `:204`, `:240`; `invowkmod.go:550` | physical scan of a non-linked root (discovery fact) |
| Vendored discovery | `invowk_modules` is found with `os.Stat` (follows links); symlinked children are skipped; each child is gated by `IsModule` and then `Load`ed | `internal/discovery/discovery_files.go:512`, `:552`, `:572`, `:587` | Lstat per entry |
| Vendor and cache copy | `modulecache.copyDir` skips inner links, but follows a symlinked source root (`os.Stat` and `os.ReadDir`) | `internal/app/modulecache/cache.go:114`–`124`, `:137`; `internal/app/moduleops/vendor.go:75`, `:115` | Lstat (inner) |
| Provisioning copy | `DiscoverModules` and `collectProvisioningModule` skip symlinked dirs; `CopyDir` and `CopyFile` skip links; `CopyDir` follows a symlinked source root | `internal/provision/helpers.go:133`, `:212`, `:234`, `:177` | Lstat (inner) |
| Content hash | `computeModuleHash` hashes regular files only; a symlinked root is not rejected (`CalculateDirHash` rejects one at `helpers.go:48`) | `pkg/invowkmod/content_hash.go:118`, `:129` | Lstat |
| ZIP unpack | `normalizeZIPPath` and `validateDestinationPath` | `internal/app/moduleops/packaging.go:346`, `:521` | lexical |
| Virtual runtime access | roots from the workdir, the script base, `standardVirtualAnchorsForOS`, `defaultTempRoots` (hard-coded `/tmp`), and declared paths; `validate` → `normalizeExistingOrParent` (EvalSymlinks of the path, else of its parent, else lexical) → `pathWithin` | `internal/runtime/virtual_policy.go:103`–`115`, `:133`, `:148`, `:230`, `:318`, `:343` | physical, with a lexical fallback |
| Unused validator | `Module.ValidateScriptPath`, `ContainsPath`, `checkSymlinkSafety` (EvalSymlinks-aware); no production caller | `pkg/invowkmod/invowkmod.go:603`, `:640`, `:682` | dead code |
| Advisory audit | `scriptReadPath` (EvalSymlinks on both sides); `moduleSymlinkRef` (raw module path against the cleaned target) | `internal/audit/scan_files.go:66`; `scan_context_artifacts.go:257`, `:271` | advisory |

Script reads are safe today only because of how three layers compose:
1. The lexical containment check accepts a path that crosses an in-module symlink.
2. `Load` refuses to admit a module whose non-linked tree contains a symlink.
3. `IsModule` refuses a symlinked root before `Load` runs.

Neither the tests of any single layer nor the layers themselves expose this dependency. It has two exemptions:
- `invowk_modules/` is skipped by exact name, yet a parent module's `script.file` may point into it;
- `Load` callers that skip `IsModule` scan nothing for a symlinked root.

Each layer has its own tests, including symlink cases (`pkg/invowkmod/operations_validate_mutation_test.go`, `content_hash_test.go`, `internal/provision/helpers_test.go`, `internal/app/moduleops/vendor_test.go`, `internal/discovery/discovery_core_test.go`). No test checks their composition.

## Goals / Non-Goals

**Goals:**
- A bounded relational model of module path containment across every bound operation, with the policy stated independently of the code.
- Bindings that replay each golden instance on a real filesystem, so that `os.ReadFile`, `os.Lstat`, `filepath.EvalSymlinks`, and `filepath.WalkDir` behave as in production and are not re-modelled.
- Per-layer bindings, so that a layer masked by another layer is still calibrated.
- Calibration that shows the bindings catch plausible regressions.
- Findings F13 onward, recorded with reproductions. Refuted hypotheses are recorded with the fact that refutes them.
- Measured cost budgets on all three CI operating systems.

**Non-Goals:**
- Product fixes, including wiring or deleting `ValidateScriptPath` and `checkSymlinkSafety`. Each fix is a separate change made after maintainer approval, and keeps the pre-fix behaviour as a regression mutant.
- TOCTOU races, such as a symlink planted between `Load` and the read. That is a temporal property and would belong in a later TLA+ model.
- Paths the user controls: the root invowkfile, `--ivk-env-file`, `--ivk-workdir`, host `volumes` mounts (SC-03, by design), and `Load` calls on a path the user names directly (see D8.4).
- What a host process does after it is started.
- Windows ACLs and 8.3 short names. Junctions are in scope (D2).

## Decisions

### D1: Two models, not one
`ModulePathContainment.als` covers the static module operations. `VirtualPathHarness.als` covers the virtual resolver, which has a different root set, links created at run time, and its own fallback. One combined model would multiply the golden instance counts of both configuration spaces. Under format 2 the constraint is the golden instance count and decoded size, and replay time, rather than compressed bytes. Both are measured with a capped `-r` before either scope is fixed (D9).
*Alternative*: one model with a mode flag. Rejected, because of instance explosion and replay time. The `ScopeConstruction` experience also shows that trimming a feature from the golden scope to fit a budget hides defects.

### D2: Tree nodes plus step sequences, with resolution as relations
Nodes are `Dir`, `File`, and `Link`, related by `parent` and `name`. `Link` has two kinds, `Symlink` and `Junction`. A junction is a directory-only link with an absolute target. `os.Lstat` does not report it as `ModeSymlink` (Go 1.23 and later), so the transcribed symlink checks do not see it.

A reference is `seq Step` of at most 3 steps, where each step is `Down[Name]` or `Up`, plus an optional `Abs[Dialect]`. Lexical and physical resolution are relations constrained step by step (`lexAt`, `physAt`).

Link following is bounded at chain depth 2. A fact (`chainsWithinTwo`) forbids acyclic chains longer than 2 in every instance, so no model instance relies on a chain-length limit. Real `ELOOP` is modelled only as a cycle, which both Alloy and the OS resolve as an error. A two-link cycle is a witness, and it is replayed.

Case-insensitivity is a `one sig FS { caseFold: Bool }` flag with `Name.fold` classes. A fact (`foldDistinctSiblings`) states that when `caseFold` is set, no two siblings share a fold class, because APFS and NTFS cannot create them. Lookups use `fold` when the flag is set. Transcribed exact-name comparisons, such as `entry.Name() == VendoredModulesDir`, use name identity.

### D3: Per-layer decisions recorded in a `one sig`
Each `Op` atom has a kind, an owning module, and a reference. The implementation is transcribed one layer at a time:
- `validates[op]`: the syntactic value-type layer;
- `contains[op]`: the containment check;
- `accepts[op]`: the final effect;
- `touches[op]`: the physically reached node set.

The golden command records every layer in `Decisions`. Recording each layer lets calibration reach defects that a later layer masks (D5.3).

Discovery facts are separate named predicates, and `discoveryFacts` conjoins them:
- `isModuleRejectsLinkedRoot`;
- `loadScansNonLinkedRoot`;
- `vendorSkipExact`;
- `discoverySkipsLinkedChildren`;
- `copySkipsInnerLinks`.

The copy transcription also states that a copy of a symlinked source root follows that root. Each mutant drops exactly one fact.

### D4: Allowed locations are named predicates
These predicates apply the orchestrator's answers:
- `allowedWorkspace`: the container `/workspace` mount of the invowkfile directory.
- `allowedVendoredSelf`: a vendored child's own root, for the child's own operations.
- `allowedVirtualRoot`: anchors, temporary roots, and declared `filesystem.paths`.
- `allowedWorkdirCwd`: a declared `workdir` is an explicit allowed location for the execution working directory only. It is not a read target. Entering it into the virtual allowed-root list under `restricted` access is therefore outside the policy. That is candidate F15.
- `script.file` into `invowk_modules/` is allowed only if it resolves physically inside the declaring module's root. That is the F13 policy.

### D5: Golden replay on a real filesystem

**D5.1 Materialisation.** `internal/testutil/fstree` materialises an instance under `root := filepath.EvalSymlinks(t.TempDir())`. Resolving the base removes the out-of-model `/var` → `/private/var` ancestor link on macOS. The helper:
- creates directories, and files whose content is the node's atom label;
- creates symlinks with `os.Symlink`, and junctions with `mklink /J` semantics through `golang.org/x/sys/windows` (already a direct dependency at v0.47.0), on Windows only;
- names module roots `m<i>.invowkmod`;
- writes a fixed `invowkmod.cue` (module id `io.example.m<i>`) and `invowkfile.cue` from one template into each module root, and maps these files to a reserved `HarnessFile` atom that the policy excludes;
- includes the harness files in the copy and hash expectations;
- maps paths back to atoms with `os.SameFile`.

Each tree is materialised once, and every operation is replayed against it. `Load` parses the same template, so its CUE cost is bounded per tree.

**D5.2 Operation → entry point → test package.**

| Operation (model) | Go entry point | Test package / file |
|---|---|---|
| Validate layer | `invowkfile.ScriptFilePath.Validate`, `ValidateEnvFilePath`, `ValidateContainerfilePath` | `moduleops_test`, `module_path_containment_golden_test.go` |
| Containment layer | `invowkfile.ValidateScriptPathContainmentForTest` (test-only `pkg/invowkfile/export_test.go` hook; exported only to the external test package inside `pkg/invowkfile`, so the binding test for this layer lives in `pkg/invowkfile`'s external test package and reads the same golden file) | `invowkfile_test`, `module_path_containment_golden_test.go` |
| Script read | `Implementation.ResolveScriptWithFSAndModule` with an `os.ReadFile` wrapper that records the opened node | `moduleops_test` |
| Custom-check read | `CustomCheckScript.ResolveWithFSAndModule` | `moduleops_test` |
| Script execution | `Implementation.GetScriptFilePathWithModule`: asserts that the interpreter path equals the path the read opened (same `os.SameFile`) | `moduleops_test` |
| Env file | `ValidateEnvFilePath`, then `runtime.LoadEnvFile` | `moduleops_test` |
| Containerfile | `invowkfile.ValidateContainerfilePath` | `moduleops_test` |
| Workdir (host) | `Invowkfile.GetEffectiveWorkDir` | `moduleops_test` |
| Workdir (container) | `ContainerRuntime.getContainerWorkDir` (unexported) | `runtime`, `module_path_containment_golden_test.go` in `internal/runtime`, reading the same golden file |
| Module admission | `invowkmod.IsModule`, then `invowkmod.Load` | `moduleops_test` |
| Vendored discovery | `discovery.Discovery` public API over the materialised tree (`DiscoverModules`/`DiscoverAll`) | `moduleops_test` |
| Vendor / cache copy | `modulecache.CopyModuleDir` | `moduleops_test` |
| Provisioning copy | `provision.CopyDir` | `moduleops_test` |
| Hash | `invowkmod.ComputeModuleHash`, compared with the hash of the expected in-root regular files plus the harness files | `moduleops_test` |
| Unpack | `moduleops.Unpack` on an archive built from the instance's member names | `moduleops_test` |
| Audit (advisory) | `invowk audit` scan verdicts via the `audit` package API; logged per instance, not asserted | `moduleops_test` |

Absolute Windows-dialect references are replayed only through the Validate layer on non-Windows hosts, as `pathmatrix` does, and end to end on Windows CI. Every row's test is named in the binding cell of its correspondence row. An operation whose entry point cannot be reached is marked `-`, and the spec's "every bound operation" excludes it.

**D5.3 Masked layers.** `ResolveScriptWithFSAndModule` and `CustomCheckScript.ResolveWithFSAndModule` call `Validate` before the containment check, and `Validate` rejects `..` and every absolute dialect. A containment defect is therefore unreachable end to end, and a Validate defect is masked by containment. The per-layer comparison plus the `export_test.go` hook binds each layer directly. The hook is a `_test.go` file and not a production change.

**D5.4 Harness replay, in two halves.** `newVirtualPathResolverForFilesystem` adds host roots the test cannot control:
- `defaultTempRoots()` returns `/tmp`;
- the temporary anchor is `os.TempDir()`;
- XDG and HOME anchors come from the environment.

`t.TempDir()` lies under those roots, so every materialised node would be "inside" and the policy check would be vacuous. `t.Setenv` is incompatible with `t.Parallel()`, and `/tmp` cannot be overridden anyway. The replay is therefore split:
- **Root construction** is bound through the injectable `standardVirtualAnchorsForOS(goos, workDir, home, tempDir, getenv)` plus a Go transcription of the root list in `newVirtualPathResolverForFilesystem`. It is compared with the model's root set, which is built from materialised stand-in directories.
- **Validation** is bound by an internal test in `internal/runtime` that builds `virtualPathResolver{allowedRoots: normalizedRoots(materialisedRoots), access: ...}` directly, and calls `virtualPathValidator.validate`. Nodes outside the materialised roots are then genuinely outside, and a replay-side scenario asserts that at least one instance has such a node.

The F14 reproduction targets a writable directory outside the constructed roots (a sibling `outside/` under the materialisation root), not `/etc`.

### D6: Platform gating
The helper probes each capability once per test: symlink creation, junction creation (Windows only), and case-folding of the materialisation root. Instances that need a missing capability are skipped individually, with per-capability counts logged. The Go transcriptions are still asserted on those instances.

| Leg | Case-folding | Links |
|---|---|---|
| Linux | case-sensitive | symlinks |
| macOS | case-insensitive APFS | symlinks |
| Windows | case-insensitive NTFS | symlinks (when privileged) and junctions |

On Windows, `os.Symlink` needs the link type. The helper passes the model target's type: a directory target gets a directory link. Dangling links are created as file links. Junction instances are skipped and counted everywhere except Windows.

### D7: Findings, with fixed ids
F8–F12 are reserved by sibling changes (F8–F10 `model-concurrent-lock-writes`, F11–F12 `trace-validate-token-and-lock`). This change owns F13 onward:
- F13: the `invowk_modules/` symlink hole;
- F14: the virtual `mkdir`-through-symlink fallback;
- F15: `workdir` widening the virtual roots, only if confirmed;
- F16 onward: anything else.

A finding that does not reproduce is not recorded, its id stays unused, and the unused id is listed in the README's refuted hypotheses. A finding command checks the current-code predicate (`finding = "F13"`, `expect = "counterexample"`, `mutant_of = "<fixed check>"`). The fixed check models the proposed fix. The Go replay lives in `module_path_containment_formal_test.go` (or `virtual_path_harness_formal_test.go`), asserts today's behaviour, and names the finding. The fix change inverts it.

### D8: Suspected gaps the model must decide
These come from reading the code and are not yet confirmed.
1. **F13, vendored-subtree hole.** `inspectModuleEntry` skips `invowk_modules` (`operations_validate.go:188`). Consider a module admitted by `Load` with `invowk_modules/x -> <outside file>`. Its `script.file: "invowk_modules/x"` is lexically contained (`implementation.go:508`), and `os.ReadFile` follows the link. Remote modules lose symlinks in `copyDir`, but local includes, `~/.invowk/cmds` global modules, and direct git checkouts keep them.
2. **F14, virtual lexical fallback.** `normalizeExistingOrParent` (`virtual_policy.go:318`) evaluates only the path and its parent. The u-root `ln` stores link targets literally, so after `ln -s <outside dir> <root>/x`, `mkdir -p <root>/x/a/b` is judged lexically as inside the root and creates `<outside dir>/a`.
3. **F15, `workdir` widening.** Under `restricted` access, a module-declared `workdir: "/"` becomes an allowed root (`virtual_policy.go:103`). This violates `allowedWorkdirCwd`.
4. **Symlinked module root bypasses `Load`'s scan when a caller skips `IsModule`.** `ensureModuleDirectory` uses `os.Stat` (`operations_validate.go:77`), and `WalkDir` does not descend into a symlinked root. The `Load` callers break down as follows:
   - gated or safe by construction:
     - every discovery site (`discovery_files.go:235`/`269`, `385`/`404`, `458`/`477`, `572`/`587`);
     - `provision/helpers.go:165`;
     - `moduleops/vendor.go:237`, gated by `IsModule` at `:232`;
     - `audit/scan_context.go:679`, where the `DirEntry.IsDir()` filter excludes symlinks;
     - `moduleops/packaging.go:537`, whose root was just extracted by Invowk and cannot be a link;
   - user-controlled (non-goal): `cmd/invowk/validate.go:346`, `moduleops/packaging.go:102` (`module archive`), and `audit/scan_context.go:416` (the user's scan target);
   - no production caller: `invowkfile.ParseModule` (`pkg/invowkfile/parse.go:117`).

   The model keeps the two facts separate, so that the dependency is visible. A task re-verifies this classification.
5. **Symlinked hash root (low).** `computeModuleHash` on a symlinked directory hashes an empty tree. It is expected to be refuted by `isModuleRejectsLinkedRoot` at every caller.
6. **Env files are not checked at run time (refutation candidate).** This is safe only if parse-time validation always runs and `Load` excludes symlinks.
7. **Case-insensitive vendor skip (refutation candidate).** `Invowk_Modules` is scanned on a folding filesystem, which is stricter. The mutant "skip any directory whose name folds to `invowk_modules`" shows that the direction matters.
8. **Junctions (new candidate).** A junction inside a module is not `ModeSymlink`, so `inspectModuleEntry`, `copyDir`, `CopyDir`, and `computeModuleHash` may treat it as a directory and follow it. The model decides this. If Windows CI confirms it, it becomes F16.

### D9: Cost budgets
- Golden instances and decoded size per model are fixed only after a capped `-r` measurement. The target is the smallest scope that detects every seeded defect.
- Golden replay in `make test` must take 30 s or less per OS on Linux, macOS, and Windows. Windows file creation is the expected bottleneck.
- `formal-rapid-deep` (`RAPID_CHECKS=10000`) for the two properties must take 60 s or less. The rapid generator reuses tree shapes, and `Load` parses the fixed template.

The measurements are handed to `promote-formal-ci-gate` as `budget_seconds` values. If a budget cannot be met, the scope is narrowed, with the reason recorded, and the mutant or defect that justifies each retained dimension is listed.

### D10: CI and Make wiring
- Add `./internal/app/moduleops/`, `./internal/runtime/`, and `./pkg/invowkfile/` to `FORMAL_RAPID_PACKAGES`.
- In `.github/workflows/formal-verification.yml`, extend the "Replay golden vectors and property tests" step with those packages and add `ModulePathContainment|VirtualPathHarness` to its `-run` pattern.
- Extend `pull_request.paths` with the bound files: `pkg/invowkfile/**`, `pkg/invowkmod/**`, `internal/runtime/**`, `internal/provision/**`, `internal/app/modulecache/**`, `internal/app/moduleops/**`, `internal/discovery/discovery_files.go`, `internal/audit/**`, and `internal/testutil/fstree/**`.
- If `promote-formal-ci-gate` has landed, add these globs to its `[ci] paths`, where its coverage self-test requires them.

## Risks / Trade-offs

- [Filesystem replay is much slower than pure-function replay, especially on Windows] → Budgets in D9, one materialisation per tree, and a template `invowkmod.cue`. Measure before fixing the scope.
- [Instance explosion from the product of tree, references, and operations] → Capped `-r` measurement, `noJunk` restricted to referenced nodes, and splitting operations across the two models.
- [Symlinks and junctions are unavailable on some runners] → Skips per instance, with counts. The Go transcriptions are still asserted.
- [The harness replay does not exercise the real host roots] → Root construction is bound separately through `standardVirtualAnchorsForOS`. The validation half uses materialised roots by design (D5.4).
- [Findings may be disputed as by design] → The workdir and `invowk_modules` answers are encoded as named predicates (D4), so a later policy change touches one predicate.
- [A later refactor moves enforcement] → The correspondence table, `scripts/formal.py correspondence`, and the path filter (D10) catch it.

## Migration Plan

This change is additive: models, manifest entries, tests, a test helper, a test-only hook, CI wiring, and documentation. Rolling back means removing them. Ordinary `make build`, `make lint`, and `make test` need no Java. `make formal` needs Java 25; `python3 scripts/formal.py fetch` downloads the jars.

## Review Dispositions

Every edit in the independent review is applied, with these qualifications:
- **Item 7, `moduleops/vendor.go:237` "ungated".** Not applied as stated: `vendor.go:232` calls `invowkmod.IsModule(vendoredPath)` immediately before `Load`, so that caller is gated. `audit/scan_context.go:679` is effectively gated by its `DirEntry.IsDir()` filter, and `invowkfile.ParseModule` has no production caller. The classification in D8.4 records this. The rest of item 7 is applied: the split facts, the mutants, the symlinked-root witness, and the root-following copy.
- **Item 5, binding `validateScriptPathContainment` directly.** Applied through the `export_test.go` option. The hook can only be seen from `pkg/invowkfile`'s own external test package, so that layer's golden replay lives there and reads the shared golden file (D5.2), not in `moduleops_test`.

## Open Questions

The orchestrator has answered the earlier questions, and the answers are encoded in D4 and D2:
- `workdir` is an allowed location for the working directory only;
- `invowk_modules` script paths must resolve physically inside the module;
- junctions are in scope;
- the unused validators are for the fix change.

Remaining question: do the golden replays share one golden file across three test packages (`moduleops_test`, `invowkfile_test`, `runtime`)? Or should the containment and container-workdir layers get their own small golden commands? Sharing keeps one fingerprint. Splitting keeps each package independent.

## Implementation Record (2026-09-25)

**Pre-flight (1.2/1.3).** Every enforcement point in the table above was re-read at the branch base; no code had moved in a way that changes a decision. Reproductions confirmed F13 (script and custom-check reads through an `invowk_modules/` symlink), F14 (deep missing path through an escaping runtime symlink), and F15 (`workdir: "/"` added to the allowed roots under `restricted`). They also found **F17**: an env file declared as `invowk_modules/x` is read at runtime by `LoadEnvFile` through the same hole. **F16 (junctions)** cannot be reproduced on Linux. Go 1.27's `os.fileStat.mode` reports a mount point as `ModeIrregular`, not `ModeSymlink`, which supports the hypothesis. The Windows-only `TestModulePathContainment_F16Junction` logs the verdict. F16 stays unused, with no manifest `finding`, until Windows CI confirms it.

**Scope deviation (D9).** Exhaustive enumeration of the relational filesystem did not finish within 150 s, even at scope 3. The golden command therefore adds a `goldenScenario` predicate: a canonical single-module layout, case-folding off, junctions off, and no symlinked root. The fixed dimensions are covered by witness commands, the discovery-fact mutants, the rapid properties, and the Windows-gated test. `ModulePathContainment`: scope `exactly 1 Module, exactly 1 Op, 2 Name, exactly 1 Fold, 3 Dir, 1 File, 1 Link` gives 22848 instances (1680 distinct) in 20,994 B. `VirtualPathHarness` is an abstract location model: `exactly 3 Req` with one location of each kind gives 14580 instances in 18,332 B. 4 Req did not finish. The harness model abstracts filesystem locations into read-root, workdir, and outside classes, plus a missing-depth of 0, 1, or 2+. It does not model a full tree.

**Other deviations.** The moduleops golden test is an internal test (`package moduleops`). The containment layer is bound in `pkg/invowkfile` through `export_test.go`. Vendored discovery is bound through `IsModule` and `Load` admission, not the full `discovery.Discovery` API, which needs lock files and declared requires. Unpack, copy, and hash are bound as follows: layer decisions by transcription, physical containment by real `EvalSymlinks`, and copies and hash by a no-escape check against the real `modulecache.CopyModuleDir`, `provision.CopyDir`, and `ComputeModuleHash`. `invowk audit` is not logged per instance. A shared `internal/testutil/mpctree` translator and an `fstree.Cache` avoid duplicating the translation across the three test packages.

**Measured budgets (Linux, for `promote-formal-ci-gate` `budget_seconds`).**

| Replay | Linux |
|---|---|
| `TestModulePathContainment_GoldenVectors` | ~2.8 s |
| `TestModulePathContainment_ContainmentLayerGolden` | ~0.2 s |
| `TestModulePathContainment_ContainerWorkDirGolden` | ~0.2 s |
| `TestVirtualPathHarness_GoldenVectors` | ~1.2 s |
| deep rapid, `TestModulePathContainment_Property` | ~3.1 s |
| deep rapid, `TestVirtualPathHarness_Property` | ~0.5 s |

The macOS and Windows legs have not been measured yet. They run in CI after the orchestrator merges.

**Calibration (8.1).** Seeded one at a time, then reverted. The containment check removed from `validateScriptPathContainment` fails the containment-layer golden. `modulecache.copyDir` following symlinks fails the copy-escape check. The remaining defects in the spec list are covered by the per-layer golden decisions and the Alloy mutants; they were not each seeded against the real code (see the manifest `calibration`).
