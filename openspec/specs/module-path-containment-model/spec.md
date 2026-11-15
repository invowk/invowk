# module-path-containment-model Specification

## Purpose
Specifies the formal models that check whether Invowk's module path handling keeps reads, executions, copies, hashes and writes inside a module's physical root, or inside a named, justified allowed location. An Alloy model covers module path references, discovery, vendoring, copying, hashing and ZIP unpack; a second Alloy model covers the virtual runtimes' path harness, which gates path access but is not a sandbox or a kernel jail. Golden-vector replays against real filesystems, rapid properties, seeded-defect calibration and recorded findings tie both models to the Go code, so that they detect escapes and regressions only within their bounded scopes.

## Requirements

### Requirement: Filesystem and path-reference model
Invowk SHALL maintain an Alloy 6 model, `formal/alloy/ModulePathContainment.als`, of a bounded filesystem tree and of the path references a module can declare. The tree SHALL contain:
- directories, regular files, symlinks, and Windows junctions;
- a host root, one or more module roots (`*.invowkmod`), including a module root that is itself a symlink;
- a `invowk_modules/` directory with vendored child modules;
- directories outside every module.

#### Scenario: Tree covers module and non-module locations
- **WHEN** the model's filesystem tree is instantiated
- **THEN** it SHALL be able to contain directories, regular files, symlinks, Windows junctions, a host root, module roots including one that is itself a symlink, an `invowk_modules/` directory with vendored child modules, and directories outside every module

### Requirement: Modelled link kinds
Each link SHALL carry a target reference, and the model SHALL cover each of these link kinds:
- a link to an in-module node;
- a link to an out-of-module node;
- a chain of two links;
- a dangling link;
- a two-link cycle;
- a link through a symlinked ancestor directory.

#### Scenario: Dangling, chained, and cyclic links
- **WHEN** a reference ends at a dangling symlink, at a chain of two symlinks, or at a two-link cycle
- **THEN** physical resolution SHALL be undefined for the dangling link, SHALL follow both hops of the chain, and SHALL be an error for the cycle, and a witness run SHALL show each case

### Requirement: Bounded link following
Link following SHALL be bounded at chain depth 2. A fact SHALL forbid acyclic chains longer than 2 in every instance, so that no instance depends on a chain-length limit. A cycle SHALL resolve to an error, as it does on the operating system.

#### Scenario: Cycle resolves to an error
- **WHEN** physical resolution reaches a two-link cycle
- **THEN** the result SHALL be an error, as it is on the operating system

### Requirement: Reference dialects and resolutions
A reference SHALL be a bounded sequence of name and `..` steps, relative to a base directory or absolute. Absolute references SHALL cover the Unix-rooted, Windows-drive, Windows-rooted, and UNC dialects that `isAbsolutePath` recognises. The model SHALL define two resolutions:
- lexical resolution, which applies `..` textually as `filepath.Clean` does;
- physical resolution, which follows links the way the operating system does.

#### Scenario: Lexical and physical resolution diverge
- **WHEN** a reference crosses an in-module symlink whose target lies outside the module
- **THEN** the model SHALL give the reference a lexical result inside the module and a physical result outside it, and a witness run SHALL show that this configuration is reachable

### Requirement: Case-insensitive filesystem mode
A filesystem mode flag SHALL make name lookup case-insensitive by folding names into equivalence classes. A fact SHALL forbid two siblings in the same fold class while the flag is set.

#### Scenario: Case-insensitive aliasing
- **WHEN** the filesystem is case-insensitive and a reference names a node with a different case
- **THEN** the lookup SHALL reach that node, while a transcribed exact-name comparison in the code SHALL still treat the two names as different

### Requirement: Independent containment policy
The model SHALL state the containment policy separately from the transcription of the implementation. For every bound operation, the policy SHALL require that each node the operation physically reads, executes, copies, hashes, or writes lies inside the module's physical root, or is admitted by a named allowed-location predicate. Each allowed-location predicate SHALL carry a stated justification.

#### Scenario: Policy is not a restatement of the code
- **WHEN** the policy predicate is compared with the implementation predicates
- **THEN** the policy SHALL be written in terms of physical resolution, root membership, and the allowed-location predicates only, without any lexical-check or discovery-fact predicate from the implementation

#### Scenario: Allowed locations are explicit
- **WHEN** an accepted operation touches a node outside the module root
- **THEN** the check SHALL pass only if a named allowed-location predicate admits that node for that operation kind, and each predicate SHALL appear in the model's correspondence table with its justification

