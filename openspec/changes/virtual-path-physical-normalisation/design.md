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

  The probe itself is not kept. The evidence a reader can re-run is the F19 replay of this change against the product file of c2fe96ff (D6, "Finding evidence").
- **Operations run on the path `validate` returns.** The u-root built-ins that take path operands replace each operand by the resolved path (`internal/uroot/registry.go:129`, `:339-349`), and the open, readdir and stat handlers pass the resolved path on. So an operation on a live symlink already acts on the link's target on every platform: `rm link` removes the target file, not the link.
- **Links whose target text is not a path (Linux), spec-review probe** (the reviewer's prototype of D1, Go 1.27.4, stdout piped). `/dev/stdout`, `/dev/stderr` and `/dev/stdin` are links to `/proc/self/fd/N`. When the descriptor is a pipe or a socket, `os.Readlink` of `/proc/<pid>/fd/N` returns text such as `pipe:[34643641]`, which names no file, although the kernel follows the link.
  - Today `filepath.EvalSymlinks("/dev/stdout")` fails, the parent fallback returns `/dev/stdout`, and the open succeeds. Under `full` access `echo msg > /dev/stderr` therefore works when stderr is a pipe (the normal case in CI).
  - Under `restricted` access today, with `/dev` declared in `virtual.filesystem.paths`: `/dev/stdout` is accepted when the descriptor is a pipe (the fallback) or a terminal (`/dev/pts/N`), and already denied when it is a regular file outside the roots (`EvalSymlinks` succeeds and returns that file).
  - No test uses these paths today (`tests/cli/testdata`, `internal/runtime`, `internal/uroot`).
  - A descriptor duplication (`>&2`, `>&1`, `<&0`) opens no path and does not reach the open handler (`mvdan.cc/sh/v3@v3.13.1/interp/runner.go:988`, `:1003`).
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
- No containment decision uses an unresolved path. In every containment decision, a failure other than "does not exist" denies. `full` access makes no containment decision and hands such a path to the operating system (D2).
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

The resolver walks the cleaned absolute path from its volume root. It keeps a resolved prefix (it exists and holds no link) and a queue of the components still to read. The queue starts as the request's own components. They hold no `.` and no `..`, because the path was cleaned; only a link target can add one.

1. Take the next component from the queue.
   - Empty or `.`: drop it.
   - `..`: the prefix becomes its parent (a volume root is its own parent). The prefix is physical, so a `..` that a link target contributes is applied to a physical path.
   - A name: `Lstat` it under the prefix.
     - It does not exist: the walk ends (step 4).
     - Any other error: return it. The caller denies.
2. The entry is a link when its mode has `ModeSymlink`, or has `ModeIrregular` without `ModeDir` (on Windows, a name-surrogate reparse point such as a junction). Read it with `os.Readlink`.
   - A `Readlink` error is returned, whatever its kind. `os.Readlink` reports `ENOENT` for a reparse point that is not a link, so a not-exist error here must not be read as a missing component.
   - An absolute target replaces the prefix with the target's volume root. A target with no volume name that starts with a separator (Windows) is rooted on the volume of the link's directory. For any other target the prefix stays the link's directory.
   - The target's components go to the front of the queue and are read by step 1 like any others. The target is never cleaned as text. More than 255 links followed is an error (the bound `filepath.EvalSymlinks` uses, `GOROOT/src/path/filepath/symlink.go:100`).
3. Any other entry is appended to the prefix.
4. **End at a missing component.** The prefix is the deepest existing ancestor. The missing component and everything left in the queue form the missing suffix, and the suffix must hold plain names only:
   - an empty or `.` component is dropped;
   - a `..` is an error (`errVirtualPathParentOfMissing`, naming the path). The caller denies.
5. If the volume root itself does not exist, there is no existing ancestor: return a dedicated error (`errVirtualPathNoExistingAncestor`). This is the missing drive and missing UNC share case.
6. Put the existing prefix in the platform's canonical form with `filepath.EvalSymlinks`. The prefix holds no link, so on Linux and macOS this returns it unchanged, and on Windows it applies only the case, long-name and drive-letter rewrite that today's result already has. An error here is returned.
7. Return the canonical prefix joined with the missing suffix.

**Why a `..` after a missing component is refused** (spec review, blocking finding). Nothing in the suffix is inspected: nothing exists below a missing directory. A `..` there could only be applied as text, and the name after it would be joined to the prefix without being walked. Shape: allowed root `root`, sibling `outside`, `root/in` a link to `outside`, and `root/d` a link to `nope/../in/new` with `nope` missing. A restricted script can create both links with the u-root `ln` built-in.
- Read without this rule, the walk stops at `nope` and returns `root` joined with `nope/../in/new`, which is `root/in/new`. That text is inside the root and still holds the unwalked link `in`. The reviewer's prototype accepted it, and the open created `outside/new`.
- With the rule, `root/d` is refused. Linux and macOS refuse the path themselves, because they cannot traverse the missing `nope` (`open(root/d, O_CREATE)` returns `ENOENT`), so no path that works there today is lost. Windows was not run (D2, accepted behaviour).
- An absolute target (`<root>/nope/../in/new`) has the same shape and gets the same answer.
- Rejected: cleaning the target as text before walking it (`filepath.Join` of the prefix and the target). It is safe, because `in` is then walked, but it applies `..` to the text of a link target. For a target such as `sub/../x`, with `sub` a link to another directory, it would answer `x` in the link's own directory, where Linux and macOS answer `x` in the parent of `sub`'s target.

**Invariant of the result.** The returned path is a canonical existing prefix in which every component was inspected with `Lstat` and is not a link, followed by zero or more plain names, the first of which does not exist under the prefix. It holds no link, no `.` and no `..`. The operating system therefore reads the returned path the way the walk did (the race in Risks aside), even where its own reading of the original path would differ from the walk's. The operation runs on the returned path (D3), so such a difference cannot become an escape. The fake-filesystem test asserts the invariant on every result it checks.

Because step 1 never inspects a component through an unresolved link, its "does not exist" answer is reliable on Windows. That is the property `EvalSymlinks` lacks there.

A dangling link is followed like any other (step 2 does not need the target to exist), so the walk ends in the target's directory, and the missing target is the suffix.

**Unresolved-link record.** The walk also reports whether it ended, or failed, while it was reading components that came from a link target. If so it returns the outermost such link on the request's own path: the canonical form of that link's directory, the link's name, and the request's components after it. Callers that make a containment decision ignore the record. Only the `full` access fallback of D2 reads it.

The resolver reads the filesystem through three injected functions (`lstat`, `readlink`, and the canonical-form function of step 6), with the `os` and `filepath` functions in production. A fake filesystem then tests, on Linux and in the Linux mutation lane, what only Windows can materialise: a junction-like entry, an unreadable reparse entry, a rooted target, a missing volume root and the canonical-form step.

Rejected alternatives:
- **(b) `os.Root` per allowed root.** `os.Root` rejects a link with an absolute target even when the target is inside the root (B1 design, Context; decision 46). Junctions always have absolute targets, and so does a typical in-root symlink made with an absolute path. That would deny in-root links, which the brief forbids for this policy. It also needs the request already expressed relative to a root, which is the comparison this change is fixing (`/tmp/x` against the root `/private/tmp`). B1's recovery wrapper is not needed, because `os.Root` is not used.
- **(a) A handle-based final path on Windows (`CreateFile` plus `GetFinalPathNameByHandle`), with `EvalSymlinks` elsewhere.**
  - Opening a dangling link follows it and is expected to report "not found" (not run on Windows), so this would not see F19 there, and `EvalSymlinks` cannot see it on Linux and macOS (probe row 1). Both halves would still need the `Lstat` and `Readlink` step of this design.
  - It returns `\\?\`-prefixed paths that the caller must convert, and it is a second code path that only Windows runs, outside the Linux mutation lane.
- **Keep `EvalSymlinks` and walk up until it succeeds.** Unsafe on Windows (the probe: a path through a junction looks missing, and the junction itself comes back unresolved), and blind to dangling links everywhere.

### D2. Errors, and what `full` access does with them

- The strict resolver (`normalizeExistingOrParent`) returns an error for: an empty path, a failed `os.Getwd`, any `Lstat` error other than not-exist, any `Readlink` error, more than 255 links, a `..` after a missing component, a missing volume root, and a failed canonical-form step. Each error names the path. It has no fallback.
- Every containment decision uses the strict resolver, and an error there denies.
- `full` access has no containment decision, so there the path is only being put in a form the operating system can open. One helper holds the only fallback: it calls the strict resolver, and only when the access mode is `full` it replaces a result the operating system could not follow:
  - the walk ended or failed inside a link's target (D1, unresolved-link record): it returns that link's own path, which is the canonical form of the link's directory, the link's name and the request's remaining components. The operating system then follows the link when it opens the path, as it does today. This covers a dangling link, a link cycle, and `/dev/stdout`, `/dev/stderr`, `/dev/stdin` and `/proc/self/fd/N` on a pipe or socket (Context);
  - the strict resolver failed anywhere else, for a reason other than an empty path or a failed `os.Getwd`: it returns the cleaned absolute path, and the operating system reports its own error, as today.
  The spec review found the earlier text of this decision wrong: it refused these paths under `full` access and said they "fail today as well", which is false for the standard-stream links and for a non-link reparse file such as an AppExecLink.
- The allowed-root check never sees a fallback result: `validate` returns before that check under `full` access (as today, `virtual_policy.go:239-241`), and `normalizedRoots` and Lua `require` call the strict resolver. Brief requirement 1 ("no lexical fallback on the decision path") holds by structure.

| Consumer | `restricted` access | `full` access |
|---|---|---|
| `validate` (shell open, readdir and stat handlers; u-root built-ins; Lua file I/O) | strict; an error refuses the operation; the resolved path is checked against the allowed roots | no root check; the resolved path, or the fallback path |
| `resolveBridgePath` (Lua `invowk.path`) | strict; the error is returned to the script | the resolved path, or the fallback path |
| Declared `virtual.filesystem.paths` (`resolveVirtualFilesystemPaths`: `INVOWK_PATH_*`, name expansion, allowed roots) | strict; an error fails the command before the script runs and names the entry | the resolved path, or the fallback path; the command is not failed |
| Allowed roots (`normalizedRoots`) | strict; a root that cannot be resolved is dropped | not consulted |
| Lua `require` (base and candidates) | strict | strict (its script-base check does not depend on the access mode) |

- `normalizedRoots` keeps dropping a root whose normalisation fails. A request below such a root meets the same error on the same prefix, so no request survives an error that removed its root.
- `resolveVirtualFilesystemPaths` receives the access mode (`filesystem.EffectiveAccess()`, which its caller already has). Under `restricted` access it keeps returning the error, wrapped with the entry's name. Today that error is reachable only for an empty value or a failed `os.Getwd`. Now it is also reachable for a declared path that cannot be resolved: a link cycle, a missing drive or share, an entry below a directory the user cannot search (`Lstat` reports `EACCES`), an entry through a regular file (Linux and macOS), or an unreadable link. The command then fails before the script runs, even when the script never uses the handle (proposal, What Changes).

Behaviour that follows and is accepted, all under `restricted` access (under `full` access the fallback hands each of these to the operating system):
- On Linux and macOS, `root/f/x` with `f` a regular file is denied (`Lstat` reports `ENOTDIR`). On Windows the same request reports not-exist and is judged by where `f` is. Nothing can be created below a file, so neither is an escape.
- A reparse point that is not a link and that Go reports as irregular (an AppExecLink file, for example) is denied, because its target cannot be read. See Open questions.
- A link whose target holds `..` after a missing component is denied (D1 step 4). Linux and macOS refuse the same path themselves. Windows is expected to collapse `..` in a relative link target as text, so such a link may work there today when everything it names stays inside the root (not run). See Open questions.
- **Standard-stream links on a pipe or socket (Linux).** The link is judged like any other, by its target text. `/dev/stdout` resolves to `/proc/<pid>/fd/pipe:[N]`, which is outside `/dev` and names no file.
  - With `/dev` declared, the request is denied when the descriptor is a pipe or a socket. Today that case is accepted. A terminal stays accepted (`/dev/pts/N` lies in `/dev`), and a regular file outside the roots stays denied (Context).
  - With `/dev/stdout` itself declared, or `/proc`, the root and the request resolve to the same text, the request is accepted, and the open fails with "no such file or directory".
  - No declaration restores it under `restricted` access. A script uses a descriptor duplication (`>&2`, `>&1`), which opens no path, or `full` access.
  - No exemption is added: it would return a path that still holds a link, which D3 rejects, or it would need a probe that tells a kernel-followed link from a dangling one. See Open questions.
- **Windows device names** (`NUL`, `CON`, `\\.\NUL`, `\\.\pipe\…`) were not analysed against the volume-root check and the canonical-form step, and nothing here has run on Windows. They cross no link, so the pre-change verdict is the reference: a windows-latest test compares the validator with a frozen copy of the pre-change function for these names (task 3.7). If it disagrees, the implement stage stops and reports; it does not adapt the test.

### D3. The resolved path is the checked path and the reported path

`validate` returns the resolved path, and the handlers perform the operation on it, as they do today. The brief prefers to keep the reported form and use the resolved form only for the decision, unless that reopens the race between check and use. It would: a returned path that still contains a link or junction is resolved again by the operating system at open time, so retargeting the link after the check redirects the operation. Today a returned path has no symlink left in it; keeping junctions and deep paths unresolved in the returned path would make them weaker than symlinks. One form also keeps roots and requests comparable by construction.

Every change of form, each one deliberate:

| # | Case | Access | Before | After |
|---|---|---|---|---|
| 1 | Two or more missing components | both | cleaned input, links unresolved | resolved existing ancestor plus the missing names (macOS: `/tmp/a/b` becomes `/private/tmp/a/b`; Windows: the existing part gets its long names and on-disk case; a link in the existing part is replaced by its target) |
| 2 | A path that names or crosses a link whose target does not resolve (a dangling link, a standard-stream link on a pipe) | `restricted` | the link's own path (resolved parent plus the last name), or the cleaned input when the link is not the last component | the link's target path |
| 3 | The same | `full` | as row 2 | the link's own path: the canonical form of the link's directory, the link's name, and the rest of the request. Unchanged when the link is the last component (`/dev/stdout` stays `/dev/stdout`); when it is not, the part before the link is now resolved |
| 4 | Windows: a path that ends at or crosses a junction | both | the junction's own path | the junction's target path. A volume mount point's target is a volume path (`\\?\Volume{…}\…`) |
| 5 | A path the resolver cannot resolve | `restricted` | the resolved parent plus the last name, or the cleaned input | an error |
| 6 | The same | `full` | as row 5 | the cleaned input, or the link's own path as in row 3 when the failure is inside a link's target |

Where these show: the `virtual path denied: <path>` message, Lua `invowk.path` results, `INVOWK_PATH_*` values, and the path an operation runs on. `INVOWK_ANCHOR_*` values do not change (they are not normalised). Every path that exists or has one missing component, and crosses no junction and no link with an unresolved target, keeps today's form: the walk follows symlinks as `EvalSymlinks` does, and step 6 is the same canonical rewrite. Existing assertions on `/private/var` and on Windows long names are expected to be in that set; `make test` and the macOS and Windows CI jobs confirm it (tasks 3.7 and 6.2).

**What rows 2 and 4 do to an operation that acts on the link itself.** An operation runs on the validated path (Context), so it acts on the link's target. For a live symlink this is today's behaviour on every platform. It is new for:
- **a junction (Windows, both access modes):** `rm -rf <junction>` removes the target directory and its contents and leaves the junction in place. Today the validated path is the junction's own path, so it is expected to remove only the junction (not run on Windows). `mv`, `test -L` and `ls -l` likewise see the target;
- **a dangling link (`restricted` access):** `rm`, `ln -sf`, `mv` and `test -L` act on the missing target, so `rm dangling` reports "no such file" and leaves the link. Under `full` access row 3 keeps today's behaviour.

This is consistent with live symlinks, and one case deletes data that was left alone before, so it is in Risks, in the docs task and in the release note. Not replacing the last component for unlink-type operations would be a per-operation policy in the built-ins and the handlers; it is not part of this change (Open questions).

Rejected: a second "display" resolver that follows symlinks but not junctions. It adds a second walk whose output is not the checked path.

### D4. In-root links and roots behind links

- A link or junction is judged only by where its target resolves, so a target inside an allowed root is accepted for relative and absolute targets (D1 step 2).
- A root is normalised by the same function as a request, so a root given through a link or junction becomes its target, and a request written through the same link becomes a path below that target, at any depth. This removes today's false denial at two or more missing components (probe row 5; the macOS `/tmp/new1/new2` case).
- A junction or symlink whose target is written as a volume path is compared in that form. It is inside a root only when the root resolves to the same form. A project on a volume mounted as a folder works, because its root resolves through the same mount point.
- A volume mounted as a folder *inside* an allowed root resolves to another volume, so paths below it are denied under `restricted` access unless that folder is declared in `virtual.filesystem.paths`. Following the mount point is deliberate: a junction's target may be written as a volume path like any other target (not run on CI), so treating volume targets as plain directories would leave a way around the F18 fix.

### D5. Consumers

`normalizeExistingOrParent` keeps its name and signature, so the `normalize` correspondence row keeps its symbol; its doc comment states the new contract. The walk is a helper in the same file, with its own correspondence row (D6). Two call shapes change, both for the `full` access fallback of D2: `resolve` and `resolveBridgePath` call the fallback helper with the resolver's access mode, and `resolveVirtualFilesystemPaths` receives the access mode from its caller. `normalizedRoots` and Lua `require` keep calling `normalizeExistingOrParent`. Each consumer gets a test (tasks 3.x): `validate`, `resolve`, `resolveBridgePath`, Lua `require`, the allowed roots, and declared `filesystem.paths`, under `restricted` access and, where D2's table differs, under `full` access.

### D6. Model, golden and bindings

- **Request fields.** `link: lone LinkKind` (`Symlink`, `Junction`) and `dangling: one Bool`. Facts: a request whose lexical and physical ancestors differ has a link; a dangling request has `link = Symlink` and a depth other than `Zero`. A link whose ancestors are the same location is allowed (an in-root link).
  - Dangling junctions are left out: nothing can be created through a junction whose target directory is missing, and the fake-filesystem test covers the resolver's answer for one.
- **Evaluation modes.** `FixedEval` returns the physical ancestor. Each pre-fix mode differs from it in one defect:
  - `PreFixF14Eval`: the lexical ancestor at depth `TwoPlus`;
  - `PreFixF18Eval`: the lexical ancestor when `link = Junction`;
  - `PreFixF19Eval`: the lexical ancestor when `dangling = True`;
  - `NoParentEval` stays, with a new meaning. The fixed code has no parent evaluation, so the mode now stands for a resolver that uses the walk only when nothing is missing and returns the unresolved path for one or more missing components. That is the shipped scenario's "mutant that compares the unevaluated path", and a superset of `PreFixF14Eval`. Its model comment and its credit in the calibration text are rewritten to say so; the manifest command keeps the name `mutantNoParentEval`, so no shipped text changes.
  `CurrentEval` is removed in the second commit (D7).
- **No model change for the `..` rule or for the `full` access fallback.**
  - A model request is a path the resolver can resolve. A path the strict resolver refuses (a `..` after a missing component, a cycle, an unreadable link) has no request in the model, and the code denies it. The fake-filesystem test, the third leg of the F19 replay and the rapid property's "unresolvable" class cover it. The 81 combinations and the `exactly 2 Req` argument below stand.
  - `H.access = Full` already accepts every request, and `TestVirtualPathHarness_RootConstruction` binds it. The fallback only changes which path string a `full` access operation runs on.
- **Manifest.** `findingF14DeepSymlink` becomes `mutantPreFixF14ParentOnlyEval`; `mutantPreFixF18JunctionNotFollowed` and `mutantPreFixF19DanglingLinkAsMissingName` are added; all three have `mutant_of = "harnessContainedFixed"`, `expect = "counterexample"` and no `finding` key. Witness commands are added for a junction ancestor, a dangling link and an in-location link. The `VirtualPathHarness: findingF14` entry leaves `unresolved_elements`; nothing is added to that list or to `binding_gaps`.
- **Golden.** `Decisions` records the fixed decision as `dAccepts` (today's `dAcceptsWorkdir`) and `dNormalized`; the current-code fields go away in the second commit. The workdir stays a root in the golden scenario (F15 is open).
  - The new fields take a request from 27 combinations to 81, so three requests per instance are too many. Requests are independent: `accepts` and `contained` read only the request and `H` (`VirtualPathHarness.als:96-106`). The golden scope goes to `exactly 2 Req`, which still shows every combination, because `noJunk` only needs the two requests to cover the three locations. The implement stage records the instance count and the replay time in the calibration text and in `formal/README.md`.
  - `make formal-golden` runs after every `.als` edit (Java is installed locally).
- **Golden replay.** For each request the replay builds: a plain directory (no link); a symlink to a directory in the physical location; on Windows a junction to it; for a dangling request, a symlink to a missing name in a directory of the physical location, with the request being the link (depth `One`) or the link plus one missing name (depth `TwoPlus`). It compares the validator with `dAccepts`. Junction requests are skipped off Windows through the capability policy, with their count logged, and the replay fails on Windows if it replayed none.
  - Off Windows the junction requests are left out inside the test and counted with `t.Log`. `TestVirtualPathHarness_GoldenVectors` itself must be reported as passed on every platform, never as skipped: it is a killer (below), and the reach gate and the mutation run fail on a killer that is skipped.
- **Correspondence rows.**
  - `normalize | normalizeExistingOrParent` keeps its binding; its abstraction cell describes the deepest-existing-ancestor walk.
  - New row `normalized | <walk helper> | internal/runtime/virtual_policy.go | TestVirtualPathHarness_GoldenVectors, <fake-filesystem test>`. `normalized` resolves in the model source, so no list entry is needed.
  - The `findingF14` row becomes a binding row for `TestVirtualPathHarness_DeepMissingPathThroughSymlinkDenied`, with the element cell naming `mutantPreFixF14ParentOnlyEval`.
  - New row for `TestVirtualPathHarness_DanglingLinkEscapeDenied`, its element cell naming the F19 mutant command.
  - New row `mutantPreFixF18JunctionNotFollowed | <walk helper> | internal/runtime/virtual_policy.go | TestVirtualPathHarness_JunctionEscapeDenied | characterisation: …`. The abstraction cell begins `characterisation:` and then says that this is the Windows-only replay of the fixed F18 escape, how junction vectors map to the model (`link = Junction`, materialised with `fstree.MustJunction`), and that the test is skipped off Windows.
- **The junction replay is never a killer** (spec review, major finding).
  - The shipped requirement "Reach evidence fails closed" fails the gate when a killer is skipped (`scripts/formal_mutation.py:579-582`, `:2135-2137`), and `TestVirtualPathHarness_JunctionEscapeDenied` is skipped on Linux, where `make formal-reach` and `make mutation-formal` run.
  - A test is kept out of the killers by a row whose abstraction cell begins `characterisation:` (`scripts/formal.py:1216-1229`). That is the marking B1 used for its Windows-only leg (`formal/alloy/ModulePathContainment.als`, the `probeF16` row; shipped requirement "The F16 leg is never a killer"). The marking excludes the test by name on every row, so the junction replay is named on its own row only, and that row is reported as `characterisation-only`: listed, not gated.
  - Here the prefix only switches the killer status off. The replay asserts the fixed behaviour; it does not characterise an open defect.
  - Killers that run on Linux: `normalize` keeps `TestVirtualPathHarness_GoldenVectors`; `normalized` has the golden test and the fake-filesystem test, and the fake-filesystem test is what kills the junction branch there; the F14 and F19 rows have their symlink replays (symlinks are required on Linux by the capability policy).
  - On Windows the junction replay is evidence by test name from the CI job (task 6.2), not through the reach gate.
- **Test files.**
  - The three escape replays stay in `internal/runtime/virtual_path_harness_formal_test.go`. That file uses only helpers that it defines or that exist at c2fe96ff, so it compiles against the pre-change package (see "Finding evidence").
  - The fake-filesystem test names the new walk helper, so it goes in its own file, `internal/runtime/virtual_path_walk_formal_test.go`. A binding test must be in a `*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go` or `*_trace_test.go` file (`scripts/formal.py:1184`).
- **Finding evidence** (spec review, finding on F19).
  - F18 cites the windows-latest probe run.
  - F19 does not cite the spec stage's throwaway probe, which exists in no repository. It cites its replay: `TestVirtualPathHarness_DanglingLinkEscapeDenied` fails against the product code of c2fe96ff. c2fe96ff is on `main`, so the citation survives the squash merge; a commit of this branch would not.
  - Recipe, run by task 3.1 and quoted in the pull request: export c2fe96ff into a scratch directory (`git archive`, or a detached scratch worktree), replace `internal/runtime/virtual_path_harness_formal_test.go` there with this change's copy, and run `go test -run 'TestVirtualPathHarness_(DanglingLinkEscapeDenied|DeepMissingPathThroughSymlinkDenied)' ./internal/runtime/`. Both replays fail there.
- **Rapid property.** It compares in both directions (spec delta): the oracle is the physical location of the deepest existing ancestor, computed by the test from how it built the tree, not by calling the resolver. The oracle has three classes: resolves inside the root (must be accepted), resolves outside (must be denied), and unresolvable (must be denied). The generator draws link targets with `..` before and after a missing component, so the reviewer's two-link shape is in its space, and after every case it checks that nothing was created outside the root.
- **Calibration.** The five resolver defects of the spec delta are seeded one at a time, shown to fail a named binding, reverted, and recorded in the calibration text.
- **Mutation ledger.** `normalizeExistingOrParent` (12 entries) is rewritten, and the helper becomes bound. The fix stage runs `make mutation-formal` and the partial refresh. The fake-filesystem test exists so that the junction and canonical-form branches are killed on Linux. A mutant that only a Windows run can kill is a deferred `binding-gap` with a follow-up, as B1 recorded for its Windows-only branch (`tasks/next/formal-binding-gaps.md:142`).

### D7. Commits

The code allows F18 to stand alone. F14 and F19 cannot be separated.

1. **F18 (junction-aware resolution).**
   - Code: the walk helper, complete (D1 steps 1 to 6, the plain-name rule of step 4 and the unresolved-link record included), used in place of the two `filepath.EvalSymlinks` calls. A call counts as resolved only when the helper returns no error and no missing suffix. The path, parent, lexical order of `normalizeExistingOrParent` stays, so a helper error still falls through to the old fallbacks in this commit.
   - Effect: on Linux and macOS nothing observable changes (F14 and F19 still reproduce, and `/dev/stdout` on a pipe keeps today's result). On Windows a junction is followed at zero and one missing component.
   - The fake-filesystem test covers the whole helper here, the `..` rule included. The denial of the two-link shape through the validator arrives with commit 2, where the suffix is first returned.
   - Formal: the `link` field, `PreFixF18Eval`, `mutantPreFixF18JunctionNotFollowed`, the junction and in-location witnesses, the golden at two requests regenerated, the replay with junction vectors still comparing with the current-code decision, the F18 README row, the fake-filesystem test.
   - Replay: `TestVirtualPathHarness_JunctionEscapeDenied` with the existing-file and new-file legs, an in-root junction accepted, and a root behind a junction accepted.
2. **F14 and F19 (deepest existing ancestor, no lexical fallback).**
   - Code: `normalizeExistingOrParent` returns the canonical prefix plus the suffix; both fallbacks go; the missing-volume error and the declared-path error wrapping are added; the `full` access fallback helper of D2 is added and used by `resolve`, `resolveBridgePath` and `resolveVirtualFilesystemPaths`.
   - Formal: the `dangling` field, `PreFixF14Eval`, `PreFixF19Eval`, `CurrentEval` removed, `findingF14DeepSymlink` renamed, the F19 mutant and witness, the golden regenerated and compared with `dAccepts`, the rapid exemption removed, the F14 replay inverted and renamed, the F19 replay, the F14 and F19 README rows.
   - Replay: the third leg of the junction replay (a new file two missing components below the junction) lands here, because that leg is F14's shape through a junction.
   - Also: the consumer tests, the macOS-shape test, the CLI test and the docs.
3. **`style(runtime)`: go:S8209.** `workDir, scriptBasePath string` in the two signatures (`virtual_policy.go:95-99`, `:123-127`).

Windows CI runs on the pushed head only, so commit 1 is never run on Windows by itself. The first two legs of the junction replay are observed on the final head; that commit 1 alone follows a junction is shown on Linux by the fake-filesystem test. The pull request says so next to the Windows evidence.

Why F14 and F19 share a commit: both come from one decision in the old code, "when `EvalSymlinks` fails, the rest of the path is a missing plain name". F14 is that decision two levels up (`:347`), F19 is the same decision for the last name (`:345`), where the name is a link. A walk that decides existence per component has no place to keep one and drop the other; fixing F19 alone would mean writing a link check into the parent fallback that the F14 commit deletes. Their replays and their pre-fix mutants stay separate, and each mutant reproduces alone.

### D8. CLI evidence

`tests/cli/testdata/virtual_path_link_escape.txtar`, with an exact entry in `tests/cli/runtime_mirror_exemptions.json` ("restricted path denials exist only in the virtual runtimes"), as the anchor tests have (`runtime_mirror_exemptions.json:7-8`).

- The invowkfile lives in a subdirectory of `$WORK`, so `$WORK` itself is not the script base.
- The outside location must be outside `/tmp` off Windows, because `/tmp` is always a root there and `$WORK` is normally below it. The test uses `/var/tmp` as the existing outside directory off Windows and `$WORK/outside` on Windows. It names a fixed, test-specific entry below it, removes that entry first, and asserts it does not exist afterwards. A passing run creates nothing outside `$WORK`.
- `virtual-sh`: `mkdir -p link/<name>/a/b` (F14), `echo x > dangling` (F19, a redirection) and `touch dangling` (F19, a u-root built-in) fail with `virtual path denied`. In the same restricted command `echo msg >&2` succeeds: a descriptor duplication is not path-checked. The `touch` step is the u-root evidence that fails before the fix: `mkdir -p` may already be denied there, because the built-in checks each component.
- `virtual-lua`: `io.open` on the deep path and on the dangling link returns an error containing `virtual path denied`.
- A second command with `virtual.filesystem.access: "full"`, off Windows only: `echo msg > /dev/stderr` succeeds and `msg` is on stderr. testscript gives the process a pipe as stderr, which is the case D2's fallback exists for.
- `[!symlink] skip`. Junctions are covered by the Go replays, not here (testscript has no junction command).
- The test is run once against the pre-fix tree, before the resolver changes (task 3.1): the two dangling-link steps must fail there, and the `full` access step must pass there.

## Risks / Trade-offs

- **[Check and open still race]** After the check, the operation opens the resolved path. A concurrent process that replaces a directory on that path with a link can still redirect it. → Recorded here and as a model abstraction in the `validate` correspondence row. Not closed by this change (brief). Returning the resolved path (D3) keeps the window as small as it is for symlinks today.
- **[Form changes on Windows]** Scripts that compare `INVOWK_PATH_*` or `invowk.path` output with a junction's own path see the target instead. → Listed in D3 and in the release note; the same already holds for symlinks on every platform.
- **[Operations on a junction or a dangling link act on the target]** `rm -rf <junction>` on Windows removes the target directory's contents instead of unlinking the junction, inside a root under `restricted` access and anywhere under `full` access. Under `restricted` access, `rm`, `mv`, `ln -sf` and `test -L` on a dangling link act on its missing target. → Live symlinks already behave this way on every platform. Listed in D3, in the docs (task 3.11) and in the release note. A per-operation rule for unlink-type operations is an open question.
- **[Standard-stream device paths under `restricted` access]** On Linux, `/dev/stdout`, `/dev/stderr`, `/dev/stdin` and `/proc/self/fd/N` on a pipe or socket are no longer accepted through a declared `/dev` (D2). → `>&2` and `>&1` are unaffected; `full` access keeps these paths working, with a CLI step that proves it (D8). In the docs and the release note.
- **[Declared path that cannot be resolved]** Under `restricted` access the command now fails before the script runs: a missing drive or share, a link cycle, a directory the user cannot search, a regular file used as a directory. → The error names the entry; `virtual.filesystem.paths` is per platform; under `full` access the entry keeps a fallback path and the command runs (D2).
- **[Windows behaviour not yet run]** The junction walk follows the probe and the Go source, but the walk itself has not run on Windows. → The junction replay, the junction golden vectors and the rapid junction branch are required on windows-latest, and the pull request quotes their names from the job (tasks 6.x).
- **[Golden cost]** The model grows two fields. → Two requests per instance; the implement stage measures and records the time.
- **[Mutants only Windows can kill]** → The fake-filesystem test; anything left is a deferred `binding-gap`.

## Migration Plan

No data migration. A script that wrote outside the allowed roots through a link or junction declares the target in `virtual.filesystem.paths`, or sets `virtual.filesystem.access: "full"`. Rollback is reverting the commits; the model, golden and manifest revert with them.

## Maintainer decisions (2026-11-24, brief `program-notes/briefs/b2-brief.md`; decisions 43, 45, 53)

1. The decision equals the model's physical containment on every platform; no lexical fallback remains.
2. Deepest existing ancestor; every error other than not-exist denies; a missing drive or share denies explicitly. This design applies "denies" wherever a containment decision is made; what `full` access does with such a path is an open question with a default (D2, Open Questions).
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

- **F19's id and the process exception.** The dangling-link escape was found by this stage's probe and is not in the brief; the brief names F18 and F14 only. Fixing it is entailed by brief requirement 1, so only the bookkeeping is open.
  - Default: record it as F19, fixed after an observed escape, as F18 is. Its evidence is the replay failing against c2fe96ff (D6), which a reader can re-run.
  - Alternative: describe it as a second shape of F14, with no new id; the replay and the mutant would stay, renamed.
  - The change also rewrites the shipped rule "Defect fixes are separate from finding records" for every later change. The exception is narrowed to evidence that can be re-run (a CI run and job id, or a committed test named with the `main` commit it fails against). The maintainer may prefer to keep the rule as shipped and treat F18 and F19 as one-off decisions.
- **`full` access and paths the resolver cannot express** (D2; spec review, major finding). Default: under `full` access a path that cannot be resolved, or that names a link whose target does not resolve, is handed to the operating system in the link's own form or as the cleaned input, as today. Alternative: refuse it under `full` access too. That breaks `> /dev/stderr` on a pipe and `test -e` on an AppExecLink, and protects nothing, because `full` access has no allowed-root check.
- **Standard-stream device paths under `restricted` access** (D2). Default: no exemption; they are judged by their target text, so a declared `/dev` no longer admits `/dev/stdout` on a pipe. Alternative: when a link's target text does not resolve and the operating system can still follow the link (`Stat` of the link succeeds), refuse with an error that says the target is not a path. That adds a fourth injected function to the walk and only improves the message. Accepting such a link is not proposed.
- **`..` after a missing component on Windows** (D1, D2). Default: refused on every platform. Not run on Windows, where such a link may work today.
- **Operations that act on a link itself** (D3). Default: they act on the resolved target, as live symlinks do today, which changes `rm -rf <junction>` on Windows. Alternative, as a later change: keep the last component of an existing path unresolved for unlink-type operations.
- **Reparse points that are not links** (D2). Default: deny under `restricted` access when the target cannot be read; under `full` access the fallback hands the path to the operating system. Cloud-storage placeholder files are expected to look like regular files to ordinary processes, but no CI runner has one, so this is not verified. If a user reports a false denial, the follow-up is to treat an unreadable non-directory reparse entry as a plain file when it is the last component.
- **Volume mounted as a folder inside a root** (D4). Default: denied unless declared. No CI evidence; creating a volume mount point needs a spare volume.
- **`internal/audit` and other `EvalSymlinks` users.** Other packages compare `EvalSymlinks` results on Windows and have the junction blind spot the probe shows (B1's design lists `internal/audit/scan_files.go`). They are out of scope here; a tracker entry is suggested.
