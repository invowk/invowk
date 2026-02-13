// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"invowk-cli/internal/container"
	"invowk-cli/internal/sshserver"
	"invowk-cli/pkg/invowkfile"
)

// containerRunMu is a fallback mutex used when flock-based cross-process
// serialization is unavailable. This includes non-Linux platforms
// (macOS/Windows Podman runs in a VM) and Linux when the lock file cannot
// be acquired (broken XDG_RUNTIME_DIR, /tmp permissions, fd exhaustion).
// On Linux, acquireRunLock() provides flock-based serialization instead.
//
// When the sysctl override IS active (local Podman on Linux), neither the flock
// nor this mutex is acquired — the override eliminates the race at source.
// Docker never acquires either lock (it doesn't implement SysctlOverrideChecker).
var containerRunMu sync.Mutex

// containerExecPrep holds all prepared data needed to run a container command.
// This struct is returned by prepareContainerExecution and used by both
// Execute and ExecuteCapture to avoid code duplication.
type containerExecPrep struct {
	image          string
	shellCmd       []string
	workDir        string
	env            map[string]string
	volumes        []string
	ports          []string
	extraHosts     []string
	sshConnInfo    *sshserver.ConnectionInfo
	tempScriptPath string
	cleanup        func() // Combined cleanup for provisioning and temp files
}

// prepareContainerExecution performs all common setup for container execution.
// It resolves configuration, prepares the image, builds environment, sets up
// SSH if enabled, and constructs the shell command. Returns a prep struct
// containing all values needed for execution, or an error Result on failure.
//
// Uses a deferred cleanup-on-error pattern: all acquired resources are tracked
// and automatically released if any step fails, so individual error paths don't
// need manual cleanup calls.
func (r *ContainerRuntime) prepareContainerExecution(ctx *ExecutionContext) (_ *containerExecPrep, errResult *Result) {
	// Track resources for cleanup-on-error
	var provisionCleanup func()
	var sshConnInfo *sshserver.ConnectionInfo
	var tempScriptPath string

	defer func() {
		if errResult != nil {
			if tempScriptPath != "" {
				_ = os.Remove(tempScriptPath)
			}
			if sshConnInfo != nil {
				r.sshServer.RevokeToken(sshConnInfo.Token)
			}
			if provisionCleanup != nil {
				provisionCleanup()
			}
		}
	}()

	// Get the container runtime config
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return nil, &Result{ExitCode: 1, Error: fmt.Errorf("runtime config not found for container runtime")}
	}
	containerCfg := containerConfigFromRuntime(rtConfig)
	invowkDir := filepath.Dir(ctx.Invowkfile.FilePath)

	// Validate explicit image policy before provisioning rewrites image tags.
	if containerCfg.Image != "" {
		if err := validateSupportedContainerImage(containerCfg.Image); err != nil {
			return nil, &Result{ExitCode: 1, Error: err}
		}
	}

	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
	if err != nil {
		return nil, &Result{ExitCode: 1, Error: err}
	}

	// Determine the image to use (with provisioning if enabled)
	image, pCleanup, err := r.ensureProvisionedImage(ctx, containerCfg, invowkDir)
	if err != nil {
		return nil, &Result{ExitCode: 1, Error: fmt.Errorf("failed to prepare container image: %w", err)}
	}
	provisionCleanup = pCleanup

	// Build environment
	env, err := r.envBuilder.Build(ctx, invowkfile.EnvInheritNone)
	if err != nil {
		return nil, &Result{ExitCode: 1, Error: fmt.Errorf("failed to build environment: %w", err)}
	}

	// Check if host SSH is enabled for this runtime
	hostSSHEnabled := ctx.SelectedImpl.GetHostSSHForRuntime(ctx.SelectedRuntime)

	// Handle host SSH access if enabled
	if hostSSHEnabled {
		sshConnInfo, err = r.setupSSHConnection(ctx, env)
		if err != nil {
			return nil, &Result{ExitCode: 1, Error: err}
		}
	}

	// Prepare volumes
	volumes := containerCfg.Volumes
	// Always mount the invowkfile directory
	volumes = append(volumes, fmt.Sprintf("%s:/workspace", invowkDir))

	// Resolve interpreter (defaults to "auto" which parses shebang)
	interpInfo := rtConfig.ResolveInterpreterFromScript(script)

	// Build shell command based on interpreter
	var shellCmd []string

	if interpInfo.Found {
		// Use the resolved interpreter
		shellCmd, tempScriptPath, err = r.buildInterpreterCommand(ctx, script, interpInfo, invowkDir)
		if err != nil {
			return nil, &Result{ExitCode: 1, Error: err}
		}
	} else {
		// Use default shell execution
		// We wrap the script in a shell to handle multi-line scripts
		// For POSIX shells: /bin/sh -c 'script' invowk arg1 arg2 ... (args become $1, $2, etc.)
		shellCmd = []string{"/bin/sh", "-c", script}
		if len(ctx.PositionalArgs) > 0 {
			shellCmd = append(shellCmd, "invowk") // $0 placeholder
			shellCmd = append(shellCmd, ctx.PositionalArgs...)
		}
	}

	// Determine working directory using the hierarchical override model
	workDir := r.getContainerWorkDir(ctx, invowkDir)

	// Build extra hosts for SSH server access
	var extraHosts []string
	if hostSSHEnabled && sshConnInfo != nil {
		// Add host gateway for accessing host from container
		extraHosts = append(extraHosts, hostGatewayMapping)
	}

	// Build combined cleanup function (used on success path by the caller)
	cleanup := func() {
		if tempScriptPath != "" {
			_ = os.Remove(tempScriptPath) // Cleanup temp file; error non-critical
		}
		if sshConnInfo != nil {
			r.sshServer.RevokeToken(sshConnInfo.Token)
		}
		if provisionCleanup != nil {
			provisionCleanup()
		}
	}

	// Success: clear errResult so the deferred cleanup doesn't run
	// (errResult is nil by default on success since we return nil for the second value)
	return &containerExecPrep{
		image:          image,
		shellCmd:       shellCmd,
		workDir:        workDir,
		env:            env,
		volumes:        volumes,
		ports:          containerCfg.Ports,
		extraHosts:     extraHosts,
		sshConnInfo:    sshConnInfo,
		tempScriptPath: tempScriptPath,
		cleanup:        cleanup,
	}, nil
}

