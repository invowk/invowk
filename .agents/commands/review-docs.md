Review all documentation surfaces for accuracy against the current codebase. Use the `/review-docs` skill (`.agents/skills/review-docs/SKILL.md`).

## Workflow

1. **Load the skill**: Read `.agents/skills/review-docs/SKILL.md`. Each subagent reads only its assigned reference files (see the orchestration table in the skill).
2. **Run programmatic checks first** (see `references/verification-commands.md`): `npm run docs:parity`, `npm run build`, `check-diagram-readability.sh`, D2 validation, container image policy grep, `make check-agent-docs`. Record results in the Context Block format.
3. **Spawn 8 parallel subagents** — one per surface (S1–S8) as defined in the skill's Orchestration Strategy. Use the Subagent Prompt Template from the skill to ensure consistent prompting.
4. **Each subagent**: follows its surface checklist from `references/surface-checklists.md`, reports PASS/FAIL/N-A for every item, and produces findings for FAIL items using the structured format.
5. **Merge findings**: verify checklist completeness, deduplicate, sort by severity, cross-check against intentional simplifications.
6. **Produce final report** using the structured output format from `references/structured-output-format.md`, including the unified Checklist Completion summary.

## Scope Rules

- Review ONLY `website/docs/` (next version). Do NOT review or change `website/versioned_docs/`.
- Review `README.md` for accuracy against code.
- Do NOT flag intentional simplifications (check `references/intentional-simplifications.md`).
- Delegate diagram-specific review details to the `/d2-diagrams` skill.
- Use the `/docs` skill only if edits are needed after review (this command is review-only).
- Use subagents and teammates coordinated by you.
