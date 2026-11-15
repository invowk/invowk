## Context

Every formal guard is fail-closed: declared verdicts, mutants, antecedents, golden freshness, the correspondence and completeness checks, and the mutation pre-flight. Two instruments are not.

**1. A binding can be named without reaching its function.**
- `check_correspondence` (`scripts/formal.py:1138-1171`) proves only that a row's symbol is declared in its file (`formal.py:1167-1169`) and that its binding tests exist (`formal.py:1155-1160`).
- `build_plan` (`scripts/formal_mutation.py:232-325`) turns each bound row into a function entry: file, `start`–`end` lines, rows, the union of the rows' killers (`formal_mutation.py:284-290`), and packages.
- Nothing checks that a killer executes the function. In the first `make mutation-formal` run, every mutant of such a row escaped (`tasks/next/formal-binding-gaps.md:19-31`).

A coverage prototype settles the scope. It is not committed; it lives in the session scratchpad as `reach.py`.
- It builds the plan, then runs each of the 25 distinct killers alone with `go test -count=1 -run '^K$' -coverpkg=<all target packages> -coverprofile=…`.
- It counts the covered statements of each (function, killer) pair inside the function's line range.
- First run (2026-11-14, before the rebase): about 21 s sequential, warm module cache. Re-run on the rebased branch (base `c0a000da`): 45 s sequential, under the shared heavy-command lock with sibling agents running. Both runs: 47 of 55 pairs reach.

The 8 unreached pairs on the rebased tree, each the only killer of its row:

| Function | File | Statements | Killer |
|---|---|---|---|
| `validateDestinationPath` | `internal/app/moduleops/packaging.go:521` | 0/4 | `TestModulePathContainment_GoldenVectors` |
| `LoadEnvFile` | `internal/runtime/dotenv.go:17` | 0/14 | same |
| `CustomCheckScript.ResolveWithFSAndModule` | `pkg/invowkfile/dependency.go:327` | 0/15 | same |
| `Invowkfile.GetEffectiveWorkDir` | `pkg/invowkfile/invowkfile.go:171` | 0/20 | same |
| `newVirtualPathResolverForFilesystem` | `internal/runtime/virtual_policy.go:93` | 0/12 | `TestVirtualPathHarness_GoldenVectors` |
| `standardVirtualAnchorsForOS` | `internal/runtime/virtual_policy.go:148` | 0/21 | same |
| `VirtualFilesystemConfig.EffectiveAccess` | `pkg/invowkfile/virtual_filesystem.go:158` | 0/1 | same |
| `Server.ValidateToken` | `internal/sshserver/server_auth.go:49` | 0/10 | `TestHostCallbackToken_LifecycleMatchesModel` |

- The 7th row is listed in `tasks/next/formal-binding-gaps.md:31`.
- The 8th is new. The lifecycle state machine authenticates through `admitConn` (`internal/sshserver/token_formal_test.go:81`). `git grep 'ValidateToken('` outside `_test.go` finds only the declaration (`server_auth.go:49`), plus documentation in `.agents/agents/security-reviewer.md:10` and `specs/003-test-suite-audit/`.

A second probe asked which binding-file tests do reach these functions.
- The only hits are characterisation tests: `TestModulePathContainment_F13VendoredSymlink`, `_F17EnvVendoredSymlink`, and `TestVirtualPathHarness_F15WorkdirWidening`.
- There is one real binding: `TestModulePathContainment_ContainerWorkDirGolden` reaches `GetEffectiveWorkDir`.
- Characterisation tests are never killers (`formal_mutation.py:274`). Naming one would make the row `characterisation-only` and exempt it, so they are not rebinding targets.

**2. Junction legs never run.** On windows-latest (run 36202690168, job 108292507012):
- `TestModulePathContainment_F16Junction` was skipped ("junction creation unavailable here"; `internal/app/moduleops/module_path_containment_formal_test.go:107-109`).
- Symlink-dependent bindings passed, for example F13 and the `ModulePathContainment` golden (34.4 s).

So symlinks work there and junction creation fails.
- `probeJunction` (`internal/testutil/fstree/links_windows.go:35-43`) throws away the error from `createJunction`.
- `Caps` holds only two booleans (`internal/testutil/fstree/fstree.go:84-87`).
- `Cache.Get` turns a missing capability into a counted skip (`fstree.go:129-142`).
- The leg only `t.Logf`s its verdict (`module_path_containment_formal_test.go:120-124`).
- `mpctree` can emit `KindJunction`, but the golden fixes `Link.kind = SymKind` (`formal/alloy/ModulePathContainment.als:305-306`), and the rapid property draws symlinks only (`module_path_containment_rapid_test.go:72`).

Since Go 1.23, a mount point is not `ModeSymlink`. Per the Go 1.23 release notes (the `winsymlink` GODEBUG; `go.mod` is on Go 1.27), `filepath.EvalSymlinks` may no longer evaluate mount points. The rapid property's oracle uses `EvalSymlinks` (`module_path_containment_rapid_test.go:94`), so on junctions it could call an escape "contained". The identity oracle (D6) is correct either way.

