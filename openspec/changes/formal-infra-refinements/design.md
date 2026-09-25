## Context

`adopt-formal-verification` shipped three Alloy models, five TLA+ models, three trace suites, and the fail-closed runner `scripts/formal.py`. Four kinds of duplication remain:

1. **Alloy commands live in two places.** Each `.als` ends with labelled commands, for example `anteBounded: run { ... } for 4 but 3 Key, 3 Entry expect 1`. `formal/manifest.toml` repeats each label with its verdict and role (`property`, `antecedent`, `mutant_of`, `witness`). `cross_check_alloy_source` uses `ALLOY_COMMAND_RE` to keep the copies in step. ScopeConstruction repeats the scope `for 4 but 3 Key, 3 Entry` 17 times. TLC does not have this problem: `tlc_config()` renders each configuration from `[model.constants]` and the command's overrides.
2. **Golden vectors are verbose.** Format 1 stores every instance as `{"sig": {name: [atom labels]}, "rel": {name: [[labels]]}}`, so every instance repeats every key and every `Cmd$0`-style label. Measured at 4831f23d:

   | File | Instances | Distinct after parse | Atoms | Decoded JSON | Compressed (committed) |
   |---|---|---|---|---|---|
   | `internal/app/deps/testdata/formal/scope_construction_golden.json.gz` | 116928 | 94920 | 11 | 64,174,365 B | 559,856 B |
   | `pkg/invowkmod/testdata/formal/dependency_closure_golden.json.gz` | 25765 | 24709 | 7 | 8,792,936 B | 111,493 B |
   | `pkg/invowkmod/testdata/formal/lock_identity_golden.json.gz` | 28858 | 18198 | 10 | 11,922,411 B | 147,160 B |
   | **Total** | 171551 | | | **84.9 MB** | **818,509 B** |

   ScopeConstruction is close to the skill's 600 KB budget. Decoding it dominates `TestScopeConstructionGoldenVectors` (1.47 s).
3. **Trace specs repeat their machinery.** `AtomicWriteTrace.tla`, `ServerbaseTrace.tla`, and `WatchTrace.tla` each declare `CONSTANTS TraceSet, TraceIndex`, `VARIABLE i`, `T`, `TraceInit`, `TraceSpec`, and `NotFullyConsumed`, and re-implement one of two consumption rules. AtomicWrite and Watch advance when the projection changes; Serverbase advances at settle points. Only `Proj` and the step constraint differ between specs. On the Go side, each harness hand-rolls change deduplication (`maps.Equal` in the AtomicWrite harness) and hard-codes its module name (`"AtomicWriteTraces"`).
4. **Serverbase frames are verbose.** `formal/tla/Serverbase.tla` has 15 variables and 32 `UNCHANGED` clauses, most listing 12–14 variables. The lists are hard to review. Omitting a variable is not the silent hazard: TLC stops with "successor state is not completely specified", which the runner reports as NO-VERDICT. The silent hazard is an over-broad frame. An `UNCHANGED` list that includes a variable the action also assigns turns the conjunction into a guard, which disables or prunes the action with no error. Grouping variables makes that mistake easier to make, so the refactor needs a check that catches it (Decision 7). The other TLA+ models have much shorter frames (2–17 clauses over fewer variables) and are out of scope, per the orchestrator decision.

Constraints: the runner stays standard-library Python 3.11+. The Go loader stays standard-library Go and ordinary `make test` needs no Java. Golden output must stay byte-deterministic (`mtime=0`). The Go `Instance` API used by five replay tests must not change:
- the methods `Atoms`, `One`, `In`, `Targets`, and `Target`;
- the exported maps `Instance.Sig` and `Instance.Rel`, which tests read directly (for example `inst.Rel["lock"]`, a ternary relation, in `internal/app/deps/scope_golden_test.go:62`). Every sig and relation key is present in every instance, with an empty slice when it has no atoms or tuples, exactly as in format 1. Every refinement must keep verdicts, traces, and mutation rejections identical.

## Goals / Non-Goals

**Goals:**
- One source of truth for every Alloy command: the manifest. Models stay readable.
- Golden files about 10 times smaller, with the same instance sequence, the same fingerprint semantics, and a loader of about 60 lines.
- Trace specs that contain only what is model-specific. Go harnesses that cannot write an inconsistent projection.
- A Serverbase model whose frames cannot silently omit a variable.
- A reusable before/after evidence mechanism that shows each refinement is behaviour-preserving.

