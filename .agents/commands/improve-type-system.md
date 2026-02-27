We're on a long multi-step work to completely avoid the use of simple primitive types (including strings) and -- generally but not always when unpractical -- mutability in invowk's Go code. Instead, we must always use either 1) type definitions or 2) structs whenever possible.

## Type Definitions
- MUST have a 'Validate() error' method with validation logic, returning an error (possibly multi-error via errors.Join) with all applicable custom error types if the validation failed
- should generally have additional semantic methods as per Domain-Driven Design's Value Types's concept if concrete possible uses have been identified

## Structs
- MUST have strong constructor functions that make use of the functional options pattern and whose use is enforced across the project
- MUST enforce that all its constructor functions return '(instance T, error)' with all applicable custom error types if the initialization validation failed
- MUST use only non-primitive types for fields
- MUST be immutable (if they're DDD Value Types) with only unexported fields and public accessor methods unless very unpractical for our use-cases; otherwise, if they're DDD Entities, they can remain mutable if it's best.

## Import cycles
- If the import/use of an existing type would create circular dependencies or similar issues, you MUST proceed even so by moving the type to `pkg/types` or another more appropriate package.

All methods MUST have unit tests for ALL conditions.

Identify ALL remaining gaps to be worked on and propose a robust plan. All pre-existing issues found during your planning MUST be fully resolved as well.

## Tool Support

Run `make check-types-all-json` to get a structured JSON report of all DDD gaps.
Each diagnostic includes a `category` field for filtering:
- `primitive` — bare primitive in struct field / function param / return type
- `missing-validate` — named type missing `Validate()` method
- `missing-stringer` — named type missing `String()` method
- `missing-constructor` — exported struct missing `NewXxx()` constructor

Use this output as the canonical source of remaining gaps instead of manually scanning the codebase. See `tools/goplint/CLAUDE.md` for full documentation.

After completing type improvements, run `make update-baseline` to shrink the baseline and commit the updated `tools/goplint/baseline.toml`.