**3. Three rows bind functions that do not carry the model's claim, or carry more than the model states.** All three were found while reviewing the rebinding plan.
- **`Auth` / `Server.ValidateToken`** (`formal/tla/HostCallbackToken.tla:28`). The function has no production caller (above). The model's `Auth` action (`HostCallbackToken.tla:75`) is implemented by `Server.admitConn`, which row `Auth/End atomicity` names (`HostCallbackToken.tla:31`). A test that calls `ValidateToken` directly would make the reach gate green without measuring the server.
- **`unpackRejectsEscape` / `validateDestinationPath`** (`ModulePathContainment.als:39`). No predicate, fact, or command of that name exists in the model or the manifest. The model's claim is `unpackInside[destCheck]` (`ModulePathContainment.als:242-247`; manifest command `unpackStaysInside`, `formal/manifest.toml:1141-1156`). Its destination check is `o.refShape = RelInside`. `refPlacement` (`ModulePathContainment.als:153-156`) places every non-`RelInside` reference outside the module, so the check rejects `AbsPath` and `ParentEscape` alike.

   Production runs two checks before the function:
   - `normalizeZIPPath` (`internal/app/moduleops/packaging.go:346-355`) rejects a leading `/`, a bare `..`, and a leading `../` before any destination is built (`packaging.go:360-361`, `:393-396`);
   - only then does `validateDestinationPath(absDestDir, Join(absDestDir, FromSlash(clean)), name)` run (`packaging.go:419-420`, `:506-507`; and `:374-375` for the module root).

   That gives three results:
   - **Alone, with the `Join` call shape:** the function accepts an `AbsPath` name, because `Join` places it under the root (`packaging.go:521-530`). The model rejects that name. The function alone is therefore not the model's destination check.
   - **Through the pipeline:** acceptance equals the model's on POSIX, but `normalizeZIPPath` makes the function's rejection unreachable there, for `..` and `/` alike. A seeded "drop the `..` check" defect therefore cannot fail any replay. That is reach without measurement, the same problem as `ValidateToken`.
   - **On Windows (unverified):** reading the code, a drive-letter name passes `normalizeZIPPath`, and only `validateDestinationPath`'s `IsAbs` test would reject it. This was reasoned, not run, and the model has no drive-letter shape either way.

   Under decision 5 the row is marked unbound (D9).
- **`anchors` / `standardVirtualAnchorsForOS`** (`formal/alloy/VirtualPathHarness.als:31`). The model abstracts every root into three location kinds: `InReadRoot` (an anchor, temp root, script base, or declared path), `InWorkdir`, and `OutsideAll` (`VirtualPathHarness.als:53-59`). Its only claims about construction are:
  - `locIsAllowedRoot`: read roots, plus the workdir when `includeWorkdir` holds (`VirtualPathHarness.als:89-92`);
  - the two access modes (`VirtualPathHarness.als:44-45`).

  It states nothing about which directories are anchors on which OS, or about the XDG and `APPDATA`/`LOCALAPPDATA` overrides. An "independent anchor table" per OS would therefore be written from the Go code or the docs, not from the model. Decision 5 rejects that kind of oracle as circular.
  - `allowedRoots` and `access` do carry modelled claims. Their replay compares acceptance per location kind with `locIsAllowedRoot` under `includeWorkdir = True`, which is the current code (F15).
  - The omitted access mode is defined as restricted by `openspec/specs/virtual-filesystem-access/spec.md:14`, which is a normative source.
  - `newVirtualPathResolverForFilesystem` reaches `standardVirtualAnchorsForOS` for the host GOOS only, through `standardVirtualAnchors` (`internal/runtime/virtual_policy.go:98,140-146`).

  The `anchors` row is therefore unbound (decision 8).

**4. A correspondence row can name a model element that does not exist.** `check_correspondence` never reads the element cell. A prototype applies exactly the D11 rule: it looks for each row's leading identifier in the model source with comments stripped, and in that model's manifest command names and bodies. The script is `f1elem4.py` and its output is `f1-unresolved-elements.txt`, both in the session scratchpad. **38 of 102 rows in 8 of 11 models (34 distinct model/element pairs)** do not resolve:
- `ModulePathContainment`: 14 rows, for example `readOp`, `workdirCwd`, `copySkipsInnerLinks`, `hashSkipsLinks`, `unpackRejectsEscape`, `findingF13`, `probeF16`;
- `ConcurrentModuleEdits`: 8 rows;
- `VirtualPathHarness`: 7 rows;
- `AtomicWrite`: 3 rows;
- `HostCallbackToken` and `Watch`: 2 rows each;
- `LockIdentity` and `Serverbase`: 1 row each.

`check_triage` keys every survivor on (model, element, symbol) (`formal_mutation.py:721-726`). 192 of the 419 ledger survivors key on these pairs, for example `readOp` 29, `workdirCwd` 28, `copySkipsInnerLinks` 25, `anchors` 22. Decision 5 asks for this check. Decision 7 settles its scope: a shrink-only list (D11). Decision 10 records that the 34 pairs are real naming drift, not matcher artifacts.

**5. The Windows replay budget is recorded but not enforced.**
- The `Cost budgets` requirement (`openspec/specs/module-path-containment-model/spec.md:255-265`) sets 30 s per OS.
- Nothing enforces it. No golden or rapid test measures its own time (`grep -n budget` over `internal/testutil/mpctree/`, the golden, rapid, and formal tests finds nothing).
- `ci.yml` runs the Windows legs under `make test` with no per-test budget.
- The formal lane, the only place with soft and hard budgets (`formal/manifest.toml:69-87`, `.github/workflows/formal-verification.yml:43,103-134`), runs on `ubuntu-latest` only (`formal-verification.yml:39`).
- The one Windows sample (34.4 s) is therefore an unenforced overrun of a documented number.

**Constraints**
- No production Go code changes.
- Windows cannot run locally: the maintainer's VM is down, so Windows CI on PR #163 is the judge.
- `make mutation-*` is not run in this change.
- `check_triage` keys survivors by (model, element, symbol, file), so those row cells must not change.

## Goals / Non-Goals

**Goals**
- A fail-closed reach gate: every killer of every bound row executes its function, in CI, within a budget.
- Calibration: on the current tree the gate fails and names exactly the eight pairs above. After this change it passes.
- Platform-only capabilities fail on their own platform instead of skipping, and the probe says why.
- F16 is decided on Windows CI, under one of three outcomes stated in advance.
- Rows are rebound to tests that call the real function and assert the model's current-code verdict, including the open findings F13, F15, and F17. That is five rows (decision 8). `Auth`, `unpackRejectsEscape`, and `anchors` become explicitly unbound, with the reason in their abstraction cells.
- The correspondence check verifies that each row's model element exists, with today's 34 unresolved pairs on a shrink-only list (D11, decision 7).
- `triage --check` reports every unbound row and every orphaned survivor through a shrink-only list (D13, decision 9).

**Non-Goals**
- Triage of the 419 baselined survivors, a baseline refresh, or new rerun evidence. All of that belongs to the follow-up survivor-triage change.
- Re-pointing `Auth` to `Server.admitConn`, and adding a `normalizeZIPPath` row for `unpackInside`. Both go to the survivor-triage follow-up (decision 4).
- Fixing F13, F14, F15, F17, a confirmed F16, or an F18 found here.
- Junctions in the golden scope (D8).
- Adding `Binding reach` to the promotion gate's required steps (`openspec/specs/formal-ci-gate/spec.md:138-147`). Changing that list affects the counted run history, so it is left to the follow-up that promotes the reach budget to `ci`.
- Coverage-based kill estimates, or any change to the mutation executor.

