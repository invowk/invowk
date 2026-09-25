<!-- SPDX-License-Identifier: MPL-2.0 -->

# Formal verification

This directory holds Invowk's formal models. They are **bounded model checks of
models that are bound to the real Go code by tests**. They are not proofs of the
Go code. Go has no verifier comparable to Kani for Rust, so every model here is
paired with Go tests that replay the model against the code that ships.

The OpenSpec change `adopt-formal-verification` owns the plan and its phases.

## Layout

| Path | Contents |
|---|---|
| `formal/alloy/*.als` | Alloy 6 relational models |
| `formal/tla/` | TLA+ models checked by TLC (phase 2) |
| `formal/manifest.toml` | Pinned tools and every command's expected verdict |
| `scripts/formal.py` | Fail-closed runner (Python standard library only) |
| `scripts/test_formal.py` | Tests that every fail-closed path fails |
| `bin/formal/` | Downloaded tool jars (ignored) |
| `artifacts/formal/` | Counterexamples, logs, and solutions (ignored) |
| `*/testdata/formal/*.json.gz` | Golden vectors replayed by Go tests |

## Running

Ordinary `make build`, `make test`, and `make lint` need no Java. The Go tests
that replay golden vectors and the property tests run in `make test`.

```sh
make formal              # models, golden freshness, correspondence (needs Java 25)
make formal-alloy        # Alloy models only
make formal-golden       # regenerate golden vectors after a model change
make formal-rapid-deep   # property tests with RAPID_CHECKS=10000
python3 scripts/test_formal.py
```

`scripts/formal.py fetch` downloads the pinned jars and verifies their SHA-256.
Every run re-verifies the checksum before use.

## How a model earns trust

A model counts as verifying a property only when all of these hold:

1. **Declared verdicts.** Every command has an expected verdict in both the
   Alloy `expect` annotation and `formal/manifest.toml`. A checker that exits
   without a recognisable verdict fails the run.
2. **Rejecting mutants.** Every safety property has a mutant that must break
   it, checked through the same predicate.
3. **Non-vacuity.** Every Alloy check has a satisfiable antecedent `run`, and
   witness runs show the situations the model claims to cover are reachable.
4. **Bindings to code.** Golden vectors enumerate every instance at a small
   scope and replay it against the real functions. The Go transcriptions of the
   model's facts and intent are checked on the same instances. Property tests
   (`pgregory.net/rapid`) then compare the real code with that intent at larger
   scopes. Each golden file records the SHA-256 of its model, so editing a model
   without `make formal-golden` fails plain `go test`.
5. **Calibration.** The bindings detect seeded defects in the real code. The
   manifest records which ones.
6. **Correspondence.** Each model's header table names the Go symbols and
   binding tests it represents. `scripts/formal.py correspondence` fails when
   one goes stale.

The guards have already paid for themselves. A first draft of the scope model
had a vacuous fact: `c.childOf in GlobalCmd` holds trivially for an empty
`childOf`, which made every source global. All safety checks passed. The
antecedents, witnesses, and mutants failed and exposed it.

## Models

| Model | Checks | Golden instances | Calibration |
|---|---|---|---|
| `ScopeConstruction` | how a command scope is built from discovery, requires, and the lock file, and queried | 116928 | 5 seeded defects detected |
| `DependencyClosure` | the one-step transitive check decides the closure; tidy adds exactly the undeclared closure | 25765 | 4 seeded defects detected |
| `LockIdentity` | lock identity and ambiguity agree across the two functions that compute them | 28858 | 4 seeded defects detected |

Retry is bound by an exhaustive contract test
(`TestRetryWithBackoff_Contract`) instead of a model.

## Findings

The findings come from reading the modelled code and from the four-axis review
of the change. Phase 2 models must reproduce each one as a declared
counterexample.

| # | Area | Finding | Status |
|---|---|---|---|
| F1 | serverbase | Stop during Start leaves the server context uncancelled | Confirmed at API level; not reachable from current callers |
| F2 | serverbase | A terminal state can be overwritten (Stopped to Failed) | Confirmed by reading |
| F3 | atomic write | No `fsync` of the temp file or directory before rename | Confirmed by reading |
| F4 | lock integrity | A v1.0 lock without hashes disables tamper detection | Likely; `LockIdentity.witnessV1Unhashed` shows the state is reachable |
| F5 | lock integrity | Fresh-cache sync never compared fetched content with the lock | Fixed by `verify-locked-module-integrity` |
| F6 | lock integrity | A vendored copy is verified against one lock and admitted through another | Undetermined |
| F7 | SSH tokens | Tokens are not cleared when the server stops | Undetermined |

Two hypotheses were refuted: an empty `SourceID` on module targets
(`ScopeConstruction` keeps discovery's guarantee as a fact, and a mutant shows
what breaks without it), and an empty identity in v2.0 locks
(`LockIdentity.v2NeverUnhashed`).

## Mutation testing

Mutation profiles run rapid with a fixed `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and
a bounded `RAPID_SHRINKTIME`, so kill and escape results stay reproducible.
`internal/app/modulesync` and `internal/container` are not in
`tools/mutation/root-packages.txt`, so their new tests add no killers to the
full profile until that manifest changes.

## Decision record: rejected tools

The measurements come from the sibling Lushtext project on this machine
(Lushtext `docs/next/formal-verification-quint-vs-tlaplus.md`, 2026-09-24),
which compared tools on its own protocols.

| Tool | Reason | Evidence |
|---|---|---|
| Quint | Quint 0.32.0 with Apalache 0.62.2 exited 0 with no verdict (a false green); its TLC backend is an unversioned jar; it could not finish a 6-action interleaving model | Lushtext §6.1 and §8 |
| Gobra | Heavy, varied toolchain (Viper, Z3, Java) and partial support for the Go subset Invowk uses | Maintainer decision, 2026-09-24 |
| Dafny | A second implementation language whose generated Go falls outside lint, goplint, and the file-length rule | Maintainer decision, 2026-09-24 |
| Lean 4 | No bridge to Go; the refinement gap would be unbounded | Exploration, 2026-09-24 |
| Goose / Perennial | Research-grade Go-to-Coq translation; thesis-scale effort | Exploration, 2026-09-24 |
| Gomela | Research tool for Go channel deadlocks; not maintained for production use | Exploration, 2026-09-24 |

Adopting a rejected tool requires new evidence that overturns its recorded reason.

Lushtext kept TLA+ only for disposable sketches because hand-written models
drifted from its Rust code, and Kani removed that drift. Invowk has no Kani, so
it keeps its models and controls drift with golden vectors, property tests,
calibration, and correspondence checks instead.
