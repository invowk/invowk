// SPDX-License-Identifier: EPL-2.0

package runtime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"invowk-cli/pkg/invkfile"
)

// NativeRuntime executes commands using the system's default shell
type NativeRuntime struct {
	// Shell overrides the default shell
	Shell string
	// ShellArgs are arguments passed to the shell before the script
	ShellArgs []string
}

// NewNativeRuntime creates a new native runtime
func NewNativeRuntime() *NativeRuntime {
	return &NativeRuntime{}
}

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
		return fmt.Errorf("no script selected for execution")
	}
	if ctx.SelectedImpl.Script == "" {
		return fmt.Errorf("script has no content to execute")
	}
	return nil
}

// Execute runs a command using the system shell or specified interpreter
func (r *NativeRuntime) Execute(ctx *ExecutionContext) *Result {
	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Get the runtime config to check for interpreter
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		// Fallback to default shell execution if no runtime config
		return r.executeWithShell(ctx, script)
	}

	// Resolve interpreter (defaults to "auto" which parses shebang)
	interpInfo := rtConfig.ResolveInterpreterFromScript(script)

	// If an interpreter was found, use interpreter-based execution
	if interpInfo.Found {
		return r.executeWithInterpreter(ctx, script, interpInfo)
	}

	// Otherwise, use default shell-based execution
	return r.executeWithShell(ctx, script)
}

// executeWithShell executes the script using the system shell (current behavior)
func (r *NativeRuntime) executeWithShell(ctx *ExecutionContext, script string) *Result {
	shell, err := r.getShell()
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Determine shell arguments
	args := r.getShellArgs(shell)
	args = append(args, script)

	// Append positional arguments for shell access ($1, $2, etc.)
	args = r.appendPositionalArgs(shell, args, ctx.PositionalArgs)

	cmd := exec.CommandContext(ctx.Context, shell, args...)

	// Set working directory with pre-validation
	workDir := r.getWorkDir(ctx)
	if workDir != "" {
		if err = validateWorkDir(workDir); err != nil {
			return &Result{ExitCode: 1, Error: fmt.Errorf("invalid working directory: %w", err)}
		}
		cmd.Dir = workDir
	}

	// Build environment
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to build environment: %w", err)}
	}
	cmd.Env = EnvToSlice(env)

	// Set I/O
	cmd.Stdout = ctx.Stdout
	cmd.Stderr = ctx.Stderr
	cmd.Stdin = ctx.Stdin

	// Execute
	if err = cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &Result{ExitCode: exitErr.ExitCode(), Error: nil}
		}
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to execute command: %w", err)}
	}

	return &Result{ExitCode: 0}
}

// executeWithInterpreter executes using the specified interpreter
func (r *NativeRuntime) executeWithInterpreter(ctx *ExecutionContext, script string, interp invkfile.ShebangInfo) *Result {
	// Verify interpreter exists
	interpreterPath, err := exec.LookPath(interp.Interpreter)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("interpreter '%s' not found in PATH: %w", interp.Interpreter, err)}
	}

	// Build command arguments
	var cmdArgs []string
	cmdArgs = append(cmdArgs, interp.Args...) // interpreter args (e.g., -u for python)

	// Handle file vs inline script
	var tempFile string
	if ctx.SelectedImpl.IsScriptFile() {
		// File script: interpreter [args] script-path positional-args
		scriptPath := ctx.SelectedImpl.GetScriptFilePath(ctx.Invkfile.FilePath)
		cmdArgs = append(cmdArgs, scriptPath)
	} else {
		// Inline script: create temp file
		tempFile, err = r.createTempScript(script, interp.Interpreter)
		if err != nil {
			return &Result{ExitCode: 1, Error: fmt.Errorf("failed to create temp script: %w", err)}
		}
		defer func() { _ = os.Remove(tempFile) }() // Cleanup temp file; error non-critical
		cmdArgs = append(cmdArgs, tempFile)
	}

	// Add positional arguments
	cmdArgs = append(cmdArgs, ctx.PositionalArgs...)

	cmd := exec.CommandContext(ctx.Context, interpreterPath, cmdArgs...)

	// Set working directory with pre-validation
	workDir := r.getWorkDir(ctx)
	if workDir != "" {
		if err = validateWorkDir(workDir); err != nil {
			return &Result{ExitCode: 1, Error: fmt.Errorf("invalid working directory: %w", err)}
		}
		cmd.Dir = workDir
	}

	// Build environment
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to build environment: %w", err)}
	}
	cmd.Env = EnvToSlice(env)

	// Set I/O
	cmd.Stdout = ctx.Stdout
	cmd.Stderr = ctx.Stderr
	cmd.Stdin = ctx.Stdin

	// Execute
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &Result{ExitCode: exitErr.ExitCode(), Error: nil}
		}
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to execute command: %w", err)}
	}

	return &Result{ExitCode: 0}
}