**Non-Goals:**
- Changing any model's semantics, scope, property, mutant, or calibration.
- Removing `FixStartOrdering`, `FixTerminalCAS`, or other pre-fix switches. They encode the regression mutants for F1–F7.
- De-duplicating golden instances. The duplicates are measured, and their investigation is recorded as a deferred item (see Decision 4).
- Frame refactors of the other TLA+ models (orchestrator decision: variable groups for Serverbase only), and a `go/packages` correspondence resolver.
- A TOML reader in Go (orchestrator decision: the Python golden-command check is sufficient).
- Making `snapshot` a CI step. It stays a local refactoring tool (orchestrator decision).
- Promoting the formal CI lane. That stays with `adopt-formal-verification` task 8.5.

## Decisions

### 1. Alloy commands are rendered into a staged copy of the model

The manifest becomes the only place commands are declared:

```toml
[[model]]
name = "ScopeConstruction"
tool = "alloy"
file = "formal/alloy/ScopeConstruction.als"
scope = "4 but 3 Key, 3 Entry"          # default for every command

[[model.command]]
name = "allowedTargetsBounded"
body = "discoveryFacts implies boundedProp"
expect = "pass"
property = "boundedProp"
antecedent = "anteBounded"

[[model.command]]
name = "golden"
body = """
discoveryFacts
Caller.allowedSet = { t: Cmd | allowedImpl[t] }
noJunk"""
scope = "3 but 2 Mod, 3 Src, 1 Key, 1 Entry"
expect = "instance"
```

Rendering rules:
- `expect = "instance"` renders as `run`, and everything else as `check`.
- The `expect` bit follows `alloy_expected_bit`: `pass` becomes 0, and `counterexample` or `instance` becomes 1.
- Output is `<name>: <kind> {\n<body>\n} for <scope> expect <bit>`, in manifest order.

The runner writes `artifacts/formal/<Model>/<Model>.als`: the committed source, then a generated banner, then the rendered commands. It runs `alloy exec` on that file. This is the same pattern as `tlc_config()` and `stage_tla_modules()`.

The guards move from the source to rendering:
- **Undeclared commands in the source.** A source that still contains a labelled or unlabelled `run`/`check` fails the run, naming the model and the source line: "commands belong in the manifest". The check scans the source after stripping all three Alloy comment forms (`//`, `--`, and `/* ... */`), with a scanner that also recognises unlabelled commands.
- **A check body must reference its property.** The same `\b<property>\b` test now runs on the manifest `body`.
- **An instance command must be a run.** This holds by construction.

**Unknown manifest keys are rejected.** `load_manifest` currently reads optional fields with `.get()` and ignores anything else. With scopes in TOML, a typo such as `scop = "3 but 2 Mod, ..."` on the golden command would silently fall back to the model default and explode the enumeration. After `make formal-golden`, the typo would be invisible, because the digest is computed from what was loaded. The loader therefore fails, naming the table and key, on:
- any key outside the allowed set for `[tools.*]`, `[[model]]`, `[model.constants]` (TLC only), `[[model.command]]`, `[model.golden]`, and `[[trace]]`;
- an Alloy command without a non-empty `body`, or an Alloy model without a default `scope` when any command omits its own;
- `body` or `scope` on a TLC model;
- `constants`, `spec`, or `temporal` on an Alloy model.

Later changes rely on this rejection (orchestrator decision).

Alternatives considered:
- **A committed generated sibling** (`<Model>.cmd.als` that `open`s the model). Plain `go test` could then hash both files, but a second committed artefact needs its own freshness check, and sigs opened from another module are labelled `ScopeConstruction/Cmd` rather than `this/Cmd` in the XML. That would change how golden sig names are parsed. Rejected.
- **Commands as named predicates in the model**, with the manifest naming them. The logic would stay in Alloy syntax, but the model would gain around 40 trivial predicates, and checks would need `assert` blocks. The duplication would move rather than disappear. Rejected.
- **Keep commands in the source and generate the manifest.** This inverts the TLC convention, and roles such as `mutant_of` and `witness` have no Alloy syntax. Rejected.

### 2. The golden fingerprint covers the rendered golden command, and Python checks it without Java

The fingerprint becomes `format=2;source=<sha256 of .als>;alloy=<jar sha>;command=golden;golden=<sha256 of the rendered golden command>`.

