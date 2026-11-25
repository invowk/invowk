# Design

## Context

See proposal.md (Why). Line numbers are from origin/main 693f0e17.

Code map:

- Step 1 of `discoverAllWithDiagnostics` reads the current directory through `discoverInDir` (`internal/discovery/discovery_files.go:155-158`, `:289-309`), which returns a file without `Module`. Its doc comment (`DiscoverAll`, `discovery_files.go:87-95`) lists the six precedence levels.
- Step 2 scans the current directory's children for modules (`discovery_files.go:161-162`). `discoverModulesInDirWithDiagnostics` judges each entry the parent's `os.ReadDir` returns:
  - a symbolic-link entry is skipped, with `module_symlink_skipped` when its name ends in `.invowkmod` (`discovery_files.go:360-370`);
  - an entry that is not a directory is skipped (`discovery_files.go:372-374`). On Windows, Go 1.23 and later give a junction neither `ModeDir` nor `ModeSymlink`, and set `ModeIrregular` (`GOROOT/src/os/types_windows.go`, `mode`), so the scan skips a junction here, silently;
  - other entries need `invowkmod.IsModule` (`discovery_files.go:385`);
  - the reserved name `invowkfile` is skipped (`discovery_files.go:392-401`);
  - `invowkmod.Load` runs, and a failure is reported as `module_load_skipped` (`discovery_files.go:404-414`).
- `IsModule` requires the exact `.invowkmod` suffix (case-sensitive `strings.HasSuffix`), a valid name prefix, and an `os.Lstat` of the entry that is a directory and not a symbolic link (`pkg/invowkmod/operations.go:36-64`). `Load` validates the tree (symbolic links, nested modules, the module id against the directory name; `pkg/invowkmod/operations_validate.go:134-145,184-200,240-266`) and reads only the root `invowkfile.cue` (`operations_validate.go:150-168`).
- A module file is parsed with `ParseLoadedModuleInvowkfile` (`internal/discovery/discovery.go:484`, `pkg/invowkfile/parse.go:139-151`), which attaches the module path and metadata that the consumers of module identity read. Other files are parsed with `invowkfile.Parse` (`discovery.go:485-487`).
- Vendored dependencies are scanned for files with `Module` set (`discovery_files.go:718`).
- Command publication puts a file without `Module` under the reserved source `invowkfile`, with bare names (`internal/discovery/discovery_commands.go:246-263`).

Names the current directory can carry:

- `os.Getwd` on Unix returns `$PWD` when it is the same file as `.` (`GOROOT/src/os/getwd.go`), so it keeps the letter case and the links the user typed.
- `filepath.EvalSymlinks` on Unix builds its result from the components as given and never changes their case. On Windows it normalises case and short (8.3) names (`toNorm`), and since Go 1.23 it does not follow junctions: `walkSymlinks` follows only `ModeSymlink` (`GOROOT/src/path/filepath/symlink.go`).
- `os.ReadDir` of the parent returns the name the directory stores. That is the name the parent scan judges.

Routes that read an `invowkfile.cue` (maintainer decision 1):

- Every command frontend builds discovery through `internal/app/commandadapters/discovery.go:482`. No other production path parses an invowkfile to run its commands.
- `invowk validate <path>` and `invowk audit` judge explicit targets by the `.invowkmod` suffix (`cmd/invowk/validate.go:241-256`, `internal/audit/scan_context.go:321-346`). Audit's directory branch also runs discovery with `WithBaseDir(target)` (`scan_context.go:573-590`), so the rule below reaches it there. The `invowk agent` parses (`internal/agentcmd/*.go`) only validate generated text.
- Configured `includes` name module directories and pass through `IsModule` (`discovery_files.go:458`; `internal/config/config_schema.cue:35-38`). An include cannot name a file.
- Discovery reads only `baseDir` itself and its children (`discovery_files.go:155-162`); it does not search parent directories.

Bare names:

- Every unambiguous command, module commands included, is registered under its simple name (`cmd/invowk/cmd_discovery.go:67-99`). Resolution accepts a simple name that only one source defines (`internal/app/commandsvc/service.go:496-511`).
- An ambiguous name fails before any first-wins lookup (`service.go:496-497`), for the `invowkfile` source too.
- Publishing the current-directory module's commands under its namespace therefore keeps their bare names whenever they are unique. No second parse path is needed.

Collisions:

- Two discovered entries with the same module id and no explicit namespace collide (`discovery.go:326-376`). Entries are not compared by location.
- An include alias is matched to a module by `filepath.Clean` of the path text (`discovery.go:446-460`).

## Goals / Non-Goals

**Goals:**

- One rule, in discovery, decides whether the current directory is a module directory. Its consumers (runtimes, env builder, dependency checks, collisions, vendored policy) do not change.
- The rule cannot fall back to a plain parse of a file in a module directory, whatever spelling, letter case or link the current directory is reached through.

**Non-Goals:**

