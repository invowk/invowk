// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/issue"
	"invowk-cli/internal/runtime"
	"invowk-cli/internal/sshserver"
	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"
	"invowk-cli/pkg/invkfile"

	tea "github.com/charmbracelet/bubbletea"
)

type (
	// commandService is the production CommandService implementation that owns
	// the SSH server lifecycle and delegates to the runtime registry for execution.
	commandService struct {
		config ConfigProvider
		stdout io.Writer
		stderr io.Writer
		ssh    *sshServerController
	}

	// sshServerController provides goroutine-safe SSH server lifecycle management
	// scoped to a single commandService instance. It lazily starts the SSH server
	// on first demand and stops it when the owning command execution completes.
	sshServerController struct {
		mu       sync.Mutex
		instance *sshserver.Server
	}

	// resolvedDefinitions holds the resolved flag/arg definitions and parsed flag values
	// after applying fallbacks from the command's invkfile definitions.
	resolvedDefinitions struct {
		flagDefs   []invkfile.Flag
		argDefs    []invkfile.Argument
		flagValues map[string]string
	}

	// resolvedRuntime holds the runtime selection result after validating platform
	// compatibility and applying any --runtime override.
	resolvedRuntime struct {
		mode invkfile.RuntimeMode
		impl *invkfile.Implementation
	}
)

// newCommandService creates the default command orchestration service.
func newCommandService(configProvider ConfigProvider, stdout, stderr io.Writer) CommandService {
	return &commandService{
		config: configProvider,
		stdout: stdout,
		stderr: stderr,
		ssh:    &sshServerController{},
	}
}

// Execute executes an invowk command through the full orchestration pipeline:
//  1. Loads config and discovers the target command by name.
//  2. Validates inputs: flags, arguments, platform compatibility, and runtime compatibility.
//  3. Manages SSH server lifecycle when the container runtime needs host access.
//  4. Builds execution context with env var projection (INVOWK_FLAG_*, INVOWK_ARG_*, ARGn).
//  5. Validates command dependencies against the runtime registry.
//  6. Dispatches to interactive mode (alternate screen + TUI server) or standard execution.
func (s *commandService) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	cfg, cmdInfo, diags, err := s.discoverCommand(ctx, req)
	if err != nil {
		return ExecuteResult{}, diags, err
	}

	defs := s.resolveDefinitions(req, cmdInfo)

	if validErr := s.validateInputs(req, cmdInfo, defs); validErr != nil {
		return ExecuteResult{}, diags, validErr
	}

	resolved, err := s.resolveRuntime(req, cmdInfo)
	if err != nil {
		return ExecuteResult{}, diags, err
	}

	if sshErr := s.ensureSSHIfNeeded(ctx, req, resolved); sshErr != nil {
		return ExecuteResult{}, diags, sshErr
	}

	execCtx, err := s.buildExecContext(req, cmdInfo, defs, resolved)
	if err != nil {
		return ExecuteResult{}, diags, err
	}

	return s.dispatchExecution(req, execCtx, cmdInfo, cfg, diags)
}

// discoverCommand loads configuration and discovers the target command by name.
// It returns the config, discovered command info, accumulated diagnostics, and
// any error. If the command is not found, it renders a styled error to stderr.
func (s *commandService) discoverCommand(ctx context.Context, req ExecuteRequest) (*config.Config, *discovery.CommandInfo, []discovery.Diagnostic, error) {
	cfg, diags := s.loadConfig(ctx, req.ConfigPath)

	disc := discovery.New(cfg)
	lookup, err := disc.GetCommand(ctx, req.Name)
	if err != nil {
		return nil, nil, diags, err
	}
	diags = append(diags, lookup.Diagnostics...)

	if lookup.Command == nil {
		rendered, _ := issue.Get(issue.CommandNotFoundId).Render("dark")
		fmt.Fprint(s.stderr, rendered)
		return nil, nil, diags, fmt.Errorf("command '%s' not found", req.Name)
	}

	return cfg, lookup.Command, diags, nil
}

