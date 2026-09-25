## Why

Module path containment ("every path a module makes Invowk read, execute, or copy stays inside that module's root") is Invowk's first defence against supply-chain attacks (SC-01, SC-05). No single function enforces it. It depends on several pieces working together:
- lexical checks (`ScriptFilePath.Validate`, `validateScriptPathContainment`, `ValidateEnvFilePath`, `ValidateContainerfilePath`);
- a discovery fact made of two parts: `invowkmod.IsModule` rejects a symlinked module root, and `invowkmod.Load` rejects any module whose tree contains a symlink, except under `invowk_modules/`;
- copy and hash helpers that skip symlinks;
- a separate virtual-runtime path harness.

Each layer has its own tests, including symlink cases, but no test checks their composition, and the virtual harness has no symlink tests. The symlink-aware validator `Module.ValidateScriptPath` has no production caller. Reading the code suggests at least two escapes that pass every existing test. Invowk's formal-verification programme already binds relational Alloy models to real code with golden vectors. This guarantee is the next one to cover, before a fix or refactor silently removes one of those pieces.

## What Changes

This change depends on `formal-infra-refinements`, which lands first. It is written against that change's machinery:
- Alloy commands are generated from `formal/manifest.toml` (`body`, `scope`, and a default model `scope`), and `.als` sources declare no commands;
- golden files use format 2, with a fingerprint that covers the rendered golden command;
- the runner rejects unknown manifest keys.

- Add an Alloy 6 model, `formal/alloy/ModulePathContainment.als`, covering:
  - a filesystem tree of directories, regular files, symlinks and Windows junctions, including links inside the module, outside it, chained, dangling, and cyclic, plus a case-insensitivity mode;
  - module roots, including a symlinked root, and the vendored `invowk_modules/` subtree;
  - path references: relative, `..`, absolute, and Windows-dialect absolute.

  For each operation a module can trigger, the model records the implementation's decision layer by layer, and checks it against an independently written containment policy with an explicit allowed-location set. The operations are:
  - script-file read and execution;
  - custom-check script read;
  - env-file read;
  - containerfile reference;
  - workdir resolution;
  - vendored-module discovery;
  - vendor and provisioning copies;
  - content hashing;
  - ZIP unpack.
- Add a second Alloy model, `formal/alloy/VirtualPathHarness.als`, for the virtual runtime harness: allowed-root construction, `normalizeExistingOrParent`, and `pathWithin`, with symlinks created at run time. It transcribes the existing requirements in `virtual-runtime-sandbox` and `virtual-filesystem-access`.
- Bind both models to the real code:
  - golden vectors are replayed by materialising each instance as a real directory tree (a new `internal/testutil/fstree` helper) and calling the real functions;
  - rapid properties compare the real code with the Go transcription of the policy at larger scopes;
  - cost budgets are explicit and measured on Linux, macOS, and Windows.
- Add calibration:
  - an antecedent for every check;
  - witness runs for each situation the model claims to cover;
  - model mutants that drop load-bearing facts;
  - seeded defects in the real code, recorded in `formal/manifest.toml`.
- Record findings with fixed ids. F8–F12 are reserved by sibling changes; this change owns F13 onward:
  - **F13**: a `script.file` under `invowk_modules/` reaches a symlink that `Load` never scans;
  - **F14**: a virtual-harness `mkdir` through an escaping symlink is judged lexically;
  - **F15**: a module-declared `workdir` widens the virtual allowed roots under `restricted` access, recorded only if confirmed;
  - **F16 onward**: anything else.

  A suspected finding that does not reproduce is not recorded, and its id stays unused. Each recorded finding is a `finding = "F<n>"` manifest command expecting a counterexample, next to a passing fixed configuration, a Go replay test, and a `formal/README.md` row. Product fixes are out of scope.
- Wire the new bindings into `FORMAL_RAPID_PACKAGES` and the formal-verification workflow. Update `formal/README.md`, the formal-verification skill, and the stale supply-chain reviewer notes.

## Capabilities

### New Capabilities
- `module-path-containment-model`: the Alloy models, golden-vector and rapid bindings, calibration, and findings for module filesystem path containment, including the virtual-runtime path harness.

### Modified Capabilities
<!-- None. No delta is made to openspec/specs/virtual-runtime-sandbox ("Traversal out of an allowed root is blocked") or openspec/specs/virtual-filesystem-access ("Restricted access limits virtual file I/O"). The harness model transcribes those requirements as its policy, and F14 is recorded as a violation of the existing requirement, not as a requirement change. The shared toolchain requirements belong to adopt-formal-verification and formal-infra-refinements. -->

## Impact

- **New files:**
  - `formal/alloy/ModulePathContainment.als` and `formal/alloy/VirtualPathHarness.als`;
  - format-2 golden files `internal/app/moduleops/testdata/formal/module_path_containment_golden.json.gz` and `internal/runtime/testdata/formal/virtual_path_harness_golden.json.gz`;
  - Go tests `module_path_containment_{golden,rapid,formal}_test.go` in `internal/app/moduleops` and `virtual_path_harness_{golden,rapid,formal}_test.go` in `internal/runtime`;
  - a test-only `export_test.go` hook in `pkg/invowkfile`;
  - the `internal/testutil/fstree` helper.
- **Updated files:**
  - `formal/manifest.toml`: two new `[[model]]` entries with a default `scope`, commands with `body`, golden sections, and calibration records;
  - `Makefile` (`FORMAL_RAPID_PACKAGES`) and `.github/workflows/formal-verification.yml` (replay step `-run` pattern, packages, and `paths`), or promote-formal-ci-gate's `[ci] paths` if that change has landed;
  - `formal/README.md`, `.agents/skills/formal-verification/SKILL.md`, `.agents/agents/supply-chain-reviewer.md`, `.agents/skills/module-security/SKILL.md`, and `docs/next/cross-platform-windows-improvements.md`.
- **Code read and bound by tests, but not changed:**
  - `pkg/invowkfile` (`script_file_path.go`, `implementation.go`, `dependency.go`, `validation_filesystem.go`, `invowkfile.go`);
  - `pkg/invowkmod` (`operations.go`, `operations_validate.go`, `invowkmod.go`, `content_hash.go`);
  - `internal/app/modulecache/cache.go`, `internal/app/moduleops/{vendor,packaging}.go`, `internal/discovery/discovery_files.go`, `internal/provision/helpers.go`;
  - `internal/runtime/{dotenv,virtual_policy,container_provision,native}.go`;
  - `internal/audit/{scan_files,scan_context_artifacts}.go`, whose verdicts are logged as an advisory cross-check on each golden instance.
- **Test cost:** `make test` gains filesystem-backed replay tests, with a budget of 30 s or less per OS. Instances that need an unavailable capability (symlink privilege, junctions off Windows, case-folding) are skipped individually and counted.
- No user-facing behaviour, schema, or CLI change.