- `alloygolden.Load` keeps checking `source=`, so a model edit without `make formal-golden` still fails plain `go test`.
- Commands now live in the manifest, so a manifest-only edit to the golden scope or body would escape the Go check. A new Java-free test in `scripts/test_formal.py` (`RepositoryManifestTests`) closes that gap. It renders each golden command and compares the digest and `format=` with the committed file header.
  - `make test-scripts` runs that test locally.
  - In CI, the test runs only in the path-filtered formal lane (`.github/workflows/formal-verification.yml:81`). No workflow runs `make test-scripts`. The check is therefore advisory until `promote-formal-ci-gate` makes that lane required.
- Freshness today: `formal.py all` (and so `make formal`) compares only the fingerprint header (`golden_is_fresh`). Only `formal.py golden --check` re-enumerates, and no Make target or CI step runs it. That falls short of the parent spec's "Golden vectors are fresh" scenario.
  - This change makes `formal.py all` call `export_golden(..., check=True)` for every golden model, so it re-enumerates and compares byte for byte.
  - The wall time of the re-enumeration is measured (task 3.9) and handed to `promote-formal-ci-gate`, which owns the lane budgets.

Alternative: have the Go loader parse `formal/manifest.toml`. That would make `BurntSushi/toml` a direct dependency, give Go a second TOML reader, and duplicate the rendering logic. Rejected; the orchestrator confirmed the Python check is sufficient.

### 3. Golden format 2: dictionary-coded, columnar, per-column value dictionary

```json
{"model":"LockIdentity","command":"golden","fingerprint":"format=2;...","count":28858,
 "atoms":["Entry$0","Entry$1","Hash$0",...],
 "sig":{"Entry":{"values":[[],[0],[0,1],...],"index":[0,0,3,...]}, ...},
 "rel":{"hash":{"arity":2,"values":[[],[0,2],...],"index":[...]}, ...}}
```

- `atoms` is the sorted list of every atom label in the file.
- Each sig column stores its distinct sorted atom-index lists once. `index[k]` selects instance k's value.
- Each relation column stores its distinct tuple sets as flat, row-major index lists of the declared `arity`. Tuples are sorted, as in format 1. The arity comes from the field's `<types>` element in Alloy's XML output, not from observed tuples, because a relation that is empty in every instance has no tuples to count.
- Keys are sorted and separators are compact, as today. The payload is gzip level 9 with `mtime=0`, so output stays byte-deterministic.
- `Load` decodes the columns into the existing `[]Instance`, including the present-but-empty map entries. The public API does not change. `count`, the `-short` stride of 16, and instance order are unchanged.
- `scripts/formal.py` gains a committed `decode_golden(path) -> list[dict]`. It decodes format 2, and also format 1 until the migration is verified (task 7.1). `snapshot`, the round-trip check, and the Java-free header test all use it, so the permanent `snapshot` subcommand keeps working after the migration.
- Export fails when a compressed file exceeds `GOLDEN_MAX_BYTES = 600_000`, naming the file and its size. An optional `[model.golden] max_bytes` can lower that limit for a model. This turns the skill's 600 KB guidance into a check that matters for `model-module-path-containment`, which adds two golden files.

Measured by re-encoding the committed format-1 files (scratch script, 4831f23d):

| Encoding | ScopeConstruction | DependencyClosure | LockIdentity | Total compressed | Total decoded |
|---|---|---|---|---|---|
| Format 1 (current) | 559,856 | 111,493 | 147,160 | 818,509 | 84.9 MB |
| Dictionary-coded rows | 378,935 | 87,742 | 104,107 | 570,784 | 22.3 MB |
| Dictionary-coded rows, flat tuples | 366,365 | 83,554 | 99,747 | 549,666 | 17.9 MB |
| Plain-text rows | 357,922 | 80,044 | 94,401 | 532,367 | — |
| Columnar | 67,943 | 39,070 | 34,567 | 141,580 | 17.6 MB |
| **Columnar with value dictionaries (chosen)** | **31,643** | **31,305** | **23,842** | **86,790** | **5.7 MB** |
| Chosen format with instances de-duplicated | 21,470 | 30,617 | 15,631 | 67,718 | 4.5 MB |

