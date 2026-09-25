## 1. Baseline evidence

- [ ] 1.1 Add `scripts/formal.py snapshot [--out] [--compare]` and a committed `decode_golden(path) -> list[dict]` that reads format 1. `snapshot` records command verdicts, TLC violated properties, distinct states, zero-coverage actions, trace-suite counts and per-trace verdicts, and the canonical digest of each golden file's decoded instance sequence. Make no other change.
- [ ] 1.2 Add `scripts/test_formal.py` cases. `--compare` passes on identical records, and fails on a changed verdict, distinct-state count, trace verdict, or golden digest, naming the entry with its old and new values.
- [ ] 1.3 Record `baseline.json` from the change's merge base with main, with Java 25 and the pinned jars (`python3 scripts/formal.py fetch`; `make formal` and `make formal-traces` green first). Keep a copy outside `artifacts/` for the whole change.

## 2. Golden format 2

- [ ] 2.1 Implement format-2 export in `export_golden` and bump `GOLDEN_FORMAT` to 2:
  - an atom dictionary;
  - sig and relation columns with value dictionaries and indices;
  - relation arity taken from the XML field `<types>`, not from observed tuples.

  Keep instance order and duplicates. Print each file's instance count, duplicate count, and compressed size. Fail when a file exceeds `GOLDEN_MAX_BYTES = 600_000`, or a smaller `[model.golden] max_bytes`, naming the file and size. Extend `decode_golden` to format 2, keeping format 1 until 7.1.
- [ ] 2.2 Add `scripts/test_formal.py` cases:
  - encoding and then decoding a small instance list returns the same list, with an empty sig, a ternary relation, a relation empty everywhere, and a duplicate instance;
  - export over the size budget fails.
- [ ] 2.3 Rewrite `alloygolden.Load` to decode format 2 into `[]Instance`, with every sig and relation key present in `Instance.Sig`/`Instance.Rel` (empty slices as in format 1). Reject `format=1` with a `make formal-golden` message. Keep the `source=` check and the `-short` stride of 16.
- [ ] 2.4 Add a Go unit test in `internal/testutil/alloygolden` that decodes a small hand-written format-2 fixture into a known instance list. Cover an empty sig, a ternary relation, a relation empty in every instance, and a duplicate.
- [ ] 2.5 Convert the three committed files to format 2 with a scratch converter that uses `decode_golden`, and check the conversion two ways:
  - In Python, each decoded sequence equals the format-1 sequence element by element.
  - A temporary Go test (not committed) decodes the old files with the format-1 loader and the new ones with the format-2 loader, and compares the `[]Instance` values with `reflect.DeepEqual`.
- [ ] 2.6 Record the duplicate-instance investigation as a deferred item in `formal/README.md`: 94,920 of 116,928, 24,709 of 25,765, and 18,198 of 28,858 instances are distinct, and the suspected cause is skolem or other data dropped by `parse_alloy_xml_instance`. Confirm the cause if the XML makes it cheap to do so, but change no instances.
- [ ] 2.7 Regenerate the three golden files with `make formal-golden`. Confirm that a second run is byte-identical, that `snapshot --compare` reports identical golden digests, and that the replay tests pass under `go test` and `go test -short`.

## 3. Alloy commands from the manifest

- [ ] 3.1 Extend the manifest loader:
  - For Alloy, add a model-level `scope` and command-level `body` and `scope`, and render commands with `alloy_expected_bit`.
  - Reject, naming the table and key: unknown keys in `[tools.*]`, `[[model]]`, `[[model.command]]`, `[model.golden]`, and `[[trace]]`; an Alloy command without a non-empty `body`; `body` or `scope` on TLC models; and `constants`, `spec`, or `temporal` on Alloy models.
- [ ] 3.2 Stage `artifacts/formal/<Model>/<Model>.als` (the source, a generated banner, and the rendered commands) for `check_alloy_model` and `export_golden`, and point `alloy_exec` at the staged copy.
- [ ] 3.3 Replace `cross_check_alloy_source`:
  - Reject any `run` or `check` left in the committed source, labelled or not, naming the model and source line, after stripping `//`, `--`, and `/* ... */` comments.
  - Apply the property-reference check to the manifest `body`.
