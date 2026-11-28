// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

const longConfigFlagInlinePrefix = "--ivk-config="

type (
	// cmdFlagValues holds the flag bindings for the `invowk cmd` subcommand.
	// These correspond to persistent and local flags registered on the cmdCmd command.
	cmdFlagValues struct {
		// runtimeOverride is the --ivk-runtime flag value (e.g., "container", "virtual-sh").
		runtimeOverride string
		// fromSource is the --ivk-from flag value for source disambiguation.
		fromSource string
		// forceRebuild forces container image rebuilds, bypassing cache.
		forceRebuild bool
		// containerName overrides the persistent container target name.
		containerName string //goplint:ignore -- Cobra flag binding converted to typed request at adapter boundary.
		// dryRun enables dry-run mode: prints what would be executed without executing.
		dryRun bool
		// watch enables watch mode: re-execute command on file changes.
		watch bool
	}

	//goplint:validate-all
	//
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

--ivk-no-cwd (or INVOWK_NO_CWD=1) skips sources 1 and 2.

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
		// Explicit: `invowk cmd` resolves its positionals itself in RunE, so the
		// group guard from installUsageGuards must not reject them first.
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create a single cache-equipped context for the entire RunE invocation.
			// All callees (validateCommandTree, checkAmbiguousCommand, listCommands,
			// runDisambiguatedCommand, executeRequest) share the same discovery cache
			// to avoid repeated filesystem scans and CUE parsing.
			if _, _, err := scopeDiscovery(cmd, app, rootFlags); err != nil {
				return err
			}

			// Validate structural command constraints in runtime flow so
			// dynamic registration failures do not break unrelated commands.
			if err := validateCommandTree(cmd.Context(), app); err != nil {
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

			// Default path delegates request mapping + execution to CommandService.
			return runCommand(cmd, app, rootFlags, cmdFlags, args)
		},
		ValidArgsFunction: completeCommands(app, rootFlags),
	}

	cmdCmd.PersistentFlags().StringVarP(&cmdFlags.runtimeOverride, "ivk-runtime", "r", "", "override the runtime (must be allowed by the command)")
	cmdCmd.PersistentFlags().StringVarP(&cmdFlags.fromSource, "ivk-from", "f", "", "source to run command from (e.g., 'invowkfile' or module name)")
	cmdCmd.PersistentFlags().BoolVar(&cmdFlags.forceRebuild, "ivk-force-rebuild", false, "force rebuild of container images (container runtime only)")
	cmdCmd.PersistentFlags().StringVar(&cmdFlags.containerName, "ivk-container-name", "", "override persistent container target name (container runtime only)")
	cmdCmd.PersistentFlags().BoolVar(&cmdFlags.dryRun, "ivk-dry-run", false, "print what would be executed without executing")
	cmdCmd.PersistentFlags().BoolVarP(&cmdFlags.watch, "ivk-watch", "W", false, "watch files for changes and re-execute")

	// Dynamic command leaves are only needed for `invowk cmd ...` flows.
	// Skipping registration for unrelated invocations (e.g., --version, init)
	// avoids startup-time discovery scans and CUE parsing.
	if shouldRegisterDiscoveredCommands(os.Args[1:]) {
		if configPath := explicitConfigPathFromArgs(os.Args[1:]); configPath != "" {
			rootFlags.configPath = configPath
		}
		// A malformed --ivk-no-cwd value or an invalid INVOWK_NO_CWD skips
		// registration; the cmd RunE then reports the error.
		if noCwd, err := noCwdForRegistration(os.Args[1:], app.Getenv); err == nil {
			registerDiscoveredCommands(commandadapters.ContextWithNoCwd(context.Background(), noCwd), app, rootFlags, cmdFlags, cmdCmd)
		}
	}

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

// parseSourceName normalizes a `@source` or `--ivk-from` value and checks that
// it is a well-formed source ID. A malformed value, such as a path like
// `./m.invowkmod`, is a command-line rejection (UsageError, exit status 2),
// like an unknown source. An empty value returns the empty SourceID (no filter).
//
//goplint:ignore -- raw @source/--ivk-from text from Cobra argv, validated into a SourceID here.
func parseSourceName(raw string) (discovery.SourceID, error) {
	if raw == "" {
		return "", nil
	}
	id := normalizeSourceName(raw)
	if err := id.Validate(); err != nil {
		return "", newUsageError(fmt.Errorf("invalid source %q: %w", raw, err))
	}
	return id, nil
}