- Changing how the scan of a parent directory judges an entry.
- Changing how `invowk validate <path>` and `invowk audit` classify an explicit target before their own scan. (Audit's discovery pass gets the rule through `WithBaseDir`.)
- Recognising a directory as a module by its contents.
- Examining directories above the current directory. The rule judges the current directory's own entry.

Deduplication (D5) applies to every discovery source, not only the current directory.

## Decisions

### D1. Where the rule lives, and what it judges

A new step-1 helper in discovery (`internal/discovery/discovery_cwd_module.go`) runs before `discoverInDir`. It takes `absBase = filepath.Abs(baseDir)` and `info = os.Lstat(absBase)`.

1. **The stored entry.** The helper reads `os.ReadDir(filepath.Dir(absBase))` and looks for the entry that is `os.SameFile` with `info`. It checks the entries whose names equal `filepath.Base(absBase)` ignoring case first, and compares every other entry only when none of those match (Windows short names, Unicode normalisation on macOS). That entry's name is the stored name. A volume root has no stored entry.
2. **Candidate.** The current directory is a module-directory candidate when any known name ends in `.invowkmod`, compared ignoring case: the logical base name, the stored name, the base name of `filepath.EvalSymlinks(absBase)`, and, for a link entry (step 3), the base name of its resolved target. When there is no stored entry (the parent cannot be listed, or no entry matches; a volume root has none):
   - a candidate by the other names fails closed (D4), since it cannot be classified;
   - a non-candidate is a plain directory only when `EvalSymlinks` succeeded; if it failed too, the helper fails closed rather than guess.
3. **Link entry.** The entry is a link when `info` is not a directory and has `ModeSymlink` or `ModeIrregular` (a junction or other name-surrogate reparse point on Windows). Its target is resolved by following `os.Readlink` (which reads junction targets) and `filepath.EvalSymlinks`. A link entry that is a candidate gets `module_symlink_skipped` and loads nothing. A link entry that is not a candidate is a plain directory, unchanged. A link whose target cannot be resolved fails closed (D4).
4. **Directory entry.** A candidate directory is classified through `loadModuleEntry` (D2) with its parent and its stored entry, exactly as the parent scan would classify it. A directory that is not a candidate goes to `discoverInDir`, unchanged.

For a candidate, discovery:

- does not call `discoverInDir`;
- skips step 2, because a module directory's children are part of that module: `Load` rejects nested modules, and vendored modules are reached through the module itself;
- passes the result through `appendModulesWithVendored`, so the vendored scan and its hash checks apply.

Consequences:

- A case variant (`TOOLS.INVOWKMOD` typed for a stored `tools.invowkmod` on a case-insensitive volume) loads the module under its stored name, as the parent scan does.
- A stored name whose suffix differs in case (`Tools.INVOWKMOD`) is a candidate but not a module: it loads nothing, with `module_load_skipped`. The parent scan does not load it either. On a case-sensitive volume this is a directory the parent scan ignores; it is never read as a plain invowkfile directory.
- `EvalSymlinks` is never the only source of the decision for a directory entry, so its failure on some Windows volumes does not open a path.

Alternatives considered:

- Patching each consumer to infer module identity from the path. Rejected: many places, and each could drift.
- Testing the logical and `EvalSymlinks` base names only. Rejected: neither normalises case on macOS, and `EvalSymlinks` does not follow junctions on Windows.
- Making `IsModule` case-insensitive. Rejected: it changes the parent scan (a non-goal) and the module-id check in `Load`.

### D2. One entry classifier for both directions

The loop body of `discoverModulesInDirWithDiagnostics` (`discovery_files.go:356-420`) becomes a helper, `loadModuleEntry(parentDir, entry fs.DirEntry)`, which returns the file or the diagnostics. The child scan and the current-directory helper both call it, the latter with the stored entry from D1.

The helper applies, in the same order and with the same codes:

- the symbolic-link skip;
- the `IsModule` gate;
- the reserved name;
- `Load`.

The new `Load` caller stays gated by `IsModule`. The `ModulePathContainment` model assumes this of every production `Load` caller (`formal/README.md:326-332`, fact `isModuleRejectsLinkedRoot`, `formal/alloy/ModulePathContainment.als:37`).

Diagnostic codes for the current directory, one per case:

| Case | Code |
| --- | --- |
| Link entry (symbolic link or junction) that is a candidate | `module_symlink_skipped` (emitted by the D1 helper, whatever the stored name) |
| Reserved name `invowkfile.invowkmod` | `reserved_module_name_skipped` (from `loadModuleEntry`) |
| `Load` fails (inner link, nested module, id mismatch, missing `invowkmod.cue`) | `module_load_skipped` (from `loadModuleEntry`) |
| Candidate that `loadModuleEntry` returns neither a file nor a diagnostic for (stored suffix in another case, invalid name prefix, not a directory) | `module_load_skipped` (added by the D1 helper) |
| `Lstat` of the current directory fails, a candidate has no stored entry, the stored entry and `EvalSymlinks` both fail, or a link target cannot be resolved | `module_load_skipped` (added by the D1 helper) |

From the current directory, a candidate's skip is never silent.

### D3. Links

Only the current directory's own entry counts (D1 step 3). `os.Lstat` does not follow its last component, so a link in an ancestor directory, such as `/var` to `/private/var` on macOS, does not change the result.

A junction module directory is not loaded in either direction: the parent scan skips it as a non-directory entry, and the current-directory helper reports `module_symlink_skipped`. A junction without the suffix whose target is a module directory gets `module_symlink_skipped` too.

### D4. Failure never falls back

A candidate that does not load contributes no file, and neither does a current directory whose own entry cannot be judged (D2 table). `discoverInDir` is not called on those paths at all, so no code path can parse a file in a module directory as a plain invowkfile.

### D5. Deduplicate by physical directory

`appendModulesWithVendored` keeps, for one discovery run, the `os.Stat` result of every top-level module directory it has appended (entries with `ParentModule == nil`). When a new entry is `os.SameFile` as a kept one, discovery:

- drops the new entry and does not scan its vendored modules;
- sets the kept entry's `IsGlobalModule` when the new entry is global, and sets it on the kept entry's vendored children too, so ScopeConstruction's `globalChildren` keeps holding;
- when the kept entry has no explicit namespace (`CommandNamespace`, or an include alias found by `getAliasForModulePath`), and the dropped entry has one, sets the dropped entry's namespace as the kept entry's `CommandNamespace`. When several dropped entries have one, the first in precedence order wins.

A directory that cannot be `Stat`ed is not deduplicated, which keeps the collision check for it.

Precedence: the current-directory module is appended at step 1, so it is the kept entry against includes, provisioned entries and the user commands directory.

Alternatives considered:

- Comparing `EvalSymlinks` strings. Rejected: case-insensitive volumes spell one directory in several ways, and `os.SameFile` compares device and inode (file index on Windows).
- A post-pass over all files. Rejected: it would scan, hash-verify and report the vendored modules of duplicates before dropping them.

The collision check is otherwise unchanged. Two different directories with the same module id still collide (`discovery.go:340-351`).

### D6. Authoring verification

`verifyGeneratedCommand` hard-codes `FromSource: invowkfile` (`cmd/invowk/agent.go:715-731`). It will take the source from the discovered command set instead: the command whose `FilePath` is the written target and whose `SimpleName` is the written name. If none matches, it keeps `invowkfile`, which fails with the existing not-found error.

### D7. Formal

- The touched discovery functions are not planned in the formal-bindings mutation profile. `formal_mutation.py symbols` on 693f0e17 reports `CheckModuleCollisions`, `moduleIdentityFor` and the discovery helpers as unknown (they are binding gaps, `formal/manifest.toml:213-215`), and reports `IsModule` as planned. `IsModule` is not edited.
- The ScopeConstruction facts keep holding:
  - `moduleSourcesHaveIDs`: `RootSrc` iff no module (`formal/alloy/ScopeConstruction.als:66-70`);
  - `srcUnique`: deduplication runs before the collision check;
  - `globalChildren`: see D5.
- The `RootSrc` row note (`ScopeConstruction.als:26`) is reworded to "a current-directory invowkfile outside a module directory", if `python3 scripts/formal.py correspondence` accepts the wording.

## Maintainer decisions (2026-11-25)

1. Key the rule on where the file lives, not on a consumer. Fix it once, in discovery.
2. A module directory is recognised by its `.invowkmod` suffix only, judged on the stored name as the parent scan judges it. A plain project root carries an `invowkmod.cue` for `requires`: `module add` and `module sync` edit `./invowkmod.cue` (`cmd/invowk/module_deps.go:172,211,300`). "Contains `invowkmod.cue`" would misclassify every project with dependencies.
3. Links: judge only the module directory's own entry, with `Lstat`. An ancestor link never counts.
4. Fail closed: a module that does not load is skipped with a diagnostic, never plain-parsed.
5. Keep bare command names from inside the module, as long as no second plain-parse path is needed. None is (see Context).
6. Deduplicate the same module by physical directory in this change.
7. Every module behaviour gets a test pair: one run from the parent directory, one from the module directory. Add the macOS ancestor-link case, the case-variant case and the Windows junction case.

## Risks / Trade-offs

- [Users who run a module's commands in its directory as `@invowkfile <cmd>`, or bare when the name is ambiguous] → **BREAKING**, stated in the proposal's migration notes. Bare names keep working when they are unique.
- [A module that `invowk validate` rejects stops running in its own directory] → Intended: a module is checked the same way everywhere. `invowk validate` names the problem.
- [A plain directory whose name ends in `.invowkmod` in any letter case stops loading] → Intended: such a name is reserved for modules. The diagnostic names the directory; the migration notes say to rename it.
- [The stored-name lookup reads the parent directory on every run] → One `ReadDir`; `Lstat`/`SameFile` runs on the name that matches ignoring case, and on every entry only when none does.
- [Deduplication hides a misconfiguration, such as an include that names the current module] → The module is the same directory, so there is nothing to disambiguate. Different directories with the same id still fail.
- [README conflicts with the open PR that also edits README] → Edit only the discovery sections, and rebase before the gates.
