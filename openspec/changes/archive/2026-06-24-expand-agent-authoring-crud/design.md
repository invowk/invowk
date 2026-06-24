## Context

Invowk currently exposes `invowk agent cmd prompt` and `invowk agent cmd create [description...]`. The command creation flow resolves an LLM provider, sends command-authoring prompts plus schemas, validates that the model returned exactly one command object, and patches a target `invowkfile.cue`.

That surface is intentionally useful but incomplete:

- command identity is selected by the model instead of the caller;
- replacement is folded into `create --replace`;
- removal is not supported;
- local module authoring has no agent surface, even though modules are the primary reusable packaging unit;
- top-level `invowk module remove` already means dependency removal, so `agent mod remove` needs a different and explicit safety contract.

This change is a clean CLI break. Existing `agent cmd create 'description only'` calls should fail after the change because create requires the durable identity as a positional argument.

## Goals / Non-Goals

**Goals:**
- Make `agent cmd` and `agent mod` sibling authoring namespaces with explicit `create`, `change`, `remove`, and `prompt` operations.
- Require caller-owned identities for create and change operations.
- Replace `create --replace` with `change`.
- Keep the public CLI explicit instead of adding a single coalesced CRUD command.
- Reuse shared prompt, completion, response parsing, diff, validation, and write-mode machinery internally where practical.
- Keep remove operations deterministic and guarded.
- Update tests, README, website docs/snippets, diagrams, and review-doc references for the clean-break contract.

**Non-Goals:**
- Preserve the legacy `agent cmd create [description...]` contract.
- Keep `--replace` on `agent cmd create`.
- Add ACP-backed interactive agent sessions.
- Make `agent mod remove` remove module dependencies from `invowkmod.cue`; top-level `invowk module remove` keeps that meaning.
- Let LLM responses create or edit arbitrary script files in this change.
- Change `invowk cmd`, `invowk module`, or `invowk audit` provider semantics except for documentation references to the authoring surface.

## Decisions

### Keep explicit public verbs

Expose:

```text
invowk agent cmd prompt [operation]
invowk agent cmd create <name> [description...]
invowk agent cmd change <name> [description...]
invowk agent cmd remove <name>

invowk agent mod prompt [operation]
invowk agent mod create <module-id> [description...]
invowk agent mod change <module-id-or-path> [description...]
invowk agent mod remove <module-id-or-path>
```

Alternative considered: one public command such as `agent cmd apply --op create|change|remove`. That centralizes parsing but makes help text, shell completion, tests, and safety preconditions less obvious. Explicit verbs match the user's mental model and Cobra's strengths.

### Caller owns identity; model owns content

For command operations, the `<name>` positional argument is the command name. For module creation, `<module-id>` is the module identity and local directory prefix. The model may generate descriptions, implementation details, dependency declarations, and command bodies, but the returned CUE must match the caller-supplied identity exactly.

Alternative considered: keep letting the model choose names. That makes natural-language authoring shorter but weakens idempotence and forces users to discover what durable name was chosen after the fact.

### Replace `--replace` with `change`

`create` must fail if the target identity already exists. `change` must fail if the target identity does not exist. This makes command behavior symmetric and removes the ambiguous "create but replace" mode.

Alternative considered: retain `--replace` for compatibility. The user requested a clean break, and keeping both paths would create two ways to express the same mutating operation.

### Keep remove deterministic

`agent cmd remove <name>` removes an existing command object from a target invowkfile without calling an LLM. `agent mod remove <module-id-or-path>` removes a local module directory through explicit filesystem safety checks and force confirmation rules, not through model output.

Alternative considered: ask the LLM to generate deletion patches. That adds unnecessary risk because deletion targets are already exact positional arguments.

### Bound module authoring to local module files

`agent mod create` may generate `invowkmod.cue` and `invowkfile.cue`, plus the optional scripts directory scaffold. `agent mod change` may update `invowkmod.cue` and `invowkfile.cue` for an existing local module. This change should not create or edit arbitrary script files; generated commands should use embedded `script.content` or reference existing files only when the user explicitly asks.

Alternative considered: allow generated script file trees. That is useful eventually, but it expands file write safety, diff rendering, and verification requirements beyond this CLI contract change.

### Share internal operation machinery without hiding public intent

The implementation should factor common pieces such as description loading, LLM resolution, structured completion fallback, response repair, CUE formatting, unified diff rendering, dry-run/print/write modes, and validation. The public Cobra commands should stay small adapters that assemble operation-specific options and call the internal authoring services.

Alternative considered: duplicate `agentcmd.CreateCommand` into each operation. That would land faster but make prompt/repair/validation drift likely.

## Risks / Trade-offs

- Breaking `agent cmd create` arity may surprise existing users -> Mitigate with clear release notes, docs, and tests that assert the new error.
- Module removal can delete user data -> Mitigate with exact module validation, dry-run support, `--force` for destructive writes, symlink rejection, and no dependency-removal behavior.
- Multi-file module generation makes `--print` and `--dry-run` output more complex -> Mitigate by using structured print output and file-level diffs for dry-run.
- Model output may ignore the requested identity -> Mitigate by validating command name, module ID, and generated file names before any write.
- Documentation drift is likely because the authoring surface appears in README, website docs, snippets, diagrams, and review-doc references -> Mitigate by treating documentation updates and `make check-agent-docs` as first-class tasks.

## Migration Plan

1. Introduce the new command tree and operation-specific validation while removing the legacy create-only assumptions.
2. Refactor existing command creation internals into reusable generation, validation, patch, diff, and render helpers.
3. Add command change/remove behavior and migrate tests away from `--replace`.
4. Add module create/change/remove/prompt behavior on top of existing module scaffold and validation packages.
5. Update docs, snippets, diagrams, and agent review references.
6. Run targeted Go tests, CLI txtar tests, documentation checks, and the standard local gates.

Rollback before release is to restore the previous `agent cmd create [description...]` command and remove the new `change`, `remove`, and `mod` commands. After release this is a documented breaking change, so rollback would require a new compatibility proposal.

## Open Questions

None. Implementation can refine exact result wording and JSON field names as long as the CLI contract and validation scenarios in the spec remain intact.
