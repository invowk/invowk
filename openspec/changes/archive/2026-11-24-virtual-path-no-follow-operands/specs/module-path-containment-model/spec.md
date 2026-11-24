## ADDED Requirements

### Requirement: Harness no-follow requests are a recorded abstraction
The `VirtualPathHarness` model SHALL keep one request kind, the path whose last component is followed. The no-follow form SHALL get its own correspondence row, whose abstraction cell says that the parent is resolved as a model request and the final name is joined without being read. A binding test SHALL compare its verdicts with the containment of the parent's physical location. The model and golden SHALL NOT change.

#### Scenario: No-follow binding test
- **WHEN** the binding test validates no-follow requests over a materialised tree (a live link, a dangling link, a link inside the root whose target is outside, a path whose parent crosses an escaping link, a final name equal to an allowed root's own path, and on Windows a junction, in a subtest so that its capability skip elsewhere does not skip the killer)
- **THEN** each verdict SHALL equal containment of the parent's physical location joined with the final name, as computed by the test from how it built the tree, and the test SHALL pass on Linux, macOS and Windows

#### Scenario: Golden unchanged
- **WHEN** `make formal-golden` runs after this change
- **THEN** the committed `VirtualPathHarness` golden SHALL be unchanged

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