### Requirement: Allowed-location predicate set
The set of allowed-location predicates SHALL contain:
- `allowedWorkspace`: the invowkfile directory mounted as the container workspace;
- `allowedVendoredSelf`: a vendored child's own root, for the child's own operations;
- `allowedVirtualRoot`: the virtual runtime's standard anchors, temporary roots, and declared `filesystem.paths`;
- `allowedWorkdirCwd`: a module-declared `workdir` as the execution working directory only, never as a read, copy, hash, or virtual allowed-root target.

#### Scenario: Workdir admitted only as working directory
- **WHEN** an operation uses a module-declared `workdir` outside the module root
- **THEN** `allowedWorkdirCwd` SHALL admit it only as the execution working directory, and never as a read, copy, hash, or virtual allowed-root target

### Requirement: Script files into vendored modules
A `script.file` into `invowk_modules/` SHALL be allowed only when it resolves physically inside the declaring module's root.

#### Scenario: Vendored script file must resolve inside the declaring module
- **WHEN** a `script.file` points into `invowk_modules/`
- **THEN** the policy SHALL allow it only if its physical resolution lies inside the declaring module's root

### Requirement: Transcription of the enforcement points
The model SHALL transcribe the decisions of the real enforcement points as separate predicates, one per layer. Discovery guarantees SHALL be named facts, so that a mutant can drop them.

The model SHALL NOT treat `Module.ValidateScriptPath` or `Module.checkSymlinkSafety` as enforcement. The correspondence table SHALL state that they have no production caller. Wiring or deleting them belongs to a fix change.

#### Scenario: Unwired helpers are not enforcement
- **WHEN** the correspondence table lists `Module.ValidateScriptPath` or `Module.checkSymlinkSafety`
- **THEN** it SHALL state that they have no production caller, and the model SHALL NOT treat them as enforcement

### Requirement: Transcribed reference checks and reads
The transcription SHALL cover these reference checks and reads:
- `ScriptFilePath.Validate` and `ScriptFilePath.ResolveFromModule`;
- `validateScriptPathContainment`, as called from `Implementation.ResolveScriptWithFSAndModule` and `CustomCheckScript.ResolveWithFSAndModule`;
- the interpreter's use of the `SelectedScriptFilePath()` path after the read.

#### Scenario: Script reference layers are separate predicates
- **WHEN** the model transcribes the script reference checks and reads
- **THEN** each listed function SHALL have its own layer predicate, from `ScriptFilePath.Validate` through the interpreter's use of the `SelectedScriptFilePath()` path

### Requirement: Transcribed env file, Containerfile, and workdir checks
The transcription SHALL also cover these reference checks and reads:
- `ValidateEnvFilePath` followed by `runtime.LoadEnvFile`;
- `ValidateContainerfilePath`;
- `Invowkfile.GetEffectiveWorkDir` and `ContainerRuntime.getContainerWorkDir`.

#### Scenario: Env file, Containerfile, and workdir layers are separate predicates
- **WHEN** the model transcribes the env file, Containerfile, and workdir checks
- **THEN** each listed function SHALL have its own layer predicate, from `ValidateEnvFilePath` through `ContainerRuntime.getContainerWorkDir`

### Requirement: Transcribed discovery facts
The transcription SHALL cover these discovery facts, as two separate named facts:
- `isModuleRejectsLinkedRoot`: the Lstat check in `invowkmod.IsModule` at discovery call sites;
- `loadScansNonLinkedRoot`: the tree scan in `invowkmod.Load` (`scanModuleTree`, `inspectModuleEntry`), including its exact-name skip of `invowk_modules`, which scans only a root that is not a symlink.

#### Scenario: Linked-root fact is load-bearing
- **WHEN** a mutant drops `isModuleRejectsLinkedRoot`
- **THEN** a check SHALL produce a counterexample in which a symlinked module root is admitted by `Load` without its tree being scanned, and its script read leaves the root

#### Scenario: Load scan is load-bearing
- **WHEN** a mutant drops `loadScansNonLinkedRoot`
- **THEN** the script-read check SHALL produce a counterexample in which a lexically contained script path reads a file outside the module through an in-module symlink

### Requirement: Transcribed discovery, copies, hashing, and unpack
The transcription SHALL cover these discovery, copy, hashing, and unpack points:
- the symlink skip in `discoverVendoredModulesWithDiagnostics`;
- the symlink-skipping copies `modulecache.copyDir` and `provision.CopyDir`, including their following of a symlinked source root;
- `computeModuleHash`;
- `normalizeZIPPath` and `validateDestinationPath` in ZIP unpack.

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

#### Scenario: Every check is non-vacuous
- **WHEN** `make formal-alloy` runs
- **THEN** every antecedent and witness command SHALL be satisfiable, every mutant SHALL produce a counterexample, and the runner SHALL fail closed otherwise