// createTempScript creates a temporary file with the script content
func (r *NativeRuntime) createTempScript(content, interpreter string) (string, error) {
	ext := invkfile.GetExtensionForInterpreter(interpreter)

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
	if runtime.GOOS != "windows" {
		_ = os.Chmod(tmpFile.Name(), 0700) // Best-effort; execution may still work
	}

	return tmpFile.Name(), nil
}

// ExecuteCapture runs a command and captures its output
func (r *NativeRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Get the runtime config to check for interpreter
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		// Fallback to default shell execution if no runtime config
		return r.executeCaptureWithShell(ctx, script)
	}

	// Resolve interpreter (defaults to "auto" which parses shebang)
	interpInfo := rtConfig.ResolveInterpreterFromScript(script)

	// If an interpreter was found, use interpreter-based execution
	if interpInfo.Found {
		return r.executeCaptureWithInterpreter(ctx, script, interpInfo)
	}

	// Otherwise, use default shell-based execution
	return r.executeCaptureWithShell(ctx, script)
}

// executeCaptureWithShell executes using shell and captures output
func (r *NativeRuntime) executeCaptureWithShell(ctx *ExecutionContext, script string) *Result {
	shell, err := r.getShell()
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	args := r.getShellArgs(shell)
	args = append(args, script)

	// Append positional arguments for shell access ($1, $2, etc.)
	args = r.appendPositionalArgs(shell, args, ctx.PositionalArgs)

	cmd := exec.CommandContext(ctx.Context, shell, args...)

	workDir := r.getWorkDir(ctx)
	if workDir != "" {
		cmd.Dir = workDir
	}

	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to build environment: %w", err)}
	}
	cmd.Env = EnvToSlice(env)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result := &Result{
		Output:    stdout.String(),
		ErrOutput: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result
}

// executeCaptureWithInterpreter executes using interpreter and captures output
func (r *NativeRuntime) executeCaptureWithInterpreter(ctx *ExecutionContext, script string, interp invkfile.ShebangInfo) *Result {
	// Verify interpreter exists
	interpreterPath, err := exec.LookPath(interp.Interpreter)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("interpreter '%s' not found in PATH: %w", interp.Interpreter, err)}
	}

	// Build command arguments
	var cmdArgs []string
	cmdArgs = append(cmdArgs, interp.Args...)

	// Handle file vs inline script
	var tempFile string
	if ctx.SelectedImpl.IsScriptFile() {
		scriptPath := ctx.SelectedImpl.GetScriptFilePath(ctx.Invkfile.FilePath)
		cmdArgs = append(cmdArgs, scriptPath)
	} else {
		tempFile, err = r.createTempScript(script, interp.Interpreter)
		if err != nil {
			return &Result{ExitCode: 1, Error: fmt.Errorf("failed to create temp script: %w", err)}
		}
		defer func() { _ = os.Remove(tempFile) }() // Cleanup temp file; error non-critical
		cmdArgs = append(cmdArgs, tempFile)
	}

	cmdArgs = append(cmdArgs, ctx.PositionalArgs...)

	cmd := exec.CommandContext(ctx.Context, interpreterPath, cmdArgs...)

	workDir := r.getWorkDir(ctx)
	if workDir != "" {
		cmd.Dir = workDir
	}

	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to build environment: %w", err)}
	}
	cmd.Env = EnvToSlice(env)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result := &Result{
		Output:    stdout.String(),
		ErrOutput: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result
}

// getShell determines which shell to use
func (r *NativeRuntime) getShell() (string, error) {
	// Use configured shell if set
	if r.Shell != "" {
		return r.Shell, nil
	}

	// Platform-specific defaults
	switch runtime.GOOS {
	case "windows":
		// Try PowerShell first, then cmd
		if pwsh, err := exec.LookPath("pwsh"); err == nil {
			return pwsh, nil
		}
		if ps, err := exec.LookPath("powershell"); err == nil {
			return ps, nil
		}
		return exec.LookPath("cmd")
	default:
		// Unix-like: use SHELL env var, or fall back to common shells
		if shell := os.Getenv("SHELL"); shell != "" {
			return shell, nil
		}
		if bash, err := exec.LookPath("bash"); err == nil {
			return bash, nil
		}
		if sh, err := exec.LookPath("sh"); err == nil {
			return sh, nil
		}
		return "", fmt.Errorf("no shell found")
	}
}

// getShellArgs returns the arguments to pass to the shell
func (r *NativeRuntime) getShellArgs(shell string) []string {
	if len(r.ShellArgs) > 0 {
		return r.ShellArgs
	}

	base := filepath.Base(shell)
	base = strings.TrimSuffix(base, ".exe")

	switch base {
	case "cmd":
		return []string{"/C"}
	case "powershell", "pwsh":
		return []string{"-NoProfile", "-Command"}
	default:
		// Assume POSIX shell
		return []string{"-c"}
	}
}