// runWithRetry wraps engine.Run with retry logic for transient container engine
// errors (rootless Podman ping_group_range race, exit code 125, overlay mount
// races). This mirrors the ensureImage() retry pattern for build operations.
// The caller's context deadline naturally bounds total retry time.
//
// engine.Run() absorbs exec.ExitError into result.ExitCode and returns nil for
// the error return. This means transient OCI failures (e.g., crun ping_group_range
// race returning exit code 126) appear as result.ExitCode != 0 with err == nil.
// The retry logic checks both the error return AND the result exit code.
//
// Stderr is buffered per-attempt so that transient error messages from the
// container engine (e.g., crun writing to the inherited stderr fd before the Go
// process can decide to retry) never leak to the user's terminal. On success or
// non-transient failure the buffer is flushed to the caller's original stderr.
func (r *ContainerRuntime) runWithRetry(ctx context.Context, runOpts container.RunOptions) (*container.RunResult, error) {
	// The ping_group_range race only affects rootless Podman. Serialize runs when
	// the engine implements SysctlOverrideChecker but the override isn't active
	// (podman-remote, non-Linux, temp file failure). Engines that don't implement the
	// checker (Docker) don't suffer from this race and skip serialization entirely.
	//
	// On Linux, acquireRunLock() provides cross-process serialization via flock so
	// that concurrent invowk processes (testscript, parallel terminal invocations)
	// don't race. On non-Linux, flock is unavailable and we fall back to sync.Mutex
	// for intra-process protection only.
	if checker, ok := r.engine.(container.SysctlOverrideChecker); ok && !checker.SysctlOverrideActive() {
		lock, lockErr := acquireRunLock()
		if lockErr != nil {
			if errors.Is(lockErr, errFlockUnavailable) {
				slog.Debug("flock unavailable, falling back to in-process mutex", "error", lockErr)
			} else {
				slog.Warn("flock acquisition failed, falling back to in-process mutex", "error", lockErr)
			}
			containerRunMu.Lock()
			defer containerRunMu.Unlock()
		} else {
			defer lock.Release()
		}
	}

	// Buffer stderr per-attempt so transient error messages from the container
	// engine (written directly to the inherited fd by crun/runc) don't leak to
	// the user's terminal when the retry succeeds.
	originalStderr := runOpts.Stderr

	var lastErr error
	var lastResult *container.RunResult
	var lastStderrBuf *bytes.Buffer
	for attempt := range maxRunRetries {
		if attempt > 0 {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("context cancelled during run retry: %w", err)
			}
			time.Sleep(baseRunBackoff * time.Duration(1<<(attempt-1)))
		}

		var stderrBuf bytes.Buffer
		runOpts.Stderr = &stderrBuf

		result, err := r.engine.Run(ctx, runOpts)
		if err != nil {
			if !container.IsTransientError(err) {
				flushStderr(originalStderr, &stderrBuf)
				return nil, err
			}
			slog.Debug("transient container error, retrying",
				"attempt", attempt+1, "maxRetries", maxRunRetries, "error", err)
			lastErr = err
			lastStderrBuf = &stderrBuf
			continue
		}

		// engine.Run() returns exit-code failures in result rather than err.
		// Check for transient engine exit codes (125 = generic engine error,
		// 126 = OCI runtime failure e.g., crun ping_group_range race).
		if result.ExitCode == 0 || !isTransientExitCode(result.ExitCode) {
			flushStderr(originalStderr, &stderrBuf)
			return result, nil
		}

		slog.Debug("transient container exit code, retrying",
			"attempt", attempt+1, "maxRetries", maxRunRetries, "exitCode", result.ExitCode)
		lastResult = result
		lastStderrBuf = &stderrBuf
	}

	// Flush stderr from the final attempt so the user gets diagnostic output
	// even after all retries are exhausted.
	if lastStderrBuf != nil {
		flushStderr(originalStderr, lastStderrBuf)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return lastResult, nil
}

