// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/issue"
	"invowk-cli/internal/runtime"
	"invowk-cli/internal/sshserver"
	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"
	"invowk-cli/pkg/invkfile"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// parseEnvVarFlags parses an array of KEY=VALUE strings into a map.
// Invalid entries (without '=') are silently ignored.
func parseEnvVarFlags(envVarFlags []string) map[string]string {
	if len(envVarFlags) == 0 {
		return nil
	}
	result := make(map[string]string, len(envVarFlags))
	for _, kv := range envVarFlags {
		idx := strings.Index(kv, "=")
		if idx > 0 {
			key := kv[:idx]
			value := kv[idx+1:]
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

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
			fmt.Fprintf(os.Stdout, "%s SSH server started on %s for host access\n", successStyle.Render("→"), srv.Address())
		}
		// Defer cleanup
		defer stopSSHServer()
	}

	// Create execution context
	ctx := runtime.NewExecutionContext(cmdInfo.Command, cmdInfo.Invkfile)
	ctx.Verbose = verbose
	ctx.SelectedRuntime = selectedRuntime
	ctx.SelectedImpl = script
	ctx.PositionalArgs = args             // Enable shell positional parameter access ($1, $2, etc.)
	ctx.RuntimeEnvFiles = runtimeEnvFiles // Env files from --env-file flag
	ctx.RuntimeEnvVars = runtimeEnvVars   // Env vars from --env-var flag (highest precedence)
	ctx.WorkDir = workdirOverride         // CLI override for working directory (--workdir flag)

	if envInheritModeOverride != "" {
		mode, err := invkfile.ParseEnvInheritMode(envInheritModeOverride)
		if err != nil {
			return err
		}
		ctx.EnvInheritModeOverride = mode
	}

	for _, name := range envInheritAllowOverride {
		if err := invkfile.ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("env-inherit-allow: %w", err)
		}
	}
	if envInheritAllowOverride != nil {
		ctx.EnvInheritAllowOverride = envInheritAllowOverride
	}

	for _, name := range envInheritDenyOverride {
		if err := invkfile.ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("env-inherit-deny: %w", err)
		}
	}
	if envInheritDenyOverride != nil {
		ctx.EnvInheritDenyOverride = envInheritDenyOverride
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
		fmt.Fprintf(os.Stdout, "%s Running '%s'...\n", successStyle.Render("→"), cmdName)
	}

	// Add command-line arguments as environment variables (legacy format)
	for i, arg := range args {
		ctx.ExtraEnv[fmt.Sprintf("ARG%d", i+1)] = arg
	}
	ctx.ExtraEnv["ARGC"] = fmt.Sprintf("%d", len(args))

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
				ctx.ExtraEnv[envName+"_COUNT"] = fmt.Sprintf("%d", len(variadicValues))

				// Set individual values
				for j, val := range variadicValues {
					ctx.ExtraEnv[fmt.Sprintf("%s_%d", envName, j+1)] = val
				}

				// Also set a space-joined version for convenience
				ctx.ExtraEnv[envName] = strings.Join(variadicValues, " ")
			case i < len(args):
				// Non-variadic arg with provided value
				ctx.ExtraEnv[envName] = args[i]
			case argDef.DefaultValue != "":
				// Non-variadic arg with default value
				ctx.ExtraEnv[envName] = argDef.DefaultValue
			}
			// If no value and no default, don't set the env var
		}
	}

	// Add flag values as environment variables
	for name, value := range flagValues {
		envName := FlagNameToEnvVar(name)
		ctx.ExtraEnv[envName] = value
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
					warningStyle.Render("!"), rt.Name())
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
		fmt.Fprintf(os.Stderr, "\n%s %v\n", errorStyle.Render("Error:"), result.Error)
		return result.Error
	}

	if result.ExitCode != 0 {
		if verbose {
			fmt.Fprintf(os.Stdout, "%s Command exited with code %d\n", warningStyle.Render("!"), result.ExitCode)
		}
		return &ExitError{Code: result.ExitCode}
	}

	return nil
}

