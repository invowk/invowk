## MODIFIED Requirements

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

### Requirement: Cost budgets
The change SHALL record measured cost budgets:
- the golden instance count and decoded size of each model, measured with a capped `-r` before the scope is fixed;
- the golden replay time in `make test`: 30 s or less per OS on Linux and macOS, and on Windows a budget derived from three CI runs;
- the deep rapid time, 60 s or less.

The golden replay time SHALL be measured on all three operating systems, and the deep rapid time on Linux, the only OS whose CI runs `make formal-rapid-deep`. CI figures SHALL state their test flags (`-race -short` replays every 16th instance). The budgets SHALL be handed to `promote-formal-ci-gate` as `budget_seconds` values.

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

## ADDED Requirements

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
