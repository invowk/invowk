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
- `allowedTrustedWorkdir` (`VirtualPathHarness` only): a trusted workdir, that is the `--ivk-workdir` override, a workdir of an invowkfile outside a module, or a module's default workdir, as a virtual allowed root under `restricted` access. Justification: the user chose the directory, or it is the module directory, so no module widens its own roots through it;
- `allowedWorkdirCwd`: a module-declared `workdir` that lies outside the module, as the execution working directory only, never as a read, copy, hash, or virtual allowed-root target.

#### Scenario: Workdir admitted only as working directory
- **WHEN** an operation uses a module-declared `workdir` outside the module root
- **THEN** `allowedWorkdirCwd` SHALL admit it only as the execution working directory, and never as a read, copy, hash, or virtual allowed-root target

#### Scenario: Trusted workdir admitted as a virtual root
- **WHEN** a request under `restricted` access resolves physically inside a trusted workdir
- **THEN** `allowedTrustedWorkdir` SHALL admit it, and the predicate SHALL appear in the `VirtualPathHarness` correspondence table with its justification

### Requirement: Script files into vendored modules
A `script.file` into `invowk_modules/` SHALL be allowed only when it resolves physically inside the declaring module's root.

#### Scenario: Vendored script file must resolve inside the declaring module
- **WHEN** a `script.file` points into `invowk_modules/`
- **THEN** the policy SHALL allow it only if its physical resolution lies inside the declaring module's root

### Requirement: Transcription of the enforcement points
The model SHALL transcribe the decisions of the real enforcement points as separate predicates, one per layer. Discovery guarantees SHALL be named facts, so that a mutant can drop them.

`Module.ValidateScriptPath` and `Module.checkSymlinkSafety` SHALL NOT exist: they had no production caller, so this change deletes them. The correspondence table, the reach list and the formal-bindings ledger SHALL NOT name them.

#### Scenario: Unwired helpers are not enforcement
- **WHEN** `scripts/formal.py correspondence` and the reach gate run
- **THEN** no correspondence row, reach entry or ledger entry SHALL name `ValidateScriptPath` or `checkSymlinkSafety`

### Requirement: Transcribed reference checks and reads
The transcription SHALL cover these reference checks and reads:
- `ScriptFilePath.Validate` and `ScriptFilePath.ResolveFromModule`;
- `validateScriptPathContainment`, as called from `Implementation.ResolveScriptWithFSAndModule` and `CustomCheckScript.ResolveWithFSAndModule`;
- the physical containment check those two resolvers run in module context before they read;
- the interpreter's use of the `SelectedScriptFilePath()` path after the read.

#### Scenario: Script reference layers are separate predicates
- **WHEN** the model transcribes the script reference checks and reads
- **THEN** each listed function SHALL have its own layer predicate, from `ScriptFilePath.Validate` through the interpreter's use of the `SelectedScriptFilePath()` path, and the physical check SHALL be bound as `physContains`

### Requirement: Transcribed env file, Containerfile, and workdir checks
The transcription SHALL also cover these reference checks and reads:
- `ValidateEnvFilePath` followed by `runtime.LoadEnvFile`, which in module context reads only a file that resolves inside the module root;
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
- whether the effective workdir and `@work` enter that list, from the workdir's kind: trusted (the CLI override, a workdir of an invowkfile outside a module, or a module's default workdir), module-declared and physically inside the module, or module-declared and physically outside the module;
- the root comparison in `pathWithin`;
- the `restricted` and `full` access modes.

#### Scenario: Existing paths are compared like with like
- **WHEN** a path exists and crosses a symlink
- **THEN** the harness SHALL compare the evaluated path with the evaluated roots, and a mutant that compares the unevaluated path SHALL produce a counterexample

#### Scenario: Workdir kinds are modelled
- **WHEN** the model's harness configuration is read
- **THEN** it SHALL hold one workdir kind out of trusted, module-inside and module-outside, and a module-inside workdir SHALL leave no location that lies inside the workdir only, because the script base, an allowed root, encloses it

### Requirement: Harness normalisation and runtime-created links
The virtual-runtime path harness model SHALL also cover:
- normalisation by `normalizeExistingOrParent`: the deepest existing ancestor is resolved physically, through every symbolic link and Windows junction, and the missing components are joined to it; there is no lexical fallback;
- symlinks created during execution by the u-root `ln` built-in, whose targets are stored literally.

#### Scenario: Deep non-existent path through an escaping link
- **WHEN** an allowed root contains a symlink to a writable directory outside every root, and a script requests a path with two or more missing components below that symlink
- **THEN** the harness predicate SHALL deny the path, the fixed configuration that evaluates the deepest existing ancestor SHALL pass, and the pre-fix mutant `mutantPreFixF14ParentOnlyEval` SHALL produce a counterexample

#### Scenario: Path through an escaping junction
- **WHEN** an allowed root contains a junction to a directory outside every root, and a script requests a path that ends at the junction or lies below it, at any number of missing components
- **THEN** the harness predicate SHALL deny the path, and the pre-fix mutant `mutantPreFixF18JunctionNotFollowed` SHALL produce a counterexample

#### Scenario: Dangling link to outside
- **WHEN** an allowed root contains a symlink whose target is missing and lies outside every root, and a script requests the link or a path below it
- **THEN** the harness predicate SHALL deny the path, and the pre-fix mutant `mutantPreFixF19DanglingLinkAsMissingName` SHALL produce a counterexample

### Requirement: Harness policy under restricted access
The policy SHALL transcribe the existing requirements "Traversal out of an allowed root is blocked" (`openspec/specs/virtual-runtime-sandbox`) and "Restricted access limits virtual file I/O" (`openspec/specs/virtual-filesystem-access`). Under `restricted` access, every accepted access SHALL resolve physically inside an allowed root. `full` access is an explicit opt-out.

