// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// CodeContainerRuntimeInitFailed indicates the container runtime could not be initialized.
	CodeContainerRuntimeInitFailed InitDiagnosticCode = "container_runtime_init_failed"
	// CodeContainerProvisioningFailed indicates non-strict container provisioning failed.
	CodeContainerProvisioningFailed InitDiagnosticCode = "container_provisioning_failed"
	// CodeContainerProvisioningWarning indicates container provisioning completed with a warning.
	CodeContainerProvisioningWarning InitDiagnosticCode = "container_provisioning_warning"
)

// ErrInvalidInitDiagnosticCode is the sentinel error wrapped by InvalidInitDiagnosticCodeError.
var ErrInvalidInitDiagnosticCode = errors.New("invalid init diagnostic code")

type (
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
		e.Value, strings.Join([]string{
			CodeContainerRuntimeInitFailed.String(),
			CodeContainerProvisioningFailed.String(),
			CodeContainerProvisioningWarning.String(),
		}, ", "))
}

// Unwrap returns ErrInvalidInitDiagnosticCode so callers can use errors.Is for programmatic detection.
func (e *InvalidInitDiagnosticCodeError) Unwrap() error { return ErrInvalidInitDiagnosticCode }

// String returns the string representation of the InitDiagnosticCode.
func (c InitDiagnosticCode) String() string { return string(c) }

// Validate returns nil if the InitDiagnosticCode is one of the defined diagnostic codes,
// or a validation error if it is not.
func (c InitDiagnosticCode) Validate() error {
	switch c {
	case CodeContainerRuntimeInitFailed,
		CodeContainerProvisioningFailed,
		CodeContainerProvisioningWarning:
		return nil
	default:
		return &InvalidInitDiagnosticCodeError{Value: c}
	}
}
