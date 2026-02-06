# General Rules

## Instruction Priority

- Follow system and user instructions first, then repo guidance (`.claude/`).

## Sources and Tooling

- Use Context7 MCP when you need library/framework docs, setup, or config steps; follow the tool protocol (resolve library ID first, respect call limits).
- Prefer local docs and source over external references; respect sandbox and approval constraints.

## Code Quality

- Keep changes focused and maintainable; favor small, testable units when designing changes.
- Use clear, descriptive naming and follow language/project conventions.
- Prefer explicit dependency injection and request-scoped inputs over global mutable state.
- Keep orchestration/parsing/rendering concerns separate from domain logic; adapters call services, services return typed results.

## Comments

- Add comments above method/function/interface/struct declarations to explain the semantics of those constructs.
- Add comments inside of method/function bodies when behavior is non-obvious or business rules are subtle.
- **Preserve comments during refactors**: When moving or splitting code across files (e.g., extracting helpers, splitting large files), carry semantic comments to their new locations. Adapt comments to reflect the new architecture — do not copy stale references to removed patterns or globals.

## Tests

- When behavior changes, add/update unit tests; add and/or update integration tests for cross-component or external integrations.
- If tests are skipped, call it out and explain why.

## Documentation and Diagrams

- Update README and website docs when user-facing behavior/configuration, CLI/API, or schemas change.
- **Evaluate and update architecture diagrams** (`docs/architecture/`) when changes affect:
  - Component relationships or package boundaries
  - Execution flow or command processing
  - Runtime selection or discovery logic
  - New external integrations or major features
- A task is NOT complete until both documentation and diagrams reflect current behavior.

## Cleanup

- After refactors, check for stale references (tests, CUE types, docs, examples).

## Plans and Fixes

CRITICAL: Whenever asked to propose or work on Plans, ToDos, fixes, and so on, if you uncover any unforeseen architectural, design, or implementation issues, you MUST ask whether you should also address those new issues in a comprehensive, coherent, and consistent way with what you were originally asked to do.
