## ADDED Requirements

### Requirement: Filesystem and path-reference model
Invowk SHALL maintain an Alloy 6 model, `formal/alloy/ModulePathContainment.als`, of a bounded filesystem tree and of the path references a module can declare. The tree SHALL contain:
- directories, regular files, symlinks, and Windows junctions;
- a host root, one or more module roots (`*.invowkmod`), including a module root that is itself a symlink;
- a `invowk_modules/` directory with vendored child modules;
- directories outside every module.

Each link SHALL carry a target reference, and the model SHALL cover each of these link kinds:
- a link to an in-module node;
- a link to an out-of-module node;
- a chain of two links;
- a dangling link;
- a two-link cycle;
- a link through a symlinked ancestor directory.

Link following SHALL be bounded at chain depth 2. A fact SHALL forbid acyclic chains longer than 2 in every instance, so that no instance depends on a chain-length limit. A cycle SHALL resolve to an error, as it does on the operating system.

A reference SHALL be a bounded sequence of name and `..` steps, relative to a base directory or absolute. Absolute references SHALL cover the Unix-rooted, Windows-drive, Windows-rooted, and UNC dialects that `isAbsolutePath` recognises. The model SHALL define two resolutions:
- lexical resolution, which applies `..` textually as `filepath.Clean` does;
- physical resolution, which follows links the way the operating system does.

A filesystem mode flag SHALL make name lookup case-insensitive by folding names into equivalence classes. A fact SHALL forbid two siblings in the same fold class while the flag is set.

#### Scenario: Lexical and physical resolution diverge
- **WHEN** a reference crosses an in-module symlink whose target lies outside the module
- **THEN** the model SHALL give the reference a lexical result inside the module and a physical result outside it, and a witness run SHALL show that this configuration is reachable

#### Scenario: Dangling, chained, and cyclic links
- **WHEN** a reference ends at a dangling symlink, at a chain of two symlinks, or at a two-link cycle
- **THEN** physical resolution SHALL be undefined for the dangling link, SHALL follow both hops of the chain, and SHALL be an error for the cycle, and a witness run SHALL show each case

#### Scenario: Case-insensitive aliasing
- **WHEN** the filesystem is case-insensitive and a reference names a node with a different case
- **THEN** the lookup SHALL reach that node, while a transcribed exact-name comparison in the code SHALL still treat the two names as different

### Requirement: Independent containment policy
The model SHALL state the containment policy separately from the transcription of the implementation. For every bound operation, the policy SHALL require that each node the operation physically reads, executes, copies, hashes, or writes lies inside the module's physical root, or is admitted by a named allowed-location predicate. Each allowed-location predicate SHALL carry a stated justification, and the set SHALL contain:
- `allowedWorkspace`: the invowkfile directory mounted as the container workspace;
- `allowedVendoredSelf`: a vendored child's own root, for the child's own operations;
- `allowedVirtualRoot`: the virtual runtime's standard anchors, temporary roots, and declared `filesystem.paths`;
- `allowedWorkdirCwd`: a module-declared `workdir` as the execution working directory only, never as a read, copy, hash, or virtual allowed-root target.

A `script.file` into `invowk_modules/` SHALL be allowed only when it resolves physically inside the declaring module's root.

#### Scenario: Policy is not a restatement of the code
- **WHEN** the policy predicate is compared with the implementation predicates
- **THEN** the policy SHALL be written in terms of physical resolution, root membership, and the allowed-location predicates only, without any lexical-check or discovery-fact predicate from the implementation

#### Scenario: Allowed locations are explicit
- **WHEN** an accepted operation touches a node outside the module root
- **THEN** the check SHALL pass only if a named allowed-location predicate admits that node for that operation kind, and each predicate SHALL appear in the model's correspondence table with its justification

### Requirement: Transcription of the enforcement points
The model SHALL transcribe the decisions of the real enforcement points as separate predicates, one per layer. Discovery guarantees SHALL be named facts, so that a mutant can drop them. The transcription SHALL cover the following.

Reference checks and reads:
- `ScriptFilePath.Validate` and `ScriptFilePath.ResolveFromModule`;
- `validateScriptPathContainment`, as called from `Implementation.ResolveScriptWithFSAndModule` and `CustomCheckScript.ResolveWithFSAndModule`;
- the interpreter's use of the `SelectedScriptFilePath()` path after the read;
- `ValidateEnvFilePath` followed by `runtime.LoadEnvFile`;
- `ValidateContainerfilePath`;
- `Invowkfile.GetEffectiveWorkDir` and `ContainerRuntime.getContainerWorkDir`.

