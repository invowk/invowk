// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"invowk-cli/internal/discovery"
	"invowk-cli/internal/issue"
	"invowk-cli/pkg/invkfile"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// registerDiscoveredCommands adds discovered commands as Cobra subcommands under `cmd`.
// Unambiguous commands are registered under their SimpleName for transparent access
// (e.g., `invowk cmd build`), while ambiguous commands — those whose SimpleName appears
// in multiple sources — are intentionally excluded from transparent registration. This
// ensures ambiguous commands fail with a helpful disambiguation message rather than
// silently picking one source. Ambiguous commands must be executed via @source or --from.
func registerDiscoveredCommands(ctx context.Context, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, cmdCmd *cobra.Command) {
	lookupCtx := contextWithConfigPath(ctx, rootFlags.configPath)
	result, err := app.Discovery.DiscoverAndValidateCommandSet(lookupCtx)
	if err != nil {
		// Dynamic registration is best-effort: command execution paths perform
		// validation again and surface actionable errors when needed.
		return
	}

	commandSet := result.Set
	// commandMap tracks synthesized nodes for space-separated command names.
	commandMap := make(map[string]*cobra.Command)
	// Ambiguous prefixes are preserved as parent nodes so they can render
	// disambiguation guidance when invoked directly.
	ambiguousPrefixes := make(map[string]bool)
	for name := range commandSet.AmbiguousNames {
		ambiguousPrefixes[name] = true
	}

	for _, cmdInfo := range commandSet.Commands {
		// Ambiguous leaf names are intentionally excluded from transparent
		// registration and must be executed via source disambiguation.
		if commandSet.AmbiguousNames[cmdInfo.SimpleName] {
			continue
		}

		registrationName := cmdInfo.SimpleName
		parts := strings.Fields(registrationName)
		parent := cmdCmd
		prefix := ""

		for i, part := range parts {
			if prefix != "" {
				prefix += " "
			}
			prefix += part

			if existing, ok := commandMap[prefix]; ok {
				parent = existing
				continue
			}

			isLeaf := i == len(parts)-1
			var newCmd *cobra.Command
			if isLeaf {
				newCmd = buildLeafCommand(app, rootFlags, cmdFlags, cmdInfo, part)
			} else {
				// Parent nodes exist to support nested command trees and ambiguity
				// handling for intermediate prefixes.
				parentPrefix := prefix
				isAmbiguous := ambiguousPrefixes[parentPrefix]
				newCmd = &cobra.Command{
					Use:   part,
					Short: fmt.Sprintf("Commands under '%s'", prefix),
					RunE: func(cmd *cobra.Command, args []string) error {
						fromFlag, _ := cmd.Flags().GetString("from")
						if fromFlag != "" {
							// Preserve full path for longest-match disambiguation.
							fullArgs := append(strings.Fields(parentPrefix), args...)
							filter := &SourceFilter{SourceID: normalizeSourceName(fromFlag), Raw: fromFlag}
							return runDisambiguatedCommand(cmd, app, rootFlags, cmdFlags, filter, fullArgs)
						}

						if isAmbiguous {
							cmdArgs := append(strings.Fields(parentPrefix), args...)
							if err := checkAmbiguousCommand(cmd.Context(), app, rootFlags, cmdArgs); err != nil {
								ambigErr := (*AmbiguousCommandError)(nil)
								if errors.As(err, &ambigErr) {
									fmt.Fprint(app.stderr, RenderAmbiguousCommandError(ambigErr))
									cmd.SilenceErrors = true
									cmd.SilenceUsage = true
								}
								return err
							}
						}

						// Non-ambiguous parents behave as help-only namespaces.
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

// buildLeafCommand creates a Cobra command for an executable leaf node in the command
// tree. It captures immutable discovery values (name, source, flags, args) in closures
// so each command instance is self-contained. Flag definitions from the invkfile are
// projected into Cobra flags with matching types, and at execution time flag values are
// extracted and projected into INVOWK_FLAG_* env vars. When --from doesn't match the
// registered source, the command re-routes through runDisambiguatedCommand.
func buildLeafCommand(app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, cmdInfo *discovery.CommandInfo, cmdPart string) *cobra.Command {
	// Capture immutable values for closures created per discovered command.
	cmdName := cmdInfo.Name
	cmdSimpleName := cmdInfo.SimpleName
	cmdSourceID := cmdInfo.SourceID
	cmdRuntimeFlags := cmdInfo.Command.Flags
	cmdArgs := cmdInfo.Command.Args

	useStr := buildCommandUsageString(cmdPart, cmdArgs)

	newCmd := &cobra.Command{
		Use:   useStr,
		Short: cmdInfo.Description,
		Long:  fmt.Sprintf("Run the '%s' command from %s", cmdInfo.Name, cmdInfo.FilePath),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromFlag, _ := cmd.Flags().GetString("from")
			if fromFlag != "" {
				filter := &SourceFilter{SourceID: normalizeSourceName(fromFlag), Raw: fromFlag}
				// If Cobra routed to the wrong source-specific leaf, delegate to
				// source-aware lookup instead of executing the wrong command.
				if filter.SourceID != cmdSourceID {
					return runDisambiguatedCommand(cmd, app, rootFlags, cmdFlags, filter, append(strings.Fields(cmdSimpleName), args...))
				}
			}

			// Extract typed flag values from Cobra state for service-side validation
			// and INVOWK_FLAG_* environment projection.
			flagValues := make(map[string]string)
			for _, flag := range cmdRuntimeFlags {
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

			envFiles, _ := cmd.Flags().GetStringArray("env-file")
			envVarFlags, _ := cmd.Flags().GetStringArray("env-var")
			envVars := parseEnvVarFlags(envVarFlags)
			workdirOverride, _ := cmd.Flags().GetString("workdir")
			envInheritMode, _ := cmd.Flags().GetString("env-inherit-mode")
			envInheritAllow, _ := cmd.Flags().GetStringArray("env-inherit-allow")
			envInheritDeny, _ := cmd.Flags().GetStringArray("env-inherit-deny")

			verbose, interactive := resolveUIFlags(cmd.Context(), app, cmd, rootFlags)
			req := ExecuteRequest{
				Name:            cmdName,
				Args:            args,
				Runtime:         cmdFlags.runtimeOverride,
				Interactive:     interactive,
				Verbose:         verbose,
				FromSource:      cmdFlags.fromSource,
				ForceRebuild:    cmdFlags.forceRebuild,
				Workdir:         workdirOverride,
				EnvFiles:        envFiles,
				EnvVars:         envVars,
				ConfigPath:      rootFlags.configPath,
				FlagValues:      flagValues,
				FlagDefs:        cmdRuntimeFlags,
				ArgDefs:         cmdArgs,
				EnvInheritMode:  envInheritMode,
				EnvInheritAllow: envInheritAllow,
				EnvInheritDeny:  envInheritDeny,
			}

			err := executeRequest(cmd, app, req)
			if err != nil {
				exitErr := (*ExitError)(nil)
				if errors.As(err, &exitErr) {
					cmd.SilenceErrors = true
					cmd.SilenceUsage = true
				}
			}

			return err
		},
		Args: buildCobraArgsValidator(cmdArgs),
	}

	// Reserved runtime flags are injected for every discovered leaf.
	newCmd.Flags().StringArrayP("env-file", "e", nil, "load environment variables from file(s) (can be specified multiple times)")
	newCmd.Flags().StringArrayP("env-var", "E", nil, "set environment variable (KEY=VALUE, can be specified multiple times)")
	newCmd.Flags().String("env-inherit-mode", "", "inherit host environment variables: none, allow, all (overrides runtime config)")
	newCmd.Flags().StringArray("env-inherit-allow", nil, "allowlist for host environment inheritance (repeatable)")
	newCmd.Flags().StringArray("env-inherit-deny", nil, "denylist for host environment inheritance (repeatable)")
	newCmd.Flags().StringP("workdir", "w", "", "override the working directory for this command")

	if len(cmdArgs) > 0 {
		newCmd.Long += "\n\nArguments:\n" + buildArgsDocumentation(cmdArgs)
	}

	for _, flag := range cmdRuntimeFlags {
		// Project invkfile flag definitions into Cobra flags with matching types.
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
				_, _ = fmt.Sscanf(flag.DefaultValue, "%d", &defaultVal)
			}
			if flag.Short != "" {
				newCmd.Flags().IntP(flag.Name, flag.Short, defaultVal, flag.Description)
			} else {
				newCmd.Flags().Int(flag.Name, defaultVal, flag.Description)
			}
		case invkfile.FlagTypeFloat:
			defaultVal := 0.0
			if flag.DefaultValue != "" {
				_, _ = fmt.Sscanf(flag.DefaultValue, "%f", &defaultVal)
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
		if flag.Required {
			// Required markers are applied at Cobra level for immediate feedback.
			_ = newCmd.MarkFlagRequired(flag.Name)
		}
	}

	return newCmd
}

// buildCommandUsageString builds the Cobra Use string including argument placeholders.
func buildCommandUsageString(cmdPart string, args []invkfile.Argument) string {
	if len(args) == 0 {
		return cmdPart
	}

	parts := []string{cmdPart}
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

// buildArgsDocumentation builds the documentation string for arguments.
func buildArgsDocumentation(args []invkfile.Argument) string {
	lines := make([]string, 0, len(args))
	for _, arg := range args {
		status := "(optional)"
		switch {
		case arg.Required:
			status = "(required)"
		case arg.DefaultValue != "":
			status = fmt.Sprintf("(default: %q)", arg.DefaultValue)
		}

		typeInfo := ""
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

// buildCobraArgsValidator creates a Cobra Args validator function for argument definitions.
func buildCobraArgsValidator(argDefs []invkfile.Argument) cobra.PositionalArgs {
	if len(argDefs) == 0 {
		return cobra.ArbitraryArgs
	}

	return func(cmd *cobra.Command, args []string) error {
		return validateArguments(cmd.Name(), args, argDefs)
	}
}

// completeCommands provides shell completion for the `invowk cmd` command.
// It returns next-token completions using longest-prefix matching against all
// discovered command names. Diagnostics are intentionally suppressed to avoid
// polluting shell completion output with stderr noise.
func completeCommands(app *App, rootFlags *rootFlagValues) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		lookupCtx := contextWithConfigPath(cmd.Context(), rootFlags.configPath)
		result, err := app.Discovery.DiscoverCommandSet(lookupCtx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		commands := result.Set.Commands
		// Completion intentionally stays silent for diagnostics to avoid polluting
		// shell completion output with stderr noise.
		completions := make([]string, 0)
		prefix := strings.Join(args, " ")
		if prefix != "" {
			prefix += " "
		}

		for _, cmdInfo := range commands {
			cmdName := cmdInfo.Name
			if prefix != "" && !strings.HasPrefix(cmdName, prefix) {
				continue
			}

			relativeName := strings.TrimPrefix(cmdName, prefix)
			parts := strings.Fields(relativeName)
			if len(parts) == 0 {
				continue
			}

			// Only return the next token for nested command completion.
			nextPart := parts[0]
			found := false
			for _, completion := range completions {
				if strings.HasPrefix(completion, nextPart+"\t") || completion == nextPart {
					found = true
					break
				}
			}

			if !found && strings.HasPrefix(nextPart, toComplete) {
				desc := cmdInfo.Description
				if len(parts) == 1 && desc != "" {
					completions = append(completions, nextPart+"\t"+desc)
				} else {
					completions = append(completions, nextPart)
				}
			}
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// listCommands displays all available commands grouped by source. Each command
// shows its name, description, available runtimes (with default marked by *),
// and supported platforms. Ambiguous commands are annotated with their source ID.
func listCommands(cmd *cobra.Command, app *App, rootFlags *rootFlagValues) error {
	lookupCtx := contextWithConfigPath(cmd.Context(), rootFlags.configPath)
	result, err := app.Discovery.DiscoverCommandSet(lookupCtx)
	if err != nil {
		rendered, _ := issue.Get(issue.InvkfileNotFoundId).Render("dark")
		fmt.Fprint(app.stderr, rendered)
		return err
	}
	app.Diagnostics.Render(cmd.Context(), result.Diagnostics, app.stderr)

	commandSet := result.Set
	if commandSet == nil || len(commandSet.Commands) == 0 {
		rendered, _ := issue.Get(issue.InvkfileNotFoundId).Render("dark")
		fmt.Fprint(app.stderr, rendered)
		return fmt.Errorf("no commands found")
	}

	verbose, _ := resolveUIFlags(cmd.Context(), app, cmd, rootFlags)

	sourceStyle := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	nameStyle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorVerbose)
	defaultRuntimeStyle := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	platformsStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	legendStyle := lipgloss.NewStyle().Foreground(ColorVerbose).Italic(true)
	ambiguousStyle := lipgloss.NewStyle().Foreground(ColorError)

	if verbose {
		fmt.Println(TitleStyle.Render("Discovery Sources"))
		fmt.Println()
		invkfileCount := 0
		moduleCount := 0
		for _, sourceID := range commandSet.SourceOrder {
			if sourceID == discovery.SourceIDInvkfile {
				invkfileCount++
			} else {
				moduleCount++
			}
			fmt.Printf("  %s %s\n", VerboseHighlightStyle.Render("•"), VerboseStyle.Render(sourceID))
		}
		fmt.Println()
		fmt.Printf("  %s\n", VerboseStyle.Render(fmt.Sprintf("Sources: %d invkfile(s), %d module(s)", invkfileCount, moduleCount)))
		fmt.Printf("  %s\n", VerboseStyle.Render(fmt.Sprintf("Commands: %d total (%d ambiguous)", len(commandSet.Commands), len(commandSet.AmbiguousNames))))
		fmt.Println()
	}

	fmt.Println(TitleStyle.Render("Available Commands"))
	fmt.Println(legendStyle.Render("  (* = default runtime)"))
	fmt.Println()

	for _, sourceID := range commandSet.SourceOrder {
		cmds := commandSet.BySource[sourceID]
		if len(cmds) == 0 {
			continue
		}

		sourceDisplay := formatSourceDisplayName(sourceID)
		fmt.Println(sourceStyle.Render(fmt.Sprintf("From %s:", sourceDisplay)))

		for _, discovered := range cmds {
			line := fmt.Sprintf("  %s", nameStyle.Render(discovered.SimpleName))
			if discovered.Description != "" {
				line += fmt.Sprintf(" - %s", descStyle.Render(discovered.Description))
			}
			if discovered.IsAmbiguous {
				line += fmt.Sprintf(" %s", ambiguousStyle.Render("(@"+sourceID+")"))
			}
			currentPlatform := invkfile.GetCurrentHostOS()
			runtimesStr := discovered.Command.GetRuntimesStringForPlatform(currentPlatform)
			if runtimesStr != "" {
				line += " [" + defaultRuntimeStyle.Render(runtimesStr) + "]"
			}
			platformsStr := discovered.Command.GetPlatformsString()
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
func formatSourceDisplayName(sourceID string) string {
	if sourceID == discovery.SourceIDInvkfile {
		return discovery.SourceIDInvkfile
	}
	if strings.Contains(sourceID, " ") {
		return sourceID
	}

	return sourceID + ".invkmod"
}
