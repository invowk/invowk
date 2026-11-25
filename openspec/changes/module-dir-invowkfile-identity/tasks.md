# Tasks

Public text uses hardening wording only, for example "the invowkfile of a module directory is always loaded as part of that module". Public text is everything that leaves this machine: the `openspec/` artifacts of this change, code comments, tests, docs, commit messages and the PR. It names no finding id, does not describe how the code behaved before this change, and never describes how to get that behaviour. Negative-control results go into the stage result for the maintainer session, never into repository text.

Every locked command (`make lint`, `make test`, `make test-scripts`, `make check-baseline`, `make formal`, mutation targets and `git commit`) runs as `flock "$(git -C <worktree> rev-parse --git-common-dir)/invowk-heavy.lock" <cmd>`, in the background, with a `head=` and an `rc=` line appended to its log, and is waited for with the Monitor tool (`until grep -q '^rc=' <log>; do sleep 5; done`).

## 1. Shared module-entry classifier (design D2)

- [x] 1.1 Extract the per-entry body of `discoverModulesInDirWithDiagnostics` (`internal/discovery/discovery_files.go:356-420`) into `loadModuleEntry(parentDir, entry fs.DirEntry)`, keeping the order and the diagnostic codes: symbolic-link skip, `IsModule` gate, reserved name, `Load`. Keep its semantic comments. No behaviour change: `go test ./internal/discovery/` passes unchanged.

## 2. The current directory as a module directory (design D1, D2, D3, D4)

- [x] 2.1 Add `internal/discovery/discovery_cwd_module.go`, following design D1 step by step:
  - `Lstat` of `absBase`; the stored entry from the parent's `ReadDir`, matched with `os.SameFile` (names equal ignoring case first, every entry only when none matches);
  - the candidate test: the logical, stored, `EvalSymlinks` and (for a link entry) resolved-target base names, suffix compared ignoring case;
  - a link entry (not a directory, `ModeSymlink` or `ModeIrregular`) resolved through `os.Readlink` and `EvalSymlinks`; a candidate link gets `module_symlink_skipped`;
  - a candidate directory classified by `loadModuleEntry(filepath.Dir(absBase), storedEntry)`;
  - every fail-closed case and code in the design D2 table.

  Wire it into step 1 of `discoverAllWithDiagnostics`: for a candidate, call neither `discoverInDir` nor step 2, and route the result through `appendModulesWithVendored`. Update the `DiscoverAll` doc comment (`discovery_files.go:87-95`): step 1 is "the current directory: its `invowkfile.cue`, or the module it is", and step 2 does not run for a module directory. Verify with `go build ./...`.
- [x] 2.2 Unit tests in a new `internal/discovery/discovery_cwd_module_test.go`, each a pair: `WithBaseDir(parent)` versus `WithBaseDir(parent/x.invowkmod)`, asserting the same outcome. Cover:
  - module identity attached: `Module` set; the parsed invowkfile's `ModulePath` is the module directory; `Metadata` is set; `IsFromModule()` is true;
  - commands under the module source, not `invowkfile`;
  - `Load` failures: an inner symbolic link, a nested module (and it is not loaded as a sibling), an id mismatch, a missing `invowkmod.cue`, each giving `module_load_skipped`;
  - an invalid name prefix (`1bad.invowkmod`) giving `module_load_skipped` from inside;
  - a stored suffix in another case (`Tools.INVOWKMOD`, created on disk) giving `module_load_skipped` and no plain parse;
  - reserved name `invowkfile.invowkmod` (`reserved_module_name_skipped`);
  - an extensionless `invowkfile` is not read;
  - a library-only module;
  - a symbolic-link module directory entry (`module_symlink_skipped`);
  - a symbolic link without the suffix to a module directory (`module_symlink_skipped`);
  - undeclared vendored module skipped (`vendored_undeclared_skipped`), and a declared and locked one loaded;
  - `WithBaseDir("")` loads nothing from it;
  - a plain directory with `invowkfile.cue` and `invowkmod.cue` stays source `invowkfile`;
  - the precedence position: the current-directory module is listed before an included module.

  Use `fstree.CapSymlink` for links. Check: `go test -run 'TestDiscovery_CurrentDirModule' ./internal/discovery/`.