#### Scenario: Restricted access stays inside allowed roots
- **WHEN** the harness accepts an access under `restricted` access
- **THEN** the access SHALL resolve physically inside an allowed root

### Requirement: Harness workdir admission
Under `restricted` access a module-declared `workdir` that resolves physically outside the module SHALL be admitted only as a working directory (`allowedWorkdirCwd`), never as an allowed root. A trusted workdir SHALL be an allowed root (`allowedTrustedWorkdir`). The fixed root rule SHALL admit the workdir exactly when it is trusted or module-inside.

#### Scenario: Fixed rule keeps the outside workdir out
- **WHEN** `make formal` checks `harnessContained` with the fixed evaluation and the fixed root rule
- **THEN** the check SHALL pass for every workdir kind

#### Scenario: Workdir widening
- **WHEN** `make formal` checks `harnessContained` with the fixed evaluation and the pre-fix rule that always admits the workdir
- **THEN** the check SHALL produce a counterexample with a module-outside workdir, recorded as `mutantPreFixF15WorkdirRoot`, the pre-fix mutant of F15

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
- in the harness resolver, one at a time: returning the unresolved path when two or more components are missing; treating a link whose target is missing as a missing name; not following a junction; treating an error other than not-exist as a missing component; and joining a `..` that follows a missing component instead of refusing it.

#### Scenario: Remaining defects are seeded
- **WHEN** the calibration defects are seeded one at a time
- **THEN** the seeded set SHALL include each listed copy, discovery, hash, unpack, and harness defect

#### Scenario: Harness resolver seeds name their failing binding
- **WHEN** each harness resolver defect is seeded and reverted
- **THEN** `VirtualPathHarness`'s calibration text SHALL record which binding failed: the golden replay, the named replay of the matching finding, or the resolver's fake-filesystem test (the junction seed is shown on Linux by the fake-filesystem test)

### Requirement: Findings recording
Findings SHALL use fixed ids. F8–F12 are reserved by sibling changes, and the model-module-path-containment change owns F13 onward:
- F13 is the `invowk_modules/` symlink hole in script reads, fixed by module-read-physical-containment;
- F14 is the virtual-runtime `mkdir`-through-symlink fallback, fixed by virtual-path-physical-normalisation;
- F15 is the module-declared `workdir` widening the virtual roots under `restricted` access, fixed by module-workdir-roots;
- F16 onward cover further discrepancies; F17 is the same hole in env-file reads, also fixed;
- F18 is the virtual-runtime path harness not following Windows junctions, and F19 is the harness judging a dangling link as a missing name; both are recorded as fixed after an observed escape by virtual-path-physical-normalisation.

#### Scenario: Vendored-subtree symlink escape
- **WHEN** a module admitted by `Load` has a symlink under `invowk_modules/` to a file outside the module, and its `script.file` names that symlink
- **THEN** the pre-fix predicate, lexical containment against the raw module path, SHALL accept the read while it escapes, and `mutantPreFixF13…` SHALL produce a counterexample. The fixed configuration, containment against the physical resolution, SHALL pass. The Go replay SHALL show `ResolveScriptWithFSAndModule` denying the read

#### Scenario: Harness findings cite their evidence
- **WHEN** the README Findings rows of F18 and F19 are read
- **THEN** the F18 row SHALL cite the windows-latest probe (CI run 44523520503, job 135557989975) as the observed escape, and the F19 row SHALL cite its replay `TestVirtualPathHarness_DanglingLinkEscapeDenied` failing against the product code of c2fe96ff, with the steps to re-run it, and each row SHALL say "Fixed" and name its pre-fix mutant and its replay

#### Scenario: F19 evidence can be re-run
- **WHEN** `internal/runtime/virtual_path_harness_formal_test.go` of this change replaces the same file in an export of c2fe96ff, and `TestVirtualPathHarness_DanglingLinkEscapeDenied` is run there
- **THEN** the file SHALL compile, and the test SHALL fail

#### Scenario: F15 row says fixed
- **WHEN** the README Findings row of F15 is read
- **THEN** it SHALL say "Fixed", name `mutantPreFixF15WorkdirRoot` and the replay `TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot`, and cite the replay `TestVirtualPathHarness_F15WorkdirWidening` of the base commit as the reproduction before the fix

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
The rapid property SHALL judge escape, for every link kind, by file identity of the opened file against the outside sentinel (`os.SameFile`, backed by a content comparison). It SHALL NOT judge escape by `filepath.EvalSymlinks` followed by a path-prefix test. An escape under `invowk_modules/` SHALL fail the property for either link kind. A non-vendored junction escape SHALL fail the property unless F16 is a recorded finding for the script read. The scenario named "Vendored junction escape is F13" keeps its pre-fix name and now asserts a violation.

#### Scenario: Junction escape is detected without EvalSymlinks
- **WHEN** a script read through a non-vendored junction opens the outside sentinel
- **THEN** the property SHALL report a policy violation, unless F16 is a recorded finding for the script read

#### Scenario: Vendored junction escape is F13
- **WHEN** a script read through a symlink or junction under `invowk_modules/` opens the outside sentinel
- **THEN** the property SHALL report a policy violation

### Requirement: The F16 leg asserts each operation's verdict
`TestModulePathContainment_F16Junction` SHALL require junctions through the capability policy. For a module with an inner junction to an outside directory, it SHALL assert, never only log, a verdict for each operation production runs: `IsModule` and `Load` admission, the script read through the junction, `modulecache.copyDir`, `provision.CopyDir`, and `computeModuleHash`. Each verdict is the one observed on windows-latest for the current code and recorded with its run id; with the contained read in place, the script read's expected verdict is containment, recorded as "pending windows-latest evidence" until the post-evidence F16 commit.

