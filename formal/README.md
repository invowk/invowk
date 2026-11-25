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
| `scripts/formal_promotion_gate.py` | Read-only promotion gate over the lane's scheduled-run history (`make formal-promotion-gate`) |
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
make formal-reach        # every binding test executes the function its row names (no Java)
make formal-promotion-gate  # read-only check of the lane's promotion rule
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
   one goes stale, when a row's model element (the leading identifier of its
   element cell, matched case-sensitively) occurs neither in the model source
   with comments stripped nor in a manifest command name or body of that model,
   and when a binding-test file (`*_formal_test.go`, `*_golden_test.go`,
   `*_rapid_test.go`, `*_trace_test.go`) calls `Skip`, `Skipf`, or `SkipNow`
   directly. Capability skips go through the `internal/testutil/fstree` policy
   (`fstree.Require`): symlinks are required on Linux and macOS and in Windows
   CI, junctions on Windows, and a required capability that is missing fails
   with the probe's error instead of skipping.
7. **Binding reach.** `make formal-reach` builds the formal-bindings plan and
   runs every killer test of every bound row alone under a coverage profile
   (`-covermode=set`, the mutation profile's rapid seed). Each (row, killer,
   function) triple must execute at least one statement of the function; a
   failing, skipped, or unrun killer, a missing or garbled profile, and an
   uninstrumented file fail the gate. Rows without a killer are listed with
   their reason and abstraction cell, not gated. Reports land in
   `artifacts/formal/reach/`.

Two lists in `formal/manifest.toml` only shrink. `[correspondence]
unresolved_elements` holds the 34 `"<model>: <element>"` pairs that named no
model element when the element check was introduced (naming drift, for
example `State` against the variable `state`); a listed pair is reported, and
one that resolves or names no row fails as stale. `[triage]` lists every
binding gap, a row with no binding that names a function (`binding_gaps`, 19
when seeded), and every ledger survivor whose row is unbound
(`orphaned_survivors`, 20). Rows unbound by design (no symbol, a symbol that
names no function, trace-only, or characterisation-only; 34) are derived and
reported, never listed, so a new one passes.
`scripts/formal_mutation.py triage --check` fails on an unlisted gap or orphan
and on a listed entry that no longer applies. It also fails on any entry of the
four lists that the base commit's manifest does not list, and prints why each
entry the base lists has left (bound, reclassified, resolved, or deleted). The
base is `--base <sha>`, else a non-empty `INVOWK_TRIAGE_BASE`, else
`git merge-base HEAD origin/main`. In CI `scripts/ci-triage-base.sh` sets the
variable for the formal lane and the script-test gate: the base branch tip on a
pull request, the commit under test otherwise. It classifies the base's tables
too: a row bound at the base that is now unbound by design fails, and so does a
new row under a listed unresolved pair (decision 28); a deleted or re-pointed
row is a notice. A `Recv.name` symbol whose receiver no longer declares `name`
fails correspondence, the plan, and the triage check (decision 27). A base with
none of the first three lists seeds them all, reporting those checks as
notices; one with only some fails. `stale_survivors` also seeds from a base
that has the first three but not it. A base that cannot be read fails closed. To
clear an entry, rename the element or bind the row, then delete the entry; an
added entry fails against the base. The survivor-triage follow-up empties
`unresolved_elements` and `orphaned_survivors` and binds the gaps.

