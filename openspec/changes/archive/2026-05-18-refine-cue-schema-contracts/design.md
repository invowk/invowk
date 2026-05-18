## Context

Invowk currently uses CUE for declarative shape validation and Go for semantic validation that needs runtime context, filesystem normalization, command discovery, scope policy, or richer diagnostics. That split is sound, but several current contracts are ambiguous:

- `depends_on.cmds[].alternatives` reuses `CommandName`, so source-like strings such as `tools lint` collide with the existing command-name grammar that also allows spaces.
- Documentation and snippets show both source-prefix-with-space and CLI-style source qualification, which can make users wonder whether `com.company.tools lint`, `tools lint`, and `@tools lint` mean the same thing.
- `containerfile` uses a broad CUE `..` substring ban even though the policy users actually need is "no parent-directory path segments".
- Runtime variant CUE disjunctions are structurally correct but can produce generic errors for common mistakes such as `persistent` on a native runtime.
- The config schema uses `#LLMNoBackendConfig` for a shape that can contain common LLM defaults, which makes top-level `model`, `timeout`, and `concurrency` sound backend-less rather than backend-defaulted.
- Some schema comments say what Invowk rejects without making clear whether CUE itself rejects it or Go rejects it after decode.

The user explicitly requested a clean break: do not preserve old syntax, do not leave compatibility aliases, and update all documentation surfaces.

## Goals / Non-Goals

**Goals:**

- Define a single command dependency reference grammar with explicit `@source command` syntax for source-qualified references.
- Preserve bare command references only for local command names, not implicit source qualification.
- Replace broad textual `..` rejection with a path-segment policy: reject path segments exactly equal to `..`, allow ordinary segment names that merely contain `..`.
- Keep runtime variants closed while improving user-facing diagnostics for common runtime-schema mistakes.
- Rename the LLM "no backend" schema concept to a defaults-oriented concept everywhere it appears.
- Make CUE-owned and Go-owned validation boundaries explicit in comments, tests, and docs.
- Update README, current website docs, snippet data, versioned snippets where maintained, samples, schema references, and agent-facing documentation so no stale syntax or terminology remains.

**Non-Goals:**

- No migration shim for old `tools lint` dependency syntax.
- No alias from `#LLMNoBackendConfig` to the new helper name.
- No attempt to make CUE perform filesystem canonicalization or command-scope analysis.
- No change to CLI command execution syntax except docs alignment where it relates to dependency references.
- No archive step in this proposal.

## Decisions

### 1. Introduce `CommandDependencyRef`

Command dependency alternatives should become a distinct value type rather than continuing to reuse `CommandName`.

Reference grammar:

```text
BareRef       = CommandName
QualifiedRef  = "@" SourceID SP CommandName
DependencyRef = BareRef | QualifiedRef
```

Where:

- `CommandName` keeps the current command-name grammar: starts with a letter, then letters, digits, underscore, hyphen, or spaces.
- `SourceID` uses the module source ID grammar: starts with a letter, then letters, digits, dots, underscores, or hyphens.
- `QualifiedRef` splits at the first whitespace after `@SourceID`; the remainder is the command name and may contain spaces.

Examples:

```cue
depends_on: {
  cmds: [
    {alternatives: ["build"]}                 // local command "build"
    {alternatives: ["@tools lint"]}           // command "lint" from source "tools"
    {alternatives: ["@tools test unit"]}      // command "test unit" from source "tools"
    {alternatives: ["@com.acme.tools lint"]}  // command "lint" from dotted source ID
  ]
}
```

Rejected examples:

```cue
{alternatives: ["tools lint"]}       // bare local command name, not source-qualified
{alternatives: ["@tools"]}           // source is present but command is missing
{alternatives: ["@tools  "]}         // command is empty after trimming
{alternatives: ["@9tools lint"]}     // invalid source ID
{alternatives: ["@tools 9lint"]}     // invalid command name
```

Rationale: this removes the ambiguity between a command named `tools lint` and a command named `lint` in source `tools`. It also aligns dependency references with the visible CLI source-disambiguation idiom.

Alternative considered: allow both `tools lint` and `@tools lint`. Rejected because the user requested a clean break and because the old syntax is inherently ambiguous with space-containing command names.

### 2. Make bare dependency refs local

Bare dependency references should resolve only against the caller's own command source.

- Root invowkfile command `depends_on.cmds: ["build"]` resolves to command `build` from source `invowkfile`.
- Module command `depends_on.cmds: ["build"]` resolves to command `build` from the same module source.
- References to another module, global command source, or included command source require `@source command`.

Source-qualified references still pass through the command-scope gate:

- Same source is allowed.
- Direct dependencies declared by the caller module are allowed.
- Global command sources are allowed when the current scope policy allows them.
- Root invowkfile commands may reference discovered sources by explicit `@source`.
- Out-of-scope sources produce forbidden-command dependency diagnostics rather than "missing command" diagnostics when the command exists but is inaccessible.

Rationale: local bare refs keep simple same-file dependencies concise while eliminating accidental binding to a similarly named command from another source.

### 3. Containerfile path policy checks raw normalized segments

The new policy is:

- `containerfile` is relative to the directory containing `invowkfile.cue`.
- Absolute paths are rejected.
- NUL bytes and invalid filename components are rejected.
- Any path segment exactly equal to `..` is rejected after converting backslashes to slashes.
- `.` segments are allowed.
- Segment names that merely contain `..` are allowed, such as `Containerfile..backup` or `v1..2`.

