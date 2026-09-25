## Context

Invowk is a Go 1.27 CLI. Its assurance stack today:

- golangci-lint;
- the pinned `goplint` DDD analyzer;
- a curated mutation-testing profile whose root baseline has two survivors;
- testscript CLI coverage.

It has no property-based tests, no fuzz tests, and no formal models.

Go has no verifier comparable to Kani. Every Go-adjacent option falls into one of these groups:

- verifies a subset of Go with a heavy toolchain (Gobra);
- translates Go into another prover (Goose, Gomela);
- generates Go from a verified language (Dafny).

The maintainer rejected Gobra and Dafny. The maintainer named relational and protocol properties as the most critical classes.

The sibling Lushtext project measured the alternatives on this machine (Lushtext `docs/next/formal-verification-quint-vs-tlaplus.md`, 2026-09-24):

- **Quint:** its Apalache backend exited 0 without a verdict, a false green. Its TLC backend is an unversioned jar, and it could not finish a 6-action interleaving model.
- **TLA+ with TLC:** reliable and fast on the same models, and trace validation against real logs worked.
- **Lushtext's decision:** it kept TLA+ only for disposable sketches, because hand re-expressed models drifted from the Rust code and Kani removed that drift.
- **Most valuable result:** a model driven against the real code found a defect that the separate models could not see.

Invowk cannot use Kani. Its answer to drift is to bind every model to the real code, and to require calibration before a model counts (D4, D5).

The proposal was reviewed on four independent axes: code fidelity, formal-methods soundness, repository governance, and value against the threat model. Their findings are folded into the decisions below and into D9.

## Goals / Non-Goals

**Goals:**

- Check where the module security guarantees actually live:
  - how a command scope is built from discovery, requires, and the lock file;
  - the explicit-only closure;
  - lock identity;
  - lock-to-content integrity under attacker actions.
- Check the concurrency- and crash-sensitive protocols:
  - the serverbase lifecycle;
  - the SSH host-callback token lifecycle;
  - the watch loop;
  - atomic-write durability.
- Bind every model to the real Go code, and make every verdict fail-closed and non-vacuous.
- Keep `make build/test/lint` free of Java and formal jars.
- Deliver in three phases that can each be verified on their own.

**Non-Goals:**

- Proving the Go code correct. All results are bounded model checking of models that are bound to the code by tests.
- The algorithmic class: the virtual path validator, CUE parsing, and semver. These are candidates for a later fuzzing change.
- Fixing findings. Each confirmed finding becomes its own change after the maintainer decides.
- The standalone `goplint` repository. Its properties are a different class (lattice laws and monotone transfers) and are governed there.

## Decisions

### D1. Alloy 6 for relational rules

The relational rules range over small universes of modules, sources, scopes, and lock entries. Alloy's SAT-backed bounded logic finds adversarial graph shapes by solving, and it can enumerate every instance at a small scope. That enumeration is what makes golden vectors possible (D5).

Commands use Alloy's native `expect 0|1` annotations as the primary verdict source, cross-checked against the manifest. Task 1.3 confirmed that headless `exec` on 6.2.0 reports per-command SAT/UNSAT and exits 1 on an `expect` mismatch, so no Java runner is needed. Two limits shaped the runner:
- The JSON `receipt.json` records `expects` only when it is 1, so the runner reads `expect` from the `.als` source.
- The JSON solutions omit subset-sig membership and list atoms unreliably. Golden vectors are therefore exported from the canonical XML solution format, with `-r 0` to enumerate every instance.

Tidy's iteration is not modelled in Alloy. Alloy asserts only the static result, `roots.^requires − roots`. rapid checks the call bound (depth + 1).

**Alternatives considered:**
- *TLA+ for everything:* verbose for static relations, and it enumerates rather than solves.
- *rapid only:* random generation rarely builds the specific graphs a solver finds.

### D2. TLA+ with TLC for protocols. No Quint, no Apalache, no CommunityModules.

