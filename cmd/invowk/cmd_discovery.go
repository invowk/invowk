// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/pkg/invowkfile"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
)

// commandGroup holds commands grouped by category for list rendering.
type commandGroup struct {
	category invowkfile.CommandCategory
	commands []*discovery.CommandInfo
}

// registerDiscoveredCommands adds discovered commands as Cobra subcommands under `cmd`.
// Unambiguous commands are registered under their SimpleName for transparent access
// (e.g., `invowk cmd build`), while ambiguous commands — those whose SimpleName appears
// in multiple sources — are intentionally excluded from transparent registration. This
// ensures ambiguous commands fail with a helpful disambiguation message rather than
// silently picking one source. Ambiguous commands must be executed via @source or --ivk-from.
//
//nolint:contextcheck // Cobra child commands carry cancellation through cmd.Context() at execution time.
func registerDiscoveredCommands(ctx context.Context, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, cmdCmd *cobra.Command) {
	// ctx already carries the pre-parsed no-cwd switch.
	registeredNoCwd := noCwdFromContext(ctx)
	lookupCtx := contextWithDiscoveryScope(ctx, rootFlags.configPath, registeredNoCwd)
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
		ambiguousPrefixes[string(name)] = true
	}

	// Build nodes in SimpleName order so a leaf `X` is always built before any
	// `X Y` from another source and becomes their parent; otherwise a group
	// node `X` created first would shadow the runnable leaf. The sort works on
	// a copy, so registration never reorders the discovery result that listing
	// (BySource, alphabetical within a source) and completion read.
	ordered := slices.Clone(commandSet.Commands)
	slices.SortStableFunc(ordered, func(a, b *discovery.CommandInfo) int {
		return cmp.Compare(a.SimpleName, b.SimpleName)
	})

	for _, cmdInfo := range ordered {
		// Ambiguous leaf names are intentionally excluded from transparent
		// registration and must be executed via source disambiguation.
		if commandSet.AmbiguousNames[cmdInfo.SimpleName] {
			continue
		}

		registrationName := string(cmdInfo.SimpleName)
		parts := strings.Fields(registrationName)
		parent := cmdCmd

		for i, part := range parts {
			prefix := strings.Join(parts[:i+1], " ")

			if existing, ok := commandMap[prefix]; ok {
				parent = existing
				continue
			}

			var newCmd *cobra.Command
			if i == len(parts)-1 {
				newCmd = buildLeafCommand(app, rootFlags, cmdFlags, cmdInfo, part, registeredNoCwd)
			} else {
				// Parent nodes exist to support nested command trees and ambiguity
				// handling for intermediate prefixes.
				newCmd = buildDiscoveredGroupCommand(app, rootFlags, cmdFlags, part, prefix, ambiguousPrefixes[prefix])
			}

			parent.AddCommand(newCmd)
			commandMap[prefix] = newCmd
			parent = newCmd
		}
	}
}

// buildDiscoveredGroupCommand creates the synthesized parent node for a
// space-separated command prefix (for example `test` for `test unit`). Its
// RunE applies, in order:
//  1. `--ivk-from` set: run the source-filtered command for prefix + args;
//  2. no args and the prefix is not itself an ambiguous command: show help;
//  3. the full path (prefix + args) is ambiguous across sources: render the
//     ambiguous-command message and fail with a usage error;
//  4. no args: show help;
//  5. otherwise reject the unmatched token (rejectUnmatchedGroupToken).
//
// Args is explicitly cobra.ArbitraryArgs because RunE does its own checks.
//
//goplint:ignore -- Cobra Use token and space-joined command prefix at the CLI adapter boundary.
func buildDiscoveredGroupCommand(app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, part, prefix string, prefixAmbiguous bool) *cobra.Command {
	return &cobra.Command{
		Use:   part,
		Short: fmt.Sprintf("Commands under '%s'", prefix),
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, _, err := scopeDiscovery(cmd, app, rootFlags)
			if err != nil {
				return err
			}

			fullArgs := append(strings.Fields(prefix), args...)
			fromFlag, _ := cmd.Flags().GetString("ivk-from")
			if fromFlag != "" {
				sourceID, parseErr := parseSourceName(fromFlag)
				if parseErr != nil {
					return parseErr
				}
				// Preserve full path for longest-match disambiguation.
				return runDisambiguatedCommand(cmd, app, rootFlags, cmdFlags, &SourceFilter{SourceID: sourceID}, fullArgs)
			}

			// A bare group is a request to browse. The ambiguity check below is
			// only needed when the prefix itself names an ambiguous command.
			if len(args) == 0 && !prefixAmbiguous {
				return cmd.Help()
			}

			diags, err := checkAmbiguousCommand(ctx, app, fullArgs)
			if err != nil {
				if ambigErr, ok := errors.AsType[*AmbiguousCommandError](err); ok {
					fmt.Fprint(app.stderr, RenderAmbiguousCommandError(ambigErr))
					cmd.SilenceErrors = true
					cmd.SilenceUsage = true
					return newUsageError(err)
				}
				return err
			}

			if len(args) == 0 {
				return cmd.Help()
			}
			return notFoundError(rejectUnmatchedGroupToken(cmd, args), diags, "")
		},
	}
}