// flushStderr writes buffered stderr content to the original writer.
// If originalStderr is nil (e.g., caller didn't provide stderr), the buffer
// is silently discarded. Write failures are non-fatal (stderr may be a closed
// pipe or remote connection) and logged at debug level.
func flushStderr(dst io.Writer, src *bytes.Buffer) {
	if dst == nil || src.Len() == 0 {
		return
	}
	if _, err := io.Copy(dst, src); err != nil {
		slog.Debug("failed to flush stderr buffer", "error", err)
	}
}

// isTransientExitCode reports whether a container exit code indicates a transient
// engine error that may succeed on retry. These codes come from the container
// engine (Docker/Podman), not from the user's command inside the container.
//
//   - 125: Generic container engine error (Docker/Podman convention for internal failures)
//   - 126: OCI runtime error (e.g., crun ping_group_range race on rootless Podman)
func isTransientExitCode(code int) bool {
	return code == 125 || code == 126
}

// Execute runs a command in a container through a three-stage pipeline:
//  1. Preparation — resolves the container image, builds the shell command,
//     sets up volumes/ports/env, and registers a cleanup callback.
//  2. Retry — delegates to [ContainerRuntime.runWithRetry], which re-attempts
//     the container run on transient engine errors (exit codes 125/126).
//  3. Result mapping — translates the container engine result into a
//     runtime [Result] with the exit code and any error from the run.
func (r *ContainerRuntime) Execute(ctx *ExecutionContext) *Result {
	prep, errResult := r.prepareContainerExecution(ctx)
	if errResult != nil {
		return errResult
	}
	defer prep.cleanup()

	// Run the container
	runOpts := container.RunOptions{
		Image:       prep.image,
		Command:     prep.shellCmd,
		WorkDir:     prep.workDir,
		Env:         prep.env,
		Volumes:     prep.volumes,
		Ports:       prep.ports,
		Remove:      true, // Always remove after execution
		Stdin:       ctx.IO.Stdin,
		Stdout:      ctx.IO.Stdout,
		Stderr:      ctx.IO.Stderr,
		Interactive: ctx.IO.Stdin != nil,
		ExtraHosts:  prep.extraHosts,
	}

	result, err := r.runWithRetry(ctx.Context, runOpts)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to run container: %w", err)}
	}

	return &Result{
		ExitCode: result.ExitCode,
		Error:    result.Error,
	}
}