Each ledger survivor is anchored to the code it mutates: `removed` and `added`
hold the exact diff lines go-mutesting hashes into the stable id, plus
`context` (the unchanged line before the insertion) for a mutant that removes no
line. `triage --check` fails, naming the id and symbol, when the anchor does not
hash to the id with the baseline's mutator, when the symbol no longer names a
function in its file, or when the anchor lines, verbatim with their indentation,
no longer appear in order inside that function's body (a removed comment line
may follow the function). It cannot tell which of several identical lines in
the body was mutated, nor see a line inserted between non-adjacent removed
lines (design D1 of refresh-anchors-mutation-ledger). Edits elsewhere in a bound file
pass. The check reads only the current tree, so a depth-1 checkout gives the
same verdict. Orphaned survivors are not anchored. A fourth shrink-only list,
`[triage] stale_survivors`, holds survivors that fail it and that their change
did not refresh (seeded empty by refresh-anchors-mutation-ledger); a listed id
that passes fails as stale. A change that rewrites a bound function refreshes
its survivors in the same change with `make mutation-formal-refresh` (the
`refresh` subcommand) from a report-only `make mutation-formal` run of the
same inputs: it removes the ids that no longer escape, adds the new escapees
with their anchor, their own evidence `digest` and an `untriaged` placeholder
that `triage --check` rejects until a person writes the class and the reason,
re-anchors kept entries, drops the refreshed ids from `stale_survivors`, and
names the reruns still owed; every other entry stays byte-identical. A full
`mutation-formal-baseline-update` writes only the top-level digest. Mutants with the same diff lines at different
places share one go-mutesting id, and go-mutesting's baseline matches by id, so
the baseline lists each id once: the merge keeps the first entry and
`triage --check` fails on a repeated id.

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
| `LockIntegrity` | TLA+ | lock-to-content integrity across sync, vendor, discovery, and admission, with attacker actions | trace validation of real sync, vendor, discovery, and admission; integrity tests from #142; Go replays of F4, F6, and F12 |
| `AtomicWrite` | TLA+ | visible and durable state of the atomic lock write under power loss | real-filesystem failure injection at every step |
| `HostCallbackToken` | TLA+ | SSH host-callback token and session lifetime across executions | trace validation of a started server with real SSH clients; rapid state machine over the token API; Go replay of F11 |
| `Watch` | TLA+ | debounce loop safety and no-lost-burst liveness under fairness | skip-if-busy scenario on a fake timer checked against `time.AfterFunc` |
| `ConcurrentModuleEdits` | TLA+ | two module commands interleaved step by step over `invowkmod.cue`, the lock, the shared module cache, and `invowk_modules/`, whose fix configurations are the base (project lock, atomic restore), the pre-fix code kept as mutants | deterministic gated replays of F8, F9, F10, and the vendor case against the real `Resolver` and `vendorDependenciesWithResolver` |
| `ModulePathContainment` | Alloy | module filesystem path containment across script/env/containerfile reads, workdir, vendored discovery, copies, hash, and unpack, with the discovery facts named | 22848 golden instances (1680 distinct trees), replayed with relative and with absolute link targets; rapid property; findings F13, F16 and F17 (fixed) |
| `VirtualPathHarness` | Alloy | the virtual runtime's allowed-root construction (whether the workdir and `@work` are roots, by an abstract workdir kind: trusted, module-declared inside the module, or module-declared outside it; the workdir's precedence source and its physical placement through links and junctions are generator dimensions of the Go replays and the rapid property, not model facts), `normalizeExistingOrParent` with its path walk (`walkVirtualPath`: symbolic links, Windows junctions and dangling links, to the deepest existing ancestor, with each walk helper bound), the no-follow form of `rm`, `mv` and `ln` operands, the `-L`/`-h` tests and, on non-Unix platforms, the access emulation's stat of the resolved path (`resolveNoFollow`, `validateNoFollow`: the parent resolved by the walk, the final name joined unread), and `pathWithin` under restricted/full access | 8790 golden instances of two requests each (81 request shapes, every workdir kind), with symlink, junction and dangling-link vectors; rapid property with an oracle in both directions, workdir placement included; a resolver test over a fake filesystem; the no-follow binding test; findings F14, F15, F18 and F19 (fixed) |

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
| `module_path_containment_golden.json.gz` | 22848 | 1680 | - | 20,994 B | - |
| `virtual_path_harness_golden.json.gz` | 8790 | 8790 | - | 6,424 B | - |

