// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	appexec "github.com/invowk/invowk/internal/app/execute"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/sshserver"
	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"
	"github.com/invowk/invowk/pkg/invowkfile"

	tea "github.com/charmbracelet/bubbletea"
)

type (
	// commandService is the production CommandService implementation that owns
	// the SSH server lifecycle and delegates to the runtime registry for execution.
	commandService struct {
		config    ConfigProvider
		discovery DiscoveryService
		stdout    io.Writer
		stderr    io.Writer
		ssh       *sshServerController
	}

	// sshServerController provides goroutine-safe SSH server lifecycle management
	// scoped to a single commandService instance. It lazily starts the SSH server
	// on first demand and stops it when the owning command execution completes.
	sshServerController struct {
		mu       sync.Mutex
		instance *sshserver.Server
	}

	// resolvedDefinitions holds the resolved flag/arg definitions and parsed flag values
	// after applying fallbacks from the command's invowkfile definitions.
	resolvedDefinitions struct {
		flagDefs   []invowkfile.Flag
		argDefs    []invowkfile.Argument
		flagValues map[string]string
	}
)

// newCommandService creates the default command orchestration service.
func newCommandService(configProvider ConfigProvider, disc DiscoveryService, stdout, stderr io.Writer) CommandService {
	return &commandService{
		config:    configProvider,
		discovery: disc,
		stdout:    stdout,
		stderr:    stderr,
		ssh:       &sshServerController{},
	}
}

// Execute executes an invowk command through the full orchestration pipeline:
//  1. Loads config and discovers the target command by name.
//  2. Validates inputs: flags, arguments, platform compatibility, and runtime compatibility.
//  3. Manages SSH server lifecycle when the container runtime needs host access.
//  4. Builds execution context with env var projection (INVOWK_FLAG_*, INVOWK_ARG_*, ARGn).
//  5. Propagates incoming context for timeout and cancellation signals.
//  6. Dry-run intercept: if --ivk-dry-run, renders the execution plan and exits.
//  7. Dispatches execution (timeout → dep validation → runtime).
func (s *commandService) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	// Capture the host environment early, before any downstream code could
	// potentially modify it via os.Setenv. Tests can pre-populate req.UserEnv
	// to inject a controlled environment.
	if req.UserEnv == nil {
		req.UserEnv = captureUserEnv()
	}

	cfg, cmdInfo, diags, err := s.discoverCommand(ctx, req)
	if err != nil {
		return ExecuteResult{}, diags, err
	}

	defs := s.resolveDefinitions(req, cmdInfo)

	if validErr := s.validateInputs(req, cmdInfo, defs); validErr != nil {
		return ExecuteResult{}, diags, validErr
	}

	resolved, err := s.resolveRuntime(req, cmdInfo, cfg)
	if err != nil {
		return ExecuteResult{}, diags, err
	}

	// Track whether we are the caller that starts the SSH server so that
	// only this Execute() invocation owns cleanup. If the server is already
	// running when we enter, we skip the defer to avoid premature shutdown.
	sshWasRunning := s.ssh.current() != nil
	if sshErr := s.ensureSSHIfNeeded(ctx, req, resolved); sshErr != nil {
		return ExecuteResult{}, diags, sshErr
	}
	if !sshWasRunning && s.ssh.current() != nil {
		defer s.ssh.stop()
	}

	execCtx, err := s.buildExecContext(req, cmdInfo, defs, resolved)
	if err != nil {
		return ExecuteResult{}, diags, err
	}

	// Propagate the incoming context so that timeout and parent cancellation
	// signals survive. NewExecutionContext (called transitively by buildExecContext)
	// sets context.Background(); overriding it here preserves the Ctrl+C
	// propagation chain and timeout deadline.
	execCtx.Context = ctx

	// Dry-run mode: print what would be executed and exit without executing.
	if req.DryRun {
		renderDryRun(s.stdout, req, cmdInfo, execCtx, resolved)
		return ExecuteResult{ExitCode: 0}, diags, nil
	}

	return s.dispatchExecution(ctx, req, execCtx, cmdInfo, cfg, diags)
}

