// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

const (
	failedBuildEnvironmentFmt = "failed to build environment: %w"
	cmdWaitDelay              = 10 * time.Second
)

type (
	// NativeRuntime executes commands using the system's default shell.
	// Shell and shell arguments are immutable after construction via NewNativeRuntime.
	NativeRuntime struct {
		// shell overrides the default shell
		shell types.ShellPath
		// shellArgs are arguments passed to the shell before the script
		shellArgs []string
		// envBuilder builds environment variables for execution
		envBuilder EnvBuilder
	}

	// NativeRuntimeOption configures a NativeRuntime.
	NativeRuntimeOption func(*NativeRuntime)

	// ShellLookupAttempt identifies one shell candidate considered by NativeRuntime.
	ShellLookupAttempt string

	// ShellLookupAttempts is the ordered list of shell candidates considered by NativeRuntime.
	ShellLookupAttempts []ShellLookupAttempt

	// ShellNotFoundError reports the shell lookup attempts made by NativeRuntime.
	ShellNotFoundError struct {
		Attempted ShellLookupAttempts
	}
)

// WithShell sets the shell path for the native runtime.
// If not set, the runtime auto-detects the shell from the environment.
func WithShell(shell types.ShellPath) NativeRuntimeOption {
	return func(r *NativeRuntime) {
		r.shell = shell
	}
}

// WithShellArgs sets the shell arguments for the native runtime.
// If not set, arguments are derived from the shell type (e.g., "-c" for bash).
func WithShellArgs(args []string) NativeRuntimeOption {
	return func(r *NativeRuntime) {
		r.shellArgs = args
	}
}

// WithEnvBuilder sets the environment builder for the native runtime.
// If not set, NewDefaultEnvBuilder() is used.
func WithEnvBuilder(b EnvBuilder) NativeRuntimeOption {
	return func(r *NativeRuntime) {
		r.envBuilder = b
	}
}

// NewNativeRuntime creates a new native runtime with optional configuration.
func NewNativeRuntime(opts ...NativeRuntimeOption) *NativeRuntime {
	r := &NativeRuntime{
		envBuilder: NewDefaultEnvBuilder(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Shell returns the configured shell path.
func (r *NativeRuntime) Shell() types.ShellPath { return r.shell }

// ShellArgs returns the configured shell arguments.
func (r *NativeRuntime) ShellArgs() []string { return r.shellArgs }

// Name returns the runtime name
func (r *NativeRuntime) Name() string {
	return "native"
}

// Available returns whether this runtime is available
func (r *NativeRuntime) Available() bool {
	_, err := r.getShell()
	return err == nil
}

// Validate checks if a command can be executed
func (r *NativeRuntime) Validate(ctx *ExecutionContext) error {
	if ctx.SelectedImpl == nil {
		return errNativeNoImpl
	}
	if ctx.SelectedImpl.Script == "" {
		return errNativeNoScript
	}
	return nil
}

// Execute runs a command using the system shell or specified interpreter
func (r *NativeRuntime) Execute(ctx *ExecutionContext) *Result {
	if err := validateExecutionContextForRun(ctx, errNativeNoImpl, errNativeNoScript); err != nil {
		return NewErrorResult(1, err)
	}

	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return NewErrorResult(1, err)
	}

	// Create streaming output configuration
	output := newStreamingOutput(ctx.IO.Stdout, ctx.IO.Stderr)

	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return r.executeShellCommon(ctx, script, output, nil, ctx.IO.Stdin)
	}

	interpInfo := rtConfig.ResolveInterpreterFromScript(script)
	if interpInfo.Found {
		return r.executeInterpreterCommon(ctx, script, interpInfo, output, nil, ctx.IO.Stdin)
	}

	return r.executeShellCommon(ctx, script, output, nil, ctx.IO.Stdin)
}

// ExecuteCapture runs a command and captures its output
func (r *NativeRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	if err := validateExecutionContextForRun(ctx, errNativeNoImpl, errNativeNoScript); err != nil {
		return NewErrorResult(1, err)
	}

	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return NewErrorResult(1, err)
	}

	// Create capturing output configuration
	output, captured := newCapturingOutput()

	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return r.executeShellCommon(ctx, script, output, captured, nil)
	}

	interpInfo := rtConfig.ResolveInterpreterFromScript(script)
	if interpInfo.Found {
		return r.executeInterpreterCommon(ctx, script, interpInfo, output, captured, nil)
	}

	return r.executeShellCommon(ctx, script, output, captured, nil)
}

// SupportsInteractive returns true as the native runtime always supports interactive mode.
func (r *NativeRuntime) SupportsInteractive() bool {
	return true
}

// PrepareInteractive prepares the native runtime for interactive execution.
// This is an alias for PrepareCommand to implement the InteractiveRuntime interface.
func (r *NativeRuntime) PrepareInteractive(ctx *ExecutionContext) (*PreparedCommand, error) {
	return r.PrepareCommand(ctx)
}

