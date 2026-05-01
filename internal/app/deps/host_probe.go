// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"

	"github.com/invowk/invowk/internal/runtime"
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
		CheckTool(ctx *runtime.ExecutionContext, tool invowkfile.BinaryName) error
		CheckFilepath(ctx *runtime.ExecutionContext, fp invowkfile.FilepathDependency) error
		CheckEnvVar(ctx *runtime.ExecutionContext, envVar invowkfile.EnvVarCheck) error
		CheckCapability(ctx *runtime.ExecutionContext, capability invowkfile.CapabilityName) error
		CheckCommand(ctx *runtime.ExecutionContext, command invowkfile.CommandName) error
		RunCustomCheck(ctx *runtime.ExecutionContext, check invowkfile.CustomCheck) error
	}
)