The two path-containment goldens are replayed on a real filesystem: `internal/testutil/fstree` materialises every instance as a directory tree under `filepath.EvalSymlinks(t.TempDir())` and the replay memoises one materialisation per distinct tree (design D9). Golden replay in `make test` measures ~2.5 s (`ModulePathContainment`), ~0.2 s (containment layer and container workdir), and about 0.4 s (`VirtualPathHarness`, 8790 instances of two requests) on Linux; the two deep-rapid properties measure ~3.1 s and ~0.5 s at `RAPID_CHECKS=10000` — well within the 30 s replay and 60 s deep-rapid budgets (design D9, "Cost budgets"). Case-folding is fixed off in both goldens, and Windows junctions are fixed off in the `ModulePathContainment` golden. For that model junctions are covered by the junction witness commands, the junction branch of `TestModulePathContainment_Property` (required on Windows), and `TestModulePathContainment_F16Junction` (required on Windows, skipped by the capability policy on Linux and macOS); no `ModulePathContainment` golden instance replays a junction (harden-formal-instruments design D8). The `VirtualPathHarness` golden carries junction vectors: a request records the kind of link it crosses (none, a symlink, or a junction) and whether that link dangles, and `TestVirtualPathHarness_GoldenVectors` compares the real validator with the model's fixed decision `dAccepts` on every request. It materialises a junction request with a real junction on Windows and fails there if it replayed none; a dangling request is a symlink to a missing name. Off Windows it leaves the junction requests out through the `fstree` capability policy and logs their count (6012 of 17580 requests); the test itself is never reported as skipped, because it is a killer. The golden scope is two requests per instance: a request's decision reads only that request and the harness configuration, so two requests still show each of the 81 request shapes, and the replay fails without `-short` if one is missing. The 30 s replay budget is a recorded figure enforced by no step or test; an overrun found when a budget is measured narrows the golden scope, and nothing in CI checks the figure afterwards. One windows-latest sample measured the `ModulePathContainment` golden at 34.4 s before the custom-check and env replays were added. The Windows golden-replay budget is 60 s: three windows-latest attempts of head 6ce407b0 (CI run 43685015559, attempts 1-3, `Test (windows)` jobs 132723566961, 132726133020, and 132728380605) measured the `ModulePathContainment` golden at 23.39 s, 23.40 s, and 39.39 s, so `ceil(1.5 x 39.39)` = 60 s. The basis is `gotestsum -- -race -short`, which replays every 16th instance. The same head's `Test (macos)` job (132723566989) measured it at 5.47 s, inside the 30 s figure. The measured head's golden-replay inputs equal the archived head's (decision 32); the 56 s figure from head 3575b947 is withdrawn, because the stage-4 changes regenerated the `ModulePathContainment` golden. Like the 30 s Linux and macOS figure, no step or test enforces it. If it is not accepted, memoising the custom-check and env replays per (tree, path) is the lever, not cutting instances (harden-formal-instruments design D12).

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
`TestWatch_TraceHarness`, `TestHostCallbackToken_TraceHarness`, and
`TestLockIntegrity_TraceHarness`) through `tlatrace.WriteSuite`, which writes the
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

