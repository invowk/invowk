# Issue: Create Validator Interface for Invkfile Validation

**Category**: Architecture
**Priority**: Low
**Effort**: Medium (2-3 days)
**Labels**: `architecture`, `refactoring`, `validation`

## Summary

Create a `Validator` interface for invkfile validation to enable custom validators and improve code organization. Currently, 5 validation files have similar patterns but no shared interface.

## Problem

**Current validation files** in `pkg/invkfile/`:
- `validation_container.go` - Container-specific validation
- `validation_filesystem.go` - File path validation
- `validation_primitives.go` - Basic type validation
- `invkfile_validation_deps.go` - Dependency validation
- `invkfile_validation_struct.go` - Structure validation

**Issues**:
1. No shared interface - validators are ad-hoc functions
2. Can't easily add custom validators
3. Validation order is implicit
4. Hard to test validators in isolation

## Solution

Create a `Validator` interface with `CompositeValidator` for chaining:

### Interface Definitions

```go
// pkg/invkfile/validation.go

// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
    "io/fs"
)

// Validator validates an Invkfile and returns validation errors.
type Validator interface {
    // Name returns the validator name for error reporting.
    Name() string

    // Validate checks the invkfile and returns validation errors.
    // Validators should return all errors found, not stop at first error.
    Validate(ctx *ValidationContext, inv *Invkfile) []ValidationError
}

// ValidationContext provides context for validation.
type ValidationContext struct {
    // WorkDir is the directory containing the invkfile
    WorkDir string

    // FS allows validators to check file existence (defaults to os filesystem)
    FS fs.FS

    // Platform indicates the target platform (e.g., "linux", "darwin", "windows")
    Platform string

    // StrictMode enables additional warnings as errors
    StrictMode bool
}

// ValidationError represents a single validation error.
type ValidationError struct {
    // Validator is the name of the validator that found the error
    Validator string

    // Field is the JSON path to the problematic field
    // e.g., "cmds.build.container.image"
    Field string

    // Message describes the validation error
    Message string

    // Severity indicates if this is an error or warning
    Severity ValidationSeverity
}

// ValidationSeverity indicates the severity of a validation error.
type ValidationSeverity int

const (
    // SeverityError indicates a blocking validation error
    SeverityError ValidationSeverity = iota

    // SeverityWarning indicates a non-blocking warning
    SeverityWarning
)

func (e ValidationError) Error() string {
    return fmt.Sprintf("%s: %s: %s", e.Validator, e.Field, e.Message)
}
```

### Composite Validator

```go
// pkg/invkfile/validation_composite.go

// CompositeValidator runs multiple validators in sequence.
type CompositeValidator struct {
    validators []Validator
}

// NewCompositeValidator creates a validator that runs all provided validators.
func NewCompositeValidator(validators ...Validator) *CompositeValidator {
    return &CompositeValidator{validators: validators}
}

// Name returns a combined name for the composite.
func (c *CompositeValidator) Name() string {
    return "composite"
}

// Validate runs all validators and collects errors.
func (c *CompositeValidator) Validate(ctx *ValidationContext, inv *Invkfile) []ValidationError {
    var errors []ValidationError
    for _, v := range c.validators {
        errors = append(errors, v.Validate(ctx, inv)...)
    }
    return errors
}

// Add adds a validator to the composite.
func (c *CompositeValidator) Add(v Validator) {
    c.validators = append(c.validators, v)
}
```

### Default Validators

```go
// pkg/invkfile/validation_defaults.go

// DefaultValidators returns the standard set of validators.
func DefaultValidators() []Validator {
    return []Validator{
        &StructureValidator{},
        &DependencyValidator{},
        &ContainerValidator{},
        &FilesystemValidator{},
        &PrimitiveValidator{},
    }
}

// NewDefaultValidator creates a composite with all default validators.
func NewDefaultValidator() *CompositeValidator {
    return NewCompositeValidator(DefaultValidators()...)
}
```

### Refactored Validators

```go
// pkg/invkfile/validation_structure.go

// StructureValidator validates the overall invkfile structure.
type StructureValidator struct{}

func (v *StructureValidator) Name() string {
    return "structure"
}

func (v *StructureValidator) Validate(ctx *ValidationContext, inv *Invkfile) []ValidationError {
    var errors []ValidationError

    // Validate command names
    for name, cmd := range inv.Cmds {
        if !isValidCommandName(name) {
            errors = append(errors, ValidationError{
                Validator: v.Name(),
                Field:     fmt.Sprintf("cmds.%s", name),
                Message:   "invalid command name: must be lowercase alphanumeric with hyphens",
                Severity:  SeverityError,
            })
        }

        // Validate implementations
        if len(cmd.Impls) == 0 {
            errors = append(errors, ValidationError{
                Validator: v.Name(),
                Field:     fmt.Sprintf("cmds.%s.impls", name),
                Message:   "command must have at least one implementation",
                Severity:  SeverityError,
            })
        }
    }

    return errors
}
```

