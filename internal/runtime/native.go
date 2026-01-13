package runtime

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"invowk-cli/pkg/invowkfile"
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

// Execute runs a command using the system shell
func (r *NativeRuntime) Execute(ctx *ExecutionContext) *Result {
	shell, err := r.getShell()
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
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
	env := r.buildEnv(ctx)
	cmd.Env = append(FilterInvowkEnvVars(os.Environ()), EnvToSlice(env)...)

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

// ExecuteCapture runs a command and captures its output
func (r *NativeRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	shell, err := r.getShell()
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
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

	env := r.buildEnv(ctx)
	cmd.Env = append(FilterInvowkEnvVars(os.Environ()), EnvToSlice(env)...)

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

// getWorkDir determines the working directory
func (r *NativeRuntime) getWorkDir(ctx *ExecutionContext) string {
	if ctx.WorkDir != "" {
		return ctx.WorkDir
	}
	if ctx.Command.WorkDir != "" {
		// Resolve relative to invowkfile location
		if !filepath.IsAbs(ctx.Command.WorkDir) {
			return filepath.Join(filepath.Dir(ctx.Invowkfile.FilePath), ctx.Command.WorkDir)
		}
		return ctx.Command.WorkDir
	}
	// Default to invowkfile directory
	return filepath.Dir(ctx.Invowkfile.FilePath)
}

// buildEnv builds the environment for the command
func (r *NativeRuntime) buildEnv(ctx *ExecutionContext) map[string]string {
	env := make(map[string]string)

	// Platform-level env from the selected implementation
	currentPlatform := invowkfile.GetCurrentHostOS()
	for _, p := range ctx.SelectedImpl.Target.Platforms {
		if p.Name == currentPlatform {
			for k, v := range p.Env {
				env[k] = v
			}
			break
		}
	}

	// Command-level env
	for k, v := range ctx.Command.Env {
		env[k] = v
	}

	// Extra env from context
	for k, v := range ctx.ExtraEnv {
		env[k] = v
	}

	return env
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
