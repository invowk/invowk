---
name: formal-verification
description: Invowk formal-verification workflow for Alloy 6 and TLA+/TLC models under formal/, the fail-closed runner scripts/formal.py and formal/manifest.toml, golden vectors replayed by Go tests, rapid property tests, calibration, and model-to-code correspondence tables. Use when adding or changing a model, when a golden-vector or property test fails, when code named in a correspondence table changes, or when `make formal` fails.
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
3. **Label every command** as `name: check { ... } for N expect 0|1`, and add a
   matching `[[model.command]]` in `formal/manifest.toml`.
4. **Guard every safety check:**
   - an antecedent `run` expecting an instance;
   - at least one mutant with `mutant_of`, checking the same predicate;
   - witness runs for the situations the model claims to cover.

   Parameterise a property by the function under test
   (`pred closedIffNoDiag[d: set Key]`), so mutants reuse the same predicate.
5. **Add a correspondence table** in the model header:
   `// | model element | Go symbol | file | binding | abstraction |`.
6. **Bind to code:**
   - Add a `golden` run that records the implementation's decisions in fields
     of a `one sig`, with a `noJunk` predicate excluding atoms that cannot
     influence a decision.
   - Run `make formal-golden`, then write a Go test that loads the vectors with
     `alloygolden.Load(t, path, "<Model>")` and replays every instance. `Load`
     fails when the `.als` changed since generation, and under `-short` (the
     mutation profiles) it returns every 16th instance.
   - Map atoms with `alloygolden.GitURL` and `alloygolden.ModuleID`.
   - Transcribe the model's facts and intent in Go and assert them on every
     golden instance too. That keeps the rapid oracle from drifting.
   - Add a rapid test that compares the real code with the intent at larger
     scopes.
7. **Calibrate.** Seed plausible defects in the real code, one at a time, and
   confirm the bindings fail. Record the result in the manifest's
   `calibration`. If a defect survives, widen the golden scope before trusting
   the model.

## TLA+ commands

- TLC configurations are generated: put base constants in `[model.constants]`
  and only overrides in each `[[model.command]]` (`constants`, `spec`,
  `temporal`). Each command checks exactly one property plus `TypeOK`.
- Thread seeded defects through a `Mutant` constant inside the real actions.
- Record a finding with `finding = "F<n>"` on the current-code command, and give
  the fixed property its own mutant; a finding never guards a property.
- Trace validation: a Go `Test<Model>_TraceHarness` gated by
  `INVOWK_FORMAL_TRACE_DIR` writes `<Model>Traces.tla` through
  `internal/testutil/tlatrace`. `formal/tla/<Model>Trace.tla` must forbid
  skipping an observable state, and targeted mutations must be rejected.
- Fakes a model depends on must follow the real contract. For example, the
  watcher's manual timer returns `time.AfterFunc`'s Reset/Stop results.

## Pitfalls

- **Vacuous implications.** `x.f in S implies ...` holds for an empty `x.f`.
  Write `some x.f and x.f in S`.
- **Instance explosion.** Unreferenced atoms and label-only sigs multiply
  golden instances. Measure with a capped `-r` first. Keep golden files under
  about 600 KB compressed.
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

```sh
make formal
python3 scripts/test_formal.py
go test ./internal/app/deps/ ./internal/app/modulesync/ ./pkg/invowkmod/ ./internal/container/
```