// ParseSourceFilter extracts source filter from @prefix in args or --ivk-from flag.
// --ivk-from takes precedence because Cobra parsed it explicitly as a named flag.
// @source is only recognized as the first positional token to avoid ambiguity
// with command arguments that happen to start with @. A malformed source name
// returns a UsageError (see parseSourceName).
func ParseSourceFilter(args []string, fromFlag string) (*SourceFilter, []string, error) {
	// `--ivk-from` takes precedence because Cobra parsed it explicitly;
	// `@source` is only recognized as the first positional token.
	raw, rest := fromFlag, args
	if raw == "" {
		if len(args) == 0 || !strings.HasPrefix(args[0], "@") {
			return nil, args, nil
		}
		raw, rest = args[0], args[1:]
	}

	sourceID, err := parseSourceName(raw)
	if err != nil {
		return nil, args, err
	}
	return &SourceFilter{SourceID: sourceID}, rest, nil
}

// runCommand builds an ExecuteRequest from CLI arguments and delegates to
// the CommandService. This is the default execution path when no source
// disambiguation (@source / --ivk-from) is specified and no ambiguity is detected.
func runCommand(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, args []string) error {
	if len(args) == 0 {
		return newUsageError(errNoCommandSpecified)
	}

	// Watch mode intercepts before normal execution.
	if cmdFlags.watch {
		return runWatchMode(cmd, app, rootFlags, cmdFlags, args)
	}

	req, err := buildExecuteRequest(cmd, rootFlags, cmdFlags, args)
	if err != nil {
		return err
	}

	err = executeRequest(cmd, app, req)
	silenceOnExitError(cmd, err)
	return err
}

// buildExecuteRequest maps an unfiltered invocation; a source filter
// (`@source` or --ivk-from) always goes through runDisambiguatedCommand.
//
//goplint:ignore -- CLI adapter maps raw positional tokens into ExecuteRequest.
func buildExecuteRequest(cmd *cobra.Command, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, args []string) (ExecuteRequest, error) {
	return buildCommandExecuteRequest(cmd, rootFlags, cmdFlags, executeRequestOptions{
		Name: args[0],
		Args: args[1:],
	})
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

// shouldRegisterDiscoveredCommands returns true when CLI argv indicates that
// the invocation targets the `cmd` subtree (or completion for it). This avoids
// eager discovery during startup for unrelated commands.
//
//goplint:ignore -- parses raw process argv tokens at CLI boundary.
func shouldRegisterDiscoveredCommands(args []string) bool {
	primary, rest := firstTopLevelToken(args)
	switch primary {
	case "cmd":
		return true
	case "__complete", "__completeNoDesc":
		next, _ := firstTopLevelToken(rest)
		return next == "cmd"
	default:
		return false
	}
}

// explicitConfigPathFromArgs extracts --ivk-config from raw argv before Cobra
// parses flags, so dynamic command registration uses the same config path as
// eventual command execution.
//
//goplint:ignore -- parses raw process argv tokens at CLI startup boundary.
func explicitConfigPathFromArgs(args []string) string {
	for i := range args {
		arg := args[i]
		if arg == "--" {
			return ""
		}

		switch {
		case arg == "--ivk-config", arg == "-c":
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		case strings.HasPrefix(arg, longConfigFlagInlinePrefix):
			return strings.TrimPrefix(arg, longConfigFlagInlinePrefix)
		case strings.HasPrefix(arg, "-c="):
			return strings.TrimPrefix(arg, "-c=")
		case strings.HasPrefix(arg, "-c") && len(arg) > 2:
			return strings.TrimPrefix(arg, "-c")
		}
	}
	return ""
}

// firstTopLevelToken extracts the first root-level command token from argv,
// skipping root persistent flags and their values.
//
//goplint:ignore -- helper intentionally operates on raw argv token slices.
func firstTopLevelToken(args []string) (token string, remaining []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1], args[i+2:]
			}
			return "", nil
		}

		switch {
		// Root bool flags (with optional "=value" form).
		case arg == "--ivk-verbose", arg == "--ivk-interactive", arg == "-v", arg == "-i",
			strings.HasPrefix(arg, "--ivk-verbose="), strings.HasPrefix(arg, "--ivk-interactive="):
			continue
		// Root config flag with separate value.
		case arg == "--ivk-config", arg == "-c", arg == longColorFlag:
			if i+1 < len(args) {
				i++
			}
			continue
		// Root config/color flag with inline value.
		case strings.HasPrefix(arg, longConfigFlagInlinePrefix), strings.HasPrefix(arg, longColorFlagInline),
			strings.HasPrefix(arg, "-c="), (strings.HasPrefix(arg, "-c") && len(arg) > 2):
			continue
		// Any other flag-like token (keep scanning for the command token).
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			return arg, args[i+1:]
		}
	}

	return "", nil
}

