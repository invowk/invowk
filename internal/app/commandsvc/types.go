// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// ErrorKindCommandNotFound means the requested command or source was not found.
	ErrorKindCommandNotFound ErrorKind = "command_not_found"
	// ErrorKindCommandAmbiguous means an unqualified command exists in multiple sources.
	ErrorKindCommandAmbiguous ErrorKind = "command_ambiguous"
	// ErrorKindScriptExecutionFailed means script execution failed for a generic reason.
	ErrorKindScriptExecutionFailed ErrorKind = "script_execution_failed"
	// ErrorKindContainerEngineNotFound means the selected container engine is unavailable.
	ErrorKindContainerEngineNotFound ErrorKind = "container_engine_not_found"
	// ErrorKindRuntimeNotAvailable means the selected runtime is unavailable.
	ErrorKindRuntimeNotAvailable ErrorKind = "runtime_not_available"
	// ErrorKindPermissionDenied means execution failed due to host permission denial.
	ErrorKindPermissionDenied ErrorKind = "permission_denied"
	// ErrorKindShellNotFound means a configured shell executable could not be found.
	ErrorKindShellNotFound ErrorKind = "shell_not_found"
)

type (
	//goplint:validate-all
	//
	// Request captures all execution inputs as an immutable value.
	// It mirrors the CLI-layer ExecuteRequest but lives in the service layer
	// to decouple from Cobra-specific concerns.
	Request struct {
		// Name is the fully-qualified command name (e.g., "io.invowk.sample build").
		Name string
		// Args are positional arguments to pass to the command script ($1, $2, etc.).
		Args []string
		// Runtime is the --ivk-runtime override (e.g., RuntimeContainer, RuntimeVirtual).
		// Zero value ("") means no override.
		Runtime invowkfile.RuntimeMode
		// Platform is the resolved execution platform. Zero value means use the
		// current host platform at the service boundary.
		Platform invowkfile.Platform
		// Interactive enables alternate screen buffer with TUI server.
		Interactive bool
		// InteractiveSet is true when the caller explicitly set Interactive.
		InteractiveSet bool
		// Verbose enables verbose diagnostic output.
		Verbose bool
		// VerboseSet is true when the caller explicitly set Verbose.
		VerboseSet bool
		// FromSource is the --ivk-from flag value for source disambiguation.
		FromSource discovery.SourceID
		// ForceRebuild forces container image rebuilds, bypassing cache.
		ForceRebuild bool
		// ContainerName overrides the persistent container target name for
		// container runtime execution. Zero value means no CLI override.
		ContainerName invowkfile.ContainerName
		// Workdir overrides the working directory for the command.
		Workdir invowkfile.WorkDir
		// EnvFiles are dotenv file paths from --ivk-env-file flags.
		EnvFiles []invowkfile.DotenvFilePath
		// EnvVars are KEY=VALUE pairs from --ivk-env-var flags (highest env priority).
		EnvVars map[string]string
		// ConfigPath is the explicit --ivk-config flag value.
		ConfigPath types.FilesystemPath
		// FlagValues are parsed flag values from Cobra state (key: flag name).
		FlagValues map[invowkfile.FlagName]string
		// FlagDefs are the command's flag definitions from the invowkfile.
		FlagDefs []invowkfile.Flag
		// ArgDefs are the command's argument definitions from the invowkfile.
		ArgDefs []invowkfile.Argument
		// EnvInheritMode overrides the runtime config env inherit mode.
		// Zero value ("") means no override.
		EnvInheritMode invowkfile.EnvInheritMode
		// EnvInheritAllow overrides the runtime config env allowlist.
		EnvInheritAllow []invowkfile.EnvVarName
		// EnvInheritDeny overrides the runtime config env denylist.
		EnvInheritDeny []invowkfile.EnvVarName
		// DryRun enables dry-run mode: returns execution plan without executing.
		DryRun bool
		// ResolvedCommand carries a pre-resolved command when the caller already
		// performed discovery (for example, dynamic Cobra leaf execution). When set,
		// Execute() can skip GetCommand discovery.
		ResolvedCommand *discovery.CommandInfo
		// UserEnv captures the host environment at execution entry, before invowk
		// injects command-level env vars. When nil, Execute() populates it eagerly
		// via the UserEnvProvider callback. Tests can set this to inject a controlled env.
		UserEnv map[string]string
	}

	//goplint:validate-all
	//
	// Result contains command execution outcomes.
	Result struct {
		// ExitCode is the command's exit code (0 = success).
		ExitCode types.ExitCode
		// DryRunData holds structured dry-run data when Request.DryRun is true.
		// When non-nil, the CLI adapter should render the dry-run view and skip execution.
		DryRunData *DryRunData
	}

	//goplint:validate-all
	//
	// DryRunData holds the structured data needed for dry-run rendering.
	// The CLI adapter uses this to render the execution plan without
	// importing service internals. All fields are plain types to avoid
	// coupling the adapter to runtime/discovery implementation details.
	DryRunData struct {
		// Plan is the service-owned command execution plan.
		Plan DryRunPlan
	}

	//goplint:validate-all
	//
	// DryRunPlan contains renderable command execution facts.
	//
	//goplint:mutable
	//
	// DryRunPlan is a service-to-adapter DTO assembled at the command boundary.
	DryRunPlan struct {
		// CommandName is the command requested by the user.
		CommandName invowkfile.CommandName
		// SourceID identifies the origin of the command.
		SourceID discovery.SourceID
		// Runtime is the selected runtime mode.
		Runtime invowkfile.RuntimeMode
		// Platform is the current target platform.
		Platform invowkfile.Platform
		// WorkDir is the explicit working-directory override, if any.
		WorkDir invowkfile.WorkDir
		// Timeout is the selected implementation timeout, if any.
		Timeout invowkfile.DurationString
		// PersistentContainerMode reports whether the container runtime would
		// use an ephemeral or persistent target.
		PersistentContainerMode string //goplint:ignore -- dry-run render DTO, not a domain value
		// PersistentContainerName is the resolved persistent container name.
		PersistentContainerName invowkfile.ContainerName
		// PersistentContainerNameSource explains where the persistent name came from.
		PersistentContainerNameSource string //goplint:ignore -- dry-run render DTO, not a domain value
		// PersistentContainerCreateIfMissing reports whether invowk would create
		// a missing managed persistent container.
		PersistentContainerCreateIfMissing bool
		// Script is the selected implementation script or script-file path.
		Script invowkfile.ScriptContent
		// ScriptIsFile is true when Script names a script file.
		ScriptIsFile bool
		// Env contains projected execution environment variables.
		Env map[string]string //goplint:ignore -- environment maps are stringly typed by os/exec and container APIs.
		// DependencyValidationSkipped is true because dry-run mode does not
		// execute dependency checks.
		DependencyValidationSkipped bool
	}

	// ClassifiedError is a typed error that carries a service-owned error kind
	// and a plain-text (unstyled) message. The CLI adapter maps Kind to the
	// presentation catalog and wraps this into a ServiceError with styled rendering.
	ClassifiedError struct {
		// Err is the underlying error.
		Err error
		// Kind classifies the domain failure without selecting presentation content.
		Kind ErrorKind
		// Message is a plain-text description of the error (no lipgloss styling).
		Message string
	}

	// AmbiguousCommandError is returned when an unqualified command exists in
	// multiple command sources and requires source disambiguation.
	AmbiguousCommandError struct {
		CommandName invowkfile.CommandName
		Sources     []discovery.SourceID
	}

	// SourceNotFoundError is returned when a source-constrained request names a
	// source that is not present in the discovered command set.
	SourceNotFoundError struct {
		Source           discovery.SourceID
		AvailableSources []discovery.SourceID
	}

	// UnsupportedPlatformError is returned when a command has no implementation
	// for the requested platform.
	UnsupportedPlatformError struct {
		CommandName invowkfile.CommandName
		Current     invowkfile.Platform
		Supported   []invowkfile.Platform
	}

	// RuntimeNotAllowedError is returned when a runtime override is incompatible
	// with the requested command on the target platform.
	RuntimeNotAllowedError struct {
		CommandName invowkfile.CommandName
		Runtime     invowkfile.RuntimeMode
		Platform    invowkfile.Platform
		Allowed     []invowkfile.RuntimeMode
	}

	//goplint:constant-only
	//
	// ErrorKind classifies command-service errors without depending on the CLI
	// issue catalog.
	ErrorKind string

	// CommandDiscovery discovers invowk commands.
	CommandDiscovery interface {
		DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error)
		GetCommand(ctx context.Context, name string) (discovery.LookupResult, error)
	}

	// UserEnvFunc captures the host environment. The service calls this
	// when Request.UserEnv is nil to eagerly snapshot the environment.
	UserEnvFunc func() map[string]string

	// resolvedDefinitions holds the resolved flag/arg definitions and parsed flag values
	// after applying fallbacks from the command's invowkfile definitions.
	resolvedDefinitions struct {
		flagDefs   []invowkfile.Flag
		argDefs    []invowkfile.Argument
		flagValues map[invowkfile.FlagName]string
	}
)