The implementation must detect `..` segments before path cleaning removes them.

Correct algorithm shape:

```text
raw = replaceBackslashesWithSlashes(containerfile)
for segment in split(raw, "/"):
    if segment == "..":
        reject
cleaned = path.Clean(raw)
validate cleaned stays relative and has valid filename component
```

Do not rely only on:

```text
cleaned = path.Clean(raw)
if cleaned starts with "../": reject
```

because `path.Clean("docker/../Containerfile")` becomes `Containerfile`, hiding the parent-directory segment the policy wants to reject.

Examples:

```cue
containerfile: "Containerfile"                 // valid
containerfile: "docker/Containerfile"          // valid
containerfile: "./Containerfile"               // valid
containerfile: "docker/./Containerfile"        // valid
containerfile: "Containerfile..backup"         // valid
containerfile: "docker/v1..2/Containerfile"    // valid
containerfile: "../Containerfile"              // invalid
containerfile: "docker/../Containerfile"       // invalid
containerfile: "docker\\..\\Containerfile"     // invalid
containerfile: "/etc/Containerfile"            // invalid
containerfile: "C:\\Temp\\Containerfile"       // invalid
```

Rationale: this matches the human rule "relative child path, no parent hops" without rejecting harmless filenames.

### 4. Keep runtime CUE variants closed and add targeted diagnostics

The runtime schema should keep native, virtual, and container variants closed. That continues to reject container-only fields on native or virtual runtimes at schema time.

The user-facing diagnostics should improve through an invowkfile parsing/validation diagnostic layer. The implementation can use structured CUE error path information, original user data inspection, or a focused preflight over runtime entries. It must not rely on broad string matching alone when structured information is available.

Targeted examples:

```cue
runtimes: [{name: "native", persistent: {create_if_missing: true}}]
```

Expected diagnostic:

```text
cmds[0].implementations[0].runtimes[0].persistent: persistent is only valid for runtime name: "container"
```

```cue
runtimes: [{name: "container"}]
```

Expected diagnostic:

```text
cmds[0].implementations[0].runtimes[0]: container runtime requires either image or containerfile
```

Rationale: the schema should stay strict, but common mistakes should read like Invowk errors, not raw disjunction details.

### 5. Rename LLM default-only schema concept

Replace `#LLMNoBackendConfig` with `#LLMDefaultsConfig` everywhere.

New conceptual model:

```cue
#LLMConfig: #LLMDefaultsConfig | #LLMProviderConfig | #LLMAPIBackendConfig

#LLMCommonConfig: {
  // Common defaults for whichever backend is selected.
  // api.model overrides this model for API backends.
  model?: string & !="" & strings.MaxRunes(256)
  timeout?: #LLMTimeoutDurationString
  concurrency?: int & >=0
}

#LLMDefaultsConfig: close({
  #LLMCommonConfig
})
```

Rationale: `llm: {model: "..."}` is not "no backend" in user mental models; it is a default applied when a backend is selected by config, CLI flags, or resolver rules.

### 6. Make validation ownership visible

Schema comments should use a consistent pattern:

- "CUE validates ..." for type, enum, length, static regex, and closed-shape rules.
- "Invowk validates after decode ..." for cross-field, filesystem, command-scope, runtime-selection, and security rules.
- "[GO-ONLY]" comments should remain on Go validators for rules CUE cannot own.

Before:

```cue
// Mutually exclusive with 'image'
// [GO-ONLY] Runtime-specific mutual exclusivity with 'image' is checked after decode
containerfile?: ...
```

After:

```cue
// CUE validates field shape only.
// Invowk rejects both containerfile+image and missing container source after decode.
// [GO-ONLY] Runtime source invariants depend on the selected runtime variant.
containerfile?: ...
```

Rationale: this prevents `cue vet` from being mistaken for full Invowk validation.

### 7. Treat docs as part of the contract

Implementation must search and update every public or agent-facing surface that repeats affected syntax or helper names:

- README.
- Current website docs and snippet data.
- Versioned snippet data when the current release stream uses it as source material.
- Samples and generated schema/reference snippets.
- `.agents` rules, skills, command docs, and AGENTS indexes if any affected instruction text exists there.
- Brainstorming or architecture docs when they describe current behavior rather than historical context.

Rationale: the user's explicit requirement is no leftovers from the old behavior.

## Risks / Trade-offs

- **Risk: The clean break may invalidate existing user invowkfiles using `tools lint` dependency references.** Mitigation: reject the old form clearly and update docs with direct before/after examples.
- **Risk: Source-qualified refs can be confused with CLI tokenization.** Mitigation: document that the CUE value is a single string beginning with `@source`, with the command name after the first space.
- **Risk: Runtime diagnostic improvement may duplicate CUE validation logic.** Mitigation: keep the schema as the source of shape validation and add only targeted diagnostics for known runtime fields and container source invariants.
- **Risk: Containerfile path handling can drift between CUE, Go value types, and runtime build behavior.** Mitigation: centralize segment policy in Go tests, update behavioral sync expectations, and document that the path is relative to `invowkfile.cue`.
- **Risk: Renaming `#LLMNoBackendConfig` breaks docs/tests that include schema snippets.** Mitigation: intentionally remove old names and update all snippet surfaces in the same change.
- **Risk: Broad docs update can create generated or versioned churn.** Mitigation: scope changes to surfaces that contain the old syntax or terminology, then run docs/agent-doc integrity checks.