// runDisambiguatedCommand executes a command from a specific source.
// It validates that the source exists and that the command is available in that source.
// This is used when @source prefix or --from flag is provided.
//
// For subcommands (e.g., "deploy staging"), this function attempts to match the longest
// possible command name by progressively joining args. For example, with args ["deploy", "staging"],
// it first tries "deploy staging", then falls back to "deploy" if no match is found.
func runDisambiguatedCommand(filter *SourceFilter, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	// Get the command set for source validation and command lookup
	commandSet, err := disc.DiscoverCommandSet()
	if err != nil {
		return err
	}

	// Validate that the source exists
	sourceExists := false
	var availableSources []string
	for _, sourceID := range commandSet.SourceOrder {
		availableSources = append(availableSources, sourceID)
		if sourceID == filter.SourceID {
			sourceExists = true
		}
	}

	if !sourceExists {
		// Render helpful error with available sources
		err := &SourceNotFoundError{
			Source:           filter.SourceID,
			AvailableSources: availableSources,
		}
		fmt.Fprint(os.Stderr, RenderSourceNotFoundError(err))
		return err
	}

	// Find the command in the specified source.
	// For subcommands, try to match the longest possible command name.
	// e.g., for args ["deploy", "staging", "arg1"], try:
	//   1. "deploy staging" (if it exists as a command)
	//   2. "deploy" (fall back if "deploy staging" doesn't exist)
	cmdsInSource := commandSet.BySource[filter.SourceID]
	var targetCmd *discovery.CommandInfo
	var matchLen int

	// Try matching progressively longer command names (greedy match)
	for i := len(args); i > 0; i-- {
		candidateName := strings.Join(args[:i], " ")
		for _, cmd := range cmdsInSource {
			if cmd.SimpleName == candidateName {
				targetCmd = cmd
				matchLen = i
				break
			}
		}
		if targetCmd != nil {
			break
		}
	}

	// Determine remaining args after the matched command name
	var cmdArgs []string
	if matchLen < len(args) {
		cmdArgs = args[matchLen:]
	}

	// For error reporting, use the first arg as the "requested" command name
	displayCmdName := args[0]
	if len(args) > 1 {
		displayCmdName = strings.Join(args, " ")
	}

	if targetCmd == nil {
		// Command not found in specified source
		fmt.Fprintf(os.Stderr, "\n%s Command '%s' not found in source '%s'\n\n",
			errorStyle.Render("✗"), displayCmdName, filter.SourceID)

		// Show what commands ARE available in that source
		if len(cmdsInSource) > 0 {
			fmt.Fprintf(os.Stderr, "Available commands in %s:\n", formatSourceDisplayName(filter.SourceID))
			for _, cmd := range cmdsInSource {
				fmt.Fprintf(os.Stderr, "  %s\n", cmd.SimpleName)
			}
			fmt.Fprintln(os.Stderr)
		}
		return fmt.Errorf("command '%s' not found in source '%s'", displayCmdName, filter.SourceID)
	}

	// Execute the command using its full Name (which includes any module prefix)
	return runCommandWithFlags(targetCmd.Name, cmdArgs, nil, nil, nil, nil, nil, "", "", nil, nil)
}