Discovery facts, as two separate named facts:
- `isModuleRejectsLinkedRoot`: the Lstat check in `invowkmod.IsModule` at discovery call sites;
- `loadScansNonLinkedRoot`: the tree scan in `invowkmod.Load` (`scanModuleTree`, `inspectModuleEntry`), including its exact-name skip of `invowk_modules`, which scans only a root that is not a symlink.

Discovery, copies, hashing, and unpack:
- the symlink skip in `discoverVendoredModulesWithDiagnostics`;
- the symlink-skipping copies `modulecache.copyDir` and `provision.CopyDir`, including their following of a symlinked source root;
- `computeModuleHash`;
- `normalizeZIPPath` and `validateDestinationPath` in ZIP unpack.

The model SHALL NOT treat `Module.ValidateScriptPath` or `Module.checkSymlinkSafety` as enforcement. The correspondence table SHALL state that they have no production caller. Wiring or deleting them belongs to a fix change.

#### Scenario: Linked-root fact is load-bearing
- **WHEN** a mutant drops `isModuleRejectsLinkedRoot`
- **THEN** a check SHALL produce a counterexample in which a symlinked module root is admitted by `Load` without its tree being scanned, and its script read leaves the root

#### Scenario: Load scan is load-bearing
- **WHEN** a mutant drops `loadScansNonLinkedRoot`
- **THEN** the script-read check SHALL produce a counterexample in which a lexically contained script path reads a file outside the module through an in-module symlink

#### Scenario: Copy and hash never follow inner links
- **WHEN** Alloy checks vendor copies, provisioning copies, and content hashing of roots that are not symlinks
- **THEN** no copied or hashed node SHALL lie outside the source module's physical root, and a mutant that follows inner symlinks in the copy SHALL produce a counterexample

#### Scenario: Unpack writes stay inside the destination
- **WHEN** a ZIP member name contains `..`, a leading slash, or a backslash traversal
- **THEN** the transcribed unpack SHALL reject it, and a mutant that drops `validateDestinationPath` SHALL produce a counterexample