#### Scenario: Model source declares no commands
- **WHEN** a `.als` file of this change contains a `run` or `check` command
- **THEN** the runner SHALL fail the model

### Requirement: Safety-check mutants mirror plausible regressions
Each mutant SHALL mirror a plausible regression, not the current code. The mutants SHALL include:
- the containment check removed from `ResolveScriptWithFSAndModule`;
- `ScriptFilePath.Validate` without backslash normalisation;
- `isAbsolutePath` without the Windows-drive dialect.

#### Scenario: Listed path-check regressions are mutants
- **WHEN** `make formal-alloy` runs the mutant commands
- **THEN** each listed path-check regression SHALL be a mutant command, and each SHALL produce a counterexample

### Requirement: Copy and discovery mutants
The mutants SHALL also include:
- a copy that follows inner symlinks;
- `inspectModuleEntry` skipping any directory whose name case-folds to `invowk_modules`;
- the vendored-discovery scan not skipping symlinks;
- `isModuleRejectsLinkedRoot` dropped;
- `loadScansNonLinkedRoot` dropped.

#### Scenario: Listed copy and discovery regressions are mutants
- **WHEN** `make formal-alloy` runs the mutant commands
- **THEN** each listed copy and discovery regression SHALL be a mutant command, and each SHALL produce a counterexample

### Requirement: Virtual-runtime path harness model
Invowk SHALL maintain a second Alloy model, `formal/alloy/VirtualPathHarness.als`, of the virtual runtime's path policy. The model SHALL cover:
- the allowed-root list built by `newVirtualPathResolverForFilesystem` from the effective workdir, the script base path, `standardVirtualAnchorsForOS` anchors, the temporary roots, and the declared paths;
- the root comparison in `pathWithin`;
- the `restricted` and `full` access modes.

#### Scenario: Existing paths are compared like with like
- **WHEN** a path exists and crosses a symlink
- **THEN** the harness SHALL compare the evaluated path with the evaluated roots, and a mutant that compares the unevaluated path SHALL produce a counterexample

### Requirement: Harness normalisation and runtime-created links
The virtual-runtime path harness model SHALL also cover:
- normalisation by `normalizeExistingOrParent`: full `EvalSymlinks`, then parent-only `EvalSymlinks`, then a lexical fallback;
- symlinks created during execution by the u-root `ln` built-in, whose targets are stored literally.

#### Scenario: Deep non-existent path through an escaping link
- **WHEN** an allowed root contains a symlink to a writable directory outside every root, and a script requests a path with two or more missing components below that symlink
- **THEN** the current-code harness predicate SHALL accept the path, the finding command F14 SHALL produce a counterexample, and the fixed configuration that evaluates the deepest existing ancestor SHALL pass, provided task 1.3 confirms the behaviour in the real code; otherwise F14 SHALL stay unused and the change SHALL record the refutation

### Requirement: Harness policy under restricted access
The policy SHALL transcribe the existing requirements "Traversal out of an allowed root is blocked" (`openspec/specs/virtual-runtime-sandbox`) and "Restricted access limits virtual file I/O" (`openspec/specs/virtual-filesystem-access`). Under `restricted` access, every accepted access SHALL resolve physically inside an allowed root. `full` access is an explicit opt-out.

#### Scenario: Restricted access stays inside allowed roots
- **WHEN** the harness accepts an access under `restricted` access
- **THEN** the access SHALL resolve physically inside an allowed root

### Requirement: Harness workdir admission
Whether a module-declared `workdir` outside the module may enter the allowed-root list SHALL be checked against `allowedWorkdirCwd`, which admits the workdir as a working directory and not as an allowed root.

#### Scenario: Workdir widening
- **WHEN** a module-declared `workdir` outside the module is added to the allowed-root list under `restricted` access
- **THEN** a check against `allowedWorkdirCwd` SHALL produce a counterexample, recorded as finding F15, if a Go replay confirms that the real resolver accepts an access below that workdir

### Requirement: Golden-vector binding to real filesystem behaviour
Each model SHALL have a golden command in the manifest. It SHALL record the implementation's decision for every bound operation, per layer, in fields of a `one sig`: the syntactic validation, the containment check, the final physical read, copy, or hash, and the touched node. A `noJunk` predicate SHALL exclude atoms that cannot influence a decision.

#### Scenario: Golden records every layer decision
- **WHEN** the golden command of a model is run
- **THEN** each instance SHALL record, per bound operation, the syntactic validation, the containment check, the final physical read, copy, or hash, and the touched node