#### Scenario: Copy and hash containment
- **WHEN** the leg checks the copies and the hash
- **THEN** each copy and both hash computations SHALL succeed, and an error SHALL fail the leg
- **THEN** a copy SHALL count as escaping if it contains the outside sentinel, and the hash SHALL count as escaping if it changes when only the outside sentinel changes

#### Scenario: Leg asserts the recorded verdicts
- **WHEN** F16 is recorded under outcome (b)
- **THEN** the leg SHALL assert the escape for each operation the outcome records as escaping and containment for every other operation, except that the script read's verdict is containment pending evidence (see "Interim verdict before the evidence")
- **THEN** the leg SHALL fail when any operation's behaviour differs from its recorded verdict

#### Scenario: Script read through the junction is contained
- **WHEN** the leg reads `j/secret` through the inner junction with the contained read in place
- **THEN** the resolver SHALL deny the read, and the leg SHALL assert that it does not return the outside sentinel

#### Scenario: Junctions unavailable on Windows CI
- **WHEN** the leg cannot create a junction on Windows CI
- **THEN** the leg SHALL fail with the probe error, and SHALL NOT be skipped

### Requirement: Copy and hash containment is modelled for junction modules
The `ModulePathContainment` model SHALL check copy and hash containment for modules that today's Windows scan admits (`admittedScanning[_, SymKind]`), which include modules with an inner non-vendored junction. The pass command SHALL have an antecedent that requires such a junction, and a mutant that follows links.

#### Scenario: Junction module copy and hash
- **WHEN** `make formal` runs the `ModulePathContainment` commands
- **THEN** `copyHashJunctionScan` SHALL pass, `anteCopyHashJunction` SHALL find an instance whose module has an inner junction, and `mutantCopyFollowsJunction` SHALL find a counterexample

### Requirement: F16 is decided by one of the stated outcomes
The Windows CI run of the change SHALL decide one of these outcomes, recorded with the run id in `ModulePathContainment`'s calibration text:
- **(a)** junction creation still fails: the leg and the job fail, and the change does not complete;
- **(b)** an operation escapes: F16 is recorded under "Recorded finding contents", and is not fixed;
- **(c)** every operation is contained: F16 is a refuted hypothesis under "Unreproduced findings", and its id stays unused;
- **(d)** an escape observed on an earlier run is closed by a later fix, and every operation is now contained: F16 is fixed, and its finding command becomes a `mutantPreFixF16…` regression mutant.

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

#### Scenario: Fixed after an observed escape
- **WHEN** F16 was recorded under outcome (b), and the windows-latest run after a fix shows every operation contained
- **THEN** F16 SHALL become fixed: the `finding = "F16"` command SHALL become a `mutantPreFixF16…` mutant with `expect = "counterexample"`, the model SHALL check the fixed behaviour, the README row SHALL say "Fixed" with the run and job ids, the rapid property's F16 carve-out SHALL be removed, and the leg SHALL assert containment for every operation

### Requirement: The F16 leg is never a killer
Under every F16 outcome, `TestModulePathContainment_F16Junction` SHALL stay a characterisation test: its row's abstraction cell SHALL begin `characterisation:`, and the test SHALL NOT be a mutation or reach killer.

#### Scenario: Leg excluded from killers
- **WHEN** the formal-bindings plan is built
- **THEN** `TestModulePathContainment_F16Junction` SHALL NOT appear among any row's killers

### Requirement: Junctions stay out of the golden scope
The `ModulePathContainment` golden SHALL keep junctions fixed off. For that model, junctions SHALL be covered by the witness commands, the Windows junction branch of the rapid property, and the F16 leg. `formal/README.md` and the model's calibration text SHALL describe exactly that coverage. The `VirtualPathHarness` golden is not bound by this requirement: it carries junction vectors (see "Harness golden replays junction vectors on Windows").

#### Scenario: README coverage claim matches the tests
- **WHEN** `formal/README.md` describes how junctions are covered for `ModulePathContainment`
- **THEN** it SHALL name only the witness commands, the rapid property's junction branch on Windows, and the F16 leg, and each named piece SHALL exist

### Requirement: Custom-check reads are replayed through the real resolver
`TestModulePathContainment_GoldenVectors` SHALL replay `CustomCheckScript.ResolveWithFSAndModule` on every `ScriptRead` instance with a recording reader. It SHALL require the same acceptance and touched node as the model's verdict. The `readOp` row of that function SHALL name this test. The scenario named "Custom-check replay reproduces F13" keeps its name from the pre-fix spec and now asserts the fixed denial.

#### Scenario: Custom-check replay reproduces F13
- **WHEN** a golden `ScriptRead` instance reaches an `invowk_modules/` symlink that escapes the module
- **THEN** the custom-check replay SHALL deny the read and touch nothing, as the model's verdict does

### Requirement: Env-file reads are replayed through the real loader
`TestModulePathContainment_GoldenVectors` SHALL call `LoadEnvFile` with the module root on every `EnvRead` instance of a module that the real `Load` admits and whose path the real `ValidateEnvFilePath` accepts. The code's verdict SHALL come only from the loader's error class: an error matching `ErrEnvFileOutsideModule` means denied, and nil or any other error means accepted. The replay SHALL NOT compute physical containment itself, nor read `dPhysContains`, to decide that verdict; the instance tree SHALL be consulted only to select the expectation for the absolute pass's class. The verdict SHALL equal the model's acceptance for `EnvRead`, and on an accepted vector the touched node SHALL equal the model's. The scenario named "Env replay reproduces F17" keeps its name from the pre-fix spec and now asserts the fixed denial. The touched file SHALL be identified by a sentinel in its content, and a denied vector SHALL merge no variable.

