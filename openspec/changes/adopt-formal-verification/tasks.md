## 1. Phase 1 — Toolchain and runner

- [x] 1.1 Add `pgregory.net/rapid@v1.3.0` as a test-only dependency (`go get`, `make tidy`), and confirm that only `_test.go` files reference it
- [x] 1.2 Record SHA-256 values for Alloy `6.2.0` `org.alloytools.alloy.dist.jar` and TLA+ `1.7.4` `tla2tools.jar`. Create `scripts/formal.py` with fetch, verify, re-verify-on-use, and version/checksum printing
- [x] 1.3 Test Alloy 6.2.0 headless `exec` on three fixtures: satisfiable, unsatisfiable, and `expect`-mismatch. It reports per-command outcomes and exits 1 on a mismatch, so no Java runner was needed (design D1)
- [x] 1.4 Define the `formal/manifest.toml` schema. Each command records:
  - the expected verdict;
  - named-violation mutants and witness invariants;
  - actions declared dead, with a reason;
  - the distinct-state count and wall-time budget;
  - the calibration record and correspondence paths.
- [x] 1.5 Implement verdict parsing with `NO-VERDICT` failure and the `expect`/manifest cross-check
- [x] 1.6 Implement the guards: named-violation mutants, witness invariants, TLC `-coverage` zero-state actions, per-check Alloy antecedent `run`s, fairness-free twins, the no-`SYMMETRY`/`VIEW` rule for liveness configurations, and state-count drift warnings
- [x] 1.7 Implement the correspondence check as a script step that confirms each named symbol exists in its named file
- [x] 1.8 Write `scripts/test_formal.py`, with one case each for: missing verdict, wrong verdict, `expect` mismatch, unguarded property, uncovered action, symmetry in a liveness configuration, a missing fairness-free twin, a checksum mismatch, and a stale correspondence row. Register it in `test-scripts`
- [x] 1.9 Add `artifacts/formal/` and `**/testdata/rapid/` to `.gitignore`. Add the Make targets `formal`, `formal-alloy`, `formal-tla`, `formal-traces`, and `formal-rapid-deep` with `make help` entries. `formal-rapid-deep` uses `RAPID_CHECKS` and runs only the rapid packages
- [x] 1.10 Add SPDX `MPL-2.0` header comments to all `.als`, `.tla`, `.cfg`, `.toml`, and `.sh` files the change creates
- [x] 1.11 Set `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and a bounded `RAPID_SHRINKTIME` in `scripts/mutation.sh`. Show that one mutant gives the same status twice (mutation-testing delta spec)

## 2. Phase 1 — Command-scope construction

- [x] 2.1 Write `formal/alloy/ScopeConstruction.als` with its correspondence table. It models:
  - source kinds;
  - self-declared `ModuleID`;
  - `SourceID` derivation;
  - the `CheckModuleCollisions` uniqueness facts;
  - `IsGlobalModule` inheritance by vendored children;
  - requires and lock entries through `IsDeclaredLockedCommandSource`;
  - `buildCommandScope`;
  - the full `CanCallTarget` decision order.
- [x] 2.2 Add these checks, each with an antecedent `run`:
  - the "allowed targets are bounded" check, including the global-children exception;
  - the no-empty-`SourceID` invariant (refuted hypothesis);
  - the root-invowkfile-command decision.
- [x] 2.3 Add the collision-fact-removal mutant, and confirm that its counterexample names the bounded-targets check
- [x] 2.4 Export every instance at small scope as golden vectors. Write `internal/app/deps/scope_golden_test.go`, which replays them through `CheckCommandDependenciesExistWithLockProvider` with fake providers
- [x] 2.5 Write `internal/app/deps/scope_rapid_test.go`, with generators restricted to inputs discovery actually produces
- [x] 2.6 Calibrate the model by showing that a seeded code defect makes the golden or rapid tests fail. Example defects: dropping the source check in `targetIsDirectDependency`, or granting global by `ModuleID`

## 3. Phase 1 — Closure, lock identity, retry

- [x] 3.1 Write `formal/alloy/DependencyClosure.als`. Model the one-step check literally, and write the specification with `^requires`, keyed by `ModuleRefKey`. Assert the "iff" and the static tidy result, and add a rejecting mutant and witness `run`s
- [x] 3.2 Write `formal/alloy/LockIdentity.als`. Identity is `module_id`, or else the namespace prefix. It covers:
  - ambiguity even with equal hashes;
  - the no-empty-`module_id` invariant for v2;
  - the v1 namespace-fallback collision;
  - empty-hash evaluation status;
  - the key-versus-`ModuleID` cross-check.
- [x] 3.3 Export golden vectors for both models. Write golden tests in `pkg/invowkmod` for `CheckMissingTransitiveDeps`, `CheckMissingVendoredTransitiveDeps`, `FindAmbiguousLockedModuleEntries`, and `EvaluateVendoredModuleHash`
- [x] 3.4 Write `internal/app/modulesync/tidy_rapid_test.go`. It checks the result key set and the `resolveAll` call bound of depth + 1. Declare the no-new-key branch dead in the manifest
- [x] 3.5 Write `internal/container/retry_rapid_test.go`. It exhaustively enumerates outcome sequences up to a small bound, including `maxAttempts <= 0`, the ordering of nil error and `retry`, and sleeper errors
- [x] 3.6 Calibrate both Alloy models against seeded code defects
- [x] 3.7 **Phase 1 gate:** `make formal-alloy`, `make test-scripts`, and `make test` are green, and three Alloy models are calibrated. Locally, the gate passed except for failures that also occur on a clean `main`: five `internal/runtime` container tests (Docker `/workspace` mount "Permission denied") and `test_bencher_registry_login.sh` (needs `BENCHER_API_TOKEN`)

## 4. Phase 2 — Lock-to-content integrity

- [x] 4.1 Write `formal/tla/LockIntegrity.tla` with its correspondence table. It models:
  - sync (`cacheModule` with and without an existing cache, `knownHashes`);
  - vendor;
  - discovery's per-parent-lock check;
  - attacker actions: edit or downgrade the lock, edit a vendored directory, move a tag, wipe the cache.
- [x] 4.2 Add the "loaded content matches a trusted hash" invariant, witness invariants for each attacker action, and declared verdicts for F4, F5, and F6
- [x] 4.3 Write replay tests: a v1.0 lock without hashes plus a tampered vendored directory (F4), and a fresh cache with a lock hash and different fetched content via a fake `moduleFetcher` (F5)
- [x] 4.4 Add a `FixedCache` configuration mirroring `verify-locked-module-integrity`. It must pass the trusted-hash invariant for the fresh-cache and moved-tag attacker actions. If that change has already merged, the F5 replay test must pass against the current code

## 5. Phase 2 — Serverbase lifecycle

- [x] 5.1 Write `formal/tla/Serverbase.tla`. It decomposes every `Load`/`Store`/CAS and every `stateMu` section as the code does. It includes all transitions and `SendError`/`CloseErrChannel`, with two or three callers drawn from all transitions
- [x] 5.2 Add these invariants:
  - errCh is closed at most once and never sent to after closing;
  - startedCh is closed once, by the winning CAS;
  - terminal absorption (expected counterexample, F2);
  - the stop-during-start cancellation safety invariant (expected counterexample, F1).

  Add `FixedOrdering.cfg`, which must pass, and add rejecting mutants
- [x] 5.3 Record in the correspondence table whether `HostAccess.Ensure` and the interactive TUI path can reach F1 and F2
- [x] 5.4 Write `internal/core/serverbase/base_rapid_test.go` as a sequential state machine, and list the interleaving-only properties it cannot reach
- [x] 5.5 Write a `-race` stress test that asserts only the model's passing invariants and bounds its iterations under `testing.Short()`. List F1 and F2 as model-only evidence unless a deterministic replay is possible. F1 and F2 need interleavings inside one transition, so they stay model-only until the fix change adds a seam

## 6. Phase 2 — SSH tokens, atomic write, watch

- [x] 6.1 Write `formal/tla/HostCallbackToken.tla`. It models generate, auth, revoke, TTL, the cleanup goroutine, Stop, and the exec success, error, and cancel paths. Add invariants for "no auth after execution ends" and "tokens do not outlive the server" (F7), plus the in-flight-session characterisation
- [x] 6.2 Write `internal/sshserver/token_formal_test.go` as a state machine over the token API
- [x] 6.3 Write `formal/tla/AtomicWrite.tla` with the named axioms and four `SyncFile`/`SyncDir` configurations. Check D-Atomic and D-Commit. Readers-never-see-partial must pass. Record F3 for the current-code configuration, and require the full-sync configuration to pass
- [x] 6.4 Write `pkg/fspath/atomic_rapid_test.go`, which injects failures at every step through `atomicWriteOps` and checks cleanup or error joining
- [x] 6.5 Write `formal/tla/Watch.tla`. It models:
  - `time.AfterFunc` `Reset`/`Stop` return values;
  - skip-if-busy re-arm and wait-group accounting;
  - callback errors and channel closure;
  - cancellation, fatal errors, and `Ready`.

  Add the safety invariants and `Arrived(e) ~> (Delivered(e) \/ Terminated)`, with fairness on system actions only, plus its fairness-free twin
- [x] 6.6 Write the fake-scheduler conformance test against `time.AfterFunc`, then the skip-if-busy binding using `newWithBackend`, a temp directory, a fake backend, and an asynchronous fake scheduler (`internal/watch/watcher_formal_test.go`). The existing manual timer always returned false from Reset/Stop, so a conformant fake was added
- [x] 6.7 **Phase 2 gate:** every F1–F7 verdict is recorded, with a replay test or a model-only reason. Present the verdicts to the maintainer and record the decision on follow-up fix changes

## 7. Phase 3 — Trace validation

- [x] 7.1 Implement the trace-module generator, which emits `<Model>Traces.tla` constants, and the existential acceptance through `INVARIANT NotFullyConsumed`
- [x] 7.2 Write trace harnesses gated by `INVOWK_FORMAL_TRACE_DIR` (done for watch, serverbase sequential runs, and atomic write; sync and tidy are bound by #142's integrity tests and replays instead, as the spec now states):
  - watch;
  - serverbase sequential runs;
  - sync, through `newResolverWithFetcher`;
  - tidy, through `resolveAllFunc`;
  - atomic write, through `atomicWriteOps` in `pkg/fspath`.
- [x] 7.3 Write the trace-validation specs for each harness (`ServerbaseTrace`, `AtomicWriteTrace`, `WatchTrace`)
- [x] 7.4 Add semantically targeted trace mutations with declared verdicts (10 across the three suites). They exposed two weak trace specs before the fix: concurrent callers merging two operations into one record, and "left the loop" conflated with "returned"

## 8. Phase 3 — CI lane

- [x] 8.1 Create `.github/workflows/formal-verification.yml`. It runs weekly and on dispatch, executing `make formal` and `make formal-rapid-deep`, using:
  - `actions/checkout@v7`, `actions/setup-go@v6`, and `actions/setup-java@v6` (Temurin 25, exact patch `java-version`), with a `java -version` check;
  - `permissions: contents: read`, `concurrency`, `timeout-minutes`, and job-level `env:`;
  - `upload-artifact@v7` for `artifacts/formal/` on failure.
- [x] 8.2 Add the pull-request trigger for `formal/` and modelled packages. It runs the full lane (about 35 s) instead of selecting models, which the spec now states
- [ ] 8.3 Measure per-model wall time on the first green CI run, and record the budgets and state counts in the manifest (locally: `make formal` 35 s, `make formal-traces` about 10 s; CI measurement pending)
- [x] 8.4 Log rapid seeds on failure in CI and in `make test` output
- [ ] 8.5 **Phase 3 gate:** trace mutations are rejected as declared. The lane stays non-required until four consecutive weekly runs are green

## 9. Documentation and governance

- [x] 9.1 Write `formal/README.md`. It covers:
  - layout and how to run;
  - the bounded-checking caveat;
  - the decision record, one entry per rejected tool, with a summary of the Lushtext measurements;
  - the F1–F7 table with verdicts;
  - the calibration status per model;
  - the mutation-manifest note for `modulesync` and `container`.
- [x] 9.2 Create `.agents/skills/formal-verification/SKILL.md` with name and description frontmatter, and `agents/openai.yaml`. Cover authoring, correspondence tables, guards, bindings, trace harnesses, and rapid conventions: `t.Parallel()` first, `t.Context()`, `TestX_Property` names, key-set comparison
- [x] 9.3 Update `.claude/CLAUDE.md` (which `AGENTS.md` symlinks to):
  - add the skill to the skills index;
  - add a `formal/` row to the code-area mapping;
  - add `formal-verification` to the rows of the modelled packages.

  Cross-link from the `testing` and `go-testing` skills
- [x] 9.4 Update:
  - `.agents/rules/version-pinning.md`: Alloy, TLA+ tools, JDK, `setup-java`. rapid is not listed;
  - `.agents/skills/ci-update/SKILL.md`: the sync-pair table;
  - `.agents/rules/commands.md`: targets, Java prerequisite, and the workflow table row.
- [x] 9.5 Add `^formal/(alloy|tla)/.*\.(als|tla|cfg)$` and `^formal/README\.md$` to `.goplint/ownership.v1.json` `documentation_patterns`
- [x] 9.6 Check whether `docs/architecture/` needs an assurance-layers note, and update it if so. Record that README and website need no change, because neither documents dev tooling. No diagram changes: the models add no component, flow, or boundary, and `formal/README.md` documents the assurance layer
- [x] 9.7 Run `make check-agent-docs` and fix all drift

## 10. Verification

- [ ] 10.1 Run `/simplify` on the changed Go tests and scripts
- [ ] 10.2 Run `make tidy`, `make license-check`, `make lint`, `make lint-scripts`, `make test-scripts`, and `make check-file-length`
- [ ] 10.3 Run `make test` and record the wall time the rapid tests add
- [ ] 10.4 Run `make check-goplint-consumer-routed` and `make check-baseline`, and triage any new goplint findings with the maintainer
- [ ] 10.5 Run `make formal` end to end, and confirm every declared verdict, guard, and trace mutation
- [ ] 10.6 Confirm that `git ls-files artifacts/ '**/testdata/rapid/**'` lists nothing
- [ ] 10.7 Run `make sonar-local` and resolve its issues. Record that `make test-cli` and the native-mirror checks do not apply, because no txtar files change
- [ ] 10.8 Run `/learn`
