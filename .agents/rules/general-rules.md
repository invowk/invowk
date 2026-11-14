# General Rules

## Sources

- Use Context7 MCP for library/framework docs, setup and configuration: resolve the library ID first and respect its call limits.
- Prefer local docs and source over external references; respect sandbox and approval constraints.

## Code

- Keep changes focused, small and testable; use clear names and follow language and project conventions.
- Prefer explicit dependency injection and request-scoped inputs over global mutable state.
- Keep orchestration, parsing and rendering out of domain logic: adapters call services, services return typed results.
- Follow the semantic commenting rules in `.agents/skills/go/SKILL.md` for Go code. When moving or splitting code, carry semantic comments along and adapt them to the new architecture; never copy stale references.
- After refactors, sweep for stale references (tests, CUE types, docs, examples).

## Tests

- When behavior changes, add or update unit tests, plus integration tests for cross-component or external integrations.
- If tests are skipped, say so and explain why.

## Scope

CRITICAL: whenever you propose or work on plans, to-dos or fixes and uncover unforeseen architectural, design or implementation issues, you MUST ask whether to also address them in a way that is comprehensive, coherent and consistent with the original request.