Alternatives considered:
- **Row-oriented dictionary coding** is simpler, but only 1.5 times smaller.
- **Bitsets over the tuple universe** would be smaller still, but the loader would need to know each relation's column types. Rejected to keep the loader simple.
- **xz or zstd** have no Go standard-library decoder.

### 4. Duplicate instances are kept for now

After `parse_alloy_xml_instance` drops built-in sigs, 19% of ScopeConstruction instances, 4% of DependencyClosure instances, and 37% of LockIdentity instances are exact duplicates. The likely cause is that Alloy solutions differ only in data the parser drops, such as skolem witnesses of the golden `run`. Replaying a duplicate adds no information. De-duplicating would save only another 22% after compression, and it would change `count`, the `-short` sample, and the instance totals quoted in calibration records and the README. That conflicts with the preservation goal. Format 2 keeps every instance. The runner prints the duplicate count at export, and a task records the root cause. The orchestrator decided not to de-duplicate in this change. The duplicate investigation is recorded as a deferred item in `formal/README.md`, for a later change that would also have to update counts, the `-short` sample, and the calibration text.

### 5. `TraceBase.tla` is instantiated, not extended, and its operators are prefixed

```tla
---------------------------- MODULE TraceBase ----------------------------
EXTENDS Sequences, Naturals
CONSTANTS Accepted, Rejected, TraceSet, TraceIndex
VARIABLE i
TraceRecord == IF TraceSet = "accepted" THEN Accepted[TraceIndex] ELSE Rejected[TraceIndex]
TraceStart(p) == i = 1 /\ p = TraceRecord[1]
\* Advance when the observable projection changes; stutter otherwise.
TraceAdvanceOnChange(p, q) ==
    IF q = p THEN i' = i ELSE i < Len(TraceRecord) /\ q = TraceRecord[i + 1] /\ i' = i + 1
\* Advance exactly at settle points.
TraceAdvanceAt(settled, q) ==
    IF settled THEN i < Len(TraceRecord) /\ q = TraceRecord[i + 1] /\ i' = i + 1 ELSE i' = i
NotFullyConsumed == i < Len(TraceRecord)
===========================================================================
```

A trace spec becomes:

```tla
EXTENDS AtomicWrite, AtomicWriteTraces
CONSTANTS TraceSet, TraceIndex
VARIABLE i
INSTANCE TraceBase
Proj == [tmp |-> tmpVisible, replaced |-> targetInode = "tmp", ok |-> returnedOk, err |-> returnedErr]
TraceInit == Init /\ TraceStart(Proj)
TraceNext == Next /\ pc' /= "crashed" /\ TraceAdvanceOnChange(Proj, Proj')
TraceSpec == TraceInit /\ [][TraceNext]_<<vars, i>>
```

An unnamed `INSTANCE` works as follows:
- It substitutes `Accepted` and `Rejected` implicitly with the definitions from the generated `<Model>Traces` module, and substitutes the constants and `i` with the spec's own.
- It imports every TraceBase definition into the trace spec's namespace. `NotFullyConsumed` therefore lands in the root module's namespace, and the runner's `INVARIANT NotFullyConsumed` and `trace_verdict()` do not change.

Serverbase keeps its one-operation-in-flight constraint and its `Settled` helper, and uses `TraceAdvanceAt(Settled' /\ pc' /= pc, Proj')`. That is textually the same rule it uses today.

**Why the prefix.** Every TraceBase definition lands in the namespace of a spec that also EXTENDS its model. Unprefixed names would clash: `formal/tla/HostCallbackToken.tla:47` already defines `Start(e)`, and `trace-validate-token-and-lock` adds `HostCallbackTokenTrace.tla`, which EXTENDS HostCallbackToken and must instantiate TraceBase. SANY would reject a second `Start`. The reserved names are `TraceRecord`, `TraceStart`, `TraceAdvanceOnChange`, `TraceAdvanceAt`, and `NotFullyConsumed`. The sibling trace changes are written against exactly these names (orchestrator decision).

**Guards.** The runner already stages every `formal/tla/*.tla`, so it copies `TraceBase.tla` automatically. Before running TLC, it fails, naming the file and definition, when:
- a `*Trace.tla` does not `INSTANCE TraceBase`;
- a `*Trace.tla` defines a TraceBase operator name;
- a `*Trace.tla` declares a variable other than `i`;
- any non-trace model in `formal/tla/` defines a reserved TraceBase name.

