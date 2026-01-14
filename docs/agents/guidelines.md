# General Guidelines

- **Always use Context7 MCP** when you need library/API documentation, code generation, setup, or configuration steps without the user having to explicitly ask.
- In all planning and design decisions, always consider that the code must be highly testable, maintainable, and extensible.
- Always add or adjust unit tests for behavior changes; add integration tests when changes touch integrations or cross-component workflows.
- Always document the code (functions, structs, etc.) with comments.
- Always use descriptive variable names.
- Always adjust the README and other documentation as needed when making significant changes to the codebase.
- Always refactor unit and integration tests when needed after code changes, considering the design and semantics of the code changes.
- After you finish code design and implementation changes, always double-check for leftovers that were not removed or changed after refactoring (e.g.: tests, CUE type definitions, README or documentation instructions, etc.).
- Always follow the best practices for the programming language being used.