The token harness drives a started `sshserver.Server` on a fake clock and logs
in with real `golang.org/x/crypto/ssh` clients over loopback; its projection
reads token admissibility under the token lock and connection liveness from
the client side, cross-checked against the server's connection map. The lock
harness runs the real `Resolver.Sync`, `LoadDeclaredFromLock` followed by
`VendorModules`, the discovery-time hash check, and command-scope admission
over local fixture trees served by commit, and maps real content hashes to the
model's `Good`, `Evil`, and `Other`. `tlatrace.WriteSuite` drops duplicate traces
and fails when a targeted mutation equals a recorded trace. A `[[trace]]` entry may override the model's base constants with
`constants` (the lock suite sets `LegacyLock = "TRUE"`); an override of an
undeclared constant fails the run. A behaviour the base constants accept
because of an open finding (F11, F12) is never a targeted mutation.

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
| F8 | concurrent module commands | Lost updates: sync or update saves a lock built from files read before its fetch, dropping a concurrent add or resurrecting a concurrent remove, and a duplicate add's rollback erases another process's successful add | Fixed (module-concurrency-rollback): `invowk module add`, `remove`, `sync`, `update`, `tidy` and `vendor` (and a writing `agent mod change`) take the project's OS advisory lock before their first read and release it after their last write or rollback; the pre-fix code is kept as `ConcurrentModuleEdits.mutantPreFixF8SyncOverwritesConcurrentAdd`, `mutantPreFixF8UpdateOverwritesConcurrentAdd`, `mutantPreFixF8UpdateResurrectsConcurrentRemove` and `mutantPreFixF8DuplicateAddRollsBackOtherAdd`; `syncAddProjectLock`, `updateAddProjectLock`, `updateRemoveProjectLock` and `addAddSameProjectLock` pass; replays `TestConcurrentEdits_F8SyncKeepsConcurrentAdd`, `TestConcurrentEdits_F8UpdateKeepsConcurrentAdd`, `TestConcurrentEdits_F8UpdateKeepsConcurrentRemove` and `TestConcurrentEdits_F8DuplicateAddKeepsOtherAdd` | (shipped) |
| F9 | module cache | Two projects sharing the cache fetch two versions of one repository through its single worktree, so a lock records one version's commit with the other's content hash (`ConcurrentModuleEdits.findingF9SharedWorktreeRecordsWrongContent`; `TestConcurrentEdits_FindingF9_SharedWorktreeRecordsWrongContent`) | Open: fix requires maintainer approval | a per-source mutex from checkout until the verified cache copy, always taken after the project mutex |
| F10 | module add rollback | The rollback restored `invowkmod.cue` and the lock with `os.WriteFile`, which truncates in place, so a concurrent reader saw an empty or partial file | Fixed (module-concurrency-rollback): the rollback restores a regular file with `fspath.AtomicWriteFile` and a link by renaming a new link over the path, so a reader sees the old or the new file; the pre-fix code is kept as `ConcurrentModuleEdits.mutantPreFixF10NonAtomicRollback`; `rollbackReadAtomicRestore` passes; the interleaving stays model-only (a rename has no intermediate state), and `TestConcurrentEdits_F10RestoreReplacesFile` checks that the restore replaces the file | (shipped) |
| F11 | SSH tokens | `Server.Stop` keeps authenticated connections open: `ssh.Server.Shutdown` closes the listener, waits for connections until `ShutdownTimeout`, and closes none, so `Stop` returns `context.DeadlineExceeded` with the client still connected. Production calls `Stop` after executions end, when revocation has already closed their connections, so F11 matters only for sessions of executions still running at `Stop`, for example under a different cancellation ordering | Open: `HostCallbackToken.findingF11StopKeepsSessions`; Go replay `TestHostCallbackToken_StopLeavesAuthenticatedConnectionOpen` | `HostCallbackToken.noSessionAfterStopFixed` (`StopClosesSessions`): Stop closes every tracked connection |
| F12 | lock integrity | Command-scope admission admits a sibling's vendored copy when the caller's lock entry has no content hash; `LoadCommandScopeLock` loads the lock without `RequireV2`, so a v1.0 lock reaches that branch | Open: `LockIntegrity.findingF12HashlessSiblingAdmission`; Go replay `TestLockIntegrity_HashlessCallerEntryAdmitsSiblingCopy` | `LockIntegrity.f12AdmitRequiresCallerHash` (`AdmitRequiresCallerHash`): admission refuses a caller entry without a hash |
| F13 | module path containment | A module's `script.file` (or custom-check file) into `invowk_modules/` reaches a symlink that `Load` never scans, and the read follows it outside the module | Fixed (module-read-physical-containment): both resolvers check the path through an `os.Root` opened on the module directory before the injected reader runs; the pre-fix code is kept as `ModulePathContainment.mutantPreFixF13LexicalScriptContainment`; `scriptReadContainedPhysical` passes; replay `TestModulePathContainment_VendoredSymlinkScriptReadDenied` | (shipped) |
| F14 | virtual path harness | The pre-fix `normalizeExistingOrParent` evaluated only the path and its parent, so a path with two or more missing components below an escaping symlink was judged lexically and admitted; the write landed outside the allowed root | Fixed (virtual-path-physical-normalisation): the resolver walks to the deepest existing ancestor through every link and joins the missing components to it, with no lexical fallback; the pre-fix code is kept as `VirtualPathHarness.mutantPreFixF14ParentOnlyEval`; `harnessContainedFixed` passes; replay `TestVirtualPathHarness_DeepMissingPathThroughSymlinkDenied` | (shipped) |
| F15 | virtual path harness | A module-declared `workdir` outside the module widened the allowed roots under `restricted` access, admitting reads that are not cwd operations. Reproduced before the fix by the replay `TestVirtualPathHarness_F15WorkdirWidening` at base commit 3bb45325 | Fixed (module-workdir-roots): the workdir and `@work` are allowed roots only when the workdir is trusted (the CLI override, a workdir of an invowkfile outside a module, a module's default workdir) or resolves physically inside the module (`virtualWorkdir.isReadRoot`); the pre-fix rule is kept as `VirtualPathHarness.mutantPreFixF15WorkdirRoot`; `harnessContainedFixed` and `trustedWorkdirReadableFixed` pass; replay `TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot` | (shipped) |
| F16 | module path containment | A Windows junction inside a module is not `ModeSymlink` (Go 1.23+), so `IsModule` and `Load` admit a module with an inner non-vendored junction and a script read follows it out of the module (both copies and the hash stay contained: `copyHashJunctionScan`) | Fixed (module-read-physical-containment, outcome d): the admission is unchanged, but the `os.Root` read check denies the script and env reads through the junction; the pre-fix configuration is kept as `ModulePathContainment.mutantPreFixF16JunctionScriptRead`; `scriptReadScanPhysical` passes; replay `TestModulePathContainment_F16Junction`, escape on windows-latest CI run 43657442894 (job 132639016048), contained on CI run 44648263846 (job 135996334113) | (shipped) |
| F17 | module path containment | An env file declared as `invowk_modules/x` is read at runtime through the same symlink hole, with no containment check in `LoadEnvFile` | Fixed (module-read-physical-containment): in module context `LoadEnvFile` reads through an `os.Root` on the module root and fails with `ErrEnvFileOutsideModule`; the pre-fix code is kept as `ModulePathContainment.mutantPreFixF17EnvNoPhysicalContainment`; `envReadContainedPhysical` passes; replay `TestModulePathContainment_VendoredSymlinkEnvReadDenied` | (shipped) |
| F18 | virtual path harness | `filepath.EvalSymlinks` does not follow a Windows junction (Go 1.23+): a path that ends at a junction comes back unchanged, and a path through one fails with an error that `errors.Is(err, fs.ErrNotExist)` reports as true. `normalizeExistingOrParent` therefore judged a path through a junction by the junction's own location, and under `restricted` access admitted an existing file, a new file, and a deep new file below a junction that leaves the allowed root. Observed on windows-latest: CI run 44523520503, job 135557989975 | Fixed (virtual-path-physical-normalisation), recorded as fixed after the observed escape: `walkVirtualPath` reads every component with `os.Lstat` and follows junctions with `os.Readlink`; the pre-fix code is kept as `VirtualPathHarness.mutantPreFixF18JunctionNotFollowed`; `harnessContainedFixed` passes; replay `TestVirtualPathHarness_JunctionEscapeDenied` (an existing file, a new file, and a new file two missing components below the junction; required on Windows, skipped elsewhere by the capability policy, so never a killer), and on Linux the junction branch is covered by `TestVirtualPathHarness_WalkOverFakeFilesystem` | (shipped) |
| F19 | virtual path harness | A symlink whose target does not exist could not be evaluated by `filepath.EvalSymlinks`, so the pre-fix `normalizeExistingOrParent` fell back to the link's parent and judged the link as a missing plain name inside the root. Under `restricted` access a write to a dangling link whose target lies outside was admitted and created the target outside (`echo x > link`); no deep path is needed. Observed: `TestVirtualPathHarness_DanglingLinkEscapeDenied` fails against the product code of `main` at c2fe96ff. To re-run: export c2fe96ff into a scratch directory (`git archive c2fe96ff \| tar -x -C <dir>`), replace `internal/runtime/virtual_path_harness_formal_test.go` there with the current one, and run `go test -run 'TestVirtualPathHarness_(DanglingLinkEscapeDenied\|DeepMissingPathThroughSymlinkDenied)' ./internal/runtime/` in it | Fixed (virtual-path-physical-normalisation), recorded as fixed after the observed escape: the walk follows a dangling link like any other, so the request is judged by the directory its target would be created in, and a link target with `..` after a missing component is refused; the pre-fix code is kept as `VirtualPathHarness.mutantPreFixF19DanglingLinkAsMissingName`; `harnessContainedFixed` passes; replay `TestVirtualPathHarness_DanglingLinkEscapeDenied` | (shipped) |

F8, F9 and F10 were found by `model-concurrent-lock-writes` and are fixed by `module-concurrency-rollback` (F9 when its source lock lands); F11 and F12 were found by `trace-validate-token-and-lock` and stay open.

**Virtual path harness: what the model does not hold (F14, F18 and F19 fixes).**
A `VirtualPathHarness` request is a path the resolver can resolve. A path it
refuses under `restricted` access (a link cycle, an unreadable link or reparse
point, a file used as a directory, a link target with `..` after a missing
component, a missing drive or share) has no request in the model, and the code
denies it with the resolution error; `TestVirtualPathHarness_WalkOverFakeFilesystem`,
the third leg of the F19 replay, the rapid property's "cannot be resolved" class
and the golden replay's resolution-error checks cover that. Under `full` access
the model accepts every request, and the code only chooses the form of the path
an operation runs on (a link whose target does not resolve keeps its own path).
The check and the open are two steps: a concurrent process that replaces a
directory on the resolved path with a link after the check can still redirect
the operation. That race is a residual risk and a model abstraction (the
`validate` row); it is not closed.

**Code stricter than the model (F13 and F17 fixes).** `os.Root` rejects a link with an
absolute target even when the target lies inside the module, while the model's
`physContains` accepts it. `TestModulePathContainment_GoldenVectors` therefore
replays the golden twice: with relative link targets, where the code equals
`dAccepts` on every vector, and with absolute targets, where the accepted
reads whose reference is a link (384 `ScriptRead` and 384 `EnvRead` vectors)
are classified "in-module link with an absolute target" and must be denied (the
pass fails if a class is empty). The env replay takes the code's verdict only
from `LoadEnvFile`'s error class (`ErrEnvFileOutsideModule`).
`TestModulePathContainment_AbsoluteInModuleLinkDenied` pins the same denial
outside the golden scope, for both script resolvers and the env read. Junctions always have absolute targets, so the
contained script and env reads also deny a path through a junction, which
fixed F16 (below).

**F16 was decided on Windows CI** (harden-formal-instruments design D7, outcome
"escape"): junction creation works on windows-latest, and
`TestModulePathContainment_F16Junction` showed the module admitted and the
script read of `j/secret` returning the outside sentinel, while both copies
and the hash stayed contained (run 43657442894). module-read-physical-containment
fixed it (outcome d): on CI run 44648263846 (job 135996334113) the leg showed the
module still admitted, the script read through the junction denied by the
`os.Root` check, and the copies and the hash contained. The leg asserts that
behaviour, with an env-read arm, the rapid property fails on every junction
escape, and the model keeps the pre-fix configuration as
`mutantPreFixF16JunctionScriptRead` next to `scriptReadScanPhysical`. On Go
1.27.4, `os.Root.Stat` on Windows panics on a link to its own directory; both
contained reads recover it (run 44645245657, job 135986033972). The copies and the
hash have a model command for those modules too: base admission counts
junctions as scanned, so `copyHashStaysInside` never examines a module with an
inner junction, and `copyHashJunctionScan` checks copy and hash containment
over the modules today's Windows scan admits (antecedent `anteCopyHashJunction`,
mutant `mutantCopyFollowsJunction`; decision 29). The leg's copy and hash arms
fail on an error, so "contained" cannot mean "failed" (decision 30).

