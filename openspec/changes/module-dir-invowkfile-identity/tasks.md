# Tasks

Public text (code comments, tests, docs, commit messages, PR) uses hardening wording only, for example "an invowkfile inside a module directory is always loaded as part of that module". It names no finding id and never describes how to get the old behaviour. Negative-control results (a pair's inside leg run against the base 693f0e17) go into the stage result for the maintainer session, never into repository text.

## 1. Shared module-entry classifier (design D2)

- [ ] 1.1 Extract the per-entry body of `discoverModulesInDirWithDiagnostics` (`internal/discovery/discovery_files.go:356-420`) into `loadModuleEntry(parentDir, entry fs.DirEntry)`, keeping the order and the diagnostic codes: symbolic-link skip, `IsModule` gate, reserved name, `Load`. Keep its semantic comments. No behaviour change: `go test ./internal/discovery/` passes unchanged.

## 2. The current directory as a module directory (design D1, D3, D4)

- [ ] 2.1 Add `internal/discovery/discovery_cwd_module.go`. The candidate test covers the logical base name and the `EvalSymlinks` base name; if `EvalSymlinks` fails, use the logical name only and log at debug. The entry comes from `Lstat` and goes through `loadModuleEntry`. A logical-name-without-suffix link to a module directory gets `module_symlink_skipped`. A candidate that yields neither a file nor a diagnostic gets `module_load_skipped`. Wire it into step 1 of `discoverAllWithDiagnostics`: for a candidate, call neither `discoverInDir` nor step 2, and route the result through `appendModulesWithVendored`. Verify with `go build ./...`.
- [ ] 2.2 Unit tests in a new `internal/discovery/discovery_cwd_module_test.go`, each a pair: `WithBaseDir(parent)` versus `WithBaseDir(parent/x.invowkmod)`, asserting the same outcome. Cover:
  - module identity attached: `Module` set; the parsed invowkfile's `ModulePath` is the module directory; `Metadata` is set; `IsFromModule()` is true;
  - commands under the module source, not `invowkfile`;
  - `Load` failures: an inner symbolic link, a nested module (and it is not loaded as a sibling), an id mismatch, each giving `module_load_skipped`;
  - reserved name `invowkfile.invowkmod` (`reserved_module_name_skipped`);
  - an extensionless `invowkfile` is not read;
  - a library-only module;
  - a symbolic-link module directory entry (`module_symlink_skipped`);
  - a link without the suffix to a module directory (skipped with a diagnostic);
  - undeclared vendored module skipped (`vendored_undeclared_skipped`), and a declared and locked one loaded;
  - `WithBaseDir("")` loads nothing from it;
  - a plain directory with `invowkfile.cue` and `invowkmod.cue` stays source `invowkfile`.

  Use `fstree.CapSymlink` for links. Check: `go test -run 'TestDiscovery_CurrentDirModule' ./internal/discovery/`.
- [ ] 2.3 Named platform tests in the same file:
  - `TestDiscovery_CurrentDirModule_AncestorLinkStillModule`: the base path runs through a linked ancestor directory, and the module still loads. On macOS, `t.TempDir()` under `/var` is also covered.
  - `TestDiscovery_CurrentDirModule_JunctionMatchesParentScan`: `fstree.CapJunction`, so it is required on Windows and skipped elsewhere. It asserts that the parent scan and the current-directory rule agree on a junction module directory, and logs which outcome it saw.

  Both run in short mode, so the CI jobs `Test (windows)` and `Test (macos)` run them. Name those jobs and their run ids in the stage result. Check: `go test -run 'TestDiscovery_CurrentDirModule_(AncestorLink|Junction)' ./internal/discovery/`.

## 3. One module directory, one module (design D5)

- [ ] 3.1 In `appendModulesWithVendored`, keep the `os.Stat` result of each appended top-level module directory for the discovery run. Drop an `os.SameFile` duplicate before its vendored scan. Merge `IsGlobalModule` into the kept entry and its vendored children. Adopt the first explicit namespace of a dropped duplicate when the kept entry has none. Do not deduplicate a directory that cannot be `Stat`ed. Verify with `go build ./...`.
- [ ] 3.2 Unit tests in `internal/discovery/discovery_dedupe_test.go`:
  - current module plus the same path in `includes`;
  - sibling module plus the same path in `includes`;
  - the same module through an include path spelled through a link;
  - include alias adopted from inside;
  - current module inside the commands directory: global, its vendored children global, no `module_shadows_global` warning;
  - two different directories with one module id still raise `ModuleCollisionError`.

  Check: `go test -run 'TestDiscovery_ModuleDedupe' ./internal/discovery/`.

## 4. Authoring verification (design D6)

- [ ] 4.1 `verifyGeneratedCommand` (`cmd/invowk/agent.go:715`) takes `FromSource` from the discovered command whose `FilePath` is the written target and whose `SimpleName` is the written name, and falls back to `invowkfile`. Add a test with the existing fake LLM fixture that runs `agent cmd create --verify` inside a module directory and one in a plain directory. Check: `go test -run 'Verify' ./cmd/invowk/`.

## 5. CLI behaviour pairs (specs: module-directory-identity, cwd-discovery-scope)

- [ ] 5.1 Add `tests/cli/testdata/module_dir_identity.txtar` (native runtime only, so it needs no mirror). Each step runs once in the module's parent directory and once inside the module directory, and asserts the same result. Cover:
  - listing under `<module>.invowkmod`;
  - a unique bare name;
  - `<module> <cmd>` and `@<module> <cmd>`;
  - an ambiguous bare name against an included module;
  - `@invowkfile` not found inside;
  - the scope gate for `depends_on.cmds` on an unrequired module;
  - `script.file` runs;
  - workspace `invowk validate` with an inner symbolic link, a nested module and an id mismatch (`module_load_skipped`, exit 1);
  - an undeclared vendored module (`vendored_undeclared_skipped`);
  - a collision with a different global module of the same id;
  - the current module also in `includes`, listed once with exit 0;
  - `invowk module list`;
  - `--ivk-no-cwd` inside the module (not found);
  - a symbolic-link module directory as the current directory (`[!windows]`).

  Check: `go test -run 'TestCLI/module_dir_identity' ./tests/cli/`.
- [ ] 5.2 Add an inside-the-module leg to `tests/cli/testdata/virtual_module_workdir_roots.txtar`, for both the virtual-sh read and the virtual-lua read: a module-declared workdir outside the module is denied when invowk runs in the module directory too. Check: `go test -run 'TestCLI/virtual_module_workdir_roots' ./tests/cli/`.
- [ ] 5.3 Negative control: build origin/main 693f0e17 in a throwaway worktree under this stage's scratch dir. Run 5.1 and 5.2 against that binary and record which inside legs fail there. Every behavioural pair must fail on the base; the bare-name and plain-directory controls must pass there. Report the result in the stage result only, then remove the throwaway worktree.

## 6. Docs (checklist step 9; sync map in `.agents/skills/docs/SKILL.md`)

- [ ] 6.1 Update the discovery sections of `README.md` (precedence lists near the "Current directory `invowkfile.cue`" lines) with hardening wording: an invowkfile inside a module directory is always loaded as part of that module; the same module directory found through several sources is one module. Touch no README hunk that the open module-CLI PR changes (`git diff origin/main` before and after the rebase). Check: `make check-agent-docs` stays green and the README diff touches only discovery text.
- [ ] 6.2 Update `website/docs/architecture/discovery.mdx`, `website/docs/core-concepts/commands-and-namespaces.mdx` and `website/docs/modules/overview.mdx`, with the pt-BR copies under `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`. Cover: namespace from inside a module, bare names when unique, deduplication, `--ivk-no-cwd`. Do not edit `modules/dependencies/cli-commands.mdx`. Re-check `docs/architecture/` for discovery flow text. Check: `cd website && npm run build`.

## 7. Formal and completion gates (`.agents/rules/checklist.md`)

- [ ] 7.1 Run `python3 scripts/formal_mutation.py plan --out <scratch>` and `symbols --plan <scratch>/formal-plan.json` on every touched function. On the base, only `IsModule` is planned, and it is not edited. A newly planned touched symbol gets a `make mutation-formal-refresh MUTATION_FORMAL_SYMBOL=…` plan in the stage result. Reword the ScopeConstruction `RootSrc` row note (design D7) if the check accepts it. Check: `python3 scripts/formal.py correspondence` and `make formal-reach`; `make formal` when Java is available.
- [ ] 7.2 Run checklist steps 2 to 10 under the heavy lock, steps 4 to 7 as background commands with an `rc=` line:
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
