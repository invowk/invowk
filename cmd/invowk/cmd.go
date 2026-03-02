// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

// Re-export deps constants for backward compatibility within cmd package.
const (
	// ArgErrMissingRequired indicates missing required arguments.
	ArgErrMissingRequired = deps.ArgErrMissingRequired
	// ArgErrTooMany indicates too many arguments were provided.
	ArgErrTooMany = deps.ArgErrTooMany
	// ArgErrInvalidValue indicates an argument value failed validation.
	ArgErrInvalidValue = deps.ArgErrInvalidValue
)

// Re-export deps sentinel errors and function aliases for backward compatibility
// within cmd package. These allow existing cmd-internal code and tests to call
// the functions without a deps. prefix, preserving pre-extraction call sites.
var (
	// ErrInvalidArgErrType is the sentinel error wrapped by InvalidArgErrTypeError.
	ErrInvalidArgErrType = deps.ErrInvalidArgErrType //nolint:errname // follows DDD pattern: Err+Invalid+TypeName

	// ErrInvalidDependencyMessage is the sentinel error wrapped by InvalidDependencyMessageError.
	ErrInvalidDependencyMessage = deps.ErrInvalidDependencyMessage

	validateRuntimeDependencies      = deps.ValidateRuntimeDependencies
	checkCommandDependenciesExist    = deps.CheckCommandDependenciesExist
	checkHostToolDependencies        = deps.CheckHostToolDependencies
	checkHostFilepathDependencies    = deps.CheckHostFilepathDependencies
	checkCapabilityDependencies      = deps.CheckCapabilityDependencies
	checkHostCustomCheckDependencies = deps.CheckHostCustomCheckDependencies
	checkEnvVarDependencies          = deps.CheckEnvVarDependencies
	validateArguments                = deps.ValidateArguments
	capabilityCheckScript            = deps.CapabilityCheckScript
	shellEscapeSingleQuote           = deps.ShellEscapeSingleQuote
	isExecutable                     = deps.IsExecutable
)

// Re-export deps types for backward compatibility within cmd package.
// These aliases allow existing cmd-internal code and tests to reference
// the types without a deps. prefix.
type (
	// DependencyMessage is a pre-formatted dependency validation message.
	DependencyMessage = deps.DependencyMessage

	// InvalidDependencyMessageError is returned when a DependencyMessage value fails validation.
	InvalidDependencyMessageError = deps.InvalidDependencyMessageError

	// DependencyError represents unsatisfied dependencies.
	DependencyError = deps.DependencyError

	// ArgErrType represents the type of argument validation error.
	ArgErrType = deps.ArgErrType

	// InvalidArgErrTypeError is returned when an ArgErrType value is not valid.
	InvalidArgErrTypeError = deps.InvalidArgErrTypeError

	// ArgumentValidationError represents an argument validation failure.
	ArgumentValidationError = deps.ArgumentValidationError

	// cmdFlagValues holds the flag bindings for the `invowk cmd` subcommand.
	// These correspond to persistent and local flags registered on the cmdCmd command.
	cmdFlagValues struct {
		// runtimeOverride is the --ivk-runtime flag value (e.g., "container", "virtual").
		runtimeOverride string
		// fromSource is the --ivk-from flag value for source disambiguation.
		fromSource string
		// forceRebuild forces container image rebuilds, bypassing cache.
		forceRebuild bool
		// dryRun enables dry-run mode: prints what would be executed without executing.
		dryRun bool
		// watch enables watch mode: re-execute command on file changes.
		watch bool
	}

	// SourceFilter represents a user-specified source constraint for disambiguation.
	// Parsed from @source prefix in args or --ivk-from flag.
	SourceFilter struct {
		// SourceID is the normalized source identifier (e.g., "invowkfile", "foo").
		SourceID discovery.SourceID
	}

	// SourceNotFoundError is returned when a specified source does not exist.
	SourceNotFoundError struct {
		Source           discovery.SourceID
		AvailableSources []discovery.SourceID
	}

	// AmbiguousCommandError is returned when a command exists in multiple sources.
	AmbiguousCommandError struct {
		CommandName invowkfile.CommandName
		Sources     []discovery.SourceID
	}
)

// parsedRuntimeMode parses the --ivk-runtime flag into a typed RuntimeMode.
// Returns zero value ("") for empty input, which serves as the "no override" sentinel.
func (f *cmdFlagValues) parsedRuntimeMode() (invowkfile.RuntimeMode, error) {
	return invowkfile.ParseRuntimeMode(f.runtimeOverride)
}

