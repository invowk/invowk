## Context

Each formal model under `formal/` has a header correspondence table with the columns `| model element | Go symbol | file | binding | abstraction |`. `scripts/formal.py correspondence` checks that every named symbol is declared in its file and every named binding test exists. The tables are the only machine-readable statement of which Go code a model claims to describe and which tests bind it.

Mutation testing already exists (`scripts/mutation.sh`, go-mutesting v2.8.3 pinned in `go.mod`). Its `full` profile mutates the packages in `tools/mutation/root-packages.txt`, runs each package's own tests under `-short` with `--coverage --per-test`, and suppresses accepted survivors through `tools/mutation/baselines/root-baseline.json`. `export_rapid_determinism_env` pins `RAPID_SEED=20260924`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME=2s`.

**Landing order.** Per the orchestrator's decision, the changes land in this order:
1. `formal-infra-refinements`, alone. It adds Alloy commands generated from the manifest, golden format 2, a golden fingerprint that covers the generated golden command, `TraceBase.tla`, the `tlatrace` Recorder and WriteSuite, and a runner that rejects unknown manifest keys.
2. `trace-validate-token-and-lock`, `model-concurrent-lock-writes`, and `model-module-path-containment`, in parallel.
3. This change.
4. `promote-formal-ci-gate`, last.

This change is written against that machinery. Its target set is whatever the correspondence tables name when it is implemented, including the rows and bindings the three model changes add. Nothing in the plan is hard-coded to today's tables.

go-mutesting v2.8.3 facts that shape the design, read from the pinned module source:

1. `--match=<regex>` filters on the bare function name (the receiver is dropped). `--match='^TransitionToStarting$'` selects 11 mutants in `base.go`; `'Base\.TransitionToStarting'` selects none.
2. The built-in executor runs `go test -overlay=<mutant> <mutated package>` only. With `--per-test` it appends its own `-run` after `--test-flags`, and the last `-run` wins.
3. `--exec=<cmd>` receives `MUTATE_ORIGINAL`, `MUTATE_CHANGED`, `MUTATE_PACKAGE`, and `MUTATE_TIMEOUT`, and maps exit codes 0/1/2 to killed/escaped/skipped. It forces one worker.
4. `--noop` is ignored together with `--exec` (`runNoopChecks`, `engine.go:579-582`). Nothing checks the clean-code run unless the wrapper does it.
5. Mutant IDs are `baseline.MutantID(relFile, mutator, diff)` (`internal/baseline/baseline.go:40-55`): file, mutator, and diff lines only. The IDs do not depend on the executor.

**Measurement on the post-sibling tree (task 2.5, 2026-09-25, on top of `model-module-path-containment`).** The HEAD 4831f23d figures (14 files, 29 leaves, 519 candidates) are superseded:
- The bound rows name 49 functions in 30 files; 51 rows are unbound (`no-symbol` 5, `no-binding` 21, `type-symbol` 3, `trace-only` 2, `characterisation-only` 20).
- The dry-run gives 982 candidates (969 and 13 in the two match groups, see D2); go-mutesting's deduplication leaves 809 executed mutants.
- 15 of the 30 files are bound only or partly from another package, mostly through `internal/app/moduleops`'s path-containment golden.
- The clean pre-flight (binding tests of a whole file, cold build excluded) takes 0.2–3.3 s per file, 7.2 s at most (`internal/runtime/dotenv.go`, whose killer lives in `moduleops`), and about 72 s in total; every derived exec timeout is 10–36 s.

## Goals / Non-Goals

**Goals:**
- Measure how much of the code named in the correspondence tables the binding tests alone constrain, per named function.
- Derive every target, test, and package from the tables so the profile cannot drift from the models.
- Keep kill and escape status reproducible and honest: clean code must pass its bindings before any mutant counts.
- Turn every survivor into a classified decision: accept it, defer it with a follow-up, strengthen a binding or the model, or record a defect as a finding.
- Stay a manual, advisory signal.

**Non-Goals:**
- A PR gate, a scheduled run, or membership in `make test`.
- Mutating callees the tables do not name. A helper that matters gets its own row.
- Running TLC or trace validation per mutant.
- Product fixes. A defect found through this profile is recorded as a finding only (D7).
- Sharding or parallel execution (deferred by the orchestrator decision).
- Changing go-mutesting, or the root `full` profile and its baseline.

## Decisions