#### Scenario: Env replay reproduces F17
- **WHEN** a golden `EnvRead` instance on an admitted module reads `invowk_modules/x`, and `x` is a symlink that escapes
- **THEN** the loader SHALL return an error matching `ErrEnvFileOutsideModule` and load no variable from the outside sentinel, as the model's verdict does

#### Scenario: Missing env target
- **WHEN** a golden `EnvRead` instance names a dangling link or a link cycle, so that no target is reached
- **THEN** the loader SHALL return an error matching `ErrEnvFileOutsideModule`, and the model SHALL deny the read

#### Scenario: Env target is a directory
- **WHEN** a golden `EnvRead` instance names a path that physically reaches a directory inside the module
- **THEN** the loader SHALL return an error that does not match `ErrEnvFileOutsideModule`, the replay SHALL count the read as accepted, as the model does, and the model's touched node SHALL be that directory

#### Scenario: Escape error from the contained read is not not-exist
- **WHEN** `os.Root`'s "path escapes from parent" error reaches `LoadEnvFile` from `Stat` or from `ReadFile`
- **THEN** the loader SHALL return an error matching `ErrEnvFileOutsideModule`, also for an optional `?` file

#### Scenario: Env calibration seed is caught by the replay
- **WHEN** the module branch of `LoadEnvFile` is seeded to bypass the root and read the joined path with `os.ReadFile`
- **THEN** the relative pass's env replay SHALL fail on the escaping-link vectors, and a seed that `root.ReadFile`'s own escape rejection masks, such as skipping only the `Stat` check, SHALL NOT be recorded as the env calibration

### Requirement: The container workdir row names a reaching binding
The `workdirCwd` row for `Invowkfile.GetEffectiveWorkDir` SHALL name `TestModulePathContainment_ContainerWorkDirGolden`, which calls that function, instead of `TestModulePathContainment_GoldenVectors`.

#### Scenario: Workdir row reaches its function
- **WHEN** the reach gate runs after the rebinding
- **THEN** the `GetEffectiveWorkDir` row SHALL be reached by `TestModulePathContainment_ContainerWorkDirGolden`

### Requirement: Virtual-harness construction rows are bound by a construction replay
`TestVirtualPathHarness_RootConstruction` SHALL bind the `allowedRoots`, `anchors`, `access` and workdir-rule rows. For each workdir kind it SHALL build the workdir input the way a command run does (from an invowkfile, a command, an implementation and a CLI override) and build the resolver with `newVirtualPathResolverForFilesystem`. Its acceptance of probes in each location kind (`InReadRoot`, `InWorkdir`, `OutsideAll`) SHALL equal `H.access = Full or locIsAllowedRoot[l, TrustedOrInside]`. `EffectiveAccess` SHALL map an omitted or `restricted` access to `restricted`, and `full` to `full`.

#### Scenario: Trusted workdir accepted
- **WHEN** the construction replay probes a workdir-only path under `restricted` access, with the workdir given as a CLI override or by an invowkfile outside a module
- **THEN** the resolver SHALL accept it

#### Scenario: Module workdir outside the module rejected
- **WHEN** the construction replay probes a workdir-only path under `restricted` access, with the workdir declared by a module invowkfile at the root, command or implementation level, outside the module by `..`, by an absolute path, or through a link
- **THEN** the resolver SHALL reject it, and SHALL accept it under `full` access

#### Scenario: Workdir widening reproduced
- **WHEN** the pre-fix root rule (the workdir and `@work` always added to the allowed roots) is seeded into `newVirtualPathResolverForFilesystem`
- **THEN** the construction replay SHALL fail on the module-outside probes, and the result SHALL be recorded in `VirtualPathHarness`'s calibration text

#### Scenario: Outside path rejected
- **WHEN** the construction replay probes a path outside every root under `restricted` access
- **THEN** the resolver SHALL reject it

#### Scenario: Omitted access is restricted
- **WHEN** the filesystem config omits the access mode
- **THEN** `EffectiveAccess` SHALL return `restricted`, as the virtual-filesystem-access spec states

### Requirement: The anchors row claims only explicit anchor roots
The construction replay SHALL require `@config`, `@data`, `@cache`, `@state` and `@tmp`, as the resolver reports them, to be accepted, `@work` to be accepted exactly when the workdir is an allowed root, and a path under `@home` outside every other root to be rejected under restricted access (`cross-platform-path-anchors` "Anchor allow roots are explicit"). The `anchors` row's abstraction cell SHALL say that anchor paths are inputs from the production resolver, not verified.

#### Scenario: Home anchor is not an allowed root
- **WHEN** the replay probes a path under `@home` that no other anchor or root encloses, under restricted or omitted access
- **THEN** the resolver SHALL reject it

#### Scenario: Work anchor follows the workdir rule
- **WHEN** the replay probes `@work` as the resolver reports it, under restricted access
- **THEN** the resolver SHALL accept it for a trusted workdir, and SHALL reject it for a module-declared workdir outside the module

#### Scenario: Seeded home root
- **WHEN** `@home` is seeded into the allowed roots of `newVirtualPathResolverForFilesystem`
- **THEN** the construction replay SHALL fail, and the result SHALL be recorded in `VirtualPathHarness`'s calibration text

#### Scenario: Abstraction cell lists what the replay catches
- **WHEN** the `anchors` row's abstraction cell is read
- **THEN** it SHALL list exactly the defect classes the replay catches: an empty anchor, a reported anchor missing from the allowed roots where no other root encloses it, an anchor that covers `@home/.ssh`, and `@work` admitted for a module-declared workdir outside the module
- **THEN** it SHALL name the masked cases: dropping `@work` is equivalent wherever the workdir itself is an allowed root, and dropping `@tmp` is masked off Windows while `TMPDIR` lies under `/tmp`

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