// buildLeafCommand creates a Cobra command for an executable leaf node in the command
// tree. It captures immutable discovery values (name, source, flags, args) in closures
// so each command instance is self-contained. Flag definitions from the invowkfile are
// projected into Cobra flags with matching types, and at execution time flag values are
// extracted and projected into INVOWK_FLAG_* env vars. When --ivk-from doesn't match the
// registered source, the command re-routes through runDisambiguatedCommand.
//
// registeredNoCwd is the --ivk-no-cwd value that discovered cmdInfo. The
// pre-parse argv scan and Cobra's flag parse can disagree (a value-taking flag
// can swallow a `--`, so pflag still sees a later --ivk-no-cwd), and a cwd
// command must never run while the switch is on. When the run-time switch
// differs, the name is resolved again under the request's own scope
// (rescopeRegisteredCommand) and the captured cmdInfo is not used.
func buildLeafCommand(app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, cmdInfo *discovery.CommandInfo, cmdPart string, registeredNoCwd bool) *cobra.Command {
	// Capture immutable values for closures created per discovered command.
	cmdName := cmdInfo.Name
	cmdSimpleName := cmdInfo.SimpleName
	cmdSourceID := cmdInfo.SourceID
	cmdRuntimeFlags := cmdInfo.Command.Flags
	cmdArgs := cmdInfo.Command.Args

	useStr := buildCommandUsageString(cmdPart, cmdArgs)

	newCmd := &cobra.Command{
		Use:   useStr,
		Short: string(cmdInfo.Description),
		Long:  fmt.Sprintf("Run the '%s' command from %s", cmdInfo.Name, cmdInfo.FilePath),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, _, err := scopeDiscovery(cmd, app, rootFlags)
			if err != nil {
				return err
			}
			resolved, flagDefs, argDefs := cmdInfo, cmdRuntimeFlags, cmdArgs
			if noCwdFromContext(ctx) != registeredNoCwd {
				resolved = rescopeRegisteredCommand(ctx, app, cmdName)
				flagDefs, argDefs = commandFlagDefs(resolved), commandArgDefs(resolved)
			}

			fromFlag, _ := cmd.Flags().GetString("ivk-from")
			fromSource, err := parseSourceName(fromFlag)
			if err != nil {
				return err
			}
			// If Cobra routed to the wrong source-specific leaf, delegate to
			// source-aware lookup instead of executing the wrong command.
			if fromSource != "" && fromSource != cmdSourceID {
				return runDisambiguatedCommand(cmd, app, rootFlags, cmdFlags, &SourceFilter{SourceID: fromSource}, append(strings.Fields(string(cmdSimpleName)), args...))
			}

			// Watch mode intercepts before normal execution.
			if cmdFlags.watch {
				return runWatchMode(cmd, app, rootFlags, cmdFlags, append([]string{string(cmdName)}, args...))
			}

			req, err := buildCommandExecuteRequest(cmd, rootFlags, cmdFlags, executeRequestOptions{
				Name:            string(cmdName),
				Args:            args,
				FromSource:      fromSource,
				ResolvedCommand: resolved,
				FlagDefs:        flagDefs,
				ArgDefs:         argDefs,
			})
			if err != nil {
				return err
			}

			err = executeRequest(cmd, app, req)
			silenceOnExitError(cmd, err)
			return err
		},
		Args: cobra.ArbitraryArgs,
	}

	// Reserved runtime flags are injected for every discovered leaf.
	newCmd.Flags().StringArrayP("ivk-env-file", "e", nil, "load environment variables from file(s) (can be specified multiple times)")
	newCmd.Flags().StringArrayP("ivk-env-var", "E", nil, "set environment variable (KEY=VALUE, can be specified multiple times)")
	newCmd.Flags().String("ivk-env-inherit-mode", "", "inherit host environment variables: none, allow, all (overrides runtime config)")
	newCmd.Flags().StringArray("ivk-env-inherit-allow", nil, "allowlist for host environment inheritance (repeatable)")
	newCmd.Flags().StringArray("ivk-env-inherit-deny", nil, "denylist for host environment inheritance (repeatable)")
	newCmd.Flags().StringP("ivk-workdir", "w", "", "override the working directory for this command")

	if len(cmdArgs) > 0 {
		newCmd.Long += "\n\nArguments:\n" + buildArgsDocumentation(cmdArgs)
	}

	for _, flag := range cmdRuntimeFlags {
		// Project invowkfile flag definitions into Cobra flags with matching types.
		name := string(flag.Name)
		short := string(flag.Short)
		switch flag.GetType() {
		case invowkfile.FlagTypeBool:
			defaultVal := flag.DefaultValue == "true"
			if short != "" {
				newCmd.Flags().BoolP(name, short, defaultVal, string(flag.Description))
			} else {
				newCmd.Flags().Bool(name, defaultVal, string(flag.Description))
			}
		case invowkfile.FlagTypeInt:
			defaultVal := 0
			if flag.DefaultValue != "" {
				_, _ = fmt.Sscanf(flag.DefaultValue, "%d", &defaultVal)
			}
			if short != "" {
				newCmd.Flags().IntP(name, short, defaultVal, string(flag.Description))
			} else {
				newCmd.Flags().Int(name, defaultVal, string(flag.Description))
			}
		case invowkfile.FlagTypeFloat:
			defaultVal := 0.0
			if flag.DefaultValue != "" {
				_, _ = fmt.Sscanf(flag.DefaultValue, "%f", &defaultVal)
			}
			if short != "" {
				newCmd.Flags().Float64P(name, short, defaultVal, string(flag.Description))
			} else {
				newCmd.Flags().Float64(name, defaultVal, string(flag.Description))
			}
		case invowkfile.FlagTypeString:
			if short != "" {
				newCmd.Flags().StringP(name, short, flag.DefaultValue, string(flag.Description))
			} else {
				newCmd.Flags().String(name, flag.DefaultValue, string(flag.Description))
			}
		}
		if flag.Required {
			// Required markers are applied at Cobra level for immediate feedback.
			_ = newCmd.MarkFlagRequired(name)
		}
	}

	return newCmd
}