// Error implements the error interface.
func (e *ClassifiedError) Error() string { return e.Err.Error() }

// Unwrap returns the underlying error for errors.Is/As chains.
func (e *ClassifiedError) Unwrap() error { return e.Err }

// Error implements the error interface.
func (e *AmbiguousCommandError) Error() string {
	return fmt.Sprintf("command %q is ambiguous", e.CommandName)
}

// Error implements the error interface.
func (e *SourceNotFoundError) Error() string {
	return fmt.Sprintf("source %q not found", e.Source)
}

// Error implements the error interface.
func (e *UnsupportedPlatformError) Error() string {
	if e == nil {
		return ErrUnsupportedPlatform.Error()
	}
	supported := make([]string, 0, len(e.Supported))
	for i := range e.Supported {
		supported = append(supported, string(e.Supported[i]))
	}
	return fmt.Sprintf("command '%s' does not support platform '%s' (supported: %s)", e.CommandName, e.Current, strings.Join(supported, ", "))
}

// Unwrap returns ErrUnsupportedPlatform for errors.Is compatibility.
func (e *UnsupportedPlatformError) Unwrap() error { return ErrUnsupportedPlatform }

// Error implements the error interface.
func (e *RuntimeNotAllowedError) Error() string {
	if e == nil {
		return ErrRuntimeNotAllowed.Error()
	}
	allowed := make([]string, 0, len(e.Allowed))
	for i := range e.Allowed {
		allowed = append(allowed, string(e.Allowed[i]))
	}
	return fmt.Sprintf(
		"runtime '%s' is not allowed for command '%s' on platform '%s' (allowed: %s)",
		e.Runtime,
		e.CommandName,
		e.Platform,
		strings.Join(allowed, ", "),
	)
}

// Unwrap returns ErrRuntimeNotAllowed for errors.Is compatibility.
func (e *RuntimeNotAllowedError) Unwrap() error { return ErrRuntimeNotAllowed }

// String returns the string representation of the error kind.
func (k ErrorKind) String() string { return string(k) }

// Validate returns nil when the error kind is one of the service-defined categories.
func (k ErrorKind) Validate() error {
	switch k {
	case ErrorKindCommandNotFound,
		ErrorKindCommandAmbiguous,
		ErrorKindScriptExecutionFailed,
		ErrorKindContainerEngineNotFound,
		ErrorKindRuntimeNotAvailable,
		ErrorKindPermissionDenied,
		ErrorKindShellNotFound:
		return nil
	default:
		return errors.New("invalid command service error kind")
	}
}