### Requirement: Golden file format
Golden files SHALL be generated in format 2. Editing the golden command's `body` or `scope` SHALL change the fingerprint. The files SHALL be loaded with `alloygolden.Load`.

#### Scenario: Golden command edits change the fingerprint
- **WHEN** the golden command's `body` or `scope` is edited
- **THEN** the golden file's fingerprint SHALL change

### Requirement: Golden replay tests
The replay tests SHALL do the following:
- materialise each instance with `internal/testutil/fstree` under `filepath.EvalSymlinks(t.TempDir())`;
- call the Go entry point bound to each operation in design.md D5, and compare every recorded layer decision and touched node;
- assert the Go transcriptions of the model's facts and policy on every instance.

An instance MAY be skipped for a missing capability only off that capability's required platforms (the `fstree` capability policy).

#### Scenario: Replay detects divergence
- **WHEN** a real function's layer decision or touched node differs from the model's golden instance
- **THEN** the replay test SHALL fail, naming the instance index, the operation, and the layer

#### Scenario: Replay places nodes outside every root
- **WHEN** the harness golden instances are replayed
- **THEN** at least one replayed instance SHALL contain a node outside every allowed root the replay constructs, and the test SHALL fail if none does

#### Scenario: Platform gating is explicit
- **WHEN** an instance needs a capability the host lacks (symlink creation, junctions off Windows, or a case-insensitive temporary filesystem), and the capability is not required on the running GOOS
- **THEN** the test SHALL skip only that instance, report the skip count per capability, and still assert the Go transcriptions on it

#### Scenario: Required capability missing
- **WHEN** an instance needs a capability that is required on the running GOOS but unavailable
- **THEN** the replay SHALL fail with the capability's probe error, and SHALL NOT skip the instance

### Requirement: Operations without a Go entry point
An operation with no Go entry point SHALL be marked `-` in the binding column of the correspondence table. It SHALL be excluded from the replay, and the spec's "every bound operation" does not include it.

#### Scenario: Unbound operation is excluded
- **WHEN** a bound operation has no Go entry point
- **THEN** its binding cell SHALL be `-`, and the replay SHALL exclude it

### Requirement: Rapid properties at larger scopes
Invowk SHALL add `pgregory.net/rapid` properties, `TestModulePathContainment_Property` and `TestVirtualPathHarness_Property`, in `*_rapid_test.go` files. Each property SHALL generate module trees, link placements, and references beyond the golden scope, materialise them, and compare the real decisions with the Go transcription of the policy. The properties SHALL follow the repository's rapid conventions. Their packages SHALL be added to `FORMAL_RAPID_PACKAGES`.

#### Scenario: Larger scope property
- **WHEN** `make formal-rapid-deep` runs the path-containment properties with `RAPID_CHECKS=10000`
- **THEN** no generated case SHALL show a policy violation other than those recorded as findings, and the run SHALL stay within the budget recorded in design.md

### Requirement: Cost budgets
The change SHALL record measured cost budgets:
- the golden instance count and decoded size of each model, measured with a capped `-r` before the scope is fixed;
- the golden replay time in `make test`: 30 s or less per OS on Linux and macOS, and on Windows a budget derived from three CI runs;
- the deep rapid time, 60 s or less.

The budgets SHALL be handed to `promote-formal-ci-gate` as `budget_seconds` values.

#### Scenario: Budget overrun blocks the scope
- **WHEN** a replay or deep-rapid time measured while the budgets are recorded exceeds its budget on any operating system
- **THEN** the golden scope or the rapid generator SHALL be narrowed, with the reason recorded, before the figure is recorded
- **THEN** `formal/README.md` SHALL say that no CI step enforces the recorded figures

#### Scenario: Windows budget is derived from three runs of the final replay inputs
- **WHEN** three windows-latest CI runs of one measured head, re-runs included, have finished
- **AND** the measured head is an ancestor of the archived head, and `git diff <measured head> <archived head>` is empty for the golden-replay inputs
- **THEN** the Windows replay budget SHALL be `ceil(1.5 × the slowest of those runs)`
- **THEN** `formal/README.md` SHALL record it with the measured head, the run ids and attempts, the test flags, and the empty input diff

#### Scenario: Windows budget is not set from other trees
- **WHEN** fewer than three windows-latest runs of such a head exist
- **THEN** the Windows replay budget SHALL stay pending, and `formal/README.md` SHALL say so and that no step or test enforces it

### Requirement: Cost budgets state where and how they were measured
The golden replay time SHALL be measured on all three operating systems, and the deep rapid time on Linux, the only OS whose CI runs `make formal-rapid-deep`. CI figures SHALL state their test flags.

