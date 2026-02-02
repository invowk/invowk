// SPDX-License-Identifier: MPL-2.0

// Package runtime provides the command execution runtime interface and implementations.
package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"invowk-cli/pkg/invkfile"
)

// Runtime type constants for different execution environments.
const (
	RuntimeTypeNative    RuntimeType = "native"
	RuntimeTypeVirtual   RuntimeType = "virtual"
	RuntimeTypeContainer RuntimeType = "container"
)

type (
	// ExecutionContext contains all information needed to execute a command
	ExecutionContext struct {
		// Command is the command to execute
		Command *invkfile.Command
		// Invkfile is the parent invkfile
		Invkfile *invkfile.Invkfile
		// Context is the Go context for cancellation
		Context context.Context
		// Stdout is where to write standard output
		Stdout io.Writer
		// Stderr is where to write standard error
		Stderr io.Writer
		// Stdin is where to read standard input
		Stdin io.Reader
		// ExtraEnv contains additional environment variables
		ExtraEnv map[string]string
		// WorkDir overrides the working directory
		WorkDir string
		// Verbose enables verbose output
		Verbose bool
		// SelectedRuntime is the runtime to use for execution (may differ from default)
		SelectedRuntime invkfile.RuntimeMode
		// SelectedImpl is the implementation to execute (based on platform and runtime)
		SelectedImpl *invkfile.Implementation
		// PositionalArgs contains command-line arguments to pass as shell positional parameters ($1, $2, etc.)
		PositionalArgs []string
		// RuntimeEnvFiles contains dotenv file paths specified via --env-file flag.
		// These are loaded after all other env sources but before RuntimeEnvVars.
		// Paths are relative to the current working directory where invowk was invoked.
		RuntimeEnvFiles []string
		// RuntimeEnvVars contains env vars specified via --env-var flag.
		// These are set last and override all other environment variables (highest priority).
		RuntimeEnvVars map[string]string
		// EnvInheritModeOverride overrides the runtime config env inherit mode when set.
		EnvInheritModeOverride invkfile.EnvInheritMode
		// EnvInheritAllowOverride overrides the runtime config allowlist when set.
		EnvInheritAllowOverride []string
		// EnvInheritDenyOverride overrides the runtime config denylist when set.
		EnvInheritDenyOverride []string

		// ExecutionID is a unique identifier for this command execution.
		ExecutionID string

		// TUIServerURL is the URL of the TUI server for interactive mode.
		// When set, runtimes should include this in the command's environment
		// as INVOWK_TUI_ADDR. For container runtimes, this should already be
		// translated to a container-accessible address (e.g., host.docker.internal).
		TUIServerURL string
		// TUIServerToken is the authentication token for the TUI server.
		// When set, runtimes should include this in the command's environment
		// as INVOWK_TUI_TOKEN.
		TUIServerToken string
	}

	// Result contains the result of a command execution
	Result struct {
		// ExitCode is the exit code of the command
		ExitCode int
		// Error contains any error that occurred
		Error error
		// Output contains captured stdout (if captured)
		Output string
		// ErrOutput contains captured stderr (if captured)
		ErrOutput string
	}

	// Runtime defines the interface for command execution
	Runtime interface {
		// Name returns the runtime name
		Name() string
		// Execute runs a command in this runtime
		Execute(ctx *ExecutionContext) *Result
		// Available returns whether this runtime is available on the current system
		Available() bool
		// Validate checks if a command can be executed with this runtime
		Validate(ctx *ExecutionContext) error
	}

	// CapturingRuntime is implemented by runtimes that support capturing output.
	CapturingRuntime interface {
		// ExecuteCapture runs a command and captures stdout/stderr.
		ExecuteCapture(ctx *ExecutionContext) *Result
	}

	// InteractiveRuntime is implemented by runtimes that support interactive mode.
	// Interactive mode allows commands to be executed with PTY attachment for
	// full terminal interaction (keyboard input, terminal UI, etc.).
	InteractiveRuntime interface {
		Runtime

		// SupportsInteractive returns true if this runtime can run interactively.
		// This may depend on system configuration or the specific execution context.
		SupportsInteractive() bool

		// PrepareInteractive prepares the runtime for interactive execution.
		// The returned PreparedCommand contains an exec.Cmd that can be attached
		// to a PTY by the caller. The caller is responsible for calling the
		// Cleanup function after execution completes.
		PrepareInteractive(ctx *ExecutionContext) (*PreparedCommand, error)
	}

	// PreparedCommand contains a command ready for execution along with any cleanup function.
	// This is used by InteractiveRuntime implementations to return a command that
	// can be attached to a PTY for interactive execution.
	PreparedCommand struct {
		// Cmd is the prepared exec.Cmd ready for PTY attachment.
		Cmd *exec.Cmd
		// Cleanup is a function to call after execution (e.g., to remove temp files).
		// May be nil if no cleanup is needed.
		Cleanup func()
	}

	// RuntimeType identifies the type of runtime.
	//
	//nolint:revive // RuntimeType is more descriptive than Type for external callers
	RuntimeType string

	// Registry holds all available runtimes
	Registry struct {
		runtimes map[RuntimeType]Runtime
	}
)