// rescopeRegisteredCommand looks cmdName up again under ctx's discovery scope
// and returns the command found there, or nil. A nil result makes the command
// service resolve the name itself and report it as not found, so a command
// registered from a source that ctx excludes never runs. Lookup errors are
// left to that service resolution, which renders them.
func rescopeRegisteredCommand(ctx context.Context, app *App, cmdName invowkfile.CommandName) *discovery.CommandInfo {
	lookup, err := app.Discovery.GetCommand(ctx, string(cmdName))
	if err != nil {
		return nil
	}
	return lookup.Command
}

// buildCommandUsageString builds the Cobra Use string including argument placeholders.
//
//plint:render
func buildCommandUsageString(cmdPart string, args []invowkfile.Argument) string {
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
//
//plint:render
func buildArgsDocumentation(args []invowkfile.Argument) string {
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
		if arg.Type != "" && arg.Type != invowkfile.ArgumentTypeString {
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

// completeCommands provides shell completion for the `invowk cmd` command.
// It returns next-token completions using longest-prefix matching against all
// discovered command names. Diagnostics are intentionally suppressed to avoid
// polluting shell completion output with stderr noise.
func completeCommands(app *App, rootFlags *rootFlagValues) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		on, _, err := resolveNoCwd(cmd.Root().PersistentFlags(), rootFlags, app.Getenv)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		lookupCtx := contextWithDiscoveryScope(cmd.Context(), rootFlags.configPath, on)
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
			cmdName := string(cmdInfo.Name)
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
					completions = append(completions, nextPart+"\t"+string(desc))
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
//
// Diagnostic rendering follows a verbose/non-verbose split:
//   - Verbose: full inline diagnostics before the listing
//   - Non-verbose: suppressed inline, with a single summary footer after the listing
func listCommands(cmd *cobra.Command, app *App, rootFlags *rootFlagValues) error {
	result, err := app.Discovery.DiscoverCommandSet(cmd.Context())
	if err != nil {
		printIssueCatalogEntry(app.stderr, issue.InvowkfileNotFoundId)
		return err
	}

	commandSet := result.Set
	if commandSet == nil || len(commandSet.Commands) == 0 {
		printIssueCatalogEntry(app.stderr, issue.InvowkfileNotFoundId)
		return errors.New("no commands found")
	}

	verbose, _ := resolveUIFlags(cmd.Context(), app, cmd, rootFlags)

	// Verbose mode: render full diagnostics inline before the listing.
	// Non-verbose mode: defer to a summary footer after the listing.
	if verbose && len(result.Diagnostics) > 0 {
		app.Diagnostics.Render(cmd.Context(), result.Diagnostics, app.stderr)
	}

	sourceStyle := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true)
	nameStyle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorVerbose)
	defaultRuntimeStyle := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	platformsStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	legendStyle := lipgloss.NewStyle().Foreground(ColorVerbose).Italic(true)
	ambiguousStyle := lipgloss.NewStyle().Foreground(ColorError)
	categoryStyle := lipgloss.NewStyle().Foreground(ColorHighlight).Italic(true)

	w := app.stdout

	if verbose {
		fmt.Fprintln(w, TitleStyle.Render("Discovery Sources"))
		fmt.Fprintln(w)
		invowkfileCount := 0
		moduleCount := 0
		for _, sourceID := range commandSet.SourceOrder {
			if sourceID == discovery.SourceIDInvowkfile {
				invowkfileCount++
			} else {
				moduleCount++
			}
			fmt.Fprintf(w, "  %s %s\n", VerboseHighlightStyle.Render("•"), VerboseStyle.Render(string(sourceID)))
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  %s\n", VerboseStyle.Render(fmt.Sprintf("Sources: %d invowkfile(s), %d module(s)", invowkfileCount, moduleCount)))
		fmt.Fprintf(w, "  %s\n", VerboseStyle.Render(fmt.Sprintf("Commands: %d total (%d ambiguous)", len(commandSet.Commands), len(commandSet.AmbiguousNames))))
		if origin := activeNoCwdOrigin(cmd, app, rootFlags); origin != "" {
			fmt.Fprintf(w, "  %s\n", VerboseStyle.Render(noCwdDisabledLine(origin)))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, TitleStyle.Render("Available Commands"))
	fmt.Fprintln(w, legendStyle.Render("  (* = default runtime)"))
	fmt.Fprintln(w)

	for _, sourceID := range commandSet.SourceOrder {
		cmds := commandSet.BySource[sourceID]
		if len(cmds) == 0 {
			continue
		}

		sourceDisplay := formatSourceDisplayName(sourceID)
		fmt.Fprintln(w, sourceStyle.Render(fmt.Sprintf("From %s:", sourceDisplay)))

		// Group commands by category within this source.
		groups := groupByCategory(cmds)
		for _, group := range groups {
			if group.category != "" {
				fmt.Fprintln(w, categoryStyle.Render(fmt.Sprintf("  [%s]", group.category)))
			}
			for _, discovered := range group.commands {
				indent := "  "
				if group.category != "" {
					indent = "    "
				}
				line := fmt.Sprintf("%s%s", indent, nameStyle.Render(string(discovered.SimpleName)))
				if discovered.Description != "" {
					line += " - " + descStyle.Render(string(discovered.Description))
				}
				if discovered.IsAmbiguous {
					line += " " + ambiguousStyle.Render("(@"+string(sourceID)+")")
				}
				currentPlatform := invowkfile.CurrentPlatform()
				runtimesStr := discovered.Command.GetRuntimesStringForPlatform(currentPlatform)
				if runtimesStr != "" {
					line += " [" + defaultRuntimeStyle.Render(runtimesStr) + "]"
				}
				platformsStr := discovered.Command.GetPlatformsString()
				if platformsStr != "" {
					line += fmt.Sprintf(" (%s)", platformsStyle.Render(platformsStr))
				}
				fmt.Fprintln(w, line)
			}
		}
		fmt.Fprintln(w)
	}

	// Non-verbose footer: show a single summary line referencing `invowk validate`
	// so users know how to get full diagnostics without cluttering the listing.
	if !verbose && len(result.Diagnostics) > 0 {
		fmt.Fprintf(app.stderr, "%s %d file(s) had issues and were skipped. Run '%s' for details.\n",
			WarningStyle.Render("!"),
			len(result.Diagnostics),
			CmdStyle.Render("invowk validate"),
		)
	}

	return nil
}

// groupByCategory groups commands by their Category field.
// Commands without a category come first, followed by categorized groups
// in alphabetical order.
func groupByCategory(cmds []*discovery.CommandInfo) []commandGroup {
	groups := make(map[invowkfile.CommandCategory][]*discovery.CommandInfo)
	for _, cmd := range cmds {
		groups[cmd.Command.Category] = append(groups[cmd.Command.Category], cmd)
	}

	var result []commandGroup

	// Uncategorized commands first.
	if uncategorized, ok := groups[""]; ok {
		result = append(result, commandGroup{category: "", commands: uncategorized})
		delete(groups, "")
	}

	// Then categorized groups in alphabetical order.
	for _, cat := range slices.Sorted(maps.Keys(groups)) {
		result = append(result, commandGroup{category: cat, commands: groups[cat]})
	}

	return result
}

// formatSourceDisplayName converts a SourceID to a user-friendly display name.
//
//plint:render
func formatSourceDisplayName(sourceID discovery.SourceID) string {
	if sourceID == discovery.SourceIDInvowkfile {
		return string(discovery.SourceIDInvowkfile)
	}
	s := string(sourceID)
	if strings.Contains(s, " ") {
		return s
	}

	return s + ".invowkmod"
}