// getWorkDir determines the working directory using the hierarchical override model.
// Precedence (highest to lowest): CLI override > Implementation > Command > Root > Default
func (r *NativeRuntime) getWorkDir(ctx *ExecutionContext) string {
	return ctx.Invkfile.GetEffectiveWorkDir(ctx.Command, ctx.SelectedImpl, ctx.WorkDir)
}

// appendPositionalArgs appends positional arguments after the script for shell access.
// For POSIX shells (bash, sh, zsh): args become $1, $2, ... (with "invowk" as $0)
// For PowerShell: args become $args[0], $args[1], ...
// For cmd.exe: no change (doesn't support inline positional args)
func (r *NativeRuntime) appendPositionalArgs(shell string, args []string, positionalArgs []string) []string {
	if len(positionalArgs) == 0 {
		return args
	}

	// Extract base name, handling both Unix and Windows path separators
	base := filepath.Base(shell)
	// Also handle Windows paths on Unix systems
	if lastSlash := strings.LastIndex(base, "\\"); lastSlash >= 0 {
		base = base[lastSlash+1:]
	}
	base = strings.TrimSuffix(base, ".exe")

	switch base {
	case "cmd":
		// cmd.exe doesn't support passing args after /C "script"
		// Scripts must use environment variables instead
		return args
	case "powershell", "pwsh":
		// PowerShell: args after -Command are available via $args array
		return append(args, positionalArgs...)
	default:
		// POSIX shells (bash, sh, zsh, etc.): bash -c 'script' $0 $1 $2 ...
		// First arg after script becomes $0 (conventionally the script name)
		// Subsequent args become $1, $2, etc.
		args = append(args, "invowk") // $0 placeholder
		return append(args, positionalArgs...)
	}
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
	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return nil, err
	}

	// Get the runtime config to check for interpreter
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		// Fallback to default shell execution if no runtime config
		return r.prepareShellCommand(ctx, script)
	}

	// Resolve interpreter (defaults to "auto" which parses shebang)
	interpInfo := rtConfig.ResolveInterpreterFromScript(script)

	// If an interpreter was found, use interpreter-based execution
	if interpInfo.Found {
		return r.prepareInterpreterCommand(ctx, script, interpInfo)
	}

	// Otherwise, use default shell-based execution
	return r.prepareShellCommand(ctx, script)
}

// prepareShellCommand builds a shell command without executing it.
func (r *NativeRuntime) prepareShellCommand(ctx *ExecutionContext, script string) (*PreparedCommand, error) {
	shell, err := r.getShell()
	if err != nil {
		return nil, err
	}

	// Determine shell arguments
	args := r.getShellArgs(shell)
	args = append(args, script)

	// Append positional arguments for shell access ($1, $2, etc.)
	args = r.appendPositionalArgs(shell, args, ctx.PositionalArgs)

	cmd := exec.CommandContext(ctx.Context, shell, args...)

	// Set working directory
	workDir := r.getWorkDir(ctx)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Build environment
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		return nil, fmt.Errorf("failed to build environment: %w", err)
	}
	cmd.Env = EnvToSlice(env)

	return &PreparedCommand{Cmd: cmd, Cleanup: nil}, nil
}

// prepareInterpreterCommand builds an interpreter command without executing it.
func (r *NativeRuntime) prepareInterpreterCommand(ctx *ExecutionContext, script string, interp invkfile.ShebangInfo) (*PreparedCommand, error) {
	// Verify interpreter exists
	interpreterPath, err := exec.LookPath(interp.Interpreter)
	if err != nil {
		return nil, fmt.Errorf("interpreter '%s' not found in PATH: %w", interp.Interpreter, err)
	}

	// Build command arguments
	var cmdArgs []string
	cmdArgs = append(cmdArgs, interp.Args...) // interpreter args (e.g., -u for python)

	// Handle file vs inline script
	var tempFile string
	var cleanup func()
	if ctx.SelectedImpl.IsScriptFile() {
		// File script: interpreter [args] script-path positional-args
		scriptPath := ctx.SelectedImpl.GetScriptFilePath(ctx.Invkfile.FilePath)
		cmdArgs = append(cmdArgs, scriptPath)
	} else {
		// Inline script: create temp file
		tempFile, err = r.createTempScript(script, interp.Interpreter)
		if err != nil {
			return nil, fmt.Errorf("failed to create temp script: %w", err)
		}
		cmdArgs = append(cmdArgs, tempFile)
		cleanup = func() { _ = os.Remove(tempFile) } // Cleanup temp file; error non-critical
	}

	// Add positional arguments
	cmdArgs = append(cmdArgs, ctx.PositionalArgs...)

	cmd := exec.CommandContext(ctx.Context, interpreterPath, cmdArgs...)

	// Set working directory
	workDir := r.getWorkDir(ctx)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Build environment
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, fmt.Errorf("failed to build environment: %w", err)
	}
	cmd.Env = EnvToSlice(env)

	return &PreparedCommand{Cmd: cmd, Cleanup: cleanup}, nil
}