### D1. The tables are the only source; the plan is generated at run time
A new module, `scripts/formal_mutation.py`, imports `correspondence_rows`, `load_manifest`, and `check_correspondence` from `scripts/formal.py`. It keeps the new code out of the heavily edited `formal.py`, as `promote-formal-ci-gate` does with `formal_promotion_gate.py`. Its `plan --out <report-dir>` subcommand emits `formal-plan.json`:
- `functions`: one entry per named function leaf, with:
  - its file;
  - its line range. gofmt'd top-level `func` declarations are found with `^func …name(` and closed by the next `^}`;
  - the rows (model and element) that name it;
  - its killer tests and their owning packages. A killer test is a binding name after the exclusions below, located by its `func TestX(` declaration and resolved with `go list` on the declaring directory.
- `files`: the target files, and `match`: the union regex `^(leaf|…)$`.
- `unbound`: rows with their reason:
  - `no-binding`: the binding cell is `-`;
  - `type-symbol`: the row names a type or interface;
  - `trace-only`: every binding is a trace harness;
  - `characterisation-only`: every binding is a characterisation test.
- `preflight`: per-file clean wall time and the derived timeout (D3a).

Binding names excluded from killer sets:
- `Test*_TraceHarness`: these skip without `INVOWK_FORMAL_TRACE_DIR` (`tlatrace.go:63`), so they would make every mutant escape.
- Characterisation or finding-replay tests (D8). These assert today's behaviour, so they are not killers of the property.

The plan fails closed, with a non-zero exit that names the model and row, when:
1. the union regex matches a function in a target file that no row names for that file (a collision);
2. a killer test is declared in zero packages or in more than one;
3. the plan has no function targets;
4. `check_correspondence` fails, including the completeness guard in D8.

*Alternatives considered:* a committed target list, which is a second source that can drift; and deriving targets from the binding tests' coverage, which is circular. Generating at run time keeps a single source. The plan is copied into the report directory, so a run stays auditable.

### D2. Function-scoped mutation with `--match`, file targets
The target files are passed together with the single union `--match` regex, and the collision guard in D1 keeps the union exact. Per-file invocations were rejected: each writes its own report, and `--update-baseline` overwrites rather than merges.

### D3. A custom exec runs only the mutated function's bindings, across packages, through an overlay
`scripts/mutation-formal-exec.sh` does the following for each mutant:
1. Refuse to run (exit 3) unless `MUTATION_FORMAL_PLAN`, `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME` are set.
2. Take the first changed line from `diff MUTATE_ORIGINAL MUTATE_CHANGED`, and find the function whose line range contains it. This is well defined because `--match` mutates only inside named function declarations. If no function or file matches, exit 3.
3. Write an overlay JSON that maps the original file to `MUTATE_CHANGED`. The tracked file is never touched.
4. Run `go test -count=1 -overlay=<ovl> -timeout <file timeout>s -run '^(<that function's killer tests>)$' <their packages…>`.
5. Map the result:
   - pass → 1 (escaped);
   - a test failure → 0 (killed);
   - a build or setup failure → 2 (skipped);
   - a timeout → 0 (killed; the log records it as `timeout-kill`, and the run summary counts these per file);
   - any other `go` failure → 3 (errored).

Tests run without `-short` (the orchestrator decision; golden vectors replay every instance), without `-race`, and without `--coverage` or `--per-test`, whose coverage comes from the mutated package's own tests.

*Alternatives considered:*
- The built-in executor with a `-run` filter. It loses the cross-package bindings of 3 of the 14 files: `command_scope.go` and `lock_integrity.go` entirely, and `vendored_policy.go` partly. The 5 modulesync and sshserver files are also outside `root-packages.txt`. `--per-test` would override the filter anyway.
- Per-file binding unions. Rejected because a test that binds a different function or a different model could kill the mutant. For example, `TestLockIntegrity_HashlessEntryIsRejected` would kill LockIdentity mutants in `verify.go`, which hides per-model gaps and misattributes kills in the ledger.

### D3a. Pre-flight on clean code
Because `--noop` is ignored with `--exec`, the wrapper runs the exec script once per target file before it invokes go-mutesting, with `MUTATE_CHANGED=MUTATE_ORIGINAL`, and uses `go test -json` for each function's killer set. Every run must:
- exit 1 (all tests pass);
- report at least one killer test per function as run and passed (not skipped).

Any other outcome fails the whole profile before a mutant runs. The clean wall time per file is written to `formal-plan.json`. The file's exec timeout is `max(10 s, 5 × clean)`, overridable with `MUTATION_FORMAL_EXEC_TIMEOUT`, and replaces a fixed guess.

