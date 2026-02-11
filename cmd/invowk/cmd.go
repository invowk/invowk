// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/pkg/invkfile"

	"github.com/spf13/cobra"
)

const (
	// ArgErrMissingRequired indicates missing required arguments.
	ArgErrMissingRequired = iota
	// ArgErrTooMany indicates too many arguments were provided.
	ArgErrTooMany
	// ArgErrInvalidValue indicates an argument value failed validation.
	ArgErrInvalidValue
)

type (
	// cmdFlagValues holds the flag bindings for the `invowk cmd` subcommand.
	// These correspond to persistent and local flags registered on the cmdCmd command.
	cmdFlagValues struct {
		// listFlag triggers command listing mode instead of execution.
		listFlag bool
		// runtimeOverride is the --runtime flag value (e.g., "container", "virtual").
		runtimeOverride string
		// fromSource is the --from flag value for source disambiguation.
		fromSource string
		// forceRebuild forces container image rebuilds, bypassing cache.
		forceRebuild bool
	}

	// DependencyError represents unsatisfied dependencies.
	DependencyError struct {
		CommandName         string
		MissingTools        []string
		MissingCommands     []string
		MissingFilepaths    []string
		MissingCapabilities []string
		FailedCustomChecks  []string
		MissingEnvVars      []string
	}

	// ArgErrType represents the type of argument validation error.
	ArgErrType int

	// ArgumentValidationError represents an argument validation failure.
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
	// Parsed from @source prefix in args or --from flag.
	SourceFilter struct {
		// SourceID is the normalized source identifier (e.g., "invkfile", "foo").
		SourceID string
		// Raw is the original user input before normalization (e.g., "@foo.invkmod").
		Raw string
	}

	// SourceNotFoundError is returned when a specified source does not exist.
	SourceNotFoundError struct {
		Source           string
		AvailableSources []string
	}

	// AmbiguousCommandError is returned when a command exists in multiple sources.
	AmbiguousCommandError struct {
		CommandName string
		Sources     []string
	}
)

// newCmdCommand creates the `invowk cmd` command tree.
func newCmdCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	cmdFlags := &cmdFlagValues{}

	cmdCmd := &cobra.Command{
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
			// Validate structural command constraints in runtime flow so
			// dynamic registration failures do not break unrelated commands.
			if err := validateCommandTree(cmd.Context(), app, rootFlags); err != nil {
				return err
			}

			// `invowk cmd` with no args behaves as command listing.
			if cmdFlags.listFlag || len(args) == 0 {
				return listCommands(cmd, app, rootFlags)
			}

			// Parse source disambiguation from `--from` or `@source` prefix.
			filter, remainingArgs, err := ParseSourceFilter(args, cmdFlags.fromSource)
			if err != nil {
				return err
			}

			// Source-filtered execution performs longest-match lookup in the
			// requested source before dispatching to the service layer.
			if filter != nil {
				return runDisambiguatedCommand(cmd, app, rootFlags, cmdFlags, filter, remainingArgs)
			}

			// Without explicit source selection, detect ambiguity up front and
			// show disambiguation guidance.
			if len(args) > 0 {
				if ambigCheckErr := checkAmbiguousCommand(cmd.Context(), app, rootFlags, args); ambigCheckErr != nil {
					if ambigErr, ok := errors.AsType[*AmbiguousCommandError](ambigCheckErr); ok {
						fmt.Fprint(app.stderr, RenderAmbiguousCommandError(ambigErr))
						cmd.SilenceErrors = true
						cmd.SilenceUsage = true
					}
					return ambigCheckErr
				}
			}

			// Default path delegates request mapping + execution to CommandService.
			return runCommand(cmd, app, rootFlags, cmdFlags, args)
		},
		ValidArgsFunction: completeCommands(app, rootFlags),
	}

	cmdCmd.Flags().BoolVarP(&cmdFlags.listFlag, "list", "l", false, "list all available commands")
	cmdCmd.PersistentFlags().StringVarP(&cmdFlags.runtimeOverride, "runtime", "r", "", "override the runtime (must be allowed by the command)")
	cmdCmd.PersistentFlags().StringVar(&cmdFlags.fromSource, "from", "", "source to run command from (e.g., 'invkfile' or module name)")
	cmdCmd.PersistentFlags().BoolVar(&cmdFlags.forceRebuild, "force-rebuild", false, "force rebuild of container images (container runtime only)")

	// Build dynamic command leaves at construction time (instead of package init).
	registerDiscoveredCommands(context.Background(), app, rootFlags, cmdFlags, cmdCmd)

	return cmdCmd
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
func normalizeSourceName(raw string) string {
	name := strings.TrimPrefix(raw, "@")

	if name == "invkfile.cue" || name == discovery.SourceIDInvkfile {
		return discovery.SourceIDInvkfile
	}

	if moduleName, found := strings.CutSuffix(name, ".invkmod"); found {
		return moduleName
	}

	return name
}

