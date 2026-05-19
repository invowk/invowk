## Context

Invowk currently stores `interpreter` on `RuntimeConfig` for native and container runtimes. That makes interpreter selection a property of where a script runs, even though it describes how the script bytes are interpreted. It also leaves `custom_checks.script` without the same explicit interpreter support used by command implementations.

The current explicit script model already separates script source as `script.content` or `script.file`, and `script.file` is constrained to invowkmod-contained files. This change extends that model so `script` owns both source selection and interpreter selection while runtime blocks keep only environment/runtime concerns.

The change is intentionally a clean break. The implementation must remove runtime-level interpreter design, parsing, generation, docs, fixtures, tests, helper APIs, and execution wiring instead of preserving aliases, compatibility shims, or ignored legacy fields.

## Goals / Non-Goals

**Goals:**
- Make `script.interpreter` the only explicit interpreter field for implementation scripts and custom-check scripts.
- Keep `script.content` and `script.file` as the only script source variants and preserve their mutual exclusivity.
- Keep `script.file` module-only and module-contained for both implementations and custom checks.
- Let shebang detection work from resolved script bytes for both inline and file-backed implementations and custom checks.
- Make custom checks portable by defaulting shell-like host checks to the embedded mvdan/sh runtime instead of fixed host shell paths.
- Ensure native, container, and virtual execution have clear interpreter rules.
- Update all user-facing docs, snippets, samples, generated output, test fixtures, benchmark fixtures, and schema references.

**Non-Goals:**
- No backward compatibility for runtime-level `interpreter`.
- No legacy aliases, dual-read decoding, compatibility warnings, generated old-shape examples, ignored runtime-level fields, or compatibility-only helper functions.
- No new arbitrary interpreter execution surface beyond the existing `InterpreterSpec` allowlist and parser rules.
- No change to `workdir`, runtime selection, platform selection, env inheritance, container image/containerfile selection, or `script.file` containment policy except where tests/docs must mention interactions.

## Decisions

### Decision 1: `interpreter` belongs to the script object

Add `interpreter?: #InterpreterSpec` to the script source structs shared by implementation scripts and custom-check scripts. The field is metadata on the script object, not a third source variant, so these remain valid shapes:

```cue
script: {
	content: "print('ok')"
	interpreter: "python3"
}
```

```cue
script: {
	file: "scripts/check.py"
	interpreter: "python3"
}
```

`content` and `file` remain exactly-one source fields. `interpreter` may be omitted, set to `"auto"`, or set to an allowlisted interpreter spec.

Alternatives considered:
- Keep interpreter on runtime configs: rejected because it couples script language to execution environment and cannot naturally support custom checks.
- Add separate `source` and `interpreter` siblings under implementation/custom-check: rejected because the current clean `script.content` / `script.file` shape already provides the right owner.

### Decision 2: Remove runtime-level interpreter completely

Delete `interpreter` from native and container runtime schema structs, Go `RuntimeConfig`, generation, parsing expectations, tests, and documentation. Runtime configs continue to describe environment concerns such as `name`, env inheritance, container image/containerfile, volumes, ports, persistent container settings, host SSH, and container-scoped dependency checks.

Runtime-level `interpreter` must fail as an unknown field through closed CUE structs. Direct Go construction with stale `RuntimeConfig.Interpreter` must not compile after the field is removed.

Alternatives considered:
- Keep `RuntimeConfig.Interpreter` as an ignored field: rejected because it leaves legacy design behind and can silently mislead users.
- Preserve read-only migration support: rejected because this is a clean break.

### Decision 3: Interpret after source resolution

Interpreter resolution always uses the final script bytes:

1. Resolve `script.content` directly, or read `script.file` after module-boundary validation.
2. If `script.interpreter` is omitted or `"auto"`, parse the first line for a shebang.
3. If an explicit `script.interpreter` is set, parse it and ignore any shebang.
4. If no interpreter is found, use the runtime or custom-check default shell behavior.

This keeps shebang behavior identical for inline and file scripts and ensures `script.file` content, not the file extension, determines auto interpreter detection.

### Decision 4: Runtime-specific execution remains environment-specific