- [ ] 3.4 Extend the golden fingerprint with `golden=<sha256 of the rendered golden command>`. Add a Java-free `RepositoryManifestTests` check that each committed golden header matches the manifest's format and golden-command digest, using `decode_golden` for the header.
- [ ] 3.5 Move every command of `ScopeConstruction.als`, `DependencyClosure.als`, and `LockIdentity.als` into `formal/manifest.toml`:
  - Use model default scopes (`4 but 3 Key, 3 Entry`, `5`, and `4`), with overrides only for the golden commands.
  - Delete the command blocks from the sources.
  - Replace the stale section comments (for example `ScopeConstruction.als:130`, "labels and `expect` annotations are cross-checked with the manifest") with a pointer to the manifest.
  - Keep each command group's explanatory comment next to its manifest entries.
- [ ] 3.6 One-time drift check: compare each rendered command with the command removed from the source (whitespace-normalised label, kind, body, scope, and expect bit). All 38 Alloy commands (18, 13, and 7) must match.
- [ ] 3.7 Add `scripts/test_formal.py` cases:
  - a labelled or unlabelled command left in the source fails, naming the line, and one inside each comment form is ignored;
  - a missing default scope fails;
  - a check body without its property fails;
  - rendering is deterministic;
  - a manifest-only golden-scope edit fails the digest check;
  - an unknown key (`scop`) fails;
  - an Alloy command without a body fails;
  - `body` on a TLC command, and `constants` on an Alloy command, fail.
- [ ] 3.8 Make `formal.py all` call `export_golden(..., check=True)` for every golden model, so `make formal` re-enumerates and compares byte for byte. Keep the fingerprint pre-check for a fast, clear message.
- [ ] 3.9 Measure the re-enumeration wall time of each golden model under `formal.py all`, and record it in the PR description as input to `promote-formal-ci-gate`'s lane budget.
- [ ] 3.10 Regenerate the golden files, because the source digest changed. Confirm that `snapshot --compare` reports identical Alloy verdicts and golden digests. If only the order differs, stop and follow the fallback in design.md Risks.

## 4. Shared trace machinery

- [ ] 4.1 Add `formal/tla/TraceBase.tla`, with an SPDX header and the operators `TraceRecord`, `TraceStart`, `TraceAdvanceOnChange`, `TraceAdvanceAt`, and `NotFullyConsumed`. Confirm that SANY accepts an unnamed `INSTANCE TraceBase` with implicit substitution of `Accepted` and `Rejected`, and fall back to an explicit `WITH` if it does not.
- [ ] 4.2 Rewrite `AtomicWriteTrace.tla` and `WatchTrace.tla` on `TraceAdvanceOnChange`, and `ServerbaseTrace.tla` on `TraceAdvanceAt`. Keep each `Proj`, step constraint, helper (`Settled`), and explanatory comment.
- [ ] 4.3 Add runner guards: a `*Trace.tla` must `INSTANCE TraceBase`, must not define a reserved TraceBase operator, and must not declare variables other than `i`; and no module in `formal/tla/` may define a reserved TraceBase name. Add `scripts/test_formal.py` cases for each rule, including a model defining `TraceStart`, and a trace spec with a helper such as `Settled`, which is accepted.
- [ ] 4.4 Add `tlatrace.Recorder` and `tlatrace.WriteSuite`: the module name comes from the model name, the JSON counts include sorted `fields`, and the test fails on empty sets, empty traces, or mismatched key sets. Add unit tests in `internal/testutil/tlatrace`.
- [ ] 4.5 Migrate `TestServerbase_TraceHarness`, `TestAtomicWrite_TraceHarness`, and `TestWatch_TraceHarness` to `WriteSuite` (and AtomicWrite to `Recorder`), then remove `tlatrace.Write`.
- [ ] 4.6 Add the runner's `Proj` field parser: read from `Proj ==` to the next top-level definition, collect `name |->` only at record depth 1, and fail closed when the body is not a record literal. Compare the fields with the recorded `fields`. Add `scripts/test_formal.py` cases for a multi-line `Proj` with `IF/THEN/ELSE` (like WatchTrace), a `Proj` that nests a record and a function constructor `[e \in Execs |-> ...]`, a non-record `Proj`, and a field mismatch.
- [ ] 4.7 Run `make formal-traces` and `snapshot --compare`. Each suite must keep its accepted and rejected counts, and every trace and targeted mutation its verdict.

