// SPDX-License-Identifier: MPL-2.0

package invowkmod

import "fmt"

// Error implements the error interface for InvalidValidationIssueError.
func (e *InvalidValidationIssueError) Error() string {
	return fmt.Sprintf("invalid validation issue: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidValidationIssue for errors.Is() compatibility.
func (e *InvalidValidationIssueError) Unwrap() error { return ErrInvalidValidationIssue }

// Validate returns nil if the ValidationIssue has valid fields, or an error
// collecting all field-level validation failures.
// Only the Type field is validated — Message and Path are display-only strings.
func (v ValidationIssue) Validate() error {
	if err := v.Type.Validate(); err != nil {
		return &InvalidValidationIssueError{FieldErrors: []error{err}}
	}
	return nil
}

// Error implements the error interface for InvalidValidationResultError.
func (e *InvalidValidationResultError) Error() string {
	return fmt.Sprintf("invalid validation result: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidValidationResult for errors.Is() compatibility.
func (e *InvalidValidationResultError) Unwrap() error { return ErrInvalidValidationResult }

// Validate returns nil if the ValidationResult has valid fields, or an error
// collecting all field-level validation failures.
// Path fields and ModuleName are validated only when non-empty (zero values are valid).
// Issues are iterated and each is validated via delegation.
func (r ValidationResult) Validate() error {
	var errs []error
	if r.ModulePath != "" {
		if err := r.ModulePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.ModuleName != "" {
		if err := r.ModuleName.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.InvowkmodPath != "" {
		if err := r.InvowkmodPath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.InvowkfilePath != "" {
		if err := r.InvowkfilePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, issue := range r.Issues {
		if err := issue.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidValidationResultError{FieldErrors: errs}
	}
	return nil
}
