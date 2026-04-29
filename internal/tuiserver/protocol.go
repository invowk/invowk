// SPDX-License-Identifier: MPL-2.0

package tuiserver

import "github.com/invowk/invowk/internal/tuiwire"

// Environment variable names for TUI server communication.
const (
	// EnvTUIAddr is the environment variable containing the TUI server address.
	// Example: "http://127.0.0.1:54321"
	EnvTUIAddr = tuiwire.EnvTUIAddr

	// EnvTUIToken is the environment variable containing the authentication token.
	EnvTUIToken = tuiwire.EnvTUIToken

	// TUI component type constants for the protocol.
	ComponentInput    Component = tuiwire.ComponentInput
	ComponentConfirm  Component = tuiwire.ComponentConfirm
	ComponentChoose   Component = tuiwire.ComponentChoose
	ComponentFilter   Component = tuiwire.ComponentFilter
	ComponentFile     Component = tuiwire.ComponentFile
	ComponentWrite    Component = tuiwire.ComponentWrite
	ComponentTextArea Component = tuiwire.ComponentTextArea
	ComponentSpin     Component = tuiwire.ComponentSpin
	ComponentPager    Component = tuiwire.ComponentPager
	ComponentTable    Component = tuiwire.ComponentTable
)

// ErrInvalidComponent is returned when a Component value is not one of the defined types.
var ErrInvalidComponent = tuiwire.ErrInvalidComponent

type (
	// Component represents a TUI component type.
	Component = tuiwire.Component
	// InvalidComponentError is returned when a component value is not recognized.
	InvalidComponentError = tuiwire.InvalidComponentError
	// Request is the common wrapper for all TUI requests.
	Request = tuiwire.Request
	// Response is the common wrapper for all TUI responses.
	Response = tuiwire.Response
	// InputRequest contains options for the input component.
	InputRequest = tuiwire.InputRequest
	// InputResult contains the result of an input prompt.
	InputResult = tuiwire.InputResult
	// ConfirmRequest contains options for the confirm component.
	ConfirmRequest = tuiwire.ConfirmRequest
	// ConfirmResult contains the result of a confirm prompt.
	ConfirmResult = tuiwire.ConfirmResult
	// ChooseRequest contains options for the choose component.
	ChooseRequest = tuiwire.ChooseRequest
	// ChooseResult contains the result of a choose prompt.
	ChooseResult = tuiwire.ChooseResult
	// FilterRequest contains options for the filter component.
	FilterRequest = tuiwire.FilterRequest
	// FilterResult contains the result of a filter prompt.
	FilterResult = tuiwire.FilterResult
	// FileRequest contains options for the file picker component.
	FileRequest = tuiwire.FileRequest
	// FileResult contains the result of a file picker.
	FileResult = tuiwire.FileResult
	// WriteRequest contains options for the write component.
	WriteRequest = tuiwire.WriteRequest
	// WriteResult is empty because write does not return a value.
	WriteResult = tuiwire.WriteResult
	// TextAreaRequest contains options for the textarea component.
	TextAreaRequest = tuiwire.TextAreaRequest
	// TextAreaResult contains the result of a textarea prompt.
	TextAreaResult = tuiwire.TextAreaResult
	// SpinRequest contains options for the spin component.
	SpinRequest = tuiwire.SpinRequest
	// SpinResult contains the result of a spin operation.
	SpinResult = tuiwire.SpinResult
	// PagerRequest contains options for the pager component.
	PagerRequest = tuiwire.PagerRequest
	// PagerResult is empty because pager does not return a value.
	PagerResult = tuiwire.PagerResult
	// TableRequest contains options for the table component.
	TableRequest = tuiwire.TableRequest
	// TableResult contains the result of a table selection.
	TableResult = tuiwire.TableResult
)
