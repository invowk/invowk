package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/issue"
	"invowk-cli/internal/runtime"
	"invowk-cli/internal/sshserver"
	"invowk-cli/pkg/invowkfile"
)

var (
	// runtimeOverride allows overriding the runtime for a command
	runtimeOverride string
	// sshServerInstance is the global SSH server instance
	sshServerInstance *sshserver.Server
	// sshServerMu protects the SSH server instance
	sshServerMu sync.Mutex
)

// listFlag controls whether to list commands
var listFlag bool

// cmdCmd is the parent command for all discovered commands
var cmdCmd = &cobra.Command{
	Use:   "cmd [command-name]",
	Short: "Execute commands from invowkfiles",
	Long: `Execute commands defined in invowkfiles.

Commands are discovered from:
  1. Current directory (highest priority)
  2. ~/.invowk/cmds/
  3. Configured search paths

Use 'invowk cmd' or 'invowk cmd --list' to see all available commands.
Use 'invowk cmd <command-name>' to execute a command.
Use 'invowk cmd <command-name> --runtime <runtime>' to execute with a specific runtime.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If --list flag is set or no arguments, show list
		if listFlag || len(args) == 0 {
			return listCommands()
		}
		return runCommand(args)
	},
	ValidArgsFunction: completeCommands,
}

func init() {
	cmdCmd.Flags().BoolVarP(&listFlag, "list", "l", false, "list all available commands")
	cmdCmd.PersistentFlags().StringVarP(&runtimeOverride, "runtime", "r", "", "override the runtime (must be allowed by the command)")

	// Dynamically add discovered commands
	// This happens at init time to enable shell completion
	registerDiscoveredCommands()
}

// registerDiscoveredCommands adds discovered commands as subcommands
func registerDiscoveredCommands() {
	cfg := config.Get()
	disc := discovery.New(cfg)

	commands, err := disc.DiscoverCommands()
	if err != nil {
		return // Silently fail during init
	}

	// Build command tree for commands with spaces in names
	commandMap := make(map[string]*cobra.Command)

	for _, cmdInfo := range commands {
		// Split command name by spaces (e.g., "test unit" -> ["test", "unit"])
		parts := strings.Fields(cmdInfo.Name)

		// Create parent commands if needed
		var parent *cobra.Command = cmdCmd
		var prefix string

		for i, part := range parts {
			if prefix != "" {
				prefix += " "
			}
			prefix += part

			if existing, ok := commandMap[prefix]; ok {
				parent = existing
				continue
			}

			// Create new command
			isLeaf := i == len(parts)-1
			var newCmd *cobra.Command

			if isLeaf {
				// Leaf command - actually runs something
				cmdName := cmdInfo.Name // Capture for closure
				newCmd = &cobra.Command{
					Use:   part,
					Short: cmdInfo.Description,
					Long:  fmt.Sprintf("Run the '%s' command from %s", cmdInfo.Name, cmdInfo.FilePath),
					RunE: func(cmd *cobra.Command, args []string) error {
						return runCommand(append([]string{cmdName}, args...))
					},
				}
			} else {
				// Parent command for nested structure
				newCmd = &cobra.Command{
					Use:   part,
					Short: fmt.Sprintf("Commands under '%s'", prefix),
					RunE: func(cmd *cobra.Command, args []string) error {
						return cmd.Help()
					},
				}
			}

			parent.AddCommand(newCmd)
			commandMap[prefix] = newCmd
			parent = newCmd
		}
	}
}

// completeCommands provides shell completion for commands
func completeCommands(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cfg := config.Get()
	disc := discovery.New(cfg)

	commands, err := disc.DiscoverCommands()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	// Build prefix from already-typed args (space-separated)
	prefix := strings.Join(args, " ")
	if prefix != "" {
		prefix += " "
	}

	for _, cmdInfo := range commands {
		cmdName := cmdInfo.Name

		// Filter by prefix if we're completing a nested command
		if prefix != "" && !strings.HasPrefix(cmdName, prefix) {
			continue
		}

		// Remove the prefix to get relative part
		relativeName := strings.TrimPrefix(cmdName, prefix)

		// Only show the next word
		parts := strings.Fields(relativeName)
		if len(parts) == 0 {
			continue
		}
		nextPart := parts[0]

		// Check for duplicates
		found := false
		for _, c := range completions {
			if strings.HasPrefix(c, nextPart+"\t") || c == nextPart {
				found = true
				break
			}
		}
		if !found && strings.HasPrefix(nextPart, toComplete) {
			desc := cmdInfo.Description
			if len(parts) == 1 && desc != "" {
				// This is the actual command, show description
				completions = append(completions, nextPart+"\t"+desc)
			} else {
				completions = append(completions, nextPart)
			}
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}

// listCommands displays all available commands
func listCommands() error {
	cfg := config.Get()
	disc := discovery.New(cfg)

	// First load all files to check for parsing errors
	files, err := disc.LoadAll()
	if err != nil {
		rendered, _ := issue.Get(issue.InvowkfileNotFoundId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return err
	}

	// Show any parsing errors
	for _, file := range files {
		if file.Error != nil {
			fmt.Fprintf(os.Stderr, "%s Failed to parse %s: %v\n", errorStyle.Render("✗"), file.Path, file.Error)
		}
	}

	commands, err := disc.DiscoverCommands()
	if err != nil {
		rendered, _ := issue.Get(issue.InvowkfileNotFoundId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return err
	}

	if len(commands) == 0 {
		rendered, _ := issue.Get(issue.InvowkfileNotFoundId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return fmt.Errorf("no commands found")
	}

	// Group commands by source
	bySource := make(map[discovery.Source][]*discovery.CommandInfo)
	for _, cmd := range commands {
		bySource[cmd.Source] = append(bySource[cmd.Source], cmd)
	}

	// Style for output
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	sourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	defaultRuntimeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	platformsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	legendStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Italic(true)

	fmt.Println(headerStyle.Render("Available Commands"))
	fmt.Println(legendStyle.Render("  (* = default runtime)"))
	fmt.Println()

	sources := []discovery.Source{discovery.SourceCurrentDir, discovery.SourceUserDir, discovery.SourceConfigPath}
	for _, source := range sources {
		cmds := bySource[source]
		if len(cmds) == 0 {
			continue
		}

		fmt.Println(sourceStyle.Render(fmt.Sprintf("From %s:", source.String())))

		for _, cmd := range cmds {
			line := fmt.Sprintf("  %s", nameStyle.Render(cmd.Name))
			if cmd.Description != "" {
				line += fmt.Sprintf(" - %s", descStyle.Render(cmd.Description))
			}
			// Show runtimes with default highlighted for current platform
			currentPlatform := invowkfile.GetCurrentHostOS()
			runtimesStr := cmd.Command.GetRuntimesStringForPlatform(currentPlatform)
			if runtimesStr != "" {
				line += " [" + defaultRuntimeStyle.Render(runtimesStr) + "]"
			}
			// Show supported platforms
			platformsStr := cmd.Command.GetPlatformsString()
			if platformsStr != "" {
				line += fmt.Sprintf(" (%s)", platformsStyle.Render(platformsStr))
			}
			fmt.Println(line)
		}
		fmt.Println()
	}

	return nil
}

// runCommand executes a command by its name
func runCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	cmdName := args[0]
	cmdArgs := args[1:]

	cfg := config.Get()
	disc := discovery.New(cfg)

	// Find the command
	cmdInfo, err := disc.GetCommand(cmdName)
	if err != nil {
		rendered, _ := issue.Get(issue.CommandNotFoundId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return fmt.Errorf("command '%s' not found", cmdName)
	}

	// Get the current platform
	currentPlatform := invowkfile.GetCurrentHostOS()

	// Validate host OS compatibility
	if !cmdInfo.Command.CanRunOnCurrentHost() {
		supportedPlatforms := cmdInfo.Command.GetPlatformsString()
		fmt.Fprint(os.Stderr, RenderHostNotSupportedError(cmdName, string(currentPlatform), supportedPlatforms))
		rendered, _ := issue.Get(issue.HostNotSupportedId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return fmt.Errorf("command '%s' does not support platform '%s' (supported: %s)", cmdName, currentPlatform, supportedPlatforms)
	}

	// Determine which runtime to use
	var selectedRuntime invowkfile.RuntimeMode
	if runtimeOverride != "" {
		// Validate that the overridden runtime is allowed for this platform
		overrideRuntime := invowkfile.RuntimeMode(runtimeOverride)
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
			fmt.Printf("%s SSH server started on %s for host access\n", successStyle.Render("→"), srv.Address())
		}
		// Defer cleanup
		defer stopSSHServer()
	}

	// Create execution context
	ctx := runtime.NewExecutionContext(cmdInfo.Command, cmdInfo.Invowkfile)
	ctx.Verbose = verbose
	ctx.SelectedRuntime = selectedRuntime
	ctx.SelectedImpl = script

	// Create runtime registry
	registry := createRuntimeRegistry(cfg)

	// Check for dependencies
	if err := executeDependencies(cmdInfo, registry, ctx); err != nil {
		// Check if it's a dependency error and render it with style
		if depErr, ok := err.(*DependencyError); ok {
			fmt.Fprint(os.Stderr, RenderDependencyError(depErr))
			rendered, _ := issue.Get(issue.DependenciesNotSatisfiedId).Render("dark")
			fmt.Fprint(os.Stderr, rendered)
		}
		return err
	}

	// Execute the command
	if verbose {
		fmt.Printf("%s Running '%s'...\n", successStyle.Render("→"), cmdName)
	}

	// Add command-line arguments as environment variables
	for i, arg := range cmdArgs {
		ctx.ExtraEnv[fmt.Sprintf("ARG%d", i+1)] = arg
	}
	ctx.ExtraEnv["ARGC"] = fmt.Sprintf("%d", len(cmdArgs))

	result := registry.Execute(ctx)
	if result.Error != nil {
		rendered, _ := issue.Get(issue.ScriptExecutionFailedId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		fmt.Fprintf(os.Stderr, "\n%s %v\n", errorStyle.Render("Error:"), result.Error)
		return result.Error
	}

	if result.ExitCode != 0 {
		if verbose {
			fmt.Printf("%s Command exited with code %d\n", warningStyle.Render("!"), result.ExitCode)
		}
		os.Exit(result.ExitCode)
	}

	return nil
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

// ensureSSHServer starts the SSH server if not already running
func ensureSSHServer() (*sshserver.Server, error) {
	sshServerMu.Lock()
	defer sshServerMu.Unlock()

	if sshServerInstance != nil && sshServerInstance.IsRunning() {
		return sshServerInstance, nil
	}

	srv, err := sshserver.New(sshserver.DefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH server: %w", err)
	}

	if err := srv.Start(); err != nil {
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

// executeDependencies checks tool dependencies and runs dependent commands
// Dependencies are merged from both command-level and script-level, and
// validated according to the selected runtime:
// - native: validated against the native standard shell from the host
// - virtual: validated against invowk's built-in sh interpreter with core utils
// - container: validated against the container's default shell from within the container
func executeDependencies(cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext) error {
	// Merge command-level and script-level dependencies
	mergedDeps := invowkfile.MergeDependsOn(cmdInfo.Command.DependsOn, parentCtx.SelectedImpl.DependsOn)

	if mergedDeps == nil {
		return nil
	}

	// Get the selected runtime for context-aware validation
	selectedRuntime := parentCtx.SelectedRuntime

	// First check tool dependencies (runtime-aware)
	if err := checkToolDependenciesWithRuntime(mergedDeps, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then check filepath dependencies (runtime-aware)
	if err := checkFilepathDependenciesWithRuntime(mergedDeps, cmdInfo.Invowkfile.FilePath, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then check capability dependencies (host-only, not runtime-aware)
	if err := checkCapabilityDependencies(mergedDeps, parentCtx); err != nil {
		return err
	}

	// Then check custom check dependencies (runtime-aware)
	if err := checkCustomCheckDependencies(mergedDeps, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then run command dependencies
	if len(mergedDeps.Commands) == 0 {
		return nil
	}

	cmdDeps := make([]string, len(mergedDeps.Commands))
	for i, dep := range mergedDeps.Commands {
		cmdDeps[i] = dep.Name
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	// Track executed dependencies to detect cycles
	executed := make(map[string]bool)

	return executeDepsRecursive(cmdDeps, disc, registry, parentCtx, executed)
}

// checkToolDependenciesWithRuntime verifies all required tools are available
// The validation method depends on the runtime:
// - native: check against host system PATH
// - virtual: check against built-in utilities
// - container: check within the container environment
func checkToolDependenciesWithRuntime(deps *invowkfile.DependsOn, runtimeMode invowkfile.RuntimeMode, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Tools) == 0 {
		return nil
	}

	var toolErrors []string

	for _, tool := range deps.Tools {
		var err error
		switch runtimeMode {
		case invowkfile.RuntimeContainer:
			err = validateToolInContainer(tool, registry, ctx)
		case invowkfile.RuntimeVirtual:
			err = validateToolInVirtual(tool, registry, ctx)
		default: // native
			err = validateToolNative(tool)
		}
		if err != nil {
			toolErrors = append(toolErrors, err.Error())
		}
	}

	if len(toolErrors) > 0 {
		return &DependencyError{
			CommandName:  ctx.Command.Name,
			MissingTools: toolErrors,
		}
	}

	return nil
}

// validateToolNative validates a tool dependency against the host system PATH
func validateToolNative(tool invowkfile.ToolDependency) error {
	_, err := exec.LookPath(tool.Name)
	if err != nil {
		return fmt.Errorf("  • %s - not found in PATH", tool.Name)
	}
	return nil
}

// validateToolInVirtual validates a tool dependency using the virtual runtime
func validateToolInVirtual(tool invowkfile.ToolDependency, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeVirtual)
	if err != nil {
		// Fall back to native validation if virtual runtime not available
		return validateToolNative(tool)
	}

	// Use 'command -v' to check if tool exists in virtual shell
	checkScript := fmt.Sprintf("command -v %s", tool.Name)

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invowkfile:      ctx.Invowkfile,
		SelectedImpl:    &invowkfile.Implementation{Script: checkScript, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}}}},
		SelectedRuntime: invowkfile.RuntimeVirtual,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)

	if result.ExitCode != 0 {
		return fmt.Errorf("  • %s - not available in virtual runtime", tool.Name)
	}
	return nil
}

// validateToolInContainer validates a tool dependency within a container
func validateToolInContainer(tool invowkfile.ToolDependency, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • %s - container runtime not available", tool.Name)
	}

	// Use 'command -v' or 'which' to check if tool exists in container
	checkScript := fmt.Sprintf("command -v %s || which %s", tool.Name, tool.Name)

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invowkfile:      ctx.Invowkfile,
		SelectedImpl:    &invowkfile.Implementation{Script: checkScript, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}}}},
		SelectedRuntime: invowkfile.RuntimeContainer,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)

	if result.ExitCode != 0 {
		return fmt.Errorf("  • %s - not available in container", tool.Name)
	}
	return nil
}

// validateCustomCheckOutput validates custom check script output against expected values
func validateCustomCheckOutput(check invowkfile.CustomCheck, outputStr string, execErr error) error {
	// Determine expected exit code (default: 0)
	expectedCode := 0
	if check.ExpectedCode != nil {
		expectedCode = *check.ExpectedCode
	}

	// Check exit code
	actualCode := 0
	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			actualCode = exitErr.ExitCode()
		} else {
			// Try to get exit code from error message for non-native runtimes
			actualCode = 1 // Default to 1 for errors
		}
	}

	if actualCode != expectedCode {
		return fmt.Errorf("  • %s - check script returned exit code %d, expected %d", check.Name, actualCode, expectedCode)
	}

	// Check output pattern if specified
	if check.ExpectedOutput != "" {
		matched, err := regexp.MatchString(check.ExpectedOutput, outputStr)
		if err != nil {
			return fmt.Errorf("  • %s - invalid regex pattern '%s': %v", check.Name, check.ExpectedOutput, err)
		}
		if !matched {
			return fmt.Errorf("  • %s - check script output '%s' does not match pattern '%s'", check.Name, outputStr, check.ExpectedOutput)
		}
	}

	return nil
}

// checkCustomCheckDependencies validates all custom check scripts
// The validation method depends on the runtime:
// - native: executed using the host's native shell
// - virtual: executed using invowk's built-in sh interpreter
// - container: executed within the container environment
func checkCustomCheckDependencies(deps *invowkfile.DependsOn, runtimeMode invowkfile.RuntimeMode, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []string

	for _, check := range deps.CustomChecks {
		var err error
		switch runtimeMode {
		case invowkfile.RuntimeContainer:
			err = validateCustomCheckInContainer(check, registry, ctx)
		case invowkfile.RuntimeVirtual:
			err = validateCustomCheckInVirtual(check, registry, ctx)
		default: // native
			err = validateCustomCheckNative(check)
		}
		if err != nil {
			checkErrors = append(checkErrors, err.Error())
		}
	}

	if len(checkErrors) > 0 {
		return &DependencyError{
			CommandName:        ctx.Command.Name,
			FailedCustomChecks: checkErrors,
		}
	}

	return nil
}

// validateCustomCheckNative runs a custom check script using the native shell
func validateCustomCheckNative(check invowkfile.CustomCheck) error {
	cmd := exec.Command("sh", "-c", check.CheckScript)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	return validateCustomCheckOutput(check, outputStr, err)
}

// validateCustomCheckInVirtual runs a custom check script using the virtual runtime
func validateCustomCheckInVirtual(check invowkfile.CustomCheck, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeVirtual)
	if err != nil {
		// Fall back to native validation if virtual runtime not available
		return validateCustomCheckNative(check)
	}

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invowkfile:      ctx.Invowkfile,
		SelectedImpl:    &invowkfile.Implementation{Script: check.CheckScript, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}}}},
		SelectedRuntime: invowkfile.RuntimeVirtual,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)
	outputStr := strings.TrimSpace(stdout.String() + stderr.String())

	return validateCustomCheckOutput(check, outputStr, result.Error)
}

// validateCustomCheckInContainer runs a custom check script within a container
func validateCustomCheckInContainer(check invowkfile.CustomCheck, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • %s - container runtime not available", check.Name)
	}

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invowkfile:      ctx.Invowkfile,
		SelectedImpl:    &invowkfile.Implementation{Script: check.CheckScript, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}}}},
		SelectedRuntime: invowkfile.RuntimeContainer,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)
	outputStr := strings.TrimSpace(stdout.String() + stderr.String())

	return validateCustomCheckOutput(check, outputStr, result.Error)
}

// checkFilepathDependenciesWithRuntime verifies all required files/directories exist
// The validation method depends on the runtime:
// - native: check against host filesystem
// - virtual: check against host filesystem (virtual shell still uses host fs)
// - container: check within the container filesystem
func checkFilepathDependenciesWithRuntime(deps *invowkfile.DependsOn, invowkfilePath string, runtimeMode invowkfile.RuntimeMode, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invowkfilePath)

	for _, fp := range deps.Filepaths {
		var err error
		switch runtimeMode {
		case invowkfile.RuntimeContainer:
			err = validateFilepathInContainer(fp, invowkDir, registry, ctx)
		default: // native and virtual use host filesystem
			err = validateFilepathAlternatives(fp, invowkDir)
		}
		if err != nil {
			filepathErrors = append(filepathErrors, err.Error())
		}
	}

	if len(filepathErrors) > 0 {
		return &DependencyError{
			CommandName:      ctx.Command.Name,
			MissingFilepaths: filepathErrors,
		}
	}

	return nil
}

// validateFilepathInContainer validates a filepath dependency within a container
func validateFilepathInContainer(fp invowkfile.FilepathDependency, invowkDir string, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • container runtime not available")
	}

	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		// Build a check script for this path
		var checks []string

		// Basic existence check
		checks = append(checks, fmt.Sprintf("test -e '%s'", altPath))

		if fp.Readable {
			checks = append(checks, fmt.Sprintf("test -r '%s'", altPath))
		}
		if fp.Writable {
			checks = append(checks, fmt.Sprintf("test -w '%s'", altPath))
		}
		if fp.Executable {
			checks = append(checks, fmt.Sprintf("test -x '%s'", altPath))
		}

		checkScript := strings.Join(checks, " && ")

		// Create a minimal context for validation
		var stdout, stderr bytes.Buffer
		validationCtx := &runtime.ExecutionContext{
			Command:         ctx.Command,
			Invowkfile:      ctx.Invowkfile,
			SelectedImpl:    &invowkfile.Implementation{Script: checkScript, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}}}},
			SelectedRuntime: invowkfile.RuntimeContainer,
			Stdout:          &stdout,
			Stderr:          &stderr,
			Context:         ctx.Context,
			ExtraEnv:        make(map[string]string),
		}

		result := rt.Execute(validationCtx)
		if result.ExitCode == 0 {
			// This alternative satisfies the dependency
			return nil
		}
		allErrors = append(allErrors, fmt.Sprintf("%s: not found or permission denied in container", altPath))
	}

	// None of the alternatives satisfied the requirements
	if len(fp.Alternatives) == 1 {
		return fmt.Errorf("  • %s - %s", fp.Alternatives[0], allErrors[0])
	}
	return fmt.Errorf("  • none of the alternatives satisfied the requirements in container:\n      - %s", strings.Join(allErrors, "\n      - "))
}

// checkToolDependencies verifies all required tools are available in PATH (legacy - uses native)
func checkToolDependencies(cmd *invowkfile.Command) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Tools) == 0 {
		return nil
	}

	var toolErrors []string

	for _, tool := range cmd.DependsOn.Tools {
		if err := validateToolNative(tool); err != nil {
			toolErrors = append(toolErrors, err.Error())
		}
	}

	if len(toolErrors) > 0 {
		return &DependencyError{
			CommandName:  cmd.Name,
			MissingTools: toolErrors,
		}
	}

	return nil
}

// checkCustomChecks verifies all custom check scripts pass (legacy - uses native)
func checkCustomChecks(cmd *invowkfile.Command) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []string

	for _, check := range cmd.DependsOn.CustomChecks {
		if err := validateCustomCheckNative(check); err != nil {
			checkErrors = append(checkErrors, err.Error())
		}
	}

	if len(checkErrors) > 0 {
		return &DependencyError{
			CommandName:        cmd.Name,
			FailedCustomChecks: checkErrors,
		}
	}

	return nil
}

// checkFilepathDependencies verifies all required files/directories exist with proper permissions (legacy - uses native)
func checkFilepathDependencies(cmd *invowkfile.Command, invowkfilePath string) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invowkfilePath)

	for _, fp := range cmd.DependsOn.Filepaths {
		if err := validateFilepathAlternatives(fp, invowkDir); err != nil {
			filepathErrors = append(filepathErrors, err.Error())
		}
	}

	if len(filepathErrors) > 0 {
		return &DependencyError{
			CommandName:      cmd.Name,
			MissingFilepaths: filepathErrors,
		}
	}

	return nil
}

// validateFilepathAlternatives checks if any of the alternative paths exists and has the required permissions
// Returns nil (success) if any alternative satisfies all requirements
func validateFilepathAlternatives(fp invowkfile.FilepathDependency, invowkDir string) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		// Resolve path relative to invowkfile if not absolute
		resolvedPath := altPath
		if !filepath.IsAbs(altPath) {
			resolvedPath = filepath.Join(invowkDir, altPath)
		}

		if err := validateSingleFilepath(altPath, resolvedPath, fp); err == nil {
			// Success! This alternative satisfies the dependency
			return nil
		} else {
			allErrors = append(allErrors, fmt.Sprintf("%s: %s", altPath, err.Error()))
		}
	}

	// None of the alternatives satisfied the requirements
	if len(fp.Alternatives) == 1 {
		return fmt.Errorf("  • %s - %s", fp.Alternatives[0], allErrors[0])
	}
	return fmt.Errorf("  • none of the alternatives satisfied the requirements:\n      - %s", strings.Join(allErrors, "\n      - "))
}

// validateSingleFilepath checks if a single filepath exists and has the required permissions
func validateSingleFilepath(displayPath string, resolvedPath string, fp invowkfile.FilepathDependency) error {
	// Check if path exists
	info, err := os.Stat(resolvedPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("path does not exist")
	}
	if err != nil {
		return fmt.Errorf("cannot access path: %v", err)
	}

	var permErrors []string

	// Check readable permission
	if fp.Readable {
		if !isReadable(resolvedPath, info) {
			permErrors = append(permErrors, "read")
		}
	}

	// Check writable permission
	if fp.Writable {
		if !isWritable(resolvedPath, info) {
			permErrors = append(permErrors, "write")
		}
	}

	// Check executable permission
	if fp.Executable {
		if !isExecutable(resolvedPath, info) {
			permErrors = append(permErrors, "execute")
		}
	}

	if len(permErrors) > 0 {
		return fmt.Errorf("missing permissions: %s", strings.Join(permErrors, ", "))
	}

	return nil
}

// validateFilepath is deprecated - use validateFilepathAlternatives instead
func validateFilepath(fp invowkfile.FilepathDependency, resolvedPath string) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - at least one path must be provided in alternatives")
	}
	return validateSingleFilepath(fp.Alternatives[0], resolvedPath, fp)
}

// isReadable checks if a path is readable (cross-platform)
func isReadable(path string, info os.FileInfo) bool {
	// Try to open the file/directory for reading
	if info.IsDir() {
		f, err := os.Open(path)
		if err != nil {
			return false
		}
		f.Close()
		return true
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// isWritable checks if a path is writable (cross-platform)
func isWritable(path string, info os.FileInfo) bool {
	// For directories, try to create a temp file
	if info.IsDir() {
		testFile := filepath.Join(path, ".invowk_write_test")
		f, err := os.Create(testFile)
		if err != nil {
			return false
		}
		f.Close()
		os.Remove(testFile)
		return true
	}
	// For files, try to open for writing
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// isExecutable checks if a path is executable (cross-platform)
func isExecutable(path string, info os.FileInfo) bool {
	// On Windows, check file extension
	if isWindows() {
		ext := strings.ToLower(filepath.Ext(path))
		execExts := []string{".exe", ".cmd", ".bat", ".com", ".ps1"}
		for _, e := range execExts {
			if ext == e {
				return true
			}
		}
		// Also check PATHEXT environment variable
		pathext := os.Getenv("PATHEXT")
		if pathext != "" {
			for _, e := range strings.Split(strings.ToLower(pathext), ";") {
				if ext == e {
					return true
				}
			}
		}
		return false
	}

	// On Unix-like systems, check execute permission bit
	mode := info.Mode()
	return mode&0111 != 0
}

// checkCapabilityDependencies verifies all required system capabilities are available.
// Capabilities are always checked against the host system, regardless of the runtime mode.
// For container runtimes, these checks represent the host's capabilities, not the container's.
func checkCapabilityDependencies(deps *invowkfile.DependsOn, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Capabilities) == 0 {
		return nil
	}

	var capabilityErrors []string

	// Track seen capabilities to detect duplicates (they're just skipped, not an error)
	seen := make(map[invowkfile.CapabilityName]bool)

	for _, cap := range deps.Capabilities {
		// Skip duplicates
		if seen[cap.Name] {
			continue
		}
		seen[cap.Name] = true

		if err := invowkfile.CheckCapability(cap.Name); err != nil {
			capabilityErrors = append(capabilityErrors, fmt.Sprintf("  • %s", err.Error()))
		}
	}

	if len(capabilityErrors) > 0 {
		return &DependencyError{
			CommandName:         ctx.Command.Name,
			MissingCapabilities: capabilityErrors,
		}
	}

	return nil
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return os.PathSeparator == '\\' && os.PathListSeparator == ';'
}

// DependencyError represents unsatisfied dependencies
type DependencyError struct {
	CommandName         string
	MissingTools        []string
	MissingCommands     []string
	MissingFilepaths    []string
	MissingCapabilities []string
	FailedCustomChecks  []string
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependencies not satisfied for command '%s'", e.CommandName)
}

// RenderDependencyError creates a styled error message for unsatisfied dependencies
func RenderDependencyError(err *DependencyError) string {
	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")).
		MarginTop(1)

	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	sb.WriteString(headerStyle.Render("✗ Dependencies not satisfied!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cannot run command %s because some dependencies are missing.\n", commandStyle.Render("'"+err.CommandName+"'")))

	if len(err.MissingTools) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing Tools:"))
		sb.WriteString("\n")
		for _, tool := range err.MissingTools {
			sb.WriteString(itemStyle.Render(tool))
			sb.WriteString("\n")
		}
	}

	if len(err.MissingCommands) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing Commands:"))
		sb.WriteString("\n")
		for _, cmd := range err.MissingCommands {
			sb.WriteString(itemStyle.Render(cmd))
			sb.WriteString("\n")
		}
	}

	if len(err.MissingFilepaths) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing or Inaccessible Files:"))
		sb.WriteString("\n")
		for _, fp := range err.MissingFilepaths {
			sb.WriteString(itemStyle.Render(fp))
			sb.WriteString("\n")
		}
	}

	if len(err.MissingCapabilities) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing Capabilities:"))
		sb.WriteString("\n")
		for _, cap := range err.MissingCapabilities {
			sb.WriteString(itemStyle.Render(cap))
			sb.WriteString("\n")
		}
	}

	if len(err.FailedCustomChecks) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Failed Custom Checks:"))
		sb.WriteString("\n")
		for _, check := range err.FailedCustomChecks {
			sb.WriteString(itemStyle.Render(check))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Install the missing tools and try again, or update your invowkfile to remove unnecessary dependencies."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderHostNotSupportedError creates a styled error message for unsupported host OS
func RenderHostNotSupportedError(cmdName, currentOS, supportedHosts string) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	sb.WriteString(headerStyle.Render("✗ Host not supported!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cannot run command %s on this operating system.\n\n", commandStyle.Render("'"+cmdName+"'")))
	sb.WriteString(labelStyle.Render("Current host:    "))
	sb.WriteString(valueStyle.Render(currentOS))
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Supported hosts: "))
	sb.WriteString(valueStyle.Render(supportedHosts))
	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("Run this command on a supported operating system, or update the 'works_on.hosts' setting in your invowkfile."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderRuntimeNotAllowedError creates a styled error message for invalid runtime selection
func RenderRuntimeNotAllowedError(cmdName, selectedRuntime, allowedRuntimes string) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	sb.WriteString(headerStyle.Render("✗ Runtime not allowed!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Cannot run command %s with the specified runtime.\n\n", commandStyle.Render("'"+cmdName+"'")))
	sb.WriteString(labelStyle.Render("Selected runtime: "))
	sb.WriteString(valueStyle.Render(selectedRuntime))
	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Allowed runtimes: "))
	sb.WriteString(valueStyle.Render(allowedRuntimes))
	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("Use one of the allowed runtimes with --runtime flag, or update the 'runtimes' setting in your invowkfile."))
	sb.WriteString("\n")

	return sb.String()
}

func executeDepsRecursive(deps []string, disc *discovery.Discovery, registry *runtime.Registry, parentCtx *runtime.ExecutionContext, executed map[string]bool) error {
	for _, depName := range deps {
		if executed[depName] {
			continue
		}

		depInfo, err := disc.GetCommand(depName)
		if err != nil {
			return fmt.Errorf("dependency '%s' not found", depName)
		}

		// Check for cycle
		if depInfo.Command.DependsOn != nil {
			cmdDeps := depInfo.Command.GetCommandDependencies()
			for _, subDep := range cmdDeps {
				if executed[subDep] {
					rendered, _ := issue.Get(issue.DependencyCycleId).Render("dark")
					fmt.Fprint(os.Stderr, rendered)
					return fmt.Errorf("dependency cycle detected: %s -> %s", depName, subDep)
				}
			}

			// Check tool dependencies for this command too
			if err := checkToolDependencies(depInfo.Command); err != nil {
				return err
			}

			// Execute sub-dependencies first
			if err := executeDepsRecursive(cmdDeps, disc, registry, parentCtx, executed); err != nil {
				return err
			}
		}

		// Execute dependency
		if verbose {
			fmt.Printf("%s Running dependency '%s'...\n", subtitleStyle.Render("→"), depName)
		}

		ctx := runtime.NewExecutionContext(depInfo.Command, depInfo.Invowkfile)
		ctx.Verbose = parentCtx.Verbose

		result := registry.Execute(ctx)
		if result.Error != nil || result.ExitCode != 0 {
			return fmt.Errorf("dependency '%s' failed", depName)
		}

		executed[depName] = true
	}

	return nil
}
