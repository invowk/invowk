// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"invowk-cli/internal/issue"
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
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Create streaming output configuration
	output := newStreamingOutput(ctx.Stdout, ctx.Stderr)

	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return r.executeShellCommon(ctx, script, output, nil, ctx.Stdin)
	}

	interpInfo := rtConfig.ResolveInterpreterFromScript(script)
	if interpInfo.Found {
		return r.executeInterpreterCommon(ctx, script, interpInfo, output, nil, ctx.Stdin)
	}

	return r.executeShellCommon(ctx, script, output, nil, ctx.Stdin)
}

// ExecuteCapture runs a command and captures its output
func (r *NativeRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
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
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
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
		return &Result{ExitCode: 1, Error: err}
	}

	args := r.getShellArgs(shell)
	args = append(args, script)
	args = r.appendPositionalArgs(shell, args, ctx.PositionalArgs)

	cmd := exec.CommandContext(ctx.Context, shell, args...)

	// Set working directory with validation
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

	// Set I/O based on output mode
	cmd.Stdout = output.stdout
	cmd.Stderr = output.stderr
	cmd.Stdin = stdin

	// Execute and extract result
	err = cmd.Run()
	return extractExitCode(err, captured)
}

// executeInterpreterCommon is the unified interpreter execution function that handles
// both streaming and capturing modes based on the output configuration.
func (r *NativeRuntime) executeInterpreterCommon(ctx *ExecutionContext, script string, interp invkfile.ShebangInfo, output *executeOutput, captured *capturedOutput, stdin io.Reader) *Result {
	interpreterPath, err := exec.LookPath(interp.Interpreter)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("interpreter '%s' not found in PATH: %w", interp.Interpreter, err)}
	}

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

	// Set working directory with validation
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

	// Set I/O based on output mode
	cmd.Stdout = output.stdout
	cmd.Stderr = output.stderr
	cmd.Stdin = stdin

	// Execute and extract result
	err = cmd.Run()
	return extractExitCode(err, captured)
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
		_ = os.Chmod(tmpFile.Name(), 0o700) // Best-effort; execution may still work
	}

	return tmpFile.Name(), nil
}

// getShell determines which shell to use
func (r *NativeRuntime) getShell() (string, error) {
	if r.Shell != "" {
		return r.Shell, nil
	}

	switch runtime.GOOS {
	case "windows":
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

// shellNotFoundError creates an actionable error for shell not found scenarios.
func (r *NativeRuntime) shellNotFoundError(attempted []string) error {
	ctx := issue.NewErrorContext().
		WithOperation("find shell").
		WithResource("shells attempted: " + strings.Join(attempted, ", "))

	switch runtime.GOOS {
	case "windows":
		ctx.WithSuggestion("Install PowerShell Core (pwsh) from https://aka.ms/powershell")
		ctx.WithSuggestion("Or ensure PowerShell or cmd.exe is in your PATH")
	case "darwin":
		ctx.WithSuggestion("Set the SHELL environment variable to a valid shell path")
		ctx.WithSuggestion("Or install bash via Homebrew: brew install bash")
	default:
		ctx.WithSuggestion("Set the SHELL environment variable to a valid shell path")
		ctx.WithSuggestion("Or install bash: apt install bash (Debian/Ubuntu) or dnf install bash (Fedora)")
	}

	ctx.WithSuggestion("Alternatively, use the virtual runtime: invowk cmd <command> --runtime virtual")

	return ctx.Wrap(fmt.Errorf("no shell found in PATH")).BuildError()
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

	workDir := r.getWorkDir(ctx)
	if workDir != "" {
		cmd.Dir = workDir
	}

	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		return nil, fmt.Errorf("failed to build environment: %w", err)
	}
	cmd.Env = EnvToSlice(env)

	return &PreparedCommand{Cmd: cmd, Cleanup: nil}, nil
}

// prepareInterpreterCommand builds an interpreter command without executing it.
func (r *NativeRuntime) prepareInterpreterCommand(ctx *ExecutionContext, script string, interp invkfile.ShebangInfo) (*PreparedCommand, error) {
	interpreterPath, err := exec.LookPath(interp.Interpreter)
	if err != nil {
		return nil, fmt.Errorf("interpreter '%s' not found in PATH: %w", interp.Interpreter, err)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, interp.Args...)

	var tempFile string
	var cleanup func()
	if ctx.SelectedImpl.IsScriptFile() {
		scriptPath := ctx.SelectedImpl.GetScriptFilePath(ctx.Invkfile.FilePath)
		cmdArgs = append(cmdArgs, scriptPath)
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

	workDir := r.getWorkDir(ctx)
	if workDir != "" {
		cmd.Dir = workDir
	}

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
