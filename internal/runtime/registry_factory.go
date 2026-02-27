// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/sshserver"
)

const (
	// CodeContainerRuntimeInitFailed indicates the container runtime could not be initialized.
	CodeContainerRuntimeInitFailed InitDiagnosticCode = "container_runtime_init_failed"
)

// ErrInvalidInitDiagnosticCode is the sentinel error wrapped by InvalidInitDiagnosticCodeError.
var ErrInvalidInitDiagnosticCode = errors.New("invalid init diagnostic code")

type (
	// BuildRegistryOptions configures runtime registry construction.
	BuildRegistryOptions struct {
		// Config controls runtime behavior and feature flags.
		Config *config.Config
		// SSHServer is forwarded to the container runtime for host callbacks.
		SSHServer *sshserver.Server
	}

	// InitDiagnosticCode categorizes non-fatal runtime initialization diagnostics.
	// Values are string-typed so the CLI layer can cast to discovery.DiagnosticCode
	// at the package boundary (runtime cannot import discovery).
	InitDiagnosticCode string

	// InvalidInitDiagnosticCodeError is returned when an InitDiagnosticCode value
	// is not one of the defined diagnostic codes.
	InvalidInitDiagnosticCodeError struct {
		Value InitDiagnosticCode
	}

	// InitDiagnostic reports non-fatal runtime initialization details.
	InitDiagnostic struct {
		Code    InitDiagnosticCode
		Message string
		Cause   error
	}

	// RegistryBuildResult contains the built registry, cleanup hook, diagnostics,
	// and any container-runtime initialization error.
	// Registry and Cleanup are always non-nil after BuildRegistry returns.
	// Callers should defer Cleanup() after use.
	RegistryBuildResult struct {
		Registry         *Registry
		Cleanup          func()
		Diagnostics      []InitDiagnostic
		ContainerInitErr error
	}
)

// Error implements the error interface.
func (e *InvalidInitDiagnosticCodeError) Error() string {
	return fmt.Sprintf("invalid init diagnostic code %q (valid: %s)",
		e.Value, CodeContainerRuntimeInitFailed)
}

// Unwrap returns ErrInvalidInitDiagnosticCode so callers can use errors.Is for programmatic detection.
func (e *InvalidInitDiagnosticCodeError) Unwrap() error { return ErrInvalidInitDiagnosticCode }

// String returns the string representation of the InitDiagnosticCode.
func (c InitDiagnosticCode) String() string { return string(c) }

// Validate returns nil if the InitDiagnosticCode is one of the defined diagnostic codes,
// or a validation error if it is not.
func (c InitDiagnosticCode) Validate() error {
	switch c {
	case CodeContainerRuntimeInitFailed:
		return nil
	default:
		return &InvalidInitDiagnosticCodeError{Value: c}
	}
}

// BuildRegistry creates and populates the runtime registry.
// Native and virtual runtimes are always registered. Container runtime
// registration is best-effort and reported via Diagnostics/ContainerInitErr.
func BuildRegistry(opts BuildRegistryOptions) RegistryBuildResult {
	cfg := opts.Config
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	result := RegistryBuildResult{
		Registry: NewRegistry(),
		Cleanup:  func() {},
	}

	result.Registry.Register(RuntimeTypeNative, NewNativeRuntime())
	result.Registry.Register(RuntimeTypeVirtual, NewVirtualRuntime(cfg.VirtualShell.EnableUrootUtils))

	containerRT, err := NewContainerRuntime(cfg)
	if err != nil {
		result.ContainerInitErr = err
		result.Diagnostics = append(result.Diagnostics, InitDiagnostic{
			Code:    CodeContainerRuntimeInitFailed,
			Message: fmt.Sprintf("container runtime unavailable: %v", err),
			Cause:   err,
		})
	} else {
		if opts.SSHServer != nil && opts.SSHServer.IsRunning() {
			containerRT.SetSSHServer(opts.SSHServer)
		}
		result.Registry.Register(RuntimeTypeContainer, containerRT)
		result.Cleanup = func() {
			if closeErr := containerRT.Close(); closeErr != nil {
				slog.Warn("container runtime cleanup failed", "error", closeErr)
			}
		}
	}

	return result
}