Trace specs may define helper operators such as `Settled`.

**Alternatives considered:**
- Extend `TraceBase` and pass the trace as an operator argument everywhere. Each spec would still define its trace, `TraceInit`, and `NotFullyConsumed`, so this removes less.
- Have `tlatrace` generate the trace selection and `NotFullyConsumed` into `<Model>Traces`. That hides TLA+ logic in Go string templates. Rejected.
- A named instance (`TB == INSTANCE TraceBase`). This avoids name clashes, but then `INVARIANT NotFullyConsumed` needs a root-level alias in every spec. Prefixing is simpler.

If SANY does not accept the implicit substitution of a constant by a defined operator, the fallback is `INSTANCE TraceBase WITH Accepted <- Accepted, Rejected <- Rejected` (task 4.1).

### 6. Shared `tlatrace` helpers validate projection shape

`internal/testutil/tlatrace` gains:
- `Recorder`, with `Observe(Record)` that appends only when the record differs from the last one (the AtomicWrite pattern) and `Trace() []Record`.
- `WriteSuite(t, model string, traces Traces)`. It writes `<model>Traces.tla`, and `<model>Traces.json` with the counts and the sorted `fields` of the projection. It fails the test when either trace set is empty, a trace is empty, or any record's key set differs from the others.

The runner parses the field names of `Proj == [...]` in `<Model>Trace.tla` and fails when they differ from `fields`. The parser:
- reads the `Proj ==` definition up to the next top-level definition (a line starting in column 0 with `<name> ==` or `<name>(...) ==`);
- requires the definition's body to be a single record literal `[...]`, and otherwise fails closed with a named error;
- collects `name |->` only at bracket depth 1, so nested records, function constructors such as `[e \in Execs |-> ...]`, tuples, and multi-line `IF/THEN/ELSE` values (WatchTrace.tla:18-19) do not contribute fields. Without this check, a misspelled key in a targeted mutation makes the mutation "rejected" vacuously, because its record can never equal `Proj`. This closes the same class of vacuity that mutants and antecedents already close for models. `Write` stays as a thin wrapper until the three harnesses migrate, then it is removed.

### 7. Serverbase frames use named variable groups that partition the variables

The variables are grouped:
- `lifeVars == <<state, firstTerminal>>`
- `ctxVars == <<ctxCreated, ctxCancelled>>`
- `errVars == <<errClosed, errCloseCount, sentAfterClose>>`
- `startedVars == <<startedCloses, startedByWinner>>`
- `callerVars == <<op, seen, cancelRead>>`
- `mu`, `stopWon`, and `pc`, which stay individual.

Each action then writes `UNCHANGED <<lifeVars, ctxVars, errVars, ...>>` minus the groups it changes. It spells out only the individual members of a group it partially changes, for example `UNCHANGED <<errClosed, errCloseCount>>` next to `sentAfterClose'`. `vars == <<lifeVars, ctxVars, errVars, startedVars, callerVars, mu, stopWon, pc>>`: a tuple of groups is a valid stuttering subscript, and it is equal exactly when every member is. Action names do not change, so `dead_actions` and coverage output stay valid.

**Partition check.** A length assertion on `vars` cannot work. `Len(vars)` is 8 for a tuple of groups, and with `\o` concatenation a length check misses a variable that is duplicated in one group and missing from another. Instead, the runner statically checks that the flattened members of the groups named in `vars` are exactly the declared `VARIABLES`, each exactly once, and names any duplicate or missing variable. The runner applies this check to any model that defines `*Vars` groups, which today is only Serverbase. It is tested in `scripts/test_formal.py`.

**Equivalence (one-time acceptance evidence).** A temporary module under `artifacts/formal/serverbase-equiv/` instantiates the pre-refactor snapshot as `Old` and the refactored model as `New`. Under every constant assignment used by a manifest command, it checks both directions:
- `Old!Spec => [][New!Next]_vars` catches the silent hazard from Context item 4: an over-broad `UNCHANGED` group that disables an old step.
- `New!Spec => [][Old!Next]_vars` catches any new step the old model did not allow.

The two models have the same `Init` and the same variables, so both directions together prove the behaviours are equal. A frame that omits a variable does not need this check: TLC already reports it as an unassigned successor (NO-VERDICT). This check is migration evidence, not a durable requirement. It is recorded in tasks 5.3 and 7.1 and in the Migration Plan, and the snapshot is not committed.

