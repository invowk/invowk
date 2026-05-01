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
		RunCustomCheck(ctx context.Context, check invowkfile.CustomCheck) error
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
		RunCustomCheck(check invowkfile.CustomCheck) error
	}
)