// executeRequest dispatches an ExecuteRequest through the App's CommandService
// and renders any diagnostics to stderr. Non-zero exit codes are wrapped in
// ExitError to signal Cobra to exit without printing usage.
func executeRequest(cmd *cobra.Command, app *App, req ExecuteRequest) error {
	// Ensure every execution path carries the explicit config path and request
	// cache, keeping the no-cwd switch the caller already resolved.
	reqCtx := contextWithDiscoveryScope(cmd.Context(), string(req.ConfigPath), noCwdFromContext(cmd.Context()))
	cmd.SetContext(reqCtx)

	// Cobra adapters always render service diagnostics in the CLI layer.
	result, diags, err := app.Commands.Execute(reqCtx, req)
	app.Diagnostics.Render(reqCtx, diags, app.errUI())
	if err != nil {
		if isSignalCancelledContext(reqCtx, err) {
			return err
		}
		if svcErr, ok := errors.AsType[*ServiceError](err); ok {
			renderServiceError(app.errUI(), svcErr, app.glamourStyle(reqCtx, &rootFlagValues{configPath: string(req.ConfigPath)}))
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

	cfg, err := loadUIConfig(ctx, app, rootFlags)
	if err != nil {
		fmt.Fprintln(app.errUI(), WarningStyle.Render("Warning: ")+formatErrorForDisplay(err, rootFlags.verbose))
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

// loadUIConfig loads the request config for UI flag defaults. A failed explicit
// config source is returned as an error instead of being discarded.
func loadUIConfig(ctx context.Context, app *App, rootFlags *rootFlagValues) (*config.Config, error) {
	if discoveryConfig, ok := app.Discovery.(uiConfigLoader); ok {
		cfg, _, err := discoveryConfig.LoadConfig(ctx)
		return cfg, err
	}
	return app.Config.Load(ctx, config.LoadOptions{ConfigFilePath: types.FilesystemPath(rootFlags.configPath)}) //goplint:ignore -- CLI flag value, may be empty
}

func explicitUIFlags(cmd *cobra.Command, rootFlags *rootFlagValues) (verbose, interactive, verboseSet, interactiveSet bool) {
	return rootFlags.verbose,
		rootFlags.interactive,
		cmd.Root().PersistentFlags().Changed("ivk-verbose"),
		cmd.Root().PersistentFlags().Changed("ivk-interactive")
}

// validateCommandTree discovers all commands and validates the command tree for
// structural conflicts (e.g., commands with both args and subcommands).
// On success, diagnostic rendering is deferred to downstream callers (listCommands,
// executeRequest) that consume the cached discovery result. On error, diagnostics
// are rendered here because downstream callers will not execute.
func validateCommandTree(ctx context.Context, app *App) error {
	result, err := app.Discovery.DiscoverAndValidateCommandSet(ctx)
	if err == nil {
		return nil // diagnostics rendered by downstream callers via the shared cache
	}

	// Error path: downstream callers won't execute, so render diagnostics here.
	app.Diagnostics.Render(ctx, result.Diagnostics, app.errUI())
	if errors.Is(err, config.ErrExplicitConfig) {
		return renderExplicitConfigError(app.errUI(), err)
	}

	if conflictErr, ok := errors.AsType[*discovery.ArgsSubcommandConflictError](err); ok {
		fmt.Fprintf(app.errUI(), "\n%s\n\n", RenderArgsSubcommandConflictError(conflictErr))
	}

	return err
}