// discoverCommand loads configuration and discovers the target command by name.
// It returns the config, discovered command info, accumulated diagnostics, and
// any error. It returns a ServiceError with rendering info when the command is not found.
//
// Discovery is routed through DiscoveryService so the per-request cache (attached
// to the context by the RunE handler) is shared across validateCommandTree,
// checkAmbiguousCommand, and this method — avoiding duplicate filesystem scans.
// Config is loaded separately because downstream callers need it for runtime
// registry construction and env builder configuration.
func (s *commandService) discoverCommand(ctx context.Context, req ExecuteRequest) (*config.Config, *discovery.CommandInfo, []discovery.Diagnostic, error) {
	cfg, _ := s.loadConfig(ctx, req.ConfigPath)

	lookup, err := s.discovery.GetCommand(ctx, req.Name)
	diags := slices.Clone(lookup.Diagnostics)
	if err != nil {
		return nil, nil, diags, err
	}

	if lookup.Command == nil {
		return nil, nil, diags, newServiceError(
			fmt.Errorf("command '%s' not found", req.Name),
			issue.CommandNotFoundId,
			"",
		)
	}

	return cfg, lookup.Command, diags, nil
}

// resolveDefinitions resolves flag/arg definitions and flag values by applying
// fallbacks from the command's invowkfile definitions when the request does not
// supply them. This supports both the Cobra-parsed path (defs provided) and the
// direct-call path (only command name + args).
func (s *commandService) resolveDefinitions(req ExecuteRequest, cmdInfo *discovery.CommandInfo) resolvedDefinitions {
	flagDefs := req.FlagDefs
	// Fallback path for requests that only supply command name + args.
	if flagDefs == nil {
		flagDefs = cmdInfo.Command.Flags
	}
	argDefs := req.ArgDefs
	if argDefs == nil {
		argDefs = cmdInfo.Command.Args
	}

	flagValues := req.FlagValues
	// Apply command defaults when the caller did not provide parsed flag values.
	if flagValues == nil && len(flagDefs) > 0 {
		flagValues = make(map[string]string)
		for _, flag := range flagDefs {
			if flag.DefaultValue != "" {
				flagValues[string(flag.Name)] = flag.DefaultValue
			}
		}
	}

	return resolvedDefinitions{
		flagDefs:   flagDefs,
		argDefs:    argDefs,
		flagValues: flagValues,
	}
}

// validateInputs validates flag values, positional arguments, and platform compatibility.
// It returns ServiceError with rendering info for argument validation and platform
// compatibility failures.
func (s *commandService) validateInputs(req ExecuteRequest, cmdInfo *discovery.CommandInfo, defs resolvedDefinitions) error {
	if err := validateFlagValues(req.Name, defs.flagValues, defs.flagDefs); err != nil {
		return err
	}

	if err := validateArguments(req.Name, req.Args, defs.argDefs); err != nil {
		if argErr, ok := errors.AsType[*ArgumentValidationError](err); ok {
			return newServiceError(
				err,
				issue.InvalidArgumentId,
				RenderArgumentValidationError(argErr),
			)
		}
		return err
	}

	currentPlatform := invowkfile.CurrentPlatform()
	if !cmdInfo.Command.CanRunOnCurrentHost() {
		supportedPlatforms := cmdInfo.Command.GetPlatformsString()
		return newServiceError(
			fmt.Errorf("command '%s' does not support platform '%s' (supported: %s)", req.Name, currentPlatform, supportedPlatforms),
			issue.HostNotSupportedId,
			RenderHostNotSupportedError(req.Name, string(currentPlatform), supportedPlatforms),
		)
	}

	return nil
}

