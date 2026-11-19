## ADDED Requirements

### Requirement: Script and env reads are accepted only under physical containment
The model's `accepts` SHALL accept a `ScriptRead` or `EnvRead` operation only when it validates, is lexically contained, is physically contained (`physContains`), and its module is admitted. The golden's `dAccepts` SHALL record that verdict, and `make formal-golden` SHALL regenerate the golden file after the edit.

#### Scenario: Vendored link escape is not accepted
- **WHEN** a golden instance's `ScriptRead` or `EnvRead` reaches an `invowk_modules/` link whose target is outside the module
- **THEN** `dAccepts` SHALL NOT contain the operation, and the real resolver and env loader SHALL deny it

#### Scenario: In-module link is accepted
- **WHEN** a golden instance's read reaches a link whose target lies inside the module
- **THEN** `dAccepts` SHALL contain the operation, and the real resolver and env loader SHALL read the target

### Requirement: Golden links are materialised with relative targets
The `ModulePathContainment` golden replay SHALL materialise each symlink with a target relative to the link's directory, so that a contained read follows an in-module link exactly as `physContains` does. Other replays that use `internal/testutil/fstree` SHALL keep their current link targets unless they opt in.

#### Scenario: Golden verdicts agree with the code on every vector
- **WHEN** `TestModulePathContainment_GoldenVectors` replays every instance
- **THEN** the real script, custom-check and env read decisions SHALL equal `dAccepts` on every `ScriptRead` and `EnvRead` instance, with no instance excluded or reclassified

### Requirement: Code stricter than the model is recorded
The correspondence rows of the contained reads SHALL state that the code is stricter than the model in one case: a link with an absolute target is denied even when the target is inside the module. A dedicated Go test SHALL pin that denial for the script read and the env read.

#### Scenario: Absolute in-module link denied
- **WHEN** the dedicated test reads a module file through a symlink with an absolute target inside the same module
- **THEN** both the script resolver and `LoadEnvFile` in module context SHALL deny it, and the correspondence rows SHALL name that test

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
After this change, the F16 leg's outcome for each operation SHALL be observed on windows-latest and reported with the run and job ids. F16 SHALL then be reclassified from those outcomes under "F16 is decided by one of three stated outcomes", in a commit made after that evidence, and the model note of `findingF16JunctionScan`, or the model, SHALL be updated so that model and code agree.

#### Scenario: Script read arm flips to contained
- **WHEN** the F16 leg runs on windows-latest with the contained read in place, and the script read through the inner junction is denied
- **THEN** the leg SHALL assert containment for that arm, and the model SHALL no longer report an escape for it

#### Scenario: Another arm still escapes
- **WHEN** any other arm of the leg still escapes on windows-latest
- **THEN** F16 SHALL stay recorded for that arm only, and the README row SHALL name the arms that remain open

## MODIFIED Requirements

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

### Requirement: Findings recording
Findings SHALL use fixed ids. F8–F12 are reserved by sibling changes, and the model-module-path-containment change owns F13 onward:
- F13 is the `invowk_modules/` symlink hole in script reads, fixed by module-read-physical-containment;
- F14 is the virtual-runtime `mkdir`-through-symlink fallback;
- F15 is the module-declared `workdir` widening the virtual roots under `restricted` access;
- F16 onward cover further discrepancies; F17 is the same hole in env-file reads, also fixed.

#### Scenario: Vendored-subtree symlink escape
- **WHEN** a module admitted by `Load` has a symlink under `invowk_modules/` to a file outside the module, and its `script.file` names that symlink
- **THEN** the pre-fix predicate, lexical containment against the raw module path, SHALL accept the read while it escapes, and `mutantPreFixF13…` SHALL produce a counterexample. The fixed configuration, containment against the physical resolution, SHALL pass. The Go replay SHALL show `ResolveScriptWithFSAndModule` denying the read

