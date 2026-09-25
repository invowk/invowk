## ADDED Requirements

### Requirement: Alloy commands are generated from the manifest
Every Alloy `run` and `check` command SHALL be declared only in `formal/manifest.toml`, as a `[[model.command]]` with a non-empty `body`, an optional `scope` that overrides the model's default `scope`, and its expected verdict. The runner SHALL render each command as `<name>: <run|check> { <body> } for <scope> expect <0|1>`, in manifest order:
- a command expecting `instance` SHALL render as `run`, and every other command as `check`;
- the `expect` bit SHALL be 0 for `pass`, and 1 for `counterexample` and `instance`.

The runner SHALL append the rendered commands to a staged copy of the model under `artifacts/formal/<Model>/`, and SHALL run Alloy on that copy. Committed `.als` files SHALL NOT declare commands. Comments in all three Alloy forms (`//`, `--`, and `/* ... */`) SHALL be ignored when checking this.

#### Scenario: Command left in the model source
- **WHEN** a committed `.als` file contains a `run` or `check` command, labelled or not, outside a comment
- **THEN** the runner SHALL fail before invoking Alloy and name the model and the source line

#### Scenario: Check body does not reference its property
- **WHEN** a `check` command's `body` does not reference the `property` the manifest declares for it
- **THEN** the runner SHALL fail and name the command

#### Scenario: Rendered verdicts match the manifest
- **WHEN** the runner renders a model's commands
- **THEN** each rendered `expect` bit SHALL follow from the manifest verdict, and the Alloy verdict for each command SHALL be compared with that manifest verdict

#### Scenario: Default scope
- **WHEN** a command declares no `scope`
- **THEN** the runner SHALL render it with the model's default `scope`, and SHALL fail if the model declares none

### Requirement: The manifest rejects unknown and misplaced keys
The manifest loader SHALL fail closed, naming the table and the key, on:
- any key outside the allowed set for `[tools.*]`, `[[model]]`, `[[model.command]]`, `[model.golden]`, and `[[trace]]`;
- an Alloy command without a non-empty `body`;
- `body` or `scope` on a command or model whose tool is `tla`;
- `constants`, `spec`, or `temporal` on a command or model whose tool is `alloy`.

#### Scenario: Misspelled scope key
- **WHEN** a `[[model.command]]` contains `scop = "3 but 2 Mod"`
- **THEN** the runner SHALL fail before rendering any command and name `scop` and the command

#### Scenario: Alloy command without a body
- **WHEN** an Alloy `[[model.command]]` has no `body`, or an empty one
- **THEN** the runner SHALL fail and name the command

#### Scenario: Key for the wrong tool
- **WHEN** a TLC command declares `body`, or an Alloy command declares `constants`
- **THEN** the runner SHALL fail and name the key, the command, and the model's tool

### Requirement: Compact deterministic golden vectors
Golden vector files SHALL use format 2:
- a sorted atom dictionary;
- one column per sig and per relation, each holding its distinct values once as atom-index lists, plus a per-instance index;
- relations flattened row-major, with an arity taken from Alloy's declared field types.

Instance order and `count` SHALL be those of the Alloy enumeration, and duplicate instances SHALL be kept. The payload SHALL be canonical JSON (sorted keys, compact separators), compressed with gzip level 9 and `mtime=0`, so that regenerating from an unchanged model yields byte-identical files.

`alloygolden.Load` SHALL:
- decode format 2 into the existing `Instance` type, with every sig and relation key present in `Instance.Sig` and `Instance.Rel` (an empty slice when it has no atoms or tuples);
- reject any other format with a message naming `make formal-golden`;
- use only the Go standard library.

`scripts/formal.py` SHALL provide a committed decoder that returns the same instance sequence.

#### Scenario: Deterministic regeneration
- **WHEN** `make formal-golden` runs twice on an unchanged tree
- **THEN** the golden files SHALL be byte-identical between the runs

#### Scenario: Encode and decode round trip
- **WHEN** a list of instances that includes an empty sig, a ternary relation, a relation empty in every instance, and a duplicate instance is encoded in format 2 and decoded, by the runner's decoder and by `alloygolden.Load`
- **THEN** both decoders SHALL return the original list, in order, with every key present

#### Scenario: Format 1 file after migration
- **WHEN** `alloygolden.Load` reads a file whose fingerprint declares `format=1`
- **THEN** the test SHALL fail with a message to run `make formal-golden`

#### Scenario: Size budget
- **WHEN** a golden export would write a file larger than the budget (600,000 compressed bytes, or a smaller `[model.golden] max_bytes`)
- **THEN** export SHALL fail and name the file and its size

### Requirement: Golden vectors are fresh against the generated golden command
A golden file's fingerprint SHALL record the format, the SHA-256 of the model source, the Alloy jar checksum, the golden command name, and the SHA-256 of the rendered golden command.
- `alloygolden.Load` SHALL keep failing plain `go test` when the model source changed after generation.
- `scripts/test_formal.py` SHALL fail, without Java, when a golden file's recorded format or golden-command digest differs from the manifest's current rendering.
- `scripts/formal.py all` SHALL re-enumerate every golden command and fail when the result differs byte for byte from the committed file.

#### Scenario: Model edited without regeneration
- **WHEN** a model `.als` changes and its golden file is not regenerated
- **THEN** the replay test that loads it SHALL fail under plain `go test`

#### Scenario: Golden scope edited in the manifest only
- **WHEN** the golden command's `body` or `scope` changes in `formal/manifest.toml` without regeneration
- **THEN** `python3 scripts/test_formal.py` SHALL fail and name the model