// resolveRuntime determines the selected runtime and implementation for the current
// platform using 3-tier precedence:
//  1. CLI flag (--ivk-runtime) — hard override, errors if incompatible.
//  2. Config default runtime (cfg.DefaultRuntime) — soft, silently falls back if incompatible.
//  3. Per-command default — first runtime of the first matching implementation.
//
// It returns ServiceError with rendering info for invalid runtime overrides (Tier 1 only).
func (s *commandService) resolveRuntime(req ExecuteRequest, cmdInfo *discovery.CommandInfo, cfg *config.Config) (appexec.RuntimeSelection, error) {
	selection, err := appexec.ResolveRuntime(cmdInfo.Command, invowkfile.CommandName(req.Name), req.Runtime, cfg, invowkfile.CurrentPlatform())
	if err != nil {
		if notAllowedErr, ok := errors.AsType[*appexec.RuntimeNotAllowedError](err); ok {
			allowed := make([]string, len(notAllowedErr.Allowed))
			for i, r := range notAllowedErr.Allowed {
				allowed[i] = string(r)
			}
			return appexec.RuntimeSelection{}, newServiceError(
				err,
				issue.InvalidRuntimeModeId,
				RenderRuntimeNotAllowedError(req.Name, string(req.Runtime), strings.Join(allowed, ", ")),
			)
		}
		return appexec.RuntimeSelection{}, fmt.Errorf("resolve runtime for '%s': %w", req.Name, err)
	}

	return selection, nil
}

// ensureSSHIfNeeded conditionally starts the SSH server when the selected runtime
// implementation requires host SSH access (used by container runtime for host callbacks).
// Cleanup is handled by the caller (Execute) via a "started-by-me" guard.
func (s *commandService) ensureSSHIfNeeded(ctx context.Context, req ExecuteRequest, resolved appexec.RuntimeSelection) error {
	if !resolved.Impl.GetHostSSHForRuntime(resolved.Mode) {
		return nil
	}

	if err := s.ssh.ensure(ctx); err != nil {
		return fmt.Errorf("failed to start SSH server for host access: %w", err)
	}
	return nil
}

// buildExecContext constructs the runtime execution context from the request,
// discovered command info, resolved definitions, and selected runtime. It projects
// flags and arguments into environment variables following the INVOWK_FLAG_*,
// INVOWK_ARG_*, ARGn, and ARGC conventions.
func (s *commandService) buildExecContext(req ExecuteRequest, cmdInfo *discovery.CommandInfo, defs resolvedDefinitions, resolved appexec.RuntimeSelection) (*runtime.ExecutionContext, error) {
	return appexec.BuildExecutionContext(appexec.BuildExecutionContextOptions{
		Command:         cmdInfo.Command,
		Invowkfile:      cmdInfo.Invowkfile,
		Selection:       resolved,
		Args:            req.Args,
		Verbose:         req.Verbose,
		Workdir:         req.Workdir,
		ForceRebuild:    req.ForceRebuild,
		EnvFiles:        req.EnvFiles,
		EnvVars:         req.EnvVars,
		FlagValues:      defs.flagValues,
		ArgDefs:         defs.argDefs,
		EnvInheritMode:  req.EnvInheritMode,
		EnvInheritAllow: req.EnvInheritAllow,
		EnvInheritDeny:  req.EnvInheritDeny,
		SourceID:        cmdInfo.SourceID,
		Platform:        invowkfile.CurrentPlatform(),
	})
}

