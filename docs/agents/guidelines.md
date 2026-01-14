# General Guidelines

## Instruction Priority

- Follow system and user instructions first, then repo guidance (AGENTS and docs/agents).

## Sources and Tooling

- Use Context7 MCP when you need library/framework docs, setup, or config steps; follow the tool protocol (resolve library ID first, respect call limits).
- Prefer local docs and source over external references; respect sandbox and approval constraints.

## Code Quality

- Keep changes focused and maintainable; favor small, testable units when designing changes.
- Use clear, descriptive naming and follow language/project conventions.

## Comments

- Add comments above method/function declarations to explain the semantics of those methods/functions.
- Add comments inside of method/function bodies only when behavior is non-obvious or business rules are subtle.

## Tests

- When behavior changes, add/update unit tests; add integration tests for cross-component or external integrations.
- If tests are skipped, call it out and explain why.

## Documentation

- Update README and docs when user-facing behavior, CLI/API, or schemas change.

## Cleanup

- After refactors, check for stale references (tests, CUE types, docs, examples).