### 8. `formal.py snapshot` records the evidence for every refinement

`scripts/formal.py snapshot [--out FILE] [--compare FILE]` writes a canonical JSON record. It fails on any difference from `--compare`. The record contains:
- every command's verdict;
- for TLC commands, the violated property, distinct-state count, and zero-coverage actions;
- every trace suite's accepted and rejected counts, and each trace's verdict;
- for each golden file, the SHA-256 of its decoded instance sequence, in canonical format-1 JSON form, so it is independent of the encoding.

It needs Java. It becomes a permanent local subcommand, not a CI step, because future refactors need the same evidence. Workflow: record `baseline.json` from the change's merge base with main before touching anything, then run `--compare` after each refinement lands. Every entry must be identical. Golden entries are digests of decoded instances, so they are expected to be identical across the format change.

## Risks / Trade-offs

- [Alloy enumeration order changes when commands leave the source.] The golden command's text and the model's declarations are identical, and Kodkod translates one command at a time, so the order is expected to stay identical. → The snapshot compares the ordered instance digest first. If only the order changes, the fallback compares the sorted multiset. Accepting it also requires a re-run of the calibration defects with `-short`, because the stride sample changes. A changed multiset is a hard failure.
- [Manifest bodies are Alloy text in TOML strings, without editor support.] → Bodies are short and mostly call named predicates. The rendered file is kept under `artifacts/formal/<Model>/` for inspection, and Alloy reports errors with its line number.
- [The golden-scope edit check moves from Go to Python.] → The check runs in the formal lane (`formal-verification.yml:81`), which stays advisory until `promote-formal-ci-gate`. `make formal` becomes authoritative because it now re-enumerates golden commands.
- [Re-enumerating golden commands in `formal.py all` adds wall time.] → Measure it (task 3.9) and hand the figure to `promote-formal-ci-gate`, which owns the lane budgets.
- [SANY rejects the implicit `INSTANCE` substitution.] → Use the explicit `WITH` form (Decision 5).
- [A TraceBase name clashes with a model operator.] → All exported names are prefixed, and the runner rejects any model that defines one.
- [A Serverbase frame group includes a variable the action assigns, silently disabling the action.] → `Old!Spec => [][New!Next]_vars` reports it. The partition check catches duplicated or missing group members.
- [The format-2 loader has a decoding bug that replays wrong instances.] → Three checks guard against it:
  - A one-time Python round trip compares the format-1 and format-2 decodings of each committed file.
  - A temporary Go check decodes the old and new files with `alloygolden` and compares the `[]Instance` values with `reflect.DeepEqual`, including present-but-empty map entries.
  - A Go unit test decodes a small fixture with a known instance list.

## Migration Plan

This change lands first and alone, before the other formal changes (orchestrator decision).

1. Record `baseline.json` with `snapshot` on the change's merge base with main, after adding only the subcommand.
2. Land the refinements in order, running `snapshot --compare baseline.json` after each: golden format, then Alloy command generation, then trace machinery, then the Serverbase frames.
3. The change is accepted only when:
   - every refinement passes `snapshot --compare` with every entry identical;
   - the one-time bidirectional Serverbase refinement check passes under every manifest constant assignment;
   - the temporary Go `reflect.DeepEqual` check of old against new golden decoding passes.

   These conditions are migration evidence. They are recorded in the PR, not in the capability spec.
4. Rollback is a revert. Format-1 files are not read after migration: `alloygolden.Load` rejects `format=1` with a "regenerate" message, and `decode_golden` drops format 1 once task 7.1 passes.

## Open Questions

The orchestrator resolved the earlier questions:
- The Python golden-command check is sufficient, so Go does not read TOML.
- Golden instances are not de-duplicated here. The investigation is recorded as a deferred item.
- `snapshot` stays a local tool.
- Only Serverbase gets variable groups.

Remaining:
1. When `adopt-formal-verification` is archived, should `formal-infrastructure-generation` be folded into `formal-verification-toolchain`? This change already edits that parent's "Fail-closed verdict handling" requirement in place.

## Review dispositions

Every edit in the independent review was applied. None was rejected. For review item 11, the manifest `distinct_states` edit was dropped from task 5.4. State-count equality comes from `snapshot --compare`, and `promote-formal-ci-gate` owns recorded state counts and budgets for passing TLC commands.
