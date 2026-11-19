## Context

See proposal.md (Why). Current behaviour, verified on `main` at 768fa8c1:

- **Script reads are contained only lexically.**
  - `Implementation.ResolveScriptWithFSAndModule` (`pkg/invowkfile/implementation.go:445`) calls `validateScriptPathContainment` (`implementation.go:457`), which is a `filepath.Rel` test against the raw module path (`implementation.go:508-519`). It then reads with the injected `readFile(string(scriptPath))` (`implementation.go:466`).
  - `CustomCheckScript.ResolveWithFSAndModule` (`pkg/invowkfile/dependency.go:327`, check at `:336`, read at `:342`) does the same.
- **Production readers.** All are `os.ReadFile`:
  - `ExecutionContext.ResolveSelectedScript` (`internal/runtime/runtime.go:533-539`) builds `NewScriptResolver(nil)`, which defaults to `os.ReadFile` (`internal/runtime/script_resolver.go:24-27`). `ResolveSelected` (`script_resolver.go:46-56`) then calls the resolver. Every runtime goes through it: `native.go:125,147,182`, `sh.go:201,292,425`, `lua.go:151,181`, `container_exec.go:109`, and the commandsvc interpreter diagnostics at `interpreter_diagnostics.go:16`.
  - Custom checks: `deps.resolveCustomCheckScript` (`internal/app/deps/checks.go:179`) uses `ctx.ReadScriptFile` (`context.go:74`), which `commandsvc` wires (`inputs.go:151`) from `commandadapters.NewDependencyScriptFileReader()`. That function returns `os.ReadFile` (`dependency_host.go:45-47`) and is wired at `cmd/invowk/app.go:166`.
  - `invowk validate` interpreter diagnostics (`cmd/invowk/validate_interpreter_diagnostics.go:57`, `:72`, `:108`) pass `os.ReadFile`. They ignore resolve errors (`:73`, `:109`).
- **The interpreter re-opens the script path after the read.**
  - Native passes `SelectedScriptFilePath()` to the interpreter (`internal/runtime/native.go:239-241`, `:453-455`).
  - The container runtime maps it under `/workspace` (`internal/runtime/container_provision.go:316-325`).
  - In both, `ResolveSelectedScript` runs first, so a denial there stops execution.
- **Env files have no containment.**
  - `LoadEnvFile` (`internal/runtime/dotenv.go:17-42`) accepts an absolute `path` as is (`:25-26`), joins a relative one onto `basePath` (`:28-30`) and calls `os.ReadFile` (`:33`).
  - A missing optional file is skipped on `os.IsNotExist` (`:35`).
  - `DefaultEnvBuilder.Build` passes `basePath = ctx.Invowkfile.GetScriptBasePath()` (`internal/runtime/env_builder.go:76`, calls at `:80`, `:87`, `:94`). That is the module root when `ModulePath != ""` (`pkg/invowkfile/invowkfile.go:154-159`).
- **Dead helpers.** `Module.ValidateScriptPath` (`pkg/invowkmod/invowkmod.go:603`) and `checkSymlinkSafety` (`invowkmod.go:682`) have no production caller.
  - Go references: `invowkmod_mutation_test.go:282-495` and `TestModule_ValidateScriptPath` (`pkg/invowkmod/operations_test.go:552-584`, plus the comment at `:522`).
  - The path-matrix coverage registry names `pkg/invowkmod.Module.ValidateScriptPath` (`tests/coverage/pathmatrix_coverage_test.go:44`, pointing at `TestModule_ValidateScriptPath`).
  - `tools/mutation/triage/current-full-scan.md:660`, `:1167` mention `ValidateScriptPath()` in past triage notes; they are history and stay.
  - Records: the `.als` rows (`formal/alloy/ModulePathContainment.als:43-44`), the manifest's `[correspondence] unresolved_elements` entries (`formal/manifest.toml:161-162`) and `[triage] binding_gaps` rows (`:219-220`), a goplint exception (`.goplint/exceptions.toml:2709`) and a doc note (`docs/next/cross-platform-windows-improvements.md:86`).
  - The scripts' seeded sets name them too (`scripts/test_formal.py:655-656` `SEEDED_UNRESOLVED_ELEMENTS`, `scripts/test_formal_mutation.py:982-983` `SEEDED_BINDING_GAPS`). Those are subset checks, so they may keep the entries after the manifest drops them.
  - `Module.ContainsPath` (`invowkmod.go:640`) has no other non-test caller.
