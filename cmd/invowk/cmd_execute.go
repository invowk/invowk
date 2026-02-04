// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/issue"
	"invowk-cli/internal/runtime"
	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"
	"invowk-cli/pkg/invkfile"

	tea "github.com/charmbracelet/bubbletea"
)

// runCommandWithFlags executes a command with the given flag values.
// flagValues is a map of flag name -> value.
// flagDefs contains the flag definitions for runtime validation (can be nil for legacy calls).
// argDefs contains the argument definitions for setting INVOWK_ARG_* env vars (can be nil for legacy calls).
// runtimeEnvFiles contains paths to env files specified via --env-file flag.
// runtimeEnvVars contains env vars specified via --env-var flag (KEY=VALUE pairs, highest precedence).
// workdirOverride is the CLI override for working directory (--workdir flag, empty means no override).
// envInheritModeOverride controls host env inheritance (empty means use runtime config/default).
// envInheritAllowOverride and envInheritDenyOverride override runtime config allow/deny lists when provided.
func runCommandWithFlags(cmdName string, args []string, flagValues map[string]string, flagDefs []invkfile.Flag, argDefs []invkfile.Argument, runtimeEnvFiles []string, runtimeEnvVars map[string]string, workdirOverride, envInheritModeOverride string, envInheritAllowOverride, envInheritDenyOverride []string) error {
	cfg := config.Get()
	disc := discovery.New(cfg)

	// Find the command
	cmdInfo, err := disc.GetCommand(cmdName)
	if err != nil {
		rendered, _ := issue.Get(issue.CommandNotFoundId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return fmt.Errorf("command '%s' not found", cmdName)
	}

	// Populate definitions from discovered command if not provided (fallback path).
	// This enables validation and INVOWK_ARG_* / INVOWK_FLAG_* env var injection
	// for commands invoked via runCommand (which passes nil for definitions).
	if flagDefs == nil {
		flagDefs = cmdInfo.Command.Flags
	}
	if argDefs == nil {
		argDefs = cmdInfo.Command.Args
	}

	// Initialize flagValues with defaults for fallback path.
	// The fallback path cannot parse flags from CLI (Cobra doesn't process them),
	// so we only apply defaults here.
	if flagValues == nil && len(flagDefs) > 0 {
		flagValues = make(map[string]string)
		for _, flag := range flagDefs {
			if flag.DefaultValue != "" {
				flagValues[flag.Name] = flag.DefaultValue
			}
		}
	}

	// Validate flag values at runtime
	if err := validateFlagValues(cmdName, flagValues, flagDefs); err != nil {
		return err
	}

	// Validate arguments
	if err := validateArguments(cmdName, args, argDefs); err != nil {
		var argErr *ArgumentValidationError
		if errors.As(err, &argErr) {
			fmt.Fprint(os.Stderr, RenderArgumentValidationError(argErr))
			rendered, _ := issue.Get(issue.InvalidArgumentId).Render("dark")
			fmt.Fprint(os.Stderr, rendered)
		}
		return err
	}

	// Get the current platform
	currentPlatform := invkfile.GetCurrentHostOS()

	// Validate host OS compatibility
	if !cmdInfo.Command.CanRunOnCurrentHost() {
		supportedPlatforms := cmdInfo.Command.GetPlatformsString()
		fmt.Fprint(os.Stderr, RenderHostNotSupportedError(cmdName, string(currentPlatform), supportedPlatforms))
		rendered, _ := issue.Get(issue.HostNotSupportedId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return fmt.Errorf("command '%s' does not support platform '%s' (supported: %s)", cmdName, currentPlatform, supportedPlatforms)
	}

	// Determine which runtime to use
	var selectedRuntime invkfile.RuntimeMode
	if runtimeOverride != "" {
		// Validate that the overridden runtime is allowed for this platform
		overrideRuntime := invkfile.RuntimeMode(runtimeOverride)
		if !cmdInfo.Command.IsRuntimeAllowedForPlatform(currentPlatform, overrideRuntime) {
			allowedRuntimes := cmdInfo.Command.GetAllowedRuntimesForPlatform(currentPlatform)
			allowedStr := make([]string, len(allowedRuntimes))
			for i, r := range allowedRuntimes {
				allowedStr[i] = string(r)
			}
			fmt.Fprint(os.Stderr, RenderRuntimeNotAllowedError(cmdName, runtimeOverride, strings.Join(allowedStr, ", ")))
			rendered, _ := issue.Get(issue.InvalidRuntimeModeId).Render("dark")
			fmt.Fprint(os.Stderr, rendered)
			return fmt.Errorf("runtime '%s' is not allowed for command '%s' on platform '%s' (allowed: %s)", runtimeOverride, cmdName, currentPlatform, strings.Join(allowedStr, ", "))
		}
		selectedRuntime = overrideRuntime
	} else {
		// Use the default runtime for this platform
		selectedRuntime = cmdInfo.Command.GetDefaultRuntimeForPlatform(currentPlatform)
	}

	// Find the matching script
	script := cmdInfo.Command.GetImplForPlatformRuntime(currentPlatform, selectedRuntime)
	if script == nil {
		return fmt.Errorf("no script found for command '%s' on platform '%s' with runtime '%s'", cmdName, currentPlatform, selectedRuntime)
	}

	// Start SSH server if enable_host_ssh is enabled for this script and runtime
	if script.GetHostSSHForRuntime(selectedRuntime) {
		srv, err := ensureSSHServer()
		if err != nil {
			return fmt.Errorf("failed to start SSH server for host access: %w", err)
		}
		if verbose {
			fmt.Fprintf(os.Stdout, "%s SSH server started on %s for host access\n", SuccessStyle.Render("→"), srv.Address())
		}
		// Defer cleanup
		defer stopSSHServer()
	}

	// Create execution context
	ctx := runtime.NewExecutionContext(cmdInfo.Command, cmdInfo.Invkfile)
	ctx.Verbose = verbose
	ctx.SelectedRuntime = selectedRuntime
	ctx.SelectedImpl = script
	ctx.PositionalArgs = args       // Enable shell positional parameter access ($1, $2, etc.)
	ctx.WorkDir = workdirOverride   // CLI override for working directory (--workdir flag)
	ctx.ForceRebuild = forceRebuild // Force rebuild container images (--force-rebuild flag)

	// Set environment configuration
	ctx.Env.RuntimeEnvFiles = runtimeEnvFiles // Env files from --env-file flag
	ctx.Env.RuntimeEnvVars = runtimeEnvVars   // Env vars from --env-var flag (highest precedence)

	if envInheritModeOverride != "" {
		mode, err := invkfile.ParseEnvInheritMode(envInheritModeOverride)
		if err != nil {
			return err
		}
		ctx.Env.InheritModeOverride = mode
	}

	for _, name := range envInheritAllowOverride {
		if err := invkfile.ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("env-inherit-allow: %w", err)
		}
	}
	if envInheritAllowOverride != nil {
		ctx.Env.InheritAllowOverride = envInheritAllowOverride
	}

	for _, name := range envInheritDenyOverride {
		if err := invkfile.ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("env-inherit-deny: %w", err)
		}
	}
	if envInheritDenyOverride != nil {
		ctx.Env.InheritDenyOverride = envInheritDenyOverride
	}

	// Create runtime registry
	registry := createRuntimeRegistry(cfg)

	// Check for dependencies
	if err := validateDependencies(cmdInfo, registry, ctx); err != nil {
		// Check if it's a dependency error and render it with style
		var depErr *DependencyError
		if errors.As(err, &depErr) {
			fmt.Fprint(os.Stderr, RenderDependencyError(depErr))
			rendered, _ := issue.Get(issue.DependenciesNotSatisfiedId).Render("dark")
			fmt.Fprint(os.Stderr, rendered)
		}
		return err
	}

	// Execute the command
	if verbose {
		fmt.Fprintf(os.Stdout, "%s Running '%s'...\n", SuccessStyle.Render("→"), cmdName)
	}

	// Add command-line arguments as environment variables (legacy format)
	for i, arg := range args {
		ctx.Env.ExtraEnv[fmt.Sprintf("ARG%d", i+1)] = arg
	}
	ctx.Env.ExtraEnv["ARGC"] = fmt.Sprintf("%d", len(args))

	// Add arguments as INVOWK_ARG_* environment variables (new format)
	if len(argDefs) > 0 {
		for i, argDef := range argDefs {
			envName := ArgNameToEnvVar(argDef.Name)

			switch {
			case argDef.Variadic:
				// For variadic args, collect all remaining arguments
				var variadicValues []string
				if i < len(args) {
					variadicValues = args[i:]
				}

				// Set count
				ctx.Env.ExtraEnv[envName+"_COUNT"] = fmt.Sprintf("%d", len(variadicValues))

				// Set individual values
				for j, val := range variadicValues {
					ctx.Env.ExtraEnv[fmt.Sprintf("%s_%d", envName, j+1)] = val
				}

				// Also set a space-joined version for convenience
				ctx.Env.ExtraEnv[envName] = strings.Join(variadicValues, " ")
			case i < len(args):
				// Non-variadic arg with provided value
				ctx.Env.ExtraEnv[envName] = args[i]
			case argDef.DefaultValue != "":
				// Non-variadic arg with default value
				ctx.Env.ExtraEnv[envName] = argDef.DefaultValue
			}
			// If no value and no default, don't set the env var
		}
	}

	// Add flag values as environment variables
	for name, value := range flagValues {
		envName := FlagNameToEnvVar(name)
		ctx.Env.ExtraEnv[envName] = value
	}

	var result *runtime.Result

	// Check if we should use interactive mode
	// Interactive mode is supported for all runtimes that implement InteractiveRuntime
	if interactive {
		// Get the runtime and check if it supports interactive mode
		rt, err := registry.GetForContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to get runtime: %w", err)
		}

		interactiveRT := runtime.GetInteractiveRuntime(rt)
		if interactiveRT != nil {
			result = executeInteractive(ctx, registry, cmdName, interactiveRT)
		} else {
			// Runtime doesn't support interactive mode, fall back to standard execution
			if verbose {
				fmt.Fprintf(os.Stdout, "%s Runtime '%s' does not support interactive mode, using standard execution\n",
					WarningStyle.Render("!"), rt.Name())
			}
			result = registry.Execute(ctx)
		}
	} else {
		// Standard execution
		result = registry.Execute(ctx)
	}

	if result.Error != nil {
		rendered, _ := issue.Get(issue.ScriptExecutionFailedId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		fmt.Fprintf(os.Stderr, "\n%s %v\n", ErrorStyle.Render("Error:"), result.Error)
		return result.Error
	}

	if result.ExitCode != 0 {
		if verbose {
			fmt.Fprintf(os.Stdout, "%s Command exited with code %d\n", WarningStyle.Render("!"), result.ExitCode)
		}
		return &ExitError{Code: result.ExitCode}
	}

	return nil
}