Two hypotheses were refuted: the symlinked-hash-root escape
(`computeModuleHash` on a symlinked root hashes an empty tree, but every caller
is gated by `IsModule`'s Lstat, so `ModulePathContainment` keeps
`isModuleRejectsLinkedRoot` as a fact and `mutantDropLinkedRootReject` shows the
escape without it), and the symlinked module root bypassing `Load`'s scan (every
production `Load` caller is gated by `IsModule` or is user-controlled, design
D8.4). Earlier suspected finding ids from sibling changes stay as those changes
record them.

Two further hypotheses were refuted: an empty `SourceID` on module targets
(`ScopeConstruction` keeps discovery's guarantee as a fact, and a mutant shows
what breaks without it), and an empty identity in v2.0 locks
(`LockIdentity.v2NeverUnhashed`). No login succeeds after its execution ends
(`HostCallbackToken.noAuthAfterExecution`), and the watch loop never loses a
burst (`Watch.noLostBurst`).

`ConcurrentModuleEdits` also reported a vendor case that never got an id:
`module vendor` returned ok after copying the modules named by a lock that a
concurrent update replaced. Discovery rejected the stale copy against the new
lock, so it failed closed. The project lock fixes it (module-concurrency-rollback):
vendor reads `invowkmod.cue` and the lock and copies the modules under it;
`vendorSyncProjectLock` passes, the pre-fix code is kept as
`ConcurrentModuleEdits.mutantPreFixVendorFromStaleLock`, and the replay
`TestConcurrentEdits_VendorFromStaleLock` checks that the concurrent lock write
waits until the copies are complete. None of the F8, F9, or F10 hypotheses was
refuted.