### Requirement: Link escapes are judged by file identity
The rapid property SHALL judge escape, for every link kind, by file identity of the opened file against the outside sentinel (`os.SameFile`, backed by a content comparison). It SHALL NOT judge escape by `filepath.EvalSymlinks` followed by a path-prefix test. An escape under `invowk_modules/` SHALL fail the property for either link kind. A non-vendored junction escape SHALL fail the property unless F16 is a recorded finding for the script read. The scenario named "Vendored junction escape is F13" keeps its pre-fix name and now asserts a violation.

#### Scenario: Junction escape is detected without EvalSymlinks
- **WHEN** a script read through a non-vendored junction opens the outside sentinel
- **THEN** the property SHALL report a policy violation, unless F16 is a recorded finding for the script read

#### Scenario: Vendored junction escape is F13
- **WHEN** a script read through a symlink or junction under `invowk_modules/` opens the outside sentinel
- **THEN** the property SHALL report a policy violation

### Requirement: The F16 leg asserts each operation's verdict
`TestModulePathContainment_F16Junction` SHALL require junctions through the capability policy. For a module with an inner junction to an outside directory, it SHALL assert, never only log, a verdict for each operation production runs: `IsModule` and `Load` admission, the script read through the junction, `modulecache.copyDir`, `provision.CopyDir`, and `computeModuleHash`. Each verdict is the one observed on windows-latest for the current code and recorded with its run id; with the contained read in place, the script read's expected verdict is containment.

#### Scenario: Copy and hash containment
- **WHEN** the leg checks the copies and the hash
- **THEN** each copy and both hash computations SHALL succeed, and an error SHALL fail the leg
- **THEN** a copy SHALL count as escaping if it contains the outside sentinel, and the hash SHALL count as escaping if it changes when only the outside sentinel changes

#### Scenario: Leg asserts the recorded verdicts
- **WHEN** F16 is recorded under outcome (b)
- **THEN** the leg SHALL assert the escape for each operation the outcome records as escaping and containment for every other operation
- **THEN** the leg SHALL fail when any operation's behaviour differs from its recorded verdict

#### Scenario: Script read through the junction is contained
- **WHEN** the leg reads `j/secret` through the inner junction with the contained read in place
- **THEN** the resolver SHALL deny the read, and the leg SHALL assert that it does not return the outside sentinel

#### Scenario: Junctions unavailable on Windows CI
- **WHEN** the leg cannot create a junction on Windows CI
- **THEN** the leg SHALL fail with the probe error, and SHALL NOT be skipped

### Requirement: Custom-check reads are replayed through the real resolver
`TestModulePathContainment_GoldenVectors` SHALL replay `CustomCheckScript.ResolveWithFSAndModule` on every `ScriptRead` instance with a recording reader. It SHALL require the same acceptance and touched node as the model's verdict. The `readOp` row of that function SHALL name this test. The scenario named "Custom-check replay reproduces F13" keeps its name from the pre-fix spec and now asserts the fixed denial.

#### Scenario: Custom-check replay reproduces F13
- **WHEN** a golden `ScriptRead` instance reaches an `invowk_modules/` symlink that escapes the module
- **THEN** the custom-check replay SHALL deny the read and touch nothing, as the model's verdict does

### Requirement: Env-file reads are replayed through the real loader
`TestModulePathContainment_GoldenVectors` SHALL call `LoadEnvFile` with the module root on every `EnvRead` instance of an admitted module whose path `ValidateEnvFilePath` accepts. It SHALL require the model's acceptance and touched node for `EnvRead`. The scenario named "Env replay reproduces F17" keeps its name from the pre-fix spec and now asserts the fixed denial. The touched file SHALL be identified by a sentinel in its content, and a non-file or missing target SHALL make the loader return an error.

#### Scenario: Env replay reproduces F17
- **WHEN** a golden `EnvRead` instance on an admitted module reads `invowk_modules/x`, and `x` is a symlink that escapes
- **THEN** the loader SHALL return a containment error and load no variable from the outside sentinel, as the model's verdict does

#### Scenario: Missing env target
- **WHEN** a golden `EnvRead` instance names a dangling link or a directory
- **THEN** the loader SHALL return an error

### Requirement: F16 is decided by one of three stated outcomes
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
