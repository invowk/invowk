# Invowk Value-Type Patterns

This reference defines the canonical design for Invowk value types.

## 1. Primitive-Wrapper Value Type (Preferred)

Use a primitive wrapper when a domain concept can be represented by one scalar value with domain-specific validation.

Sentinel messages are constant-backed, per the `go` skill's Errors section.

```go
const invalidExampleErrMsg = "invalid example"

// ErrInvalidExample is the sentinel error wrapped by InvalidExampleError.
var ErrInvalidExample = errors.New(invalidExampleErrMsg)

type (
    // Example is a domain-specific wrapper around a primitive.
    Example string

    // InvalidExampleError wraps ErrInvalidExample for errors.Is compatibility.
    InvalidExampleError struct {
        Value  Example
        Reason string
    }
)

func (e Example) String() string { return string(e) }

func (e Example) Validate() error {
    if strings.TrimSpace(string(e)) == "" {
        return &InvalidExampleError{Value: e, Reason: "must not be empty"}
    }
    return nil
}

func (e *InvalidExampleError) Error() string {
    return fmt.Sprintf("invalid example %q: %s", e.Value, e.Reason)
}

func (e *InvalidExampleError) Unwrap() error { return ErrInvalidExample }
```

## 2. Composite Validator Type

Use a composite validator when invariants span multiple fields and cannot be expressed by a single scalar wrapper.

```go
type ExampleConfig struct {
    Name  ExampleName
    Limit int
}

func (c ExampleConfig) Validate() error {
    var errs []error

    if err := c.Name.Validate(); err != nil {
        errs = append(errs, err)
    }
    if c.Limit < 0 {
        errs = append(errs, errors.New("limit must be >= 0")) // no verbs: errors.New (perfsprint)
    }

    return errors.Join(errs...)
}
```

## 3. Alias/Re-export Pattern

Use aliases only when intentionally preserving compatibility boundaries.

```go
type (
    DescriptionText = types.DescriptionText
    InvalidDescriptionTextError = types.InvalidDescriptionTextError
)

var ErrInvalidDescriptionText = types.ErrInvalidDescriptionText
```

## 4. Sentinel + Typed Error Rules

- Sentinel variable: `ErrInvalid<Type>`.
- Typed error: `Invalid<Type>Error`.
- Typed error should include contextual fields (`Value`, `Reason`, etc.).
- `Unwrap()` must return the sentinel to preserve `errors.Is` behavior.

## 5. Validation Method Rules

- Signature must be `Validate() error`.
- For composite validators, use `errors.Join(errs...)` to return all actionable errors when multiple checks are independent.
- Keep messages precise and stable enough for tests.
- Use domain terminology in error text, not generic phrasing.

## 6. Naming and Design Guidance

- Use domain names (`ModuleID`, `PortProtocol`, `ContainerImage`) instead of generic names.
- Prefer wrapper types over raw primitives in public/internal contracts.
- Reuse existing wrappers from the catalog before introducing a new one.
- If a wrapper already exists in `pkg/types`, prefer re-export aliases over duplication.

## 7. goplint Directives Used In This Repository

Annotations seen in `cmd/`, `internal/`, and `pkg/` (rebuild the list with
`rg -oN --no-filename '//goplint:[a-z-]+' cmd internal pkg | sort | uniq -c`):
`//goplint:ignore -- <reason>`, `validate-all`, `nonzero`, `constant-only`,
`enum-cue`, `mutable`, `cue-fed-path`, `trusted-boundary`, `no-delegate`, and
`path-domain`. Their semantics are documented in the goplint repository
(`github.com/invowk/goplint`, `docs/goplint/`), not here; read them there
before adding a directive. Copy an existing in-repo use as the template.

## 8. Anti-patterns to Avoid

- Adding new function parameters/struct fields as raw `string`/`int` where a domain wrapper exists.
- Returning only one error when multiple field errors are known.
- Skipping `Unwrap()` in typed invalid errors.
- Introducing aliases without documenting compatibility intent.