## CI budgets

`.github/workflows/formal-verification.yml` runs on the weekly schedule, on
dispatch, and on every pull request. Its `Classify changed paths` step runs
`scripts/formal.py affected`, which answers `run=false` only for a pull request
whose changed paths match none of `[ci] paths` in `formal/manifest.toml`
(`fnmatch.fnmatchcase`, where `*` also matches `/`). Every other event, any
error, an empty diff, and a manifest that does not load or validate answer
`run=true`. Unrelated pull requests skip Setup Go, Setup Java, and every formal
step, and still report success under the same check name.
`scripts/test_formal.py` fails unless `[ci] paths` matches every
correspondence-table file, binding-test file, trace package, and golden output.
Code a model depends on that no table names must be added by hand.

The replay step runs the binding tests named in the correspondence tables'
binding cells, minus the trace harnesses, from `scripts/formal.py replay-plan`.

Budgets have two tiers:

- **Hard:** step `timeout-minutes` on `Check models`, `Trace validation`, the
  replay step, `Binding reach`, and `Deep property tests`, each
  `max(2, ceil(4 x max CI seconds / 60))`. The job `timeout-minutes` must
  exceed their sum plus 3, which `scripts/test_formal.py` checks, so a step
  timeout fires first and the upload step (`failure() || cancelled()`) runs.
