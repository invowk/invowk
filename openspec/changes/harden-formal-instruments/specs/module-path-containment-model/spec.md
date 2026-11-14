## ADDED Requirements

### Requirement: Junction creation is diagnosable and verified
On Windows, `internal/testutil/fstree` SHALL create directory junctions that a functional check confirms:
- the link is not reported as `ModeSymlink`;
- a file read through the link is `os.SameFile` with the file in the target directory.

A Windows-only test SHALL require junction creation and report the creation error.

Junction-writer failures SHALL be investigated in this order:
1. surface the error;
2. compare the mount-point reparse buffer with a known-good writer (Go's own Windows `os` test helper);
3. fall back to `cmd /c mklink /J`.

When the fallback also fails, both errors SHALL be reported.

#### Scenario: Junction writer works on Windows CI
- **WHEN** the Windows-only junction test runs on windows-latest
- **THEN** it SHALL pass the functional check, or fail and name the creation error(s)

#### Scenario: Off Windows the writer is not compiled
- **WHEN** `internal/testutil/fstree` is built for Linux or macOS
- **THEN** junction creation SHALL return the "junctions are supported only on Windows" error, and the policy SHALL treat junctions as not required

### Requirement: The rapid property generates junctions on Windows
`TestModulePathContainment_Property` SHALL draw the link kind from {symlink, junction} wherever junctions are available. On Windows they are required, so the junction branch SHALL run there.

A junction SHALL target a directory, either inside the module or outside it with a unique sentinel file. Escape SHALL be judged by file identity of the opened file against the outside sentinel (`os.SameFile`, backed by content). It SHALL NOT be judged by `filepath.EvalSymlinks` followed by a path-prefix test, which may not evaluate mount points on Go 1.23 and later.

An escape under `invowk_modules/` SHALL remain admissible as the recorded F13 hole for either link kind. A non-vendored junction escape SHALL fail the property unless F16 has been recorded as a finding.

#### Scenario: Junction branch is exercised
- **WHEN** the property runs on Windows
- **THEN** generated cases SHALL include junction links, both escaping and contained

#### Scenario: Junction escape is detected without EvalSymlinks
- **WHEN** a script read through a non-vendored junction opens the outside sentinel
- **THEN** the property SHALL report a policy violation, unless F16 is a recorded finding

### Requirement: F16 is decided with pre-stated outcomes
`TestModulePathContainment_F16Junction` SHALL require junctions through the capability policy. For a module with an inner junction to an outside directory, it SHALL assert containment for each operation production runs for that module:
- `IsModule` and `Load` admission;
- the script read through the junction;
- `modulecache.copyDir` and `provision.CopyDir`, judged by whether the copy contains the outside sentinel;
- `computeModuleHash`, judged by whether the hash changes when only the outside sentinel changes.

The leg SHALL assert. It SHALL NOT only log its verdict.

The Windows CI run of the change SHALL decide one of three outcomes, and the outcome SHALL be recorded with the run id in `ModulePathContainment`'s calibration text:
- **(a)** Junction creation still fails: the leg and the CI job fail, investigation continues, and the change does not complete.
- **(b)** Any operation escapes: F16 is confirmed and recorded through the finding procedure. The procedure comprises:
  - a `finding = "F16"` command in `formal/manifest.toml` next to a passing fix configuration;
  - a `formal/README.md` Findings row replacing the "unconfirmed" paragraph;
  - the leg kept as a characterisation replay that asserts the escape;
  - an F16 carve-out in the rapid property that mirrors F13.

  It SHALL NOT be fixed in this change.
- **(c)** Every operation is contained: the F16 hypothesis is recorded as refuted in `formal/README.md`, and the leg keeps asserting containment.

The leg SHALL stay a characterisation test (a probe of today's behaviour) under every outcome, and SHALL NOT be a mutation killer.

#### Scenario: Escape confirmed
- **WHEN** the leg observes an escape on unmodified code on Windows CI
- **THEN** F16 SHALL be recorded as a finding with a manifest `finding` command, a README row, and a characterisation replay, and the failing containment assertion SHALL NOT be committed

#### Scenario: Containment confirmed
- **WHEN** every operation stays inside the module on Windows CI
- **THEN** README SHALL list F16 among the refuted hypotheses, and the leg SHALL keep asserting containment

#### Scenario: Junctions unavailable on Windows CI
- **WHEN** the leg cannot create a junction on Windows CI
- **THEN** the leg SHALL fail with the probe error, and SHALL NOT be skipped

### Requirement: Junctions stay out of the golden scope
The `ModulePathContainment` golden SHALL keep junctions fixed off. Junctions SHALL be covered by the witness commands, the Windows junction branch of the rapid property, and the F16 leg. `formal/README.md` SHALL describe exactly that coverage.

#### Scenario: README coverage claim matches the tests
- **WHEN** `formal/README.md` describes how junctions are covered
- **THEN** it SHALL name only the witness commands, the rapid property's junction branch on Windows, and the F16 leg, and each named piece SHALL exist

### Requirement: Path-containment rows are bound by real-function replays
Each row below SHALL be bound by a test that calls the named function and compares it with the model's current-code verdict, including open findings:
- `CustomCheckScript.ResolveWithFSAndModule`: `TestModulePathContainment_GoldenVectors` SHALL replay the custom-check resolver on every `ScriptRead` instance, with a recording reader, and SHALL require the same acceptance and touched node as the model, including F13 instances.
- `LoadEnvFile`: `TestModulePathContainment_GoldenVectors` SHALL call `LoadEnvFile` on every `EnvRead` instance of an admitted module whose path `ValidateEnvFilePath` accepts. It SHALL require the decisions the golden records for `EnvRead` (acceptance and touched node, as `accepts` defines them for env reads), including F17 instances, with the touched file identified by a sentinel in the file's content.
- `validateDestinationPath`: `TestModulePathContainment_GoldenVectors` SHALL call it with the `packaging.go` call shape on every `Unpack` instance, and compare it with the decision the golden records for `Unpack` as the model defines it (`unpackInside[destCheck]`, `accepts`). The row's abstraction cell SHALL state any call-shape difference between the model's reference shapes and the real call.
- `Invowkfile.GetEffectiveWorkDir`: the row SHALL name `TestModulePathContainment_ContainerWorkDirGolden`, which calls it.
- `newVirtualPathResolverForFilesystem`, `standardVirtualAnchorsForOS`, and `VirtualFilesystemConfig.EffectiveAccess`: a new `TestVirtualPathHarness_RootConstruction` SHALL compare the constructed tagged roots, the anchors for `linux`, `darwin`, and `windows` (with and without environment overrides), and the access mode with an independent transcription of the model's construction, with the workdir included as the current code does (F15).

#### Scenario: Custom-check replay reproduces F13
- **WHEN** a golden `ScriptRead` instance reaches an `invowk_modules/` symlink that escapes the module
- **THEN** the custom-check replay SHALL accept and touch the outside node, as the model's current-code verdict does

#### Scenario: Env replay reproduces F17
- **WHEN** a golden `EnvRead` instance on an admitted module reads `invowk_modules/x`, and `x` is a symlink that escapes
- **THEN** `LoadEnvFile` SHALL load the outside sentinel, as the model's current-code verdict does

#### Scenario: Seeded defect in a rebound function
- **WHEN** a plausible defect is seeded in one rebound function, for example dropping the `..` check in `validateDestinationPath`
- **THEN** its new binding SHALL fail, and the result SHALL be recorded in the model's calibration text