- [ ] 2.3 Named platform tests in the same file:
  - `TestDiscovery_CurrentDirModule_AncestorLinkStillModule`: the base path runs through a linked ancestor directory, and the module still loads. On macOS, `t.TempDir()` under `/var` is also covered.
  - `TestDiscovery_CurrentDirModule_CaseVariantCwd`: the module is stored as `tools.invowkmod` and `WithBaseDir` names `TOOLS.INVOWKMOD`. On a case-insensitive volume (detected by `Lstat` of the variant succeeding: macOS APFS, Windows NTFS) the module loads under its stored name with the parent scan's result. On a case-sensitive volume the variant does not exist and the test asserts that nothing is loaded and a diagnostic is reported.
  - `TestDiscovery_CurrentDirModule_JunctionModuleDir`: `fstree.CapJunction`, so it is required on Windows and skipped elsewhere. A junction `j.invowkmod` to a valid module loads nothing in either direction; from inside, `module_symlink_skipped`.
  - `TestDiscovery_CurrentDirModule_JunctionWithoutSuffix`: Windows; a junction `tools` to `tools.invowkmod` as the base gives `module_symlink_skipped` and no plain parse.
  - `TestDiscovery_CurrentDirModule_JunctionChainWithoutSuffix`: Windows; a junction `tools` to a junction `mid` to `tools.invowkmod` as the base gives `module_symlink_skipped` and no plain parse.
  - `TestDiscovery_CurrentDirModule_ShortNameCwd`: Windows; the base is the 8.3 short name of `tools.invowkmod` and the module loads under its stored name. Skipped, with a log line, when the volume has no short names.

  All run in short mode, so the CI jobs `Test (windows)` and `Test (macos)` run them. Name those jobs, their run ids and each test's pass or skip in the stage result. Check: `go test -run 'TestDiscovery_CurrentDirModule_(AncestorLink|CaseVariant|Junction|ShortName)' ./internal/discovery/`.

## 3. One module directory, one module (design D5)

- [x] 3.1 In `appendModulesWithVendored`, keep the `os.Stat` result of each appended top-level module directory for the discovery run. Drop an `os.SameFile` duplicate before its vendored scan. Merge `IsGlobalModule` into the kept entry and its vendored children. Adopt the first explicit namespace of a dropped duplicate when the kept entry has none. Do not deduplicate a directory that cannot be `Stat`ed. Verify with `go build ./...`.
- [x] 3.2 Unit tests in `internal/discovery/discovery_dedupe_test.go`:
  - current module plus the same path in `includes`;
  - current module entered in another letter case plus the same path in `includes` (case-insensitive volumes; skipped elsewhere);
  - sibling module plus the same path in `includes`;
  - the same module through an include path spelled through a link;
  - include alias adopted from inside;
  - current module inside the commands directory: global, its vendored children global, no `module_shadows_global` warning;
  - two different directories with one module id still raise `ModuleCollisionError`.

  Check: `go test -run 'TestDiscovery_ModuleDedupe' ./internal/discovery/`.

## 4. Authoring verification (design D6)

- [x] 4.1 `verifyGeneratedCommand` (`cmd/invowk/agent.go:715`) takes `FromSource` from the discovered command whose `FilePath` is the written target and whose `SimpleName` is the written name, and falls back to `invowkfile`. Add a test with the existing fake LLM fixture that runs `agent cmd create --verify` inside a module directory and one in a plain directory. Check: `go test -run 'Verify' ./cmd/invowk/`.

## 5. CLI behaviour pairs (specs: module-directory-identity, cwd-discovery-scope, agent-authoring-crud)