### Requirement: Script and env reads are accepted only under physical containment
The model's `accepts` SHALL accept a `ScriptRead` or `EnvRead` operation only when it validates, is lexically contained, is physically contained (`physContains`), and its module is admitted. The golden's `dAccepts` SHALL record that verdict, and `make formal-golden` SHALL regenerate the golden file after the edit.

#### Scenario: Vendored link escape is not accepted
- **WHEN** a golden instance's `ScriptRead` or `EnvRead` reaches an `invowk_modules/` link whose target is outside the module
- **THEN** `dAccepts` SHALL NOT contain the operation, and the real resolver and env loader SHALL deny it

#### Scenario: In-module link is accepted
- **WHEN** a golden instance's read reaches a link whose target lies inside the module
- **THEN** `dAccepts` SHALL contain the operation, and the real resolver and env loader SHALL read the target

### Requirement: The golden is replayed with relative and with absolute link targets
`TestModulePathContainment_GoldenVectors` SHALL replay the golden twice: a relative pass, which materialises each symlink with a target relative to the link's directory, and an absolute pass, which materialises each symlink with an absolute target on every instance that contains at least one link. Other replays that use `internal/testutil/fstree` SHALL keep their current link targets unless they opt in.

#### Scenario: Relative pass agrees with the model on every vector
- **WHEN** the relative pass replays every instance
- **THEN** the real script, custom-check and env read decisions and touched nodes SHALL equal `dAccepts` on every `ScriptRead` and `EnvRead` instance, with no instance excluded or reclassified

### Requirement: The absolute pass classifies in-module absolute-target links
In the absolute pass, the replay SHALL classify explicitly, from each instance's tree, the vectors in the class "in-module link with an absolute target": a `ScriptRead` or `EnvRead` whose reference reaches a link whose target lies inside the module, and which `dAccepts` accepts. The class SHALL apply to an operation kind only once that kind's read is physically contained in the code.

#### Scenario: Absolute pass denies the classified vectors
- **WHEN** the absolute pass replays an instance
- **THEN** on a vector in the class, the real read SHALL be denied and touch nothing
- **THEN** on every other vector, absolute-target escapes included, the real read decision and touched node SHALL equal `dAccepts`

#### Scenario: Empty class fails
- **WHEN** the absolute pass finds no vector in the class for an operation kind the class applies to
- **THEN** the replay SHALL fail, and the class size of each kind SHALL be recorded in the model's calibration text

### Requirement: Code stricter than the model is recorded
The correspondence rows of the contained reads SHALL state that the code is stricter than the model in one case: a link with an absolute target is denied even when the target is inside the module. The golden's absolute pass SHALL classify that case explicitly, and a dedicated Go test SHALL also pin the denial for the script read and the env read outside the golden scope.

#### Scenario: Absolute in-module link denied
- **WHEN** the dedicated test reads a module file through a symlink with an absolute target inside the same module
- **THEN** both the script resolver and `LoadEnvFile` in module context SHALL deny it, and the correspondence rows SHALL name that test and the golden test

### Requirement: Correspondence list entries are only removed
The change SHALL NOT add or rename entries in `[correspondence] unresolved_elements` or `[triage] binding_gaps` of `formal/manifest.toml`: both lists only shrink. An entry whose row is removed, or whose element now resolves in the model source or as a manifest command name, SHALL be removed in the same commit.

#### Scenario: Renamed finding rows drop their entries
- **WHEN** the `findingF13` or `findingF17` row's element cell names the renamed `mutantPreFix…` manifest command
- **THEN** its `ModulePathContainment: findingF13` or `ModulePathContainment: findingF17` entry SHALL be removed from `unresolved_elements`, and `scripts/test_formal.py` and `formal_mutation.py triage --check` SHALL pass

#### Scenario: New physContains row adds no entry
- **WHEN** the `physContains` row is added
- **THEN** no list entry SHALL be added for it, because `physContains` resolves in the model source

### Requirement: Fixed findings keep pre-fix regression mutants
F13 and F17 SHALL be recorded as fixed:
- each finding command SHALL become a regression mutant (`mutantPreFixF13…`, `mutantPreFixF17…`) with `expect = "counterexample"` and its `mutant_of`, without a `finding` key;
- each Go replay SHALL be inverted to assert the denial, and renamed to the fixed behaviour;
- each README Findings row SHALL say "Fixed" and name the pre-fix mutant.

#### Scenario: Pre-fix mutants still reproduce
- **WHEN** `make formal` runs
- **THEN** `scriptReadContainedPhysical` and `envReadContainedPhysical` SHALL pass, and each `mutantPreFix` command SHALL produce a counterexample on its own

#### Scenario: Inverted replays assert the denial
- **WHEN** the renamed F13 and F17 replays run on Linux, macOS and Windows
- **THEN** each SHALL assert that the read through the `invowk_modules/` link is denied and that the outside sentinel is not returned or loaded

### Requirement: F16 is reclassified after the contained read
After this change, the F16 leg's outcome for each operation SHALL be observed on windows-latest and reported with the run and job ids. F16 SHALL then be reclassified from those outcomes under "F16 is decided by one of the stated outcomes", in a commit made after that evidence, and the model note of `findingF16JunctionScan`, or the model, SHALL be updated so that model and code agree.

#### Scenario: Interim verdict before the evidence
- **WHEN** the F13 commit is in place and the post-evidence F16 commit is not
- **THEN** the leg's script-read arm SHALL assert containment, its recorded verdict SHALL be "containment pending windows-latest evidence", and the `findingF16JunctionScan` model note and the F16 calibration text SHALL say so, while the model still reports the escape for that arm

#### Scenario: Script read arm flips to contained
- **WHEN** the F16 leg runs on windows-latest with the contained read in place, and the script read through the inner junction is denied
- **THEN** the leg SHALL assert containment for that arm, and the model SHALL no longer report an escape for it

