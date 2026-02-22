We're on a long multi-step work to completely avoid the use of simple primitive types (including strings) and -- generally but not always when unpractical -- mutability in invowk's Go code. Instead, we must always use either 1) type definitions or 2) structs whenever possible.

## Type Definitions
- MUST have an 'IsValid() (isValid bool, errors []error)' method with validation logic, returning the validation result and all applicable custom error types if the validation failed
- should generally have additional semantic methods as per Domain-Driven Design's Value Types's concept if concrete possible uses have been identified

## Structs
- MUST have strong constructor functions that make use of the functional options pattern and whose use is enforced across the project
- MUST enforce that all its constructor functions return '(instance T, errors []error)' with all applicable custom error types if the initialization validation failed
- MUST use only non-primitive types for fields
- MUST be immutable with only unexported fields and public accessor methods unless very unpractical for our use-cases

All methods MUST have unit tests for ALL conditions.

Identify ALL remaining gaps to be worked on and propose a robust plan. All pre-existing issues found during your planning MUST be fully resolved as well.