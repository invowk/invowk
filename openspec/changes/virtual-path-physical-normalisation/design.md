## Context

See proposal.md (Why). Current behaviour, verified on `main` at c2fe96ff. `GOROOT` below is the Go 1.27.4 source of the local toolchain.

- **One function feeds every decision.** `normalizeExistingOrParent` (`internal/runtime/virtual_policy.go:325-348`) cleans the path and makes it absolute, then returns the first of:
  - `filepath.EvalSymlinks(path)` (`:340`);
  - `filepath.EvalSymlinks(parent)` joined with the last name (`:343-345`);
  - the cleaned, unresolved path (`:347`).
  It returns an error only for an empty path or a failed `os.Getwd` (`:326-336`).
- **Its callers:**
  - `virtualPathValidator.validate` through `resolve` (`:234-248`, `:250-256`). The shell open, readdir and stat handlers perform the operation on the path `validate` returns (`:361-365`, `:372-376`, `:383-387`), and so do the Lua file functions (`internal/runtime/lua_io.go:291`);
  - `resolveBridgePath`, behind Lua `invowk.path` (`virtual_policy.go:258-271`, `internal/runtime/lua.go:428`);
  - `normalizedRoots`, which drops a root when normalisation fails (`virtual_policy.go:199-209`);
  - `resolveVirtualFilesystemPaths`, whose result becomes `INVOWK_PATH_*` and an allowed root, and whose error fails the resolver's construction (`:211-232`, `:101-104`, `:312-314`);
  - Lua `require`, for its base and for each candidate (`lua_io.go:580`, `:595-603`).
- **`/tmp` is always an allowed root off Windows** (`defaultTempRoots`, `virtual_policy.go:135-140`).
- **Anchor variables are not normalised** (`INVOWK_ANCHOR_*`, `:308-311`); declared paths are (`:225`, `:312-314`).
- **What `filepath.EvalSymlinks` does on Windows:**
  - it follows only `ModeSymlink` entries; a component that is neither a symlink nor a directory, with components after it, gives `ENOTDIR` (`GOROOT/src/path/filepath/symlink.go:90-92`);
  - a junction is a name-surrogate reparse point: it gets `ModeIrregular`, and neither `ModeSymlink` nor `ModeDir` (`GOROOT/src/os/types_windows.go:193-196`, `:206-229`);
  - on success it rewrites the result with the on-disk case and long name of every component and an upper-case drive letter (`toNorm`, `GOROOT/src/path/filepath/symlink_windows.go:25-41`, `:108-117`). Comparing a root with a request by string therefore depends on that step on Windows.
