# General Rules

## Instruction Priority

- Follow system and user instructions first, then repo guidance (`.claude/`).

## Sources and Tooling

- Use Context7 MCP when you need library/framework docs, setup, or config steps; follow the tool protocol (resolve library ID first, respect call limits).
- Prefer local docs and source over external references; respect sandbox and approval constraints.

## Code Quality

- Keep changes focused and maintainable; favor small, testable units when designing changes.
- Use clear, descriptive naming and follow language/project conventions.

## Comments

- Add comments above method/function/interface/struct declarations to explain the semantics of those constructs.
- Add comments inside of method/function bodies when behavior is non-obvious or business rules are subtle.

## Tests

- When behavior changes, add/update unit tests; add and/or update integration tests for cross-component or external integrations.
- If tests are skipped, call it out and explain why.

## Documentation

- Update README and website docs when user-facing behavior/configuration, CLI/API, or schemas change.

## Cleanup

- After refactors, check for stale references (tests, CUE types, docs, examples).

## Plans and Fixes

CRITICAL: Whenever asked to propose or work on Plans, ToDos, fixes, and so on, if you uncover any unforeseen architectural, design, or implementation issues, you MUST ask whether you should also address those new issues in a comprehensive, coherent, and consistent way with what you were originally asked to do.