#### Scenario: A CI figure names its basis
- **WHEN** a golden-replay figure is recorded from a CI run
- **THEN** `formal/README.md` SHALL state the run's test flags, such as `-race -short`, which replays every 16th instance

#### Scenario: Deep rapid is measured on Linux only
- **WHEN** the deep rapid budget is recorded
- **THEN** it SHALL come from a Linux measurement

### Requirement: Calibration record
Each model's `calibration` entry in `formal/manifest.toml` SHALL list the seeded real-code defects its bindings detect. Defects SHALL be seeded one at a time. If a defect survives, the golden scope SHALL be widened, or the layer binding added, before the model is trusted.

#### Scenario: Surviving defect blocks trust
- **WHEN** a seeded defect does not make any golden replay, layer comparison, or rapid property fail
- **THEN** the calibration record SHALL say so, and the model SHALL NOT be listed as verifying that property until the defect is detected

### Requirement: Seeded script-path calibration defects
The seeded defects SHALL include:
- the containment check removed from `ResolveScriptWithFSAndModule`;
- `validateScriptPathContainment` accepting a `..` prefix, detected through a direct binding via a test-only `export_test.go` hook, because `Validate` masks it at every exported entry point;
- `ScriptFilePath.Validate` skipping backslash normalisation, detected by the Validate-layer decision.

#### Scenario: Script-path defects are seeded
- **WHEN** the calibration defects are seeded one at a time
- **THEN** the seeded set SHALL include each listed script-path defect, detected through the stated binding or layer

### Requirement: Seeded copy, discovery, hash, unpack, and harness calibration defects
The seeded defects SHALL also include:
- `modulecache.copyDir` following symlinks;
- `provision.CopyDir` following symlinks;
- `IsModule` using `os.Stat` instead of `os.Lstat`;
- `inspectModuleEntry` not recording symlinks;
- `computeModuleHash` following symlinks;
- `validateDestinationPath` omitted;
- `normalizeExistingOrParent` skipping the parent evaluation.

#### Scenario: Remaining defects are seeded
- **WHEN** the calibration defects are seeded one at a time
- **THEN** the seeded set SHALL include each listed copy, discovery, hash, unpack, and harness defect

### Requirement: Findings recording
Findings SHALL use fixed ids. F8–F12 are reserved by sibling changes, and this change owns F13 onward:
- F13 is the `invowk_modules/` symlink hole in script reads;
- F14 is the virtual-runtime `mkdir`-through-symlink fallback;
- F15 is the module-declared `workdir` widening the virtual roots under `restricted` access;
- F16 onward cover further discrepancies.

#### Scenario: Vendored-subtree symlink escape
- **WHEN** a module admitted by `Load` has a symlink under `invowk_modules/` to a file outside the module, and its `script.file` names that symlink
- **THEN** the current-code predicate, lexical containment against the raw module path, SHALL accept the read while it escapes. The finding command F13 SHALL produce a counterexample. The fixed configuration, containment against the physical resolution, SHALL pass. The Go replay SHALL show `ResolveScriptWithFSAndModule` returning the outside file's content

### Requirement: Unreproduced findings
A suspected finding that does not reproduce in the real code SHALL NOT be recorded. Its id SHALL stay unused, and the change SHALL note it.

#### Scenario: Refuted hypothesis
- **WHEN** a suspected escape is shown unreachable under the modelled facts
- **THEN** the README SHALL record it as a refuted hypothesis naming the fact that prevents it, and a mutant that drops that fact SHALL exhibit the escape

### Requirement: Recorded finding contents
Each recorded finding SHALL consist of:
- a manifest command with `finding = "F<n>"`, `expect = "counterexample"`, and `mutant_of` naming the fixed check;
- the passing fixed configuration;
- a Go replay test in a `*_formal_test.go` file, named in a binding cell of the correspondence table;
- a row in the Findings table of `formal/README.md`.

A finding SHALL NOT count as its property's vacuity guard. Fixes are out of scope.

#### Scenario: Finding is complete and not a vacuity guard
- **WHEN** a finding is recorded
- **THEN** it SHALL have its manifest command, passing fixed configuration, Go replay test, and README row, and it SHALL NOT count as its property's vacuity guard

### Requirement: Correspondence, wiring, and documentation
Each model header SHALL contain a correspondence table `| model element | Go symbol | file | binding | abstraction |`. The table SHALL cover every transcribed predicate, discovery fact, and allowed-location predicate. Every test in the change's `*_golden_test.go`, `*_rapid_test.go`, and `*_formal_test.go` files SHALL appear in a binding cell. `scripts/formal.py correspondence` SHALL pass.

