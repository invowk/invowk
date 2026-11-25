# Spec Delta

## MODIFIED Requirements

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

### Requirement: Harness workdir admission
Under `restricted` access a module-declared `workdir` that resolves physically outside the module SHALL be admitted only as a working directory (`allowedWorkdirCwd`), never as an allowed root. A trusted workdir SHALL be an allowed root (`allowedTrustedWorkdir`). The fixed root rule SHALL admit the workdir exactly when it is trusted or module-inside.

#### Scenario: Fixed rule keeps the outside workdir out
- **WHEN** `make formal` checks `harnessContained` with the fixed evaluation and the fixed root rule
- **THEN** the check SHALL pass for every workdir kind

#### Scenario: Workdir widening
- **WHEN** `make formal` checks `harnessContained` with the fixed evaluation and the pre-fix rule that always admits the workdir
- **THEN** the check SHALL produce a counterexample with a module-outside workdir, recorded as `mutantPreFixF15WorkdirRoot`, the pre-fix mutant of F15

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

## ADDED Requirements

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

### Requirement: Harness rapid property covers workdir placement
`TestVirtualPathHarness_Property` SHALL also generate a module directory, a workdir source and a workdir placement, and compare the production workdir-root decision, and the validator's verdict on a request below the workdir, with an oracle computed from how the test built the tree.

#### Scenario: Generated workdir shapes
- **WHEN** the property generates a case
- **THEN** the source SHALL be one of CLI override, invowkfile outside a module, module default and module-declared, and the placement one of inside the module, outside by `..`, an absolute outside path, and a link (on Windows also a junction) that leaves or stays in the module

#### Scenario: Oracle in both directions
- **WHEN** the property generates a case
- **THEN** a request below a trusted or module-inside workdir that the validator denies SHALL fail the property, and a request below a module-outside workdir, with no other root enclosing it, that the validator accepts SHALL fail it
