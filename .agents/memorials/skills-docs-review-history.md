# Review-tests and review-docs skills history

Historical notes moved out of the `review-tests` and `review-docs` skills on 2026-11-14. None of
this is normative; the current skills win.

- **Retired review-tests checks T8-C06 and T8-C07.** SS8 once covered `tools/goplint/**/*_test.go`
  with two WARNING items: T8-C06 (goplint tests use per-test analyzer instances, not shared
  process-wide state) and T8-C07 (semaphore tokens in goplint test helpers are released via
  `defer` in the same call, not `t.Cleanup`). Both rows and the `tools/goplint` file scope were
  removed when goplint moved to the standalone `github.com/invowk/goplint` repository
  (3181129b, #132, 2026-08-26). The surviving T8 IDs were kept stable rather than renumbered,
  because registries and the promotion table cite IDs. The skill description and totals kept
  saying "102 checklist items" (SS8 = 12) until this cleanup; the live total is 100 (SS8 = 10).
  The same-call `defer` rule for multi-acquire semaphore helpers survives in
  `.agents/rules/testing.md` and in review-tests check T2-C10.
- **CUE snippet `platforms` bulk fix.** Missing `platforms` in snippet implementation blocks was
  the most common CUE drift. One bulk fix applied 35 corrections in
  `website/src/components/Snippet/data/dependencies.ts`, 10 in `core-concepts.ts` and 5 in
  `advanced.ts`. The operative hint (where it concentrates, and that new snippets reintroduce
  it) stays in `review-docs/references/cue-drift-patterns.md` Pattern 1.