- **The model's accepts is lexical.** `accepts` uses `lexContains` for every kind. The fixed configurations `scriptReadContainedPhysical` and `envReadContainedPhysical` (`readContainedKind[_, PhysMode]`, `formal/manifest.toml:1285`, `:1313`) already pass. The findings are `findingF13VendoredSymlink` (`:1304`) and `findingF17EnvVendoredSymlink` (`:1332`).
- **The golden links are absolute.** `fstree.createLink` writes absolute targets (`internal/testutil/fstree/fstree.go:249-263`). The golden holds 22848 instances. In 384 `ScriptRead` and 384 `EnvRead` instances, `dAccepts` accepts a read whose `ref` is a link with an in-module target. That count was computed from the golden file.
- **`os.Root` behaviour, probed with Go 1.27.4 on Linux (this stage):**
  - a relative in-root link is followed;
  - these are rejected with "path escapes from parent", which is not `fs.ErrNotExist`: an absolute link to an in-root file, a relative link out of the root, an absolute link out of the root, a dangling absolute link, and any path below an escaping link;
  - a dangling relative link gives `ErrNotExist` from `Stat` and `ReadFile`, while `Lstat` succeeds;
  - a missing name gives `ErrNotExist` from both;
  - `os.OpenRoot` on a missing directory gives `ErrNotExist`;
  - a relative link to an in-root directory, and one to the root itself (`..`), are followed. `Stat` succeeds, `ReadFile` of the link fails with `EISDIR`, and a file below it is read;
  - a relative link above the root (`../..`) is rejected;
  - `Stat` of a mode-000 file succeeds, and `ReadFile` fails with `EACCES`;
  - the escape error's inner error is a plain `*errors.errorString`, while the OS errors are `syscall.Errno`;
  - `root.ReadFile` of a path that escapes lexically (`../x`) fails with the same "path escapes from parent" error, and `errors.Is(err, fs.ErrNotExist)` is false for it (re-probed with Go 1.27.0 on Linux, revise stage).
- **Link cycles** (Go 1.27.0, Linux, revise stage): for a self-link and for a two-link cycle, `root.Stat` and `root.ReadFile` fail with `ELOOP` ("too many levels of symbolic links", a `syscall.Errno`), and `root.Lstat` succeeds. Only Linux was probed, so D2 and D3 do not key on `ELOOP`: they use "`Stat` fails while `Lstat` succeeds".
- **Unresolvable links in the golden.** The golden has one `Link` atom. Its `ScriptRead` and `EnvRead` vectors whose `ref` is that link include, per kind, 288 with a dangling link (no `tgt`) and 288 with a self-cycle (`tgt` is the link itself). Today's lexical `dAccepts` accepts 96 of each per kind; with `physContains` the model denies all of them (`physTarget` is none). Counted from the golden file in the revise stage.
- **Windows junctions** (probe run 44523520503, job 135557989975):
  - `os.Root` rejects `Open`, `Stat` and `Create` through an escaping junction with "path escapes from parent";
  - `filepath.EvalSymlinks` through the same junction returns `ERROR_PATH_NOT_FOUND`, which `errors.Is(err, fs.ErrNotExist)` reports as true.

## Goals / Non-Goals

**Goals:**
- One physical-containment rule for module script, custom-check and env reads that holds for every production caller and on Linux, macOS and Windows.
- The golden's `dAccepts` agrees with the code on every vector.
- Each of the F13 and F17 commits stands alone: its own model edit, inverted replay and pre-fix mutant.

**Non-Goals:**
- Closing the interpreter's re-open race on `SelectedScriptFilePath()` (see Risks).
- Physical containment for plain (non-module) invowkfiles.
- Changing `invowk validate` so that it reports a physical escape (see Open questions).
- F14, F15, F18 and the F16 admission scan (`inspectModuleEntry`).