// newCmdCommand creates the `invowk cmd` command tree.
func newCmdCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	cmdFlags := &cmdFlagValues{}

	cmdCmd := &cobra.Command{
		Use:   "cmd [command-name]",
		Short: "Execute commands from invowkfiles",
		Long: `Execute commands defined in invowkfiles and sibling modules.

Commands are discovered from:
  1. Current directory's invowkfile.cue (highest priority)
  2. Sibling *.invowkmod module directories
  3. Configured includes (module paths from config)
  4. ~/.invowk/cmds/ (modules only, non-recursive)

Commands use their simple names when unique across sources. When a command
name exists in multiple sources, disambiguation is required.

Usage:
  invowk cmd                                        List all available commands
  invowk cmd <command-name>                         Execute a command (if unambiguous)
  invowk cmd @<source> <command-name>               Disambiguate with @source prefix
  invowk cmd --ivk-from <source> <command-name>    Disambiguate with --ivk-from flag

Examples:
  invowk cmd build                        Run unique 'build' command
  invowk cmd @invowkfile deploy             Run 'deploy' from invowkfile
  invowk cmd @foo deploy                  Run 'deploy' from foo.invowkmod
  invowk cmd --ivk-from invowkfile deploy  Same using --ivk-from flag`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a single cache-equipped context for the entire RunE invocation.
			// All callees (validateCommandTree, checkAmbiguousCommand, listCommands,
			// runDisambiguatedCommand, executeRequest) share the same discovery cache
			// to avoid repeated filesystem scans and CUE parsing.
			ctx := contextWithConfigPath(cmd.Context(), rootFlags.configPath)
			cmd.SetContext(ctx)

			// Validate structural command constraints in runtime flow so
			// dynamic registration failures do not break unrelated commands.
			if err := validateCommandTree(cmd.Context(), app, rootFlags); err != nil {
				return err
			}

			// `invowk cmd` with no args behaves as command listing.
			if len(args) == 0 {
				return listCommands(cmd, app, rootFlags)
			}

			// Parse source disambiguation from `--ivk-from` or `@source` prefix.
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

	cmdCmd.PersistentFlags().StringVarP(&cmdFlags.runtimeOverride, "ivk-runtime", "r", "", "override the runtime (must be allowed by the command)")
	cmdCmd.PersistentFlags().StringVarP(&cmdFlags.fromSource, "ivk-from", "f", "", "source to run command from (e.g., 'invowkfile' or module name)")
	cmdCmd.PersistentFlags().BoolVar(&cmdFlags.forceRebuild, "ivk-force-rebuild", false, "force rebuild of container images (container runtime only)")
	cmdCmd.PersistentFlags().BoolVar(&cmdFlags.dryRun, "ivk-dry-run", false, "print what would be executed without executing")
	cmdCmd.PersistentFlags().BoolVarP(&cmdFlags.watch, "ivk-watch", "W", false, "watch files for changes and re-execute")

	// Build dynamic command leaves at construction time (instead of package init).
	registerDiscoveredCommands(context.Background(), app, rootFlags, cmdFlags, cmdCmd)

	return cmdCmd
}

func (e *SourceNotFoundError) Error() string {
	return fmt.Sprintf("source '%s' not found", e.Source)
}

func (e *AmbiguousCommandError) Error() string {
	return fmt.Sprintf("command '%s' is ambiguous", e.CommandName)
}

// normalizeSourceName converts various source name formats to a canonical SourceID.
func normalizeSourceName(raw string) discovery.SourceID {
	name := strings.TrimPrefix(raw, "@")

	if name == "invowkfile.cue" || discovery.SourceID(name) == discovery.SourceIDInvowkfile {
		return discovery.SourceIDInvowkfile
	}

	if moduleName, found := strings.CutSuffix(name, ".invowkmod"); found {
		return discovery.SourceID(moduleName)
	}

	return discovery.SourceID(name)
}

// ParseSourceFilter extracts source filter from @prefix in args or --ivk-from flag.
// --ivk-from takes precedence because Cobra parsed it explicitly as a named flag.
// @source is only recognized as the first positional token to avoid ambiguity
// with command arguments that happen to start with @.
func ParseSourceFilter(args []string, fromFlag string) (*SourceFilter, []string, error) {
	// `--ivk-from` takes precedence because Cobra parsed it explicitly.
	if fromFlag != "" {
		return &SourceFilter{SourceID: normalizeSourceName(fromFlag)}, args, nil
	}

	// `@source` is only recognized as the first positional token.
	if len(args) > 0 && strings.HasPrefix(args[0], "@") {
		return &SourceFilter{SourceID: normalizeSourceName(args[0])}, args[1:], nil
	}

	return nil, args, nil
}

