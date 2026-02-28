We're on a long multi-step work to completely avoid the use of simple primitive types (including strings) and — generally but not always when unpractical — mutability in invowk's Go code. Instead, we must always use either 1) type definitions or 2) structs whenever possible.

## Type Definitions
- MUST have a `Validate() error` method with validation logic
- MUST have a `String() string` method
- MUST have sentinel error `ErrInvalid<Type>` + typed error struct `Invalid<Type>Error` with `Unwrap()` returning the sentinel
- Mark zero-value-invalid types with `//goplint:nonzero`
- Mark compile-time-constant-only types with `//goplint:constant-only`
- Enum types with CUE counterparts MUST have `//goplint:enum-cue=<path>`

## Structs
- Constructor-backed structs should use `NewXxx()` returning `(*T, error)`
- Functional options only when >3 optional parameters; not every struct needs a constructor (CUE-parsed DTOs and data carriers are fine as-is)
- Structs with constructors should be immutable (unexported fields + getters) unless marked `//goplint:mutable`
- Structs with `Validate()` + validatable fields should use `//goplint:validate-all` for delegation completeness checking
- Constructor `Validate()` calls must exist unless `//goplint:constant-only`

## Import cycles
- If importing an existing type would create circular dependencies, move the type to `pkg/types` or another more appropriate package.

All methods MUST have unit tests for ALL conditions.

Identify ALL remaining gaps to be worked on and propose a robust plan. All pre-existing issues found during your planning MUST be fully resolved as well.

## Tool Support

Run `make check-types-all-json` for a structured JSON report.
Key diagnostic categories:
- `primitive` — bare primitive in struct field / param / return
- `missing-validate` / `missing-stringer` — missing methods
- `missing-constructor` / `wrong-constructor-sig` — constructor issues
- `missing-immutability` — exported fields on constructor-backed structs
- `unvalidated-cast` — DDD cast without Validate() check (CFA-enabled)
- `incomplete-validate-delegation` — missing field Validate() calls
- `nonzero-value-field` — nonzero type used as value (should be *Type)
- `enum-cue-missing-go` / `enum-cue-extra-go` — CUE/Go enum drift

See `tools/goplint/CLAUDE.md` for all 18+ categories and directives.

## Workflow
1. `make check-baseline` — verify no regressions first
2. Apply type improvements
3. `make update-baseline` — shrink baseline
4. `make check-baseline` — verify clean
5. Commit updated `tools/goplint/baseline.toml`

## Cost-Benefit Rule
If typing adds more `string()` casts than it removes (e.g., `filepath.Join`, `os.Stat`, `exec.LookPath` boundaries), DEFER the typing and add an exception to `tools/goplint/exceptions.toml` with a `reason` and optionally `review_after` + `blocked_by` fields.