#### Scenario: Stale correspondence fails
- **WHEN** a Go symbol named in either correspondence table is renamed or removed
- **THEN** `scripts/formal.py correspondence` SHALL fail and name the model and the stale row

### Requirement: Formal workflow wiring and documentation
The new packages and a matching `-run` pattern SHALL be added to the formal-verification workflow's replay step. Its path filter SHALL include the bound files (or promote-formal-ci-gate's `[ci] paths`, if that change has landed). `formal/README.md` and `.agents/skills/formal-verification/SKILL.md` SHALL document the models and the filesystem-materialising replay pattern.

#### Scenario: Edits to bound code trigger the lane
- **WHEN** a pull request changes a file bound by either model
- **THEN** the formal-verification workflow SHALL run the new replays and properties

### Requirement: Junction creation is verified
On Windows, `internal/testutil/fstree` SHALL create directory junctions that pass a functional check:
- the link is not reported as `ModeSymlink`;
- a file read through the link is `os.SameFile` with the file in the target directory.

A Windows-only test SHALL require junction creation and report the creation error. Off Windows, junction creation SHALL return the "directory junctions are supported only on Windows" error.

#### Scenario: Junction writer works on Windows CI
- **WHEN** the Windows-only junction test runs on windows-latest
- **THEN** it SHALL pass the functional check, or fail and name the creation errors

#### Scenario: Off Windows the writer is not compiled
- **WHEN** `internal/testutil/fstree` is built for Linux or macOS
- **THEN** junction creation SHALL return the "directory junctions are supported only on Windows" error, and the policy SHALL treat junctions as not required

### Requirement: Junction-writer failures are investigated in a fixed order
Junction-writer failures SHALL be investigated in this order, and the second step SHALL land only if the first did not resolve the failure:
1. surface the creation error;
2. compare the mount-point reparse buffer field by field with Go's own Windows `os` test helper, and fix each divergence.

Junction creation SHALL NOT fall back to another writer, such as `cmd /c mklink /J`, because the capability probe uses the same creation path.

#### Scenario: Reparse writer fails
- **WHEN** the reparse writer fails on Windows
- **THEN** junction creation and the capability probe SHALL report that error, and no other writer SHALL be tried

### Requirement: The rapid property generates junctions on Windows
`TestModulePathContainment_Property` SHALL draw the link kind from {symlink, junction} wherever junctions are available. On Windows they are required, so the junction branch SHALL run there. A junction SHALL target a directory: either inside the module, or outside it with a unique sentinel file.

#### Scenario: Junction branch is exercised
- **WHEN** the property runs on Windows
- **THEN** generated cases SHALL include junction links, both escaping and contained

### Requirement: Link escapes are judged by file identity
The rapid property SHALL judge escape, for every link kind, by file identity of the opened file against the outside sentinel (`os.SameFile`, backed by a content comparison). It SHALL NOT judge escape by `filepath.EvalSymlinks` followed by a path-prefix test. An escape under `invowk_modules/` SHALL remain admissible as the recorded F13 hole for either link kind. A non-vendored junction escape SHALL fail the property unless F16 is a recorded finding.

#### Scenario: Junction escape is detected without EvalSymlinks
- **WHEN** a script read through a non-vendored junction opens the outside sentinel
- **THEN** the property SHALL report a policy violation, unless F16 is a recorded finding

#### Scenario: Vendored junction escape is F13
- **WHEN** a script read through a junction under `invowk_modules/` opens the outside sentinel
- **THEN** the property SHALL admit it as the recorded F13 hole

### Requirement: The F16 leg asserts each operation's verdict
`TestModulePathContainment_F16Junction` SHALL require junctions through the capability policy. For a module with an inner junction to an outside directory, it SHALL assert, never only log, a verdict for each operation production runs: `IsModule` and `Load` admission, the script read through the junction, `modulecache.copyDir`, `provision.CopyDir`, and `computeModuleHash`. Before F16 is decided, each verdict is containment; afterwards, it is the recorded outcome's verdict.

#### Scenario: Copy and hash containment
- **WHEN** the leg checks the copies and the hash
- **THEN** each copy and both hash computations SHALL succeed, and an error SHALL fail the leg
- **THEN** a copy SHALL count as escaping if it contains the outside sentinel, and the hash SHALL count as escaping if it changes when only the outside sentinel changes

#### Scenario: Leg asserts the recorded verdicts
- **WHEN** F16 is recorded under outcome (b)
- **THEN** the leg SHALL assert the escape for each operation the outcome records as escaping and containment for every other operation
- **THEN** the leg SHALL fail when any operation's behaviour differs from its recorded verdict

#### Scenario: Junctions unavailable on Windows CI
- **WHEN** the leg cannot create a junction on Windows CI
- **THEN** the leg SHALL fail with the probe error, and SHALL NOT be skipped