Native implementations execute explicit interpreters on the host via PATH/path lookup. Container implementations execute explicit interpreters inside the selected container environment. Virtual implementations use mvdan/sh for omitted/auto/no-shebang scripts and for shell-compatible shebangs or explicit shell-compatible interpreter specs. Virtual implementations must reject non-shell interpreters.

Shell-compatible virtual interpreter specs are normalized to mvdan/sh execution rather than requiring host `/bin/sh`, `bash`, or Windows shell paths. This preserves the virtual runtime promise: built-in shell interpreter, not host shell discovery.

### Decision 5: Custom checks share script interpreter semantics

Custom checks use the same `CustomCheckScript` source and interpreter model as implementations. Before a custom check runs, dependency resolution must produce both resolved script content and the script-level interpreter spec.

Host custom checks:
- Omitted/`"auto"` with no shebang runs through the embedded mvdan/sh shell on every platform.
- Shell-compatible explicit interpreters or shebangs run through the embedded mvdan/sh shell.
- Non-shell explicit interpreters or shebangs run through the allowlisted host interpreter, with normal PATH/path lookup.

Container custom checks:
- Run inside the selected container runtime when declared under container runtime `depends_on`.
- Omitted/`"auto"` with no shebang uses the container default shell behavior.
- Explicit or shebang interpreters are resolved inside the container.

This blend avoids hardcoded Windows shell paths while still letting users intentionally check host Python, Node, Ruby, or similar tools.

### Decision 6: Docs and generated examples must use only the new shape

Every documentation and fixture surface must prefer examples like:

```cue
script: {
	content: """
	print("hello")
	"""
	interpreter: "python3"
}
runtimes: [{name: "native"}]
```

Runtime examples must not include `interpreter`. Custom-check docs must include examples for default shell checks, explicit interpreter checks, shebang checks, and module-contained `script.file` checks.

## Risks / Trade-offs

- [Risk] Multi-runtime implementations can no longer assign different explicit interpreters per runtime in one implementation. → Mitigation: require separate implementations when script language or interpreter command differs by environment; this is clearer and matches platform/runtime matching.
- [Risk] Virtual shell-compatible `bash` or `zsh` declarations may imply shell features mvdan/sh does not implement. → Mitigation: document that virtual accepts only shell-compatible intent and still runs mvdan/sh; users needing Bash-specific behavior must use native or container runtime.
- [Risk] Host custom checks with non-shell interpreters depend on host PATH and can vary by machine. → Mitigation: keep explicit interpreter allowlist validation, document the environment boundary, and add tests for missing interpreter diagnostics.
- [Risk] Documentation drift is likely because interpreter examples appear in README, website snippets, versioned snippets, i18n current docs, and reference pages. → Mitigation: include targeted search tasks and docs verification gates.
- [Risk] Generated fixtures such as benchmark invowkfiles can regress by emitting old runtime-level or old script shapes. → Mitigation: explicitly include generator/fixture tasks and CLI/benchmark smoke coverage.

## Clean-Break Removal Plan

1. Update the CUE schema and Go value model so only script objects own interpreter specs.
2. Rewire interpreter resolution in native, virtual, container, and dependency custom-check execution paths.
3. Remove all runtime-level interpreter generation, validation, helper APIs, tests, and docs.
4. Add parser, schema sync, runtime, dependency, CLI, and docs tests for the new contract.
5. Update README, website current docs, i18n current docs, snippets, generated references, samples, benchmark fixtures, and authoring guidance.
6. Run OpenSpec, schema, Go, CLI, docs, and repo guardrail checks before implementation is considered complete.

## Open Questions

- Should `script.interpreter: "bash"` with virtual runtime be accepted as shell-compatible mvdan/sh intent, or should only `sh`/`/bin/sh`/`env sh` be accepted for virtual? The recommended default is to accept shell-compatible names but document mvdan/sh semantics clearly.
- Should host custom checks with explicit shell names such as `bash` always use mvdan/sh, or should only omitted/auto/default shell checks use mvdan/sh while explicit `bash` invokes host Bash? The recommended default is mvdan/sh for shell-compatible custom checks to keep checks portable by default.