#### Scenario: Another arm still escapes
- **WHEN** any other arm of the leg still escapes on windows-latest
- **THEN** F16 SHALL stay recorded for that arm only, and the README row SHALL name the arms that remain open

### Requirement: Harness link kinds and dangling links are modelled
A harness request SHALL record the kind of link it crosses, if any (symlink or junction), and whether that link's target is missing (a dangling link, symlinks only, with at least one missing component). A request whose lexical and physical ancestors differ SHALL cross a link. A request MAY cross a link whose target stays in the same location. The model SHALL have a witness command for a junction ancestor, for a dangling link, and for a link that stays in its location.

#### Scenario: Witnesses exist
- **WHEN** `make formal` runs
- **THEN** the junction, dangling-link and in-location-link witness commands SHALL each find an instance

#### Scenario: Fixed evaluation ignores the link kind
- **WHEN** the fixed evaluation mode normalises a request
- **THEN** the normalised location SHALL be the physical ancestor's location for every link kind, every depth, and for dangling links

### Requirement: Fixed harness findings keep pre-fix regression mutants
F14, F15, F18 and F19 SHALL each keep a manifest command that reintroduces only that defect into the fixed configuration: `mutantPreFixF14ParentOnlyEval`, `mutantPreFixF15WorkdirRoot`, `mutantPreFixF18JunctionNotFollowed` and `mutantPreFixF19DanglingLinkAsMissingName`. Each SHALL have `expect = "counterexample"`, `mutant_of = "harnessContainedFixed"` and no `finding` key. No command with `finding = "F14"` or `finding = "F15"` SHALL remain.

#### Scenario: Each pre-fix mutant reproduces alone
- **WHEN** `make formal` runs
- **THEN** `harnessContainedFixed` SHALL pass, and each of the four pre-fix mutants SHALL produce a counterexample

#### Scenario: Each pre-fix evaluation differs from the fixed one in one defect
- **WHEN** the model's pre-fix evaluation modes are read
- **THEN** the F14 mode SHALL return the lexical ancestor only for a request with two or more missing components, the F18 mode only for a request that crosses a junction, and the F19 mode only for a dangling request, and each SHALL return the physical ancestor otherwise
- **THEN** the F15 mutant SHALL use the fixed evaluation and differ from `harnessContainedFixed` only in its root rule, which admits the workdir for every kind

#### Scenario: Stale list entry removed
- **WHEN** the `findingF14` and `findingF15` correspondence rows name the renamed manifest commands
- **THEN** the `VirtualPathHarness: findingF14` and `VirtualPathHarness: findingF15` entries SHALL be absent from `[correspondence] unresolved_elements`, and no entry SHALL be added for the F15, F18 and F19 rows

### Requirement: Harness golden replay compares with the fixed decision
`TestVirtualPathHarness_GoldenVectors` SHALL compare the real validator's verdict with the model's fixed-evaluation decision on every golden request, with no finding carve-out. It SHALL materialise a symlink request with a symlink, a dangling request with a symlink to a missing target, and a request with no link with a plain directory.

#### Scenario: Verdicts agree on Linux, macOS and Windows
- **WHEN** the harness golden is replayed on Linux, macOS and Windows
- **THEN** the real verdict SHALL equal the model's fixed decision on every replayed request, and a disagreement SHALL name the instance, the request, its link kind, its depth, and whether it dangles

#### Scenario: In-location links are replayed
- **WHEN** the golden contains a request that crosses a link whose target stays in an allowed location
- **THEN** the replay SHALL materialise that link and the validator SHALL accept the request

### Requirement: Harness golden replays junction vectors on Windows
The `VirtualPathHarness` golden SHALL contain requests whose link kind is a junction. On Windows the replay SHALL materialise them with real junctions and compare them like every other request. Off Windows they SHALL be skipped through the `fstree` capability policy, and the replay SHALL report how many were skipped.

#### Scenario: Junction vectors run on Windows
- **WHEN** the harness golden is replayed on windows-latest
- **THEN** every junction request SHALL be materialised and compared, and the replay SHALL fail if it replayed none

#### Scenario: Junction vectors are counted off Windows
- **WHEN** the harness golden is replayed on Linux or macOS
- **THEN** the junction requests SHALL be left out by the capability policy and their count SHALL be logged, and the test itself SHALL NOT be reported as skipped

### Requirement: Harness escape replays assert the denial
Three replays in `internal/runtime/virtual_path_harness_formal_test.go` SHALL assert the fixed behaviour, each named in the binding cell of its own correspondence row: `TestVirtualPathHarness_DeepMissingPathThroughSymlinkDenied` (F14), `TestVirtualPathHarness_JunctionEscapeDenied` (F18) and `TestVirtualPathHarness_DanglingLinkEscapeDenied` (F19). Each SHALL also show that nothing is created outside the allowed root. `TestVirtualPathHarness_F14DeepSymlink` SHALL no longer exist.

#### Scenario: Which replays are killers
- **WHEN** the formal-bindings plan is built
- **THEN** the F14 and F19 replays SHALL be killers of their rows, and the F18 replay, required on Windows and skipped elsewhere by the capability policy, SHALL NOT be one (see "The junction replay is never a killer")

#### Scenario: Deep path replay
- **WHEN** the F14 replay requests a path two missing components below an escaping symlink
- **THEN** the validator SHALL deny it, and the outside directory SHALL stay empty

#### Scenario: Junction replay
- **WHEN** the F18 replay runs on windows-latest with a junction to an outside directory
- **THEN** the validator SHALL deny an existing file below the junction, a new file directly below it, and a new file two missing components below it, and the outside directory SHALL hold only its sentinel