- [x] 5.1 Add `tests/cli/testdata/module_dir_identity.txtar`. Its commands use the native runtime with a platform split (`sh` on linux and macos, PowerShell on windows; invowk-schema skill), so it needs no mirror; steps that create symbolic links run under `[!windows]`. The CLI test step runs it on ubuntu and, in short mode, in `Test (windows)` and `Test (macos)`. Each step runs once in the module's parent directory and once inside the module directory, and asserts the same result. Cover:
  - listing under `<module>.invowkmod`;
  - a unique bare name;
  - `<module> <cmd>` and `@<module> <cmd>`;
  - an ambiguous bare name against an included module (exit 2, both sources named);
  - `@invowkfile` not found inside (exit 2);
  - the scope gate for `depends_on.cmds` on an unrequired module (the command does not run);
  - `script.file` runs;
  - an inner symbolic link (`[!windows]`): workspace `invowk validate` gives `module_load_skipped` and exit 1, and running the command that reads the linked env file fails without printing the linked content;
  - a nested module: workspace `invowk validate` gives `module_load_skipped` and exit 1, the listing has neither module, and `invowk cmd <nested> <cmd>` is not found;
  - an id mismatch: workspace `invowk validate` gives `module_load_skipped` and exit 1, and the listing has no command from it;
  - an undeclared vendored module: workspace `invowk validate` reports `vendored_undeclared_skipped`, and the command that depends on it fails with missing commands;
  - a collision with a different global module of the same id;
  - the current module also in `includes`, listed once with exit 0;
  - `invowk module list`;
  - `invowk audit --format json`: the module is scanned as a module in both directions;
  - `--ivk-no-cwd` inside the module (not found);
  - a symbolic-link module directory as the current directory (`[!windows]`, `module_symlink_skipped`);
  - a plain directory with `invowkfile.cue` and `invowkmod.cue` (control: source `invowkfile`).

  Check: `go test -run 'TestCLI/module_dir_identity' ./tests/cli/`.
- [x] 5.2 Add an inside-the-module leg to `tests/cli/testdata/virtual_module_workdir_roots.txtar`, for both the virtual-sh read and the virtual-lua read: a module-declared workdir outside the module is denied when invowk runs in the module directory too. Check: `go test -run 'TestCLI/virtual_module_workdir_roots' ./tests/cli/`.
- [x] 5.3 Negative control: run 5.1 and 5.2 against a build of the base in a throwaway worktree under this stage's scratch dir, investigate any leg that deviates from the expectation, report the result in the stage result only, then remove the throwaway worktree.

## 6. Docs (checklist step 9; sync map in `.agents/skills/docs/SKILL.md`)

- [x] 6.1 Update the discovery sections of `README.md` (precedence lists near the "Current directory `invowkfile.cue`" lines) with hardening wording: the invowkfile of a module directory is always loaded as part of that module; a directory whose name ends in `.invowkmod` is only ever loaded as a module; the same module directory found through several sources is one module. Touch no README hunk that the open module-CLI PR changes (`git diff origin/main` before and after the rebase). Check: `make check-agent-docs` stays green and the README diff touches only discovery text.
- [x] 6.2 Update `website/docs/architecture/discovery.mdx`, `website/docs/core-concepts/commands-and-namespaces.mdx` and `website/docs/modules/overview.mdx`, with the pt-BR copies under `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`. Cover: namespace in a module directory, bare names when unique, precedence of the current-directory module, deduplication, `--ivk-no-cwd`. Do not edit `modules/dependencies/cli-commands.mdx`. Re-check `docs/architecture/` for discovery flow text. Check: `cd website && npm run build`.

## 7. Formal and completion gates (`.agents/rules/checklist.md`)

- [x] 7.1 Run `python3 scripts/formal_mutation.py plan --out <scratch>` and `symbols --plan <scratch>/formal-plan.json` on every touched function. On the base, only `IsModule` is planned, and it is not edited. A newly planned touched symbol gets a `make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=…` plan in the stage result. Reword the ScopeConstruction `RootSrc` row note (design D7) if the check accepts it. Check: `python3 scripts/formal.py correspondence` and `make formal-reach`; `make formal` (locked, backgrounded) when Java is available.
  - Done: only `IsModule` is planned, and it is not edited, so no refresh is needed. The `RootSrc` row note stays as it is: it still holds (a current-directory invowkfile now exists only outside a module directory), and editing `ScopeConstruction.als` would make its golden vectors stale for a comment-only change.
- [ ] 7.2 Run checklist steps 2 to 10. Every locked command among them runs under the heavy lock in the background with `head=` and `rc=` lines and a Monitor wait (preamble):
  - `make tidy`, plus `make license-check` for the new Go files;
  - `make lint`;
  - `make test`;
  - `make check-baseline`, triaging every new goplint finding;
  - `make check-file-length`, with `discovery_files.go` under 1000 lines;
  - `make sonar-local`;
  - `make test-scripts` if `formal/` changed;
  - `make check-agent-docs`;
  - `go run . validate` for each `samples/invowkmods/*.invowkmod`.

  Check: every gate log ends with `rc=0` at the final HEAD.
- [ ] 7.3 Run `/learn` (`.agents/skills/learn/`) last.