// runCommand builds an ExecuteRequest from CLI arguments and delegates to
// the CommandService. This is the default execution path when no source
// disambiguation (@source / --ivk-from) is specified and no ambiguity is detected.
func runCommand(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, args []string) error {
	if len(args) == 0 {
		return errors.New("no command specified")
	}

	// Watch mode intercepts before normal execution.
	if cmdFlags.watch {
		return runWatchMode(cmd, app, rootFlags, cmdFlags, args)
	}

	parsedRuntime, err := cmdFlags.parsedRuntimeMode()
	if err != nil {
		return err
	}

	// Resolve UI flags with CLI-over-config precedence before building the request.
	verbose, interactive := resolveUIFlags(cmd.Context(), app, cmd, rootFlags)
	req := ExecuteRequest{
		Name:         args[0],
		Args:         args[1:],
		Runtime:      parsedRuntime,
		Interactive:  interactive,
		Verbose:      verbose,
		FromSource:   discovery.SourceID(cmdFlags.fromSource), //goplint:ignore -- CLI flag value, validated downstream
		ForceRebuild: cmdFlags.forceRebuild,
		ConfigPath:   types.FilesystemPath(rootFlags.configPath), //goplint:ignore -- CLI flag value, may be empty
		DryRun:       cmdFlags.dryRun,
	}

	err = executeRequest(cmd, app, req)
	silenceOnExitError(cmd, err)
	return err
}

// silenceOnExitError suppresses Cobra's error/usage printing when the error is
// an ExitError (non-zero exit code). This prevents double-printing of the error.
func silenceOnExitError(cmd *cobra.Command, err error) {
	if err != nil {
		if _, ok := errors.AsType[*ExitError](err); ok { //nolint:errcheck // type match only; error is handled via ok
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
		}
	}
}

// executeRequest dispatches an ExecuteRequest through the App's CommandService
// and renders any diagnostics to stderr. Non-zero exit codes are wrapped in
// ExitError to signal Cobra to exit without printing usage.
func executeRequest(cmd *cobra.Command, app *App, req ExecuteRequest) error {
	// Ensure every execution path carries the explicit config path and request cache.
	reqCtx := contextWithConfigPath(cmd.Context(), string(req.ConfigPath))
	cmd.SetContext(reqCtx)

	// Cobra adapters always render service diagnostics in the CLI layer.
	result, diags, err := app.Commands.Execute(reqCtx, req)
	app.Diagnostics.Render(reqCtx, diags, app.stderr)
	if err != nil {
		if svcErr, ok := errors.AsType[*ServiceError](err); ok {
			renderServiceError(app.stderr, svcErr)
		}
		return err
	}

	if result.ExitCode != 0 {
		return &ExitError{Code: result.ExitCode}
	}

	return nil
}

// resolveUIFlags applies CLI-over-config precedence for verbose and interactive flags.
// Explicitly set CLI flags (--ivk-verbose, --ivk-interactive) take priority over config values
// (ui.verbose, ui.interactive). Config values serve as defaults when flags are not set.
func resolveUIFlags(ctx context.Context, app *App, cmd *cobra.Command, rootFlags *rootFlagValues) (verbose, interactive bool) {
	verbose = rootFlags.verbose
	interactive = rootFlags.interactive

	cfg, err := app.Config.Load(ctx, config.LoadOptions{ConfigFilePath: types.FilesystemPath(rootFlags.configPath)}) //goplint:ignore -- CLI flag value, may be empty
	if err != nil {
		fmt.Fprintln(app.stderr, WarningStyle.Render("Warning: ")+formatErrorForDisplay(err, rootFlags.verbose))
		return verbose, interactive
	}

	// CLI flags win over config values when explicitly set.
	if !cmd.Root().PersistentFlags().Changed("ivk-verbose") {
		verbose = cfg.UI.Verbose
	}

	if !cmd.Root().PersistentFlags().Changed("ivk-interactive") {
		interactive = cfg.UI.Interactive
	}

	return verbose, interactive
}

// validateCommandTree discovers all commands and validates the command tree for
// structural conflicts (e.g., commands with both args and subcommands).
// On success, diagnostic rendering is deferred to downstream callers (listCommands,
// executeRequest) that consume the cached discovery result. On error, diagnostics
// are rendered here because downstream callers will not execute.
func validateCommandTree(ctx context.Context, app *App, rootFlags *rootFlagValues) error {
	result, err := app.Discovery.DiscoverAndValidateCommandSet(ctx)
	if err == nil {
		return nil // diagnostics rendered by downstream callers via the shared cache
	}

	// Error path: downstream callers won't execute, so render diagnostics here.
	app.Diagnostics.Render(ctx, result.Diagnostics, app.stderr)

	if conflictErr, ok := errors.AsType[*discovery.ArgsSubcommandConflictError](err); ok {
		fmt.Fprintf(app.stderr, "\n%s\n\n", RenderArgsSubcommandConflictError(conflictErr))
	}

	return err
}
