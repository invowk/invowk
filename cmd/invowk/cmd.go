// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
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
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const (
	// ArgErrMissingRequired indicates missing required arguments
	ArgErrMissingRequired = iota
	// ArgErrTooMany indicates too many arguments were provided
	ArgErrTooMany
	// ArgErrInvalidValue indicates an argument value failed validation
	ArgErrInvalidValue
)

var (
	// runtimeOverride allows overriding the runtime for a command
	runtimeOverride string
	// fromSource allows specifying the source for disambiguation
	fromSource string
	// sshServerInstance is the global SSH server instance
	sshServerInstance *sshserver.Server
	// sshServerMu protects the SSH server instance
	sshServerMu sync.Mutex
	// listFlag controls whether to list commands
	listFlag bool
	// cmdCmd is the parent command for all discovered commands
	cmdCmd = &cobra.Command{
		Use:   "cmd [command-name]",
		Short: "Execute commands from invkfiles",
		Long: `Execute commands defined in invkfiles and sibling modules.

Commands are discovered from:
  1. Current directory's invkfile.cue (highest priority)
  2. Sibling *.invkmod module directories
  3. ~/.invowk/cmds/
  4. Configured search paths

Commands use their simple names when unique across sources. When a command
name exists in multiple sources, disambiguation is required.

Usage:
  invowk cmd                              List all available commands
  invowk cmd <command-name>               Execute a command (if unambiguous)
  invowk cmd @<source> <command-name>     Disambiguate with @source prefix
  invowk cmd --from <source> <command-name>  Disambiguate with --from flag

Examples:
  invowk cmd build                        Run unique 'build' command
  invowk cmd @invkfile deploy             Run 'deploy' from invkfile
  invowk cmd @foo deploy                  Run 'deploy' from foo.invkmod
  invowk cmd --from invkfile deploy       Same using --from flag`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --list flag is set or no arguments, show list
			if listFlag || len(args) == 0 {
				return listCommands()
			}

			// Parse source filter from @prefix or --from flag
			filter, remainingArgs, err := ParseSourceFilter(args, fromSource)
			if err != nil {
				return err
			}

			// If we have a source filter, try to run the disambiguated command
			if filter != nil {
				return runDisambiguatedCommand(filter, remainingArgs)
			}

			// Check if the command is ambiguous (exists in multiple sources)
			// This handles the case where user types `invowk cmd deploy` and deploy is ambiguous.
			// For subcommands like `invowk cmd deploy staging`, we check progressively longer
			// command names to detect ambiguity at the correct hierarchical level.
			if len(args) > 0 {
				if ambigCheckErr := checkAmbiguousCommand(args); ambigCheckErr != nil {
					ambigErr := (*AmbiguousCommandError)(nil)
					if errors.As(ambigCheckErr, &ambigErr) {
						fmt.Fprint(os.Stderr, RenderAmbiguousCommandError(ambigErr))
						cmd.SilenceErrors = true
						cmd.SilenceUsage = true
					}
					return ambigCheckErr
				}
			}

			// No disambiguation needed, run normally (Cobra will handle registered commands)
			err = runCommand(args)
			if err != nil {
				exitErr := (*ExitError)(nil)
				if errors.As(err, &exitErr) {
					cmd.SilenceErrors = true
					cmd.SilenceUsage = true
				}
			}
			return err
		},
		ValidArgsFunction: completeCommands,
	}
)

type (
	// DependencyError represents unsatisfied dependencies
	DependencyError struct {
		CommandName         string
		MissingTools        []string
		MissingCommands     []string
		MissingFilepaths    []string
		MissingCapabilities []string
		FailedCustomChecks  []string
		MissingEnvVars      []string
	}

	// ArgErrType represents the type of argument validation error
	ArgErrType int

	// ArgumentValidationError represents an argument validation failure
	ArgumentValidationError struct {
		Type         ArgErrType
		CommandName  string
		ArgDefs      []invkfile.Argument
		ProvidedArgs []string
		MinArgs      int
		MaxArgs      int
		InvalidArg   string
		InvalidValue string
		ValueError   error
	}

	// SourceFilter represents a user-specified source constraint for disambiguation.
	// It is used to filter commands to a specific source when executing ambiguous commands.
	SourceFilter struct {
		// SourceID is the normalized source name (e.g., "foo" not "foo.invkmod")
		SourceID string
		// Raw is the original input (e.g., "@foo.invkmod" or "foo" from --from)
		Raw string
	}

	// SourceNotFoundError is returned when a specified source does not exist.
	SourceNotFoundError struct {
		Source           string
		AvailableSources []string
	}

	// AmbiguousCommandError is returned when trying to execute a command that exists
	// in multiple sources without explicit disambiguation.
	AmbiguousCommandError struct {
		CommandName string
		Sources     []string // SourceIDs where the command exists
	}
)

func init() {
	cmdCmd.Flags().BoolVarP(&listFlag, "list", "l", false, "list all available commands")
	cmdCmd.PersistentFlags().StringVarP(&runtimeOverride, "runtime", "r", "", "override the runtime (must be allowed by the command)")
	cmdCmd.PersistentFlags().StringVar(&fromSource, "from", "", "source to run command from (e.g., 'invkfile' or module name)")

	// Dynamically add discovered commands
	// This happens at init time to enable shell completion
	registerDiscoveredCommands()
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependencies not satisfied for command '%s'", e.CommandName)
}

func (e *ArgumentValidationError) Error() string {
	switch e.Type {
	case ArgErrMissingRequired:
		return fmt.Sprintf("missing required arguments for command '%s': expected at least %d, got %d", e.CommandName, e.MinArgs, len(e.ProvidedArgs))
	case ArgErrTooMany:
		return fmt.Sprintf("too many arguments for command '%s': expected at most %d, got %d", e.CommandName, e.MaxArgs, len(e.ProvidedArgs))
	case ArgErrInvalidValue:
		return fmt.Sprintf("invalid value for argument '%s': %v", e.InvalidArg, e.ValueError)
	default:
		return fmt.Sprintf("argument validation failed for command '%s'", e.CommandName)
	}
}

func (e *SourceNotFoundError) Error() string {
	return fmt.Sprintf("source '%s' not found", e.Source)
}

func (e *AmbiguousCommandError) Error() string {
	return fmt.Sprintf("command '%s' is ambiguous", e.CommandName)
}

// normalizeSourceName converts various source name formats to a canonical form.
// It accepts: "foo", "foo.invkmod", "invkfile", "invkfile.cue"
// And returns: "foo" or "invkfile"
func normalizeSourceName(raw string) string {
	// Remove @ prefix if present
	name := strings.TrimPrefix(raw, "@")

	// Handle invkfile variants
	if name == "invkfile.cue" || name == discovery.SourceIDInvkfile {
		return discovery.SourceIDInvkfile
	}

	// Handle module variants - strip .invkmod suffix
	if moduleName, found := strings.CutSuffix(name, ".invkmod"); found {
		return moduleName
	}

	return name
}

