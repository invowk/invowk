We're in a long multi-step work to fully convert invowk's Go version (source directory which must NOT be changed) to a Rust version (target directory), with 100% surface-area equivalence and feature-parity with matching config, UI style, UX, and behavior.

The Rust version MUST use Domain-Driven Design in all its aspects (strong encapsulation, no primitive types allowed in function signatures or structs, etc.). Refactor pre-existing Rust files as needed to fix that.

CRITICAL: NO Rust file can be longer than 1000 lines (including both production and test code) under any circumstances. Plan abstractions, patterns, etc. accordingly. If changes will exceed that limit, refactor.

CRITICAL: Rust must have semantically equivalent tests to Go's -- adapted to Rust's design and implementation -- plus any other tests that are appropriate to it. Ask me about libraries, frameworks, and foundational choices.

Identify the next best foundational items to continue with the conversion and propose a robust and very detailed plan + tasks.