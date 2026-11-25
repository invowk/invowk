---
name: formal-verification
description: >-
  Invowk formal verification: Alloy 6 and TLA+/TLC models in formal/ (*.als,
  *.tla), scripts/formal.py, formal/manifest.toml, golden vectors
  (alloygolden, `make formal-golden`), rapid property tests, trace validation
  (tlatrace), calibration, model-to-code correspondence tables, formal-bindings
  mutation survivors. Use when adding or changing a model, when a
  golden-vector, property or trace test fails or a golden is stale, when code
  a correspondence table names changes, or when `make formal` or
  `make mutation-formal` fails.
---

# Formal Verification

Models are bounded checks bound to the real Go code by tests. Read
`formal/README.md` for the trust rules and the decision record before adding a
tool or a model.

## Workflow

1. **Model the guarantee where it lives.** Include the code that builds the
   inputs, not only the function that queries them. Discovery or parser
   guarantees become named facts in predicates, so a mutant can drop one.
2. **Write intent independently.** Keep the implementation transcription
   (`allowedImpl`, `diagKeys`) separate from the policy (`intendedAllowed`,
   `closure`). A check that relates a function to itself proves nothing.
3. **Declare every command in `formal/manifest.toml`**, never in the `.als`:
   a `[[model.command]]` with `name`, `body` (the formula), `expect`, and an
   optional `scope` overriding the model's default `scope`. The runner renders
   `name: run|check { body } for scope expect 0|1` into
   `artifacts/formal/<Model>/<Model>.als` and fails on a `run` or `check` left
   in the source. Unknown keys (a `scop =` typo) and keys for the other tool
   (`body` on TLC, `constants` on Alloy) are rejected; the full command key set
   is `COMMAND_KEYS` in `scripts/formal.py` (`property`, `antecedent`,
   `witness`, `finding`, `fairness_twin_of`, `dead_actions`,
   `distinct_states`, …).
4. **Guard every safety check:**
   - an antecedent `run` expecting an instance;
   - at least one mutant with `mutant_of`, checking the same predicate;
   - witness runs for the situations the model claims to cover.

   Parameterise a property by the function under test
   (`pred closedIffNoDiag[d: set Key]`), so mutants reuse the same predicate.
5. **Add a correspondence table** in the model header:
   `// | model element | Go symbol | file | binding | abstraction |`.
   - A binding cell may list several tests: `TestA, TestB`. Each must exist.
   - Element check: the element cell's leading identifier must occur, as a
     whole case-sensitive word, in the model source with comments stripped or
     in a manifest command name or body of that model. Pairs that drifted
     before the check are in the shrink-only `[correspondence]
     unresolved_elements`; clear one by renaming the element (and re-keying
     its ledger survivors), then delete the entry. Never add one.
   - A `Recv.name` symbol must name a method of `Recv` when the file declares
     `name` as a function; after renaming a receiver type, update the table,
     or correspondence, the plan, and `triage --check` fail.
   - A row whose Go function does not carry the model's claim (no production
     caller, or the claim lives in a different function) is unbound (binding
     `-`, the reason in its abstraction cell), never given a direct-call test
     that makes the reach gate green without measuring the claim. When the
     model states only part of a function's behaviour, bind it with that
     narrow claim and say in the abstraction cell which inputs are taken from
     production unverified (the `anchors` row: anchors are explicit allowed
     roots; the anchor paths are not verified).
   - Completeness guard: every test in a `*_formal_test.go`,
     `*_golden_test.go`, `*_rapid_test.go`, or `*_trace_test.go` file must be
     named by some row, except `Test*_TraceHarness`, which must live in a
     package a `[[trace]]` suite declares.
   - Characterisation tests (finding replays and probes that assert today's
     behaviour) are marked by the name `Test…_Characterisation` or
     `Test…_FindingF<n>`, or by an abstraction note beginning
     `characterisation:` on the row that tables them. They never kill a
     mutant.
   - A finding's re-run recipe that drops the current replay file into an
     export of an older commit (formal/README.md, F19) works only while that
     file defines its own helpers or uses ones that exist at that commit.
     Moving its helpers into a shared test file breaks the recipe without
     failing any gate, so run the recipe again after editing such a file.
   - Editing an `.als` table changes the model source hash, so run
     `make formal-golden` for that model afterwards.