// resolveDefinitions resolves flag/arg definitions and flag values by applying
// fallbacks from the command's invkfile definitions when the request does not
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
				flagValues[flag.Name] = flag.DefaultValue
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
// It renders styled errors to stderr for argument validation and platform compatibility
// failures before returning the error.
func (s *commandService) validateInputs(req ExecuteRequest, cmdInfo *discovery.CommandInfo, defs resolvedDefinitions) error {
	if err := validateFlagValues(req.Name, defs.flagValues, defs.flagDefs); err != nil {
		return err
	}

	if err := validateArguments(req.Name, req.Args, defs.argDefs); err != nil {
		if argErr, ok := errors.AsType[*ArgumentValidationError](err); ok {
			fmt.Fprint(s.stderr, RenderArgumentValidationError(argErr))
			rendered, _ := issue.Get(issue.InvalidArgumentId).Render("dark")
			fmt.Fprint(s.stderr, rendered)
		}
		return err
	}

	currentPlatform := invkfile.GetCurrentHostOS()
	if !cmdInfo.Command.CanRunOnCurrentHost() {
		supportedPlatforms := cmdInfo.Command.GetPlatformsString()
		fmt.Fprint(s.stderr, RenderHostNotSupportedError(req.Name, string(currentPlatform), supportedPlatforms))
		rendered, _ := issue.Get(issue.HostNotSupportedId).Render("dark")
		fmt.Fprint(s.stderr, rendered)
		return fmt.Errorf("command '%s' does not support platform '%s' (supported: %s)", req.Name, currentPlatform, supportedPlatforms)
	}

	return nil
}

// resolveRuntime determines the selected runtime and implementation for the current
// platform. It validates any --runtime override against the command's compatibility
// matrix and renders styled errors for invalid runtime overrides.
func (s *commandService) resolveRuntime(req ExecuteRequest, cmdInfo *discovery.CommandInfo) (resolvedRuntime, error) {
	currentPlatform := invkfile.GetCurrentHostOS()
	selectedRuntime := cmdInfo.Command.GetDefaultRuntimeForPlatform(currentPlatform)

	if req.Runtime != "" {
		// Runtime override must still respect command/runtime compatibility matrix.
		overrideRuntime := invkfile.RuntimeMode(req.Runtime)
		if !cmdInfo.Command.IsRuntimeAllowedForPlatform(currentPlatform, overrideRuntime) {
			allowedRuntimes := cmdInfo.Command.GetAllowedRuntimesForPlatform(currentPlatform)
			allowedStr := make([]string, len(allowedRuntimes))
			for i, r := range allowedRuntimes {
				allowedStr[i] = string(r)
			}
			fmt.Fprint(s.stderr, RenderRuntimeNotAllowedError(req.Name, req.Runtime, strings.Join(allowedStr, ", ")))
			rendered, _ := issue.Get(issue.InvalidRuntimeModeId).Render("dark")
			fmt.Fprint(s.stderr, rendered)
			return resolvedRuntime{}, fmt.Errorf("runtime '%s' is not allowed for command '%s' on platform '%s' (allowed: %s)", req.Runtime, req.Name, currentPlatform, strings.Join(allowedStr, ", "))
		}
		selectedRuntime = overrideRuntime
	}

	impl := cmdInfo.Command.GetImplForPlatformRuntime(currentPlatform, selectedRuntime)
	if impl == nil {
		return resolvedRuntime{}, fmt.Errorf("no implementation found for command '%s' on platform '%s' with runtime '%s'", req.Name, currentPlatform, selectedRuntime)
	}

	return resolvedRuntime{mode: selectedRuntime, impl: impl}, nil
}

// ensureSSHIfNeeded conditionally starts the SSH server when the selected runtime
// implementation requires host SSH access (used by container runtime for host callbacks).
// If started, the SSH server is stopped via defer when Execute returns.
func (s *commandService) ensureSSHIfNeeded(ctx context.Context, req ExecuteRequest, resolved resolvedRuntime) error {
	if !resolved.impl.GetHostSSHForRuntime(resolved.mode) {
		return nil
	}

	// Host SSH lifecycle is service-scoped, not package-global state.
	srv, err := s.ssh.ensure(ctx)
	if err != nil {
		return fmt.Errorf("failed to start SSH server for host access: %w", err)
	}
	if req.Verbose {
		fmt.Fprintf(s.stdout, "%s SSH server started on %s for host access\n", SuccessStyle.Render("→"), srv.Address())
	}
	// Defer cleanup in the caller (Execute) is handled by the SSH controller's
	// stop method being called when the service is done. Since ensureSSHIfNeeded
	// is called from Execute, the defer in Execute's scope handles this.
	return nil
}