- **`os.Readlink` reads junctions.** It returns the target of a symlink and of a mount point (junction), and `ENOENT` for any other reparse point (`GOROOT/src/os/file_windows.go:489-501`). A relative symlink target is returned as stored (`:492-494`). A volume target comes back as `\\?\Volume{…}\` (`:421-438`).
- **Windows probe** (windows-latest, CI run 44523520503, job 135557989975; raw lines in `program-notes/w2-findings-triage.md`, "Windows probe results"): `EvalSymlinks` of a path that ends at a junction returns it unchanged; of a path through a junction it fails with an error that `errors.Is(err, fs.ErrNotExist)` reports as true; `os.Lstat` of the junction reports `ModeIrregular`, not a directory. The harness accepted `inside\j\secret.txt`, `inside\j\new.txt` and `inside\j\a\b\new.txt`. This is F18.
- **Linux probe of this stage** (Go 1.27.4, c2fe96ff, a throwaway test compiled into `internal/runtime` with `go test -overlay`; nothing was written to the worktree). With one allowed root `root` and a sibling `outside`:

  | Request | Result today |
  |---|---|
  | `root/d`, `d` a symlink to the missing `outside/new` | accepted as `root/d`; opening it with `O_CREATE` created `outside/new` (F19) |
  | `root/d3`, `d3` a relative symlink to the missing `../outside/new3` | accepted |
  | `root/x/a/b`, `x` a symlink to `outside` | accepted as `root/x/a/b` (F14); `root/x` and `root/x/a` denied |
  | `root/rel/a/b` and `root/abs/a/b`, links to `root/in` with a relative and an absolute target | accepted, in lexical form; one level up the form is `root/in/a` |
  | `alias/n1/n2`, `alias` a symlink to `root`, the root given as `alias` | **denied**: the request stays `alias/n1/n2` while the root is `root` (`alias/n1` is accepted) |
  | `root/f/x`, `f` a regular file | accepted |
  | `root/loop`, a symlink to itself | accepted |

- **The model** (`formal/alloy/VirtualPathHarness.als`): a request has a depth (missing components), the location of its deepest existing ancestor read lexically, and that ancestor's physical location (`:63-70`). It has no link kind. `CurrentEval` returns the lexical ancestor only at depth `TwoPlus` (`:78-85`); `FixedEval` always returns the physical one. The golden records both decisions (`:141-155`), with the workdir kept as a root (`:159-162`).
- **The bindings:**
  - the golden replay compares with `dAcceptsCurrent` (`internal/runtime/virtual_path_harness_golden_test.go:54`, `:66`) and builds every link as a symlink to an existing directory (`:81-89`);
  - the rapid property exempts a deep path through the link (`virtual_path_harness_rapid_test.go:78-83`);
  - `TestVirtualPathHarness_F14DeepSymlink` asserts the escape (`virtual_path_harness_formal_test.go:19-47`);
  - the manifest has `findingF14DeepSymlink` (`formal/manifest.toml:1594`), the golden scope `exactly 3 Req …` (`:1657`) and the list entry `VirtualPathHarness: findingF14` (`:167`).
- **`fstree` creates junctions** and has the capability policy: junctions are required on Windows, symlinks on Linux, macOS and Windows CI (`internal/testutil/fstree/capability.go:47-60`, `:106-112`).
- **Shipped spec text this change touches:** "Defect fixes are separate from finding records" (`openspec/specs/formal-verification-toolchain/spec.md:280-285`) forbids recording and fixing a finding in one change. The maintainer brief asks for exactly that for F18, citing the F16 outcome "fixed after an observed escape" (`openspec/specs/module-path-containment-model/spec.md:444-466`). The delta adds that case as a stated exception instead of ignoring the requirement.

## Goals / Non-Goals

**Goals:**
- The harness verdict equals the model's fixed decision on every golden vector on Linux, macOS and Windows, junction vectors included.
- No decision uses an unresolved path. Every failure other than "does not exist" denies.
- Links and junctions that stay inside a root, and roots behind a link or junction, keep working.
- Path forms change only where D3 lists them.

**Non-Goals:**
- Closing the race between the check and the open (Risks).
- F15 (the workdir as an allowed root). `newVirtualPathResolverForFilesystem` changes only in its signature formatting.
- Case-insensitive comparison on macOS: `pathWithin` (`virtual_policy.go:350-356`) keeps comparing strings.
- Host binaries: an allowed binary runs as a host process and is not path-checked.
- Resolving `subst` drives or mapped network drives. They are not reparse points; roots and requests keep the same drive form, as today.

## Decisions

### D1. Mechanism: walk the components and follow every link (brief candidate (c))

The resolver walks the cleaned absolute path from its volume root, one component at a time, on a prefix that is already resolved:

1. `Lstat` the next component under the resolved prefix.
   - It does not exist: stop. The prefix is the deepest existing ancestor; this component and the rest are the missing suffix.
   - Any other error: return it. The caller denies.
2. The entry is a link when its mode has `ModeSymlink`, or has `ModeIrregular` without `ModeDir` (on Windows, a name-surrogate reparse point such as a junction). Read it with `os.Readlink`.
   - A `Readlink` error is returned, whatever its kind. `os.Readlink` reports `ENOENT` for a reparse point that is not a link, so a not-exist error here must not be read as a missing component.
   - An absolute target replaces the prefix. A target with no volume name that starts with a separator (Windows) is rooted on the volume of the link's directory. Any other target is joined to the link's directory, which is the resolved prefix, so `..` in a target is applied to a physical path.
   - The walk continues with the target followed by the remaining components. More than 255 links followed is an error (the bound `filepath.EvalSymlinks` uses, `GOROOT/src/path/filepath/symlink.go:100`).
3. Any other entry is appended to the prefix.
4. If the volume root itself does not exist, there is no existing ancestor: return a dedicated error (`errVirtualPathNoExistingAncestor`). This is the missing drive and missing UNC share case.
5. Put the existing prefix in the platform's canonical form with `filepath.EvalSymlinks`. The prefix holds no link, so on Linux and macOS this returns it unchanged, and on Windows it applies only the case, long-name and drive-letter rewrite that today's result already has. An error here is returned.
6. Return the canonical prefix joined with the missing suffix.

Because step 1 never inspects a component through an unresolved link, its "does not exist" answer is reliable on Windows. That is the property `EvalSymlinks` lacks there.

A dangling link is followed like any other (step 2 does not need the target to exist), so the walk ends in the target's directory, and the missing target is the suffix.

The resolver reads the filesystem through three injected functions (`lstat`, `readlink`, and the canonical-form function of step 5), with the `os` and `filepath` functions in production. A fake filesystem then tests, on Linux and in the Linux mutation lane, what only Windows can materialise: a junction-like entry, an unreadable reparse entry, a rooted target, a missing volume root and the canonical-form step.

Rejected alternatives:
- **(b) `os.Root` per allowed root.** `os.Root` rejects a link with an absolute target even when the target is inside the root (B1 design, Context; decision 46). Junctions always have absolute targets, and so does a typical in-root symlink made with an absolute path. That would deny in-root links, which the brief forbids for this policy. It also needs the request already expressed relative to a root, which is the comparison this change is fixing (`/tmp/x` against the root `/private/tmp`). B1's recovery wrapper is not needed, because `os.Root` is not used.
- **(a) A handle-based final path on Windows (`CreateFile` plus `GetFinalPathNameByHandle`), with `EvalSymlinks` elsewhere.**
  - Opening a dangling link follows it and is expected to report "not found" (not run on Windows), so this would not see F19 there, and `EvalSymlinks` cannot see it on Linux and macOS (probe row 1). Both halves would still need the `Lstat` and `Readlink` step of this design.
  - It returns `\\?\`-prefixed paths that the caller must convert, and it is a second code path that only Windows runs, outside the Linux mutation lane.
- **Keep `EvalSymlinks` and walk up until it succeeds.** Unsafe on Windows (the probe: a path through a junction looks missing, and the junction itself comes back unresolved), and blind to dangling links everywhere.

### D2. Errors

- The resolver returns an error for: an empty path, a failed `os.Getwd`, any `Lstat` error other than not-exist, any `Readlink` error, more than 255 links, a missing volume root, and a failed canonical-form step. Each error names the path.
- `validate` returns the resolver's error under `restricted` and under `full` access, so the handlers do not perform the operation. Under `full` access these paths fail today as well, later, with the operating system's error.
- `normalizedRoots` keeps dropping a root whose normalisation fails. A request below such a root meets the same error on the same prefix, so no request survives an error that removed its root.
- `resolveVirtualFilesystemPaths` keeps returning the error, wrapped with the entry's name. Today that error is reachable only for an empty value or a failed `os.Getwd`; now it is also reachable for a declared path that cannot be resolved, which fails the command before the script runs (proposal, What Changes).

Behaviour that follows and is accepted:
- On Linux and macOS, `root/f/x` with `f` a regular file is denied (`Lstat` reports `ENOTDIR`). On Windows the same request reports not-exist and is judged by where `f` is. Nothing can be created below a file, so neither is an escape.
- A reparse point that is not a link and that Go reports as irregular (an AppExecLink file, for example) is denied, because its target cannot be read. See Open questions.

### D3. The resolved path is the checked path and the reported path

`validate` returns the resolved path, and the handlers perform the operation on it, as they do today. The brief prefers to keep the reported form and use the resolved form only for the decision, unless that reopens the race between check and use. It would: a returned path that still contains a link or junction is resolved again by the operating system at open time, so retargeting the link after the check redirects the operation. Today a returned path has no symlink left in it; keeping junctions and deep paths unresolved in the returned path would make them weaker than symlinks. One form also keeps roots and requests comparable by construction.

Every change of form, each one deliberate:

| # | Case | Before | After |
|---|---|---|---|
| 1 | Two or more missing components | cleaned input, links unresolved | resolved existing ancestor plus the missing names (macOS: `/tmp/a/b` becomes `/private/tmp/a/b`; Windows: the existing part gets its long names and on-disk case; a link in the existing part is replaced by its target) |
| 2 | A path through a dangling link | the link's own path | the link's target path |
| 3 | Windows: a path that ends at or crosses a junction | the junction's own path | the junction's target path. A volume mount point's target is a volume path (`\\?\Volume{…}\…`) |
| 4 | A path the resolver cannot resolve | cleaned input | an error |

Where these show: the `virtual path denied: <path>` message, Lua `invowk.path` results, `INVOWK_PATH_*` values, and the path an operation runs on. `INVOWK_ANCHOR_*` values do not change (they are not normalised). Every path that exists or has one missing component, and crosses no junction and no dangling link, keeps today's form: the walk follows symlinks as `EvalSymlinks` does, and step 5 is the same canonical rewrite. Existing assertions on `/private/var` and on Windows long names are expected to be in that set; `make test` and the macOS and Windows CI jobs confirm it (tasks 3.7 and 6.2).

Rejected: a second "display" resolver that follows symlinks but not junctions. It adds a second walk whose output is not the checked path.

### D4. In-root links and roots behind links

- A link or junction is judged only by where its target resolves, so a target inside an allowed root is accepted for relative and absolute targets (D1 step 2).
- A root is normalised by the same function as a request, so a root given through a link or junction becomes its target, and a request written through the same link becomes a path below that target, at any depth. This removes today's false denial at two or more missing components (probe row 5; the macOS `/tmp/new1/new2` case).
- A junction or symlink whose target is written as a volume path is compared in that form. It is inside a root only when the root resolves to the same form. A project on a volume mounted as a folder works, because its root resolves through the same mount point.
- A volume mounted as a folder *inside* an allowed root resolves to another volume, so paths below it are denied under `restricted` access unless that folder is declared in `virtual.filesystem.paths`. Following the mount point is deliberate: a junction's target may be written as a volume path like any other target (not run on CI), so treating volume targets as plain directories would leave a way around the F18 fix.

### D5. Consumers

No caller changes its call shape. `normalizeExistingOrParent` keeps its name and signature, so the `normalize` correspondence row keeps its symbol; its doc comment states the new contract. The walk is a helper in the same file, with its own correspondence row (D6). Each consumer gets a test (tasks 3.x): `validate`, `resolve`, `resolveBridgePath`, Lua `require`, the allowed roots, and declared `filesystem.paths`.

### D6. Model, golden and bindings

- **Request fields.** `link: lone LinkKind` (`Symlink`, `Junction`) and `dangling: one Bool`. Facts: a request whose lexical and physical ancestors differ has a link; a dangling request has `link = Symlink` and a depth other than `Zero`. A link whose ancestors are the same location is allowed (an in-root link).
  - Dangling junctions are left out: nothing can be created through a junction whose target directory is missing, and the fake-filesystem test covers the resolver's answer for one.
- **Evaluation modes.** `FixedEval` returns the physical ancestor. Each pre-fix mode differs from it in one defect:
  - `PreFixF14Eval`: the lexical ancestor at depth `TwoPlus`;
  - `PreFixF18Eval`: the lexical ancestor when `link = Junction`;
  - `PreFixF19Eval`: the lexical ancestor when `dangling = True`;
  - `NoParentEval` stays as the existing seeded mutant.
  `CurrentEval` is removed in the second commit (D7).
- **Manifest.** `findingF14DeepSymlink` becomes `mutantPreFixF14ParentOnlyEval`; `mutantPreFixF18JunctionNotFollowed` and `mutantPreFixF19DanglingLinkAsMissingName` are added; all three have `mutant_of = "harnessContainedFixed"`, `expect = "counterexample"` and no `finding` key. Witness commands are added for a junction ancestor, a dangling link and an in-location link. The `VirtualPathHarness: findingF14` entry leaves `unresolved_elements`; nothing is added to that list or to `binding_gaps`.
- **Golden.** `Decisions` records the fixed decision as `dAccepts` (today's `dAcceptsWorkdir`) and `dNormalized`; the current-code fields go away in the second commit. The workdir stays a root in the golden scenario (F15 is open).
  - The new fields take a request from 27 combinations to 81, so three requests per instance are too many. Requests are independent: `accepts` and `contained` read only the request and `H` (`VirtualPathHarness.als:96-106`). The golden scope goes to `exactly 2 Req`, which still shows every combination, because `noJunk` only needs the two requests to cover the three locations. The implement stage records the instance count and the replay time in the calibration text and in `formal/README.md`.
  - `make formal-golden` runs after every `.als` edit (Java is installed locally).
- **Golden replay.** For each request the replay builds: a plain directory (no link); a symlink to a directory in the physical location; on Windows a junction to it; for a dangling request, a symlink to a missing name in a directory of the physical location, with the request being the link (depth `One`) or the link plus one missing name (depth `TwoPlus`). It compares the validator with `dAccepts`. Junction requests are skipped off Windows through the capability policy, with their count logged, and the replay fails on Windows if it replayed none.
- **Correspondence rows.**
  - `normalize | normalizeExistingOrParent` keeps its binding; its abstraction cell describes the deepest-existing-ancestor walk.
  - New row `normalized | <walk helper> | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, <fake-filesystem test>`. `normalized` resolves in the model source, so no list entry is needed.
  - The `findingF14` row becomes a binding row for `TestVirtualPathHarness_DeepMissingPathThroughSymlinkDenied`, with the element cell naming `mutantPreFixF14ParentOnlyEval`.
  - New rows for `TestVirtualPathHarness_JunctionEscapeDenied` and `TestVirtualPathHarness_DanglingLinkEscapeDenied`, their element cells naming the F18 and F19 mutant commands. The junction row's abstraction cell says how junction vectors map to the model: `link = Junction`, materialised with `fstree.MustJunction`, replayed on Windows only.
- **Rapid property.** It compares in both directions (spec delta): the oracle is the physical location of the deepest existing ancestor, computed by the test from how it built the tree, not by calling the resolver.
- **Calibration.** The four resolver defects of the spec delta are seeded one at a time, shown to fail a named binding, reverted, and recorded in the calibration text.
- **Mutation ledger.** `normalizeExistingOrParent` (12 entries) is rewritten, and the helper becomes bound. The fix stage runs `make mutation-formal` and the partial refresh. The fake-filesystem test exists so that the junction and canonical-form branches are killed on Linux. A mutant that only a Windows run can kill is a deferred `binding-gap` with a follow-up, as B1 recorded for its Windows-only branch (`tasks/next/formal-binding-gaps.md:142`).

### D7. Commits

The code allows F18 to stand alone. F14 and F19 cannot be separated.

1. **F18 (junction-aware resolution).**
   - Code: the walk helper, used in place of the two `filepath.EvalSymlinks` calls. It counts as resolved only when no suffix is missing. The path, parent, lexical order of `normalizeExistingOrParent` stays.
   - Effect: on Linux and macOS nothing observable changes (F14 and F19 still reproduce). On Windows a junction is followed at zero and one missing component.
   - Formal: the `link` field, `PreFixF18Eval`, `mutantPreFixF18JunctionNotFollowed`, the junction and in-location witnesses, the golden at two requests regenerated, the replay with junction vectors still comparing with the current-code decision, the F18 README row, the fake-filesystem test.
   - Replay: `TestVirtualPathHarness_JunctionEscapeDenied` with the existing-file and new-file legs, an in-root junction accepted, and a root behind a junction accepted.
2. **F14 and F19 (deepest existing ancestor, no lexical fallback).**
   - Code: `normalizeExistingOrParent` returns the canonical prefix plus the suffix; both fallbacks go; the missing-volume error and the declared-path error wrapping are added.
   - Formal: the `dangling` field, `PreFixF14Eval`, `PreFixF19Eval`, `CurrentEval` removed, `findingF14DeepSymlink` renamed, the F19 mutant and witness, the golden regenerated and compared with `dAccepts`, the rapid exemption removed, the F14 replay inverted and renamed, the F19 replay, the F14 and F19 README rows.
   - Replay: the third leg of the junction replay (a new file two missing components below the junction) lands here, because that leg is F14's shape through a junction.
   - Also: the consumer tests, the macOS-shape test, the CLI test and the docs.
3. **`style(runtime)`: go:S8209.** `workDir, scriptBasePath string` in the two signatures (`virtual_policy.go:95-99`, `:123-127`).

Why F14 and F19 share a commit: both come from one decision in the old code, "when `EvalSymlinks` fails, the rest of the path is a missing plain name". F14 is that decision two levels up (`:347`), F19 is the same decision for the last name (`:345`), where the name is a link. A walk that decides existence per component has no place to keep one and drop the other; fixing F19 alone would mean writing a link check into the parent fallback that the F14 commit deletes. Their replays and their pre-fix mutants stay separate, and each mutant reproduces alone.

### D8. CLI evidence

`tests/cli/testdata/virtual_path_link_escape.txtar`, with an exact entry in `tests/cli/runtime_mirror_exemptions.json` ("restricted path denials exist only in the virtual runtimes"), as the anchor tests have (`runtime_mirror_exemptions.json:7-8`).

- The invowkfile lives in a subdirectory of `$WORK`, so `$WORK` itself is not the script base.
- The outside location must be outside `/tmp` off Windows, because `/tmp` is always a root there and `$WORK` is normally below it. The test uses `/var/tmp` as the existing outside directory off Windows and `$WORK/outside` on Windows. It names a fixed, test-specific entry below it, removes that entry first, and asserts it does not exist afterwards. A passing run creates nothing outside `$WORK`.
- `virtual-sh`: `mkdir -p link/<name>/a/b` (F14) and `echo x > dangling` (F19) fail with `virtual path denied`.
- `virtual-lua`: `io.open` on the deep path and on the dangling link returns an error containing `virtual path denied`.
- `[!symlink] skip`. Junctions are covered by the Go replays, not here (testscript has no junction command).

## Risks / Trade-offs

- **[Check and open still race]** After the check, the operation opens the resolved path. A concurrent process that replaces a directory on that path with a link can still redirect it. → Recorded here and as a model abstraction in the `validate` correspondence row. Not closed by this change (brief). Returning the resolved path (D3) keeps the window as small as it is for symlinks today.
- **[Form changes on Windows]** Scripts that compare `INVOWK_PATH_*` or `invowk.path` output with a junction's own path see the target instead. → Listed in D3 and in the release note; the same already holds for symlinks on every platform.
- **[Declared path on a missing drive]** The command now fails before the script runs. → The error names the entry; `virtual.filesystem.paths` is per platform.
- **[Windows behaviour not yet run]** The junction walk follows the probe and the Go source, but the walk itself has not run on Windows. → The junction replay, the junction golden vectors and the rapid junction branch are required on windows-latest, and the pull request quotes their names from the job (tasks 6.x).
- **[Golden cost]** The model grows two fields. → Two requests per instance; the implement stage measures and records the time.
- **[Mutants only Windows can kill]** → The fake-filesystem test; anything left is a deferred `binding-gap`.

## Migration Plan

No data migration. A script that wrote outside the allowed roots through a link or junction declares the target in `virtual.filesystem.paths`, or sets `virtual.filesystem.access: "full"`. Rollback is reverting the commits; the model, golden and manifest revert with them.

## Maintainer decisions (2026-11-24, brief `program-notes/briefs/b2-brief.md`; decisions 43, 45, 53)

1. The decision equals the model's physical containment on every platform; no lexical fallback remains.
2. Deepest existing ancestor; every error other than not-exist denies; a missing drive or share denies explicitly.
3. Junction-aware on Windows; the design chooses among (a), (b), (c). This design picks (c) (D1). No new false denials for in-root links; roots behind a link or junction keep working, by the same resolver; the race is recorded, not closed.
4. F18 is recorded with the probe run as evidence and closed in this change.
5. F14's formal artefacts change as the triage lists; all model, golden, replay, manifest and reach edits belong to the implement stage.
6. Every consumer has a test, including the macOS `/tmp/new1/new2` case on macOS CI.
7. A `virtual_*.txtar` shows the denial for `virtual-sh` and `virtual-lua`, with a mirror exemption.
8. Windows and macOS evidence is by test name, with run and job ids.
9. One commit per finding where the code allows it (D7).
10. Docs: `virtual.mdx`, `virtual-lua.mdx`, their pt-BR copies, `formal/README.md`; never "sandbox" or "isolated".
11. go:S8209 in the two signatures.
12. Path forms stay unless a change is deliberate and listed (D3).
13. Do not touch F15 or `.claude/issues/cst-adoption-gaps.md`.

## Open Questions

None of these changes the approach. Each has a default this change follows, and each is listed for the maintainer.

- **F19's id.** The dangling-link escape was found by this stage's probe and is not in the brief. Default: record it as F19, fixed after an observed escape, as F18 is. Alternative: describe it as a second shape of F14, with no new id; the replay and the mutant would stay, renamed.
- **Reparse points that are not links** (D2). Default: deny when the target cannot be read. Cloud-storage placeholder files are expected to look like regular files to ordinary processes, but no CI runner has one, so this is not verified. If a user reports a false denial, the follow-up is to treat an unreadable non-directory reparse entry as a plain file when it is the last component.
- **Volume mounted as a folder inside a root** (D4). Default: denied unless declared. No CI evidence; creating a volume mount point needs a spare volume.
- **`internal/audit` and other `EvalSymlinks` users.** Other packages compare `EvalSymlinks` results on Windows and have the junction blind spot the probe shows (B1's design lists `internal/audit/scan_files.go`). They are out of scope here; a tracker entry is suggested.
