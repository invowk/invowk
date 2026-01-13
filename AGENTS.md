## Built-in Examples of Invowk Commands (invowkfile.cue at project root)

- All commands should be idempotent and not cause any side effects on the host.
- No commands should be related to building invowk itself or manipulating any of its source code.
- Examples should range from simple (e.g.: native 'hello-world') to complex (e.g.: container 'hello-world' with the enable_host_ssh feature).
- Examples should illustrate the use of different features of Invowk, such as:
  - Native vs. Container execution
  - Volume mounts for Container execution
  - Environment variables
  - Host SSH access enabled vs. disabled
  - Capabilities checks (with and without alternatives)
  - Tools checks (with and without alternatives)
  - Custom checks (with and without alternatives)

## Key Guidelines

- In all planning and design decisions, always consider that the code must be highly testable, maintainable, and extensible.
- Always add unit and integration tests to new code.
- Always document the code (functions, structs, etc.) with comments.
- Always use descriptive variable names.
- Always adjust the README and other documentation as needed when making significant changes to the codebase.
- Always refactor unit and integration tests when needed after code changes, considering both the design and semantics of the code changes.
- After you finish code design and implementation changes, always double-check for leftovers that were not removed or changed after refactoring (e.g.: tests, CUE type definitions, README or documentation instructions, etc.).
- Always follow the best practices for the programming language being used.