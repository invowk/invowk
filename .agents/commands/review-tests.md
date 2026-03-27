Review our whole test suite (unit, integration, CLI, domain-specific) using the structured
review-tests skill. Evaluates 102 checklist items across 8 surfaces: structural hygiene,
parallelism/context, test patterns/assertions, integration gating, testscript quality,
virtual/native mirrors, coverage guardrails, and TUI/domain-specific testing.

Uses 8 parallel subagents with deterministic file traversal and pre-assigned severity.
Detects both coverage gaps (missing branches, error paths) and low-value tests (circular,
tautological, excessive mocking). Produces a structured report with RT-{NNN} findings.

See `.agents/skills/review-tests/SKILL.md` for the full orchestration workflow.