// ParseSourceFilter extracts source filter from @prefix in args or --from flag.
// Returns the filter (may be nil if no filter specified), remaining args, and any error.
// The @source prefix must be the first argument if present.
func ParseSourceFilter(args []string, fromFlag string) (*SourceFilter, []string, error) {
	// --from flag takes precedence (already parsed by Cobra)
	if fromFlag != "" {
		return &SourceFilter{
			SourceID: normalizeSourceName(fromFlag),
			Raw:      fromFlag,
		}, args, nil
	}

	// Check for @source prefix in first arg
	if len(args) > 0 && strings.HasPrefix(args[0], "@") {
		raw := args[0]
		sourceID := normalizeSourceName(raw)
		return &SourceFilter{
			SourceID: sourceID,
			Raw:      raw,
		}, args[1:], nil
	}

	// No filter specified
	return nil, args, nil
}

// registerDiscoveredCommands adds discovered commands as subcommands.
// For transparent namespace support (US2), unambiguous commands are registered
// under their SimpleName, allowing `invowk cmd build` to work when only one
// source defines "build". Ambiguous commands are NOT registered - they require
// explicit disambiguation via @source or --from.
func registerDiscoveredCommands() {
	cfg := config.Get()
	disc := discovery.New(cfg)

	commandSet, err := disc.DiscoverAndValidateCommandSet()
	if err != nil {
		// Check for args/subcommand conflict - this is now a hard error
		var conflictErr *discovery.ArgsSubcommandConflictError
		if errors.As(err, &conflictErr) {
			fmt.Fprintf(os.Stderr, "\n%s\n\n", RenderArgsSubcommandConflictError(conflictErr))
			os.Exit(1)
		}
		return // Silently fail during init for other errors
	}

	// Build command tree for commands with spaces in names
	commandMap := make(map[string]*cobra.Command)
	// Track which prefixes are ambiguous so parent commands can check
	ambiguousPrefixes := make(map[string]bool)
	for name := range commandSet.AmbiguousNames {
		ambiguousPrefixes[name] = true
	}

	// Register commands using SimpleName for unambiguous commands (transparent namespace)
	for _, cmdInfo := range commandSet.Commands {
		// Skip ambiguous commands - they require explicit disambiguation via @source or --from
		// This ensures `invowk cmd deploy` fails with a helpful message when deploy exists
		// in multiple sources, rather than silently picking one or failing obscurely
		if commandSet.AmbiguousNames[cmdInfo.SimpleName] {
			continue
		}

		registrationName := cmdInfo.SimpleName

		// Split registration name by spaces (e.g., "deploy staging" -> ["deploy", "staging"])
		parts := strings.Fields(registrationName)

		// Create parent commands if needed
		parent := cmdCmd
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
				newCmd = buildLeafCommand(cmdInfo, part)
			} else {
				// Parent command for nested structure.
				// If this parent's name is ambiguous, we need to check for ambiguity
				// and fail with a helpful message instead of showing help.
				parentPrefix := prefix // Capture for closure
				isAmbiguous := ambiguousPrefixes[parentPrefix]
				newCmd = &cobra.Command{
					Use:   part,
					Short: fmt.Sprintf("Commands under '%s'", prefix),
					RunE: func(cmd *cobra.Command, args []string) error {
						// Check if --from flag was specified
						fromFlag, _ := cmd.Flags().GetString("from")
						if fromFlag != "" {
							// Delegate to runDisambiguatedCommand with the full command path
							fullArgs := append(strings.Fields(parentPrefix), args...)
							filter := &SourceFilter{
								SourceID: normalizeSourceName(fromFlag),
								Raw:      fromFlag,
							}
							return runDisambiguatedCommand(filter, fullArgs)
						}

						// If this command name is ambiguous, show ambiguity error
						if isAmbiguous {
							cmdArgs := append(strings.Fields(parentPrefix), args...)
							if err := checkAmbiguousCommand(cmdArgs); err != nil {
								ambigErr := (*AmbiguousCommandError)(nil)
								if errors.As(err, &ambigErr) {
									fmt.Fprint(os.Stderr, RenderAmbiguousCommandError(ambigErr))
									cmd.SilenceErrors = true
									cmd.SilenceUsage = true
								}
								return err
							}
						}

						// Not ambiguous (or didn't match ambiguous pattern), show help
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

// buildLeafCommand creates a Cobra command for a leaf command (one that actually runs something).
// Extracted to reduce complexity in registerDiscoveredCommands.
func buildLeafCommand(cmdInfo *discovery.CommandInfo, cmdPart string) *cobra.Command {
	// Capture for closures - these are used in the RunE function
	cmdName := cmdInfo.Name             // Full name for command execution
	cmdSimpleName := cmdInfo.SimpleName // SimpleName for source matching
	cmdSourceID := cmdInfo.SourceID     // SourceID to check against --from filter
	cmdFlags := cmdInfo.Command.Flags   // Flags for this command
	cmdArgs := cmdInfo.Command.Args     // Positional args for this command

	// Build usage string with args
	useStr := buildCommandUsageString(cmdPart, cmdArgs)

	newCmd := &cobra.Command{
		Use:   useStr,
		Short: cmdInfo.Description,
		Long:  fmt.Sprintf("Run the '%s' command from %s", cmdInfo.Name, cmdInfo.FilePath),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if --from flag was specified (inherited from parent).
			// If so, we need to verify this command matches the requested source,
			// or delegate to the correct source's command.
			fromFlag, _ := cmd.Flags().GetString("from")
			if fromFlag != "" {
				filter := &SourceFilter{
					SourceID: normalizeSourceName(fromFlag),
					Raw:      fromFlag,
				}

				// If this command's source doesn't match the filter, we need to
				// find and execute the correct command from the requested source.
				if filter.SourceID != cmdSourceID {
					// Build the full command path from the command tree
					// (this command was routed to by Cobra based on SimpleName)
					return runDisambiguatedCommand(filter, append(strings.Fields(cmdSimpleName), args...))
				}
				// Source matches - continue to execute this command
			}

			// Extract flag values from Cobra command based on type
			flagValues := make(map[string]string)
			for _, flag := range cmdFlags {
				var val string
				var err error
				switch flag.GetType() {
				case invkfile.FlagTypeBool:
					var boolVal bool
					boolVal, err = cmd.Flags().GetBool(flag.Name)
					if err == nil {
						val = fmt.Sprintf("%t", boolVal)
					}
				case invkfile.FlagTypeInt:
					var intVal int
					intVal, err = cmd.Flags().GetInt(flag.Name)
					if err == nil {
						val = fmt.Sprintf("%d", intVal)
					}
				case invkfile.FlagTypeFloat:
					var floatVal float64
					floatVal, err = cmd.Flags().GetFloat64(flag.Name)
					if err == nil {
						val = fmt.Sprintf("%g", floatVal)
					}
				case invkfile.FlagTypeString:
					val, err = cmd.Flags().GetString(flag.Name)
				}
				if err == nil {
					flagValues[flag.Name] = val
				}
			}

			// Extract --env-file flag values
			envFiles, _ := cmd.Flags().GetStringArray("env-file")

			// Extract --env-var flag values and parse KEY=VALUE pairs
			envVarFlags, _ := cmd.Flags().GetStringArray("env-var")
			envVars := parseEnvVarFlags(envVarFlags)

			// Extract --workdir flag value
			workdirOverride, _ := cmd.Flags().GetString("workdir")

			// Extract env inherit flags
			envInheritMode, _ := cmd.Flags().GetString("env-inherit-mode")
			envInheritAllow, _ := cmd.Flags().GetStringArray("env-inherit-allow")
			envInheritDeny, _ := cmd.Flags().GetStringArray("env-inherit-deny")

			err := runCommandWithFlags(cmdName, args, flagValues, cmdFlags, cmdArgs, envFiles, envVars, workdirOverride, envInheritMode, envInheritAllow, envInheritDeny)
			if err != nil {
				var exitErr *ExitError
				if errors.As(err, &exitErr) {
					cmd.SilenceErrors = true
					cmd.SilenceUsage = true
				}
			}
			return err
		},
		Args: buildCobraArgsValidator(cmdArgs),
	}

	// Add the reserved --env-file flag for loading environment variables from files
	newCmd.Flags().StringArrayP("env-file", "e", nil, "load environment variables from file(s) (can be specified multiple times)")

	// Add the reserved --env-var flag for setting environment variables
	newCmd.Flags().StringArrayP("env-var", "E", nil, "set environment variable (KEY=VALUE, can be specified multiple times)")

	// Add the reserved flags for host env inheritance
	newCmd.Flags().String("env-inherit-mode", "", "inherit host environment variables: none, allow, all (overrides runtime config)")
	newCmd.Flags().StringArray("env-inherit-allow", nil, "allowlist for host environment inheritance (repeatable)")
	newCmd.Flags().StringArray("env-inherit-deny", nil, "denylist for host environment inheritance (repeatable)")

	// Add the reserved --workdir flag for overriding working directory
	newCmd.Flags().StringP("workdir", "w", "", "override the working directory for this command")

	// Add arguments documentation to Long description
	if len(cmdArgs) > 0 {
		newCmd.Long += "\n\nArguments:\n" + buildArgsDocumentation(cmdArgs)
	}

	// Add flags defined in the command with proper types and short aliases
	for _, flag := range cmdFlags {
		switch flag.GetType() {
		case invkfile.FlagTypeBool:
			defaultVal := flag.DefaultValue == "true"
			if flag.Short != "" {
				newCmd.Flags().BoolP(flag.Name, flag.Short, defaultVal, flag.Description)
			} else {
				newCmd.Flags().Bool(flag.Name, defaultVal, flag.Description)
			}
		case invkfile.FlagTypeInt:
			defaultVal := 0
			if flag.DefaultValue != "" {
				_, _ = fmt.Sscanf(flag.DefaultValue, "%d", &defaultVal) // Parse error uses 0
			}
			if flag.Short != "" {
				newCmd.Flags().IntP(flag.Name, flag.Short, defaultVal, flag.Description)
			} else {
				newCmd.Flags().Int(flag.Name, defaultVal, flag.Description)
			}
		case invkfile.FlagTypeFloat:
			defaultVal := 0.0
			if flag.DefaultValue != "" {
				_, _ = fmt.Sscanf(flag.DefaultValue, "%f", &defaultVal) // Parse error uses 0.0
			}
			if flag.Short != "" {
				newCmd.Flags().Float64P(flag.Name, flag.Short, defaultVal, flag.Description)
			} else {
				newCmd.Flags().Float64(flag.Name, defaultVal, flag.Description)
			}
		case invkfile.FlagTypeString:
			if flag.Short != "" {
				newCmd.Flags().StringP(flag.Name, flag.Short, flag.DefaultValue, flag.Description)
			} else {
				newCmd.Flags().String(flag.Name, flag.DefaultValue, flag.Description)
			}
		}
		// Mark required flags
		if flag.Required {
			_ = newCmd.MarkFlagRequired(flag.Name)
		}
	}

	return newCmd
}

// buildCommandUsageString builds the Cobra Use string including argument placeholders
func buildCommandUsageString(cmdPart string, args []invkfile.Argument) string {
	if len(args) == 0 {
		return cmdPart
	}

	var parts []string
	parts = append(parts, cmdPart)

	for _, arg := range args {
		var argStr string
		switch {
		case arg.Variadic && arg.Required:
			argStr = fmt.Sprintf("<%s>...", arg.Name)
		case arg.Variadic:
			argStr = fmt.Sprintf("[%s]...", arg.Name)
		case arg.Required:
			argStr = fmt.Sprintf("<%s>", arg.Name)
		default:
			argStr = fmt.Sprintf("[%s]", arg.Name)
		}
		parts = append(parts, argStr)
	}

	return strings.Join(parts, " ")
}

// buildArgsDocumentation builds the documentation string for arguments
func buildArgsDocumentation(args []invkfile.Argument) string {
	var lines []string
	for _, arg := range args {
		var status string
		switch {
		case arg.Required:
			status = "(required)"
		case arg.DefaultValue != "":
			status = fmt.Sprintf("(default: %q)", arg.DefaultValue)
		default:
			status = "(optional)"
		}

		var typeInfo string
		if arg.Type != "" && arg.Type != invkfile.ArgumentTypeString {
			typeInfo = fmt.Sprintf(" [%s]", arg.Type)
		}

		variadicInfo := ""
		if arg.Variadic {
			variadicInfo = " (variadic)"
		}

		lines = append(lines, fmt.Sprintf("  %-20s %s%s%s - %s", arg.Name, status, typeInfo, variadicInfo, arg.Description))
	}
	return strings.Join(lines, "\n")
}

// buildCobraArgsValidator creates a Cobra Args validator function for the given argument definitions.
// It delegates to validateArguments for the actual validation logic.
func buildCobraArgsValidator(argDefs []invkfile.Argument) cobra.PositionalArgs {
	if len(argDefs) == 0 {
		return cobra.ArbitraryArgs // Backward compatible: allow any args if none defined
	}
	return func(cmd *cobra.Command, args []string) error {
		return validateArguments(cmd.Name(), args, argDefs)
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
		rendered, _ := issue.Get(issue.InvkfileNotFoundId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return err
	}

	// Check if we found any files at all
	if len(files) == 0 {
		rendered, _ := issue.Get(issue.InvkfileNotFoundId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return fmt.Errorf("no invkfile found")
	}

	// Count files with errors and show them
	filesWithErrors := 0
	for _, file := range files {
		if file.Error != nil {
			filesWithErrors++
			fmt.Fprintf(os.Stderr, "%s Failed to parse %s: %v\n", errorStyle.Render("✗"), file.Path, file.Error)
		}
	}

	// If ALL files have errors, show the parse error issue
	if filesWithErrors == len(files) {
		rendered, _ := issue.Get(issue.InvkfileParseErrorId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return fmt.Errorf("all invkfiles have parsing errors")
	}

	// Use DiscoverCommandSet to get grouped commands with conflict detection
	commandSet, err := disc.DiscoverCommandSet()
	if err != nil {
		// If we have files but DiscoverCommandSet fails, it's likely a parse error
		if filesWithErrors > 0 {
			rendered, _ := issue.Get(issue.InvkfileParseErrorId).Render("dark")
			fmt.Fprint(os.Stderr, rendered)
		} else {
			rendered, _ := issue.Get(issue.InvkfileNotFoundId).Render("dark")
			fmt.Fprint(os.Stderr, rendered)
		}
		return err
	}

	if len(commandSet.Commands) == 0 {
		// If we have files with errors and no commands, show parse error
		if filesWithErrors > 0 {
			rendered, _ := issue.Get(issue.InvkfileParseErrorId).Render("dark")
			fmt.Fprint(os.Stderr, rendered)
		} else {
			// Files parsed successfully but no commands defined
			rendered, _ := issue.Get(issue.InvkfileNotFoundId).Render("dark")
			fmt.Fprint(os.Stderr, rendered)
		}
		return fmt.Errorf("no commands found")
	}

	// Style for output
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	verboseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	verboseHighlightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

	// Verbose mode: show discovery source details (FR-013)
	if GetVerbose() {
		fmt.Println(headerStyle.Render("Discovery Sources"))
		fmt.Println()

		// Count sources by type and show details
		var invkfileCount, moduleCount int
		for _, file := range files {
			if file.Error != nil {
				continue
			}
			if file.Module != nil {
				moduleCount++
			} else {
				invkfileCount++
			}
		}

		// Show each discovered file with its source type
		for _, file := range files {
			if file.Error != nil {
				continue
			}
			var sourceType string
			if file.Module != nil {
				sourceType = fmt.Sprintf("module (%s)", file.Module.Name())
			} else {
				sourceType = file.Source.String()
			}
			fmt.Printf("  %s %s\n",
				verboseHighlightStyle.Render("•"),
				verboseStyle.Render(fmt.Sprintf("%s [%s]", file.Path, sourceType)))
		}
		fmt.Println()

		// Show summary
		fmt.Printf("  %s\n",
			verboseStyle.Render(fmt.Sprintf("Sources: %d invkfile(s), %d module(s)", invkfileCount, moduleCount)))
		fmt.Printf("  %s\n",
			verboseStyle.Render(fmt.Sprintf("Commands: %d total (%d ambiguous)",
				len(commandSet.Commands), len(commandSet.AmbiguousNames))))
		fmt.Println()
	}
	sourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	defaultRuntimeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	platformsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	legendStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Italic(true)
	ambiguousStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	fmt.Println(headerStyle.Render("Available Commands"))
	fmt.Println(legendStyle.Render("  (* = default runtime)"))
	fmt.Println()

	// Iterate in source order (invkfile first, then modules alphabetically)
	for _, sourceID := range commandSet.SourceOrder {
		cmds := commandSet.BySource[sourceID]
		if len(cmds) == 0 {
			continue
		}

		// Format source header: "From invkfile:" or "From foo.invkmod:"
		sourceDisplay := formatSourceDisplayName(sourceID)
		fmt.Println(sourceStyle.Render(fmt.Sprintf("From %s:", sourceDisplay)))

		for _, cmd := range cmds {
			// Display SimpleName (unprefixed) instead of full Name (which may be prefixed)
			line := fmt.Sprintf("  %s", nameStyle.Render(cmd.SimpleName))
			if cmd.Description != "" {
				line += fmt.Sprintf(" - %s", descStyle.Render(cmd.Description))
			}
			// Show ambiguity annotation if this command name conflicts with another source
			if cmd.IsAmbiguous {
				line += fmt.Sprintf(" %s", ambiguousStyle.Render("(@"+sourceID+")"))
			}
			// Show runtimes with default highlighted for current platform
			currentPlatform := invkfile.GetCurrentHostOS()
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

// formatSourceDisplayName converts a SourceID to a user-friendly display name.
// "invkfile" -> "invkfile"
// "foo" -> "foo.invkmod"
// "current directory" -> "current directory" (legacy non-module sources)
func formatSourceDisplayName(sourceID string) string {
	if sourceID == discovery.SourceIDInvkfile {
		return discovery.SourceIDInvkfile
	}
	// Check if it's a legacy source type (from Source.String())
	// These have spaces and don't need .invkmod suffix
	if strings.Contains(sourceID, " ") {
		return sourceID
	}
	// Module source - add .invkmod suffix
	return sourceID + ".invkmod"
}

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
			fmt.Printf("%s SSH server started on %s for host access\n", successStyle.Render("→"), srv.Address())
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
		fmt.Printf("%s Running '%s'...\n", successStyle.Render("→"), cmdName)
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
				fmt.Printf("%s Runtime '%s' does not support interactive mode, using standard execution\n",
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
			fmt.Printf("%s Command exited with code %d\n", warningStyle.Render("!"), result.ExitCode)
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

// validateDependencies validates merged dependencies for a command.
// Dependencies are merged from root-level, command-level, and implementation-level, and
// validated according to the selected runtime:
// - native: validated against the native standard shell from the host
// - virtual: validated against invowk's built-in sh interpreter with core utils
// - container: validated against the container's default shell from within the container
//
// Note: `depends_on.cmds` is an existence check only. Invowk validates that referenced
// commands are discoverable (in this invkfile, modules, or configured search paths),
// but it does not execute them automatically.
func validateDependencies(cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext) error {
	// Merge root-level, command-level, and implementation-level dependencies
	mergedDeps := invkfile.MergeDependsOnAll(cmdInfo.Invkfile.DependsOn, cmdInfo.Command.DependsOn, parentCtx.SelectedImpl.DependsOn)

	if mergedDeps == nil {
		return nil
	}

	// Get the selected runtime for context-aware validation
	selectedRuntime := parentCtx.SelectedRuntime

	// FIRST: Check env var dependencies (host-only, validated BEFORE invowk sets any env vars)
	// We capture the user's environment here to ensure we validate against their actual env,
	// not any variables that invowk might set from the 'env' construct
	if err := checkEnvVarDependencies(mergedDeps, captureUserEnv(), parentCtx); err != nil {
		return err
	}

	// Then check tool dependencies (runtime-aware)
	if err := checkToolDependenciesWithRuntime(mergedDeps, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then check filepath dependencies (runtime-aware)
	if err := checkFilepathDependenciesWithRuntime(mergedDeps, cmdInfo.Invkfile.FilePath, selectedRuntime, registry, parentCtx); err != nil {
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

	// Then check command dependencies (existence-only; these are not executed automatically)
	// Get module ID from metadata (nil for non-module invkfiles)
	currentModule := ""
	if cmdInfo.Invkfile.Metadata != nil {
		currentModule = cmdInfo.Invkfile.Metadata.Module
	}
	if err := checkCommandDependenciesExist(mergedDeps, currentModule, parentCtx); err != nil {
		return err
	}

	return nil
}

func checkCommandDependenciesExist(deps *invkfile.DependsOn, currentModule string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Commands) == 0 {
		return nil
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	availableCommands, err := disc.DiscoverCommands()
	if err != nil {
		return fmt.Errorf("failed to discover commands for dependency validation: %w", err)
	}

	available := make(map[string]struct{}, len(availableCommands))
	for _, cmd := range availableCommands {
		available[cmd.Name] = struct{}{}
	}

	var commandErrors []string

	for _, dep := range deps.Commands {
		var alternatives []string
		for _, alt := range dep.Alternatives {
			alt = strings.TrimSpace(alt)
			if alt == "" {
				continue
			}
			alternatives = append(alternatives, alt)
		}
		if len(alternatives) == 0 {
			continue
		}

		// OR semantics: any alternative being discoverable satisfies this dependency.
		found := false
		for _, alt := range alternatives {
			if _, ok := available[alt]; ok {
				found = true
				break
			}

			// Also allow referencing commands from the current invkfile without a module prefix.
			qualified := currentModule + " " + alt
			if _, ok := available[qualified]; ok {
				found = true
				break
			}
		}

		if !found {
			if len(alternatives) == 1 {
				commandErrors = append(commandErrors, fmt.Sprintf("  • %s - command not found", alternatives[0]))
			} else {
				commandErrors = append(commandErrors, fmt.Sprintf("  • none of [%s] found", strings.Join(alternatives, ", ")))
			}
		}
	}

	if len(commandErrors) > 0 {
		return &DependencyError{
			CommandName:     ctx.Command.Name,
			MissingCommands: commandErrors,
		}
	}

	return nil
}

// checkToolDependenciesWithRuntime verifies all required tools are available
// The validation method depends on the runtime:
// - native: check against host system PATH
// - virtual: check against built-in utilities
// - container: check within the container environment
// Each ToolDependency has alternatives with OR semantics (any alternative found satisfies the dependency)
func checkToolDependenciesWithRuntime(deps *invkfile.DependsOn, runtimeMode invkfile.RuntimeMode, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Tools) == 0 {
		return nil
	}

	var toolErrors []string

	for _, tool := range deps.Tools {
		// OR semantics: try each alternative until one succeeds
		var lastErr error
		found := false
		for _, alt := range tool.Alternatives {
			var err error
			switch runtimeMode {
			case invkfile.RuntimeContainer:
				err = validateToolInContainer(alt, registry, ctx)
			case invkfile.RuntimeVirtual:
				err = validateToolInVirtual(alt, registry, ctx)
			case invkfile.RuntimeNative:
				err = validateToolNative(alt)
			}
			if err == nil {
				found = true
				break // Early return on first match
			}
			lastErr = err
		}
		if !found && lastErr != nil {
			if len(tool.Alternatives) == 1 {
				toolErrors = append(toolErrors, lastErr.Error())
			} else {
				toolErrors = append(toolErrors, fmt.Sprintf("  • none of [%s] found", strings.Join(tool.Alternatives, ", ")))
			}
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

// validateToolNative validates a tool dependency against the host system PATH.
// It accepts a tool name string and checks if it exists in the system PATH.
func validateToolNative(toolName string) error {
	_, err := exec.LookPath(toolName)
	if err != nil {
		return fmt.Errorf("  • %s - not found in PATH", toolName)
	}
	return nil
}

// validateToolInVirtual validates a tool dependency using the virtual runtime.
// It accepts a tool name string and checks if it exists in the virtual shell environment.
func validateToolInVirtual(toolName string, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeVirtual)
	if err != nil {
		// Fall back to native validation if virtual runtime not available
		return validateToolNative(toolName)
	}

	// Use 'command -v' to check if tool exists in virtual shell
	checkScript := fmt.Sprintf("command -v %s", toolName)

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invkfile:        ctx.Invkfile,
		SelectedImpl:    &invkfile.Implementation{Script: checkScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeVirtual}}},
		SelectedRuntime: invkfile.RuntimeVirtual,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)

	if result.ExitCode != 0 {
		return fmt.Errorf("  • %s - not available in virtual runtime", toolName)
	}
	return nil
}

// validateToolInContainer validates a tool dependency within a container.
// It accepts a tool name string and checks if it exists in the container environment.
func validateToolInContainer(toolName string, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • %s - container runtime not available", toolName)
	}

	// Use 'command -v' or 'which' to check if tool exists in container
	checkScript := fmt.Sprintf("command -v %s || which %s", toolName, toolName)

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invkfile:        ctx.Invkfile,
		SelectedImpl:    &invkfile.Implementation{Script: checkScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}}},
		SelectedRuntime: invkfile.RuntimeContainer,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)

	if result.ExitCode != 0 {
		return fmt.Errorf("  • %s - not available in container", toolName)
	}
	return nil
}

// validateCustomCheckOutput validates custom check script output against expected values
func validateCustomCheckOutput(check invkfile.CustomCheck, outputStr string, execErr error) error {
	// Determine expected exit code (default: 0)
	expectedCode := 0
	if check.ExpectedCode != nil {
		expectedCode = *check.ExpectedCode
	}

	// Check exit code
	actualCode := 0
	if execErr != nil {
		var exitErr *exec.ExitError
		if errors.As(execErr, &exitErr) {
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
			return fmt.Errorf("  • %s - invalid regex pattern '%s': %w", check.Name, check.ExpectedOutput, err)
		}
		if !matched {
			return fmt.Errorf("  • %s - check script output '%s' does not match pattern '%s'", check.Name, outputStr, check.ExpectedOutput)
		}
	}

	return nil
}

// checkCustomCheckDependencies validates all custom check scripts.
// The validation method depends on the runtime:
// - native: executed using the host's native shell
// - virtual: executed using invowk's built-in sh interpreter
// - container: executed within the container environment
// Each CustomCheckDependency can be either a direct check or a list of alternatives.
// For alternatives, OR semantics are used (early return on first passing check).
func checkCustomCheckDependencies(deps *invkfile.DependsOn, runtimeMode invkfile.RuntimeMode, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []string

	for _, checkDep := range deps.CustomChecks {
		checks := checkDep.GetChecks()
		var lastErr error
		passed := false

		for _, check := range checks {
			var err error
			switch runtimeMode {
			case invkfile.RuntimeContainer:
				err = validateCustomCheckInContainer(check, registry, ctx)
			case invkfile.RuntimeVirtual:
				err = validateCustomCheckInVirtual(check, registry, ctx)
			case invkfile.RuntimeNative:
				err = validateCustomCheckNative(check)
			}
			if err == nil {
				passed = true
				break // Early return on first passing check
			}
			lastErr = err
		}

		if !passed && lastErr != nil {
			if len(checks) == 1 {
				checkErrors = append(checkErrors, lastErr.Error())
			} else {
				// Collect all check names for the error message
				names := make([]string, len(checks))
				for i, c := range checks {
					names[i] = c.Name
				}
				checkErrors = append(checkErrors, fmt.Sprintf("  • none of custom checks [%s] passed", strings.Join(names, ", ")))
			}
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
func validateCustomCheckNative(check invkfile.CustomCheck) error {
	cmd := exec.CommandContext(context.Background(), "sh", "-c", check.CheckScript)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	return validateCustomCheckOutput(check, outputStr, err)
}

// validateCustomCheckInVirtual runs a custom check script using the virtual runtime
func validateCustomCheckInVirtual(check invkfile.CustomCheck, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeVirtual)
	if err != nil {
		// Fall back to native validation if virtual runtime not available
		return validateCustomCheckNative(check)
	}

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invkfile:        ctx.Invkfile,
		SelectedImpl:    &invkfile.Implementation{Script: check.CheckScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeVirtual}}},
		SelectedRuntime: invkfile.RuntimeVirtual,
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
func validateCustomCheckInContainer(check invkfile.CustomCheck, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • %s - container runtime not available", check.Name)
	}

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invkfile:        ctx.Invkfile,
		SelectedImpl:    &invkfile.Implementation{Script: check.CheckScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}}},
		SelectedRuntime: invkfile.RuntimeContainer,
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
func checkFilepathDependenciesWithRuntime(deps *invkfile.DependsOn, invkfilePath string, runtimeMode invkfile.RuntimeMode, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invkfilePath)

	for _, fp := range deps.Filepaths {
		var err error
		switch runtimeMode {
		case invkfile.RuntimeContainer:
			err = validateFilepathInContainer(fp, invowkDir, registry, ctx)
		case invkfile.RuntimeNative, invkfile.RuntimeVirtual:
			// Native and virtual use host filesystem
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
func validateFilepathInContainer(fp invkfile.FilepathDependency, invowkDir string, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
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
			Invkfile:        ctx.Invkfile,
			SelectedImpl:    &invkfile.Implementation{Script: checkScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}}},
			SelectedRuntime: invkfile.RuntimeContainer,
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
// checkToolDependencies verifies all required tools are available (legacy - uses native only).
// Each ToolDependency contains a list of alternatives; if any alternative is found, the dependency is satisfied.
func checkToolDependencies(cmd *invkfile.Command) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Tools) == 0 {
		return nil
	}

	var toolErrors []string

	for _, tool := range cmd.DependsOn.Tools {
		var lastErr error
		found := false
		for _, alt := range tool.Alternatives {
			if err := validateToolNative(alt); err == nil {
				found = true
				break // Early return on first match
			} else {
				lastErr = err
			}
		}
		if !found && lastErr != nil {
			if len(tool.Alternatives) == 1 {
				toolErrors = append(toolErrors, lastErr.Error())
			} else {
				toolErrors = append(toolErrors, fmt.Sprintf("  • none of [%s] found", strings.Join(tool.Alternatives, ", ")))
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

// checkCustomChecks verifies all custom check scripts pass (legacy - uses native).
// Each CustomCheckDependency can be either a direct check or a list of alternatives.
// For alternatives, OR semantics are used (early return on first passing check).
func checkCustomChecks(cmd *invkfile.Command) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []string

	for _, checkDep := range cmd.DependsOn.CustomChecks {
		checks := checkDep.GetChecks()
		var lastErr error
		passed := false

		for _, check := range checks {
			if err := validateCustomCheckNative(check); err == nil {
				passed = true
				break // Early return on first passing check
			} else {
				lastErr = err
			}
		}

		if !passed && lastErr != nil {
			if len(checks) == 1 {
				checkErrors = append(checkErrors, lastErr.Error())
			} else {
				names := make([]string, len(checks))
				for i, c := range checks {
					names[i] = c.Name
				}
				checkErrors = append(checkErrors, fmt.Sprintf("  • none of custom checks [%s] passed", strings.Join(names, ", ")))
			}
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
func checkFilepathDependencies(cmd *invkfile.Command, invkfilePath string) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invkfilePath)

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
func validateFilepathAlternatives(fp invkfile.FilepathDependency, invowkDir string) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		// Resolve path relative to invkfile if not absolute
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
func validateSingleFilepath(displayPath, resolvedPath string, fp invkfile.FilepathDependency) error {
	// Check if path exists
	info, err := os.Stat(resolvedPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("path does not exist")
	}
	if err != nil {
		return fmt.Errorf("cannot access path: %w", err)
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

// isReadable checks if a path is readable (cross-platform)
func isReadable(path string, info os.FileInfo) bool {
	// Try to open the file/directory for reading
	if info.IsDir() {
		f, err := os.Open(path)
		if err != nil {
			return false
		}
		_ = f.Close() // Readability check; close error non-critical
		return true
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close() // Readability check; close error non-critical
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
		_ = f.Close()           // Test file; error non-critical
		_ = os.Remove(testFile) // Cleanup test file; error non-critical
		return true
	}
	// For files, try to open for writing
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close() // Probe only; error non-critical
	return true
}

// isExecutable checks if a path is executable (cross-platform)
func isExecutable(path string, info os.FileInfo) bool {
	// On Windows, check file extension
	if isWindows() {
		ext := strings.ToLower(filepath.Ext(path))
		execExts := []string{".exe", ".cmd", ".bat", ".com", ".ps1"}
		if slices.Contains(execExts, ext) {
			return true
		}
		// Also check PATHEXT environment variable
		pathext := os.Getenv("PATHEXT")
		if pathext != "" {
			pathExtList := strings.Split(strings.ToLower(pathext), ";")
			if slices.Contains(pathExtList, ext) {
				return true
			}
		}
		return false
	}

	// On Unix-like systems, check execute permission bit
	mode := info.Mode()
	return mode&0o111 != 0
}

// checkCapabilityDependencies verifies all required system capabilities are available.
// Capabilities are always checked against the host system, regardless of the runtime mode.
// For container runtimes, these checks represent the host's capabilities, not the container's.
// Each CapabilityDependency contains a list of alternatives; if any alternative is satisfied, the dependency is met.
func checkCapabilityDependencies(deps *invkfile.DependsOn, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Capabilities) == 0 {
		return nil
	}

	var capabilityErrors []string

	// Track seen capability sets to detect duplicates (they're just skipped, not an error)
	seen := make(map[string]bool)

	for _, cap := range deps.Capabilities {
		// Create a unique key for this set of alternatives
		key := strings.Join(func() []string {
			s := make([]string, len(cap.Alternatives))
			for i, alt := range cap.Alternatives {
				s[i] = string(alt)
			}
			return s
		}(), ",")

		// Skip duplicates
		if seen[key] {
			continue
		}
		seen[key] = true

		var lastErr error
		found := false
		for _, alt := range cap.Alternatives {
			if err := invkfile.CheckCapability(alt); err == nil {
				found = true
				break // Early return on first match
			} else {
				lastErr = err
			}
		}

		if !found && lastErr != nil {
			if len(cap.Alternatives) == 1 {
				capabilityErrors = append(capabilityErrors, fmt.Sprintf("  • %s", lastErr.Error()))
			} else {
				alts := make([]string, len(cap.Alternatives))
				for i, alt := range cap.Alternatives {
					alts[i] = string(alt)
				}
				capabilityErrors = append(capabilityErrors, fmt.Sprintf("  • none of capabilities [%s] satisfied", strings.Join(alts, ", ")))
			}
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

// checkEnvVarDependencies verifies all required environment variables exist.
// IMPORTANT: This function validates against the provided userEnv map, which should be captured
// at the START of execution before invowk sets any command-level env vars.
// This ensures the check validates the user's actual environment, not variables set by invowk.
// Each EnvVarDependency contains alternatives with OR semantics (early return on first match).
func checkEnvVarDependencies(deps *invkfile.DependsOn, userEnv map[string]string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.EnvVars) == 0 {
		return nil
	}

	var envVarErrors []string

	for _, envVar := range deps.EnvVars {
		var lastErr error
		found := false

		for _, alt := range envVar.Alternatives {
			// Trim whitespace from name as per schema
			name := strings.TrimSpace(alt.Name)
			if name == "" {
				lastErr = fmt.Errorf("  • (empty) - environment variable name cannot be empty")
				continue
			}

			// Check if env var exists
			value, exists := userEnv[name]
			if !exists {
				lastErr = fmt.Errorf("  • %s - not set in environment", name)
				continue
			}

			// If validation pattern is specified, validate the value
			if alt.Validation != "" {
				matched, err := regexp.MatchString(alt.Validation, value)
				if err != nil {
					lastErr = fmt.Errorf("  • %s - invalid validation regex '%s': %w", name, alt.Validation, err)
					continue
				}
				if !matched {
					lastErr = fmt.Errorf("  • %s - value '%s' does not match required pattern '%s'", name, value, alt.Validation)
					continue
				}
			}

			// Env var exists and passes validation (if any)
			found = true
			break // Early return on first match
		}

		if !found && lastErr != nil {
			if len(envVar.Alternatives) == 1 {
				envVarErrors = append(envVarErrors, lastErr.Error())
			} else {
				// Collect all alternative names for the error message
				names := make([]string, len(envVar.Alternatives))
				for i, alt := range envVar.Alternatives {
					names[i] = strings.TrimSpace(alt.Name)
				}
				envVarErrors = append(envVarErrors, fmt.Sprintf("  • none of [%s] found or passed validation", strings.Join(names, ", ")))
			}
		}
	}

	if len(envVarErrors) > 0 {
		return &DependencyError{
			CommandName:    ctx.Command.Name,
			MissingEnvVars: envVarErrors,
		}
	}

	return nil
}

// captureUserEnv captures the current environment as a map.
// This should be called at the start of execution to capture the user's
// actual environment before invowk sets any command-level env vars.
func captureUserEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		if key, value, found := strings.Cut(e, "="); found {
			env[key] = value
		}
	}
	return env
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return os.PathSeparator == '\\' && os.PathListSeparator == ';'
}

// FlagNameToEnvVar converts a flag name to an environment variable name.
// Example: "output-file" -> "INVOWK_FLAG_OUTPUT_FILE"
func FlagNameToEnvVar(name string) string {
	// Replace hyphens with underscores and convert to uppercase
	envName := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	return "INVOWK_FLAG_" + envName
}

// ArgNameToEnvVar converts an argument name to an environment variable name.
// Example: "output-file" -> "INVOWK_ARG_OUTPUT_FILE"
func ArgNameToEnvVar(name string) string {
	// Replace hyphens with underscores and convert to uppercase
	envName := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	return "INVOWK_ARG_" + envName
}

// validateFlagValues validates flag values at runtime.
// It checks that required flags are provided and validates values against type and regex patterns.
func validateFlagValues(cmdName string, flagValues map[string]string, flagDefs []invkfile.Flag) error {
	if flagDefs == nil {
		return nil
	}

	var validationErrs []string

	for _, flag := range flagDefs {
		value, hasValue := flagValues[flag.Name]

		// Check required flags
		// Note: Cobra handles required flag checking via MarkFlagRequired,
		// but we double-check here for runtime validation (legacy calls)
		if flag.Required && (!hasValue || value == "") {
			validationErrs = append(validationErrs, fmt.Sprintf("required flag '--%s' was not provided", flag.Name))
			continue
		}

		// Validate the value if provided (skip empty values for non-required flags)
		if hasValue && value != "" {
			if err := flag.ValidateFlagValue(value); err != nil {
				validationErrs = append(validationErrs, err.Error())
			}
		}
	}

	if len(validationErrs) > 0 {
		return fmt.Errorf("flag validation failed for command '%s':\n  %s", cmdName, strings.Join(validationErrs, "\n  "))
	}

	return nil
}

// validateArguments validates provided arguments against their definitions.
// It returns an *ArgumentValidationError if validation fails.
func validateArguments(cmdName string, providedArgs []string, argDefs []invkfile.Argument) error {
	if len(argDefs) == 0 {
		return nil // No argument definitions, allow any args (backward compatible)
	}

	// Count required args and check for variadic
	minArgs := 0
	maxArgs := len(argDefs)
	hasVariadic := false

	for _, arg := range argDefs {
		if arg.Required {
			minArgs++
		}
		if arg.Variadic {
			hasVariadic = true
		}
	}

	// Check minimum args
	if len(providedArgs) < minArgs {
		return &ArgumentValidationError{
			Type:         ArgErrMissingRequired,
			CommandName:  cmdName,
			ArgDefs:      argDefs,
			ProvidedArgs: providedArgs,
			MinArgs:      minArgs,
			MaxArgs:      maxArgs,
		}
	}

	// Check maximum args (only if not variadic)
	if !hasVariadic && len(providedArgs) > maxArgs {
		return &ArgumentValidationError{
			Type:         ArgErrTooMany,
			CommandName:  cmdName,
			ArgDefs:      argDefs,
			ProvidedArgs: providedArgs,
			MinArgs:      minArgs,
			MaxArgs:      maxArgs,
		}
	}

	// Validate each provided argument
	for i, argValue := range providedArgs {
		if i >= len(argDefs) {
			// Extra args go to the last (variadic) argument - already validated to have one
			break
		}

		argDef := argDefs[i]

		// For variadic args, validate all remaining values
		if argDef.Variadic {
			for j := i; j < len(providedArgs); j++ {
				if err := argDef.ValidateArgumentValue(providedArgs[j]); err != nil {
					return &ArgumentValidationError{
						Type:         ArgErrInvalidValue,
						CommandName:  cmdName,
						ArgDefs:      argDefs,
						ProvidedArgs: providedArgs,
						InvalidArg:   argDef.Name,
						InvalidValue: providedArgs[j],
						ValueError:   err,
					}
				}
			}
			break
		}

		// Validate non-variadic argument
		if err := argDef.ValidateArgumentValue(argValue); err != nil {
			return &ArgumentValidationError{
				Type:         ArgErrInvalidValue,
				CommandName:  cmdName,
				ArgDefs:      argDefs,
				ProvidedArgs: providedArgs,
				InvalidArg:   argDef.Name,
				InvalidValue: argValue,
				ValueError:   err,
			}
		}
	}

	return nil
}

// RenderArgumentValidationError creates a styled error message for argument validation failures
func RenderArgumentValidationError(err *ArgumentValidationError) string {
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

	switch err.Type {
	case ArgErrMissingRequired:
		sb.WriteString(headerStyle.Render("✗ Missing required arguments!"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Command %s requires at least %d argument(s), but got %d.\n\n",
			commandStyle.Render("'"+err.CommandName+"'"), err.MinArgs, len(err.ProvidedArgs)))

		sb.WriteString(labelStyle.Render("Expected arguments:"))
		sb.WriteString("\n")
		for _, arg := range err.ArgDefs {
			var reqStr string
			switch {
			case arg.Required:
				reqStr = " (required)"
			case arg.DefaultValue != "":
				reqStr = fmt.Sprintf(" (default: %q)", arg.DefaultValue)
			default:
				reqStr = " (optional)"
			}
			sb.WriteString(valueStyle.Render(fmt.Sprintf("  • %s%s - %s\n", arg.Name, reqStr, arg.Description)))
		}

	case ArgErrTooMany:
		sb.WriteString(headerStyle.Render("✗ Too many arguments!"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Command %s accepts at most %d argument(s), but got %d.\n\n",
			commandStyle.Render("'"+err.CommandName+"'"), err.MaxArgs, len(err.ProvidedArgs)))

		sb.WriteString(labelStyle.Render("Expected arguments:"))
		sb.WriteString("\n")
		for _, arg := range err.ArgDefs {
			sb.WriteString(valueStyle.Render(fmt.Sprintf("  • %s - %s\n", arg.Name, arg.Description)))
		}
		sb.WriteString("\n")
		sb.WriteString(labelStyle.Render("Provided:"))
		sb.WriteString(valueStyle.Render(fmt.Sprintf(" %v", err.ProvidedArgs)))

	case ArgErrInvalidValue:
		sb.WriteString(headerStyle.Render("✗ Invalid argument value!"))
		sb.WriteString("\n\n")
		sb.WriteString(fmt.Sprintf("Command %s received an invalid value for argument %s.\n\n",
			commandStyle.Render("'"+err.CommandName+"'"), commandStyle.Render("'"+err.InvalidArg+"'")))

		sb.WriteString(labelStyle.Render("Value:  "))
		sb.WriteString(valueStyle.Render(fmt.Sprintf("%q", err.InvalidValue)))
		sb.WriteString("\n")
		sb.WriteString(labelStyle.Render("Error:  "))
		sb.WriteString(valueStyle.Render(err.ValueError.Error()))
	}

	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("Run the command with --help for usage information."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderArgsSubcommandConflictError creates a styled error message when a command
// has both positional arguments and subcommands defined. This is a structural error
// because positional arguments can only be accepted by leaf commands.
func RenderArgsSubcommandConflictError(err *discovery.ArgsSubcommandConflictError) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")). // Red for error
		MarginBottom(1)

	commandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true).
		MarginTop(1)

	sb.WriteString(headerStyle.Render("✗ Invalid command structure!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("Command %s defines positional arguments but also has subcommands.\n",
		commandStyle.Render("'"+err.CommandName+"'")))
	if err.FilePath != "" {
		sb.WriteString(pathStyle.Render(fmt.Sprintf("  in %s\n", err.FilePath)))
	}
	sb.WriteString("\nPositional arguments can only be defined on leaf commands (commands without subcommands).\n\n")

	sb.WriteString(labelStyle.Render("Defined args:"))
	sb.WriteString("\n")
	for _, arg := range err.Args {
		sb.WriteString(valueStyle.Render(fmt.Sprintf("  • %s - %s\n", arg.Name, arg.Description)))
	}

	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("Subcommands:"))
	sb.WriteString("\n")
	for _, subcmd := range err.Subcommands {
		sb.WriteString(valueStyle.Render(fmt.Sprintf("  • %s\n", subcmd)))
	}

	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Remove either the 'args' field or the subcommands to resolve this conflict."))
	sb.WriteString("\n")

	return sb.String()
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

	if len(err.MissingEnvVars) > 0 {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render("Missing or Invalid Environment Variables:"))
		sb.WriteString("\n")
		for _, envVar := range err.MissingEnvVars {
			sb.WriteString(itemStyle.Render(envVar))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Fix the missing dependencies and try again, or update your invkfile to remove unnecessary ones."))
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
	sb.WriteString(hintStyle.Render("Run this command on a supported operating system, or update the 'works_on.hosts' setting in your invkfile."))
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
	sb.WriteString(hintStyle.Render("Use one of the allowed runtimes with --runtime flag, or update the 'runtimes' setting in your invkfile."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderSourceNotFoundError creates a styled error message when a specified source doesn't exist.
func RenderSourceNotFoundError(err *SourceNotFoundError) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		MarginBottom(1)

	sourceStyle := lipgloss.NewStyle().
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

	sb.WriteString(headerStyle.Render("✗ Source not found!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("The source %s does not exist.\n\n", sourceStyle.Render("'"+err.Source+"'")))
	sb.WriteString(labelStyle.Render("Available sources: "))
	if len(err.AvailableSources) > 0 {
		var formatted []string
		for _, s := range err.AvailableSources {
			formatted = append(formatted, formatSourceDisplayName(s))
		}
		sb.WriteString(valueStyle.Render(strings.Join(formatted, ", ")))
	} else {
		sb.WriteString(valueStyle.Render("(none)"))
	}
	sb.WriteString("\n\n")
	sb.WriteString(hintStyle.Render("Use @<source> or --from <source> with a valid source name."))
	sb.WriteString("\n")

	return sb.String()
}

// RenderAmbiguousCommandError creates a styled error message when a command exists in multiple sources.
func RenderAmbiguousCommandError(err *AmbiguousCommandError) string {
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

	sourceStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("242")).
		Italic(true)

	sb.WriteString(headerStyle.Render("✗ Ambiguous command!"))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("The command %s exists in multiple sources:\n\n", commandStyle.Render("'"+err.CommandName+"'")))

	for _, source := range err.Sources {
		// Show source with @prefix for disambiguation (e.g., "@invkfile", "@foo")
		sb.WriteString(fmt.Sprintf("  • %s (%s)\n", sourceStyle.Render("@"+source), formatSourceDisplayName(source)))
	}

	sb.WriteString("\n")
	sb.WriteString(labelStyle.Render("To run this command, specify the source:\n\n"))

	// Show examples with actual source names
	if len(err.Sources) > 0 {
		firstSource := err.Sources[0]
		sb.WriteString(fmt.Sprintf("  invowk cmd %s %s\n", sourceStyle.Render("@"+firstSource), err.CommandName))
		sb.WriteString(fmt.Sprintf("  invowk cmd %s %s %s\n", sourceStyle.Render("--from"), firstSource, err.CommandName))
	}

	sb.WriteString("\n")
	sb.WriteString(hintStyle.Render("Use 'invowk cmd --list' to see all commands with their sources."))
	sb.WriteString("\n")

	return sb.String()
}