### Requirement: Copy and hash containment is modelled for junction modules
The `ModulePathContainment` model SHALL check copy and hash containment for modules that today's Windows scan admits (`admittedScanning[_, SymKind]`), which include modules with an inner non-vendored junction. The pass command SHALL have an antecedent that requires such a junction, and a mutant that follows links.

#### Scenario: Junction module copy and hash
- **WHEN** `make formal` runs the `ModulePathContainment` commands
- **THEN** `copyHashJunctionScan` SHALL pass, `anteCopyHashJunction` SHALL find an instance whose module has an inner junction, and `mutantCopyFollowsJunction` SHALL find a counterexample

### Requirement: F16 is decided by one of three stated outcomes
The Windows CI run of the change SHALL decide one of these outcomes, recorded with the run id in `ModulePathContainment`'s calibration text:
- **(a)** junction creation still fails: the leg and the job fail, and the change does not complete;
- **(b)** an operation escapes: F16 is recorded under "Recorded finding contents", and is not fixed;
- **(c)** every operation is contained: F16 is a refuted hypothesis under "Unreproduced findings", and its id stays unused.

#### Scenario: Escape confirmed
- **WHEN** the leg observes an escape on unmodified code on Windows CI
- **THEN** F16 SHALL be recorded with a manifest `finding = "F16"` command next to a passing fix configuration, a README Findings row, the leg kept as a characterisation replay asserting the escape, and an F16 carve-out in the rapid property mirroring F13
- **THEN** the failing containment assertion SHALL NOT be committed

#### Scenario: Containment confirmed
- **WHEN** every operation stays inside the module on Windows CI
- **THEN** `formal/README.md` SHALL list F16 among the refuted hypotheses, naming for each operation the modelled fact that keeps it contained and the manifest mutant that drops that fact, and the leg SHALL keep asserting containment

#### Scenario: Containment without a modelled reason
- **WHEN** an operation is contained for a reason that no modelled fact states
- **THEN** the work SHALL stop and report it, and no fact SHALL be invented to explain it

### Requirement: The F16 leg is never a killer
Under every F16 outcome, `TestModulePathContainment_F16Junction` SHALL stay a characterisation test: its row's abstraction cell SHALL begin `characterisation:`, and the test SHALL NOT be a mutation or reach killer.

#### Scenario: Leg excluded from killers
- **WHEN** the formal-bindings plan is built
- **THEN** `TestModulePathContainment_F16Junction` SHALL NOT appear among any row's killers

### Requirement: Junctions stay out of the golden scope
The `ModulePathContainment` golden SHALL keep junctions fixed off. Junctions SHALL be covered by the witness commands, the Windows junction branch of the rapid property, and the F16 leg. `formal/README.md` and the model's calibration text SHALL describe exactly that coverage.

#### Scenario: README coverage claim matches the tests
- **WHEN** `formal/README.md` describes how junctions are covered
- **THEN** it SHALL name only the witness commands, the rapid property's junction branch on Windows, and the F16 leg, and each named piece SHALL exist

### Requirement: Custom-check reads are replayed through the real resolver
`TestModulePathContainment_GoldenVectors` SHALL replay `CustomCheckScript.ResolveWithFSAndModule` on every `ScriptRead` instance with a recording reader. It SHALL require the same acceptance and touched node as the model's current-code verdict, including F13 instances. The `readOp` row of that function SHALL name this test.

#### Scenario: Custom-check replay reproduces F13
- **WHEN** a golden `ScriptRead` instance reaches an `invowk_modules/` symlink that escapes the module
- **THEN** the custom-check replay SHALL accept and touch the outside node, as the model's current-code verdict does

### Requirement: Env-file reads are replayed through the real loader
`TestModulePathContainment_GoldenVectors` SHALL call `LoadEnvFile` on every `EnvRead` instance of an admitted module whose path `ValidateEnvFilePath` accepts. It SHALL require the model's acceptance and touched node for `EnvRead`, including F17 instances. The touched file SHALL be identified by a sentinel in its content, and a non-file or missing target SHALL make `LoadEnvFile` return an error.

#### Scenario: Env replay reproduces F17
- **WHEN** a golden `EnvRead` instance on an admitted module reads `invowk_modules/x`, and `x` is a symlink that escapes
- **THEN** `LoadEnvFile` SHALL load the outside sentinel, as the model's current-code verdict does

#### Scenario: Missing env target
- **WHEN** a golden `EnvRead` instance names a dangling link or a directory
- **THEN** `LoadEnvFile` SHALL return an error