TLC 1.7.4 is the pinned stable version (1.8.0 is a prerelease). The jar ships no `Json` or `IOUtils` module. Trace harnesses therefore emit generated `<Model>Traces.tla` constant modules, as Lushtext's `gen_traces.py` did, and nothing further is pinned.

### D3. Model boundaries follow where the guarantee lives

```
   discovery facts ──┐
   invowkmod requires├──► buildCommandScope ──► CanCallTarget ──► allow/deny
   lock entries ─────┘         ▲
                               └── modelled together (ScopeConstruction.als)

   attacker: edit/downgrade lock, edit vendor dir, move tag, wipe cache
        │
        ▼
   sync (cacheModule, knownHashes) ──► vendor ──► discover (per-parent lock)
        └──────────────── modelled together (LockIntegrity.tla) ─────────┘
```

Modelling `CanCallTarget` on its own would only restate a four-way "or". A correct query over a wrongly built scope still leaks. The scope model therefore includes the following facts, because the guarantees rest on them:

- the source kinds;
- `SourceID` derivation;
- the uniqueness facts from `CheckModuleCollisions`;
- inheritance of the global flag by vendored children.

### D4. Fail-closed runner, vacuity guards, calibration

`scripts/formal.py` (Python standard library, `tomllib`) reads `formal/manifest.toml`. For each command the manifest records:

- the expected verdict;
- the named-violation mutants;
- witness invariants;
- actions declared dead, with a reason;
- the distinct-state count;
- the wall-time budget;
- the calibration record.

Coverage uses TLC's `-coverage` report. Deadlock checking stays on, and terminal states use an explicit stuttering action.

Liveness configurations use no `SYMMETRY` or `VIEW`, because TLC does not guarantee sound liveness under symmetry. Fairness is attached only to system actions. If fairness were attached to cancellation, "delivered or terminated" would hold trivially.

The fail-closed behaviour is itself tested by `scripts/test_formal.py`, which follows the existing `scripts/test_check_skill_packages.py` pattern. It covers missing verdict, wrong verdict, unguarded property, uncovered action, and an `expect` mismatch.

**Alternative considered:** trusting exit codes. Rejected, because that is exactly the Quint false green.

### D5. Four bindings to the real code

| Binding | Used for | Why |
|---|---|---|
| Golden vectors (all Alloy instances at small scope → JSON → Go test on the real function) | all relational models | Removes the "two hand encodings by one author" weakness of a rapid-only transcription |
| Counterexample replay (each recorded counterexample → Go regression test) | every recorded finding | Turns model evidence into code evidence, following Lushtext's `seed_from_tlc` pattern |
| rapid property and state-machine tests (same property names) | every model | Runs in `make test`, and adds mutation killers in the curated packages |
| Trace validation (generated trace module; existential acceptance via `NotFullyConsumed`) | watch, serverbase sequential runs, sync/tidy, atomic write | Checks the real code's behaviour against the spec's transitions |

The seams come from the code-fidelity review:

- **Sync:** `newResolverWithFetcher` with a fake `moduleFetcher` over a temp directory. `Resolver.Sync` calls `m.resolveAll` directly, so `resolveAllFunc` applies only to tidy.
- **Atomic write:** recorded inside `pkg/fspath` through `atomicWriteOps`, which is unexported. `SyncLock` composes the two traces.
- **Watch:** `newWithBackend` with an assignable `schedule`. The fake scheduler must not run `fire` synchronously on the event loop, because that deadlocks on `mu`. A conformance test checks the fake against `time.AfterFunc`, so the fake does not simply encode whatever the loop expects.
- **Scope:** fake `CommandSetProvider` and `CommandScopeLockProvider` through `CheckCommandDependenciesExistWithLockProvider`.

Harnesses are ordinary `_test.go` files that skip unless `INVOWK_FORMAL_TRACE_DIR` is set. A build tag would hide them from golangci-lint, vet, gopls, and goplint, and they would rot.

Concurrent serverbase interleavings stay model-only. A `-race` stress test asserts only the invariants the model proves pass. The race detector cannot see logic races like F1, and asserting a known counterexample would fail `make test` for a finding this change does not fix.

