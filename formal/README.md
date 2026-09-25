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
| `formal/alloy/*.als` | Alloy 6 relational models; they declare no commands |
| `formal/tla/*.tla` | TLA+ models and trace specs; TLC configurations are generated from the manifest |
| `formal/tla/TraceBase.tla` | Shared trace-validation machinery every `*Trace.tla` instantiates |
| `formal/manifest.toml` | Pinned tools, each model's base constants or default scope, and every command's body or overrides and expected verdict; unknown keys are rejected |
| `scripts/formal.py` | Fail-closed runner (Python standard library only) |
| `scripts/test_formal.py` | Tests that every fail-closed path fails |
| `bin/formal/` | Downloaded tool jars (ignored) |
| `artifacts/formal/` | Staged Alloy models with rendered commands (`<Model>/<Model>.als`), counterexamples, logs, solutions, and snapshots (ignored) |
| `*/testdata/formal/*.json.gz` | Golden vectors (format 2) replayed by Go tests |

## Running

Ordinary `make build`, `make test`, and `make lint` need no Java. The Go tests
that replay golden vectors and the property tests run in `make test`.

```sh
make formal              # models, golden re-enumeration, correspondence (needs Java 25)
make formal-alloy        # Alloy models only
make formal-golden       # regenerate golden vectors after a model change
make formal-traces       # trace validation of real-code traces against the models
make formal-rapid-deep   # property tests with RAPID_CHECKS=10000
python3 scripts/test_formal.py
python3 scripts/formal.py snapshot [--out FILE] [--compare FILE]
```

`scripts/formal.py fetch` downloads the pinned jars and verifies their SHA-256.
Every run re-verifies the checksum before use.

`make formal` re-enumerates every golden command and fails unless the result is
byte-identical to the committed file; a fingerprint pre-check first gives a
fast "stale" message. Re-enumeration takes about 2.5 minutes, most of it
`ScopeConstruction`.

`scripts/formal.py snapshot` records every observable result of the suite:
command verdicts; for TLC the violated property, distinct states, and
zero-coverage actions; trace-suite counts and per-trace verdicts; and a digest
of each golden file's decoded instances that does not depend on the encoding.
Record a snapshot before refactoring models or the runner, and require
`snapshot --compare` to report every entry identical afterwards. It is a local
tool, not a CI step.

## How a model earns trust

A model counts as verifying a property only when all of these hold:

1. **Declared verdicts.** Every command and its expected verdict live in
   `formal/manifest.toml`. The runner renders Alloy commands, including their
   `expect` bits, from the manifest into a staged copy of the model, and
   rejects a model source that still declares a `run` or `check`. A checker
   that exits without a recognisable verdict fails the run.
2. **Rejecting mutants.** Every safety property has a mutant that must break
   it, checked through the same predicate.
3. **Non-vacuity.** Every Alloy check has a satisfiable antecedent `run`, and
   witness runs show the situations the model claims to cover are reachable.
4. **Bindings to code.** Golden vectors enumerate every instance at a small
   scope and replay it against the real functions. The Go transcriptions of the
   model's facts and intent are checked on the same instances. Property tests
   (`pgregory.net/rapid`) then compare the real code with that intent at larger
   scopes. Each golden file records the SHA-256 of its model, so editing a model
   without `make formal-golden` fails plain `go test`. It also records the
   SHA-256 of the rendered golden command, so a manifest-only edit of the
   golden body or scope fails `python3 scripts/test_formal.py` without Java.
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

| Model | Tool | Checks | Binding |
|---|---|---|---|
| `ScopeConstruction` | Alloy | how a command scope is built from discovery, requires, and the lock file, and queried | 116928 golden instances; rapid intent test |
| `DependencyClosure` | Alloy | the one-step transitive check decides the closure; tidy adds exactly the undeclared closure | 25765 golden instances; rapid tidy test |
| `LockIdentity` | Alloy | lock identity and ambiguity agree across the two functions that compute them | 28858 golden instances |
| `Serverbase` | TLA+ | lifecycle under two concurrent transitions, split into CAS, lock, and cancel steps | rapid sequential state machine; concurrent stress test |
| `LockIntegrity` | TLA+ | lock-to-content integrity across sync, vendor, discovery, and admission, with attacker actions | integrity tests from #142; Go replays of F4 and F6 |
| `AtomicWrite` | TLA+ | visible and durable state of the atomic lock write under power loss | real-filesystem failure injection at every step |
| `HostCallbackToken` | TLA+ | SSH host-callback token and session lifetime across executions | rapid state machine over the token API |
| `Watch` | TLA+ | debounce loop safety and no-lost-burst liveness under fairness | skip-if-busy scenario on a fake timer checked against `time.AfterFunc` |

### Golden vectors

Golden files use format 2: a sorted atom dictionary and one column per sig and
relation, each holding its distinct values once plus a per-instance index.
Relation arity comes from Alloy's declared field types. Instance order and
duplicates are kept, and output is byte-deterministic (gzip with `mtime=0`).
Export fails when a file exceeds 600,000 compressed bytes, or a smaller
`[model.golden] max_bytes`. `alloygolden.Load` decodes format 2 into the same
`Instance` values and rejects any other format.

