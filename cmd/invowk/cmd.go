package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/issue"
	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invowkfile"
)

var (
	// runtimeOverride allows overriding the runtime for a command
	runtimeOverride string
)

// cmdCmd is the parent command for all discovered commands
var cmdCmd = &cobra.Command{
	Use:   "cmd",
	Short: "Execute commands from invowkfiles",
	Long: `Execute commands defined in invowkfiles.

Commands are discovered from:
  1. Current directory (highest priority)
  2. ~/.invowk/cmds/
  3. Configured search paths

Use 'invowk cmd list' to see all available commands.
Use 'invowk cmd <command-name>' to execute a command.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return runCommand(args)
	},
	ValidArgsFunction: completeCommands,
}

// cmdListCmd lists all available commands
var cmdListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available commands",
	Long:  "List all available commands from all discovered invowkfiles.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listCommands()
	},
}

func init() {
	cmdCmd.AddCommand(cmdListCmd)
	cmdCmd.PersistentFlags().StringVarP(&runtimeOverride, "runtime", "r", "", "override the runtime (native, virtual, container)")

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
	runtimeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))

	fmt.Println(headerStyle.Render("Available Commands"))
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
			if cmd.Command.Runtime != "" {
				line += fmt.Sprintf(" [%s]", runtimeStyle.Render(string(cmd.Command.Runtime)))
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

	// Create execution context
	ctx := runtime.NewExecutionContext(cmdInfo.Command, cmdInfo.Invowkfile)
	ctx.Verbose = verbose

	// Handle runtime override
	if runtimeOverride != "" {
		cmdInfo.Command.Runtime = invowkfile.RuntimeMode(runtimeOverride)
	}

	// Create runtime registry
	registry := createRuntimeRegistry(cfg)

	// Check for dependencies
	if err := executeDependencies(cmdInfo, registry, ctx); err != nil {
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
		registry.Register(runtime.RuntimeTypeContainer, containerRT)
	}

	return registry
}

// executeDependencies runs dependent commands first
func executeDependencies(cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext) error {
	if len(cmdInfo.Command.DependsOn) == 0 {
		return nil
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	// Track executed dependencies to detect cycles
	executed := make(map[string]bool)

	return executeDepsRecursive(cmdInfo.Command.DependsOn, disc, registry, parentCtx, executed)
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
			for _, subDep := range depInfo.Command.DependsOn {
				if executed[subDep] {
					rendered, _ := issue.Get(issue.DependencyCycleId).Render("dark")
					fmt.Fprint(os.Stderr, rendered)
					return fmt.Errorf("dependency cycle detected: %s -> %s", depName, subDep)
				}
			}

			// Execute sub-dependencies first
			if err := executeDepsRecursive(depInfo.Command.DependsOn, disc, registry, parentCtx, executed); err != nil {
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