// ExecuteCapture runs a command in a container and captures its stdout/stderr.
// This implements the CapturingRuntime interface, enabling container-based
// dependency validation through custom checks that need to capture output.
func (r *ContainerRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	prep, errResult := r.prepareContainerExecution(ctx)
	if errResult != nil {
		return errResult
	}
	defer prep.cleanup()

	// Capture stdout and stderr into buffers
	var stdout, stderr bytes.Buffer

	// Run the container with output capture
	runOpts := container.RunOptions{
		Image:       prep.image,
		Command:     prep.shellCmd,
		WorkDir:     prep.workDir,
		Env:         prep.env,
		Volumes:     prep.volumes,
		Ports:       prep.ports,
		Remove:      true,    // Always remove after execution
		Stdin:       nil,     // No stdin for capture mode
		Stdout:      &stdout, // Capture stdout
		Stderr:      &stderr, // Capture stderr
		Interactive: false,   // Non-interactive for capture mode
		ExtraHosts:  prep.extraHosts,
	}

	result, err := r.runWithRetry(ctx.Context, runOpts)
	if err != nil {
		return &Result{
			ExitCode:  1,
			Error:     fmt.Errorf("failed to run container: %w", err),
			Output:    stdout.String(),
			ErrOutput: stderr.String(),
		}
	}

	return &Result{
		ExitCode:  result.ExitCode,
		Error:     result.Error,
		Output:    stdout.String(),
		ErrOutput: stderr.String(),
	}
}

// setupSSHConnection sets up SSH connection for container host access
func (r *ContainerRuntime) setupSSHConnection(ctx *ExecutionContext, env map[string]string) (*sshserver.ConnectionInfo, error) {
	if r.sshServer == nil {
		return nil, fmt.Errorf("enable_host_ssh is enabled but SSH server is not configured")
	}
	if !r.sshServer.IsRunning() {
		return nil, fmt.Errorf("enable_host_ssh is enabled but SSH server is not running")
	}

	// Generate connection info with a unique token for this command execution
	executionID := ctx.ExecutionID
	if executionID == "" {
		executionID = newExecutionID()
	}
	commandID := fmt.Sprintf("%s-%s", ctx.Command.Name, executionID)
	sshConnInfo, err := r.sshServer.GetConnectionInfo(commandID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SSH credentials: %w", err)
	}

	// Add SSH connection info to environment
	// Use hostDockerInternal for Docker or hostContainersInternal for Podman
	hostAddr := hostDockerInternal
	if r.engine.Name() == "podman" {
		hostAddr = hostContainersInternal
	}

	env["INVOWK_SSH_HOST"] = hostAddr
	env["INVOWK_SSH_PORT"] = fmt.Sprintf("%d", sshConnInfo.Port)
	env["INVOWK_SSH_USER"] = sshConnInfo.User
	env["INVOWK_SSH_TOKEN"] = sshConnInfo.Token
	env["INVOWK_SSH_ENABLED"] = "true"

	return sshConnInfo, nil
}
