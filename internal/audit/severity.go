// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// SeverityInfo indicates an informational finding with no direct risk.
	SeverityInfo Severity = iota
	// SeverityLow indicates a low-risk finding that may warrant attention.
	SeverityLow
	// SeverityMedium indicates a moderate risk that should be reviewed.
	SeverityMedium
	// SeverityHigh indicates a high-risk finding that should be addressed.
	SeverityHigh
	// SeverityCritical indicates a critical finding requiring immediate attention.
	SeverityCritical
)

// ErrInvalidSeverity is the sentinel error for unrecognized severity values.
var ErrInvalidSeverity = errors.New(invalidSeverityErrMsg)

type (
	// Severity represents the severity level of a security finding.
	// Values are ordered from least to most severe for comparison (e.g., s > SeverityMedium).
	//
	//nolint:recvcheck // Severity uses value receiver for Validate/String/MarshalJSON and pointer for UnmarshalJSON — intentional DDD pattern.
	Severity int

	// InvalidSeverityError is returned when a severity string cannot be parsed.
	InvalidSeverityError struct {
		Value string
	}
)

// String returns the lowercase string representation of the Severity.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return severityInfoStr
	case SeverityLow:
		return severityLowStr
	case SeverityMedium:
		return severityMediumStr
	case SeverityHigh:
		return severityHighStr
	case SeverityCritical:
		return severityCriticalStr
	default:
		return fmt.Sprintf("severity(%d)", int(s))
	}
}

// Validate returns nil if the Severity is one of the defined levels.
func (s Severity) Validate() error {
	switch s {
	case SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return nil
	default:
		return &InvalidSeverityError{Value: strconv.Itoa(int(s))}
	}
}

// MarshalJSON encodes the Severity as a lowercase JSON string.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON decodes a JSON string into a Severity value.
func (s *Severity) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	parsed, err := ParseSeverity(str)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// ParseSeverity converts a string to a Severity value.
func ParseSeverity(s string) (Severity, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case severityInfoStr:
		return SeverityInfo, nil
	case severityLowStr:
		return SeverityLow, nil
	case severityMediumStr:
		return SeverityMedium, nil
	case severityHighStr:
		return SeverityHigh, nil
	case severityCriticalStr:
		return SeverityCritical, nil
	default:
		return SeverityInfo, &InvalidSeverityError{Value: s}
	}
}

// Error implements the error interface.
func (e *InvalidSeverityError) Error() string {
	return fmt.Sprintf("invalid severity %q (must be info, low, medium, high, or critical)", e.Value)
}

// Unwrap returns ErrInvalidSeverity for errors.Is() compatibility.
func (e *InvalidSeverityError) Unwrap() error { return ErrInvalidSeverity }