// buildExecContext constructs the runtime execution context from the request,
// discovered command info, resolved definitions, and selected runtime. It projects
// flags and arguments into environment variables following the INVOWK_FLAG_*,
// INVOWK_ARG_*, ARGn, and ARGC conventions.
func (s *commandService) buildExecContext(req ExecuteRequest, cmdInfo *discovery.CommandInfo, defs resolvedDefinitions, resolved resolvedRuntime) (*runtime.ExecutionContext, error) {
	execCtx := runtime.NewExecutionContext(cmdInfo.Command, cmdInfo.Invkfile)
	// Request fields are projected into runtime execution context.
	execCtx.Verbose = req.Verbose
	execCtx.SelectedRuntime = resolved.mode
	execCtx.SelectedImpl = resolved.impl
	execCtx.PositionalArgs = req.Args
	execCtx.WorkDir = req.Workdir
	execCtx.ForceRebuild = req.ForceRebuild

	execCtx.Env.RuntimeEnvFiles = req.EnvFiles
	execCtx.Env.RuntimeEnvVars = req.EnvVars

	if err := s.applyEnvInheritOverrides(req, execCtx); err != nil {
		return nil, err
	}

	s.projectEnvVars(req, defs, execCtx)

	return execCtx, nil
}

// applyEnvInheritOverrides validates and applies env inheritance overrides from the
// request (--env-inherit-mode, --env-inherit-allow, --env-inherit-deny) onto the
// execution context.
func (s *commandService) applyEnvInheritOverrides(req ExecuteRequest, execCtx *runtime.ExecutionContext) error {
	if req.EnvInheritMode != "" {
		mode, err := invkfile.ParseEnvInheritMode(req.EnvInheritMode)
		if err != nil {
			return err
		}
		execCtx.Env.InheritModeOverride = mode
	}

	for _, name := range req.EnvInheritAllow {
		if err := invkfile.ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("env-inherit-allow: %w", err)
		}
	}
	if req.EnvInheritAllow != nil {
		execCtx.Env.InheritAllowOverride = req.EnvInheritAllow
	}

	for _, name := range req.EnvInheritDeny {
		if err := invkfile.ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("env-inherit-deny: %w", err)
		}
	}
	if req.EnvInheritDeny != nil {
		execCtx.Env.InheritDenyOverride = req.EnvInheritDeny
	}

	return nil
}

// projectEnvVars projects positional arguments and flag values into the execution
// context's extra environment variables. This includes ARG1..ARGn, ARGC,
// INVOWK_ARG_* (with variadic support), and INVOWK_FLAG_* variables.
func (s *commandService) projectEnvVars(req ExecuteRequest, defs resolvedDefinitions, execCtx *runtime.ExecutionContext) {
	for i, arg := range req.Args {
		execCtx.Env.ExtraEnv[fmt.Sprintf("ARG%d", i+1)] = arg
	}
	execCtx.Env.ExtraEnv["ARGC"] = fmt.Sprintf("%d", len(req.Args))

	// Inject INVOWK_ARG_* variables for structured argument access in scripts.
	if len(defs.argDefs) > 0 {
		for i, argDef := range defs.argDefs {
			envName := ArgNameToEnvVar(argDef.Name)

			switch {
			case argDef.Variadic:
				var variadicValues []string
				if i < len(req.Args) {
					variadicValues = req.Args[i:]
				}

				execCtx.Env.ExtraEnv[envName+"_COUNT"] = fmt.Sprintf("%d", len(variadicValues))
				for j, val := range variadicValues {
					execCtx.Env.ExtraEnv[fmt.Sprintf("%s_%d", envName, j+1)] = val
				}
				execCtx.Env.ExtraEnv[envName] = strings.Join(variadicValues, " ")
			case i < len(req.Args):
				execCtx.Env.ExtraEnv[envName] = req.Args[i]
			case argDef.DefaultValue != "":
				execCtx.Env.ExtraEnv[envName] = argDef.DefaultValue
			}
		}
	}

	for name, value := range defs.flagValues {
		envName := FlagNameToEnvVar(name)
		execCtx.Env.ExtraEnv[envName] = value
	}
}

