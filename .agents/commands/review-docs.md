Review all documentation surfaces for accuracy against the current codebase. Use the `/review-docs` skill (`.agents/skills/review-docs/SKILL.md`).

## Workflow

1. **Load the skill**: Read `.agents/skills/review-docs/SKILL.md`. Each subagent reads only its assigned reference files (see the orchestration table in the skill).
2. **Run programmatic checks first** (see `references/verification-commands.md`): `npm run docs:parity`, `npm run build`, `check-diagram-readability.sh`, D2 validation, container image policy grep, `make check-agent-docs`.
3. **Spawn parallel subagents** for the 4 surface groups defined in the skill's Orchestration Strategy.
4. **Each subagent**: reviews assigned surfaces, checks against the intentional-simplifications registry, produces findings in the structured format.
5. **Merge findings**: deduplicate, sort by severity, cross-check against intentional simplifications.
6. **Produce final report** using the structured output format from `references/structured-output-format.md`.

## Scope Rules

- Review ONLY `website/docs/` (next version). Do NOT review or change `website/versioned_docs/`.
- Review `README.md` for accuracy against code.
- Do NOT flag intentional simplifications (check `references/intentional-simplifications.md`).
- Delegate diagram-specific review details to the `/d2-diagrams` skill.
- Use the `/docs` skill only if edits are needed after review (this command is review-only).
- Use subagents and teammates coordinated by you.
