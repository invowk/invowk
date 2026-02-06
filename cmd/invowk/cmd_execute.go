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
	commandService struct {
		config ConfigProvider
		stdout io.Writer
		stderr io.Writer
		ssh    *sshServerController
	}

	sshServerController struct {
		mu       sync.Mutex
		instance *sshserver.Server
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

// Execute executes an invowk command.
func (s *commandService) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	cfg, diags := s.loadConfig(ctx, req.ConfigPath)

	disc := discovery.New(cfg)
	lookup, err := disc.GetCommand(ctx, req.Name)
	if err != nil {
		return ExecuteResult{}, diags, err
	}
	diags = append(diags, lookup.Diagnostics...)

	if lookup.Command == nil {
		rendered, _ := issue.Get(issue.CommandNotFoundId).Render("dark")
		fmt.Fprint(s.stderr, rendered)
		return ExecuteResult{}, diags, fmt.Errorf("command '%s' not found", req.Name)
	}

	cmdInfo := lookup.Command

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

	if err := validateFlagValues(req.Name, flagValues, flagDefs); err != nil {
		return ExecuteResult{}, diags, err
	}

	if err := validateArguments(req.Name, req.Args, argDefs); err != nil {
		argErr := (*ArgumentValidationError)(nil)
		if errors.As(err, &argErr) {
			fmt.Fprint(s.stderr, RenderArgumentValidationError(argErr))
			rendered, _ := issue.Get(issue.InvalidArgumentId).Render("dark")
			fmt.Fprint(s.stderr, rendered)
		}
		return ExecuteResult{}, diags, err
	}

	currentPlatform := invkfile.GetCurrentHostOS()
	if !cmdInfo.Command.CanRunOnCurrentHost() {
		supportedPlatforms := cmdInfo.Command.GetPlatformsString()
		fmt.Fprint(s.stderr, RenderHostNotSupportedError(req.Name, string(currentPlatform), supportedPlatforms))
		rendered, _ := issue.Get(issue.HostNotSupportedId).Render("dark")
		fmt.Fprint(s.stderr, rendered)
		return ExecuteResult{}, diags, fmt.Errorf("command '%s' does not support platform '%s' (supported: %s)", req.Name, currentPlatform, supportedPlatforms)
	}

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
			return ExecuteResult{}, diags, fmt.Errorf("runtime '%s' is not allowed for command '%s' on platform '%s' (allowed: %s)", req.Runtime, req.Name, currentPlatform, strings.Join(allowedStr, ", "))
		}
		selectedRuntime = overrideRuntime
	}

	script := cmdInfo.Command.GetImplForPlatformRuntime(currentPlatform, selectedRuntime)
	if script == nil {
		return ExecuteResult{}, diags, fmt.Errorf("no script found for command '%s' on platform '%s' with runtime '%s'", req.Name, currentPlatform, selectedRuntime)
	}

	if script.GetHostSSHForRuntime(selectedRuntime) {
		// Host SSH lifecycle is service-scoped, not package-global state.
		srv, err := s.ssh.ensure(ctx)
		if err != nil {
			return ExecuteResult{}, diags, fmt.Errorf("failed to start SSH server for host access: %w", err)
		}
		if req.Verbose {
			fmt.Fprintf(s.stdout, "%s SSH server started on %s for host access\n", SuccessStyle.Render("→"), srv.Address())
		}
		defer s.ssh.stop()
	}

	execCtx := runtime.NewExecutionContext(cmdInfo.Command, cmdInfo.Invkfile)
	// Request fields are projected into runtime execution context.
	execCtx.Verbose = req.Verbose
	execCtx.SelectedRuntime = selectedRuntime
	execCtx.SelectedImpl = script
	execCtx.PositionalArgs = req.Args
	execCtx.WorkDir = req.Workdir
	execCtx.ForceRebuild = req.ForceRebuild

	execCtx.Env.RuntimeEnvFiles = req.EnvFiles
	execCtx.Env.RuntimeEnvVars = req.EnvVars

	if req.EnvInheritMode != "" {
		mode, err := invkfile.ParseEnvInheritMode(req.EnvInheritMode)
		if err != nil {
			return ExecuteResult{}, diags, err
		}
		execCtx.Env.InheritModeOverride = mode
	}

	for _, name := range req.EnvInheritAllow {
		if err := invkfile.ValidateEnvVarName(name); err != nil {
			return ExecuteResult{}, diags, fmt.Errorf("env-inherit-allow: %w", err)
		}
	}
	if req.EnvInheritAllow != nil {
		execCtx.Env.InheritAllowOverride = req.EnvInheritAllow
	}

	for _, name := range req.EnvInheritDeny {
		if err := invkfile.ValidateEnvVarName(name); err != nil {
			return ExecuteResult{}, diags, fmt.Errorf("env-inherit-deny: %w", err)
		}
	}
	if req.EnvInheritDeny != nil {
		execCtx.Env.InheritDenyOverride = req.EnvInheritDeny
	}

	registry := createRuntimeRegistry(cfg, s.ssh.current())

	if err := validateDependencies(cfg, cmdInfo, registry, execCtx); err != nil {
		depErr := (*DependencyError)(nil)
		if errors.As(err, &depErr) {
			fmt.Fprint(s.stderr, RenderDependencyError(depErr))
			rendered, _ := issue.Get(issue.DependenciesNotSatisfiedId).Render("dark")
			fmt.Fprint(s.stderr, rendered)
		}
		return ExecuteResult{}, diags, err
	}

	if req.Verbose {
		fmt.Fprintf(s.stdout, "%s Running '%s'...\n", SuccessStyle.Render("→"), req.Name)
	}

	for i, arg := range req.Args {
		execCtx.Env.ExtraEnv[fmt.Sprintf("ARG%d", i+1)] = arg
	}
	execCtx.Env.ExtraEnv["ARGC"] = fmt.Sprintf("%d", len(req.Args))

	// Inject INVOWK_ARG_* variables for structured argument access in scripts.
	if len(argDefs) > 0 {
		for i, argDef := range argDefs {
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

	for name, value := range flagValues {
		envName := FlagNameToEnvVar(name)
		execCtx.Env.ExtraEnv[envName] = value
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

func (s *commandService) loadConfig(ctx context.Context, configPath string) (*config.Config, []discovery.Diagnostic) {
	cfg, err := s.config.Load(ctx, config.LoadOptions{ConfigFilePath: configPath})
	if err == nil {
		return cfg, nil
	}

	// Keep execution usable with defaults while surfacing a structured warning.
	return config.DefaultConfig(), []discovery.Diagnostic{{
		Severity: discovery.SeverityWarning,
		Code:     "config_load_failed",
		Message:  fmt.Sprintf("failed to load config, using defaults: %v", err),
		Path:     configPath,
		Cause:    err,
	}}
}

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

func (s *sshServerController) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.instance != nil {
		// Best-effort shutdown: execution already completed at this point.
		_ = s.instance.Stop()
		s.instance = nil
	}
}

func (s *sshServerController) current() *sshserver.Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.instance
}

// executeInteractive runs a command in interactive mode using an alternate screen buffer.
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
