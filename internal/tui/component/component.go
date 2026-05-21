// SPDX-License-Identifier: MPL-2.0

// Package component defines the canonical delegated TUI component vocabulary.
package component

import (
	"errors"
	"fmt"
)

const (
	// TypeInput represents the text input component.
	TypeInput Type = "input"
	// TypeConfirm represents the yes/no confirmation component.
	TypeConfirm Type = "confirm"
	// TypeChoose represents the single/multi-select component.
	TypeChoose Type = "choose"
	// TypeFilter represents the filterable list component.
	TypeFilter Type = "filter"
	// TypeFile represents the file picker component.
	TypeFile Type = "file"
	// TypeWrite represents the styled text output component.
	TypeWrite Type = "write"
	// TypeTextArea represents the multi-line text input component.
	TypeTextArea Type = "textarea"
	// TypeSpin represents the spinner/loading component.
	TypeSpin Type = "spin"
	// TypePager represents the scrollable text viewer component.
	TypePager Type = "pager"
	// TypeTable represents the table selection component.
	TypeTable Type = "table"
)

var (
	// ErrInvalidType is returned when a Type value is not one of the defined values.
	ErrInvalidType = errors.New("invalid component type")
	// ErrCancelled is returned when a user cancels an interactive TUI component.
	ErrCancelled = errors.New("user cancelled")
)

type (
	// Type represents a delegated TUI component type.
	Type string

	// InvalidTypeError is returned when a Type value is not recognized.
	InvalidTypeError struct {
		Value Type
	}
)

// Error implements the error interface for InvalidTypeError.
func (e *InvalidTypeError) Error() string {
	return fmt.Sprintf("invalid component type %q (valid: input, confirm, choose, filter, file, write, textarea, spin, pager, table)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidTypeError) Unwrap() error {
	return ErrInvalidType
}

// String returns the string representation of the Type.
func (t Type) String() string { return string(t) }

// Validate returns nil if the Type is one of the defined component types.
func (t Type) Validate() error {
	switch t {
	case TypeInput,
		TypeConfirm,
		TypeChoose,
		TypeFilter,
		TypeFile,
		TypeWrite,
		TypeTextArea,
		TypeSpin,
		TypePager,
		TypeTable:
		return nil
	default:
		return &InvalidTypeError{Value: t}
	}
}