| File | Instances | Distinct | Format 1 (compressed) | Format 2 (compressed) | Format 2 (decoded) |
|---|---|---|---|---|---|
| `scope_construction_golden.json.gz` | 116928 | 94920 | 559,856 B | 32,072 B | 4.3 MB |
| `dependency_closure_golden.json.gz` | 25765 | 24709 | 111,493 B | 31,647 B | 0.5 MB |
| `lock_identity_golden.json.gz` | 28858 | 18198 | 147,160 B | 24,218 B | 0.9 MB |

**Deferred: duplicate instances.** The counts above show that Alloy's
enumeration repeats instances. The XML of a duplicate pair is byte-identical
(for example `LockIdentity` solutions 1 and 2, and `ScopeConstruction`
solutions 0 and 9), so `parse_alloy_xml_instance` drops nothing. The solver
distinguishes solutions by state the XML does not show, not by skolems.
Removing duplicates would change `count`, the `-short` sample, and the
instance totals in the calibration records, so it is left to a later change
that updates all three.

Retry is bound by an exhaustive contract test (`TestRetryWithBackoff_Contract`)
instead of a model. Every model's calibration record in `formal/manifest.toml`
lists the seeded defects its bindings detect.

## Trace validation

Harnesses gated by `INVOWK_FORMAL_TRACE_DIR` record traces from the real code
(`TestServerbase_TraceHarness`, `TestAtomicWrite_TraceHarness`,
`TestWatch_TraceHarness`) through `tlatrace.WriteSuite`, which writes the
generated `<Model>Traces` module and fails when a trace set is empty or records
have different keys. Each trace spec (`formal/tla/*Trace.tla`) extends its
model, instantiates `TraceBase` (the reserved operators `TraceRecord`,
`TraceStart`, `TraceAdvanceOnChange`, `TraceAdvanceAt`, and
`NotFullyConsumed`), and defines only its projection `Proj`, its step
constraint, and helpers. It must reach every record in order without skipping
an observable state. The runner fails when the harness's record keys differ
from the fields of `Proj`, because a misspelled key would make a targeted
mutation vacuously rejected. Every suite also carries targeted
mutations that must be rejected. They already exposed two weak specs:
concurrent callers merging two operations into one record, and a projection
that treated leaving the loop as returning.

## Findings

Each finding is a command with a `finding` field: a declared counterexample of
the current code, next to a fix configuration that passes. A finding never
counts as its property's vacuity guard, so every fixed property also has its
own seeded mutant that stays after the fix lands. Fixing one is a separate
change.

| # | Area | Finding | Verdict | Fix the model validates |
|---|---|---|---|---|
| F1 | serverbase | Stop during Start leaves the server context uncancelled | Fixed: CAS and context store now share one critical section; the pre-fix code is kept as `Serverbase.mutantPreFixF1UnlockedStart` | (shipped) |
| F2 | serverbase | A terminal state is overwritten: Stopped becomes Failed | Fixed: terminal transitions CAS from the observed state and re-read on failure; the pre-fix code is kept as `Serverbase.mutantPreFixF2BlindStore` | (shipped) |
| F3 | atomic write | No fsync: power loss can leave an empty lock file | Fixed: the temp file is fsynced before the rename and the directory after it (skipped on Windows); the pre-fix code is kept as `AtomicWrite.mutantPreFixNoSync*` | (shipped) |
| F4 | lock integrity | A v1.0 lock without hashes accepts any vendored content | Fixed: discovery and vendoring reject hashless entries, and sync refetches instead of trusting a cached copy; the pre-fix code is kept as `LockIntegrity.mutantPreFixF4TrustHashless` | (shipped) |
| F5 | lock integrity | Fresh-cache sync trusted fetched content and rewrote the lock hash | Fixed by #142; `LockIntegrity.f5FixedSync` passes | (shipped) |
| F6 | lock integrity | A sibling's vendored copy is admitted through the caller's lock without the caller's hash | Fixed: admission compares the caller's locked hash with the discovered copy; the pre-fix code is kept as `LockIntegrity.mutantPreFixF6AdmitByIdentity` | (shipped) |
| F7 | SSH tokens | A session opened with a token outlives its execution; revocation blocks only new logins | Fixed: revocation closes the connections the token authenticated; the pre-fix code is kept as `HostCallbackToken.mutantPreFixF7RevokeKeepsSessions` | (shipped) |

Two hypotheses were refuted: an empty `SourceID` on module targets
(`ScopeConstruction` keeps discovery's guarantee as a fact, and a mutant shows
what breaks without it), and an empty identity in v2.0 locks
(`LockIdentity.v2NeverUnhashed`). No login succeeds after its execution ends
(`HostCallbackToken.noAuthAfterExecution`), and the watch loop never loses a
burst (`Watch.noLostBurst`).

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
