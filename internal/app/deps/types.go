// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// ArgErrMissingRequired indicates missing required arguments.
	ArgErrMissingRequired ArgErrType = 0
	// ArgErrTooMany indicates too many arguments were provided.
	ArgErrTooMany ArgErrType = 1
	// ArgErrInvalidValue indicates an argument value failed validation.
	ArgErrInvalidValue ArgErrType = 2

	// DependencyFailureTool means a host/runtime tool dependency is missing.
	DependencyFailureTool DependencyFailureKind = "tool"
	// DependencyFailureCommand means a command dependency is missing.
	DependencyFailureCommand DependencyFailureKind = "command"
	// DependencyFailureFilepath means a filepath dependency is missing.
	DependencyFailureFilepath DependencyFailureKind = "filepath"
	// DependencyFailureCapability means a capability dependency is missing.
	DependencyFailureCapability DependencyFailureKind = "capability"
	// DependencyFailureCustomCheck means a custom check dependency failed.
	DependencyFailureCustomCheck DependencyFailureKind = "custom_check"
	// DependencyFailureEnvVar means an environment variable dependency is missing.
	DependencyFailureEnvVar DependencyFailureKind = "env_var"
	// DependencyFailureForbiddenCommand means a command dependency is outside scope.
	DependencyFailureForbiddenCommand DependencyFailureKind = "forbidden_command"
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

	// ErrCommandScopeLockLoadFailed is returned when command scope validation
	// cannot load the module lock file needed for direct-dependency identity.
	ErrCommandScopeLockLoadFailed = errors.New("command scope lock load failed")
)