// dispatchExecution runs the post-context-build execution pipeline:
//  1. Creates runtime registry.
//  2. Validates timeout string (fail-fast on invalid values).
//  3. Wraps context with timeout.
//  4. Validates dependencies (tools, cmds, filepaths, capabilities, custom checks, env vars).
//  5. Dispatches to interactive mode (alternate screen + TUI server) or standard execution.
//
// It handles result rendering for errors and non-zero exit codes.
func (s *commandService) dispatchExecution(ctx context.Context, req ExecuteRequest, execCtx *runtime.ExecutionContext, cmdInfo *discovery.CommandInfo, cfg *config.Config, diags []discovery.Diagnostic) (ExecuteResult, []discovery.Diagnostic, error) {
	registryResult := createRuntimeRegistry(cfg, s.ssh.current())
	if req.Verbose || execCtx.SelectedRuntime == invowkfile.RuntimeContainer {
		diags = append(diags, registryResult.Diagnostics...)
	}
	defer registryResult.Cleanup()

	// Assign a unique execution ID now that the registry is available.
	// NewExecutionContext leaves ExecutionID empty; it is set here because
	// the registry (which owns the monotonic counter) is created at this point.
	execCtx.ExecutionID = registryResult.Registry.NewExecutionID()

	if registryResult.ContainerInitErr != nil && execCtx.SelectedRuntime == invowkfile.RuntimeContainer {
		issueID, styledMsg := classifyExecutionError(registryResult.ContainerInitErr, req.Verbose)
		return ExecuteResult{}, diags, newServiceError(
			registryResult.ContainerInitErr,
			issueID,
			styledMsg,
		)
	}

	// Validate timeout early so an invalid timeout (e.g., "not-a-duration")
	// fails fast before dependency validation or execution.
	var timeoutDuration time.Duration
	if execCtx.SelectedImpl != nil {
		var parseErr error
		timeoutDuration, parseErr = execCtx.SelectedImpl.ParseTimeout()
		if parseErr != nil {
			return ExecuteResult{}, diags, newServiceError(
				parseErr,
				issue.InvalidArgumentId,
				"",
			)
		}
	}

	// Apply execution timeout. Duration was already validated above
	// (fail-fast on invalid strings). Cancel is deferred to release timer resources.
	if timeoutDuration > 0 {
		var cancel context.CancelFunc
		execCtx.Context, cancel = context.WithTimeout(execCtx.Context, timeoutDuration)
		defer cancel()
	}

	// Dependency validation needs the registry to check runtime-aware dependencies.
	if err := s.validateAndRenderDeps(cmdInfo, execCtx, registryResult.Registry, req.UserEnv); err != nil {
		return ExecuteResult{}, diags, err
	}

	if req.Verbose {
		fmt.Fprintf(s.stdout, "%s Running '%s'...\n", SuccessStyle.Render("→"), req.Name)
	}

	var result *runtime.Result
	if req.Interactive {
		// Interactive mode is opportunistic and falls back to standard execution
		// when the selected runtime does not implement interactive support.
		rt, err := registryResult.Registry.GetForContext(execCtx)
		if err != nil {
			err = fmt.Errorf("failed to get runtime: %w", err)
			issueID, styledMsg := classifyExecutionError(err, req.Verbose)
			return ExecuteResult{}, diags, newServiceError(
				err,
				issueID,
				styledMsg,
			)
		}

		interactiveRT := runtime.GetInteractiveRuntime(rt)
		if interactiveRT != nil {
			result = executeInteractive(execCtx, registryResult.Registry, req.Name, interactiveRT)
		} else {
			if req.Verbose {
				fmt.Fprintf(s.stdout, "%s Runtime '%s' does not support interactive mode, using standard execution\n",
					WarningStyle.Render("!"), rt.Name())
			}
			result = registryResult.Registry.Execute(execCtx)
		}
	} else {
		result = registryResult.Registry.Execute(execCtx)
	}

	if result.Error != nil {
		issueID, styledMsg := classifyExecutionError(result.Error, req.Verbose)
		return ExecuteResult{}, diags, newServiceError(
			result.Error,
			issueID,
			styledMsg,
		)
	}

	return ExecuteResult{ExitCode: int(result.ExitCode)}, diags, nil
}

// validateAndRenderDeps validates command dependencies and returns ServiceError
// for dependency failures. Discovery is routed through s.discovery so the
// per-request cache avoids redundant filesystem scans.
func (s *commandService) validateAndRenderDeps(cmdInfo *discovery.CommandInfo, execCtx *runtime.ExecutionContext, registry *runtime.Registry, userEnv map[string]string) error {
	if err := validateDependencies(s.discovery, cmdInfo, registry, execCtx, userEnv); err != nil {
		if depErr, ok := errors.AsType[*DependencyError](err); ok {
			return newServiceError(
				err,
				issue.DependenciesNotSatisfiedId,
				RenderDependencyError(depErr),
			)
		}
		return err
	}
	return nil
}

func (s *commandService) loadConfig(ctx context.Context, configPath string) (*config.Config, []discovery.Diagnostic) {
	return loadConfigWithFallback(ctx, s.config, configPath)
}

// ensure lazily starts the SSH server if not already running. It blocks until
// the server is ready to accept connections. The server is reused across
// multiple calls within the same command execution. The started server is
// stored internally and accessed via current().
func (s *sshServerController) ensure(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.instance != nil && s.instance.IsRunning() {
		return nil
	}

	// Start blocks until SSH server is ready to accept connections.
	srv := sshserver.New(sshserver.DefaultConfig())
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("failed to start SSH server: %w", err)
	}

	s.instance = srv
	return nil
}

