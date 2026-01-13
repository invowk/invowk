package cmd

import (
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
	otherRuntimeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	hostsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
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
			// Show runtimes with default highlighted
			if len(cmd.Command.Runtimes) > 0 {
				line += " ["
				for i, r := range cmd.Command.Runtimes {
					if i > 0 {
						line += ", "
					}
					if i == 0 {
						// Default runtime is highlighted and marked with *
						line += defaultRuntimeStyle.Render(string(r) + "*")
					} else {
						line += otherRuntimeStyle.Render(string(r))
					}
				}
				line += "]"
			}
			// Show supported hosts
			hostsStr := cmd.Command.GetHostsString()
			if hostsStr != "" {
				line += fmt.Sprintf(" (%s)", hostsStyle.Render(hostsStr))
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

	// Validate host OS compatibility
	if !cmdInfo.Command.CanRunOnCurrentHost() {
		currentOS := invowkfile.GetCurrentHostOS()
		supportedHosts := cmdInfo.Command.GetHostsString()
		fmt.Fprint(os.Stderr, RenderHostNotSupportedError(cmdName, string(currentOS), supportedHosts))
		rendered, _ := issue.Get(issue.HostNotSupportedId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return fmt.Errorf("command '%s' does not support host '%s' (supported: %s)", cmdName, currentOS, supportedHosts)
	}

	// Determine which runtime to use
	var selectedRuntime invowkfile.RuntimeMode
	if runtimeOverride != "" {
		// Validate that the overridden runtime is allowed for this command
		overrideRuntime := invowkfile.RuntimeMode(runtimeOverride)
		if !cmdInfo.Command.IsRuntimeAllowed(overrideRuntime) {
			allowedRuntimes := cmdInfo.Command.GetAllowedRuntimesString()
			fmt.Fprint(os.Stderr, RenderRuntimeNotAllowedError(cmdName, runtimeOverride, allowedRuntimes))
			rendered, _ := issue.Get(issue.InvalidRuntimeModeId).Render("dark")
			fmt.Fprint(os.Stderr, rendered)
			return fmt.Errorf("runtime '%s' is not allowed for command '%s' (allowed: %s)", runtimeOverride, cmdName, allowedRuntimes)
		}
		selectedRuntime = overrideRuntime
	} else {
		// Use the default runtime (first in the list)
		selectedRuntime = cmdInfo.Command.GetDefaultRuntime()
	}

	// Start SSH server if host_ssh is enabled for this command
	if cmdInfo.Command.HostSSH && selectedRuntime == invowkfile.RuntimeContainer {
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
func executeDependencies(cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext) error {
	if !cmdInfo.Command.HasDependencies() {
		return nil
	}

	// First check tool dependencies
	if err := checkToolDependencies(cmdInfo.Command); err != nil {
		return err
	}

	// Then check filepath dependencies
	if err := checkFilepathDependencies(cmdInfo.Command, cmdInfo.Invowkfile.FilePath); err != nil {
		return err
	}

	// Then run command dependencies
	cmdDeps := cmdInfo.Command.GetCommandDependencies()
	if len(cmdDeps) == 0 {
		return nil
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	// Track executed dependencies to detect cycles
	executed := make(map[string]bool)

	return executeDepsRecursive(cmdDeps, disc, registry, parentCtx, executed)
}

// checkToolDependencies verifies all required tools are available in PATH
func checkToolDependencies(cmd *invowkfile.Command) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Tools) == 0 {
		return nil
	}

	var toolErrors []string

	for _, tool := range cmd.DependsOn.Tools {
		if tool.CheckScript != "" {
			// Use custom validation script
			if err := validateToolWithScript(tool); err != nil {
				toolErrors = append(toolErrors, err.Error())
			}
		} else {
			// Just check if tool exists in PATH
			_, err := exec.LookPath(tool.Name)
			if err != nil {
				toolErrors = append(toolErrors, fmt.Sprintf("  • %s - not found in PATH", tool.Name))
			}
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

// validateToolWithScript runs a custom validation script for a tool
func validateToolWithScript(tool invowkfile.ToolDependency) error {
	// First check if the tool exists in PATH
	_, err := exec.LookPath(tool.Name)
	if err != nil {
		return fmt.Errorf("  • %s - not found in PATH", tool.Name)
	}

	// Run the check script
	cmd := exec.Command("sh", "-c", tool.CheckScript)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	// Determine expected exit code (default: 0)
	expectedCode := 0
	if tool.ExpectedCode != nil {
		expectedCode = *tool.ExpectedCode
	}

	// Check exit code
	actualCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			actualCode = exitErr.ExitCode()
		} else {
			return fmt.Errorf("  • %s - check script failed: %v", tool.Name, err)
		}
	}

	if actualCode != expectedCode {
		return fmt.Errorf("  • %s - check script returned exit code %d, expected %d", tool.Name, actualCode, expectedCode)
	}

	// Check output pattern if specified
	if tool.ExpectedOutput != "" {
		matched, err := regexp.MatchString(tool.ExpectedOutput, outputStr)
		if err != nil {
			return fmt.Errorf("  • %s - invalid regex pattern '%s': %v", tool.Name, tool.ExpectedOutput, err)
		}
		if !matched {
			return fmt.Errorf("  • %s - check script output '%s' does not match pattern '%s'", tool.Name, outputStr, tool.ExpectedOutput)
		}
	}

	return nil
}

// checkFilepathDependencies verifies all required files/directories exist with proper permissions
func checkFilepathDependencies(cmd *invowkfile.Command, invowkfilePath string) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invowkfilePath)

	for _, fp := range cmd.DependsOn.Filepaths {
		// Resolve path relative to invowkfile if not absolute
		path := fp.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(invowkDir, path)
		}

		if err := validateFilepath(fp, path); err != nil {
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

// validateFilepath checks if a filepath exists and has the required permissions
func validateFilepath(fp invowkfile.FilepathDependency, resolvedPath string) error {
	// Check if path exists
	info, err := os.Stat(resolvedPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("  • %s - path does not exist", fp.Path)
	}
	if err != nil {
		return fmt.Errorf("  • %s - cannot access path: %v", fp.Path, err)
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
		return fmt.Errorf("  • %s - missing permissions: %s", fp.Path, strings.Join(permErrors, ", "))
	}

	return nil
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

// isWindows returns true if running on Windows
func isWindows() bool {
	return os.PathSeparator == '\\' && os.PathListSeparator == ';'
}

// DependencyError represents unsatisfied dependencies
type DependencyError struct {
	CommandName      string
	MissingTools     []string
	MissingCommands  []string
	MissingFilepaths []string
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