### D4. Wiring into `scripts/mutation.sh`
- Add `--target-set root|formal-bindings` (`MUTATION_TARGET_SET`, default `root`). The usage text and the Makefile help list document `MUTATION_TARGET_SET` and `MUTATION_FORMAL_EXEC_TIMEOUT`.
- For `formal-bindings`:
  - `resolve_targets` runs `python3 scripts/formal_mutation.py plan`, then the pre-flight;
  - the arguments are `--match`, `--exec`, the maximum per-file timeout as `--exec-timeout` (the exec script applies the per-file value itself), `--baseline=tools/mutation/baselines/formal-bindings-baseline.json`, and the logger flags;
  - reports go to `artifacts/mutation/<profile>/formal-bindings/`;
  - `pr` rejects the target set;
  - `dirty_path_is_allowed` gains the new baseline and ledger;
  - `dry-run` uses the built-in `--dry-run`.
- Make targets: `mutation-formal-dry-run`, `mutation-formal`, `mutation-formal-baseline-update`, and `mutation-formal-rerun MUTATION_MUTANT_ID=…`. All are advisory by default.

### D5. Determinism and rerun evidence
- The exec script runs inside the `export_rapid_determinism_env` subshell and enforces all three rapid variables (D3, step 1).
- `-count=1` disables the result cache.
- Two tests depend on goroutine scheduling: `TestServerbase_ConcurrentSafetyInvariants` and `TestRevocationRacesAuthenticationSafely`. Each `mutation-formal-rerun` appends `{id, status, timestamp, digest}` (plus `commit`, information only) to the tracked evidence file `tools/mutation/triage/formal-bindings-reruns.jsonl`, where `digest` is a SHA-256 over the plan inputs (the mutated target files and the files declaring their killer tests, sorted by path, each as path + NUL + bytes).
- The baseline records the same digest of the inputs it was generated from. The triage check requires, for every baselined ID, at least two `escaped` rerun records whose digest equals the baseline's, and no other status at that digest. Commit ancestry is not used: every PR is squash-merged and its branch deleted, so the commits the evidence was recorded at never reach `main`, while the digest survives squash merges, rebases, and fresh clones as long as the inputs are unchanged. `confirmed_reruns` is therefore derived from recorded evidence, not typed in by hand.

### D6. Separate baseline and a checked triage ledger
go-mutesting's baseline stores only `{id, file, mutator, line}`. The reasons live in `tools/mutation/triage/formal-bindings.toml`:

```toml
[[survivor]]
id = "<stable id>"
file = "pkg/invowkmod/verify.go"
symbol = "EvaluateVendoredModuleHash"
model = "LockIdentity"
element = "evaluation"
class = "abstraction"   # equivalent | abstraction | binding-gap | model-gap | defect
reason = "content hashing abstracted: a single hashed entry is \"compared\""
follow_up = ""          # required for binding-gap, model-gap (change or task) and defect (finding id)
```

`python3 scripts/formal_mutation.py triage --check` fails when:
- the ID sets in the baseline and the ledger differ;
- `binding-gap` or `model-gap` has no `follow_up`;
- `defect` has no `follow_up` naming a finding id that exists as a `finding =` command in `formal/manifest.toml`;
- an `abstraction` reason is not a substring of the abstraction cell of the row identified by `model` and `element`, after collapsing whitespace (quotes are compared literally, as the example shows);
- the baseline carries no input digest, or an ID lacks two escaped rerun records at the baseline's digest (D5);
- an ID recorded as closed (see D7) is missing from its model's `calibration` text.

IDs are identical across executors (Context, fact 5), so this baseline and the root baseline can be compared directly when needed.

*Revision (evidence identity):* the first implementation stamped the baseline and each rerun with a commit and accepted a rerun only when `git merge-base --is-ancestor <baseline commit> <rerun commit>` held. Squash merges drop those commits from `main`'s history, so the check would fail on `main` and in fresh clones. The digest replaced the commit. The existing evidence was migrated without rerunning: the digest was recomputed from each stamped commit's tree (`git show <commit>:<path>`) and compared with the digest at the rebased HEAD; the baseline commit and both rerun commits matched, so all 860 records were kept and none dropped.

The check runs in `make test-scripts` only, through `scripts/test_formal_mutation.py` and a `triage --check` over the committed files, and from `mutation-formal-baseline-update`. It does not join `make formal`, so `promote-formal-ci-gate`'s `[ci] paths` need not cover the two files.