6. **Bind to code:**
   - Add a `golden` run that records the implementation's decisions in fields
     of a `one sig`, with a `noJunk` predicate excluding atoms that cannot
     influence a decision.
   - Run `make formal-golden`, then write a Go test that loads the vectors with
     `alloygolden.Load(t, path, "<Model>")` and replays every instance. Files
     use golden format 2 (columnar, dictionary-coded). `Load` fails when the
     `.als` changed since generation (the message says to run
     `make formal-golden`). Under `-short` (`make test-short` and the root
     mutation profiles) it returns every 16th instance; the formal-bindings
     mutation profile runs without `-short` and replays every vector. The
     fingerprint also holds a digest of the rendered golden command, which
     `python3 scripts/test_formal.py` checks against the manifest, and
     `make formal` re-enumerates the golden command byte for byte.
   - Map atoms with `alloygolden.GitURL` and `alloygolden.ModuleID`.
   - Transcribe the model's facts and intent in Go and assert them on every
     golden instance too. That keeps the rapid oracle from drifting.
   - Add a rapid test that compares the real code with the intent at larger
     scopes.
   - **Filesystem-materialising replay** (path-containment models), whenever
     the bound code calls `os.ReadFile`, `os.Lstat`, `filepath.EvalSymlinks`,
     or `filepath.WalkDir`:
     - Do not re-model the OS. Materialise each golden instance as a real tree
       with `internal/testutil/fstree` under `filepath.EvalSymlinks(t.TempDir())`
       (the raw temp dir is a symlinked alias on macOS) and call the real
       functions.
     - `internal/testutil/mpctree` translates a `ModulePathContainment`
       instance into an `fstree.Spec`; memoise one materialisation per distinct
       tree (`Case.StructuralKey`) so a large instance count stays cheap.
     - Record every layer decision (`validate`, `lexContains`, `physContains`,
       `accept`, `contained`, `touched`) in a golden `one sig` and compare each
       against the real function, so a layer masked end to end by a later one
       is still calibrated. Bind the masked layer directly through a test-only
       `export_test.go` hook.
     - Probe symlink, junction, and case-fold capability once (`fstree.Probe`,
       which keeps each probe's error) and apply the capability policy:
       symlinks are required on Linux, macOS, and Windows CI
       (`GITHUB_ACTIONS=true`), junctions on Windows. A required capability
       that is missing fails the test with the probe error (`fstree.Require`,
       `fstree.Cache.Get`); elsewhere the test or instance is skipped and
       counted. Binding-test files skip only through these helpers:
       `scripts/formal.py correspondence` rejects a direct `Skip`, `Skipf`, or
       `SkipNow`. Rapid draws create links with `fstree.MustSymlink` and
       `fstree.MustJunction` after a test-level `fstree.Require`. Never give a
       capability writer a fallback writer: the probe shares the creation path,
       so a fallback would let a broken writer pass the policy silently (the
       junction writer has no `mklink /J` fallback).
     - Measure a Windows golden-replay budget from three windows-latest
       attempts of one head (`gh run rerun <id> --job <job>`), never one
       sample: attempts of the same head have differed by 1.7x (23.4 s vs
       39.4 s, `formal/README.md`). A later docs-only head may carry the
       figure if `git diff` of the replay inputs against the measured head is
       empty.
     - Judge a junction escape by file identity (`os.SameFile` against a unique
       outside sentinel, backed by its content), never by
       `filepath.EvalSymlinks` plus a prefix test: since Go 1.23 a mount point
       is not `ModeSymlink`, and `EvalSymlinks` may not evaluate it.
     - Exhaustive enumeration of a relational filesystem is intractable for a
       materialised replay, so fix case-folding and junctions off in the
       `ModulePathContainment` golden and constrain the tree topology with a
       `goldenScenario` predicate; cover the fixed dimensions with witnesses,
       the rapid property, and platform-gated findings tests. A model whose
       requests are independent can carry the dimension instead: the
       `VirtualPathHarness` golden holds junction and dangling-link requests
       at two requests per instance. Its replay materialises junction requests
       on Windows, leaves them out elsewhere through `fstree.Probe` and logs
       their count, and is never skipped (a skipped killer fails the reach
       gate and the mutation run).
     - A replay that can only run on one platform (a junction replay) is never
       a killer: table it alone on a row whose abstraction cell begins
       `characterisation:`, and cover the same branch on Linux through
       injected filesystem lookups (`TestVirtualPathHarness_WalkOverFakeFilesystem`).
     - Budget: golden replay ≤ 30 s per OS, deep rapid ≤ 60 s (decision D9,
       "Cost budgets", of the model-module-path-containment OpenSpec change).
     - Build the virtual-harness validator directly over materialised roots so
       the host's own enclosing roots (`/tmp`, HOME) do not make the check
       vacuous. Where production code decides whether a root exists (the
       workdir: `virtualWorkdir.isReadRoot`), the replay calls that decision
       per instance and adds the root only when it holds, so the directly
       built roots still reach it.
7. **Calibrate.** Seed plausible defects in the real code, one at a time, and
   confirm the bindings fail. Record the result in the manifest's
   `calibration`. If a defect survives, widen the golden scope before trusting
   the model.
8. **Check reach.** `make formal-reach` (no Java) runs every killer of every
   bound row alone under a coverage profile and fails unless each one executes
   at least one statement of the row's function. A row that names a binding
   which never calls its function fails here, not only in a mutation run.
   Every binding gap (a row with no binding that names a function) and every
   ledger survivor of an unbound row is listed in the shrink-only `[triage]`
   table; rows unbound by design (no symbol, a non-function symbol,
   trace-only, characterisation-only) are derived and reported, never listed.
   A characterisation-only row (its binding is a `_Finding*` test) has no
   killer, so reach never runs its binding: check a claim that such a row
   "will fail reach" with `make formal-reach`, not with coverage of that test.
   `triage --check` fails on any entry of `[triage]` or
   `[correspondence] unresolved_elements` that the base does not list. The
   base is `--base <sha>`, else a non-empty `INVOWK_TRIAGE_BASE`, else the
   merge-base with `origin/main`; CI sets the variable with
   `scripts/ci-triage-base.sh` (the base branch tip on a pull request). It
   also classifies the base's tables: a row bound at the base that is now
   by design (for example an abstraction cell turned `characterisation:`)
   fails, and so does a new row under a listed unresolved pair. Bind a row or delete its survivors' entries to shrink a
   list, never add an entry. A seeded defect that another layer fully masks at
   that entry point is recorded as redundant there in the calibration and
   `tasks/next/formal-binding-gaps.md`, and a defect the binding catches is
   seeded instead.
9. **Budget and wire into CI.** Every `[[model]]` and `[[trace]]` needs
   `budget_seconds` and `budget_source`: use `budget_source = "local"` with
   `ceil(3 x local seconds)` (read the runner's `timing` lines) until CI
   dispatch runs measure it. Every TLC command that expects `pass` needs
   `distinct_states` (deterministic under `-workers 1`; read the `states`
   lines). Add every newly modelled file or package to `formal/manifest.toml`
   `[ci] paths`; `scripts/test_formal.py` fails when a table file, binding-test
   file, trace package, or golden output is not matched. The CI replay step
   runs `scripts/formal.py replay-plan`, generated from the binding cells
   (comma-separated, trace harnesses excluded), so a new binding test only
   needs its table row. See "CI budgets" in `formal/README.md`.

## Mutation testing the bindings

`make mutation-formal` (the `formal-bindings` target set of
`scripts/mutation.sh`) measures how much of the code a table names its
bindings alone constrain. The plan, executor, baseline and rerun mechanics live
in `.agents/runbooks/mutation-testing.md`; this skill owns the survivor loop
below.

Resolve every escaped mutant (rerun it twice with `make mutation-formal-rerun`
first; the evidence lands in `tools/mutation/triage/formal-bindings-reruns.jsonl`):

1. **binding-gap, closed:** strengthen the binding test (assertion, generator
   dimension, wider golden scope), confirm the focused rerun kills it, append
   `mutation <id>: <defect>` to the model's `calibration`, and list the id
   under `[[closed]]` in `tools/mutation/triage/formal-bindings.toml`.
2. **model-gap, closed:** add the fact or property with its `mutant_of` and
   antecedent, extend the table, regenerate goldens, bind it, then as in 1.
3. **deferred gap:** keep it in the ledger with a `follow_up`.
4. **abstraction:** state it in the row's abstraction cell; the ledger reason
   must be a substring of that cell.
5. **equivalent:** record the reason. It must hold for every input the
   function accepts. "A later check rejects the same input" is equivalent only
   if that check always runs: four `LoadEnvFile` pre-check mutants were masked
   by `os.Root` only while `os.OpenRoot` succeeded (module-read-physical-containment).
   Probe the claim with `go test -overlay` and the mutated file in a scratch
   directory; never mutate the worktree's bound files. Otherwise defer it.
6. **defect:** a strengthened binding fails on unmodified code. Record a
   finding under the next free id (a `finding =` command next to a passing
   fix configuration, a characterisation test, a `formal/README.md` Findings
   row); never commit the failing binding, and leave the product fix to a
   later change. The ledger entry is `defect` with the finding id as
   `follow_up`.

A survivor decided by a helper the table does not name gets its own row (with
model-gap rigour) or an abstraction note on the calling row. Then run
`make mutation-formal-baseline-update`; it fails until
`python3 scripts/formal_mutation.py triage --check` passes.

Every `[[survivor]]` carries its anchor: `removed` and `added` (the exact
removed and added diff lines, as TOML multi-line literal strings; go-mutesting
hashes them, with the file and mutator, into the id) and, when it removes no
line, `context` (the unchanged line before its first added line). `triage
--check` fails on an entry whose anchor does not hash to its id, whose symbol is
gone, or whose anchor lines no longer appear verbatim (indentation included:
go-mutesting hashes it) and in order inside the symbol's body. Orphaned
survivors are exempt. `[triage] stale_survivors` only shrinks; never add to it.

A change that rewrites a bound function so that a survivor's anchor lines
disappear refreshes that function's entries in the same change, with
`make mutation-formal-refresh` (`scripts/formal_mutation.py refresh`,
mutation-partial-refresh):

0. Check whether the refresh is needed: `python3 scripts/formal_mutation.py
   plan --out DIR` (it must exit 0), then `python3 scripts/formal_mutation.py
   symbols --plan DIR/formal-plan.json <[file:]symbol> ...` with the rewritten
   symbols and `FILE:OLD` for a renamed or deleted one. It resolves each name as
   the refresh does and labels it `planned`, `removal-only`, `unplanned`
   (declared, row unbound: no planned mutants) or `unknown` (exit 1). On
   `-> skip` (all unplanned) there is nothing to refresh; `triage --check` must
   still pass.
1. Commit the change, then run `make mutation-formal` (report only) in a
   throwaway `git worktree add --detach` at that commit, under the heavy-command
   lock, so the wrapper's exit restore cannot touch the working checkout. There
   is no per-function run: it plans every bound function (about 35 minutes),
   and a `git commit` queued behind the lock waits that long.
2. In the change's worktree, run `make mutation-formal-refresh
   MUTATION_FORMAL_SYMBOL=<[file:]symbol>[,...]` (or `MUTATION_FORMAL_STALE=1`
   for every symbol with an entry that fails the anchor check), with
   `MUTATION_REPORT_DIR=<throwaway>/artifacts/mutation`, under the lock. A run
   in another worktree is accepted when its plan inputs, functions and rows
   equal the current tree's. The inputs digest does not cover golden-vector
   data files: rerun step 1 after `make formal-golden`.
   It refuses a run that is not a complete full run of the current inputs
   (profile, `inputs_digest` and each `group_status` in `run-metadata.txt`),
   an unknown or unplanned symbol, and an id that escapes only in a symbol you
   did not choose; it writes nothing then. Otherwise it removes the ids the run
   no longer reports escaped (do not rerun them), adds each new escapee with
   its anchor, its own `digest` and an `untriaged` placeholder, re-anchors kept
   entries whose anchor fails (class and reason kept), drops the symbols' ids
   from `[triage] stale_survivors`, and leaves every other byte unchanged.
3. Triage each placeholder as above: `triage --check` rejects the class
   `untriaged`, a reason starting `UNTRIAGED:` and a reason that still
   contains the placeholder's "write the class and the reason", so replace the
   whole reason, not only the mark. An id moved between two chosen symbols
   comes back untriaged too.
4. Run the `make mutation-formal-rerun MUTATION_MUTANT_ID=...` line the refresh
   printed twice. First commit every tracked edit outside the wrapper's
   allow-list (`dirty_path_is_allowed` in `scripts/mutation.sh`: the two
   mutation baselines, the formal-bindings ledger and reruns file, and
   `artifacts/mutation/`): the wrapper refuses to start while any other tracked
   file is dirty, such as a `formal/manifest.toml` the refresh changed or a
   `tasks/next/formal-binding-gaps.md` update. The refreshed ledger and
   baseline themselves may stay uncommitted. The reruns record evidence at the
   entries' own digest; no digest is copied by hand.
5. `python3 scripts/formal_mutation.py triage --check` must pass. Remove the
   throwaway worktree.

A deleted or renamed function: first remove or rename its correspondence row
(the plan, and so the refresh, fails while a row names a missing function).
Editing an `.als` table changes the model source hash, so run `make
formal-golden` for that model and `python3 scripts/formal.py correspondence`
before the mutation run; otherwise its pre-flight fails. Any row or golden
change goes before the run, which the refresh then matches.
Then `--symbol FILE:OLD` (or `--stale`) is removal-only: its entries are
removed and nothing is added under it. Refresh the new name with `--symbol` in
the same command, so ids that moved to it are added (untriaged) instead of
refused.

## TLA+ commands

- TLC configurations are generated: put base constants in `[model.constants]`
  and only overrides in each `[[model.command]]` (`constants`, `spec`,
  `temporal`). Each command checks exactly one property plus `TypeOK`.
- Thread seeded defects through a `Mutant` constant inside the real actions.
- Record a finding with `finding = "F<n>"` on the current-code command, and give
  the fixed property its own mutant; a finding never guards a property.
- Select per-process programs with a string `Scenario` constant and a `CASE`
  inside the spec (`ConcurrentModuleEdits`): TLC configurations cannot assign
  function values, and no new manifest key is needed.
- Check circular wait between mutexes as an invariant (`NoCircularWait`), with
  explicit stuttering only in a terminal state, so the deadlock check stays on.
- **Trace validation:**
  - Harness: a Go `Test<Model>_TraceHarness` gated by `INVOWK_FORMAL_TRACE_DIR`
    calls `tlatrace.WriteSuite(t, "<Model>", traces)`; use `tlatrace.Recorder`
    to record one entry per projection change. It must live in a package a
    `[[trace]]` suite declares (step 5 completeness guard).
  - Spec: `formal/tla/<Model>Trace.tla` EXTENDS the model and `<Model>Traces`,
    declares `CONSTANTS TraceSet, TraceIndex`, and writes `INSTANCE TraceBase`.
    It defines only `Proj` (a record literal whose fields must equal the
    harness's keys), `TraceInit`, `TraceNext`, `TraceSpec`, and helpers, built
    on `TraceStart`, `TraceAdvanceOnChange`, or `TraceAdvanceAt`.
  - Runner-enforced (`check_tla_sources` in `scripts/formal.py`): a `*Trace.tla`
    must `INSTANCE TraceBase` and may declare no variable other than `i`, and
    every definition in `formal/tla/TraceBase.tla` is reserved in every module
    (today `TraceRecord`, `TraceStart`, `TraceAdvanceOnChange`,
    `TraceAdvanceAt`, `NotFullyConsumed`).
  - The spec must forbid skipping an observable state, and targeted mutations
    must be rejected. Keep `Proj` flat so the key-set checks see every field.
  - `WriteSuite` drops duplicate traces and fails when a mutation equals a
    recorded trace. `tlatrace.RecordEach` runs one parallel subtest per
    sequence and returns a `Recorded` (`Accepted()` for the suite,
    `Base(t, name)` for a mutation's base trace); `tlatrace.Edit`/`Extend`
    build mutations from recorded traces. Concurrency replays park a process
    at a seam with `internal/testutil/procgate`.
  - A `[[trace]]` entry may set `constants` to override the model's base
    constants (only declared ones). Never declare as a mutation a behaviour
    the base constants accept because of an open finding.
- Fakes a model depends on must follow the real contract. For example, the
  watcher's manual timer returns `time.AfterFunc`'s Reset/Stop results.

## Pitfalls

- **Vacuous implications.** `x.f in S implies ...` holds for an empty `x.f`.
  Write `some x.f and x.f in S`.
- **Instance explosion.** Unreferenced atoms and label-only sigs multiply
  golden instances. Measure with a capped `-r` first. Export fails when a
  golden file exceeds 600 KB compressed, or a smaller `[model.golden]
  max_bytes` (it can only lower the cap).
- **Excluding a feature from the golden scope hides defects.** Leaving out
  explicit-scope sources hid two of five seeded scope defects.
- **Alloy's JSON receipt omits subset-sig membership.** The runner reads the
  receipt only for SAT and UNSAT, and exports golden vectors from the XML
  solutions.
- **rapid conventions:** `t.Parallel()` first; name tests `TestX_Property`;
  compare key sets, not order-dependent lists. Mutation profiles pin
  `RAPID_SEED` (`scripts/mutation.sh`).
- **TLC configurations:** never `SYMMETRY` or `VIEW` in a liveness
  configuration, never `CHECK_DEADLOCK FALSE`, and fairness only on system
  actions.

## Verification

Before refactoring models or the runner, record
`python3 scripts/formal.py snapshot --out <outside artifacts>/baseline.json`,
and afterwards require `snapshot --compare` to report every entry identical.

```bash
python3 scripts/formal.py correspondence   # no Java needed
make formal-reach                          # binding reach, no Java needed
make formal                                # needs Java
python3 scripts/test_formal.py
# Replay every bound golden-vector and property test (packages and -run
# pattern generated from the correspondence tables, as in CI):
plan="$(python3 scripts/formal.py replay-plan --format github)" &&
  packages="$(sed -n 's/^packages=//p' <<<"$plan")" &&
  run="$(sed -n 's/^run=//p' <<<"$plan")" &&
  go test -count=1 -run "$run" $packages
```

Any change under `formal/`, `scripts/formal*.py`, or `tools/mutation/` also
needs `make test-scripts`, which includes `scripts/test_formal.py`,
`scripts/test_formal_mutation.py`, and
`python3 scripts/formal_mutation.py triage --check`. When the change touches Go
code a table names, also follow `.agents/rules/formal-bindings.md`.
