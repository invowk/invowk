// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/issue"
	"invowk-cli/pkg/invkfile"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

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

			err := runCommandWithFlags(runCommandOptions{
				CommandName:             cmdName,
				Args:                    args,
				FlagValues:              flagValues,
				FlagDefs:                cmdFlags,
				ArgDefs:                 cmdArgs,
				RuntimeEnvFiles:         envFiles,
				RuntimeEnvVars:          envVars,
				WorkdirOverride:         workdirOverride,
				EnvInheritModeOverride:  envInheritMode,
				EnvInheritAllowOverride: envInheritAllow,
				EnvInheritDenyOverride:  envInheritDeny,
			})
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
			fmt.Fprintf(os.Stderr, "%s Failed to parse %s: %v\n", ErrorStyle.Render("✗"), file.Path, file.Error)
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

	// Style for output - derived from shared color constants
	sourceStyle := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	nameStyle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorVerbose)
	defaultRuntimeStyle := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	platformsStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	legendStyle := lipgloss.NewStyle().Foreground(ColorVerbose).Italic(true)
	ambiguousStyle := lipgloss.NewStyle().Foreground(ColorError)

	// Verbose mode: show discovery source details (FR-013)
	if GetVerbose() {
		fmt.Println(TitleStyle.Render("Discovery Sources"))
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
				VerboseHighlightStyle.Render("•"),
				VerboseStyle.Render(fmt.Sprintf("%s [%s]", file.Path, sourceType)))
		}
		fmt.Println()

		// Show summary
		fmt.Printf("  %s\n",
			VerboseStyle.Render(fmt.Sprintf("Sources: %d invkfile(s), %d module(s)", invkfileCount, moduleCount)))
		fmt.Printf("  %s\n",
			VerboseStyle.Render(fmt.Sprintf("Commands: %d total (%d ambiguous)",
				len(commandSet.Commands), len(commandSet.AmbiguousNames))))
		fmt.Println()
	}

	fmt.Println(TitleStyle.Render("Available Commands"))
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
