# Spec Delta

## MODIFIED Requirements

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

### Requirement: Findings recording
Findings SHALL use fixed ids. F8–F12 are reserved by sibling changes, and the model-module-path-containment change owns F13 onward:
- F13 is the `invowk_modules/` symlink hole in script reads, fixed by module-read-physical-containment;
- F14 is the virtual-runtime `mkdir`-through-symlink fallback, fixed by virtual-path-physical-normalisation;
- F15 is the module-declared `workdir` widening the virtual roots under `restricted` access;
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

### Requirement: Junctions stay out of the golden scope
The `ModulePathContainment` golden SHALL keep junctions fixed off. For that model, junctions SHALL be covered by the witness commands, the Windows junction branch of the rapid property, and the F16 leg. `formal/README.md` and the model's calibration text SHALL describe exactly that coverage. The `VirtualPathHarness` golden is not bound by this requirement: it carries junction vectors (see "Harness golden replays junction vectors on Windows").

#### Scenario: README coverage claim matches the tests
- **WHEN** `formal/README.md` describes how junctions are covered for `ModulePathContainment`
- **THEN** it SHALL name only the witness commands, the rapid property's junction branch on Windows, and the F16 leg, and each named piece SHALL exist

## ADDED Requirements

### Requirement: Harness link kinds and dangling links are modelled
A harness request SHALL record the kind of link it crosses, if any (symlink or junction), and whether that link's target is missing (a dangling link, symlinks only, with at least one missing component). A request whose lexical and physical ancestors differ SHALL cross a link. A request MAY cross a link whose target stays in the same location. The model SHALL have a witness command for a junction ancestor, for a dangling link, and for a link that stays in its location.

#### Scenario: Witnesses exist
- **WHEN** `make formal` runs
- **THEN** the junction, dangling-link and in-location-link witness commands SHALL each find an instance

#### Scenario: Fixed evaluation ignores the link kind
- **WHEN** the fixed evaluation mode normalises a request
- **THEN** the normalised location SHALL be the physical ancestor's location for every link kind, every depth, and for dangling links

### Requirement: Fixed harness findings keep pre-fix regression mutants
F14, F18 and F19 SHALL each keep a manifest command that reintroduces only that defect into the fixed evaluation: `mutantPreFixF14ParentOnlyEval`, `mutantPreFixF18JunctionNotFollowed` and `mutantPreFixF19DanglingLinkAsMissingName`. Each SHALL have `expect = "counterexample"`, `mutant_of = "harnessContainedFixed"` and no `finding` key. No command with `finding = "F14"` SHALL remain.

#### Scenario: Each pre-fix mutant reproduces alone
- **WHEN** `make formal` runs
- **THEN** `harnessContainedFixed` SHALL pass, and each of the three pre-fix mutants SHALL produce a counterexample

#### Scenario: Each pre-fix evaluation differs from the fixed one in one defect
- **WHEN** the model's pre-fix evaluation modes are read
- **THEN** the F14 mode SHALL return the lexical ancestor only for a request with two or more missing components, the F18 mode only for a request that crosses a junction, and the F19 mode only for a dangling request, and each SHALL return the physical ancestor otherwise

#### Scenario: Stale list entry removed
- **WHEN** the `findingF14` correspondence row names the renamed manifest command
- **THEN** the `VirtualPathHarness: findingF14` entry SHALL be removed from `[correspondence] unresolved_elements`, and no entry SHALL be added for the F18 and F19 rows

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