#### Scenario: Stale vectors under the full run
- **WHEN** `scripts/formal.py all` runs and re-enumerating a golden command produces a payload different from the committed file
- **THEN** it SHALL fail and name the file and `scripts/formal.py golden <Model>`

### Requirement: Shared trace-validation machinery
`formal/tla/TraceBase.tla` SHALL define these reserved operators:
- `TraceRecord`: the trace selected by `TraceSet` and `TraceIndex`;
- `TraceStart`: the initial match of the cursor `i`;
- `TraceAdvanceOnChange`: advance when the projection changes;
- `TraceAdvanceAt`: advance at a settle point;
- `NotFullyConsumed`.

Every `formal/tla/<Model>Trace.tla`:
- SHALL `INSTANCE TraceBase`;
- SHALL NOT define any reserved TraceBase operator;
- SHALL NOT declare variables other than `i`;
- MAY define helper operators, such as `Settled`, besides `Proj`, `TraceInit`, `TraceNext`, and `TraceSpec`.

No other module in `formal/tla/` SHALL define a reserved TraceBase operator name.

#### Scenario: Trace spec re-implements the machinery
- **WHEN** a `*Trace.tla` omits `INSTANCE TraceBase`, defines `NotFullyConsumed` or another reserved TraceBase operator, or declares a variable other than `i`
- **THEN** `scripts/formal.py traces` SHALL fail before running TLC and name the spec and the offending definition

#### Scenario: Model clashes with a reserved name
- **WHEN** a model in `formal/tla/` defines an operator named `TraceStart`, `TraceRecord`, `TraceAdvanceOnChange`, `TraceAdvanceAt`, or `NotFullyConsumed`
- **THEN** the runner SHALL fail and name the model and the operator

#### Scenario: Unchanged trace verdicts
- **WHEN** trace validation runs on specs built on `TraceBase`
- **THEN** every recorded trace SHALL be accepted, and every targeted mutation rejected, as declared by its harness

### Requirement: Trace harnesses cannot write an inconsistent projection
`internal/testutil/tlatrace` SHALL provide a recorder that appends a record only when it differs from the previous one, and a suite writer that:
- derives the module name `<Model>Traces` from the model name;
- records the sorted projection field names alongside the trace counts;
- fails the test when either trace set is empty, a trace is empty, or records in the suite have different key sets.

The runner SHALL take the projection fields from the top-level fields of the `Proj ==` record literal in `<Model>Trace.tla`, and SHALL ignore fields of nested records and function constructors. It SHALL fail closed when `Proj` is not a record literal, and SHALL fail when the parsed fields differ from the recorded ones.

#### Scenario: Misspelled field in a targeted mutation
- **WHEN** a harness writes a rejected trace whose record uses a key that no other record uses
- **THEN** the harness test SHALL fail instead of producing a vacuously rejected trace

#### Scenario: Projection drift between harness and spec
- **WHEN** a trace spec's `Proj` fields differ from the fields the harness recorded
- **THEN** `scripts/formal.py traces` SHALL fail and name both field sets

#### Scenario: Multi-line and nested projection
- **WHEN** `Proj` spans several lines, contains `IF/THEN/ELSE`, or nests a record or a function constructor such as `[e \in Execs |-> ...]`
- **THEN** the parser SHALL return only the top-level field names

#### Scenario: Projection is not a record
- **WHEN** `Proj` is defined as anything other than a record literal
- **THEN** the runner SHALL fail and name the spec

### Requirement: Serverbase frames use named variable groups
`formal/tla/Serverbase.tla` SHALL frame its actions with named variable groups instead of repeating variable lists, and `vars` SHALL be built from those groups. Across the groups and the individual variables named in `vars`, every declared variable SHALL appear exactly once. The runner SHALL check this statically for any TLA+ model whose `vars` is built from named groups. Action names SHALL be those reported in TLC coverage output and named in `dead_actions`.

#### Scenario: Group duplicates or omits a variable
- **WHEN** a variable appears in two groups, or in no group named in `vars`
- **THEN** the runner SHALL fail before running TLC and name the model and the variable

#### Scenario: Over-broad UNCHANGED group
- **WHEN** a refactored action's `UNCHANGED` group includes a variable the action also assigns
- **THEN** checking the pre-refactor `Spec` against `[][NewNext]_vars` SHALL report a violation

### Requirement: Before-and-after evidence for infrastructure refactors
`scripts/formal.py snapshot` SHALL write a canonical record of:
- every command's verdict;
- for TLC commands, the violated property, distinct-state count, and zero-coverage actions;
- every trace suite's counts and per-trace verdicts;
- for each golden file, the SHA-256 of its decoded instance sequence in an encoding-independent canonical form.

`snapshot --compare FILE` SHALL exit 0 only when every entry is identical, and otherwise SHALL list each differing entry with its old and new values.

#### Scenario: Identical record
- **WHEN** `scripts/formal.py snapshot --compare baseline.json` runs on a tree whose recorded entries all match the baseline
- **THEN** it SHALL exit 0

#### Scenario: A change alters an entry
- **WHEN** any command verdict, violated property, distinct-state count, coverage, trace verdict, or golden instance digest differs from the baseline
- **THEN** `snapshot --compare` SHALL exit non-zero and name the entry with its old and new values

#### Scenario: Golden digest is encoding-independent
- **WHEN** the same instance sequence is stored in format 1 and in format 2
- **THEN** `snapshot` SHALL record the same golden digest for both