## 5. Serverbase frame refactor

- [ ] 5.1 Snapshot the pre-refactor `Serverbase.tla` to `artifacts/formal/serverbase-equiv/ServerbaseOld.tla`, with the module renamed. Do not commit it.
- [ ] 5.2 Introduce `lifeVars`, `ctxVars`, `errVars`, `startedVars`, and `callerVars`, and rewrite every `UNCHANGED` clause with them. Define `vars` from the groups plus `mu`, `stopWon`, and `pc`. Keep the action names, the correspondence table, and the comments.
- [ ] 5.3 Add the runner's static partition check: the members of the groups named in `vars` are exactly the declared `VARIABLES`, each once. Add `scripts/test_formal.py` cases for a variable that is duplicated and for one that is missing.
- [ ] 5.4 One-time acceptance evidence: write the temporary equivalence module. Under each distinct constant assignment used by the Serverbase manifest commands, TLC must confirm both `Old!Spec => [][New!Next]_vars`, which catches over-broad `UNCHANGED` groups, and `New!Spec => [][Old!Next]_vars`. Seed one over-broad group temporarily and confirm the first direction reports it. Record the results in the PR description.
- [ ] 5.5 Confirm with `snapshot --compare` that every Serverbase command has identical verdicts, violated properties, state counts, and zero-coverage actions. Do not record `distinct_states` in the manifest; `promote-formal-ci-gate` owns recorded state counts for passing TLC commands.

## 6. Documentation

- [ ] 6.1 Update `formal/README.md`:
  - "How a model earns trust" item 1: verdicts live in the manifest, and Alloy `expect` bits are rendered from it;
  - the layout: staged Alloy copies and `TraceBase.tla`;
  - golden format 2, its sizes, and the size budget;
  - `formal.py all` now re-enumerating golden commands;
  - the duplicate-instance deferred item;
  - the `snapshot` subcommand;
  - the trace-validation paragraph.
- [ ] 6.2 Update `.agents/skills/formal-verification/SKILL.md`:
  - step 3: commands go in the manifest with `body` and `scope`, and unknown keys are rejected;
  - step 6: format 2 and the golden digest check;
  - the TLA+ trace bullet: `INSTANCE TraceBase`, the reserved `Trace*` names, and `WriteSuite`;
  - the golden-size pitfall, now enforced.
- [ ] 6.3 Update the `scripts/formal.py` module docstring Usage block and the argparse `choices` with `snapshot`, and add `scripts/formal.py snapshot` to `.agents/rules/commands.md`. Then run `make check-agent-docs`.
- [ ] 6.4 Confirm that the parent `adopt-formal-verification` spec edit made by this proposal ("Fail-closed verdict handling": the `expect` bit is rendered from the manifest, and the "Manifest and model disagree" scenario is replaced) still matches the implementation. Then run `openspec validate adopt-formal-verification --strict`.

## 7. Verification

- [ ] 7.1 Final `scripts/formal.py snapshot --compare <baseline>`: every entry must be identical. Then drop format-1 support from `decode_golden`.
- [ ] 7.2 Re-run each model's calibration defects against the regenerated golden files and the trace suites. Every defect listed in the manifest `calibration` records must still be detected.
- [ ] 7.3 Run `/simplify` on changed Go and Python files.
- [ ] 7.4 Run `make tidy`, `make license-check`, `make lint`, `make test-scripts` (which includes `python3 scripts/test_formal.py`), and `make check-file-length`.
- [ ] 7.5 Run `make test` in full, and record the change in `TestScopeConstructionGoldenVectors` wall time (1.47 s before).
- [ ] 7.6 Run `make formal` and `make formal-traces` end to end.
- [ ] 7.7 Run `make check-goplint-consumer-routed` and `make check-baseline`, and triage any new findings with the maintainer.
- [ ] 7.8 Run `make sonar-local`, and record that `make test-cli` and the native-mirror checks do not apply, because no txtar files change.
- [ ] 7.9 Confirm that `git ls-files artifacts/` lists nothing. The baseline, the staged models, and the equivalence module stay untracked.
- [ ] 7.10 Run `/learn`.