## Maintainer decisions (2026-11-14)

The maintainer approved these on 2026-11-14 ("the recommendations, as amended by the advisor review"). They are binding.

1. **Seven rows, not eight, get a reaching binding; `ValidateToken` is not gated.** `Server.ValidateToken` has no non-test caller, so a direct-call test would make the gate green without measuring the server. The `Auth` row gets an explicit not-gated reason: it binds a function with no production caller, and re-pointing `Auth` to `Server.admitConn` is the survivor-triage follow-up. Applied in D9: binding `-`, with the reason in the abstraction cell. This follows the precedent of `ValidateScriptPath` (`ModulePathContainment.als:43`). The decision's "seven" assumed that the unpack row could be rebound. Under decision 5 it cannot (Context 3). Applying decision 5's oracle rule to the `anchors` row as well leaves five rebound and three unbound. Decision 8 confirms that count and supersedes the "seven".
2. **F16's third outcome, "contained", is accepted:** the hypothesis is refuted (D7 c).
3. **On Windows, symlinks are required only when `GITHUB_ACTIONS=true`** (D4).
4. **Re-pointing `Auth` to `Server.admitConn` is deferred** to the survivor-triage follow-up.
5. **The unpack row binds only if it carries the model's claim.** `unpackRejectsEscape` is not a predicate in the `.als`. The row binds to `unpackInside[destCheck]` only if the Go function makes the same claim, with replay expectations derived from the model, because a Go-side oracle would be circular. Otherwise the row is marked unbound. Applied: it is not the same claim (Context 3), so the row is unbound (D9). The decision also extends the correspondence check to verify that each row's model element exists in the model source (D11). Decision 7 sets its scope.
6. **The Windows golden-replay budget is not set from one sample.** First establish whether the 30 s budget is enforced on Windows at all. It is not (Context 5). Then derive the value from the first three Windows CI runs on PR #163 (D12; task 8.2, open until those runs exist).

**Cross-cutting (stage 1):** the `formal/README.md` notes mention the eight reach rows, how many were rebound and how many unbound, and the `ValidateToken` finding (task 7.1).

**Round 2 (2026-11-14, after stages 2 and 3).**

7. **The model-element check uses a shrink-only list (option A of the former Open question 1).** It is fail-closed. `[correspondence] unresolved_elements` is seeded with the 34 model/element pairs (Context 4). No new row may enter the list. The survivor-triage follow-up renames or re-keys the listed elements and empties the list. Applied in D11.
8. **The `anchors` row is unbound (option A of the former Open question 2),** with an explicit reason in its abstraction cell. Extending the model with per-OS anchors is a follow-up. `allowedRoots` and `access` stay bound, with expectations derived from the model and the spec. Result: five rows rebound, three unbound (`Auth`, `unpackRejectsEscape`, `anchors`). This supersedes the "seven" of decision 1. Applied in D9.

**Round 3 (2026-11-14, after the advisor review of all acceptances).**

9. **`triage --check` reports unbound rows and orphaned survivors through a shrink-only list.** It covers every unbound correspondence row and every baselined survivor whose function is no longer in the formal-bindings plan. A new orphan or unbound row fails. A listed entry that no longer applies fails as stale. The survivor-triage follow-up empties it. Applied in D13.
   - Applied note, reported to the maintainer: 30 of the 54 listed rows are unbound by design (no-symbol, type-symbol, trace-only, characterisation-only), so no follow-up can empty the row part. D13 and the `mutation-testing` delta therefore commit only to emptying `orphaned_survivors`.
   - The decision gives the orphan count as 42, split "Auth 12, unpack 8, anchors 22". The ledger (`tools/mutation/triage/formal-bindings.toml`) has `Auth` 8 and `unpackRejectsEscape` 12; anchors 22 and the total 42 agree. Each of the three functions is named by exactly one row. This change uses the measured split.
10. **The 34 unresolved pairs were verified as real naming drift** (advisor, 2026-11-14). Their identifiers occur only in the correspondence tables and prose, for example `State` against the variable `state`; `Generate`, `Revoke`, and `cleanup` folded into other actions; `AddProg` against scenario strings. They are not matcher artifacts, so option A stands. The match is case-sensitive (D11).

## Decisions

### D1. The reach gate is a `formal_mutation.py` subcommand over the plan
`python3 scripts/formal_mutation.py reach [--out DIR]` (default `artifacts/formal/reach/`), and `make formal-reach`.

1. Build the plan with `build_plan`, which keeps all of its fail-closed checks:
   - correspondence;
   - the completeness guard;
   - collisions;
   - killers declared in zero or several packages (`formal_mutation.py:239-300`).
2. For each distinct killer K, run `go test -count=1 -json -covermode=set -coverpkg=<P_K> -coverprofile=<DIR>/<K>.cover -run '^K$' <K's package>`.
   - `P_K` is the set of import paths of the directories of the target files of the functions K is a killer for.
   - The determinism variables of the mutation profile are set: `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME`.
   - Killers run in parallel through `formal.parallel_jobs()` (`scripts/formal.py:625`).
3. K is valid only if all of these hold:
   - the exit status is 0;
   - `test_outcomes` (`formal_mutation.py:456`) reports K as `pass`;
   - the profile exists and starts with `mode:`.

   Anything else fails and names K and its rows: a failure, a skip, "not run", a build error, or a missing or garbled profile.
4. For each (row, killer, function) triple, select the profile blocks of `<module path>/<file>` that lie inside `[start, end]`.
   - When the file has no blocks at all, the gate fails with "not instrumented".
   - When the range has no blocks, it fails with "no coverage blocks in lines a–b".
   - When no block in the range has a count above 0, it fails with "executed 0 of N statements".
5. It writes `reach.json` and a text summary. `reach.json` holds each triple with its covered/total counts, each failure, and the unbound rows with their plan reason and abstraction cell. It reports a `reach` timing line through `formal.report_timings` (`formal.py:414`) against the `[reach]` budget.