// ParseSourceFilter extracts source filter from @prefix in args or --from flag.
// --from takes precedence because Cobra parsed it explicitly as a named flag.
// @source is only recognized as the first positional token to avoid ambiguity
// with command arguments that happen to start with @.
func ParseSourceFilter(args []string, fromFlag string) (*SourceFilter, []string, error) {
	// `--from` takes precedence because Cobra parsed it explicitly.
	if fromFlag != "" {
		return &SourceFilter{SourceID: normalizeSourceName(fromFlag), Raw: fromFlag}, args, nil
	}

	// `@source` is only recognized as the first positional token.
	if len(args) > 0 && strings.HasPrefix(args[0], "@") {
		raw := args[0]
		return &SourceFilter{SourceID: normalizeSourceName(raw), Raw: raw}, args[1:], nil
	}

	return nil, args, nil
}

// runCommand builds an ExecuteRequest from CLI arguments and delegates to
// the CommandService. This is the default execution path when no source
// disambiguation (@source / --from) is specified and no ambiguity is detected.
func runCommand(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	// Resolve UI flags with CLI-over-config precedence before building the request.
	verbose, interactive := resolveUIFlags(cmd.Context(), app, cmd, rootFlags)
	req := ExecuteRequest{
		Name:         args[0],
		Args:         args[1:],
		Runtime:      cmdFlags.runtimeOverride,
		Interactive:  interactive,
		Verbose:      verbose,
		FromSource:   cmdFlags.fromSource,
		ForceRebuild: cmdFlags.forceRebuild,
		ConfigPath:   rootFlags.configPath,
	}

	err := executeRequest(cmd, app, req)
	if err != nil {
		if _, ok := errors.AsType[*ExitError](err); ok { //nolint:errcheck // type match only; error is handled via ok
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
		}
	}

	return err
}

// executeRequest dispatches an ExecuteRequest through the App's CommandService
// and renders any diagnostics to stderr. Non-zero exit codes are wrapped in
// ExitError to signal Cobra to exit without printing usage.
func executeRequest(cmd *cobra.Command, app *App, req ExecuteRequest) error {
	// Cobra adapters always render service diagnostics in the CLI layer.
	result, diags, err := app.Commands.Execute(cmd.Context(), req)
	app.Diagnostics.Render(cmd.Context(), diags, app.stderr)
	if err != nil {
		return err
	}

	if result.ExitCode != 0 {
		return &ExitError{Code: result.ExitCode}
	}

	return nil
}

// resolveUIFlags applies CLI-over-config precedence for verbose and interactive flags.
// Explicitly set CLI flags (--verbose, --interactive) take priority over config values
// (ui.verbose, ui.interactive). Config values serve as defaults when flags are not set.
func resolveUIFlags(ctx context.Context, app *App, cmd *cobra.Command, rootFlags *rootFlagValues) (verbose, interactive bool) {
	verbose = rootFlags.verbose
	interactive = rootFlags.interactive

	cfg, err := app.Config.Load(ctx, config.LoadOptions{ConfigFilePath: rootFlags.configPath})
	if err != nil {
		fmt.Fprintln(app.stderr, WarningStyle.Render("Warning: ")+formatErrorForDisplay(err, rootFlags.verbose))
		return verbose, interactive
	}

	// CLI flags win over config values when explicitly set.
	if !cmd.Root().PersistentFlags().Changed("verbose") {
		verbose = cfg.UI.Verbose
	}

	if !cmd.Root().PersistentFlags().Changed("interactive") {
		interactive = cfg.UI.Interactive
	}

	return verbose, interactive
}

// validateCommandTree discovers all commands and validates the command tree for
// structural conflicts (e.g., commands with both args and subcommands). It renders
// non-fatal diagnostics and returns ArgsSubcommandConflictError if found.
func validateCommandTree(ctx context.Context, app *App, rootFlags *rootFlagValues) error {
	lookupCtx := contextWithConfigPath(ctx, rootFlags.configPath)
	result, err := app.Discovery.DiscoverAndValidateCommandSet(lookupCtx)
	// Always render non-fatal diagnostics produced during discovery.
	app.Diagnostics.Render(ctx, result.Diagnostics, app.stderr)
	if err == nil {
		return nil
	}

	if conflictErr, ok := errors.AsType[*discovery.ArgsSubcommandConflictError](err); ok {
		fmt.Fprintf(app.stderr, "\n%s\n\n", RenderArgsSubcommandConflictError(conflictErr))
	}

	return err
}