// dispatchExecution validates dependencies, then dispatches the command to either
// interactive mode (alternate screen + TUI server) or standard execution. It handles
// result rendering for errors and non-zero exit codes.
func (s *commandService) dispatchExecution(req ExecuteRequest, execCtx *runtime.ExecutionContext, cmdInfo *discovery.CommandInfo, cfg *config.Config, diags []discovery.Diagnostic) (ExecuteResult, []discovery.Diagnostic, error) {
	registry := createRuntimeRegistry(cfg, s.ssh.current())

	// Dependency validation needs the registry to check runtime-aware dependencies.
	if err := s.validateAndRenderDeps(cfg, cmdInfo, execCtx, registry); err != nil {
		return ExecuteResult{}, diags, err
	}

	if req.Verbose {
		fmt.Fprintf(s.stdout, "%s Running '%s'...\n", SuccessStyle.Render("→"), req.Name)
	}

	// SSH server stop is deferred here because this is the last step before execution,
	// and the SSH server must remain running during command execution.
	if s.ssh.current() != nil {
		defer s.ssh.stop()
	}

	var result *runtime.Result
	if req.Interactive {
		// Interactive mode is opportunistic and falls back to standard execution
		// when the selected runtime does not implement interactive support.
		rt, err := registry.GetForContext(execCtx)
		if err != nil {
			return ExecuteResult{}, diags, fmt.Errorf("failed to get runtime: %w", err)
		}

		interactiveRT := runtime.GetInteractiveRuntime(rt)
		if interactiveRT != nil {
			result = executeInteractive(execCtx, registry, req.Name, interactiveRT)
		} else {
			if req.Verbose {
				fmt.Fprintf(s.stdout, "%s Runtime '%s' does not support interactive mode, using standard execution\n",
					WarningStyle.Render("!"), rt.Name())
			}
			result = registry.Execute(execCtx)
		}
	} else {
		result = registry.Execute(execCtx)
	}

	if result.Error != nil {
		rendered, _ := issue.Get(issue.ScriptExecutionFailedId).Render("dark")
		fmt.Fprint(s.stderr, rendered)
		fmt.Fprintf(s.stderr, "\n%s %v\n", ErrorStyle.Render("Error:"), result.Error)
		return ExecuteResult{}, diags, result.Error
	}

	if result.ExitCode != 0 {
		if req.Verbose {
			fmt.Fprintf(s.stdout, "%s Command exited with code %d\n", WarningStyle.Render("!"), result.ExitCode)
		}
		return ExecuteResult{ExitCode: result.ExitCode}, diags, nil
	}

	return ExecuteResult{ExitCode: 0}, diags, nil
}

// validateAndRenderDeps validates command dependencies and renders styled errors
// for dependency failures.
func (s *commandService) validateAndRenderDeps(cfg *config.Config, cmdInfo *discovery.CommandInfo, execCtx *runtime.ExecutionContext, registry *runtime.Registry) error {
	if err := validateDependencies(cfg, cmdInfo, registry, execCtx); err != nil {
		if depErr, ok := errors.AsType[*DependencyError](err); ok {
			fmt.Fprint(s.stderr, RenderDependencyError(depErr))
			rendered, _ := issue.Get(issue.DependenciesNotSatisfiedId).Render("dark")
			fmt.Fprint(s.stderr, rendered)
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
// multiple calls within the same command execution.
func (s *sshServerController) ensure(ctx context.Context) (*sshserver.Server, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.instance != nil && s.instance.IsRunning() {
		return s.instance, nil
	}

	// Start blocks until SSH server is ready to accept connections.
	srv := sshserver.New(sshserver.DefaultConfig())
	if err := srv.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start SSH server: %w", err)
	}

	s.instance = srv
	return srv, nil
}

// stop shuts down the SSH server if running. This is a best-effort operation
// called via defer after command execution completes.
func (s *sshServerController) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.instance != nil {
		// Best-effort shutdown: execution already completed at this point.
		_ = s.instance.Stop()
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
	defer func() { _ = tuiServer.Stop() }() // Best-effort cleanup.

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
		ExitCode: interactiveResult.ExitCode,
		Error:    interactiveResult.Error,
	}
}
