// SPDX-License-Identifier: MPL-2.0

// Package tuiwire defines the shared wire vocabulary for delegated TUI
// components.
package tuiwire

import (
	"errors"
	"fmt"
)

// Environment variable names for TUI server communication.
const (
	// EnvTUIAddr is the environment variable containing the TUI server address.
	EnvTUIAddr = "INVOWK_TUI_ADDR"

	// EnvTUIToken is the environment variable containing the authentication token.
	//nolint:gosec // G101: This is an env var name, not a hardcoded credential
	EnvTUIToken = "INVOWK_TUI_TOKEN"

	// ComponentInput represents the text input component.
	ComponentInput Component = "input"
	// ComponentConfirm represents the yes/no confirmation component.
	ComponentConfirm Component = "confirm"
	// ComponentChoose represents the single/multi-select component.
	ComponentChoose Component = "choose"
	// ComponentFilter represents the filterable list component.
	ComponentFilter Component = "filter"
	// ComponentFile represents the file picker component.
	ComponentFile Component = "file"
	// ComponentWrite represents the styled text output component.
	ComponentWrite Component = "write"
	// ComponentTextArea represents the multi-line text input component.
	ComponentTextArea Component = "textarea"
	// ComponentSpin represents the spinner/loading component.
	ComponentSpin Component = "spin"
	// ComponentPager represents the scrollable text viewer component.
	ComponentPager Component = "pager"
	// ComponentTable represents the table selection component.
	ComponentTable Component = "table"
)

// ErrInvalidComponent is returned when a Component value is not one of the defined types.
var ErrInvalidComponent = errors.New("invalid component")

type (
	// Component represents a delegated TUI component type.
	Component string

	// InvalidComponentError is returned when a Component value is not recognized.
	// It wraps ErrInvalidComponent for errors.Is() compatibility.
	InvalidComponentError struct {
		Value Component
	}
)

// Error implements the error interface for InvalidComponentError.
func (e *InvalidComponentError) Error() string {
	return fmt.Sprintf("invalid component %q (valid: input, confirm, choose, filter, file, write, textarea, spin, pager, table)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidComponentError) Unwrap() error {
	return ErrInvalidComponent
}

// Validate returns nil if the Component is one of the defined component types,
// or an error wrapping ErrInvalidComponent if it is not.
func (c Component) Validate() error {
	switch c {
	case ComponentInput, ComponentConfirm, ComponentChoose, ComponentFilter,
		ComponentFile, ComponentWrite, ComponentTextArea, ComponentSpin,
		ComponentPager, ComponentTable:
		return nil
	default:
		return &InvalidComponentError{Value: c}
	}
}

// String returns the string representation of the Component.
func (c Component) String() string { return string(c) }
