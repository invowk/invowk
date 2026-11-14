## Context

Every formal guard is fail-closed: declared verdicts, mutants, antecedents, golden freshness, the correspondence and completeness checks, and the mutation pre-flight. Two instruments are not.

**1. A binding can be named without reaching its function.** `formal.py check_correspondence` proves only that a row's symbol is declared in its file and that its binding tests exist. `formal_mutation.py plan` turns each bound row into a function entry (file, `start`–`end` lines, rows, the union of killers, packages). Nothing checks that a killer executes the function. In the first `make mutation-formal` run, every mutant of such a row escaped.

A prototype run on 2026-11-14 settles the scope (it was not committed; the script is in the session scratchpad):
- It builds the plan, then runs each of the 25 distinct killers alone with `go test -count=1 -run '^K$' -coverpkg=<all target packages> -coverprofile=…`.
- It counts the covered statements of each (function, killer) pair inside the function's line range.
- Total wall time was about 21 s (sequential, warm module cache). 47 of 55 pairs reach.

The 8 unreached pairs, each the only killer of its row:

| Function | File | Statements | Killer |
|---|---|---|---|
| `validateDestinationPath` | `internal/app/moduleops/packaging.go` | 0/4 | `TestModulePathContainment_GoldenVectors` |
| `LoadEnvFile` | `internal/runtime/dotenv.go` | 0/14 | same |
| `CustomCheckScript.ResolveWithFSAndModule` | `pkg/invowkfile/dependency.go` | 0/15 | same |
| `Invowkfile.GetEffectiveWorkDir` | `pkg/invowkfile/invowkfile.go` | 0/22 | same |
| `newVirtualPathResolverForFilesystem` | `internal/runtime/virtual_policy.go` | 0/12 | `TestVirtualPathHarness_GoldenVectors` |
| `standardVirtualAnchorsForOS` | `internal/runtime/virtual_policy.go` | 0/23 | same |
| `VirtualFilesystemConfig.EffectiveAccess` | `pkg/invowkfile/virtual_filesystem.go` | 0/1 | same |
| `Server.ValidateToken` | `internal/sshserver/server_auth.go` | 0/15 | `TestHostCallbackToken_LifecycleMatchesModel` |

The 7th row is listed in `tasks/next/formal-binding-gaps.md` but was not among the six in the brief. The 8th is new. The lifecycle state machine authenticates through `admitConn`, and `ValidateToken` has no production caller (`git grep 'ValidateToken('` outside `_test.go` finds only its declaration).

A second probe asked which binding-file tests do reach these functions. The only hits are characterisation tests: `TestModulePathContainment_F13VendoredSymlink`, `_F17EnvVendoredSymlink`, and `TestVirtualPathHarness_F15WorkdirWidening`. There is one real binding, `TestModulePathContainment_ContainerWorkDirGolden`, which reaches `GetEffectiveWorkDir`. Characterisation tests are never killers. Naming them would turn a row `characterisation-only` and exempt it, so they are not a rebinding target.

**2. Junction legs never run.** On windows-latest (run 36202690168, job 108292507012):
- `TestModulePathContainment_F16Junction` was skipped ("junction creation unavailable here").
- Symlink-dependent bindings passed, for example F13 and the `ModulePathContainment` golden (34.4 s).

So symlinks work there and junction creation fails. `probeJunction` (`internal/testutil/fstree/links_windows.go:35`) drops the error from `createJunction`. The leg only `t.Logf`s its verdict. `mpctree` can emit `KindJunction`, but the golden fixes junctions off, and the rapid property draws symlinks only. Since Go 1.23, a mount point is not `ModeSymlink`. Per the Go 1.23 release notes (the `winsymlink` GODEBUG; `go.mod` is on Go 1.27), `filepath.EvalSymlinks` may no longer evaluate mount points, and Windows CI confirms it. An oracle built on `EvalSymlinks` plus a path-prefix check could therefore call a junction escape "contained". The identity oracle (D6) is right either way.

**Constraints**
- No production Go code changes.
- Windows cannot be run locally: the maintainer's VM is down, so Windows CI on the PR is the judge.
- `make mutation-*` is not run in this change.
- `check_triage` keys survivors by (model, element, symbol, file), so those row cells must not change.