### D7. Survivors feed back into bindings, models, or findings
Each survivor is resolved in one of these ways:
1. **binding-gap, closed**: the model states the property, but the binding misses it. Strengthen the Go binding (widen the golden scope, add a generator dimension, or add an assertion). A focused rerun must report the mutant killed. Then append `mutation <id>: <one-line defect>` to the model's `calibration` text, and remove the mutant from the baseline and the ledger. Only a closed record needs to stay checkable, so the ledger keeps a `[[closed]]` table of `{id, model}` that the calibration check reads.
2. **model-gap, closed**: add a fact or property with its rejecting `mutant_of` command and antecedent, extend the table, regenerate the golden files if the scope changed, and bind it. Then proceed as in step 1.
3. **binding-gap or model-gap, deferred**: the mutant stays in the ledger with a `follow_up`.
4. **abstraction**: make sure the row's abstraction cell states the abstraction, then baseline the mutant.
5. **equivalent**: baseline it with the reason.
6. **defect**: a strengthened or new binding fails on unmodified code, so the code violates the model's intent. Following the orchestrator decision, record it as a finding under the next free id after those taken by `model-module-path-containment` at landing time (F8–F10, F11–F12, and F13 onward are already allocated). A finding record consists of:
   - a `finding =` command next to a passing fix configuration;
   - a Go characterisation test that asserts today's behaviour and is tabled as a characterisation test (D8);
   - a `formal/README.md` Findings row.

   The strengthened binding that fails is not committed as a failing test. The product fix is deferred to a later change after maintainer approval. The mutant stays in the ledger as `defect`, with `follow_up` set to the finding id.

A survivor whose behaviour is decided by a helper the table does not name is resolved by adding a row, with model-gap rigour, or by an abstraction note on the calling row.

Scope rule for this change's first run (task 7.3): close a gap only when the fix is an assertion or generator change inside an existing binding test. Everything else is deferred with a follow-up.

### D8. Correspondence extensions (in `scripts/formal.py`)
- **Multi-test cells.** The binding cell may list several tests, separated by commas, and `check_correspondence` validates each. This supersedes the singular "binding test" column wording in the `adopt-formal-verification` requirement (spec scenario; task 1.4).
- **Completeness guard.** It scans `*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go`, and `*_trace_test.go`:
  - every `Test*_TraceHarness` must sit in a package declared by a `[[trace]]` suite, and may be tabled;
  - every other test must appear in some row's binding cell.
- **Characterisation policy.** A finding-replay or characterisation test asserts today's defective behaviour. It is either tabled on its finding row with an abstraction note that begins `characterisation:`, or named `Test…_Characterisation` / `Test…_FindingF<n>`. Both forms are recognised as characterisation tests and excluded from killer sets (D1).
- **Tabling after the siblings land.** The guard is run on the tree after the three preceding changes. Every test it reports is tabled. Today's only gaps are `TestScopeConstruction_MatchesIntent` and `TestTidyToFixedPoint_GoldenVectors`; the sibling changes add more.
- **Golden regeneration.** Whether table edits force golden regeneration depends on the fingerprint defined by `formal-infra-refinements`. If it still hashes the whole `.als` file, run `make formal-golden`; with the jars from `python3 scripts/formal.py fetch` and Java 25, this is local work.

## Risks / Trade-offs

- [Runtime] Today's clean-cost estimate is roughly 519 mutants × 0.2–2.5 s, or 10–25 min on one worker. Timeout-bound mutants add up to N × their file timeout. The 519 candidates include 33 loop-break and select-removal mutants in `Watcher.Run`, the serverbase transitions, `SendError`, and `admitConn`, which are likely to deadlock. The sibling changes add rows, and the containment golden replays build real directory trees for every instance, at an unknown cost. → Mitigation:
  - the first run records wall time, timeout-kill count per file, and clean cost per package (task 7.1);
  - the manual job starts with `timeout-minutes: 90`.

  If a full run exceeds that budget, the rule is: raise the job budget, up to the GitHub-hosted limit, and open the deferred sharding follow-up. A per-model `-short` opt-in was considered and **rejected**, because the orchestrator decided that golden replays run without `-short`, so that the full binding strength is what is measured.
- [Flaky kills or escapes from concurrency tests] → rerun evidence is recorded and required (D5).
- [Survivors in unbound rows stay invisible] → `unbound-rows.txt` lists each such row with its reason and candidate count. An opt-in `--include-unbound` flag is a possible follow-up.
- [A later same-name function could be selected by the leaf union] → the collision guard fails closed.
- [Characterisation tests misclassified as killers] → the naming and abstraction-note convention (D8) plus the completeness guard. A mistabled characterisation test shows up as a mutant killed by a test that asserts buggy behaviour, which triage can spot.
- [Merge conflicts with the five sibling changes in `scripts/formal.py` and the tables] → the new logic lives in `scripts/formal_mutation.py`; only the D8 extensions touch `formal.py`; the change lands after `formal-infra-refinements` and the three model changes, and before `promote-formal-ci-gate`.
- [The go-mutesting exec contract changes on upgrade] → `scripts/test_mutation.sh` covers the exec script with fake `go` stubs, and the upgrade checklist re-verifies report contracts.