// stop shuts down the SSH server if running. This is a best-effort operation
// called via defer after command execution completes.
func (s *sshServerController) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.instance != nil {
		// Best-effort shutdown: execution already completed at this point.
		if err := s.instance.Stop(); err != nil {
			slog.Debug("SSH server stop failed during cleanup", "error", err)
		}
		s.instance = nil
	}
}

// current returns the active SSH server instance, or nil if not started.
// Used to pass the server reference to the container runtime for host access.
func (s *sshServerController) current() *sshserver.Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.instance
}

// executeInteractive runs a command in interactive mode using Bubble Tea's alternate
// screen buffer. It starts an HTTP-based TUI server for bidirectional component requests
// between the running command and the terminal UI. For container runtimes, the TUI server
// URL is rewritten to use the host-reachable address so containers can call back.
func executeInteractive(ctx *runtime.ExecutionContext, registry *runtime.Registry, cmdName string, interactiveRT runtime.InteractiveRuntime) *runtime.Result {
	if err := interactiveRT.Validate(ctx); err != nil {
		return &runtime.Result{ExitCode: 1, Error: err}
	}

	tuiServer, err := tuiserver.New()
	if err != nil {
		return &runtime.Result{ExitCode: 1, Error: fmt.Errorf("failed to create TUI server: %w", err)}
	}

	if err = tuiServer.Start(context.Background()); err != nil {
		return &runtime.Result{ExitCode: 1, Error: fmt.Errorf("failed to start TUI server: %w", err)}
	}
	defer func() {
		if stopErr := tuiServer.Stop(); stopErr != nil {
			slog.Debug("TUI server stop failed during cleanup", "error", stopErr)
		}
	}()

	// Rewrite the TUI server URL for container runtimes. The TUI server
	// listens on the host's localhost, but a container's network namespace
	// isolates it from the host — "localhost" inside the container refers to
	// the container itself, not the host. We replace localhost with the
	// engine-specific host-reachable address (e.g., "host.docker.internal"
	// for Docker, or the host gateway IP for Podman) so the containerized
	// command can call back to the TUI server over the bridge network.
	var tuiServerURL string
	if containerRT, ok := interactiveRT.(*runtime.ContainerRuntime); ok {
		hostAddr := containerRT.GetHostAddressForContainer()
		tuiServerURL = tuiServer.URLWithHost(hostAddr)
	} else {
		tuiServerURL = tuiServer.URL()
	}

	ctx.TUI.ServerURL = tuiServerURL
	ctx.TUI.ServerToken = tuiServer.Token()

	prepared, err := interactiveRT.PrepareInteractive(ctx)
	if err != nil {
		return &runtime.Result{ExitCode: 1, Error: fmt.Errorf("failed to prepare command: %w", err)}
	}

	if prepared.Cleanup != nil {
		defer prepared.Cleanup()
	}

	prepared.Cmd.Env = append(prepared.Cmd.Env,
		// Native/virtual runtimes read TUI server coordinates from process env.
		fmt.Sprintf("%s=%s", tuiserver.EnvTUIAddr, tuiServerURL),
		fmt.Sprintf("%s=%s", tuiserver.EnvTUIToken, tuiServer.Token()),
	)

	execCtx := ctx.Context
	if execCtx == nil {
		execCtx = context.Background()
	}

	interactiveResult, err := tui.RunInteractiveCmd(
		execCtx,
		tui.InteractiveOptions{
			Title:       "Running Command",
			CommandName: cmdName,
			OnProgramReady: func(p *tea.Program) {
				go bridgeTUIRequests(tuiServer, p)
			},
		},
		prepared.Cmd,
	)
	if err != nil {
		return &runtime.Result{ExitCode: 1, Error: fmt.Errorf("interactive execution failed: %w", err)}
	}

	return &runtime.Result{
		ExitCode: runtime.ExitCode(interactiveResult.ExitCode),
		Error:    interactiveResult.Error,
	}
}