type (
	// CommandSetProvider discovers commands for dependency validation.
	// This is a minimal local interface -- cmd's DiscoveryService satisfies it.
	CommandSetProvider interface {
		DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error)
	}

	// DependencyMessage is a plain dependency validation detail used in
	// DependencyError fields. Adapters decide how to render bullets and spacing.
	DependencyMessage string

	// DependencyFailureKind categorizes a dependency validation failure.
	DependencyFailureKind string

	// InvalidDependencyMessageError is returned when a DependencyMessage value
	// fails validation (empty string).
	InvalidDependencyMessageError struct {
		Value DependencyMessage
	}

	// DependencyFailure is a structured dependency validation failure.
	DependencyFailure struct {
		kind   DependencyFailureKind
		detail DependencyMessage
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
		// StructuredFailures is the adapter-facing source of truth for dependency
		// rendering. Legacy categorized slices are retained for compatibility
		// while validators are migrated to populate structured records directly.
		StructuredFailures []DependencyFailure
	}

	// FlagValidationError represents runtime validation failures for dynamic
	// command flags.
	FlagValidationError struct {
		CommandName invowkfile.CommandName
		Failures    []DependencyMessage
	}

	// CommandScopeLockError reports lock-file load failures that affect
	// depends_on.cmds scope enforcement.
	CommandScopeLockError struct {
		Path types.FilesystemPath
		Err  error
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

// Error implements the error interface for FlagValidationError.
func (e *FlagValidationError) Error() string {
	if e == nil {
		return ErrFlagValidationFailed.Error()
	}
	failures := make([]string, 0, len(e.Failures))
	for i := range e.Failures {
		failures = append(failures, e.Failures[i].String())
	}
	return fmt.Sprintf("%s for command '%s':\n  %s", ErrFlagValidationFailed, e.CommandName, strings.Join(failures, "\n  "))
}

// Unwrap returns ErrFlagValidationFailed for errors.Is compatibility.
func (e *FlagValidationError) Unwrap() error { return ErrFlagValidationFailed }

// Error implements the error interface for CommandScopeLockError.
func (e *CommandScopeLockError) Error() string {
	if e == nil || e.Err == nil {
		return ErrCommandScopeLockLoadFailed.Error()
	}
	return fmt.Sprintf("load command scope lock %q: %v", e.Path, e.Err)
}

// Unwrap returns ErrCommandScopeLockLoadFailed and the underlying cause.
func (e *CommandScopeLockError) Unwrap() error {
	if e == nil {
		return ErrCommandScopeLockLoadFailed
	}
	return errors.Join(ErrCommandScopeLockLoadFailed, e.Err)
}

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

// String returns the string representation of the DependencyFailureKind.
func (k DependencyFailureKind) String() string { return string(k) }

// Validate returns nil if the dependency failure kind is known.
func (k DependencyFailureKind) Validate() error {
	switch k {
	case DependencyFailureTool,
		DependencyFailureCommand,
		DependencyFailureFilepath,
		DependencyFailureCapability,
		DependencyFailureCustomCheck,
		DependencyFailureEnvVar,
		DependencyFailureForbiddenCommand:
		return nil
	default:
		return fmt.Errorf("invalid dependency failure kind %q", k)
	}
}

// NewDependencyFailure creates a validated dependency failure.
func NewDependencyFailure(kind DependencyFailureKind, detail DependencyMessage) (DependencyFailure, error) {
	failure := DependencyFailure{kind: kind, detail: detail}
	if err := failure.Validate(); err != nil {
		return DependencyFailure{}, err
	}
	return failure, nil
}

// Kind returns the dependency failure category.
func (f DependencyFailure) Kind() DependencyFailureKind { return f.kind }

// Detail returns the dependency failure detail.
func (f DependencyFailure) Detail() DependencyMessage { return f.detail }

// Validate returns nil when all dependency failure fields are valid.
func (f DependencyFailure) Validate() error {
	return errors.Join(f.kind.Validate(), f.detail.Validate())
}

// Failures returns categorized dependency failures without requiring adapters to
// infer failure kinds from rendered section text.
func (e *DependencyError) Failures() []DependencyFailure {
	if e == nil {
		return nil
	}
	if len(e.StructuredFailures) > 0 {
		return slices.Clone(e.StructuredFailures)
	}
	var failures []DependencyFailure
	appendDependencyFailures(&failures, DependencyFailureTool, e.MissingTools)
	appendDependencyFailures(&failures, DependencyFailureCommand, e.MissingCommands)
	appendDependencyFailures(&failures, DependencyFailureFilepath, e.MissingFilepaths)
	appendDependencyFailures(&failures, DependencyFailureCapability, e.MissingCapabilities)
	appendDependencyFailures(&failures, DependencyFailureCustomCheck, e.FailedCustomChecks)
	appendDependencyFailures(&failures, DependencyFailureEnvVar, e.MissingEnvVars)
	appendDependencyFailures(&failures, DependencyFailureForbiddenCommand, e.ForbiddenCommands)
	return failures
}

func appendDependencyFailures(dst *[]DependencyFailure, kind DependencyFailureKind, details []DependencyMessage) {
	for _, detail := range details {
		failure, err := NewDependencyFailure(kind, detail)
		if err != nil {
			continue
		}
		*dst = append(*dst, failure)
	}
}

func dependencyFailures(kind DependencyFailureKind, details []DependencyMessage) []DependencyFailure {
	failures := make([]DependencyFailure, 0, len(details))
	appendDependencyFailures(&failures, kind, details)
	return failures
}

// dependencyMessageFromDetail constructs a plain dependency validation detail.
//
//goplint:ignore -- dependency diagnostics normalize free-form details from typed errors; empty messages are rejected at aggregate construction.
func dependencyMessageFromDetail(detail string) DependencyMessage {
	return DependencyMessage(normalizeDependencyMessage(detail))
}

//goplint:ignore -- helper strips presentation bullets from legacy free-form diagnostics.
func normalizeDependencyMessage(detail string) string {
	text := strings.TrimSpace(detail)
	text = strings.TrimSpace(strings.TrimPrefix(text, "•"))
	text = strings.TrimSpace(strings.TrimPrefix(text, "-"))
	return text
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