### D6. Tool acquisition and pins

Jars are fetched from GitHub release URLs into `bin/formal/`, with SHA-256 recorded in the script. CI uses `actions/setup-java@v6`, the current major, with Temurin 25 at an exact patch `java-version`. rapid v1.3.0 is a test-only `require` in `go.mod`, whose single source of truth is `go.mod`. It is not listed in `version-pinning.md`, because Dependabot bumps would otherwise break the sync rule.

### D7. Mutation-profile determinism

Under mutation profiles, rapid runs with the following settings:

- **`RAPID_SEED` fixed:** random seeds would make kill and escape status nondeterministic against stable IDs.
- **`RAPID_NOFAILFILE=1`:** a failure otherwise writes `testdata/rapid/*.fail`.
- **`RAPID_SHRINKTIME` small:** the default shrink is up to 30 s per killed mutant.

`**/testdata/rapid/` is ignored. `internal/app/modulesync` and `internal/container` are not in `tools/mutation/root-packages.txt`, so their rapid tests add no killers until that manifest changes. This is stated explicitly rather than claimed away.

### D8. goplint, lint, and file placement

`.goplint/ownership.v1.json` has only `documentation_patterns`. The change adds `^formal/(alloy|tla)/.*\.(als|tla|cfg)$` and `^formal/README\.md$`. `manifest.toml` and scripts stay unmatched, which falls back to the consumer tier (fail-closed).

No Go package lives under `formal/`, because `GOPLINT_PACKAGES` covers only `./cmd ./internal ./pkg`. The correspondence check is a script step that confirms each named symbol exists in its named file. A `go/packages` resolver is deferred.

### D9. Findings to date

The first reading and the four reviews produced these findings. **Confirmed by reading** means that tracing the code established the behaviour, but the model has not yet produced it. Each model must reproduce its finding as a declared counterexample, which also serves as its calibration (D4).

| # | Area | Finding | Status | Model |
|---|------|---------|--------|-------|
| F1 | serverbase | `TransitionToStarting` does its CAS to Starting before storing `ctx`/`cancel`. A concurrent stopper reads `cancel == nil`, and the later context is never cancelled. A variant exists via `TransitionToFailed`/`TransitionToStopped` reading Created. The sshserver path would leak its listener; the tuiserver path would hang in `WaitForReady`. Production callers serialise Start (`HostAccess.Ensure` holds `h.mu`; the TUI defers Stop until after Start). | Confirmed at API level; not reachable from current callers | `Serverbase.tla` + `FixedOrdering.cfg` |
| F2 | serverbase | `TransitionToFailed` checks for a terminal state under `stateMu`, then `Store(Failed)`. A lock-free Created→Stopped CAS in between yields Stopped→Failed. This contradicts `doc.go`. | Confirmed by reading | `Serverbase.tla` |
| F3 | atomic write | `atomicWriteFile` renames without `fsync` of the temp file or the directory. It is safe against a process crash, but not durable across power loss. The lock file is git-tracked and can be regenerated. | Confirmed by reading | `AtomicWrite.tla` (4 configs) |
| F4 | lock integrity | Discovery still accepts v1.0 lock files, and `VerifyLockedVendoredModuleHash` returns nil when `ContentHash == ""`. A lock rewritten as v1.0 without hashes turns off tamper detection. `invowk audit` only warns. | Likely; the model and a replay test decide it | `LockIntegrity.tla` |
| F5 | lock integrity *(fix: `verify-locked-module-integrity`)* | `cacheModule` compares against the expected hash only when the cache directory already exists. On a fresh cache it copies and returns the computed hash without comparing, even when the lock holds one. The locked `GitCommit` is never compared, so a moved tag is not detected. | Confirmed by reading | `LockIntegrity.tla` |
| F6 | lock integrity | A vendored copy is hash-checked against its parent's lock, but admitted through the caller's lock entry. It is unclear whether both bind the same content. | Undetermined | `LockIntegrity.tla` |
| F7 | SSH tokens | Revocation depends on every execution path calling cleanup. Nothing clears tokens on Stop. Revocation affects only new logins, not open sessions. | Undetermined | `HostCallbackToken.tla` |
| — | command scope | *Refuted:* an empty `SourceID` on a same-module target. Discovery always attaches one. | Refuted; kept as a checked invariant | `ScopeConstruction.als` |
| — | lock identity | *Refuted:* empty identities in v2. The parser and saver reject them. The residual risk is a namespace-fallback collision in v1. | Refuted; kept as a checked invariant | `LockIdentity.als` |