// PrepareCommand builds an exec.Cmd for the given execution context without running it.
// This is useful for interactive mode where the command needs to be run on a PTY.
// The caller must call the returned cleanup function after execution.
func (r *NativeRuntime) PrepareCommand(ctx *ExecutionContext) (*PreparedCommand, error) {
	if err := validateExecutionContextForRun(ctx, errNativeNoImpl, errNativeNoScript); err != nil {
		return nil, err
	}

	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return nil, err
	}

	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return r.prepareShellCommand(ctx, script)
	}

	interpInfo := rtConfig.ResolveInterpreterFromScript(script)
	if interpInfo.Found {
		return r.prepareInterpreterCommand(ctx, script, interpInfo)
	}

	return r.prepareShellCommand(ctx, script)
}

// executeShellCommon is the unified shell execution function that handles both
// streaming and capturing modes based on the output configuration.
func (r *NativeRuntime) executeShellCommon(ctx *ExecutionContext, script string, output *executeOutput, captured *capturedOutput, stdin io.Reader) *Result {
	shell, err := r.getShell()
	if err != nil {
		return NewErrorResult(1, err)
	}

	args := r.getShellArgs(shell)
	args = append(args, script)
	args = r.appendPositionalArgs(shell, args, ctx.PositionalArgs)

	cmd := exec.CommandContext(ctx.Context, shell, args...)
	cmd.WaitDelay = cmdWaitDelay

	if err = r.configureCommandDirAndEnv(cmd, ctx); err != nil {
		return NewErrorResult(1, err)
	}

	// Set I/O based on output mode
	cmd.Stdout = output.stdout
	cmd.Stderr = output.stderr
	cmd.Stdin = stdin

	// Execute and extract result
	err = cmd.Run()
	result := extractExitCode(err, captured)
	promoteContextError(ctx, result)
	return result
}

// executeInterpreterCommon is the unified interpreter execution function that handles
// both streaming and capturing modes based on the output configuration.
func (r *NativeRuntime) executeInterpreterCommon(ctx *ExecutionContext, script string, interp invowkfile.ShebangInfo, output *executeOutput, captured *capturedOutput, stdin io.Reader) *Result {
	interpreterPath, err := exec.LookPath(interp.Interpreter)
	if err != nil {
		return NewErrorResult(1, fmt.Errorf("interpreter '%s' not found in PATH: %w", interp.Interpreter, err))
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, interp.Args...)

	// Handle file vs inline script
	var tempFile string
	if ctx.SelectedImpl.IsScriptFile() {
		scriptPath := ctx.SelectedScriptFilePath()
		cmdArgs = append(cmdArgs, string(scriptPath))
	} else {
		tempFile, err = r.createTempScript(script, interp.Interpreter)
		if err != nil {
			return NewErrorResult(1, fmt.Errorf("failed to create temp script: %w", err))
		}
		defer func() { _ = os.Remove(tempFile) }() // Cleanup temp file; error non-critical
		cmdArgs = append(cmdArgs, tempFile)
	}

	cmdArgs = append(cmdArgs, ctx.PositionalArgs...)

	cmd := exec.CommandContext(ctx.Context, interpreterPath, cmdArgs...)
	cmd.WaitDelay = cmdWaitDelay

	if err = r.configureCommandDirAndEnv(cmd, ctx); err != nil {
		return NewErrorResult(1, err)
	}

	// Set I/O based on output mode
	cmd.Stdout = output.stdout
	cmd.Stderr = output.stderr
	cmd.Stdin = stdin

	// Execute and extract result
	err = cmd.Run()
	result := extractExitCode(err, captured)
	promoteContextError(ctx, result)
	return result
}

// createTempScript creates a temporary file with the script content
func (r *NativeRuntime) createTempScript(content, interpreter string) (string, error) {
	ext := invowkfile.GetExtensionForInterpreter(interpreter)

	tmpFile, err := os.CreateTemp("", "invowk-script-*"+ext)
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()           // Best-effort close on error path
		_ = os.Remove(tmpFile.Name()) // Best-effort cleanup on error path
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name()) // Best-effort cleanup on error path
		return "", err
	}

	// Make executable (needed for some interpreters on Unix)
	if runtime.GOOS != platform.Windows {
		_ = os.Chmod(tmpFile.Name(), 0o700) // Best-effort; execution may still work
	}

	return tmpFile.Name(), nil
}

// getShell determines which shell to use
func (r *NativeRuntime) getShell() (string, error) {
	if r.shell != "" {
		return string(r.shell), nil
	}

	switch runtime.GOOS {
	case platform.Windows:
		if pwsh, err := exec.LookPath("pwsh"); err == nil {
			return pwsh, nil
		}
		if ps, err := exec.LookPath("powershell"); err == nil {
			return ps, nil
		}
		if cmd, err := exec.LookPath("cmd"); err == nil {
			return cmd, nil
		}
		return "", r.shellNotFoundError([]string{"pwsh", "powershell", "cmd"})
	default:
		if shell := os.Getenv("SHELL"); shell != "" {
			return shell, nil
		}
		if bash, err := exec.LookPath("bash"); err == nil {
			return bash, nil
		}
		if sh, err := exec.LookPath("sh"); err == nil {
			return sh, nil
		}
		return "", r.shellNotFoundError([]string{"$SHELL", "bash", "sh"})
	}
}

