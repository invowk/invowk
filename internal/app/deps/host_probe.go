// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrHostProbeRequired is returned when host dependencies must be evaluated but
	// the application layer did not inject an infrastructure host probe.
	ErrHostProbeRequired = errors.New("host dependency probe is required")

	// ErrRuntimeDependencyProbeRequired is returned when runtime dependencies must
	// be evaluated but the application layer did not inject a runtime probe.
	ErrRuntimeDependencyProbeRequired = errors.New("runtime dependency probe is required")
)

type (
	// HostProbe performs host-device checks for dependency validation.
	HostProbe interface {
		CheckTool(invowkfile.BinaryName) error
		CheckFilepath(displayPath, resolvedPath types.FilesystemPath, fp invowkfile.FilepathDependency) error
		RunCustomCheck(ctx context.Context, check invowkfile.CustomCheck) (CustomCheckResult, error)
	}

	// CommandScopeLockProvider loads lock-file state for command-scope policy.
	CommandScopeLockProvider interface {
		LoadCommandScopeLock(inv *invowkfile.Invowkfile) (*invowkmod.LockFile, error)
	}

	// RuntimeDependencyProbe performs selected-runtime checks for dependency validation.
	RuntimeDependencyProbe interface {
		CheckTool(tool invowkfile.BinaryName) error
		CheckFilepath(fp invowkfile.FilepathDependency) error
		CheckEnvVar(envVar invowkfile.EnvVarCheck) error
		CheckCapability(capability invowkfile.CapabilityName) error
		CheckCommand(command invowkfile.CommandName) error
		RunCustomCheck(check invowkfile.CustomCheck) (CustomCheckResult, error)
	}

	// CustomCheckResult is the process result data needed by dependency policy.
	CustomCheckResult struct {
		output   CustomCheckOutput
		exitCode types.ExitCode
	}

	// CustomCheckOutput is the combined stdout/stderr text from a custom check.
	CustomCheckOutput string
)

// NewCustomCheckResult creates a validated custom-check result.
func NewCustomCheckResult(output CustomCheckOutput, exitCode types.ExitCode) (CustomCheckResult, error) {
	result := CustomCheckResult{
		output:   output,
		exitCode: exitCode,
	}
	if err := result.Validate(); err != nil {
		return CustomCheckResult{}, err
	}
	return result, nil
}

// Output returns the combined stdout/stderr text.
func (r CustomCheckResult) Output() CustomCheckOutput { return r.output }

// ExitCode returns the process exit code.
func (r CustomCheckResult) ExitCode() types.ExitCode { return r.exitCode }

// String returns the custom-check output text.
func (o CustomCheckOutput) String() string { return string(o) }

// Validate returns nil because custom-check output is free-form process text.
func (o CustomCheckOutput) Validate() error { return nil }

// Validate returns nil when the custom-check result is structurally valid.
func (r CustomCheckResult) Validate() error {
	var errs []error
	if err := r.output.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := r.exitCode.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
