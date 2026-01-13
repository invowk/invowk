// Package runtime provides the command execution runtime interface and implementations.
package runtime

import (
	"context"
	"fmt"
	"io"
	"os"

	"invowk-cli/pkg/invowkfile"
)

// ExecutionContext contains all information needed to execute a command
type ExecutionContext struct {
	// Command is the command to execute
	Command *invowkfile.Command
	// Invowkfile is the parent invowkfile
	Invowkfile *invowkfile.Invowkfile
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
}

// NewExecutionContext creates a new execution context with defaults
func NewExecutionContext(cmd *invowkfile.Command, inv *invowkfile.Invowkfile) *ExecutionContext {
	return &ExecutionContext{
		Command:    cmd,
		Invowkfile: inv,
		Context:    context.Background(),
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Stdin:      os.Stdin,
		ExtraEnv:   make(map[string]string),
	}
}

// Result contains the result of a command execution
type Result struct {
	// ExitCode is the exit code of the command
	ExitCode int
	// Error contains any error that occurred
	Error error
	// Output contains captured stdout (if captured)
	Output string
	// ErrOutput contains captured stderr (if captured)
	ErrOutput string
}

// Success returns true if the command executed successfully
func (r *Result) Success() bool {
	return r.ExitCode == 0 && r.Error == nil
}

// Runtime defines the interface for command execution
type Runtime interface {
	// Name returns the runtime name
	Name() string
	// Execute runs a command in this runtime
	Execute(ctx *ExecutionContext) *Result
	// Available returns whether this runtime is available on the current system
	Available() bool
	// Validate checks if a command can be executed with this runtime
	Validate(ctx *ExecutionContext) error
}

// RuntimeType identifies the type of runtime
type RuntimeType string

const (
	RuntimeTypeNative    RuntimeType = "native"
	RuntimeTypeVirtual   RuntimeType = "virtual"
	RuntimeTypeContainer RuntimeType = "container"
)

// Registry holds all available runtimes
type Registry struct {
	runtimes map[RuntimeType]Runtime
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

// GetForCommand returns the appropriate runtime for a command
func (r *Registry) GetForCommand(cmd *invowkfile.Command) (Runtime, error) {
	typ := RuntimeType(cmd.Runtime)
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

// Execute runs a command using the appropriate runtime
func (r *Registry) Execute(ctx *ExecutionContext) *Result {
	rt, err := r.GetForCommand(ctx.Command)
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

// MergeEnv merges environment variables from multiple sources
func MergeEnv(base map[string]string, overlay ...map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for _, m := range overlay {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// EnvToSlice converts a map of environment variables to a slice
func EnvToSlice(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}