#### Scenario: Dangling link replay
- **WHEN** the F19 replay requests a symlink whose missing target lies in an outside directory
- **THEN** the validator SHALL deny it, and the target SHALL NOT exist afterwards

#### Scenario: Link target that climbs out of a missing directory
- **WHEN** the F19 replay requests a link `d` whose target is `nope/../in/new`, with `nope` missing and `in` a link to an outside directory, written as a relative and as an absolute target
- **THEN** the validator SHALL deny it, and the outside directory SHALL stay empty

#### Scenario: Evidence names the tests
- **WHEN** the change's pull request reports its Windows and macOS evidence
- **THEN** it SHALL quote the CI run id, the job id and the names of the replays that ran and passed in that job, and a replay reported as skipped SHALL NOT count

### Requirement: The junction replay is never a killer
`TestVirtualPathHarness_JunctionEscapeDenied` SHALL NOT be a mutation or reach killer, because it is skipped off Windows and a skipped killer fails the reach gate and the mutation run. Its correspondence row SHALL name the F18 mutant command and the walk helper, and its abstraction cell SHALL begin `characterisation:`. The test SHALL be named on no other row. The walk helper's row SHALL keep killers that run on Linux.

#### Scenario: Replay excluded from killers
- **WHEN** the formal-bindings plan is built
- **THEN** `TestVirtualPathHarness_JunctionEscapeDenied` SHALL NOT appear among any row's killers, and its row SHALL be reported as `characterisation-only`

#### Scenario: Gates pass on Linux
- **WHEN** `make formal-reach` runs on Linux
- **THEN** it SHALL pass, with the walk helper reached by `TestVirtualPathHarness_GoldenVectors` and by the fake-filesystem test, and with no killer reported as skipped

#### Scenario: Golden test is never reported as skipped
- **WHEN** the harness golden is replayed on Linux or macOS
- **THEN** `TestVirtualPathHarness_GoldenVectors` SHALL be reported as passed, with the junction requests left out inside the test and counted in its log

### Requirement: Harness rapid property has no finding exemption
`TestVirtualPathHarness_Property` SHALL fail on every accepted path that resolves outside the allowed root, with no exemption for a deep path through a link, on every accepted path that cannot be resolved, and on every denied path that resolves inside the root.

#### Scenario: Generated shapes
- **WHEN** the property generates its cases
- **THEN** they SHALL include links that escape and links that stay inside, with relative and with absolute targets, dangling links, link targets with `..` before and after a missing component, and zero to three missing components, and on Windows also junctions

#### Scenario: Escape is a failure at every depth
- **WHEN** the property generates a path through an escaping link with two or more missing components
- **THEN** an acceptance SHALL fail the property

#### Scenario: False denial is a failure
- **WHEN** the property generates a path that resolves inside the allowed root, through a link or not
- **THEN** a denial SHALL fail the property

#### Scenario: Unresolvable path is denied
- **WHEN** the property generates a path whose link target holds `..` after a missing component
- **THEN** an acceptance SHALL fail the property, and nothing SHALL exist outside the allowed root afterwards

### Requirement: Harness resolver is tested over a fake filesystem
The harness resolver SHALL read the filesystem through injected lookups, and a test SHALL drive it with a fake filesystem on every platform. That test SHALL cover what a Linux run cannot materialise: a junction-like entry, a reparse entry whose target cannot be read, a link target that is rooted but has no volume, a missing volume root, and a hop limit. It SHALL live in a `*_formal_test.go` file of its own, apart from the escape replays.

#### Scenario: Result invariant and `..` handling
- **WHEN** the fake-filesystem test checks a result of the resolver
- **THEN** it SHALL assert that the existing prefix holds no link and that the missing components are plain names
- **THEN** its cases SHALL include a `..` on an existing prefix (applied to the physical prefix), a `..` after a missing component (refused), and the report of a link whose target does not resolve

#### Scenario: Junction branch is killed on Linux
- **WHEN** the junction-following branch of the resolver is removed and the fake-filesystem test runs on Linux
- **THEN** the test SHALL fail

#### Scenario: Unwalked link after a missing component
- **WHEN** the fake filesystem holds `root/in`, a link to `outside`, and `root/d`, a link to `nope/../in/new` with `nope` missing, and the resolver is given `root/d`
- **THEN** it SHALL return an error, and SHALL NOT return `root/in/new`

### Requirement: Harness no-follow requests are a recorded abstraction
The `VirtualPathHarness` model SHALL keep one request kind, the path whose last component is followed. The no-follow form SHALL get its own correspondence row, whose abstraction cell says that the parent is resolved as a model request and the final name is joined without being read. A binding test SHALL compare its verdicts with the containment of the parent's physical location. The no-follow form SHALL add no request kind, model dimension, or golden field; the model and golden change only for other requirements, such as the workdir kind of "Virtual-runtime path harness model".