// NewExecutionContext creates a new execution context with defaults
func NewExecutionContext(cmd *invkfile.Command, inv *invkfile.Invkfile) *ExecutionContext {
	currentPlatform := invkfile.GetCurrentHostOS()
	defaultRuntime := cmd.GetDefaultRuntimeForPlatform(currentPlatform)
	defaultScript := cmd.GetImplForPlatformRuntime(currentPlatform, defaultRuntime)

	return &ExecutionContext{
		Command:         cmd,
		Invkfile:        inv,
		Context:         context.Background(),
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
		Stdin:           os.Stdin,
		ExtraEnv:        make(map[string]string),
		SelectedRuntime: defaultRuntime,
		SelectedImpl:    defaultScript,
		ExecutionID:     newExecutionID(),
	}
}

func newExecutionID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Success returns true if the command executed successfully
func (r *Result) Success() bool {
	return r.ExitCode == 0 && r.Error == nil
}

// GetInteractiveRuntime returns the runtime as an InteractiveRuntime if it supports
// interactive mode, otherwise returns nil.
func GetInteractiveRuntime(rt Runtime) InteractiveRuntime {
	if ir, ok := rt.(InteractiveRuntime); ok && ir.SupportsInteractive() {
		return ir
	}
	return nil
}

// NewRegistry creates a new runtime registry
func NewRegistry() *Registry {
	return &Registry{
		runtimes: make(map[RuntimeType]Runtime),
	}
}

// Register adds a runtime to the registry
func (r *Registry) Register(typ RuntimeType, rt Runtime) {
	r.runtimes[typ] = rt
}

// Get returns a runtime by type
func (r *Registry) Get(typ RuntimeType) (Runtime, error) {
	rt, ok := r.runtimes[typ]
	if !ok {
		return nil, fmt.Errorf("runtime '%s' not registered", typ)
	}
	return rt, nil
}

// GetForCommand returns the appropriate runtime for a command based on its default runtime for current platform.
//
// Deprecated: Use GetForContext instead for proper runtime selection.
func (r *Registry) GetForCommand(cmd *invkfile.Command) (Runtime, error) {
	currentPlatform := invkfile.GetCurrentHostOS()
	typ := RuntimeType(cmd.GetDefaultRuntimeForPlatform(currentPlatform))
	return r.Get(typ)
}

// GetForContext returns the appropriate runtime based on the execution context's selected runtime
func (r *Registry) GetForContext(ctx *ExecutionContext) (Runtime, error) {
	typ := RuntimeType(ctx.SelectedRuntime)
	return r.Get(typ)
}

// Available returns all available runtimes
func (r *Registry) Available() []RuntimeType {
	var types []RuntimeType
	for typ, rt := range r.runtimes {
		if rt.Available() {
			types = append(types, typ)
		}
	}
	return types
}

// Execute runs a command using the appropriate runtime from the execution context
func (r *Registry) Execute(ctx *ExecutionContext) *Result {
	rt, err := r.GetForContext(ctx)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	if !rt.Available() {
		return &Result{
			ExitCode: 1,
			Error:    fmt.Errorf("runtime '%s' is not available on this system", rt.Name()),
		}
	}

	if err := rt.Validate(ctx); err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	return rt.Execute(ctx)
}

// EnvToSlice converts a map of environment variables to a slice
func EnvToSlice(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

// FilterInvowkEnvVars filters out Invowk-specific environment variables from the given
// environment slice. This prevents leakage of INVOWK_ARG_*, INVOWK_FLAG_*, ARGC, and ARGn
// variables when a command's script invokes another invowk command.
func FilterInvowkEnvVars(environ []string) []string {
	result := make([]string, 0, len(environ))
	for _, e := range environ {
		idx := findEnvSeparator(e)
		if idx == -1 {
			// Malformed env var, keep it
			result = append(result, e)
			continue
		}
		name := e[:idx]
		if shouldFilterEnvVar(name) {
			continue
		}
		result = append(result, e)
	}
	return result
}

// findEnvSeparator returns the index of the '=' separator in an environment variable string
func findEnvSeparator(e string) int {
	for i := 0; i < len(e); i++ {
		if e[i] == '=' {
			return i
		}
	}
	return -1
}

// shouldFilterEnvVar returns true if the environment variable name should be filtered
// to prevent leakage between nested invowk command invocations.
func shouldFilterEnvVar(name string) bool {
	// Filter INVOWK_ARG_* and INVOWK_FLAG_* variables
	if len(name) >= 11 && name[:11] == "INVOWK_ARG_" {
		return true
	}
	if len(name) >= 12 && name[:12] == "INVOWK_FLAG_" {
		return true
	}

	// Filter legacy ARGC variable
	if name == "ARGC" {
		return true
	}

	// Filter legacy ARGn variables (ARG1, ARG2, etc.)
	if len(name) > 3 && name[:3] == "ARG" {
		// Check if the rest is all digits
		for i := 3; i < len(name); i++ {
			if name[i] < '0' || name[i] > '9' {
				return false
			}
		}
		return true
	}

	return false
}