**Granularity: every killer of every row.** The plan merges killers per function (`formal_mutation.py:284-290`), so a row whose binding does not reach its function could hide behind another row's killer.
- The plan therefore records each row's own killers (`functions[].rows[].killers`), which bumps `PLAN_VERSION` (`formal_mutation.py:58`) to 2.
- `load_plan` (`formal_mutation.py:339-343`) already rejects other versions. Plans are regenerated on every run, so nothing needs migrating.
- The prototype shows the strict rule costs nothing: every multi-killer row already passes it.

**Exempt rows.** `no-symbol`, `no-binding`, `type-symbol`, `trace-only`, and `characterisation-only` rows have no killer. They are listed in the report and not gated, so the plan's unbound-row semantics (`formal_mutation.py:251-277`) carry over unchanged. The report also prints each unbound row's abstraction cell, so the reason for the three rows unbound by D9 is visible.

**Why "at least one covered statement".** The gate checks reach; it is not a coverage target. Requiring every statement would gate on branch coverage, which is survivor-triage work. A closure inside the range counts as reach, because it can only run if its enclosing function ran.

*Alternatives considered*
- **Static call-graph reachability (`go/callgraph`):** needs a dependency, and cannot see calls through the interfaces and function values that the golden replays use.
- **`-cover` per package without `-coverpkg`:** blind to cross-package killers, which are 15 of 30 files.
- **Per-function `-run` with `testing.Coverage()`:** more machinery for the same answer.
- **Putting the gate in `formal.py`:** it needs the plan, and `formal_mutation.py` already imports `formal` (`formal_mutation.py:54`), so a reverse import would be circular.
- **A gate exemption list for `ValidateToken`, keeping its binding:** rejected. The binding never reached the function, so an unbound row states the truth. The `ValidateScriptPath` precedent already exists, and the reach gate needs no exemption key. The `[triage]` list of D13 is a separate, decision-9 record that `triage --check` reads; the reach gate does not read it.

### D2. Where it runs: a dedicated CI step, not `make formal` or `make test-scripts`
- **The CI step.** The formal-verification workflow gains a `Binding reach` step after `Replay golden vectors and property tests` (`formal-verification.yml:118-130`). It is gated on the same `affected` output, and it needs Go but not Java.
- **Not `make formal`.** That target needs Java and is the model check. Keeping reach separate lets a binding author run it without Java.
- **Not `make test-scripts`.** That target stays hermetic. It runs the gate's unit tests against fixture profiles and a stub runner, but not the gate itself.
- **Artifacts.** The upload step already uploads all of `artifacts/formal/` on `failure() || cancelled()` (`formal-verification.yml:137-144`). The default `--out` sits under it, so no upload change is needed.
- **Paths.** The gate's inputs are already in `[ci] paths`: the table files, the binding-test files, and `internal/testutil/**` (`formal/manifest.toml:58`). `scripts/formal_mutation.py` and `scripts/test_formal_mutation.py` are not, so they are added, and a change to the gate itself runs the lane.

### D3. Budgets
- **Measured.** The prototype took 21 s and then 45 s locally (Context 1). The soft budget comes from the implementation's own `timing` line on an idle machine, not from the prototype.
- **Soft budget.** `[reach] budget_seconds = ceil(3 × local seconds)` (`local_headroom`, `formal/manifest.toml:77-80`), with `budget_source = "local"`.
- **Hard limit.** CI is assumed to be at most 2× slower than local until it is measured. The step's `timeout-minutes` is `max(2, ceil(hard_headroom 4 × 2 × local seconds / 60))`, which gives 6 at 45 s. If the implementation measures more than 45 s, the formula is applied again and the job timeout is recomputed.
- **Job limit.** The job timeout must exceed the sum of the step timeouts plus 3 (`openspec/specs/formal-ci-gate/spec.md:65-75`). Today the sum is 26 + 12 + 2 + 2 = 42 (`formal-verification.yml:103,108,120,134`) and the job timeout is 46 (`:43`). With reach it becomes 26 + 12 + 2 + 2 + 6 + 3 = 51, so the job timeout goes to 52.
- **Validation.** `validate_manifest` (`formal.py:277`) accepts `[reach]` with the keys `budget_seconds`, `budget_source`, and `calibration`, and rejects any other key, using `check_keys` (`formal.py:176`).
- **Promotion.** After three `workflow_dispatch` runs on `main`, a follow-up sets `budget_source = "ci"`, as the lane did for the other steps.

### D4. Capability policy per GOOS, with probe errors
`internal/testutil/fstree` gains:
- **`Capability`** (`CapSymlink`, `CapJunction`) and a policy that says on which GOOS each one is required:
  - `CapJunction`: `windows`;
  - `CapSymlink`: `linux` and `darwin`, and `windows` when `GITHUB_ACTIONS=true` (decision 3; a local Windows host without the symlink privilege may skip).
- **Probe errors.** `Caps` keeps each probe's error (`SymlinkErr`, `JunctionErr`), and `probeJunction` and `probeSymlink` (`links_windows.go:35`, `fstree.go:269`) return the error. Off Windows, `probeJunction` returns `errNoJunctions` (`fstree.go:34-36`; `links_other.go`).
- **`Require(t, cap)`.** It returns when the capability is available. When the capability is required on `runtime.GOOS`, it calls `t.Fatalf("<cap> is required on <goos>: <probe error>")`. Otherwise it calls `t.Skipf` with the GOOS and the error.
- **`Cache.Get`** applies the same rule (`fstree.go:129-142`). A missing capability that is required fails the test; one that is not required is counted, as today.
- **`MustSymlink` and `MustJunction(rt Fataler, …)`** for rapid draws. They replace the per-draw `rt.Skipf("symlink: %v")`.
- **Testable policy.** The policy reads GOOS and the environment through an injectable function, so the unit test can cover every GOOS on one host.

**Static guard.** `formal.py correspondence` fails on any `.Skip(`, `.Skipf(`, or `.SkipNow(` call in a binding-test file. Binding-test files are the `BINDING_TEST_SUFFIXES` files (`formal.py:1064`): `*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go`, and `*_trace_test.go`. Binding tests can then skip only through the policy.
- Today there are eight sites, all capability skips:
  - `module_path_containment_formal_test.go:23,108,118`;
  - `module_path_containment_rapid_test.go:27,72`;
  - `virtual_path_harness_golden_test.go:87`;
  - `virtual_path_harness_formal_test.go:25`;
  - `virtual_path_harness_rapid_test.go:49`.