## Decisions

### D1. Mechanism: `os.Root` (decision 46 option (a))

Module reads are judged by an `os.Root` opened on the module directory, with the path relative to it.
- `os.Root` is junction-aware on Windows.
- It needs no string comparison of evaluated paths. That removes the macOS `/var` → `/private/var` alias problem and the case-folding problem: `OpenRoot` resolves the root path through the OS, and lookups below it are the filesystem's own.

Rejected alternatives:
- **(c) `EvalSymlinks` on both sides, failing closed.** The Windows probe shows that a path through an escaping junction returns an error that `errors.Is(err, fs.ErrNotExist)` reports as true. Under decision 5, an optional `?` env file through such a junction would then be silently skipped. The error cannot be told apart from a genuinely missing file without an extra `Lstat` walk. The walk would also need two-sided evaluation, because of the macOS aliases, and normalised comparison, because of case.
- **(b) model the link-target kind.** This would add a field to `Link` and a dimension to the golden. The only behaviour it captures, the denial of absolute in-root targets, is covered by D4's absolute-target golden pass, which classifies those vectors explicitly, and by D4's dedicated test.

**How this meets option (a) to the letter.** Decision 46 has the golden replay classify the absolute in-root case explicitly. D4 replays the golden twice: once with relative link targets, where the code equals `dAccepts` on every vector, and once with absolute link targets, where the code equals `dAccepts` except on the explicitly classified "in-module link with an absolute target" vectors, which must be denied. The relative pass keeps the golden's coverage of followed in-module links; the absolute pass keeps every absolute-target vector, the escaping ones included (the original F13 form). The correspondence rows record "code stricter than model", as option (a) asks.

### D2. Script reads: containment check before the injected reader

A new unexported helper in `pkg/invowkfile`, for example `checkModuleFilePhysicalContainment(scriptPath, modulePath)`, runs in both resolvers after the lexical check and before the read. It returns:
- **contained**, when `root.Stat(rel)` succeeds. The resolver then calls the injected reader with the lexical path, as today.
- **absent**, when `os.OpenRoot` reports `ErrNotExist`, or `root.Stat(rel)` and `root.Lstat(rel)` both report `ErrNotExist`. The resolver still calls the injected reader.
  - In production, that reader then fails with the read error users see today.
  - For a virtual or fake filesystem in tests, the module directory does not exist on disk, so the fake reader keeps working.
  - Callers covered by this: the fake-reader unit tests (`implementation_test.go:240`, `implementation_mutation_test.go:114`, `dependency_mutation_test.go:495-512`, `checks_mutation_test.go:386`) and the golden and rapid recording readers (`module_path_containment_golden_test.go:115-126`, `:194-209`; `module_path_containment_rapid_test.go:122`).
