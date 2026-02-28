// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
)

const (
	// ConstraintOpEqual is the exact equality operator (=).
	ConstraintOpEqual ConstraintOp = "="
	// ConstraintOpCaret is the caret operator (^), allowing non-breaking changes.
	ConstraintOpCaret ConstraintOp = "^"
	// ConstraintOpTilde is the tilde operator (~), allowing patch-level changes.
	ConstraintOpTilde ConstraintOp = "~"
	// ConstraintOpGT is the greater-than operator (>).
	ConstraintOpGT ConstraintOp = ">"
	// ConstraintOpGTE is the greater-than-or-equal operator (>=).
	ConstraintOpGTE ConstraintOp = ">="
	// ConstraintOpLT is the less-than operator (<).
	ConstraintOpLT ConstraintOp = "<"
	// ConstraintOpLTE is the less-than-or-equal operator (<=).
	ConstraintOpLTE ConstraintOp = "<="
)

// ErrInvalidSemVer is the sentinel error wrapped by InvalidSemVerError.
// ErrInvalidSemVerConstraint is the sentinel error wrapped by InvalidSemVerConstraintError.
// ErrInvalidConstraintOp is the sentinel error wrapped by InvalidConstraintOpError.
var (
	ErrInvalidSemVer           = errors.New("invalid semver")
	ErrInvalidSemVerConstraint = errors.New("invalid semver constraint")
	ErrInvalidConstraintOp     = errors.New("invalid constraint operator")
)

type (
	// SemVer represents a concrete semantic version string (e.g., "1.0.0", "v2.3.4-alpha.1").
	// Validation delegates to the existing ParseVersion function in semver.go.
	SemVer string

	// InvalidSemVerError is returned when a SemVer value does not match
	// the expected semantic version format.
	InvalidSemVerError struct {
		Value SemVer
	}

	// SemVerConstraint represents a version constraint string (e.g., "^1.2.0", "~1.0.0", ">=1.0.0").
	// Validation delegates to the existing SemverResolver.ParseConstraint in semver.go.
	SemVerConstraint string

	// InvalidSemVerConstraintError is returned when a SemVerConstraint value
	// does not match the expected constraint format.
	InvalidSemVerConstraintError struct {
		Value SemVerConstraint
	}

	// ConstraintOp represents a version constraint comparison operator.
	// Valid operators are: =, ^, ~, >, >=, <, <=.
	ConstraintOp string

	// InvalidConstraintOpError is returned when a ConstraintOp value is not
	// one of the recognized operators.
	InvalidConstraintOpError struct {
		Value ConstraintOp
	}
)

// Error implements the error interface.
func (e *InvalidSemVerError) Error() string {
	return fmt.Sprintf("invalid semver %q", e.Value)
}

// Unwrap returns ErrInvalidSemVer so callers can use errors.Is for programmatic detection.
func (e *InvalidSemVerError) Unwrap() error { return ErrInvalidSemVer }

//goplint:nonzero

// Validate returns nil if the SemVer is a valid semantic version string,
// or an error describing the validation failure.
func (s SemVer) Validate() error {
	if _, err := ParseVersion(string(s)); err != nil {
		return &InvalidSemVerError{Value: s}
	}
	return nil
}

// String returns the string representation of the SemVer.
func (s SemVer) String() string { return string(s) }

// Error implements the error interface.
func (e *InvalidSemVerConstraintError) Error() string {
	return fmt.Sprintf("invalid semver constraint %q", e.Value)
}

// Unwrap returns ErrInvalidSemVerConstraint so callers can use errors.Is for programmatic detection.
func (e *InvalidSemVerConstraintError) Unwrap() error { return ErrInvalidSemVerConstraint }

// Validate returns nil if the SemVerConstraint is a valid version constraint string,
// or an error describing the validation failure.
func (s SemVerConstraint) Validate() error {
	r := &SemverResolver{}
	if _, err := r.ParseConstraint(string(s)); err != nil {
		return &InvalidSemVerConstraintError{Value: s}
	}
	return nil
}

// String returns the string representation of the SemVerConstraint.
func (s SemVerConstraint) String() string { return string(s) }

// Error implements the error interface for InvalidConstraintOpError.
func (e *InvalidConstraintOpError) Error() string {
	return fmt.Sprintf("invalid constraint operator %q (valid: =, ^, ~, >, >=, <, <=)", e.Value)
}

// Unwrap returns ErrInvalidConstraintOp for errors.Is() compatibility.
func (e *InvalidConstraintOpError) Unwrap() error { return ErrInvalidConstraintOp }

// Validate returns nil if the ConstraintOp is one of the defined operators,
// or an error describing the validation failure.
func (op ConstraintOp) Validate() error {
	switch op {
	case ConstraintOpEqual, ConstraintOpCaret, ConstraintOpTilde,
		ConstraintOpGT, ConstraintOpGTE, ConstraintOpLT, ConstraintOpLTE:
		return nil
	default:
		return &InvalidConstraintOpError{Value: op}
	}
}

// String returns the string representation of the ConstraintOp.
func (op ConstraintOp) String() string { return string(op) }
