// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
)

// ErrInvalidSemVer is the sentinel error wrapped by InvalidSemVerError.
// ErrInvalidSemVerConstraint is the sentinel error wrapped by InvalidSemVerConstraintError.
var (
	ErrInvalidSemVer           = errors.New("invalid semver")
	ErrInvalidSemVerConstraint = errors.New("invalid semver constraint")
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
)

// Error implements the error interface.
func (e *InvalidSemVerError) Error() string {
	return fmt.Sprintf("invalid semver %q", e.Value)
}

// Unwrap returns ErrInvalidSemVer so callers can use errors.Is for programmatic detection.
func (e *InvalidSemVerError) Unwrap() error { return ErrInvalidSemVer }

// IsValid returns whether the SemVer is a valid semantic version string,
// and a list of validation errors if it is not.
func (s SemVer) IsValid() (bool, []error) {
	if _, err := ParseVersion(string(s)); err != nil {
		return false, []error{&InvalidSemVerError{Value: s}}
	}
	return true, nil
}

// String returns the string representation of the SemVer.
func (s SemVer) String() string { return string(s) }

// Error implements the error interface.
func (e *InvalidSemVerConstraintError) Error() string {
	return fmt.Sprintf("invalid semver constraint %q", e.Value)
}

// Unwrap returns ErrInvalidSemVerConstraint so callers can use errors.Is for programmatic detection.
func (e *InvalidSemVerConstraintError) Unwrap() error { return ErrInvalidSemVerConstraint }

// IsValid returns whether the SemVerConstraint is a valid version constraint string,
// and a list of validation errors if it is not.
func (s SemVerConstraint) IsValid() (bool, []error) {
	r := &SemverResolver{}
	if _, err := r.ParseConstraint(string(s)); err != nil {
		return false, []error{&InvalidSemVerConstraintError{Value: s}}
	}
	return true, nil
}

// String returns the string representation of the SemVerConstraint.
func (s SemVerConstraint) String() string { return string(s) }