// runCommand executes a command by its name (legacy - no flag values)
func runCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	cmdName := args[0]
	cmdArgs := args[1:]

	// Delegate to runCommandWithFlags with empty flag values, no arg definitions, and no overrides
	return runCommandWithFlags(cmdName, cmdArgs, nil, nil, nil, nil, nil, "", "", nil, nil)
}

// executeInteractive runs a command in interactive mode using an alternate screen buffer.
// This provides a full PTY for the command, forwarding keyboard input during execution
// and allowing output review after completion.
// It also starts a TUI server so that nested `invowk tui *` commands can delegate
// their rendering to the parent process.
//
// The interactiveRT parameter is the runtime that implements InteractiveRuntime.
// This allows the function to work with any runtime that supports interactive mode.
func executeInteractive(ctx *runtime.ExecutionContext, registry *runtime.Registry, cmdName string, interactiveRT runtime.InteractiveRuntime) *runtime.Result {
	// Validate the context using the runtime
	if err := interactiveRT.Validate(ctx); err != nil {
		return &runtime.Result{ExitCode: 1, Error: err}
	}

	// Start the TUI server FIRST so we can pass its info to PrepareInteractive()
	// This is necessary because container runtimes need to include the TUI server
	// URL in the docker command arguments (as -e flags), not as process env vars.
	tuiServer, err := tuiserver.New()
	if err != nil {
		return &runtime.Result{ExitCode: 1, Error: fmt.Errorf("failed to create TUI server: %w", err)}
	}

	if err = tuiServer.Start(context.Background()); err != nil {
		return &runtime.Result{ExitCode: 1, Error: fmt.Errorf("failed to start TUI server: %w", err)}
	}
	defer func() { _ = tuiServer.Stop() }() // Best-effort cleanup

	// Determine the TUI server URL for the command
	// For container runtimes, use the container-accessible host address
	// (host.docker.internal or host.containers.internal)
	var tuiServerURL string
	if containerRT, ok := interactiveRT.(*runtime.ContainerRuntime); ok {
		hostAddr := containerRT.GetHostAddressForContainer()
		tuiServerURL = tuiServer.URLWithHost(hostAddr)
	} else {
		// Native/virtual runtimes use localhost
		tuiServerURL = tuiServer.URL()
	}

	// Set TUI server info in the execution context so runtimes can include it
	// in their environment setup (especially important for container runtime)
	ctx.TUI.ServerURL = tuiServerURL
	ctx.TUI.ServerToken = tuiServer.Token()

	// Prepare the command without executing it
	// Now that TUI server info is in the context, container runtime will
	// include INVOWK_TUI_ADDR and INVOWK_TUI_TOKEN in the docker command args
	prepared, err := interactiveRT.PrepareInteractive(ctx)
	if err != nil {
		return &runtime.Result{ExitCode: 1, Error: fmt.Errorf("failed to prepare command: %w", err)}
	}

	// Ensure cleanup is called when done
	if prepared.Cleanup != nil {
		defer prepared.Cleanup()
	}

	// Add TUI server environment variables to the command's process environment
	// This is for native/virtual runtimes that run directly on the host.
	// For container runtime, the env vars are already in the docker args.
	prepared.Cmd.Env = append(prepared.Cmd.Env,
		fmt.Sprintf("%s=%s", tuiserver.EnvTUIAddr, tuiServerURL),
		fmt.Sprintf("%s=%s", tuiserver.EnvTUIToken, tuiServer.Token()),
	)

	// Run the command in interactive mode
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
				// Start a bridge goroutine that reads TUI requests from the server
				// and sends them to the parent Bubbletea program for rendering.
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