## Goals / Non-Goals

**Goals**
- A fail-closed reach gate: every killer of every bound row executes its function, in CI, within a budget.
- Calibration: on the current tree the gate fails and names exactly the eight rows above. After the rebinding it passes.
- Platform-only capabilities fail on their own platform instead of skipping, and the probe says why.
- F16 is decided on Windows CI, under one of three outcomes stated in advance.
- All eight rows are rebound to tests that call the real function and assert the model's current-code verdict, including the open findings F13, F15, and F17.

**Non-Goals**
- Triage of the 419 baselined survivors, a baseline refresh, or new rerun evidence. All of that belongs to the follow-up survivor-triage change.
- Fixing F13, F14, F15, F17, a confirmed F16, or an F18 found here.
- Junctions in the golden scope (D8).
- Coverage-based kill estimates, or any change to the mutation executor.

## Decisions

### D1. The reach gate is a `formal_mutation.py` subcommand over the plan
`python3 scripts/formal_mutation.py reach [--out DIR]` (default `artifacts/formal/reach/`), and `make formal-reach`.

1. Build the plan with `build_plan`, which keeps all of its fail-closed checks: correspondence, the completeness guard, collisions, and killers in zero or several packages.
2. For each distinct killer K, run `go test -count=1 -json -covermode=set -coverpkg=<P_K> -coverprofile=<DIR>/<K>.cover -run '^K$' <K's package>`.
   - `P_K` is the set of import paths of the target files of the functions K is a killer for.
   - The determinism variables of the mutation profile are set: `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and `RAPID_SHRINKTIME`.
   - Killers run in parallel through `formal.parallel_jobs()`.
3. K is valid only if all of these hold:
   - the exit status is 0;
   - `test_outcomes` reports K as `pass`;
   - the profile exists and starts with `mode:`.

   Anything else fails, naming K and its rows: a failure, a skip, "not run", a build error, or a missing or garbled profile.
4. For each (row, killer, function) triple, select the profile blocks of `<module path>/<file>` that lie inside `[start, end]`.
   - The gate fails with "not instrumented" when the file has no blocks at all.
   - It fails with "no coverage blocks in lines a–b" when the range has none.
   - It fails with "executed 0 of N statements" when no block in the range has a count above 0.
5. It writes `reach.json` (each triple with its covered/total counts, each failure, and the unbound rows with their reasons) and a text summary. It reports a `reach` timing line through `formal.report_timings` against the `[reach]` budget.

**Granularity: every killer of every row.** The plan merges killers per function. A row whose binding does not reach its function could otherwise hide behind another row's killer. The plan therefore records each row's own killers (`functions[].rows[].killers`), which bumps `PLAN_VERSION` to 2. Plans are regenerated on every run, so nothing needs migrating. The prototype shows the strict rule costs nothing: every multi-killer row already passes it.

**Exempt rows.** `no-symbol`, `no-binding`, `type-symbol`, `trace-only`, and `characterisation-only` rows have no killer. They are listed in the report and not gated, so the plan's unbound-row semantics carry over unchanged.

**Why "at least one covered statement".** The gate is a reach check, not a coverage target. "Every statement" would gate on branch coverage, which is the survivor-triage work. A closure inside the range counts as reach: it can only run if its enclosing function ran.

*Alternatives considered*
- Static call-graph reachability (`go/callgraph`): needs a dependency and cannot see calls through interfaces and function values that the golden replays use.
- `-cover` per package without `-coverpkg`: blind to cross-package killers, which are 15 of 30 files.
- Per-function `-run` with `testing.Coverage()`: more machinery for the same answer.
- Putting the gate in `formal.py`: it needs the plan, and `formal_mutation.py` already imports `formal`, so a reverse import would be circular.

### D2. Where it runs: a dedicated CI step, not `make formal` or `make test-scripts`
- **The CI step.** The formal-verification workflow gains a `Binding reach` step, gated on the same `affected` output. It runs after the replay step, because it needs Go but not Java.
- **Not `make formal`.** That target needs Java and is the model check. Keeping reach separate lets a binding author run it without Java.
- **Not `make test-scripts`.** That target stays hermetic. It runs the gate's unit tests against fixture profiles and a stub runner, but not the gate itself.
- **Documentation.** The skill's Verification block lists `make formal-reach` next to `make formal`.
- **Paths.** The gate's inputs (table files, binding-test files, `internal/testutil/**`) are already in `[ci] paths`. `scripts/formal_mutation.py` and `scripts/test_formal_mutation.py` are not, and are added, so a change to the gate itself runs the lane.

### D3. Budgets
- **Measured.** The prototype took about 21 s locally, cold coverage builds included.
- **Soft budget.** `[reach] budget_seconds = 63`, `budget_source = "local"` (`ceil(3 × 21)`), re-measured from the implementation's own `timing` line before commit.
- **Hard limit.** The step's `timeout-minutes` assumes CI is 4× slower, as `Deep property tests` did before it was measured: `max(2, ceil(4 × 84 / 60)) = 6`.
- **Job limit.** The job timeout must exceed the sum of step timeouts plus 3, so it goes from 46 to 52 (26 + 12 + 2 + 2 + 6 + 3 = 51).
- **Validation.** `validate_manifest` accepts `[reach]` with the keys `budget_seconds`, `budget_source`, and `calibration`, and rejects any other key.
- **Promotion.** After three `workflow_dispatch` runs on `main`, a follow-up sets `budget_source = "ci"`, as the lane did for the other steps.

### D4. Capability policy per GOOS, with probe errors
`internal/testutil/fstree` gains:
- `Capability` (`CapSymlink`, `CapJunction`) and a policy that says on which GOOS each one is required:
  - `CapJunction`: `windows`;
  - `CapSymlink`: `linux` and `darwin`, and `windows` when `GITHUB_ACTIONS=true` (the windows-latest evidence above; a local Windows host without the symlink privilege may skip).
- `Caps` that keeps each probe's error (`SymlinkErr`, `JunctionErr`); `probeJunction` and `probeSymlink` return the error.
- `Require(t, cap)`: it returns when the capability is available. When the capability is required on `runtime.GOOS`, it calls `t.Fatalf("<cap> is required on <goos>: <probe error>")`. Otherwise it calls `t.Skipf` with the GOOS and the error.
- `Cache.Get` applies the same rule. A missing capability that is required here fails, and one that is not required is counted as today.
- `MustSymlink` and `MustJunction(rt Fataler, …)` for rapid draws. They replace the per-draw `rt.Skipf("symlink: %v")`.

**Static guard.** `formal.py correspondence` fails on any `.Skip(`, `.Skipf(`, or `.SkipNow(` call in a binding-test file (`*_formal_test.go`, `*_golden_test.go`, `*_rapid_test.go`, `*_trace_test.go`). Binding tests can then skip only through the policy. Today there are eight sites, all capability skips. Trace harnesses skip without `INVOWK_FORMAL_TRACE_DIR` through `tlatrace`, which is not a binding-test file, so they are unaffected.

**Superseded text.** This narrows the "skip instances lacking a capability" scenario of `model-module-path-containment` and the skill's "Junctions are Windows-only: their leg is skipped and counted on Linux/macOS". Both now apply only off the capability's required platforms.

*Alternatives considered*
- Parsing Windows `go test -json` output in `ci.yml` for skips: it splits the rule across workflows and only fires in CI.
- A one-off `t.Fatal` in the F16 leg: it leaves the golden `Cache` and the rapid draws silently skippable.

### D5. Junction creation: diagnose, compare, then fall back
The order is fixed, and each step lands only if the previous one did not resolve the failure:
1. **Surface the error.** The probe keeps the error, and `Require` prints it. A Windows-only `internal/testutil/fstree/links_windows_test.go` `TestCreateJunction` requires creation and checks it functionally:
   - `os.Lstat(link)` is not `ModeSymlink`;
   - `os.SameFile(stat(link/f), stat(target/f))`.
2. **Compare with a known-good writer.** Compare `writeMountPointReparse` field by field with Go's own Windows test helper (`src/os/os_windows_test.go`: `createMountPoint`, `createDirLink`, and `reparseData` with `addSubstituteName`/`addPrintName`, used by `TestDirectoryJunction`; confirmed present in the local Go 1.27.0 toolchain source). There, every name is stored with its NUL, `ReparseDataLength = offsetof(PathBuffer) + pathBuffeLen()`, and the ioctl input length is `ReparseDataLength + sizeof(header)`.
   - Hypothesis, unverified: our `PathBuffer` holds the substitute name and its NUL, and `PrintNameOffset = subBytes+2` with length 0 points past the end of the buffer. The print name has no NUL inside `ReparseDataLength`, which `FSCTL_SET_REPARSE_POINT` may reject with `ERROR_INVALID_REPARSE_DATA`.
   - Candidate fix: append the print name's NUL (or use the absolute target as the print name, as `mklink` does), and set `ReparseDataLength = 8 + len(PathBuffer)`.
3. **Fall back.** If the corrected writer still fails on CI, fall back to `cmd /c mklink /J <link> <target>`. Both errors are reported when both fail.

The cause is not asserted ahead of CI. The PR's Windows run decides, and design.md records which step fixed it.

### D6. The rapid property generates junctions, and escapes are judged by identity
In `TestModulePathContainment_Property`, the link kind is drawn from {symlink, junction} where `CapJunction` is available. On Windows it is required, so the junction branch always runs there.
- A junction targets a directory: the module's own subdirectory for the contained case, or an outside directory that holds a unique sentinel file.
- The script path becomes `<rel>x/secret`.
- **The escape oracle for every link kind** is file identity: `os.SameFile` of the opened file against the outside sentinel, backed by a content-marker comparison. It is never `EvalSymlinks` plus a prefix test. The symlink branch keeps its current semantics.
- **The F13 carve-out stays path-based.** An escape under `invowk_modules/` is the recorded F13 hole for either link kind, because the hole is the unscanned directory. A non-vendored junction escape is F16, and it fails the property until F16 is recorded (D7, outcome b).

### D7. The F16 leg is decided, with three outcomes stated in advance
`TestModulePathContainment_F16Junction` calls `fstree.Require(t, CapJunction)` and asserts, for a module with an inner junction to an outside directory, each operation that production would run for that module:
- `Load` and `IsModule` admission;
- the script read of `j/secret`;
- `modulecache.copyDir` and `provision.CopyDir`: does the copy contain the outside sentinel?
- `computeModuleHash`: does the hash change when only the outside sentinel changes?

It first lands asserting containment, the policy. Windows CI on the PR then decides:
- **(a) Creation still fails.** The leg fails through `Require`, and so does the CI job. Investigation continues under D5. The change does not merge while the leg is skipped or failing.
- **(b) Escape in any operation.** The binding fails on unmodified code, so it is a defect, and F16 is confirmed through the established procedure:
  - a `finding = "F16"` command in `formal/manifest.toml`, next to a fix configuration that passes. Any new predicate in `ModulePathContainment.als` changes its source hash, so `make formal-golden` regenerates the fingerprint;
  - a Findings row in `formal/README.md`, replacing the "unconfirmed" paragraph;
  - the leg becomes a characterisation replay that asserts the escape. Its row keeps a `characterisation:` note, reworded to name the confirmed finding;
  - the rapid property admits the non-vendored junction escape as the recorded F16 hole, mirroring F13.

  Nothing is fixed here.
- **(c) Everything contained.** The hypothesis is refuted. README moves F16 to the refuted hypotheses, and the F16 id is retired as refuted. The leg stays and asserts containment. It keeps a `characterisation:` note (a probe that asserts today's behaviour), reworded to say F16 is refuted.
  - It is not made a killer under any outcome. The Linux-only reach gate would see it skip, and a skip is a gate failure.
  - On Windows, junction behaviour is checked by the junction branch of the rapid property. The property binds the `contained` policy row, which has no Go symbol, so it is not a killer either.

Under (b) and (c) the outcome is recorded with the CI run id in `ModulePathContainment`'s calibration text.

### D8. Junctions stay out of the golden
Enabling `JunKind` in `goldenScenario` would regenerate the vectors and change the instance counts and the calibration records. The junction instances would also skip on two of the three OSes. Exhaustive junction instances on Windows add little over the rapid property and the F16 leg, which decide the same question. The README sentence is corrected to describe what actually covers junctions: the witness commands, the junction branch of the rapid property on Windows, and the F16 leg.

### D9. Rebinding the eight rows keeps the row identity
Only binding cells and abstraction notes change. The model, element, symbol, and file cells stay, so `check_triage` still matches every ledger survivor. Each new assertion compares the real function with the model's current-code verdict. Where that verdict reproduces an open finding (F13, F15, F17), the replay asserts the finding.

| Row | Binding after | How it reaches the function |
|---|---|---|
| `workdirCwd` / `GetEffectiveWorkDir` | `TestModulePathContainment_ContainerWorkDirGolden` (replaces `…GoldenVectors`) | Already calls it (prototype) |
| `readOp` / `CustomCheckScript.ResolveWithFSAndModule` | `TestModulePathContainment_GoldenVectors` (unchanged) | For every `ScriptRead` instance, also replay the custom-check resolver with a recording reader. Require the same accept and touched node as the model, including F13 instances |
| `readOp` / `LoadEnvFile` | `TestModulePathContainment_GoldenVectors` (unchanged) | For every `EnvRead` instance on an admitted module whose path `ValidateEnvFilePath` accepts (the production pipeline: validate at parse, read at runtime), call the real `LoadEnvFile(env, path, moduleRoot)`. Its accept and touched atom must equal the model's, including F17 instances. `LoadEnvFile` has no reader seam, so `mpctree` writes each regular file as a dotenv sentinel (`INVOWK_ATOM=<atom id>`) and the touched file is read from `env`. A non-file or missing target must make `LoadEnvFile` return an error. If the universal sentinel breaks another assertion, the alternative is an env-only content override in `mpctree` |
| `unpackRejectsEscape` / `validateDestinationPath` | `TestModulePathContainment_GoldenVectors` (unchanged) | For every `Unpack` instance, call `validateDestinationPath(root, filepath.Join(root, FromSlash(path)), path)` (the `packaging.go` call shape). Compare it with the decision the golden records for `Unpack`, after confirming in the `.als` which predicate defines it: `unpackInside[destCheck]` and `accepts` (validate, `lexContains`, admission). The model's `AbsPath` shape and the real call shape differ: `Join` places an absolute zip name under the root. That difference is an abstraction to state in the row, not a defect. The abstraction cell states that this is a Go-side replay of the destination check (see open questions) |
| `allowedRoots` / `newVirtualPathResolverForFilesystem`, `anchors` / `standardVirtualAnchorsForOS`, `access` / `VirtualFilesystemConfig.EffectiveAccess` | new `TestVirtualPathHarness_RootConstruction` in `internal/runtime/virtual_path_harness_formal_test.go` | `standardVirtualAnchorsForOS` for `linux`, `darwin`, and `windows`, with and without the XDG or `APPDATA`/`LOCALAPPDATA` overrides, against an independent anchor table written from the model's tags and the documented layout. `newVirtualPathResolverForFilesystem` for access unset, `restricted`, and `full`, with and without declared paths. The tagged-root set and the access mode must equal the model's construction with `includeWorkdir = True` (the current code, F15) |
| `Auth` / `Server.ValidateToken` | `TestHostCallbackToken_LifecycleMatchesModel` (unchanged) | After every step, call `ValidateToken` for each generated token and require `ok` to equal the expectation the Auth step already computes: `model[e] == tokenValid && !clock.Now().After(issuedAt[e].Add(TokenTTL))`. The model keeps a token `valid` after the clock passes its TTL and derives expiry at login. Its expiry side effect (`dropExpiredToken`) is model-neutral: `revokeLocked` closes `s.conns[token]` whether or not the token entry exists |

**Defect rule.** If any of these fails on unmodified code, other than through a known open finding, first confirm in the model source that the model makes that claim for that operation kind, because a replay transcription error is fixed, not recorded. Then the implementer stops and reports it. The assertion is never weakened, and product code is never touched. The failure is recorded as F18 through the finding procedure: a manifest `finding` command next to a passing fix configuration, a characterisation test, and a README row. The failing binding is not committed.

**Ledger fallout.** About 100 baselined survivors on these eight functions keep their now-stale reasons ("never calls …"). The baseline carries one plan-inputs digest, and this change edits killer-test files that many functions share. By the documented convention, the rerun evidence of all 419 survivors goes stale. `check_triage` still passes, because it compares the baseline digest with the rerun records, not with the current tree (`scripts/test_formal_mutation.py` has no live-tree digest check). `tasks/next/formal-binding-gaps.md` marks the rows as rebound and flags their survivors for rerun in the follow-up.

### D10. Calibration plan
- **Reach gate, live.** Before any rebinding, `make formal-reach` must fail and list exactly the eight triples in Context, with no others and no tooling errors. The output goes into `[reach] calibration`. After D9 it must pass with every bound triple reached.
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
  - exempt unbound rows;
  - the per-row killer split, where a reaching killer of row A does not satisfy row B;
  - `PLAN_VERSION` 2 loading.
- **Skip guard.** `scripts/test_formal.py` fixtures: a raw `t.Skip` in a `_golden_test.go` fails, and the same call in a plain `_test.go` passes. A Go unit test of the policy (`fstree`, all GOOS) checks that `Require` fatals when the capability is required and unavailable, and skips otherwise.
- **Rebinding.** For each new replay, seed one plausible defect in the real function and confirm the binding fails. Revert each one, and record it in the model's `calibration`. For example:
  - drop the `validateScriptPathContainment` call in the custom-check resolver;
  - make `LoadEnvFile` ignore `basePath`;
  - drop the `/..` check in `validateDestinationPath`;
  - drop the XDG override in `standardVirtualAnchorsForOS`;
  - skip the expiry check in `ValidateToken`.
- **Windows.** The junction writer, the policy on Windows, the junction branch of the property, and the F16 leg are calibrated on the PR's Windows CI run. The leg's verdict is the calibration. Seeded Windows defects wait until the Windows VM is available, and the calibration text says so explicitly.
- **Golden digests.** Table edits change only fingerprints. `python3 scripts/formal.py snapshot --compare` against a pre-change snapshot must report every golden digest and verdict identical.

## Risks / Trade-offs

- **[The ledger and baseline describe stale reasons after rebinding]** → Out of scope by decision. `triage --check` stays green, and the follow-up change reruns and retriages. The gaps document flags the affected survivors.
- **[The universal dotenv sentinel changes file content that other assertions read]** → Keep the `content-<id>` substring inside the sentinel, run all three golden packages, and fall back to an env-only override (D9).
- **[The `Unpack` lexical layer and `validateDestinationPath` disagree on absolute or `..` shapes]** → That is a defect on unmodified code. Stop and allocate F18 (D9). Do not adjust the comparison to make it pass.
- **[Windows replay time]** → The `ModulePathContainment` golden already takes 34.4 s on windows-latest, over the 30 s per-OS replay budget of the containment design §7. The custom-check and env replays add to it. Measure on the PR. If it is still over, record the overrun and either raise the budget with evidence or memoise per (tree, path). Do not cut instances.
- **[`GITHUB_ACTIONS` in the symlink policy is environment-dependent]** → It is limited to symlinks on Windows, where local privilege legitimately varies. Junctions need no privilege and are required unconditionally.
- **[The F16 outcome is unknown until CI runs]** → All three outcomes are pre-specified (D7). The change cannot be archived until one is recorded.
- **[Coverage builds slow the lane]** → The cost is bounded by a soft budget and a hard step timeout, and the killers run in parallel.
- **[A rapid killer reaches a function only on some seeds]** → `RAPID_SEED` is pinned in the gate, so the result is deterministic. A seed-dependent reach shows up as a gate failure when the seed changes.

## Migration Plan

This change adds a gate and has no runtime migration. Rollback is `git revert`. The CI step can be removed alone without affecting the other gates.

## Open Questions

1. **`Server.ValidateToken` has no production caller.** The model's Auth action is implemented by `admitConn`. Should the follow-up re-symbol the `Auth` row to `Server.admitConn`, which means re-keying its 8 ledger survivors, and treat `ValidateToken` as test-only API? This change keeps the row and binds it (D9).
2. **The model element `unpackRejectsEscape` appears only in the `ModulePathContainment` correspondence table.** No predicate or fact of that name exists in the `.als`. Should the follow-up add it as a model fact with a mutant (model-gap rigour), or is a Go-side replay with an abstraction note enough?
3. **The symlink policy on Windows.** Should symlinks be required only under `GITHUB_ACTIONS`, or unconditionally (failing local Windows hosts without the privilege)?
4. **The Windows replay budget.** Whether to raise the 30 s per-OS replay budget, given 34.4 s today.
