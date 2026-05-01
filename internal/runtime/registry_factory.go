// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
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
		// HostCallbacks is forwarded to the container runtime for host callbacks.
		HostCallbacks HostCallbackServer
		// SelectedRuntime limits outside-device initialization to the runtime
		// needed for the current execution. The zero value preserves full
		// registry construction for callers that are not dispatching a command.
		SelectedRuntime invowkfile.RuntimeMode
		// ContainerRuntimeFactory constructs the container runtime. Tests use
		// this to prove non-container dispatch does not probe container engines.
		ContainerRuntimeFactory func(*config.Config) (*ContainerRuntime, error)
		// VirtualInteractiveCommandFactory creates the subprocess used by virtual
		// interactive execution. The CLI/composition adapter owns hidden argv.
		VirtualInteractiveCommandFactory VirtualInteractiveCommandFactory
	}

	// HostCallbackServer provides scoped host callback credentials to runtimes.
	HostCallbackServer interface {
		IsRunning() bool
		GetConnectionInfo(commandID HostCallbackCommandID) (*HostCallbackConnectionInfo, error)
		RevokeToken(token HostCallbackToken)
	}

	// HostCallbackCommandID identifies one execution-scoped callback credential.
	HostCallbackCommandID string

	// HostCallbackHost is the host address used by container runtimes to reach
	// a scoped host callback service.
	HostCallbackHost string

	// HostCallbackToken is the credential token used by container runtimes to
	// authenticate to a scoped host callback service.
	HostCallbackToken string

	// HostCallbackUser is the callback username passed to container runtimes.
	HostCallbackUser string

	// HostCallbackConnectionInfo contains transport-neutral host callback
	// coordinates. The runtime package owns this port type so it does not depend
	// on the concrete SSH server adapter.
	HostCallbackConnectionInfo struct {
		Host  HostCallbackHost
		Port  types.ListenPort
		Token HostCallbackToken
		User  HostCallbackUser
	}

	//goplint:constant-only
	//
	// InitDiagnosticCode categorizes non-fatal runtime initialization diagnostics.
	// Values are string-typed so the CLI layer can cast to discovery.DiagnosticCode
	// at the package boundary (runtime cannot import discovery).
	InitDiagnosticCode string

	// InvalidInitDiagnosticCodeError is returned when an InitDiagnosticCode value
	// is not one of the defined diagnostic codes.
	InvalidInitDiagnosticCodeError struct {
		Value InitDiagnosticCode
	}

	//goplint:validate-all
	//
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

// String returns the string representation of the host callback command ID.
func (id HostCallbackCommandID) String() string { return string(id) }

// Validate returns an error if the host callback command ID is empty.
func (id HostCallbackCommandID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return errors.New("host callback command ID must not be empty")
	}
	return nil
}

// String returns the string representation of the host callback host.
func (h HostCallbackHost) String() string { return string(h) }

// Validate returns an error if the host callback host is empty.
func (h HostCallbackHost) Validate() error {
	if strings.TrimSpace(string(h)) == "" {
		return errors.New("host callback host must not be empty")
	}
	return nil
}

// String returns the string representation of the host callback token.
func (t HostCallbackToken) String() string { return string(t) }

// Validate returns an error if the host callback token is empty.
func (t HostCallbackToken) Validate() error {
	if strings.TrimSpace(string(t)) == "" {
		return errors.New("host callback token must not be empty")
	}
	return nil
}

// String returns the string representation of the host callback user.
func (u HostCallbackUser) String() string { return string(u) }

// Validate returns an error if the host callback user is empty.
func (u HostCallbackUser) Validate() error {
	if strings.TrimSpace(string(u)) == "" {
		return errors.New("host callback user must not be empty")
	}
	return nil
}

// Validate returns nil when all host callback connection fields are valid.
func (info HostCallbackConnectionInfo) Validate() error {
	return errors.Join(
		info.Host.Validate(),
		info.Port.Validate(),
		info.Token.Validate(),
		info.User.Validate(),
	)
}

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
		Cleanup: func() {
			// No-op when container runtime is unavailable.
		},
	}

	result.Registry.Register(RuntimeTypeNative, NewNativeRuntime())
	result.Registry.Register(RuntimeTypeVirtual, NewVirtualRuntime(
		cfg.VirtualShell.EnableUrootUtils,
		WithInteractiveCommandFactory(opts.VirtualInteractiveCommandFactory),
	))

	if !shouldInitializeContainerRuntime(opts.SelectedRuntime) {
		return result
	}

	containerRT, err := buildContainerRuntime(cfg, opts.ContainerRuntimeFactory)
	if err != nil {
		result.ContainerInitErr = err
		result.Diagnostics = append(result.Diagnostics, InitDiagnostic{
			Code:    CodeContainerRuntimeInitFailed,
			Message: fmt.Sprintf("container runtime unavailable: %v", err),
			Cause:   err,
		})
	} else {
		if opts.HostCallbacks != nil && opts.HostCallbacks.IsRunning() {
			containerRT.SetHostCallbacks(opts.HostCallbacks)
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

func shouldInitializeContainerRuntime(selectedRuntime invowkfile.RuntimeMode) bool {
	return selectedRuntime == "" || selectedRuntime == invowkfile.RuntimeContainer
}

func buildContainerRuntime(cfg *config.Config, factory func(*config.Config) (*ContainerRuntime, error)) (*ContainerRuntime, error) {
	if factory == nil {
		return NewContainerRuntime(cfg)
	}
	return factory(cfg)
}
