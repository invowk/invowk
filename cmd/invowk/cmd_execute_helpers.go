// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"log/slog"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

// parseEnvVarFlags parses an array of KEY=VALUE strings into a map.
// Malformed entries (missing '=') are logged as warnings and skipped.
func parseEnvVarFlags(envVarFlags []string) map[string]string {
	if len(envVarFlags) == 0 {
		return nil
	}

	result := make(map[string]string, len(envVarFlags))
	for _, kv := range envVarFlags {
		idx := strings.Index(kv, "=")
		if idx > 0 {
			result[kv[:idx]] = kv[idx+1:]
		} else {
			slog.Warn("ignoring malformed --ivk-env-var value (expected KEY=VALUE format)", "value", kv)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

// toEnvVarNames converts a CLI string slice (from Cobra flags) to typed EnvVarName values.
func toEnvVarNames(names []string) []invowkfile.EnvVarName {
	if len(names) == 0 {
		return nil
	}
	result := make([]invowkfile.EnvVarName, len(names))
	for i, name := range names {
		result[i] = invowkfile.EnvVarName(name) //goplint:ignore -- CLI flag boundary conversion
	}
	return result
}

// toDotenvFilePaths converts a CLI string slice (from Cobra flags) to typed DotenvFilePath values.
func toDotenvFilePaths(paths []string) []invowkfile.DotenvFilePath {
	if len(paths) == 0 {
		return nil
	}
	result := make([]invowkfile.DotenvFilePath, len(paths))
	for i, path := range paths {
		result[i] = invowkfile.DotenvFilePath(path) //goplint:ignore -- CLI flag boundary conversion
	}
	return result
}

// runDisambiguatedCommand executes a command from a specific source.
// The command service validates source existence and resolves longest command
// matches for both normal execution and watch mode.
// This is used when @source prefix or --ivk-from flag is provided.
//
// For subcommands (e.g., "deploy staging"), the command service resolves the
// longest possible command name and returns any remaining positional arguments.
func runDisambiguatedCommand(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, filter *SourceFilter, args []string) error {
	ctx := contextWithConfigPath(cmd.Context(), rootFlags.configPath)
	cmd.SetContext(ctx)

	if len(args) == 0 {
		return errNoCommandSpecified
	}

	if !cmdFlags.watch {
		parsedRuntime, err := cmdFlags.parsedRuntimeMode()
		if err != nil {
			return err
		}

		verbose, interactive := resolveUIFlags(ctx, app, cmd, rootFlags)
		req := ExecuteRequest{
			Name:         args[0],
			Args:         args[1:],
			Runtime:      parsedRuntime,
			Interactive:  interactive,
			Verbose:      verbose,
			FromSource:   filter.SourceID,
			ForceRebuild: cmdFlags.forceRebuild,
			DryRun:       cmdFlags.dryRun,
			ConfigPath:   types.FilesystemPath(rootFlags.configPath), //goplint:ignore -- CLI flag value, may be empty
		}

		return executeRequest(cmd, app, req)
	}

	req := ExecuteRequest{
		Name:       args[0],
		Args:       args[1:],
		FromSource: filter.SourceID,
		ConfigPath: types.FilesystemPath(rootFlags.configPath), //goplint:ignore -- CLI flag value, may be empty
	}
	_, resolvedReq, diags, err := app.Commands.ResolveFromSource(ctx, req)
	app.Diagnostics.Render(ctx, diags, app.stderr)
	if err != nil {
		return renderAndWrapServiceError(err, req)
	}

	return runWatchMode(cmd, app, rootFlags, cmdFlags, append([]string{resolvedReq.Name}, resolvedReq.Args...))
}

// checkAmbiguousCommand checks if a command name (including nested subcommands) is
// ambiguous across sources. It mirrors Cobra's longest-match resolution for nested
// command names and returns an AmbiguousCommandError when the resolved name exists
// in multiple sources, requiring explicit disambiguation via @source or --ivk-from.
func checkAmbiguousCommand(ctx context.Context, app *App, args []string) error {
	if len(args) == 0 {
		return nil
	}

	commandSetResult, discoverErr := app.Discovery.DiscoverCommandSet(ctx)
	if discoverErr != nil {
		slog.Debug("skipping ambiguity check due to discovery error", "error", discoverErr)
		return nil
	}

	app.Diagnostics.Render(ctx, commandSetResult.Diagnostics, app.stderr)

	commandSet := commandSetResult.Set
	var cmdName invowkfile.CommandName
	// Mirror Cobra longest-match behavior for nested command names.
	for i := len(args); i > 0; i-- {
		candidateName := invowkfile.CommandName(strings.Join(args[:i], " ")) //goplint:ignore -- CLI args joined for Cobra resolution
		if _, exists := commandSet.BySimpleName[candidateName]; exists {
			cmdName = candidateName
			break
		}
	}

	if cmdName == "" {
		// Unknown command path: let normal Cobra command resolution handle errors.
		cmdName = invowkfile.CommandName(args[0]) //goplint:ignore -- CLI arg, validated by Cobra resolution
	}

	if !commandSet.AmbiguousNames[cmdName] {
		return nil
	}

	var sources []discovery.SourceID
	for _, c := range commandSet.BySimpleName[cmdName] {
		sources = append(sources, c.SourceID)
	}

	return &AmbiguousCommandError{CommandName: cmdName, Sources: sources}
}
