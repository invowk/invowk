Review all documentation surfaces for accuracy against the current codebase. Use the `/review-docs` skill (`.agents/skills/review-docs/SKILL.md`).

## Workflow

1. **Load the skill**: Read `.agents/skills/review-docs/SKILL.md`. Each subagent reads only its assigned reference files (see the orchestration table in the skill).
2. **Run programmatic checks first** (see `references/verification-commands.md`): `npm run docs:parity`, live doc inventory capture, container image policy grep, `check-diagram-readability.sh`, D2 validation, `make check-agent-docs`, `node scripts/validate-version-assets.mjs`, `npm run typecheck`, and `npm run build`. Record results in the Context Block format.
3. **Spawn 11 subagents** — one per surface (S1–S11) as defined in the skill's Orchestration Strategy. If the runtime has a live-subagent cap, queue later surfaces in deterministic order and launch them only as slots become available.
4. **Each subagent**: follows its surface checklist from `references/surface-checklists.md`, reports PASS/FAIL/N/A for every item, and produces findings only for FAIL items that satisfy the structured Finding Admission Gate.
5. **Merge findings**: verify checklist completeness, reject incomplete or speculative findings, deduplicate, sort by severity, cross-check against intentional simplifications.
6. **Produce final report** using the structured output format from `references/structured-output-format.md`, including the unified Checklist Completion summary.

## Scope Rules

- Review ONLY `website/docs/` (next version). Do NOT review or change `website/versioned_docs/`.
- Review `README.md` for accuracy against code.
- Do NOT flag intentional simplifications (check `references/intentional-simplifications.md`).
- Do NOT flag style preferences, wording nuance, or possible omissions unless they map to an explicit checklist item with exact source-of-truth evidence.
- Delegate diagram-specific review details to the `/d2-diagrams` skill.
- Use the `/docs` skill only if edits are needed after review (this command is review-only).
- Use subagents and teammates coordinated by you.
