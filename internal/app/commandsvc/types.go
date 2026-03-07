// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"

	appexec "github.com/invowk/invowk/internal/app/execute"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
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
		// Interactive enables alternate screen buffer with TUI server.
		Interactive bool
		// Verbose enables verbose diagnostic output.
		Verbose bool
		// FromSource is the --ivk-from flag value for source disambiguation.
		FromSource discovery.SourceID
		// ForceRebuild forces container image rebuilds, bypassing cache.
		ForceRebuild bool
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
		// SourceID identifies the origin of the command.
		SourceID discovery.SourceID
		// Selection is the resolved runtime mode + implementation.
		Selection appexec.RuntimeSelection
		// ExecCtx is the constructed execution context.
		ExecCtx *runtime.ExecutionContext
	}

	// ClassifiedError is a typed error that carries an issue catalog ID and
	// a plain-text (unstyled) message. The CLI adapter wraps this into a
	// ServiceError with styled rendering.
	ClassifiedError struct {
		// Err is the underlying error.
		Err error
		// IssueID is the issue catalog ID for rendering help text.
		IssueID issue.Id
		// Message is a plain-text description of the error (no lipgloss styling).
		Message string
	}

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
