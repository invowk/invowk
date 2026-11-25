# Design

## Context

See proposal.md (Why). Line numbers are from origin/main 693f0e17.

Discovery today:

- Step 1 of `discoverAllWithDiagnostics` loads the current directory's invowkfile through `discoverInDir` (`internal/discovery/discovery_files.go:155-158`). `discoverInDir` returns a `DiscoveredFile` without `Module` for `invowkfile.cue` or an extensionless `invowkfile` (`discovery_files.go:289-309`). It does not look at the directory's name.
- Step 2 scans the current directory's children for modules (`discovery_files.go:161-162`). Each child entry is judged in `discoverModulesInDirWithDiagnostics`:
  - a symbolic-link entry is skipped, with `module_symlink_skipped` when its name ends in `.invowkmod` (`discovery_files.go:360-370`);
  - other entries need `invowkmod.IsModule` (`discovery_files.go:385`);
  - the reserved name `invowkfile` is skipped (`discovery_files.go:392-401`);
  - `invowkmod.Load` runs, and a failure is reported as `module_load_skipped` (`discovery_files.go:404-414`).
- `IsModule` requires the `.invowkmod` suffix, a valid name prefix, and an `os.Lstat` of the entry that is a directory and not a symbolic link (`pkg/invowkmod/operations.go:36-64`). `Load` validates the tree: symbolic links, nested modules, the module id against the directory name (`pkg/invowkmod/operations_validate.go:134-145,184-200,240-266`). It reads only `invowkfile.cue` (`operations_validate.go:150-168`).
- A file without `Module` is parsed with `invowkfile.Parse` (`internal/discovery/discovery.go:485-487`). A module file is parsed with `ParseLoadedModuleInvowkfile` (`discovery.go:484`, `pkg/invowkfile/parse.go:139-151`), which attaches the module path and metadata that the consumers of module identity read.
- Vendored dependencies are scanned only for files with `Module` set (`discovery_files.go:718`).
- Command publication puts a file without `Module` under the reserved source `invowkfile`, with bare names (`internal/discovery/discovery_commands.go:246-263`).
- Every command frontend builds discovery through `internal/app/commandadapters/discovery.go:482`. No other production path parses an invowkfile to run its commands. `invowk validate <path>` and `invowk audit` judge explicit targets by the `.invowkmod` suffix (`cmd/invowk/validate.go:241-256`, `internal/audit/scan_context.go:321-346`). The `invowk agent` parses (`internal/agentcmd/*.go`) only validate generated text.
- Configured `includes` name module directories and pass through `IsModule` (`discovery_files.go:458`; `internal/config/config_schema.cue:35-38`). An include cannot name a file.
- Discovery does not search parent directories: only `baseDir` itself is read (`discovery_files.go:155-162`).

Bare names:

- Every unambiguous command, module commands included, is registered under its simple name (`cmd/invowk/cmd_discovery.go:67-99`). Resolution accepts a simple name that only one source defines (`internal/app/commandsvc/service.go:496-511`).
- An ambiguous name fails before any first-wins lookup (`service.go:496-497`), for the `invowkfile` source too.
- Publishing the current-directory module's commands under its namespace therefore keeps their bare names whenever they are unique. No second parse path is needed.

Collisions:

- Two discovered entries with the same module id and no explicit namespace collide (`discovery.go:326-376`). Entries are not compared by location, so the same directory reached through a sibling scan and through `includes` fails today.
- An include alias is matched to a module by `filepath.Clean` of the path text (`discovery.go:446-460`).

## Goals / Non-Goals

**Goals:**

- One rule, in discovery, decides whether the current directory is a module directory. Its consumers (runtimes, env builder, dependency checks, collisions, vendored policy) do not change.
- The rule cannot fall back to a plain parse of a file in a module directory.

**Non-Goals:**

- Changing how the scan of a parent directory judges an entry, or how `invowk validate <path>` and `invowk audit` judge explicit targets.
- Recognising a directory as a module by its contents.
- Changing the discovery of directories other than the current directory itself.

## Decisions

### D1. Where the rule lives

A new step-1 helper in discovery (a new file, `internal/discovery/discovery_cwd_module.go`) runs before `discoverInDir`. It takes `absBase = filepath.Abs(baseDir)`.

The current directory is a module-directory candidate when either name below ends in `invowkmod.ModuleSuffix`:

- `filepath.Base(absBase)`;
- the base name of `filepath.EvalSymlinks(absBase)`.

For a candidate, discovery:

- does not call `discoverInDir`;
- skips step 2, because a module directory's children are part of that module: `Load` rejects nested modules, and vendored modules are reached through the module itself;
- passes the result through `appendModulesWithVendored`, so the vendored scan and its hash checks apply.

Alternatives considered:

- Patching each consumer to infer module identity from the path. Rejected: many places, and each could drift.
- Testing only the logical name. Rejected: a link without the suffix whose target is a module directory would still be plain-parsed.

### D2. One entry classifier for both directions

The loop body of `discoverModulesInDirWithDiagnostics` (`discovery_files.go:356-420`) becomes a helper, `loadModuleEntry(parentDir, entry fs.DirEntry)`, which returns the file or the diagnostics. The child scan and the current-directory helper both call it. The current-directory entry is `fs.FileInfoToDirEntry(os.Lstat(absBase))` with parent `filepath.Dir(absBase)`.

The helper therefore applies, in the same order and with the same codes:

- the symbolic-link skip;
- the `IsModule` gate;
- the reserved name;
- `Load`.

The new `Load` caller stays gated by `IsModule`. The `ModulePathContainment` model assumes this of every production `Load` caller (`formal/README.md:326-332`, fact `isModuleRejectsLinkedRoot`, `formal/alloy/ModulePathContainment.als:37`).

When the candidate yields neither a file nor a diagnostic (an entry that is not a directory, or a name prefix that `IsModule` rejects), the helper adds a `module_load_skipped` diagnostic. From inside, the skip is never silent.

### D3. Links

Only the current directory's own entry counts. `os.Lstat` does not follow its last component, so a link in an ancestor directory, such as `/var` to `/private/var` on macOS, does not change the result.

When the logical base name lacks the suffix but the physical one has it, the entry itself resolves to a module directory. It is skipped with `module_symlink_skipped`, as the parent scan skips a linked module entry.

On Windows, a junction is judged by the same classifier. The paired test records what that classifier does on `windows-latest`, and both directions agree by construction.

If `EvalSymlinks` fails, the candidate test uses the logical name only, and the failure is logged at debug level. Failing closed here would drop every current-directory invowkfile on volumes where `EvalSymlinks` is known to fail on Windows.

### D4. Failure never falls back

A candidate that does not load contributes no file. `discoverInDir` is not called on that path at all, so no code path can parse a file in a module directory as a plain invowkfile.

### D5. Deduplicate by physical directory

`appendModulesWithVendored` keeps, for one discovery run, the `os.Stat` result of every top-level module directory it has appended (entries with `ParentModule == nil`). When a new entry is `os.SameFile` as a kept one, discovery:

- drops the new entry and does not scan its vendored modules;
- sets the kept entry's `IsGlobalModule` when the new entry is global, and sets it on the kept entry's vendored children too, so ScopeConstruction's `globalChildren` keeps holding;
- when the kept entry has no explicit namespace (`CommandNamespace`, or an include alias found by `getAliasForModulePath`), and the dropped entry has one, sets the dropped entry's namespace as the kept entry's `CommandNamespace`. When several dropped entries have one, the first in precedence order wins.

A directory that cannot be `Stat`ed is not deduplicated, which keeps today's collision check for it.

Alternatives considered:

- Comparing `EvalSymlinks` strings. Rejected: case-insensitive volumes spell one directory in several ways, and `os.SameFile` compares device and inode (file index on Windows).
- A post-pass over all files. Rejected: it would scan, hash-verify and report the vendored modules of duplicates before dropping them.

The precedence order and the collision check are otherwise unchanged. Two different directories with the same module id still collide (`discovery.go:340-351`).

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
2. A module directory is recognised by its `.invowkmod` suffix only. A plain project root carries an `invowkmod.cue` for `requires`: `module add` and `module sync` edit `./invowkmod.cue` (`cmd/invowk/module_deps.go:172,211,300`). "Contains `invowkmod.cue`" would misclassify every project with dependencies.
3. Links: judge only the module directory's own entry, with `Lstat`. An ancestor link never counts.
4. Fail closed: a module that does not load is skipped with `module_load_skipped`, never plain-parsed.
5. Keep bare command names from inside the module, as long as no second plain-parse path is needed. None is (see Context).
6. Deduplicate the same module by physical directory in this change.
7. Every module behaviour gets a test pair: one run from the parent directory, one from inside. Add the macOS ancestor-link case and the Windows junction case.

## Risks / Trade-offs

- [Users who run a module's commands from inside it as `@invowkfile <cmd>`, or bare when the name is ambiguous] → **BREAKING**, stated in the proposal's migration notes. Bare names keep working when they are unique.
- [A module that loads from nowhere today (symlink, nested module, id mismatch) stops working from inside] → Intended. `invowk validate` names the problem.
- [Deduplication hides a misconfiguration, such as an include that names the current module] → The module is the same directory, so there is nothing to disambiguate. Different directories with the same id still fail.
- [`EvalSymlinks` failure fallback] → Judged by the logical name. The only case missed is a link without the suffix on such a volume. That is no worse than the parent scan, which never loads a link.
- [README conflicts with the open PR that also edits README] → Edit only the discovery sections, and rebase before the gates.