#### Scenario: No-follow binding test
- **WHEN** the binding test validates no-follow requests over a materialised tree (a live link, a dangling link, a link inside the root whose target is outside, a path whose parent crosses an escaping link, a final name equal to an allowed root's own path, and on Windows a junction, in a subtest so that its capability skip elsewhere does not skip the killer)
- **THEN** each verdict SHALL equal containment of the parent's physical location joined with the final name, as computed by the test from how it built the tree, and the test SHALL pass on Linux, macOS and Windows

#### Scenario: Golden unchanged
- **WHEN** `make formal-golden` runs
- **THEN** the no-follow form SHALL leave the committed `VirtualPathHarness` golden unchanged: it SHALL hold no no-follow request kind or field, and any other change to it SHALL come from another requirement, such as the workdir kind

#### Scenario: No-follow seed
- **WHEN** the no-follow resolver is seeded to resolve the final name too
- **THEN** the binding test SHALL fail, and the result SHALL be recorded in `VirtualPathHarness`'s calibration text

### Requirement: Harness walk helpers are accounted for
Each of these functions SHALL be accounted for in the `VirtualPathHarness` correspondence table: `virtualPathWalker.walk`, `.next`, `.enterVolume`, `.follow` and `.unresolvedLink`, `virtualPathWalk.path`, `missingVirtualPathSuffix`, `isVirtualPathLink`, `normalizeVirtualPathForAccess`, and the no-follow resolver and validator entry. The `normalize` row's element SHALL be `normalized`, and `VirtualPathHarness: normalize` SHALL leave `[correspondence] unresolved_elements`.

#### Scenario: Reach gate covers the helpers
- **WHEN** `make formal-reach` runs on Linux
- **THEN** it SHALL pass, every helper with a row of its own SHALL be reached by each killer its row names, and no killer SHALL be reported as skipped

#### Scenario: Helper that a Linux killer reaches
- **WHEN** some Linux killer reaches one of the named helpers
- **THEN** that helper SHALL be named on a correspondence row of its own, whose binding cell names only killers that reach it on Linux

#### Scenario: Helper that no Linux killer reaches
- **WHEN** the reach gate shows that no Linux killer reaches one of the named helpers
- **THEN** that helper SHALL have no row of its own, and the `normalized` row's abstraction cell SHALL name it as calibrated by hand seeds only

#### Scenario: Lexical preparation is not bound
- **WHEN** the rows are reviewed for the functions between a request and its decision
- **THEN** anchor expansion, making a request absolute, cleaning and splitting it, and `virtualPathResolver.resolve` (which only sequences expansion and the walk) SHALL be left out, because a model request is already an absolute, cleaned, expanded path

#### Scenario: Newly bound helpers are calibrated
- **WHEN** a helper gets a correspondence row of its own for the first time
- **THEN** the calibration text SHALL name a seeded defect in that helper and the binding that fails it, either an existing walk seed that sits in it or a new seed run and reverted for it; a helper named only in the `normalized` row's abstraction cell SHALL be listed there as calibrated by hand seeds only

#### Scenario: Unresolved element removed
- **WHEN** `python3 scripts/formal.py correspondence` and `scripts/test_formal.py` run after the rename
- **THEN** both SHALL pass with no `VirtualPathHarness: normalize` entry in `unresolved_elements`, and no entry SHALL be added

### Requirement: Trusted workdirs stay readable in the harness model
The model SHALL check that the fixed root rule accepts, under `restricted` access, every request that resolves physically inside a trusted workdir, so that the narrow fix cannot become the strict one (no workdir is ever a root) unnoticed. A mutant whose root rule never admits the workdir SHALL fail that check.

#### Scenario: Completeness check passes
- **WHEN** `make formal` runs `trustedWorkdirReadableFixed`
- **THEN** it SHALL pass

#### Scenario: Strict rule is caught
- **WHEN** `make formal` runs `mutantStrictWorkdirNeverRoot`, the same check with a root rule that never admits the workdir
- **THEN** it SHALL produce a counterexample with a trusted workdir

### Requirement: The F15 replay asserts the denial
`TestVirtualPathHarness_F15WorkdirWidening` SHALL be inverted and renamed `TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot`, and SHALL assert that a module-declared workdir at the filesystem root (the volume root on Windows) is not an allowed root under `restricted` access. Its correspondence row SHALL name `mutantPreFixF15WorkdirRoot`, and it SHALL be a killer of that row.

#### Scenario: Replay denies the outside read
- **WHEN** the replay runs on Linux, macOS and Windows
- **THEN** the resolver's allowed roots SHALL hold neither that workdir nor `@work`, and a read of a path below it that no other root encloses SHALL be denied with `virtual path denied`

#### Scenario: CLI override of the same directory
- **WHEN** the replay gives the same directory as a CLI override
- **THEN** the same read SHALL be accepted

### Requirement: Harness golden replays every workdir kind
The harness golden SHALL vary the workdir kind and record the fixed root rule's decision. The golden replay SHALL derive, for each instance, whether the workdir directory it builds is an allowed root by calling the production workdir-root decision on a workdir input of that kind (trusted, module-declared inside the module, module-declared outside the module), and SHALL compare the real verdict with the model's fixed decision on every request.

#### Scenario: Outside module workdir in the golden
- **WHEN** a golden instance has a module-outside workdir and a request that resolves inside the workdir only
- **THEN** the model's decision and the real validator SHALL both deny it

#### Scenario: Trusted workdir in the golden
- **WHEN** a golden instance has a trusted workdir and a request that resolves inside the workdir only
- **THEN** the model's decision and the real validator SHALL both accept it

#### Scenario: Module-inside workdir in the golden
- **WHEN** a golden instance has a module-inside workdir
- **THEN** the replay SHALL build its workdir input with the workdir below the instance's read root and the module root at that read root, and the production decision SHALL report the workdir as an allowed root

### Requirement: Harness rapid property covers workdir placement
`TestVirtualPathHarness_Property` SHALL also generate a module directory, a workdir source and a workdir placement, and compare the production workdir-root decision, and the validator's verdict on a request below the workdir, with an oracle computed from how the test built the tree.

#### Scenario: Generated workdir shapes
- **WHEN** the property generates a case
- **THEN** the source SHALL be one of CLI override, invowkfile outside a module, module default and module-declared, and the placement one of inside the module, outside by `..`, an absolute outside path, and a link (on Windows also a junction) that leaves or stays in the module

#### Scenario: Oracle in both directions
- **WHEN** the property generates a case
- **THEN** a request below a trusted or module-inside workdir that the validator denies SHALL fail the property, and a request below a module-outside workdir, with no other root enclosing it, that the validator accepts SHALL fail it
