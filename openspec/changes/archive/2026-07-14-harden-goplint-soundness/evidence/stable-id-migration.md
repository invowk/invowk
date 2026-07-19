# Stable Finding ID Migration Review

Date: 2026-07-14

## Result

| Set | Count |
|---|---:|
| Accepted IDs before canonical migration | 0 |
| Accepted IDs after canonical migration | 43 |
| Retained old IDs | 0 |
| Removed old IDs | 0 |
| Added accepted IDs | 43 |

There is no old-to-new ID mapping because the prior accepted baseline was
empty. The canonical production scan now records 43 newly accepted stable IDs:
26 unvalidated casts, 12 inconclusive cast paths, one missing constructor
validation, and four inconclusive constructor paths.

## Rejected migration candidates

The first flagless canonical scan produced ten candidates. They were not added
to the baseline:

- five constructor inconclusive findings exposed an IFDS finalization bug:
  unresolved option/helper calls before a later checked validation were being
  finalized immediately instead of carrying uncertainty until validation;
- three cast findings inside loops exposed missing `continue` handling for a
  terminating validation-failure branch;
- two provision-environment casts used a non-terminating fallback assignment;
  they were refactored through a checked helper with an explicit error return.

The current review additionally fixed non-nil error-return recognition so
validated casts are not rejected merely because their enclosing function has
typed results. Repository scans were restricted to production packages to
avoid duplicate augmented test-package analysis and timeout-only IDs; analyzer
fixtures remain covered by the dedicated goplint tests.

After those analyzer fixes and focused tests, the accepted production findings
were regenerated only after the semantic catalog, independent oracle, fuzz
seeds, refinement, determinism, all targeted mutants, and benchmark checks
passed. The blocking CI scan uses this reviewed baseline and still fails for
new IDs and for categories whose policy is always blocking.