// checkAmbiguousCommand checks if a command (including subcommands) is ambiguous.
// It takes the full args slice and tries to find the longest matching command name,
// then checks if that command exists in multiple sources.
//
// For example, with args ["deploy", "staging"], it checks:
//  1. Is "deploy staging" ambiguous?
//  2. If not a known command, is "deploy" ambiguous?
//
// This function is called before Cobra's normal command matching when no explicit source is specified.
func checkAmbiguousCommand(args []string) error {
	if len(args) == 0 {
		return nil
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	commandSet, err := disc.DiscoverCommandSet()
	if err != nil {
		// If we can't discover commands, let the normal flow handle it.
		// Intentionally returning nil here to allow Cobra to attempt command matching,
		// which will produce an appropriate error if the command doesn't exist.
		return nil //nolint:nilerr // Intentional: fall through to normal flow on discovery errors
	}

	// Try to find the longest matching command name.
	// This handles subcommands like "deploy staging" which would be passed as ["deploy", "staging"].
	var cmdName string
	for i := len(args); i > 0; i-- {
		candidateName := strings.Join(args[:i], " ")
		// Check if this candidate is a known command name
		if _, exists := commandSet.BySimpleName[candidateName]; exists {
			cmdName = candidateName
			break
		}
	}

	// If no matching command found, fall back to just the first arg
	// (let Cobra handle "unknown command" errors)
	if cmdName == "" {
		cmdName = args[0]
	}

	// Check if this command name is ambiguous
	if !commandSet.AmbiguousNames[cmdName] {
		return nil
	}

	// Collect the sources where this command exists
	var sources []string
	for _, sourceID := range commandSet.SourceOrder {
		cmdsInSource := commandSet.BySource[sourceID]
		for _, cmd := range cmdsInSource {
			if cmd.SimpleName == cmdName {
				sources = append(sources, sourceID)
				break
			}
		}
	}

	return &AmbiguousCommandError{
		CommandName: cmdName,
		Sources:     sources,
	}
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
	ctx.TUIServerURL = tuiServerURL
	ctx.TUIServerToken = tuiServer.Token()

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

// bridgeTUIRequests reads TUI component requests from the server's channel
// and sends them to the parent Bubbletea program for rendering as overlays.
// It runs in a goroutine and terminates when the server stops.
func bridgeTUIRequests(server *tuiserver.Server, program *tea.Program) {
	for req := range server.RequestChannel() {
		// Send the request to the interactive model for rendering
		program.Send(tui.TUIComponentMsg{
			Component:  tui.ComponentType(req.Component),
			Options:    req.Options,
			ResponseCh: req.ResponseCh,
		})
	}
}

// createRuntimeRegistry creates and populates the runtime registry
func createRuntimeRegistry(cfg *config.Config) *runtime.Registry {
	registry := runtime.NewRegistry()

	// Register native runtime
	registry.Register(runtime.RuntimeTypeNative, runtime.NewNativeRuntime())

	// Register virtual runtime
	registry.Register(runtime.RuntimeTypeVirtual, runtime.NewVirtualRuntime(cfg.VirtualShell.EnableUrootUtils))

	// Register container runtime (may fail if no engine available)
	containerRT, err := runtime.NewContainerRuntime(cfg)
	if err == nil {
		// Set the SSH server if it's running
		sshServerMu.Lock()
		if sshServerInstance != nil && sshServerInstance.IsRunning() {
			containerRT.SetSSHServer(sshServerInstance)
		}
		sshServerMu.Unlock()
		registry.Register(runtime.RuntimeTypeContainer, containerRT)
	}

	return registry
}

// sshServerInstance and sshServerMu are declared in cmd.go
// These variables are package-level and shared across files

// ensureSSHServer starts the SSH server if not already running
func ensureSSHServer() (*sshserver.Server, error) {
	sshServerMu.Lock()
	defer sshServerMu.Unlock()

	if sshServerInstance != nil && sshServerInstance.IsRunning() {
		return sshServerInstance, nil
	}

	srv := sshserver.New(sshserver.DefaultConfig())

	// Start blocks until the server is ready to accept connections or fails
	if err := srv.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start SSH server: %w", err)
	}

	sshServerInstance = srv
	return srv, nil
}

// stopSSHServer stops the SSH server if running
func stopSSHServer() {
	sshServerMu.Lock()
	defer sshServerMu.Unlock()

	if sshServerInstance != nil {
		_ = sshServerInstance.Stop()
		sshServerInstance = nil
	}
}