// Error implements the error interface.
func (e *ShellNotFoundError) Error() string {
	if e == nil {
		return ErrShellNotFound.Error()
	}
	if len(e.Attempted) == 0 {
		return ErrShellNotFound.Error()
	}
	attempts := make([]string, 0, len(e.Attempted))
	for _, attempt := range e.Attempted {
		attempts = append(attempts, attempt.String())
	}
	return fmt.Sprintf("%s; attempted: %s", ErrShellNotFound, strings.Join(attempts, ", "))
}

// Unwrap returns ErrShellNotFound so callers can classify shell lookup failures.
func (e *ShellNotFoundError) Unwrap() error { return ErrShellNotFound }

// String returns the shell lookup attempt text.
func (a ShellLookupAttempt) String() string { return string(a) }

// Validate returns an error if the shell lookup attempt is empty.
func (a ShellLookupAttempt) Validate() error {
	if strings.TrimSpace(string(a)) == "" {
		return errors.New("shell lookup attempt must be non-empty")
	}
	return nil
}

// shellNotFoundError creates a runtime-owned shell lookup error.
func (r *NativeRuntime) shellNotFoundError(attempted []string) error {
	copied := slices.Clone(attempted)
	result := make(ShellLookupAttempts, 0, len(copied))
	for _, attempt := range copied {
		lookupAttempt := ShellLookupAttempt(attempt)
		if err := lookupAttempt.Validate(); err != nil {
			continue
		}
		result = append(result, lookupAttempt)
	}
	return &ShellNotFoundError{Attempted: result}
}

// getShellArgs returns the arguments to pass to the shell
func (r *NativeRuntime) getShellArgs(shell string) []string {
	if len(r.shellArgs) > 0 {
		return r.shellArgs
	}

	base := filepath.Base(shell)
	base = strings.TrimSuffix(base, ".exe")

	switch base {
	case "cmd":
		return []string{"/C"}
	case "powershell", "pwsh":
		return []string{"-NoProfile", "-Command"}
	default:
		return []string{"-c"}
	}
}

// appendPositionalArgs appends positional arguments after the script for shell access.
// For POSIX shells (bash, sh, zsh): args become $1, $2, ... (with "invowk" as $0)
// For PowerShell: args become $args[0], $args[1], ...
// For cmd.exe: no change (doesn't support inline positional args)
func (r *NativeRuntime) appendPositionalArgs(shell string, args, positionalArgs []string) []string {
	if len(positionalArgs) == 0 {
		return args
	}

	base := filepath.Base(shell)
	if lastSlash := strings.LastIndex(base, "\\"); lastSlash >= 0 {
		base = base[lastSlash+1:]
	}
	base = strings.TrimSuffix(base, ".exe")

	switch base {
	case "cmd":
		return args
	case "powershell", "pwsh":
		return append(args, positionalArgs...)
	default:
		args = append(args, "invowk") // $0 placeholder
		return append(args, positionalArgs...)
	}
}

// prepareShellCommand builds a shell command without executing it.
func (r *NativeRuntime) prepareShellCommand(ctx *ExecutionContext, script string) (*PreparedCommand, error) {
	shell, err := r.getShell()
	if err != nil {
		return nil, err
	}

	args := r.getShellArgs(shell)
	args = append(args, script)
	args = r.appendPositionalArgs(shell, args, ctx.PositionalArgs)

	cmd := exec.CommandContext(ctx.Context, shell, args...)
	cmd.WaitDelay = cmdWaitDelay

	if err := r.configureCommandDirAndEnv(cmd, ctx); err != nil {
		return nil, err
	}

	return &PreparedCommand{Cmd: cmd, Cleanup: nil}, nil
}

// prepareInterpreterCommand builds an interpreter command without executing it.
func (r *NativeRuntime) prepareInterpreterCommand(ctx *ExecutionContext, script string, interp invowkfile.ShebangInfo) (*PreparedCommand, error) {
	interpreterPath, err := exec.LookPath(interp.Interpreter)
	if err != nil {
		return nil, fmt.Errorf("interpreter '%s' not found in PATH: %w", interp.Interpreter, err)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, interp.Args...)

	var tempFile string
	var cleanup func()
	if ctx.SelectedImpl.IsScriptFile() {
		scriptPath := ctx.SelectedScriptFilePath()
		cmdArgs = append(cmdArgs, string(scriptPath))
	} else {
		tempFile, err = r.createTempScript(script, interp.Interpreter)
		if err != nil {
			return nil, fmt.Errorf("failed to create temp script: %w", err)
		}
		cmdArgs = append(cmdArgs, tempFile)
		cleanup = func() { _ = os.Remove(tempFile) } // Cleanup temp file; error non-critical
	}

	cmdArgs = append(cmdArgs, ctx.PositionalArgs...)

	cmd := exec.CommandContext(ctx.Context, interpreterPath, cmdArgs...)
	cmd.WaitDelay = cmdWaitDelay

	if err = r.configureCommandDirAndEnv(cmd, ctx); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, err
	}

	return &PreparedCommand{Cmd: cmd, Cleanup: cleanup}, nil
}