```go
// pkg/invkfile/validation_container.go

// ContainerValidator validates container-specific configuration.
type ContainerValidator struct{}

func (v *ContainerValidator) Name() string {
    return "container"
}

func (v *ContainerValidator) Validate(ctx *ValidationContext, inv *Invkfile) []ValidationError {
    var errors []ValidationError

    for cmdName, cmd := range inv.Cmds {
        for i, impl := range cmd.Impls {
            if impl.Container == nil {
                continue
            }

            field := fmt.Sprintf("cmds.%s.impls[%d].container", cmdName, i)

            // Must have image or containerfile
            if impl.Container.Image == "" && impl.Container.Containerfile == "" {
                errors = append(errors, ValidationError{
                    Validator: v.Name(),
                    Field:     field,
                    Message:   "container must specify either 'image' or 'containerfile'",
                    Severity:  SeverityError,
                })
            }

            // Warn about Alpine images (not supported)
            if strings.Contains(impl.Container.Image, "alpine") {
                errors = append(errors, ValidationError{
                    Validator: v.Name(),
                    Field:     field + ".image",
                    Message:   "Alpine images are not supported; use debian:stable-slim instead",
                    Severity:  SeverityWarning,
                })
            }
        }
    }

    return errors
}
```

### Updated Invkfile.Validate()

```go
// pkg/invkfile/invkfile.go

// ValidateOption configures validation behavior.
type ValidateOption func(*validateOptions)

type validateOptions struct {
    validators []Validator
    ctx        *ValidationContext
}

// WithValidators specifies custom validators to use.
func WithValidators(validators ...Validator) ValidateOption {
    return func(o *validateOptions) {
        o.validators = validators
    }
}

// WithValidationContext specifies the validation context.
func WithValidationContext(ctx *ValidationContext) ValidateOption {
    return func(o *validateOptions) {
        o.ctx = ctx
    }
}

// Validate checks the invkfile for errors.
func (inv *Invkfile) Validate(opts ...ValidateOption) []ValidationError {
    options := &validateOptions{
        validators: DefaultValidators(),
        ctx: &ValidationContext{
            WorkDir:  inv.Dir,
            FS:       os.DirFS(inv.Dir),
            Platform: runtime.GOOS,
        },
    }

    for _, opt := range opts {
        opt(options)
    }

    composite := NewCompositeValidator(options.validators...)
    return composite.Validate(options.ctx, inv)
}

// HasErrors returns true if any errors (not warnings) exist.
func HasErrors(errs []ValidationError) bool {
    for _, e := range errs {
        if e.Severity == SeverityError {
            return true
        }
    }
    return false
}
```

## Files to Modify

### New Files

| File | Description |
|------|-------------|
| `pkg/invkfile/validation.go` | Interface definitions |
| `pkg/invkfile/validation_composite.go` | CompositeValidator |
| `pkg/invkfile/validation_defaults.go` | Default validators list |

### Files to Refactor

| File | Changes |
|------|---------|
| `pkg/invkfile/validation_container.go` | Implement `Validator` interface |
| `pkg/invkfile/validation_filesystem.go` | Implement `Validator` interface |
| `pkg/invkfile/validation_primitives.go` | Implement `Validator` interface |
| `pkg/invkfile/invkfile_validation_deps.go` | Implement `Validator` interface |
| `pkg/invkfile/invkfile_validation_struct.go` | Implement `Validator` interface |
| `pkg/invkfile/invkfile.go` | Update `Validate()` method |

## Implementation Steps

1. [ ] Create `validation.go` with interface definitions
2. [ ] Create `validation_composite.go` with CompositeValidator
3. [ ] Create `validation_defaults.go` with DefaultValidators()
4. [ ] Refactor `validation_container.go` to implement interface
5. [ ] Refactor `validation_filesystem.go` to implement interface
6. [ ] Refactor `validation_primitives.go` to implement interface
7. [ ] Refactor `invkfile_validation_deps.go` to implement interface
8. [ ] Refactor `invkfile_validation_struct.go` to implement interface
9. [ ] Update `Invkfile.Validate()` to use composite
10. [ ] Add functional options for customization
11. [ ] Add tests for each validator
12. [ ] Add tests for composite behavior

## Acceptance Criteria

- [ ] `Validator` interface defined with clear contract
- [ ] `ValidationError` type with severity levels
- [ ] `CompositeValidator` runs all validators
- [ ] All existing validators refactored to interface
- [ ] `Validate()` method uses functional options
- [ ] Custom validators can be added via options
- [ ] Existing validation behavior preserved
- [ ] All tests pass
- [ ] `make lint` passes

## Testing

```bash
# Run validation tests
go test -v ./pkg/invkfile/... -run Validation

# Verify CLI validation still works
invowk validate invkfile.cue
```

## Example Custom Validator

```go
// Custom validator for a specific project
type NoSudoValidator struct{}

func (v *NoSudoValidator) Name() string { return "no-sudo" }

func (v *NoSudoValidator) Validate(ctx *ValidationContext, inv *Invkfile) []ValidationError {
    var errors []ValidationError
    for name, cmd := range inv.Cmds {
        for i, impl := range cmd.Impls {
            if strings.Contains(impl.Script, "sudo") {
                errors = append(errors, ValidationError{
                    Validator: v.Name(),
                    Field:     fmt.Sprintf("cmds.%s.impls[%d].script", name, i),
                    Message:   "sudo is not allowed in scripts",
                    Severity:  SeverityError,
                })
            }
        }
    }
    return errors
}

// Usage
errs := inv.Validate(WithValidators(
    append(DefaultValidators(), &NoSudoValidator{})...,
))
```

## Notes

- This is lower priority but improves extensibility
- Consider adding validator for Windows path restrictions
- Consider adding validator for security best practices
- The interface enables future plugin-based validation