### Requirement: The container workdir row names a reaching binding
The `workdirCwd` row for `Invowkfile.GetEffectiveWorkDir` SHALL name `TestModulePathContainment_ContainerWorkDirGolden`, which calls that function, instead of `TestModulePathContainment_GoldenVectors`.

#### Scenario: Workdir row reaches its function
- **WHEN** the reach gate runs after the rebinding
- **THEN** the `GetEffectiveWorkDir` row SHALL be reached by `TestModulePathContainment_ContainerWorkDirGolden`

### Requirement: Virtual-harness construction rows are bound by a construction replay
A new `TestVirtualPathHarness_RootConstruction` SHALL bind the `allowedRoots`, `anchors`, and `access` rows. It SHALL build resolvers with `newVirtualPathResolverForFilesystem` and require their acceptance of probe paths in each location kind (`InReadRoot`, `InWorkdir`, `OutsideAll`) to equal `H.access = Full or locIsAllowedRoot[l, True]`, the current code (F15). `EffectiveAccess` SHALL return `restricted` when access is omitted or set to `restricted`, and `full` when set to `full`.

#### Scenario: Workdir widening reproduced
- **WHEN** the construction replay probes a workdir-only path under `restricted` access
- **THEN** the resolver SHALL accept it, as the model's current-code construction (F15) does

#### Scenario: Outside path rejected
- **WHEN** the construction replay probes a path outside every root under `restricted` access
- **THEN** the resolver SHALL reject it

#### Scenario: Omitted access is restricted
- **WHEN** the filesystem config omits the access mode
- **THEN** `EffectiveAccess` SHALL return `restricted`, as the virtual-filesystem-access spec states

### Requirement: The anchors row claims only explicit anchor roots
The construction replay SHALL require `@config`, `@data`, `@cache`, `@state`, `@tmp`, and `@work`, as the resolver reports them, to be accepted, and a path under `@home` outside every other root to be rejected under restricted access (`cross-platform-path-anchors` "Anchor allow roots are explicit"). The `anchors` row's abstraction cell SHALL say that anchor paths are inputs from the production resolver, not verified.

#### Scenario: Home anchor is not an allowed root
- **WHEN** the replay probes a path under `@home` that no other anchor or root encloses, under restricted or omitted access
- **THEN** the resolver SHALL reject it

#### Scenario: Seeded home root
- **WHEN** `@home` is seeded into the allowed roots of `newVirtualPathResolverForFilesystem`
- **THEN** the construction replay SHALL fail, and the result SHALL be recorded in `VirtualPathHarness`'s calibration text

#### Scenario: Abstraction cell lists what the replay catches
- **WHEN** the `anchors` row's abstraction cell is read
- **THEN** it SHALL list exactly the defect classes the replay catches: an empty anchor, a reported anchor missing from the allowed roots where no other root encloses it, and an anchor that covers `@home/.ssh`
- **THEN** it SHALL name the masked cases: dropping `@work` is equivalent, because the workdir is always a root, and dropping `@tmp` is masked off Windows while `TMPDIR` lies under `/tmp`

### Requirement: The unpack destination row is unbound
The `unpackRejectsEscape` row SHALL have binding `-`, because `validateDestinationPath` alone is not the model's destination check (`unpackInside[destCheck]`, `refShape = RelInside`). With the `Join` call shape it accepts an absolute name, and after `normalizeZIPPath` its rejection is unreachable on POSIX. The row's abstraction cell SHALL state this, and SHALL name the follow-up that binds `unpackInside` to `normalizeZIPPath`.

#### Scenario: Unpack row reported, not gated
- **WHEN** the formal-bindings plan or the reach gate runs
- **THEN** the `unpackRejectsEscape` row SHALL be reported as `no-binding`, with its abstraction cell

### Requirement: Seeded defects calibrate each rebound function
For each function rebound by this change, one plausible defect SHALL be seeded in the real function and the new binding SHALL be shown to fail. The defect SHALL be reverted, and the result SHALL be recorded in the model's calibration text.

#### Scenario: Seeded defect in a rebound function
- **WHEN** a plausible defect is seeded in one rebound function, for example making `LoadEnvFile` ignore `basePath`
- **THEN** its new binding SHALL fail, and the result SHALL be recorded in the model's calibration text

#### Scenario: Masked defect
- **WHEN** a planned defect survives because another layer fully masks it at that entry point, as `ScriptFilePath.Validate` masks `validateScriptPathContainment` in `CustomCheckScript.ResolveWithFSAndModule`
- **THEN** a defect the binding catches SHALL be seeded instead, and the masked one SHALL be recorded as redundant there in the calibration and `tasks/next/formal-binding-gaps.md`, not in the ledger