- Trace harnesses skip without `INVOWK_FORMAL_TRACE_DIR` through `tlatrace`, which is not a binding-test file, so they are unaffected.

**Superseded text.** This narrows two statements, which now apply only off the capability's required platforms:
- the "Platform gating is explicit" scenario of the `module-path-containment-model` requirement "Golden replay tests" (`openspec/specs/module-path-containment-model/spec.md:237-239`), which is MODIFIED in this change;
- the skill's "Junctions are Windows-only: their leg is skipped and counted" (`.agents/skills/formal-verification/SKILL.md:93-96`).

*Alternatives considered*
- **Parsing Windows `go test -json` output in `ci.yml` for skips:** it splits the rule across workflows and only fires in CI.
- **A one-off `t.Fatal` in the F16 leg:** it leaves the golden `Cache` and the rapid draws silently skippable.
- **Requiring symlinks on Windows unconditionally:** rejected by decision 3.

### D5. Junction creation: diagnose, compare, then fall back
The order is fixed. Each step lands only if the previous one did not resolve the failure:
1. **Surface the error.** The probe keeps the error, and `Require` prints it. A Windows-only `internal/testutil/fstree/links_windows_test.go` `TestCreateJunction` requires creation and checks it functionally:
   - `os.Lstat(link)` is not `ModeSymlink`;
   - `os.SameFile(stat(link/f), stat(target/f))`.
2. **Compare with a known-good writer.** Compare `writeMountPointReparse` (`links_windows.go:76-99`) field by field with Go's own Windows test helper in `src/os/os_windows_test.go`: `createMountPoint`, `createDirLink`, and `reparseData` with `addSubstituteName`/`addPrintName`, used by `TestDirectoryJunction` (present in the local Go 1.27 toolchain source). There, every name is stored with its NUL, `ReparseDataLength = offsetof(PathBuffer) + pathBuffeLen()`, and the ioctl input length is `ReparseDataLength + sizeof(header)`.
   - **Hypothesis, unverified.** Our `PathBuffer` holds the substitute name and its NUL. `PrintNameOffset = subBytes+2` with length 0 points at the end of the buffer (`links_windows.go:79-87`). The print name has no NUL inside `ReparseDataLength`, which `FSCTL_SET_REPARSE_POINT` may reject with `ERROR_INVALID_REPARSE_DATA`.
   - **Candidate fix.** Append the print name's NUL, or use the absolute target as the print name as `mklink` does, and set `ReparseDataLength = 8 + len(PathBuffer)`.
3. **Fall back.** If the corrected writer still fails on CI, fall back to `cmd /c mklink /J <link> <target>`. When both fail, both errors are reported.

The cause is not asserted ahead of CI. The PR's Windows run decides, and design.md records which step fixed it.

**Recorded (2026-11-14).** Step 2 fixed creation: on windows-latest (CI run 43660058330, job 132647141440) `TestCreateJunction` reports `created via reparse point`, so the Go-matching writer succeeds and the `mklink /J` fallback is not used. The first PR run (43657442894) already passed `TestCreateJunction` with the same writer. The fallback stays as a reported second attempt.