### D10. Phasing

```
 Phase 1  toolchain + relational            ── gate: make formal green, test_formal.py green,
          runner, manifest, guards,              golden vectors pass, 3 Alloy models calibrated
          ScopeConstruction, Closure,
          LockIdentity, retry rapid
 Phase 2  protocol models                   ── gate: F1–F7 verdicts recorded with replay tests
          LockIntegrity, Serverbase,             or model-only reasons; maintainer decision on
          HostCallbackToken, AtomicWrite,        follow-up fix changes
          Watch; rapid state machines
 Phase 3  trace validation + CI promotion   ── gate: targeted trace mutations rejected,
          harnesses, trace modules,              four green weekly runs before required
          PR subset job, promotion
```

LockIntegrity and Serverbase come first in phase 2, because they carry the highest-risk findings (F4, F5) and the cheapest calibration (F1, F2).

**Alternative considered:** three separate OpenSpec changes, which both the governance and value reviews recommended. This design keeps one change with hard phase gates, because the three phases share one toolchain, one manifest, and one decision record. Splitting remains a cheap option before phase 1 is applied (see Open Questions).

## Risks / Trade-offs

- **Model drift from code** → Golden vectors, counterexample replay, trace validation, a correspondence check, and calibration (D4, D5).
- **Vacuous green** → Declared verdicts, `expect` cross-checks, named-violation mutants, witness invariants, action coverage, per-check antecedents, fairness-free twins, and deadlock checking left on (D4).
- **Unsound liveness** → No symmetry or `VIEW` in liveness configurations, and fairness on system actions only (D4).
- **State-space explosion** in serverbase and watch → Two or three callers or events, and an environment budget. Safety-only `VIEW` must match a no-`VIEW` run. Bounds are stated in each abstraction column.
- **Fake timer encodes the expected answer** → A conformance test against `time.AfterFunc` (D5).
- **rapid nondeterminism** in `gotestsum --rerun-fails` and in mutation → The seed is logged on failure, and mutation profiles use a fixed seed (D7).
- **Java dependency** → Confined to one workflow. Ordinary targets need no Java.
- **Bounded results read as proofs** → Docs describe bounded model checking of bound models, never proofs of Go code.
- **Findings widen scope** → Findings are recorded and replayed, not fixed. The maintainer decides at the phase-2 gate.
- **Alloy CLI mode insufficient** → A fallback Java runner (D1).

## Migration Plan

The change is additive, with no production or user-facing changes. Rollback deletes the following:

- `formal/`;
- the rapid, golden-vector, and trace test files;
- the workflow;
- the `go.mod` require;
- the `mutation.sh` rapid environment settings.

The CI lane starts as a non-required check.

## Resolved Decisions (maintainer, 2026-09-24)

- **One change with hard phase gates**, not three changes. The phases share one runner, one manifest, and one decision record (D10).
- **F5 is fixed ahead of the phases** in the separate change `verify-locked-module-integrity`. That change also fixes a latent false mismatch: the expected hash is keyed without a version, so a cached copy of a new version was compared with the old version's hash. `LockIntegrity.tla` keeps F5. Its current-code configuration records the counterexample, and a fixed configuration must pass. The F5 replay test becomes that change's regression test.
- **F1–F4, F6, and F7** are decided at the phase-2 gate. Each confirmed finding becomes its own follow-up change.
- **JDK:** Temurin 25 at an exact patch version, matching the local OpenJDK 25 toolbox.