### Requirement: Guarded safety checks
Each safety check SHALL be a `[[model.command]]` in `formal/manifest.toml` with a `body`, an optional `scope` (defaulting to the model's `scope`), and `expect = "pass"`. The runner renders the Alloy command, and the `.als` source SHALL declare no `run` or `check` commands. Each safety check SHALL also have:
- a satisfiable antecedent command;
- at least one mutant command with `mutant_of`, checking the same parameterised predicate.

Each mutant SHALL mirror a plausible regression, not the current code. The mutants SHALL include:
- the containment check removed from `ResolveScriptWithFSAndModule`;
- `ScriptFilePath.Validate` without backslash normalisation;
- `isAbsolutePath` without the Windows-drive dialect;
- a copy that follows inner symlinks;
- `inspectModuleEntry` skipping any directory whose name case-folds to `invowk_modules`;
- the vendored-discovery scan not skipping symlinks;
- `isModuleRejectsLinkedRoot` dropped;
- `loadScansNonLinkedRoot` dropped.

#### Scenario: Every check is non-vacuous
- **WHEN** `make formal-alloy` runs
- **THEN** every antecedent and witness command SHALL be satisfiable, every mutant SHALL produce a counterexample, and the runner SHALL fail closed otherwise

#### Scenario: Model source declares no commands
- **WHEN** a `.als` file of this change contains a `run` or `check` command
- **THEN** the runner SHALL fail the model

### Requirement: Virtual-runtime path harness model
Invowk SHALL maintain a second Alloy model, `formal/alloy/VirtualPathHarness.als`, of the virtual runtime's path policy. The model SHALL cover:
- the allowed-root list built by `newVirtualPathResolverForFilesystem` from the effective workdir, the script base path, `standardVirtualAnchorsForOS` anchors, the temporary roots, and the declared paths;
- normalisation by `normalizeExistingOrParent`: full `EvalSymlinks`, then parent-only `EvalSymlinks`, then a lexical fallback;
- the root comparison in `pathWithin`;
- the `restricted` and `full` access modes;
- symlinks created during execution by the u-root `ln` built-in, whose targets are stored literally.

The policy SHALL transcribe the existing requirements "Traversal out of an allowed root is blocked" (`openspec/specs/virtual-runtime-sandbox`) and "Restricted access limits virtual file I/O" (`openspec/specs/virtual-filesystem-access`). Under `restricted` access, every accepted access SHALL resolve physically inside an allowed root. `full` access is an explicit opt-out. Whether a module-declared `workdir` outside the module may enter the allowed-root list SHALL be checked against `allowedWorkdirCwd`, which admits the workdir as a working directory and not as an allowed root.

#### Scenario: Existing paths are compared like with like
- **WHEN** a path exists and crosses a symlink
- **THEN** the harness SHALL compare the evaluated path with the evaluated roots, and a mutant that compares the unevaluated path SHALL produce a counterexample

#### Scenario: Deep non-existent path through an escaping link
- **WHEN** an allowed root contains a symlink to a writable directory outside every root, and a script requests a path with two or more missing components below that symlink
- **THEN** the current-code harness predicate SHALL accept the path, the finding command F14 SHALL produce a counterexample, and the fixed configuration that evaluates the deepest existing ancestor SHALL pass, provided task 1.3 confirms the behaviour in the real code; otherwise F14 SHALL stay unused and the change SHALL record the refutation

#### Scenario: Workdir widening
- **WHEN** a module-declared `workdir` outside the module is added to the allowed-root list under `restricted` access
- **THEN** a check against `allowedWorkdirCwd` SHALL produce a counterexample, recorded as finding F15, if a Go replay confirms that the real resolver accepts an access below that workdir

### Requirement: Golden-vector binding to real filesystem behaviour
Each model SHALL have a golden command in the manifest. It SHALL record the implementation's decision for every bound operation, per layer, in fields of a `one sig`: the syntactic validation, the containment check, the final physical read, copy, or hash, and the touched node. A `noJunk` predicate SHALL exclude atoms that cannot influence a decision.

Golden files SHALL be generated in format 2. Editing the golden command's `body` or `scope` SHALL change the fingerprint. The files SHALL be loaded with `alloygolden.Load`.

The replay tests SHALL do the following:
- materialise each instance with `internal/testutil/fstree` under `filepath.EvalSymlinks(t.TempDir())`;
- call the Go entry point bound to each operation in design.md D5, and compare every recorded layer decision and touched node;
- assert the Go transcriptions of the model's facts and policy on every instance.

An operation with no Go entry point SHALL be marked `-` in the binding column of the correspondence table. It SHALL be excluded from the replay, and the spec's "every bound operation" does not include it.

#### Scenario: Replay detects divergence
- **WHEN** a real function's layer decision or touched node differs from the model's golden instance
- **THEN** the replay test SHALL fail, naming the instance index, the operation, and the layer

#### Scenario: Replay places nodes outside every root
- **WHEN** the harness golden instances are replayed
- **THEN** at least one replayed instance SHALL contain a node outside every allowed root the replay constructs, and the test SHALL fail if none does

#### Scenario: Platform gating is explicit
- **WHEN** an instance needs a capability the host lacks (symlink creation, junctions off Windows, or a case-insensitive temporary filesystem)
- **THEN** the test SHALL skip only that instance, report the skip count per capability, and still assert the Go transcriptions on it

### Requirement: Rapid properties at larger scopes
Invowk SHALL add `pgregory.net/rapid` properties, `TestModulePathContainment_Property` and `TestVirtualPathHarness_Property`, in `*_rapid_test.go` files. Each property SHALL generate module trees, link placements, and references beyond the golden scope, materialise them, and compare the real decisions with the Go transcription of the policy. The properties SHALL follow the repository's rapid conventions. Their packages SHALL be added to `FORMAL_RAPID_PACKAGES`.

#### Scenario: Larger scope property
- **WHEN** `make formal-rapid-deep` runs the path-containment properties with `RAPID_CHECKS=10000`
- **THEN** no generated case SHALL show a policy violation other than those recorded as findings, and the run SHALL stay within the budget recorded in design.md

### Requirement: Cost budgets
The change SHALL record measured cost budgets:
- the golden instance count and decoded size of each model, measured with a capped `-r` before the scope is fixed;
- the golden replay time in `make test`, 30 s or less per OS on Linux, macOS, and Windows;
- the deep rapid time, 60 s or less.

Budgets SHALL be measured on all three operating systems. They SHALL be handed to `promote-formal-ci-gate` as `budget_seconds` values.

#### Scenario: Budget overrun blocks the scope
- **WHEN** a measured replay or deep-rapid time exceeds its budget on any operating system
- **THEN** the golden scope or the rapid generator SHALL be narrowed, with the reason recorded, before the model is listed in `formal/README.md`

### Requirement: Calibration record
Each model's `calibration` entry in `formal/manifest.toml` SHALL list the seeded real-code defects its bindings detect. Defects SHALL be seeded one at a time. They SHALL include:
- the containment check removed from `ResolveScriptWithFSAndModule`;
- `validateScriptPathContainment` accepting a `..` prefix, detected through a direct binding via a test-only `export_test.go` hook, because `Validate` masks it at every exported entry point;
- `ScriptFilePath.Validate` skipping backslash normalisation, detected by the Validate-layer decision;
- `modulecache.copyDir` following symlinks;
- `provision.CopyDir` following symlinks;
- `IsModule` using `os.Stat` instead of `os.Lstat`;
- `inspectModuleEntry` not recording symlinks;
- `computeModuleHash` following symlinks;
- `validateDestinationPath` omitted;
- `normalizeExistingOrParent` skipping the parent evaluation.

If a defect survives, the golden scope SHALL be widened, or the layer binding added, before the model is trusted.

#### Scenario: Surviving defect blocks trust
- **WHEN** a seeded defect does not make any golden replay, layer comparison, or rapid property fail
- **THEN** the calibration record SHALL say so, and the model SHALL NOT be listed as verifying that property until the defect is detected

### Requirement: Findings recording
Findings SHALL use fixed ids. F8–F12 are reserved by sibling changes, and this change owns F13 onward:
- F13 is the `invowk_modules/` symlink hole in script reads;
- F14 is the virtual-runtime `mkdir`-through-symlink fallback;
- F15 is the module-declared `workdir` widening the virtual roots under `restricted` access;
- F16 onward cover further discrepancies.

A suspected finding that does not reproduce in the real code SHALL NOT be recorded. Its id SHALL stay unused, and the change SHALL note it.

Each recorded finding SHALL consist of:
- a manifest command with `finding = "F<n>"`, `expect = "counterexample"`, and `mutant_of` naming the fixed check;
- the passing fixed configuration;
- a Go replay test in a `*_formal_test.go` file, named in a binding cell of the correspondence table;
- a row in the Findings table of `formal/README.md`.

A finding SHALL NOT count as its property's vacuity guard. Fixes are out of scope.

#### Scenario: Vendored-subtree symlink escape
- **WHEN** a module admitted by `Load` has a symlink under `invowk_modules/` to a file outside the module, and its `script.file` names that symlink
- **THEN** the current-code predicate, lexical containment against the raw module path, SHALL accept the read while it escapes. The finding command F13 SHALL produce a counterexample. The fixed configuration, containment against the physical resolution, SHALL pass. The Go replay SHALL show `ResolveScriptWithFSAndModule` returning the outside file's content

#### Scenario: Refuted hypothesis
- **WHEN** a suspected escape is shown unreachable under the modelled facts
- **THEN** the README SHALL record it as a refuted hypothesis naming the fact that prevents it, and a mutant that drops that fact SHALL exhibit the escape

### Requirement: Correspondence, wiring, and documentation
Each model header SHALL contain a correspondence table `| model element | Go symbol | file | binding | abstraction |`. The table SHALL cover every transcribed predicate, discovery fact, and allowed-location predicate. Every test in the change's `*_golden_test.go`, `*_rapid_test.go`, and `*_formal_test.go` files SHALL appear in a binding cell. `scripts/formal.py correspondence` SHALL pass.

The new packages and a matching `-run` pattern SHALL be added to the formal-verification workflow's replay step. Its path filter SHALL include the bound files (or promote-formal-ci-gate's `[ci] paths`, if that change has landed). `formal/README.md` and `.agents/skills/formal-verification/SKILL.md` SHALL document the models and the filesystem-materialising replay pattern.

#### Scenario: Stale correspondence fails
- **WHEN** a Go symbol named in either correspondence table is renamed or removed
- **THEN** `scripts/formal.py correspondence` SHALL fail and name the model and the stale row

#### Scenario: Edits to bound code trigger the lane
- **WHEN** a pull request changes a file bound by either model
- **THEN** the formal-verification workflow SHALL run the new replays and properties
