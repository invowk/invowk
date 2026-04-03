// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	// ArgErrMissingRequired indicates missing required arguments.
	ArgErrMissingRequired ArgErrType = iota
	// ArgErrTooMany indicates too many arguments were provided.
	ArgErrTooMany
	// ArgErrInvalidValue indicates an argument value failed validation.
	ArgErrInvalidValue
)

var (
	// ErrInvalidArgErrType is the sentinel error wrapped by InvalidArgErrTypeError.
	// The name follows the DDD Validate() pattern: Err + Invalid + <TypeName>.
	ErrInvalidArgErrType = errors.New("invalid argument error type") //nolint:errname // follows DDD pattern: Err+Invalid+TypeName

	// ErrInvalidDependencyMessage is the sentinel error wrapped by InvalidDependencyMessageError.
	// The name follows the DDD Validate() pattern: Err + Invalid + <TypeName>.
	ErrInvalidDependencyMessage = errors.New("invalid dependency message")

	// ErrContainerRuntimeNotAvailable is returned when a container runtime is required
	// but not registered in the runtime registry (e.g., no Docker/Podman available).
	ErrContainerRuntimeNotAvailable = errors.New("container runtime not available")

	// ErrNoPathAlternatives is returned when a filepath dependency has an empty
	// alternatives list. Both host and container filepath validators use this sentinel.
	ErrNoPathAlternatives = errors.New("at least one path must be provided in alternatives")

	// ErrContainerEngineFailure is returned when a container validation probe exits
	// with a transient exit code (125/126), indicating the container engine itself
	// failed rather than the check script.
	ErrContainerEngineFailure = errors.New("container engine failure")

	// ErrContainerValidationFailed is returned when a container validation probe fails
	// due to an infrastructure error (non-ExitError), as opposed to a domain-level check
	// failure indicated by exit code.
	ErrContainerValidationFailed = errors.New("container validation failed")

	// ErrFlagValidationFailed is returned when one or more flag values fail validation
	// (missing required flags or values not matching type/regex constraints).
	ErrFlagValidationFailed = errors.New("flag validation failed")

	// ErrContainerEnvVarNotSet is returned when a required environment variable
	// is not set inside the container environment.
	ErrContainerEnvVarNotSet = errors.New("not set in container environment")

	// ErrContainerCommandNotFound is returned when a required command
	// is not found inside the container.
	ErrContainerCommandNotFound = errors.New("not found in container")

	// ErrPathNotExists is returned when a required filepath does not exist
	// on the host filesystem.
	ErrPathNotExists = errors.New("path does not exist")

	// ErrDependencyDiscoveryFailed is returned when the discovery pipeline fails
	// while resolving command dependencies for validation.
	ErrDependencyDiscoveryFailed = errors.New("failed to discover commands for dependency validation")
)

type (
	// CommandSetProvider discovers commands for dependency validation.
	// This is a minimal local interface -- cmd's DiscoveryService satisfies it.
	CommandSetProvider interface {
		DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error)
	}

	// DependencyMessage is a pre-formatted dependency validation message
	// used in DependencyError fields. Each message describes a single
	// unsatisfied dependency (e.g., "  - kubectl - not found in PATH").
	DependencyMessage string

	// InvalidDependencyMessageError is returned when a DependencyMessage value
	// fails validation (empty string).
	InvalidDependencyMessageError struct {
		Value DependencyMessage
	}

	// DependencyError represents unsatisfied dependencies.
	DependencyError struct {
		CommandName         invowkfile.CommandName
		MissingTools        []DependencyMessage
		MissingCommands     []DependencyMessage
		MissingFilepaths    []DependencyMessage
		MissingCapabilities []DependencyMessage
		FailedCustomChecks  []DependencyMessage
		MissingEnvVars      []DependencyMessage
		// ForbiddenCommands are commands that exist in discovery but are in a module
		// outside the caller's CommandScope (not a direct dependency).
		ForbiddenCommands []DependencyMessage
	}

	//goplint:constant-only
	//
	// ArgErrType represents the type of argument validation error.
	ArgErrType int

	// InvalidArgErrTypeError is returned when an ArgErrType value is not
	// one of the defined argument error types.
	InvalidArgErrTypeError struct {
		Value ArgErrType
	}

	// ArgumentValidationError represents an argument validation failure.
	ArgumentValidationError struct {
		Type         ArgErrType
		CommandName  invowkfile.CommandName
		ArgDefs      []invowkfile.Argument
		ProvidedArgs []string
		MinArgs      int
		MaxArgs      int
		InvalidArg   invowkfile.ArgumentName
		InvalidValue string
		ValueError   error
	}
)

// Error implements the error interface.
func (e *InvalidArgErrTypeError) Error() string {
	return fmt.Sprintf("invalid argument error type %d (valid: 0=missing_required, 1=too_many, 2=invalid_value)", e.Value)
}

// Unwrap returns ErrInvalidArgErrType so callers can use errors.Is for programmatic detection.
func (e *InvalidArgErrTypeError) Unwrap() error { return ErrInvalidArgErrType }

// String returns the human-readable name of the ArgErrType.
func (t ArgErrType) String() string {
	switch t {
	case ArgErrMissingRequired:
		return "missing_required"
	case ArgErrTooMany:
		return "too_many"
	case ArgErrInvalidValue:
		return "invalid_value"
	default:
		return fmt.Sprintf("unknown(%d)", int(t))
	}
}

// Validate returns nil if the ArgErrType is one of the defined argument error types,
// or a validation error if it is not.
func (t ArgErrType) Validate() error {
	switch t {
	case ArgErrMissingRequired, ArgErrTooMany, ArgErrInvalidValue:
		return nil
	default:
		return &InvalidArgErrTypeError{Value: t}
	}
}

// Validate returns nil if the DependencyMessage is non-empty and non-whitespace,
// or a validation error if it is not.
func (m DependencyMessage) Validate() error {
	if strings.TrimSpace(string(m)) == "" {
		return &InvalidDependencyMessageError{Value: m}
	}
	return nil
}

// String returns the string representation of the DependencyMessage.
func (m DependencyMessage) String() string {
	return string(m)
}

// Error implements the error interface for InvalidDependencyMessageError.
func (e *InvalidDependencyMessageError) Error() string {
	return fmt.Sprintf("invalid dependency message: %q", e.Value)
}

// Unwrap returns ErrInvalidDependencyMessage so callers can use errors.Is for programmatic detection.
func (e *InvalidDependencyMessageError) Unwrap() error { return ErrInvalidDependencyMessage }

// Error implements the error interface for DependencyError.
func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependencies not satisfied for command '%s'", e.CommandName)
}

// Error implements the error interface for ArgumentValidationError.
func (e *ArgumentValidationError) Error() string {
	switch e.Type {
	case ArgErrMissingRequired:
		return fmt.Sprintf("missing required arguments for command '%s': expected at least %d, got %d", e.CommandName, e.MinArgs, len(e.ProvidedArgs))
	case ArgErrTooMany:
		return fmt.Sprintf("too many arguments for command '%s': expected at most %d, got %d", e.CommandName, e.MaxArgs, len(e.ProvidedArgs))
	case ArgErrInvalidValue:
		return fmt.Sprintf("invalid value for argument '%s': %v", e.InvalidArg, e.ValueError)
	default:
		return fmt.Sprintf("argument validation failed for command '%s'", e.CommandName)
	}
}