- **Soft:** `budget_seconds` on every `[[model]]` and `[[trace]]`, compared
  with the runner's `timing` lines: a model's summed checker seconds (Alloy
  commands plus golden re-enumeration, or every TLC command), and a suite's
  harness run plus every trace check. An overrun prints `WARNING` (and a
  `::warning::` in CI) and never changes the exit status. `budget_source =
  "ci"` means `ceil(1.5 x max CI seconds)`; `"local"` is provisional,
  `ceil(3 x local seconds)`, and gets a `::notice::` in CI. Under Actions the
  runner appends a timing table to the step summary.

Every TLC command that expects `pass` records `distinct_states`; TLC runs with
`-workers 1`, so the count is deterministic, and drift warns.

Measurements (2026-09-25; `[ci.budget]` records the run IDs):

| Step | Measured | Hard limit | Source |
|---|---|---|---|
| Check models | 369 s (CI run 36180026549, full model set); 173 s local | 26 min | ci |
| Trace validation | 164 s (CI max over post-sibling PR runs); 25 s local | 12 min | ci |
| Replay golden vectors and property tests | 19 s (CI max); 3.3 s local | 2 min | ci |
| Deep property tests | never run in CI; 15.7 s local, assumed 4x slower on a 4-core runner | 2 min | local |
| Binding reach | 37.1 s in CI (run 43657442830, one run: a soft overrun of the 36 s local budget); 11.8 s local with a cold coverage build (3-9 s warm) | 3 min (the larger of `max(2, ceil(4 x 2 x 11.8 / 60))` = 2 and `ceil(4 x 37.1 / 60)` = 3, until three CI runs replace it) | local (`[reach]`, 36 s soft) |
| Job | | 49 min (the workflow's step timeouts sum to 45; the job must exceed that plus 3) | |

Every per-model and per-suite soft budget has `budget_source = "ci"`: three
`workflow_dispatch` runs on `main` (2026-09-25; `[ci.budget]` records the run
IDs) set them to `ceil(1.5 x CI max)` (promote-formal-ci-gate task 2.5). Only
the `[reach]` budget is still `local`, until three CI runs replace it.

## Promoting the lane

The lane is advisory until the promotion gate passes. The gate reads workflow
history with GET requests only and never changes repository settings.

1. Run `make formal-promotion-gate`. It passes only when the four most recent
   scheduled runs on `main` of the current pipeline (runs whose formal job has
   the `Classify changed paths` step) all succeeded on their first attempt,
   every heavy step concluded `success` rather than `skipped`, no two counted
   runs are more than 8 days apart, and the newest is at most 8 days old.
   `FORMAL_GATE_ARGS="--since YYYY-MM-DD"` adds a creation-date cut-off, and
   `--json` prints the verdict as JSON.
2. When it prints PASS, a maintainer adds the context
   `Models, golden vectors, and correspondence` (source: GitHub Actions) to the
   required status checks of the "Safety" ruleset (Settings, Rules), next to
   `goplint consumer gates`. For reference only, the equivalent API call is:

   ```sh
   gh api -X PUT repos/invowk/invowk/rulesets/12472716 --input - <<'EOF'
   {"rules": [{"type": "required_status_checks", "parameters": {
     "strict_required_status_checks_policy": false,
     "do_not_enforce_on_create": false,
     "required_status_checks": [
       {"context": "goplint consumer gates"},
       {"context": "Models, golden vectors, and correspondence", "integration_id": 15368}]}}]}
   EOF
   ```

   The PUT replaces the ruleset's whole `rules` list, so merge in the other
   rules from a fresh `gh api repos/invowk/invowk/rulesets/12472716` first.
3. Tick `adopt-formal-verification` task 8.5 with the date and the gate
   output, and drop the "Not a required status check" comment from the
   workflow header.

Done on 2026-11-14: the gate passed on four weekly runs (2026-10-19 to
2026-11-09) and the context is required, pinned to GitHub Actions
(`integration_id` 15368).

Rollback: remove the context from the ruleset's required status checks.

## Mutation testing

Mutation profiles run rapid with a fixed `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and
a bounded `RAPID_SHRINKTIME`, so kill and escape results stay reproducible.

The root profiles cannot say whether the bindings pin down the code a model
names: they mutate curated packages, run every package test under `-short`, and
credit only tests of the mutated package. The `formal-bindings` target set
(`make mutation-formal`) measures exactly that. It mutates only the functions
named by bound correspondence rows and, for each mutant, runs only the killer
tests of the rows naming the mutated function, across packages, through a
`go test -overlay`, without `-short`. Trace harnesses and characterisation
tests (finding replays and probes, marked by name or by a `characterisation:`
abstraction note) are never killers. The plan is generated from the tables at
run time, rows without a killer are reported with a reason, and a clean-code
pre-flight must pass before any mutant counts.

A survivor marks code the model claims to describe but the bindings do not
constrain. Each one is classified in `tools/mutation/triage/formal-bindings.toml`
(`equivalent`, `abstraction`, `binding-gap`, `model-gap`, `defect`), backed by
two recorded escaped reruns, and resolved by a stronger binding, a new model
property, a stated abstraction, or a finding. Closed gaps are listed in the
model's `calibration` by mutant id. `.agents/runbooks/mutation-testing.md` has the
profiles; `.agents/skills/formal-verification/SKILL.md` has the feedback loop.

The first run (2026-09-25) mutated 49 functions in 30 files in 28 minutes:
809 mutants, 310 killed, 441 escaped, 58 skipped (mutants that do not build),
0 errored, and 22 of the kills were timeouts. A stronger copy assertion in the
path-containment golden closed 19 escapes; the other 419 are deferred
binding gaps, grouped by function in `tasks/next/formal-binding-gaps.md`. No
survivor was shown to be a product defect, so no finding was recorded.

That run listed seven rows whose binding never calls their function. The
binding reach gate (`make formal-reach`, 2026-11-14) found eight: the seven,
plus `Auth` / `Server.ValidateToken`, which has no production caller at all
(the model's `Auth` is implemented by `Server.admitConn`). Six rows are
rebound to tests that call the real function and assert the model's verdict:
`workdirCwd` (`GetEffectiveWorkDir`, through
`TestModulePathContainment_ContainerWorkDirGolden`), both `readOp` rows
(`CustomCheckScript.ResolveWithFSAndModule` and `LoadEnvFile`, replayed on
every `ScriptRead` and `EnvRead` golden instance, F13 and F17 included), and
`allowedRoots`, `anchors`, and `access` (`TestVirtualPathHarness_RootConstruction`).
`anchors` carries only the narrow claim that anchors are explicit allowed roots
(`@config`, `@data`, `@cache`, `@state` and `@tmp` accepted, `@work` accepted
exactly when the workdir is an allowed root, `@home/.ssh` denied under
restricted access); the anchor paths are inputs from
the production resolver, not verified, because the model states no per-OS
layout. Two are unbound, with the reason in their abstraction cells: `Auth` (no
production caller; re-pointing it to `Server.admitConn` is follow-up work) and
`unpackRejectsEscape` (the model's destination check transcribes
`normalizeZIPPath` followed by `validateDestinationPath`, not the function
alone). Their 20 baselined survivors are orphaned and listed in `[triage]`.

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
