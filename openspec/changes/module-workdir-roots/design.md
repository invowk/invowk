# Design

## Context

See proposal.md (Why). Current behaviour, on base `origin/main` 3bb45325:

- The resolver adds the effective workdir to the allowed roots unconditionally: `roots := []string{workDir, scriptBasePath, anchors[virtualAnchorTmp]}` (`internal/runtime/virtual_policy.go:169`). The loop after it adds `@work` (`virtual_policy.go:170-174`), which is the same path (`virtualAnchorWork: workDir`, `virtual_policy.go:221`).
- Its only input is a path string. `newVirtualPathResolver` passes `ctx.EffectiveWorkDir()` (`virtual_policy.go:140-147`), which is `Invowkfile.GetEffectiveWorkDir(cmd, impl, ctx.WorkDir)` (`internal/runtime/runtime.go:525-527`). That function picks the CLI override, then the implementation, command and root `workdir`, then the invowkfile directory (`pkg/invowkfile/invowkfile.go:171-221`). It keeps absolute workdirs as they are (`invowkfile.go:190-192`) and joins relative ones to the script base path (`invowkfile.go:195-196`), so `..` can leave the module. The result carries no source.
- The script base path is the module directory for a module and the invowkfile's directory otherwise (`invowkfile.go:154-159`). It is always an allowed root (`virtual_policy.go:169`).
- The interactive paths rebuild the resolver in a subprocess from flags: `--workdir` and `--script-base-path` (`internal/app/commandadapters/sh_interactive.go:40-48`, `lua_interactive.go:31`; parsed in `cmd/invowk/internal_exec_sh.go:34-43,56-57` and `internal_exec_lua.go:27-28,51-52`). From there the path goes through `ShScriptOptions`/`LuaScriptOptions` into `newVirtualPathResolverForInteractiveConfig` (`internal/runtime/sh.go:371-378`, `internal/runtime/lua_interactive.go:211-218`, `virtual_policy.go:187-196`). The parent builds the subprocess request in `internal/runtime/virtual_interactive.go:101-145`.
- The physical resolver from B2 (#198) is `normalizeExistingOrParent` (`virtual_policy.go:415-424`). It walks every symbolic link and Windows junction to the deepest existing ancestor (`walkVirtualPath`, `virtual_policy.go:519-538`), canonicalises it, and returns an error for every failure instead of falling back. Allowed roots go through it too (`normalizedRoots`, `virtual_policy.go:266-278`). `pathWithin` returns false when `filepath.Rel` fails, for example across Windows volumes (`virtual_policy.go:710-716`).
- The process cwd is the workdir in every runtime (for example `interp.Dir(ctx.EffectiveWorkDir())`, `internal/runtime/sh.go:449`). Lua `require` reads only below the script base (`internal/runtime/lua_io.go:578-600`).
- Formal: the model's root rule is a Bool, `locIsAllowedRoot[l, includeWorkdir]` (`formal/alloy/VirtualPathHarness.als:160-162`), and `H` holds `includeWorkdir` (`VirtualPathHarness.als:97`). The policy says no read is admitted by the workdir (`allowedWorkdirCwd = no r`, `VirtualPathHarness.als:173-176`). The golden fixes `includeWorkdir = True` (`VirtualPathHarness.als:230-233`). The finding command is `findingF15WorkdirWidening` (`formal/manifest.toml:1701-1707`), and its row is `findingF15` (`VirtualPathHarness.als:71`; `"VirtualPathHarness: findingF15"` is on the shrink-only unresolved list, `formal/manifest.toml:166`). The golden replay builds its roots directly, workdir included (`internal/runtime/virtual_path_harness_golden_test.go:85-93`). The construction replay expects a workdir-only probe to be accepted (`internal/runtime/virtual_path_harness_formal_test.go:325-326`). The F15 replay asserts the widening (`virtual_path_harness_formal_test.go:207-233`). The rapid property builds its root directly and has no workdir (`internal/runtime/virtual_path_harness_rapid_test.go:86`).
- Provenance of the CLI override: `ExecutionContext.WorkDir`, the `cliOverride` argument, is set only from `--ivk-workdir` (`cmd/invowk/cmd_execute_helpers.go:137` to `orchestrator.go:271`). Dependency checks copy the parent's value (`internal/app/commandadapters/dependency_runtime.go:271`), and `internal/app/commandsvc/service.go:239` only reports it in a dry-run plan. A non-empty override is therefore always user-chosen, which is why D1 treats `cli-override` as trusted.
- Mutation ledger (`tools/mutation/triage/formal-bindings.toml`): 21 entries keyed `symbol = "GetEffectiveWorkDir"` (the brief counted 20) and 4 keyed `symbol = "newVirtualPathResolverForFilesystem"` (the triage counted 12 survivors in that function). The implement stage takes both counts from `formal_mutation.py symbols`, not from this file.

## Goals / Non-Goals

**Goals:**
- One decision point, used by the non-interactive and the interactive paths, for "is the workdir an allowed root".
- The decision uses the same physical resolution as every other containment decision, and fails closed.
- The body of the formal-bound `GetEffectiveWorkDir` stays as it is.
- The model, golden, construction replay, rapid property and the F15 replay agree with the code.

**Non-Goals:**
- Native and container runtimes, `getContainerWorkDir`, watch base directories and host-binary lookup through an empty `PATH` entry (`virtual_policy.go:841-857`) are unchanged.
- No validation-time rejection of outside workdirs (alternative (c) of the triage), and no new diagnostic text on a denial (see Open questions).
- The frozen F22 follow-ups: the `cp -P` rule-3 title, the `VirtualPathHarness.als` validate-row wording, a Windows `cp -r` junction test and the 19 binding-gap mutants. They go to a separate change.
- Nothing about the check-then-use race of the validator beyond what the model already records.

## Decisions

### D1. A typed effective-workdir result, without touching `GetEffectiveWorkDir`'s body

`pkg/invowkfile` gains:
- `WorkDirSource`, a string value type with `Validate()`. Its values are `cli-override`, `implementation`, `command`, `root` and `default`, and it has `DeclaredByInvowkfile() bool` (true for `implementation`, `command` and `root`).
- `EffectiveWorkDir{Path FilesystemPath; Source WorkDirSource}`, with `Validate()`.
- `(*Invowkfile).ResolveEffectiveWorkDir(cmd, impl, cliOverride) EffectiveWorkDir`, which returns `GetEffectiveWorkDir(...)` as `Path` and the level that supplied the value as `Source`. That level is picked by an unexported `effectiveWorkDirSource` with the same precedence tests (`cliOverride != ""`, `impl != nil && impl.WorkDir != ""`, `cmd != nil && cmd.WorkDir != ""`, `inv.WorkDir != ""`, else `default`).

`GetEffectiveWorkDir`'s body does not change, so its 21 ledger entries and the ModulePathContainment `workdirCwd` row stay as they are, and they need no refresh. The precedence then exists twice. A table test covers all 16 combinations of the four levels: the workdir named by `Source` must resolve, through `GetEffectiveWorkDir` with only that level set, to the same `Path`. That test fails if the two drift. A comment on `effectiveWorkDirSource` says why it is separate, so the simplify stage does not merge it back.

The one case where the two can disagree is harmless. When a level holds an invalid `FilesystemPath`, `GetEffectiveWorkDir` falls back to the invowkfile directory (`invowkfile.go:181-186`) while `Source` names that level. The fallback path is the script base, which is inside the module, so the decision is the same either way.

Alternatives:
- (a) Move the body into `ResolveEffectiveWorkDir` and make `GetEffectiveWorkDir` a wrapper. That re-keys 21 ledger entries to a new symbol, and it changes the ModulePathContainment correspondence table and so its golden fingerprint. That is a second formal model in this change.
- (b) Change `GetEffectiveWorkDir`'s return type. That breaks the public `pkg/invowkfile` API and has the same ledger churn.
- (c) A second boolean parameter on the resolver. The brief rejects it.

### D2. The resolver takes a typed workdir and decides once

`internal/runtime` gains `virtualWorkdir{path string; moduleRoot string}`, in a new file `internal/runtime/virtual_workdir.go`, because `virtual_policy.go` is at 907 of its 1000 lines. Its fields are raw OS strings with a `goplint:ignore`, like the walker's structs:
- `path` is the effective workdir (the cwd).
- `moduleRoot` is set only when the invowkfile is a module (`ModulePath != ""`) and `Source.DeclaredByInvowkfile()` is true. It is then the module directory.

`virtualWorkdirFor(inv, cmd, impl, cliOverride)` builds it from `ResolveEffectiveWorkDir`. `newVirtualPathResolver` calls it, and `newVirtualPathResolverForFilesystem` takes a `virtualWorkdir` in place of `workDir string`. The `@work` anchor still gets `path`, so `INVOWK_ANCHOR_WORK` and `invowk.path("@work")` keep reporting the cwd.

`(virtualWorkdir).isReadRoot() bool` makes the decision:
- `moduleRoot == ""` gives true (trusted).
- Otherwise it resolves both with `normalizeExistingOrParent(·, "")`. Any error gives false, and otherwise it returns `pathWithin(module, workdir)`.

`newVirtualPathResolverForFilesystem` adds `path` and `@work` to the roots only when `isReadRoot()` is true. The script base, the other anchors, `/tmp` and the declared `filesystem.paths` are unchanged. A `paths` entry of `@work` still resolves to the workdir and is a root (the escape hatch).

Why the module root is resolved and not trusted as written: the module directory can itself be reached through a link (a vendored module, a global module directory, macOS `/var` aliases). Resolving both with the same function compares like with like, which is how `normalizedRoots` already treats the roots.

Under `full` access the decision has no effect (`validate` returns before the root check), so nothing changes there. The decision is still computed; it is cheap.

### D3. The interactive subprocesses get the module root

`ShInteractiveCommandSpec` and `LuaInteractiveCommandSpec` get `WorkDirModuleRoot *types.FilesystemPath` (nil means trusted). The adapters pass it as a hidden `--workdir-module-root` flag only when it is set (`internal/app/commandadapters/{sh,lua}_interactive.go`). `internal exec-virtual-sh` and `exec-virtual-lua` parse it into `ShScriptOptions.WorkDirModuleRoot` and `LuaScriptOptions.WorkDirModuleRoot`. `newVirtualPathResolverForInteractiveConfig` builds the same `virtualWorkdir`, so the child makes the decision with the same function at its own start.

An absent flag means trusted. That is safe because only invowk itself launches these hidden commands, with the flag set from the parent's `virtualWorkdir`. A person who runs them by hand is the user, who can pass `--ivk-workdir` anyway. Passing the parent's boolean verdict instead was rejected because it splits the decision across two processes.

The internal commands stay hidden. The flag is not documented on the website, only in README.md if README lists those flags (commands rule).

### D4. Model: a workdir kind and a root rule

In `VirtualPathHarness.als`:
- `H.includeWorkdir: Bool` becomes `H.workdir: one WorkdirKind`, with `TrustedWorkdir`, `ModuleInside` and `ModuleOutside`. A fact says `H.workdir = ModuleInside implies no InWorkdir`, because an inside workdir is enclosed by the script base, which is a read root.
- The code's rule becomes a parameter, `abstract sig WorkdirRule`, with `AlwaysRoot` (pre-fix), `TrustedOrInside` (fixed) and `NeverRoot` (strict). `pred workdirIsRoot[rule]` holds when `rule = AlwaysRoot`, or when `rule = TrustedOrInside` and `H.workdir in TrustedWorkdir + ModuleInside`. `locIsAllowedRoot[l, rule]` is `l in InReadRoot or (workdirIsRoot[rule] and l in InWorkdir)`, and `accepts` and `harnessContained` take `rule` in place of the Bool.
- Policy: `contained[r]` is `H.access = Full or r.physAnc in InReadRoot or allowedTrustedWorkdir[r] or allowedWorkdirCwd[r]`, where `allowedTrustedWorkdir[r]` is `H.workdir = TrustedWorkdir and r.physAnc in InWorkdir`, and `allowedWorkdirCwd` stays `no r`. The policy is written from the specs (virtual-runtime-sandbox "The work directory is an allowed root unless a module declares it outside the module"), not from the code's rule.
- Commands (manifest):
  - `harnessContainedFixed` = `harnessContained[FixedEval, TrustedOrInside]` (pass).
  - `mutantPreFixF15WorkdirRoot` = `harnessContained[FixedEval, AlwaysRoot]` (counterexample, `mutant_of = "harnessContainedFixed"`, no `finding`). It replaces `findingF15WorkdirWidening`.
  - The F14, F18 and F19 mutants switch their second argument to `TrustedOrInside`.
  - New: `trustedWorkdirReadableFixed` = `trustedWorkdirReadable[TrustedOrInside]` (pass), and `mutantStrictWorkdirNeverRoot` = `trustedWorkdirReadable[NeverRoot]` (counterexample, `mutant_of = "trustedWorkdirReadableFixed"`). `trustedWorkdirReadable[rule]` says that every request with `H.access = Restricted` and `allowedTrustedWorkdir[r]` is accepted under the fixed evaluation.
  - `anteHarness` uses `TrustedOrInside`. `workdirWidening` is rewritten for `ModuleOutside`. A witness `witModuleOutsideWorkdir` (`H.workdir = ModuleOutside and some InWorkdir`) is added.
- Golden: `goldenScenario` keeps `H.access = Restricted` and leaves `H.workdir` free. `dAccepts` uses `TrustedOrInside`. The main requirement 'Harness no-follow requests are a recorded abstraction' said the model and golden SHALL NOT change; this change's MODIFIED delta scopes that to the no-follow form (no no-follow dimension in the golden), so regenerating the golden for the workdir kind does not contradict it. The instance count grows from 2592 to about 2.2x that; the file is 2.9 KB gzipped today. The implement stage records the new count and replay time in the calibration text.
- Correspondence:
  - The `findingF15` row becomes a `mutantPreFixF15WorkdirRoot` row naming `TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot`, and `"VirtualPathHarness: findingF15"` leaves `[correspondence] unresolved_elements`, as `findingF14` did.
  - New `workdirIsRoot` rows, one per bound symbol, each naming only killers that reach it (the reach gate checks every (row, killer, function) triple):
    - `virtualWorkdir.isReadRoot`: `TestVirtualPathHarness_RootConstruction`, `TestVirtualPathHarness_GoldenVectors` (it builds `virtualWorkdir` directly and calls `isReadRoot`) and `TestVirtualPathHarness_Property`.
    - `virtualWorkdirFor`, `Invowkfile.ResolveEffectiveWorkDir` and `WorkDirSource.DeclaredByInvowkfile`: `TestVirtualPathHarness_RootConstruction` and `TestVirtualPathHarness_Property`. Both build the workdir input through `virtualWorkdirFor` from an `Invowkfile`, a command, an implementation and a CLI override, so a mutant that makes `DeclaredByInvowkfile` false for a declared level turns a module-outside probe into an accepted one and fails them.
    - `effectiveWorkDirSource`: `TestVirtualPathHarness_RootConstruction`, `TestVirtualPathHarness_Property` and the pkg/invowkfile drift test `TestResolveEffectiveWorkDir` (D1). Each declared level alone with an outside workdir kills a mutant that drops that level's test (the source falls through to `default`, which is trusted, and the outside probe is accepted). The construction replay's combined case, a CLI override naming the outside directory together with a module-declared outside workdir (D5), kills a mutant that drops the CLI-override test or moves it after a declared level: the source then names a declared level, the override path is judged outside, and the trusted probe is rejected. A swap of the implementation and command tests leaves the source declared either way, so the root decision is the same and only the drift test sees it, which is why the drift test is a killer of this row.
    - The `workdirIsRoot` abstraction cell names the masked case: `H.workdir = ModuleInside implies no InWorkdir`, so an `isReadRoot` mutant that returns false for a module-inside workdir is equivalent in the model and in the golden (the script base encloses the workdir). The unit test of D5 (module inside, `isReadRoot()` true) is its killer outside the formal lane.
  - New `allowedTrustedWorkdir` row (model element only, file `formal/alloy/VirtualPathHarness.als`, binding `TestVirtualPathHarness_Property`): policy, a trusted workdir (the CLI override, a workdir of an invowkfile outside a module, a module's default workdir) is an allowed root; justification: the user chose it, or it is the module directory. This follows the MODIFIED 'Allowed-location predicate set'.
  - The `allowedRoots`, `anchors`, `contained` and `allowedWorkdirCwd` abstraction cells are updated, including the `@work` "masked (decision 31)" text, which now holds only where the workdir is a root. `allowedWorkdirCwd` reads "a module-declared workdir outside the module is a cwd, not an allowed read root".
- The calibration text's F15 sentences (`formal/manifest.toml:1546-1548,1556`) are rewritten for the fixed rule, with the seeds of the construction replay ("Workdir widening reproduced"). The implement stage runs `make formal-reach` for the new rows.

### D5. Replays and tests

- `TestVirtualPathHarness_F15WorkdirWidening` is inverted and renamed `TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot`. With `virtualWorkdir{path: fsRoot, moduleRoot: module}` the roots hold neither `fsRoot` nor `@work`, and `<fsRoot>/etc/hostname` (Windows: a missing name under the volume root) is denied. With `moduleRoot: ""` it is accepted.
- `TestVirtualPathHarness_RootConstruction` loops over workdir kinds. Each kind builds an `Invowkfile` (`ModulePath` set or not), a command and implementation with `workdir` at the root, command or implementation level, or a CLI override, and calls `virtualWorkdirFor`. Its expectation is `full || loc == inReadRoot || (loc == inWorkdir && kind allows)`. `@work` is classed `InReadRoot` for trusted kinds and `OutsideAll` for module-outside. The outside workdir stays a missing directory under the volume root (D5.4 of the earlier change), so no temp or anchor root encloses it. For the "through a link" kind, a link inside the module points at that directory. One combined case sets a CLI override naming that outside directory together with a module-declared outside workdir at each level; it is trusted, so its workdir-only probe is accepted (the killer of the CLI-override precedence test, D4).
- Golden replay: per instance it builds `virtualWorkdir` for the instance's kind over its `work` directory and calls `isReadRoot()`:
  - trusted: no module root, and `isReadRoot()` must be true;
  - module-outside: module root = `read`, `work` outside it, and `isReadRoot()` must be false;
  - module-inside: the model has no `InWorkdir` location, so the replay builds `work` below `read` with module root = `read`, and `isReadRoot()` must be true (spec scenario 'Module-inside workdir in the golden').
  It adds `work` to the directly built roots exactly when `isReadRoot()` says so. So the golden reaches the production decision, while the roots stay free of `t.TempDir`'s enclosing roots.
- Rapid: new generators for the source and the placement (spec "Harness rapid property covers workdir placement"). It builds the workdir input through `virtualWorkdirFor`, so it reaches the source decision too (D4). The oracle comes from how the tree was built. A link placement records where it points. Its verdict half builds the roots directly, as the existing property does, so no implicit root encloses the generated tree.
- Unit tests in `internal/runtime` (`virtual_workdir_test.go`). Off Windows the implicit `/tmp` root and on every platform the `@tmp` anchor (`os.TempDir()`) enclose `t.TempDir()`, so a verdict below an outside directory built there is always "accepted". The tests therefore assert, for every case, `isReadRoot()` and whether the workdir and `@work` are in the resolver's normalised `allowedRoots`. They assert the resolver's verdict on a read below the workdir only where no implicit root encloses it: for trusted and inside cases (accepted), under `full` access (accepted), and for the module-outside absolute case with the construction replay's placement (a missing directory under the volume root, denied). The cases:
  - one per source: CLI override, plain invowkfile, module default, module inside, module outside by `..`, by an absolute path, and through a link;
  - a link cycle and a missing workdir below the module;
  - `runtime.GOOS == "darwin"`: the module given through `/var/...` and the workdir through `/private/var/...`;
  - `windows`: a workdir on another volume, in two cases. A drive letter that does not exist covers the resolution-error branch of `normalizeExistingOrParent`. A second existing volume covers the cross-volume `filepath.Rel` failure in `pathWithin` (brief decision 7): the test looks for an existing volume other than the module's (`t.TempDir()` is on `C:` and the checkout on `D:` on GitHub windows runners; it may use the working directory's volume) and skips that subtest with a logged reason only when none exists. A UNC admin share is not used, since CI may not offer it. A junction inside the module to an outside directory is built with `fstree.MustJunction`.
- Unit test in `pkg/invowkfile` for `ResolveEffectiveWorkDir` and `WorkDirSource.Validate` (D1's drift test).
- Interactive: extend `internal/runtime/interactive_test.go` so the launcher spec carries `WorkDirModuleRoot` for a module-outside workdir and nil otherwise, and add an adapter test that the flag appears only when set.
- CLI: `tests/cli/testdata/virtual_module_workdir_roots.txtar`.
  - Placement. Off Windows `/tmp` is always a root and `$WORK` is normally below it, so a `../shared` next to a module in `$WORK` stays readable after the fix, and the denial cannot be observed there. The test follows `virtual_path_link_escape.txtar`: off Windows `OUTSIDE=/var/tmp`, on Windows `OUTSIDE=$WORK/outside` (testscript points `TEMP`/`TMP` at `$WORK/.tmp`, and Windows has no `/tmp` root). The fixture tree (the module directory, its sibling `shared` with `data.txt`, and the plain-invowkfile project) lives in one fixed entry below `$OUTSIDE`, for example `$OUTSIDE/invowk-module-workdir-roots`. It is removed and recreated from the files of the txtar before every step that depends on it, the test `cd`s into it to run invowk, and the entry is removed at the end. Every denial step checks that `out.txt` is missing afterwards.
  - A module whose command declares `workdir: "../shared"`. virtual-sh under restricted access reads `./data.txt` there and fails with `virtual path denied`, and its write of `./out.txt` fails the same way. A virtual-lua command fails on `io.open`. The command's `pwd` is the `shared` directory.
  - The same command with `virtual.filesystem.paths: {WORK: "@work"}` succeeds, `--ivk-workdir` naming the same directory succeeds, and `access: "full"` succeeds.
  - A plain invowkfile with `workdir: "../shared"` succeeds.
  - Native leg (spec scenario 'Native and container runtimes are unaffected'): off Windows, a native command of the same module with the same `workdir` reads `./data.txt` and prints its `pwd`, and both succeed (`[!windows]`; the native shell differs on Windows, and the file is mirror-exempt anyway). The container runtime is covered by the non-goal: task 3.1 checks that `git diff origin/main -- internal/runtime/container*.go internal/containerplan/` is empty.
  - It gets an exact entry in `tests/cli/runtime_mirror_exemptions.json` ("restricted path denials exist only in the virtual runtimes", the reason `virtual_path_link_escape.txtar` already uses). A native mirror would show only that native has no path policy.
- Windows and macOS evidence comes from the CI matrix. The result names the test names and the run and job ids.

### D6. Mutation refresh plan

- `GetEffectiveWorkDir`: unchanged body, no refresh.
- `newVirtualPathResolverForFilesystem`: its root-building lines change, so its entries get a partial refresh (`make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=newVirtualPathResolverForFilesystem`).
- New bound symbols (`virtualWorkdir.isReadRoot`, `virtualWorkdirFor`, `Invowkfile.ResolveEffectiveWorkDir`, `effectiveWorkDirSource`, `WorkDirSource.DeclaredByInvowkfile`; their killers are in D4): their survivors are triaged into the ledger in the same change. The module-inside `isReadRoot` mutant is an expected equivalent survivor in the formal lane (D4's masked case). A swap of the implementation and command tests in `effectiveWorkDirSource` does not change the root decision; the drift test named on that row kills it.
- Run `formal_mutation.py symbols` first to get the real list, and `triage --check` last.
- `tools/mutation/triage/formal-bindings.toml` may also be edited by the concurrent lock-unchanged-message run. On rebase, keep both sides' entries.

### D7. Docs

- `website/docs/runtime-modes/virtual.mdx`: the root list (L154).
- `website/docs/runtime-modes/virtual-lua.mdx`: the implicit roots sentence (L31).
- `website/docs/advanced/workdir.mdx`: a note for virtual runtimes. A module's outside workdir is a cwd, not a root, under restricted access; declare it in `virtual.filesystem.paths` or pass `--ivk-workdir`. `full` access is unaffected.
- The pt-BR copies of all three (`website/i18n/pt-BR/docusaurus-plugin-content-docs/current/...`).
- README if it lists the roots.
- `formal/README.md`: the F15 row and the VirtualPathHarness summary row.

The text describes the change as a hardening of the virtual-runtime read roots, never calls the virtual runtimes a sandbox, and gives no exploit steps.

## Risks / Trade-offs

- [A module that relied on an outside workdir for virtual I/O breaks] → It is the narrow attack shape the finding describes. The release note and the workdir docs name the fix (`virtual.filesystem.paths` or `--ivk-workdir`), and `full` access and plain invowkfiles are untouched.
- [The precedence is written twice (D1)] → The 16-combination drift test, plus a comment that names the formal binding as the reason.
- [The decision is made once, at resolver construction, and the filesystem can change before the script runs (a directory on the workdir path swapped for a link)] → The roots are resolved at construction too (`normalizedRoots`), and every request is resolved physically at use time and compared with them. A later swap can therefore make a request resolve outside the roots, which is denied, but it cannot make an outside directory a root. The residual check-then-use race of the validator itself is unchanged and stays recorded as not modelled.
- [macOS case-insensitive APFS: a workdir that differs from the module directory only in letter case is judged outside] → This fails closed. The module directory stays a root, and the declared-path escape hatch covers it. It is recorded in the docs only if the implement stage reproduces it.
- [Golden size and replay time grow about 2.2x] → The file is small (2.9 KB gzipped), and the replay takes about 0.1 s today. The new figures go into the calibration text.
- [The interactive child gets the module root from a flag] → Only invowk launches it. An absent flag is the trusted case, which matches what a user can already ask for with `--ivk-workdir`.

## Migration Plan

No data migration. Release note (security fix, not `!` breaking for plain invowkfiles): "Virtual runtimes no longer treat a module-declared `workdir` outside the module as an allowed root under `restricted` access. A module that reads or writes there must declare the directory in `virtual.filesystem.paths` (for example `WORK: "@work"`)." Rollback is a revert of the change; no state is written.

## Maintainer decisions

- 2026-11-19 (fixes program decision 45, advisor-reviewed): narrow variant (b). The CLI override and plain-invowkfile workdirs stay roots, `virtual.filesystem.paths` is the escape hatch, and the strict variant is not chosen. MODIFIED deltas narrow `virtual-runtime-sandbox` and `cross-platform-path-anchors`. The model gets the source and containment dimension, F15 becomes a pre-fix mutant, the replay is inverted, and the rapid property gets the workdir dimension. "Outside" is decided physically and fails closed. A typed result is preferred over a boolean parameter.
- 2026-11-24 (decision 102): the scope is frozen to the F15 brief. The F22 follow-ups go to a separate virtual-runtime follow-ups change.
- 2026-11-24 (this spec stage, under the brief's decision 6): `GetEffectiveWorkDir`'s body is kept, and the source is computed beside it (D1), so its 21 ledger entries and the ModulePathContainment model are not touched. This is open to the spec review.
- 2026-11-24 (spec-fix stage, after the independent spec review of 16dcfe89): the review accepted D1. Its blocking finding (an unconditional denial where `/tmp` or `@tmp` encloses the outside directory) is resolved by the "unless another allowed root encloses it" qualifier in the sandbox delta and by the `/var/tmp` placement of the CLI test and the root-membership assertions of the unit tests (D5). Its major findings added the MODIFIED 'Allowed-location predicate set' (`allowedTrustedWorkdir`, D4 row) and the MODIFIED no-follow requirement (golden clause scoped). The `invowk audit` question stays open for the maintainer (Open questions).
- 2026-11-24 (revise spec stage, checking 98d68d51 against the ten findings): nine are resolved as written. For the minor finding on bound symbols, the killer of `effectiveWorkDirSource` is made exact: the construction replay gains a CLI-override-plus-declared case, and the drift test becomes a killer of that row, because an implementation/command swap does not change the root decision (D4, D5, D6).
- 2026-11-25 (implement stage): the golden scope allows at most one `InWorkdir` location (`1 InWorkdir` in place of `exactly 1`), because the fact `H.workdir = ModuleInside implies no InWorkdir` leaves no module-inside instance under `exactly 1`, and the spec scenario 'Module-inside workdir in the golden' needs one. The golden then holds 8790 instances (3794 trusted, 3794 module-outside, 1202 module-inside), about 3.4x the old 2592 rather than the 2.2x estimated above; the 81 request shapes are unchanged and the replay takes about 0.4 s on Linux. The seed "isReadRoot is false for every module-declared workdir" fails the golden replay, the rapid property and `TestVirtualWorkdir`, not the construction replay, whose module-inside kind has no workdir-only probe: the masked case of D4. The calibration text records every seed.

## Open questions

- Should a denial below an excluded module workdir say why, for example "the module's workdir is outside the module; declare it in virtual.filesystem.paths"? This change leaves the error text as it is. Adding a hint touches the bound `validate` path and can follow separately.
- Should `invowk audit` report module-declared workdirs outside the module? It is out of scope, and a candidate for the module-security follow-ups.