### D6. The rapid property generates junctions, and escapes are judged by identity
In `TestModulePathContainment_Property`, the link kind is drawn from {symlink, junction} where `CapJunction` is available. On Windows it is required, so the junction branch always runs there.
- **Targets.** A junction targets a directory: the module's own subdirectory for the contained case, or an outside directory that holds a unique sentinel file. The script path becomes `<rel>x/secret`.
- **The escape oracle for every link kind** is file identity: `os.SameFile` of the opened file against the outside sentinel, backed by a content-marker comparison. It is never `EvalSymlinks` plus a prefix test (today's `module_path_containment_rapid_test.go:94`). The symlink branch keeps its current semantics.
- **The F13 carve-out stays path-based.** An escape under `invowk_modules/` is the recorded F13 hole for either link kind, because the hole is the unscanned directory. A non-vendored junction escape is F16, and it fails the property until F16 is recorded (D7, outcome b).

### D7. The F16 leg is decided, with three outcomes stated in advance
`TestModulePathContainment_F16Junction` calls `fstree.Require(t, CapJunction)`. For a module with an inner junction to an outside directory, it asserts each operation that production would run for that module:
- `Load` and `IsModule` admission;
- the script read of `j/secret`;
- `modulecache.copyDir` and `provision.CopyDir`: the copy must not contain the outside sentinel;
- `computeModuleHash`: the hash must not change when only the outside sentinel changes.

It first lands asserting containment, which is the policy. Windows CI on the PR then decides:
- **(a) Creation still fails.** The leg fails through `Require`, and so does the CI job. Investigation continues under D5. The change does not merge while the leg is skipped or failing.
- **(b) Escape in any operation.** The binding fails on unmodified code, so this is a defect. F16 is confirmed under the "Recorded finding contents" requirement (`openspec/specs/module-path-containment-model/spec.md:316-327`):
  - a `finding = "F16"` command in `formal/manifest.toml`, next to a fix configuration that passes. Any new predicate in `ModulePathContainment.als` changes its source hash, so `make formal-golden` regenerates the fingerprint;
  - a Findings row in `formal/README.md`, replacing the "unconfirmed" paragraph (`formal/README.md:201-206`);
  - the leg becomes a characterisation replay that asserts the escape. Its row keeps a `characterisation:` note, reworded to name the confirmed finding;
  - the rapid property admits the non-vendored junction escape as the recorded F16 hole, mirroring F13.

  F16 is an id this change already owns (`spec.md:298-307`), so (b) is not a stop condition. Nothing is fixed here.
- **(c) Everything contained.** The hypothesis is refuted. The "Unreproduced findings" requirement (`spec.md:309-314`) then applies:
  - F16 stays unused;
  - README records F16 among the refuted hypotheses, naming for each operation the modelled fact that keeps it contained (for example `loadScansNonLinkedRoot` if `Load` rejects the module) and the existing manifest mutant that drops that fact (for example `mutantDropLoadScan`);
  - if an operation is contained for a reason no modelled fact states, the stop rule applies: report it, and do not invent a fact;
  - the leg stays and asserts containment. It keeps a `characterisation:` note (a probe that asserts today's behaviour), reworded to say F16 is refuted.

The leg is not made a killer under any outcome. The Linux-only reach gate would see it skip, and a skip is a gate failure. On Windows, junction behaviour is checked by the junction branch of the rapid property. That property binds the `contained` policy row, which has no Go symbol, so it is not a killer either.

Under (b) and (c), the outcome is recorded with the CI run id in `ModulePathContainment`'s calibration text.

### D8. Junctions stay out of the golden
Enabling `JunKind` in `goldenScenario` (`ModulePathContainment.als:287-311`) would regenerate the vectors and change the instance counts and the calibration records. The junction instances would also skip on two of the three OSes. Exhaustive junction instances on Windows add little over the rapid property and the F16 leg, which decide the same question.

The README sentence (`formal/README.md:125`) and the calibration text (`formal/manifest.toml:996-997`) are corrected to name what actually covers junctions: the witness commands, the junction branch of the rapid property on Windows, and the F16 leg.

### D9. Rebinding the rows keeps the row identity
Only binding cells and abstraction notes change. The model, element, symbol, and file cells stay, so `check_triage` (`formal_mutation.py:721-726`) still matches every ledger survivor. Every ledger entry of the unbound rows (12 unpack, 8 `Auth`, and 22 `anchors`) is `binding-gap`, so the abstraction-substring check (`formal_mutation.py:731`) does not apply to them.

Each new assertion compares the real function with the model's current-code verdict. Where that verdict reproduces an open finding (F13, F15, F17), the replay asserts the finding.

| Row | Binding after | How it reaches the function |
|---|---|---|
| `workdirCwd` / `GetEffectiveWorkDir` (`.als:32`) | `TestModulePathContainment_ContainerWorkDirGolden`, replacing `…GoldenVectors` | Already calls it (prototype). |
| `readOp` / `CustomCheckScript.ResolveWithFSAndModule` (`.als:30`) | `TestModulePathContainment_GoldenVectors` (unchanged) | For every `ScriptRead` instance, also replay the custom-check resolver with a recording reader, as `realScriptRead` does (`module_path_containment_golden_test.go:133-148`). Require the same accept and touched node as the model, including F13 instances. |
| `readOp` / `LoadEnvFile` (`.als:31`) | `TestModulePathContainment_GoldenVectors` (unchanged) | See the `LoadEnvFile` replay below. |
| `allowedRoots` / `newVirtualPathResolverForFilesystem`, `access` / `VirtualFilesystemConfig.EffectiveAccess` (`formal/alloy/VirtualPathHarness.als:30,32`) | New `TestVirtualPathHarness_RootConstruction` in `internal/runtime/virtual_path_harness_formal_test.go` | See the construction replay below. |
| `anchors` / `standardVirtualAnchorsForOS` (`VirtualPathHarness.als:31`) | `-` (unbound, decision 8) | See the unbound rows below. |
| `unpackRejectsEscape` / `validateDestinationPath` (`.als:39`) | `-` (unbound) | See the unbound rows below. |
| `Auth` / `Server.ValidateToken` (`HostCallbackToken.tla:28`) | `-` (unbound) | See the unbound rows below. |

**The `LoadEnvFile` replay.**
- It runs on every `EnvRead` instance on an admitted module whose path `ValidateEnvFilePath` accepts. That is the production pipeline: validate at parse time, read at runtime.
- It calls the real `LoadEnvFile(env, path, moduleRoot)` (`dotenv.go:17`). Its accept and touched atom must equal the model's, including F17 instances.
- `LoadEnvFile` has no reader seam, so `mpctree` writes each regular file as the dotenv line `INVOWK_ATOM=content-<atom id>`, keeping today's `content-<id>` substring (`internal/testutil/mpctree/mpctree.go:97`). The touched file is then read back from `env`.
- A non-file or missing target must make `LoadEnvFile` return an error.
- If the universal sentinel breaks another assertion, the fallback is an env-only content override in `mpctree`.

**The construction replay.** Its expectations come only from the model, plus one normative spec for the omitted access mode (Context 3).
- It builds a resolver with `newVirtualPathResolverForFilesystem` on a materialised tree with a workdir, a script base, one declared path, and an outside directory. It does this for access omitted, `restricted`, and `full`, each with and without the declared path.
- It classifies probe paths by the model's location kinds:
  - `InReadRoot`: inside the script base, the declared path, the temp root, or an anchor root the resolver reports;
  - `InWorkdir`: inside the workdir only;
  - `OutsideAll`: outside every root.
- It requires the resolver's acceptance to equal `H.access = Full or locIsAllowedRoot[l, includeWorkdir = True]` (`VirtualPathHarness.als:89-97`). `includeWorkdir = True` is the current code, F15, so a workdir-only path is accepted under `restricted`.
- For `access`, it requires `EffectiveAccess` to return `restricted` when access is omitted (`virtual-filesystem-access/spec.md:14`) or set to `restricted`, and `full` when set to `full`.

**The unbound rows.** Each gets binding `-`, which the plan reports as `no-binding` (`formal_mutation.py:258-261`).
- **`unpackRejectsEscape`:** it is not the model's claim (Context 3; decision 5). The abstraction cell says the model's destination check, `unpackInside[destCheck]` (refShape = RelInside), transcribes `normalizeZIPPath` followed by `validateDestinationPath`. Alone, with the `Join` call shape, `validateDestinationPath` accepts an `AbsPath` name. After `normalizeZIPPath`, its rejection is unreachable on POSIX. Binding `unpackInside` to `normalizeZIPPath` is left to the survivor-triage follow-up. The element cell stays, so the 12 ledger entries keep matching.
- **`Auth`:** "not gated" (decision 1). The cell keeps "TTL expiry is a nondeterministic action". It adds that `ValidateToken` has no production caller (`server_auth.go:49` is the only non-test reference), that the model's `Auth` is implemented by `Server.admitConn` (row `Auth/End atomicity`), and that re-pointing the row is the survivor-triage follow-up. `TestHostCallbackToken_LifecycleMatchesModel` stays tabled by the `Generate` row, so binding-test completeness (`formal.py:1120-1135`) still holds.
- **`anchors`** (decision 8): the model abstracts anchor locations into `InReadRoot` and does not state the per-OS layout or the environment overrides (Context 3). The abstraction cell says so, says that the construction replay checks only that anchor roots are read roots (through `allowedRoots`), and names extending the model with per-OS anchors as the follow-up. Its 22 ledger entries are `binding-gap`, so they keep matching.

**Defect rule.** Suppose a rebound assertion fails on unmodified code, and no open finding explains it:
1. Confirm in the model source that the model makes that claim for that operation kind. A transcription error in the replay is fixed, not recorded.
2. If the claim is real, the implementer stops and reports it. The assertion is never weakened, and product code is never touched.
3. The failure is recorded as F18 under "Recorded finding contents": a manifest `finding` command next to a passing fix configuration, a characterisation test, and a README row. The failing binding is not committed.

**Ledger fallout.**
- 59 baselined survivors on the five rebound functions keep their now-stale reasons ("never calls …"), and 42 on the three unbound rows (12 unpack, 8 `Auth`, 22 `anchors`) describe functions that are no longer planned. Those 42 are the orphaned survivors that D13 lists.
- The baseline carries one plan-inputs digest, and this change edits killer-test files that many functions share. By the documented convention, the rerun evidence of all 419 survivors therefore goes stale.
- `check_triage` still passes once D13's list is seeded. Its rerun check compares the baseline digest with the rerun records (`formal_mutation.py:737-753`), not with the current tree.
- `tasks/next/formal-binding-gaps.md` marks the rows as rebound or unbound and flags their survivors for the follow-up.

### D10. Calibration plan
- **Reach gate, live.** Before any rebinding, `make formal-reach` must fail and list exactly the eight triples in Context, with no others and no tooling errors. The output goes into `[reach] calibration`. After D9 it must pass with every bound triple reached, and must list the rows that D9 unbinds as `no-binding`.
- **Reach gate, seeded.** Temporarily removing the new `LoadEnvFile` replay call must fail exactly that triple again. Revert it afterwards.
- **Reach gate, unit tests.** `scripts/test_formal_mutation.py` uses fixture profiles and a stub runner. It covers:
  - reached;
  - zero covered;
  - file not instrumented;
  - an empty range;
  - a missing profile;
  - a garbled profile;
  - killer fail, skip, and not-run;
  - a non-zero exit with passing JSON;
  - exempt unbound rows (all five reasons);
  - the per-row killer split, where a reaching killer of row A does not satisfy row B;
  - `PLAN_VERSION` 2 loading.
- **Skip guard.** `scripts/test_formal.py` fixtures check that a raw `t.Skip` in a `_golden_test.go` fails and that the same call in a plain `_test.go` passes. A Go unit test of the policy (`fstree`, every GOOS) checks that `Require` fatals when the capability is required and unavailable, and skips otherwise.
- **Element check.** Covered by the `scripts/test_formal.py` fixtures in D11.
- **Triage list.** Covered by the `scripts/test_formal_mutation.py` fixtures in D13. On the final tree, `triage --check` must pass with 54 listed rows and 42 listed orphans, and must fail when one listed entry is removed.
- **Rebinding.** For each new replay, seed one plausible defect in the real function and confirm that the binding fails. Revert each one, and record it in the model's `calibration`:
  - drop the `validateScriptPathContainment` call in `CustomCheckScript.ResolveWithFSAndModule`;
  - make `LoadEnvFile` ignore `basePath`;
  - make `GetEffectiveWorkDir` ignore the implementation-level `workdir`;
  - drop the declared paths from the roots in `newVirtualPathResolverForFilesystem`;
  - make `EffectiveAccess` return `full` for an omitted access mode.
- **Windows.** The junction writer, the policy on Windows, the junction branch of the property, and the F16 leg are calibrated on the PR's Windows CI run. The leg's verdict is the calibration. Seeded Windows defects wait until the Windows VM is available, and the calibration text says so.
- **Golden digests.** Table edits change only fingerprints. `python3 scripts/formal.py snapshot --compare` against a pre-change snapshot must report every golden digest and verdict identical, except entries added by a recorded F16 or F18 outcome.

### D11. Correspondence rows name existing model elements (decisions 5, 7, and 10)
`check_correspondence` gains an element check:
- **Leading identifier.** A row's element cell resolves when its leading identifier, the first `[A-Za-z_][A-Za-z0-9_]*` token, occurs as a whole, case-sensitive word in the model source with comments stripped (`strip_alloy_comments` and `strip_tla_comments`, `formal.py:478,665`) or in a command `body` or `name` of that model in `formal/manifest.toml`.
- **Failure.** A row that does not resolve fails as `<model>: row <element> names a model element that is not in <file> or its manifest commands`.
- **The rows that do not resolve today** (decision 7): 38 rows, 34 distinct pairs (Context 4), verified as naming drift (decision 10). A `[correspondence] unresolved_elements` array in `formal/manifest.toml` lists the 34 pairs as `"<model>: <element>"`, regenerated on the implementation tree. A listed pair is reported, not failed. A listed pair that resolves, or that names no row, fails as stale. `validate_manifest` rejects other keys in `[correspondence]`. The list may only shrink: no new or edited row may enter it. The survivor-triage follow-up renames the elements, re-keys the ledger, and empties it.
- **Case.** The match is case-sensitive, so the `Serverbase` row `State` does not resolve against the variable `state` (decision 10).
- **Tests.** `scripts/test_formal.py` fixtures cover a resolving row, an unresolved unlisted row, a listed row, a stale entry, and an entry with no row.

### D12. The Windows golden-replay budget comes from three CI runs (decision 6)
The 30 s budget is enforced nowhere (Context 5), so the windows-latest number stays a recorded figure, as on Linux and macOS:
- after the first three Windows CI runs of PR #163 that include this change's replays, the slowest `ModulePathContainment` golden replay time across them sets `ceil(soft_headroom 1.5 × max)`;
- that value, the three run ids, and a note that no step or test enforces it go into the `formal/README.md` cost paragraph (`formal/README.md:125`).

This needs CI evidence, so its task stays open until the runs exist. If the derived value is an overrun that the maintainer does not accept, memoising the custom-check and env replays per (tree, path) is the lever, not cutting instances.

### D13. `triage --check` lists unplanned rows and orphaned survivors (decision 9)
- **Where.** `formal/manifest.toml` gains a `[triage]` table with exactly two keys, which `validate_manifest` checks with `check_keys`:
  - `unbound_rows`: inline tables `{ model, element, symbol }`, one per row the plan reports as unbound (any of the five reasons);
  - `orphaned_survivors`: mutant ids.

  The list is in the manifest, not the ledger, because `check_triage` rejects ledger tables other than `survivor` and `closed` (`formal_mutation.py:692-694`), and D9 keeps the ledger unedited.
- **Plan membership.** `triage --check` today loads only the manifest models and calls `model_rows` (`formal_mutation.py:869-873`, `:706`). It gains the plan's row classification: the loop of `build_plan` (`formal_mutation.py:251-290`) is factored into a helper that both use. The helper needs no package resolution, so `triage --check` still runs without `go list` (`formal_mutation.py:302`).
- **Orphan.** A `[[survivor]]` is orphaned when the correspondence row it names, matched on (model, element, symbol) as `check_triage` already matches it (`formal_mutation.py:721-722`), is unbound. It is keyed by row, not by the plan's `functions[].symbol`, because the plan qualifies method names (`Invowkfile.GetEffectiveWorkDir`) and the ledger and the row cell do not (`GetEffectiveWorkDir`). Measured on the current tree: 0 orphans today, and 42 once the three rows are unbound.
- **Checks.** `triage --check` fails on:
  - an unbound row that is not listed, or an orphaned survivor that is not listed (new);
  - a listed row that is bound or names no row, or a listed id that is not in the ledger or whose row is bound (stale).

  It prints the listed entries as a report.
- **Seed.** The list is seeded after tasks 6.4, 6.5b, and 6.6 from the plan on the implementation tree. Expected: the 51 rows unbound today (no-binding 21, characterisation-only 20, no-symbol 5, type-symbol 3, trace-only 2), plus `Auth`, `unpackRejectsEscape`, and `anchors`, so 54 rows; and the 42 orphaned survivors (`Auth` 8, `unpackRejectsEscape` 12, `anchors` 22).
- **Shrink-only.** Neither part may grow after seeding. The survivor-triage follow-up empties `orphaned_survivors`. Of the rows, 30 are unbound by design (no-symbol, type-symbol, trace-only, characterisation-only), so the row part is not expected to reach zero.
- **The reach gate is unaffected.** It still lists unbound rows and does not gate them (D1). Only `triage --check` reads `[triage]`.
- **Tests.** `scripts/test_formal_mutation.py` fixtures cover: a listed unbound row; an unlisted unbound row; a listed row that is now bound; a listed row that names no row; a listed orphan; an unlisted orphan; a listed id whose row is bound; a listed id not in the ledger; and an unknown `[triage]` key.

## Risks / Trade-offs

- **The ledger and baseline describe stale reasons after rebinding.** Out of scope by decision. `triage --check` stays green, the follow-up reruns and retriages, and the gaps document flags the affected survivors.
- **The universal dotenv sentinel changes file content that other assertions read.** Keep the `content-<id>` substring inside the sentinel, run all three golden packages, and fall back to an env-only override (D9).
- **The construction replay could become a Go-side oracle.** Its expected acceptance comes from `locIsAllowedRoot` and the access modes, and the omitted access mode comes from the virtual-filesystem-access spec. It never comes from calling other helpers of `virtual_policy.go`. The per-OS anchor layout has no model source, so the `anchors` row is unbound (decision 8).
- **A pre-existing calibration claim has no Go detector.** `module-path-containment-model` "Seeded copy, discovery, hash, unpack, and harness calibration defects" (`openspec/specs/module-path-containment-model/spec.md:284-296`) lists "`validateDestinationPath` omitted" among seeded real-code defects. No test calls the function today, and the calibration text credits only the Alloy mutant `mutantUnpackNoCheck` (`formal/manifest.toml:986-997`). Unbinding the row makes this visible but does not change it. Reconciling that requirement (an Alloy-only calibration, or a `normalizeZIPPath` binding) belongs to the survivor-triage follow-up and is reported to the maintainer.
- **Windows replay time.** The `ModulePathContainment` golden already takes 34.4 s on windows-latest, and the custom-check and env replays add to it. D12 derives the budget from three runs and does not cut instances.
- **`GITHUB_ACTIONS` in the symlink policy depends on the environment.** It is limited to symlinks on Windows, where local privilege legitimately varies. Junctions need no privilege and are required unconditionally.
- **The F16 outcome is unknown until CI runs.** All three outcomes are specified in advance (D7). The change cannot be archived until one is recorded.
- **Coverage builds slow the lane.** The cost is bounded by a soft budget and a hard step timeout, and the killers run in parallel.
- **A rapid killer reaches a function only on some seeds.** `RAPID_SEED` is pinned in the gate, so the result is deterministic. A seed-dependent reach shows up as a gate failure when the seed changes.
- **A new finding row would grow the `[triage]` list.** An F18 recorded under D9 adds a characterisation test. If it needs a new characterisation-only row, that row is unbound and the shrink-only list rejects it. The implementer then stops and reports it to the maintainer (the stop rule), rather than adding the entry. F16 under D7 (b) or (c) keeps the existing `probeF16` row, so it adds nothing.

## Migration Plan

This change adds gates and has no runtime migration. Nothing is breaking for users: the changes are confined to test utilities, formal scripts, CI, and documentation. Rollback is `git revert`. The CI step can be removed alone without affecting the other gates.

## Open Questions

None. Resolved and moved to "Maintainer decisions (2026-11-14)":
- the `Auth` re-pointing (decisions 1 and 4);
- the `unpackRejectsEscape` model fact (decision 5);
- the Windows symlink policy (decision 3) and the Windows replay budget (decision 6);
- the scope of the model-element check (decision 7, then decision 10);
- the `anchors` row (decision 8);
- reporting unbound rows and orphaned survivors (decision 9).