- **escaping**, when `Stat` returns a non-OS error (the inner error is not a `syscall.Errno`, as with `os.Root`'s "path escapes from parent"), and when `Stat` fails with any OS error while `Lstat` of the same path succeeds (an unresolvable link: dangling, a broken chain, or a cycle). The result is `ErrScriptPathTraversal`, wrapped with the selected path and the module, and the reader is not called. The message names the case: "resolves outside module" for an escape, "dangling link" when `Stat` reports `ErrNotExist`, and "link does not resolve" for any other `Stat` error, so an in-module dangling link or cycle is not reported as an escape. The rule does not depend on the errno, so it holds on Windows as well (see the cycle probe in Context).
- **denied**, for any other OS error from `OpenRoot`, `Stat` or `Lstat`, such as `EACCES` on a directory (there `Lstat` fails too). The resolver returns that error as a script read error and does not call the reader.

The label of a denial is best effort; the read is denied in every case except contained and absent.

The check lives in the resolvers, not in the readers, so it holds whatever reader a caller injects (decision 46 item 2). No production caller changes. `ResolveScriptWithModule` keeps returning `ErrScriptReaderRequired` for file scripts, because the reader-nil and reader-required paths are unchanged.

Rejected alternatives:
- **Read the content through `root.ReadFile` and ignore the injected reader when the root exists.** This is race-free for the content. But it breaks every fake-reader test that uses a real `t.TempDir()` root, and it breaks the recording readers that define accept and touched in the golden and rapid replays. It also leaves the race open, because the interpreter re-opens the path anyway.
- **Change the seam type to a root-relative reader.** A caller could still inject an uncontained reader, which decision 46 item 2 forbids.

### D3. Env files: contained read in module context

The containment goes inside the bound `LoadEnvFile`, so its correspondence row, reach entry and binding test stay as they are. `LoadEnvFile` gains a `moduleRoot` parameter: `LoadEnvFile(env, path, basePath, moduleRoot string)`.
- **`moduleRoot == ""`** (a plain invowkfile): behaviour is unchanged. This keeps plain invowkfiles' symlinked `.env` working (decision 46 item 5).
- **`moduleRoot != ""`**:
  - an absolute `path` is rejected with a containment error;
  - otherwise `rel` is `filepath.Rel(moduleRoot, filepath.Join(basePath, path))`. When `Rel` fails, or `rel` is `..` or starts with `../` (a lexical escape, for example because `basePath` lies outside `moduleRoot`), the result is a containment error. Today `Build` passes `basePath == moduleRoot` in module context (`GetScriptBasePath()` returns `ModulePath`), so `rel` equals the cleaned `path`; the definition keeps a future caller with a different `basePath` correct, and a unit test covers a `basePath` below `moduleRoot` and one outside it;
  - `os.OpenRoot(moduleRoot)` is opened first. Its outcomes:
    - not-exist (the module root is missing, for example deleted after discovery): the file is treated as absent. An optional `?` file returns nil; a required file returns the read error wrapping `fs.ErrNotExist`, as a missing file does today, not a containment error. This meets decision 46 item 5: the skip happens only on a not-exist from the contained read, and a missing directory holds no link that could escape. Nothing is read, so there is no fallback read as in D2;
    - any other error: a read error, and nothing is read;
  - `root.Stat(rel)` runs next. A non-OS error from it (the inner error is not a `syscall.Errno`, as with "path escapes from parent") is a containment error. When `Stat` fails with any OS error while `Lstat` succeeds, the path is an unresolvable link (dangling, broken chain or cycle) and the result is a containment error too, as in D2; this keeps the golden's dangling and cycle vectors (Context) denied on every OS;
  - only when `Stat` and `Lstat` both report `fs.ErrNotExist` is the file absent: an optional `?` file returns nil, and a required one returns the read error. Any other `Stat` OS error with a failing `Lstat` is a read error;
  - then `root.ReadFile(rel)` reads the file. The content read is race-free. A non-OS error from `ReadFile` (an escape that appeared after `Stat`) is a containment error, never the not-exist path, so it is never skipped as optional. Its OS errors, such as `EISDIR` or `EACCES`, are read errors, as today;
  - every containment error wraps `ErrEnvFileOutsideModule` and names the path; no variable is merged into `env` when any error is returned.

`DefaultEnvBuilder.Build` passes `ctx.Invowkfile.ModulePath` as `moduleRoot` for root-, command- and implementation-level files (`env_builder.go:80`, `:87`, `:94`). `LoadEnvFileFromCwd` (`env_builder.go:114`, the `--ivk-env-file` flag) passes `moduleRoot = ""`: those files are named by the user on the command line, not by the module, so they keep today's behaviour. The test callers of `LoadEnvFile` are updated. Env files have no reader seam, so nothing argues for D2's check-then-delegate form here.

Rejected alternative: a separate `LoadModuleEnvFile`. It would move the bound read to a new symbol, and leave `LoadEnvFile`'s row, reach entry and 11 ledger entries without a module-context replay.

A new sentinel, `ErrEnvFileOutsideModule` in `internal/runtime`, distinguishes the containment error from read errors. Scripts reuse `ErrScriptPathTraversal`, which callers and docs already know.

### D4. The model and the golden

- **Model.** `accepts` gains `physContains[o]` for `o.kind in (ScriptRead + EnvRead)`. Then `make formal-golden` regenerates the golden. The F13 commit makes the edit for `ScriptRead`, and the F17 commit extends it to `EnvRead`, so each commit has its own regeneration.
- **Golden materialisation: two passes.** `fstree.Build` gains an opt-in option (for example `RelativeLinkTargets`). It writes each symlink target relative to the link's parent directory. `TestModulePathContainment_GoldenVectors` replays the golden twice:
  - **Relative pass** (the option on). The code SHALL equal `dAccepts`, with the same touched node, on every `ScriptRead` and `EnvRead` vector, with nothing excluded or reclassified. This is semantics-preserving for the golden: a golden `Link` is never a parent (`parent: lone Dir`), so no link sits below another link, and the 384 + 384 in-module link vectors agree with `os.Root`.
  - **Absolute pass** (today's absolute targets). The replay classifies each vector explicitly, from the instance's own tree: a read whose `ref` reaches a `Link` node whose target lies inside the module, and which `dAccepts` accepts, is in the class "in-module link with an absolute target". On those vectors the code SHALL deny the read and touch nothing. On every other vector the code SHALL equal `dAccepts`, the escaping absolute-target links included. The pass SHALL fail when the class is empty for a kind it applies to, so a classifier that matches nothing cannot pass silently, and the class sizes (384 `ScriptRead` and 384 `EnvRead` on the current golden) go into the calibration text.
  - The class applies to `ScriptRead` from the F13 commit and to `EnvRead` from the F17 commit. In the F13 commit `LoadEnvFile` is still uncontained and the model's `EnvRead` still lexical, so the env replay equals `dAccepts` on every vector in both passes.
  - To bound the cost, the absolute pass replays only the instances that contain at least one link; on the others it would repeat the relative pass. If the golden replay's wall time then exceeds the `[reach]` budget, the implement stage re-measures and records it in the `[reach]` calibration text.
  - What this catches that the relative pass alone does not: a defect that denies relative escapes but follows absolute targets, for example a hand-rolled `Readlink` check that rejects only `..`.
  - **How the env replay ties F17 to the code.** In the F17 commit `replayEnvRead` gates on the real `Load` admission and the real `ValidateEnvFilePath`, as today. On every admitted and validated vector it calls the real `LoadEnvFile` with the module root, and the code's verdict comes only from the error class: `errors.Is(err, ErrEnvFileOutsideModule)` means denied, and nil or any other error means accepted. The replay SHALL NOT compute `admit && validates && physContains`, nor consult `dPhysContains`, to decide the code's verdict. That verdict is then compared with `dAccepts`, and on an accepted vector the touched node is checked as today: a regular-file touch must load its sentinel, and a non-file touch (a directory) must return a non-containment error. A denied vector must merge no variable. The instance tree is consulted only to select the expectation for the absolute pass's class, never to compute the code's verdict.
  - Junctions are always absolute, but they stay out of the golden scope.
  - The VirtualPathHarness replay keeps absolute targets and runs once.
- **Code stricter than the model.** The absolute golden pass classifies the absolute in-module target denial explicitly. A dedicated test, for example `TestModulePathContainment_AbsoluteInModuleLinkDenied`, also pins it outside the golden scope (a link to a file in a subdirectory, and a module path given through an alias) for both reads. The `readOp` rows for `ResolveScriptWithFSAndModule`, `ResolveWithFSAndModule` and the env read say "code stricter than model: an absolute link target is denied even inside the module (golden absolute pass)" and name both tests.
- **Correspondence rows.** The reach gate reads its rows from the `.als` correspondence table; `[reach]` has only a budget and calibration text, and no per-row entries. The manifest lists that can name rows are `[correspondence] unresolved_elements` and `[triage] binding_gaps`. Both only shrink: `scripts/test_formal.py:1047` checks `unresolved_elements` is a subset of `SEEDED_UNRESOLVED_ELEMENTS`, `formal_mutation.py triage --check` fails on an entry the base commit does not list, and a listed pair that resolves or names no row fails as stale. So entries are removed, never added or renamed:
  - Add one new row, `physContains | <script helper> | pkg/invowkfile/<file> | TestModulePathContainment_GoldenVectors`. `physContains` resolves in the `.als` (`ModulePathContainment.als:200`), so no list entry is added for it. The golden already records `dPhysContains`.
  - The `readOp | LoadEnvFile` row keeps its symbol and binding. Its abstraction cell gains "module context: contained read; code stricter than model for absolute link targets". The `ModulePathContainment: readOp` entry stays in `unresolved_elements`, because `readOp` still does not resolve.
  - The `findingF13` and `findingF17` characterisation rows become binding rows for the renamed replays, with the element cell naming the renamed manifest command (`mutantPreFixF13LexicalScriptContainment`, `mutantPreFixF17EnvNoPhysicalContainment`), which resolves as a manifest command name. Their `unresolved_elements` entries (`ModulePathContainment: findingF13`, `ModulePathContainment: findingF17`) are then removed, in the commit that renames each, because they would be stale. Only if an element stays unresolvable does its existing entry stay.
  - The `ValidateScriptPath` and `checkSymlinkSafety` rows are removed, with their `unresolved_elements` entries and their `[triage] binding_gaps` rows.
  - The seeded sets in `scripts/test_formal.py` and `scripts/test_formal_mutation.py` may keep the removed entries (subset checks); removing them there is optional.
- **Calibration of the new bindings.** These defects are seeded, each must fail its binding, and each is then reverted and recorded in the manifest calibration text ("Seeded defects calibrate each rebound function"):
  - in the new script helper, treating an escape error as "absent" (caught by the relative pass and the F13 replay);
  - in the new script helper, a check that follows absolute link targets, for example replacing the `os.Root` `Stat` with `EvalSymlinks` of both sides and a prefix test (caught only by the absolute pass's classified vectors and the dedicated test);
  - in `LoadEnvFile`'s module branch, bypassing the root: after the lexical `rel` check, read `filepath.Join(basePath, path)` with `os.ReadFile` instead of `root.Stat` and `root.ReadFile`. The relative pass's env replay must fail on the escaping-link vectors (it loads the outside sentinel with no error, so the code accepts where `dAccepts` denies), and the calibration text records the failing assertion and the number of vectors it failed on.
  - Skipping only the `Stat` check does NOT count as an env calibration seed: `root.ReadFile` rejects the same escapes with "path escapes from parent", which D3 also classifies as a containment error, so the escaping vectors mask the defect. A second, optional seed that the error-class verdict catches is treating an unresolvable link (`Stat` fails, `Lstat` succeeds) as not-exist; it must fail the env replay on the dangling and cycle vectors.
- **Mutation survivors of the new binding.** The new helper becomes a formal-bindings target, so the fix stage's `make mutation-formal` may report new survivors for it.
  - The fix stage may class them only `equivalent`, or as a deferred `binding-gap` or `model-gap` with a `follow_up`.
  - Anything else goes into `maintainer_asks`, and the stage returns `blocked` (`.agents/runbooks/change-flow.md`).
  - The implement stage should therefore give the helper tests that kill its plausible mutants: each branch of the classification, and the `Lstat` disambiguation.
- **Rapid property.** It drops the `invowk_modules/` exemption (`module_path_containment_rapid_test.go:133-143`). It judges accept by the resolver's result rather than by the reader having been called alone, and it generates relative link targets as well as absolute ones. The F16 junction exemption stays only if F16 remains recorded for the script read (D6).

### D5. Dead code

- Delete `Module.ValidateScriptPath` and `checkSymlinkSafety`, their mutation tests and their goplint exception. This is a `pkg/` API removal, stated in the proposal.
- `Module.ContainsPath` stays: decision 46 does not name it (see Open questions).
- Their ledger entries leave through the removal-only refresh, `MUTATION_FORMAL_SYMBOL=pkg/invowkmod/invowkmod.go:<OLD>`, in the fix stage.

### D6. F16

The F16 leg (`internal/app/moduleops/module_path_containment_formal_test.go:77`) reads `j/secret` through a non-vendored junction.
- With D2, `os.Root` should deny it. This follows the probe's `Open` and `Stat` rejection, but it has not been observed through the resolver. The script-read arm is changed to assert containment.
- **Interim state between the F13 commit and the F16 commit.** The leg's script-read arm asserts containment while F16 is still recorded under outcome (b). For that window the script arm's recorded verdict is "containment pending windows-latest evidence", and the F13 commit says so in the `findingF16JunctionScan` model note and in the F16 calibration text: the model still reports the escape for the script read because `scriptReadScanGuarded` is unchanged until the evidence is in. The other arms keep their recorded verdicts. The delta spec states this exception to "Leg asserts the recorded verdicts".
- The other arms are asserted as recorded: admission, both copies and the hash.
- After the PR's windows-latest run, the outcome of every arm is reported with run and job ids, and F16 is reclassified from that evidence. The requirement "F16 is decided by one of three stated outcomes" is renamed "F16 is decided by one of the stated outcomes" and adds outcome (d) for this case: fixed after an observed escape.
- **Sequencing.** `findingF16JunctionScan` expects a counterexample. If the F13 commit edited `scriptReadScanGuarded`, `make formal` would fail there.
  - The F13 commit only flips the leg's script arm and adds a note to the model's F16 calibration text.
  - The model, manifest, golden (if the `.als` changes), correspondence and README edits that reclassify F16 go in a third, post-evidence commit, made once the Windows run is in. It runs `make formal-golden` when needed, then `python3 scripts/formal.py correspondence` and `make formal`.
  - Depending on the outcome, the edit takes one of two forms:
    - the finding becomes `mutantPreFixF16…`, and `scriptReadScanGuarded` uses `physContains`;
    - or the finding stays, narrowed to the arms that still escape.
- An in-module junction is denied too, because junction targets are absolute. This is expected from `os.Root`'s absolute-target rule but has not been observed on Windows.

### D7. Commits

1. **F13.** Contains:
   - the helper and both resolvers;
   - the `ScriptRead` model edit and golden regeneration;
   - the fstree option and the two-pass golden replay;
   - the `unresolved_elements` and `binding_gaps` removals for `findingF13`, `ValidateScriptPath` and `checkSymlinkSafety`;
   - the inverted and renamed F13 replay;
   - `mutantPreFixF13LexicalScriptContainment`;
   - the dead-code deletion;
   - the F16 script arm;
   - the absolute-link test;
   - the testscript pair;
   - its README row.
2. **F17.** Contains:
   - `LoadEnvFile`'s module branch and the `Build` arguments;
   - the `EnvRead` model edit and golden regeneration;
   - the inverted and renamed F17 replay, the absolute-pass class for `EnvRead`, and the `findingF17` `unresolved_elements` removal;
   - `mutantPreFixF17EnvNoPhysicalContainment`;
   - its README row.
3. **F16 reclassification** (post-evidence, D6).

Each pre-fix mutant must produce a counterexample on its own.

## Risks / Trade-offs

- **[Residual race]** After D2's check, the injected reader opens the lexical path again, and the native interpreter and the container mapping open it once more. A local attacker who can swap a link in the module tree between the check and those opens could still redirect a read. This is the time-of-check race decision 46 item 3 puts out of scope. → The risk is recorded here and in the tracker. A later change could execute from a temporary copy of the checked content, as inline scripts already do.
- **[Absolute in-module links stop working]** → Legitimate vendored modules are real directories created by `invowk module vendor`. The release note says so, and D4's test pins the behaviour.
- **[Windows symlink type with relative targets]** On Windows, `os.Symlink` picks a file or directory link by stat-ing the target relative to the link. A target created after its link gets the wrong type. → The fstree option creates links after their targets, as the absolute form already must. The Windows golden run confirms it.
- **[Fake-root fallback]** The "absent" branch calls the injected reader when the module directory does not exist. A directory created between `OpenRoot` and the read is the same race as above. A module directory that does not exist cannot hold an escaping link.
- **[Mutation ledger]** `ResolveScriptWithFSAndModule` (3 entries), `ResolveWithFSAndModule` (15) and `LoadEnvFile` (11) are bound and rewritten, and the new helper becomes bound. → The fix stage runs `make mutation-formal` and the partial refresh. Formal artefacts are implement-stage only, because the fix stage may write only `equivalent` or deferred-gap classes. A survivor that needs any other class blocks the run (D4).

## Migration Plan

There is no data migration. A user whose module reads a file through a link out of the module, or through a link with an absolute target, replaces the link with a real copy, for example with `invowk module vendor`. Rollback is reverting the two commits. The model, golden and manifest revert with them.

## Maintainer decisions (2026-11-19, decision 46, advisor-reviewed)

1. **Mechanism.** Use contained reads, with `os.Root` preferred. The spec stage picks (a), (b) or (c) and justifies the choice. This design picks (a), per D1 and D4. `dAccepts` must agree with the code on every golden vector. Add a cross-platform test for the macOS `/private` alias and for case sensitivity.
2. **Readers.** Every module-context production path gets the contained read, whatever reader it injects. Fake-reader unit tests keep working (D2).
3. **Race.** The native and container re-open race is out of scope and is recorded as a residual risk. Acceptance: the F13 vendored-link fixture run through `invowk cmd` is denied on native and on virtual-sh, with a native mirror.
4. **Dead code.** Delete `ValidateScriptPath` and `checkSymlinkSafety`, and say in the proposal that this removes a `pkg/` API. Remove their correspondence rows and reach entries. The ledger entries leave through the `FILE:OLD` removal-only refresh.
5. **F17 scope.** Module context only. A plain invowkfile's symlinked `.env` keeps working, with a test. An optional `?` file is skipped only on not-exist from the contained read. A dangling or escaping link fails. An absolute path built in Go is tested.
6. **F16.** Report every arm's outcome after the fix and reclassify F16 from it. Do not presume "admission-only". Keep the model and the code in step.
7. **Formal artefacts** (model, golden, replays, manifest, reach, rapid exemptions, README, correspondence, `make formal`) belong to the implement stage.
8. **Commits.** One commit per finding, each with its inverted replay and pre-fix mutant, and each mutant reproducing alone.
9. **Windows evidence.** The PR's Windows job shows the inverted F13, F16 and F17 replays running and passing, with run and job ids and test names.
10. **Docs and release note.**
    - Update `modules/validating.mdx` and `creating-modules.mdx` with their pt-BR copies, the module-security skill and its check catalog (SC-01), and `formal/README.md`.
    - The release note is a security fix with no `!` marker. It says that linking a sibling module into `invowk_modules/` and reading through it stops working.

## Open Questions

- **Resolved (maintainer, revise notes): `Module.ContainsPath` (`pkg/invowkmod/invowkmod.go:640`)** loses its only non-test caller with D5. This change keeps it, and a tracker note records the follow-up.
- **Resolved (maintainer, revise notes): `invowk validate`** reporting an escaping vendored link is a tracker follow-up, not part of this change (see the item below for today's behaviour).
- **Resolved (maintainer, revise notes): absolute link targets.** D4's double replay (relative pass equal to `dAccepts`; absolute pass classifying in-module absolute links as denied) is accepted as meeting decision 46 option (a).
- **`internal/audit` script-read containment** (`internal/audit/scan_files.go:66-89`, `scriptReadPath`) evaluates both sides with `EvalSymlinks` and compares strings. After this change it still accepts an in-module link with an absolute target, which the runtime denies, and it keeps the macOS-alias and case risks D1 avoids. It is not a security hole: the audit only reads content for analysis. Align it with the `os.Root` rule in a follow-up, or leave it? This change leaves it and raises the question for the maintainer (general-rules Scope).
- **`invowk validate` (resolved above as a follow-up; context kept)** checks `script.file` lexically only (`validation_structure_command.go:181`, `validation_structure_deps.go:205`), so a module with an escaping vendored link still validates and fails only at run time. Should validate also report the physical escape? That would be a follow-up change. This change only makes sure validate's interpreter diagnostics no longer read the outside file.