## First run (task 7.1)

`make mutation-formal` on a clean tree at 6f07db5a (2026-09-25, one local worker):
- **Wall time:** 1673 s (28 min), including the ~90 s pre-flight; well inside the 90-minute job budget, so no sharding follow-up was opened.
- **Counts:** 809 mutants: 310 killed, 441 escaped, 58 skipped (mutants that do not build), 0 errored. 22 kills were timeouts: 13 in `internal/watch/watcher.go`, 8 in `internal/core/serverbase/base.go`, 1 in `internal/sshserver/server_auth.go`.
- **Escapes per file:** `virtual_policy.go` 58, `vendor.go` 43, `watcher.go` 38, `modulecache/cache.go` 23, `server_auth.go` 22, `serverbase/base.go` 21, `provision/helpers.go` 21, `invowkfile.go` 20, `content_hash.go` 17, `dependency.go` 15, `modulesync/cache.go` 15, `verify.go` 14, `command_scope.go` 13, `validation_filesystem.go` 13, `packaging.go` 12, `resolver.go` 12, `dotenv.go` 11, `resolver_tidy.go` 11, `implementation.go` 10, `deps.go` 8, `container_provision.go` 8, `script_file_path.go` 7, `lock_integrity.go` 7, `vendored_policy.go` 6, `server_conns.go` 5, `atomic.go` 4, `operations_validate.go` 3, `transitive_policy.go` 3, `virtual_filesystem.go` 1, `operations.go` 0.
- **Rows whose binding never calls the function:** every mutant of `validateDestinationPath`, `LoadEnvFile`, `CustomCheckScript.ResolveWithFSAndModule`, `Invowkfile.GetEffectiveWorkDir`, and `VirtualFilesystemConfig.EffectiveAccess` escaped; `newVirtualPathResolverForFilesystem` and `standardVirtualAnchorsForOS` are bypassed by the harness golden, which builds its validator directly.

**Triage (tasks 7.2–7.5).** One gap was closed under the D7 scope rule: `TestModulePathContainment_GoldenVectors` now compares both module copies with an independent walk of the source, which kills 19 copy mutants (focused reruns recorded them killed; they are listed under `[[closed]]` and in `ModulePathContainment`'s calibration). The baseline-update run at ea14dd5c left 420 distinct survivors (421 entries: one id names the same change on two lines). Each was rerun twice and escaped both times, and each is a deferred `binding-gap` whose follow-up is its function's section in `tasks/next/formal-binding-gaps.md`. One `server_conns.go` mutant escaped in the first run and was killed in the second (the `TestRevocationRacesAuthenticationSafely` race), which is the flakiness the rerun evidence guards against. No survivor was shown to be a defect, so no finding was recorded and F18 stays free.

**Deviation from D2 (match groups).** The single union `--match` could not be exact: the leaf `Validate` of `ScriptFilePath.Validate` selects 23 other `Validate` methods in 10 target files, which the collision guard rejects. The plan therefore partitions the files into the fewest go-mutesting invocations whose union regex is exact (greedy; two today), writes each group's reports under `group-<n>/`, and merges summaries, agentic reports, and baselines in `scripts/formal_mutation.py`. A collision inside one file still fails closed.

## Migration Plan

This is additive. After the three preceding changes land:
1. Table the new tests (D8), then run `make mutation-formal-dry-run`.
2. Run `make mutation-formal`; the pre-flight must pass.
3. Rerun each escaped mutant twice.
4. Triage each mutant (D6, D7), then run `make mutation-formal-baseline-update`.
5. Commit the baseline, the ledger, and the rerun evidence.

Rollback: remove the target set, the exec script, `formal_mutation.py`, the Make targets, and the tools files. The D8 extensions can stay.

## Open Questions

- Would a future go-mutesting that allows several workers with `--exec` make sharding unnecessary? Tracked with the deferred sharding follow-up.

## Review edits not applied as written

- Review item 5 offered a per-model `-short` opt-in or sharding. I did not add `-short`, because the orchestrator decided that golden replays run without it. Sharding is deferred, and the rule is to raise the job budget and open the follow-up (Risks).
- Review item 10 offered two options; I chose the evidence file (D5) over a hand-typed `confirmed_reruns`, and dropped that field from the ledger.
- Review item 11 asked that calibration text list the IDs of closed gaps. Once a closed gap leaves the baseline, the ledger has nothing to check it against, so I added a `[[closed]]` ledger table to hold those IDs (D7).
