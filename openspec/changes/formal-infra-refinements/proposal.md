## Why

The formal-verification infrastructure from `adopt-formal-verification` works, but it repeats itself in four places, and each repetition can drift. Alloy commands are written twice, once in the `.als` source and once in `formal/manifest.toml`, and a regex cross-check keeps the copies in step. TLC configurations are already generated from the manifest. Golden vectors decompress to 85 MB of repeated JSON keys and atom labels: 818 KB compressed, and the largest file (560 KB) is close to the 600 KB budget. Each `<Model>Trace.tla` re-implements the same cursor, trace selection, and `NotFullyConsumed` machinery. `Serverbase.tla` spells out a full `UNCHANGED` frame of up to 14 variables in 32 places. Now that all seven findings are fixed and the models are stable, this is the time to remove the duplication, before more models copy it.

## What Changes

- **Alloy commands generated from the manifest.** Each `[[model.command]]` gains `body` and an optional `scope`; each Alloy model gains a default `scope`. The runner renders every labelled `run`/`check` command, with its `expect` bit derived from the manifest verdict. It appends them to a temporary copy of the model under `artifacts/formal/`, and runs Alloy on that copy. The `.als` files keep only the relational model: signatures, facts, predicates, and correspondence tables. A model source that still declares a command fails the run. The regex cross-check between source and manifest is replaced by that rejection plus render-time guards: an instance command must be a `run`, and a check body must reference its property.
- **The manifest rejects unknown keys.** Once scopes and bodies live in TOML, a typo such as `scop =` would otherwise fall back silently to the default scope. The loader fails on:
  - an unknown key in `[[model]]`, `[[model.command]]`, `[model.golden]`, `[[trace]]`, or `[tools.*]`;
  - an Alloy command without a non-empty `body`;
  - `body` or `scope` on a TLC model;
  - `constants`, `spec`, or `temporal` on an Alloy model.
- **Compact golden-vector format (format 2).** Atoms are dictionary-coded. Each sig and relation becomes one column, and each column stores its distinct values once plus a per-instance index. Instance order, instance count, duplicates, and the SHA-256 model fingerprint are kept. The output stays byte-deterministic. Export fails when a file exceeds the 600 KB compressed budget. A committed Python decoder serves `snapshot` and the round-trip check. `alloygolden.Load` decodes format 2 into the same `Instance` API, so replay tests do not change. Measured size: 818 KB → 87 KB compressed, 85 MB → 5.7 MB decoded.
- **Golden fingerprint covers the generated golden command.** The fingerprint adds a digest of the rendered golden command. A manifest-only change to the golden scope or body is then caught without Java by `scripts/test_formal.py`. That script runs in the formal lane, which stays advisory until `promote-formal-ci-gate`. `formal.py all` (and so `make formal`) also re-enumerates each golden command and compares byte for byte, as the parent spec's "Golden vectors are fresh" scenario requires. Today it compares only the fingerprint header.
- **Shared trace machinery.** A new `formal/tla/TraceBase.tla` holds the selected trace, the cursor `i`, the initial match, the two consumption rules, and `NotFullyConsumed`. Its operators are prefixed (`TraceRecord`, `TraceStart`, `TraceAdvanceOnChange`, `TraceAdvanceAt`), so they cannot clash with model operators such as `HostCallbackToken`'s `Start`. Each `<Model>Trace.tla` keeps only its projection `Proj`, its step constraint, any helpers, and one `INSTANCE TraceBase` line. The sibling trace changes build on these names. `internal/testutil/tlatrace` gains shared helpers: a change-deduplicating recorder, and a suite writer that derives the module name from the model name and fails when records have different projection keys or a trace set is empty.
- **Serverbase frame refactor.** Named variable groups replace the repeated `UNCHANGED` lists in `formal/tla/Serverbase.tla`. A runner check confirms that the groups partition the declared variables. A one-time refinement check in both directions confirms that the old and new `Next` relations are equivalent.
- **Behaviour preservation is checked, not assumed.** Before and after each refinement: every command keeps its verdict, and TLC commands keep their violated property, distinct-state count, and action coverage. Golden instance payloads decode to the same sequence of instances. Every trace suite keeps its accepted and rejected counts and per-trace verdicts. The calibration records stay valid.

No product code, CLI behaviour, schema, or user-facing documentation changes. The format-2 golden files are internal test data. Nothing here is **BREAKING**.

## Capabilities

### New Capabilities
- `formal-infrastructure-generation`: the runner generates Alloy commands and TLC configurations from one manifest, uses a compact deterministic golden-vector format with a fingerprint that covers the generated golden command, provides shared trace-validation machinery (TLA+ `TraceBase` and Go `tlatrace` helpers), and requires refactors of models and infrastructure to preserve behaviour, with evidence.

### Modified Capabilities
- None in `openspec/specs/`. The toolchain requirements this change refines still live in the unarchived `adopt-formal-verification` change (`formal-verification-toolchain`):
  - "Fail-closed verdict handling": Alloy `expect` bits are now rendered from the manifest instead of cross-checked against the source. This change edits that requirement, and replaces its "Manifest and model disagree" scenario, directly in the parent change.
  - "Vacuity and coverage guards".
  - "Model-to-code bindings", including the "Golden vectors are fresh" scenario, which `formal.py all` now meets by re-enumerating.

  All other refinements are added as a separate capability rather than as deltas against an unarchived spec.

## Impact

- **Runner:** `scripts/formal.py`, including the manifest schema, Alloy staging, golden export (format 2), fingerprint, and trace-spec staging of `TraceBase.tla`. Tests go in `scripts/test_formal.py`.
- **Models:** `formal/alloy/{ScopeConstruction,DependencyClosure,LockIdentity}.als` (command blocks removed), `formal/tla/{Serverbase,ServerbaseTrace,AtomicWriteTrace,WatchTrace}.tla`, and the new `formal/tla/TraceBase.tla`.
- **Manifest:** `formal/manifest.toml` (Alloy `body`/`scope` fields, and recorded `distinct_states` for Serverbase commands).
- **Go test helpers:** `internal/testutil/alloygolden/alloygolden.go` (format-2 decoder) and `internal/testutil/tlatrace/tlatrace.go` (recorder and suite writer). The harnesses in `internal/core/serverbase/base_trace_test.go`, `pkg/fspath/atomic_trace_test.go`, and `internal/watch/watcher_trace_test.go` move to the new helpers.
- **Golden data:** the three `testdata/formal/*.json.gz` files are regenerated in format 2.
- **Parent change:** `openspec/changes/adopt-formal-verification/specs/formal-verification-toolchain/spec.md` ("Fail-closed verdict handling").
- **Docs:**
  - `formal/README.md`, including trust item 1;
  - `.agents/skills/formal-verification/SKILL.md`: steps 3 and 6, the TLA+ commands section, and the golden-size pitfall;
  - `.agents/rules/commands.md`: the `snapshot` entry;
  - the `formal.py` usage docstring.
- **Dependencies:** none. The runner stays standard-library Python, and the loader stays standard-library Go (`compress/gzip`, `encoding/json`).
