// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

type executeRequestOptions struct {
	Name            string   //goplint:ignore -- raw command token from Cobra/discovery boundary
	Args            []string //goplint:ignore -- raw positional tokens from Cobra/discovery boundary
	FromSource      discovery.SourceID
	ResolvedCommand *discovery.CommandInfo
	FlagDefs        []invowkfile.Flag
	ArgDefs         []invowkfile.Argument
}

func (opts executeRequestOptions) Validate() error {
	var errs []error
	if opts.FromSource != "" {
		if err := opts.FromSource.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if opts.ResolvedCommand != nil {
		if err := opts.ResolvedCommand.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, flag := range opts.FlagDefs {
		if err := flag.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, arg := range opts.ArgDefs {
		if err := arg.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidExecuteRequestError{FieldErrors: errs}
	}
	return nil
}

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

// buildCommandExecuteRequest maps Cobra state and resolved command metadata to
// the command service request used by every CLI execution route.
func buildCommandExecuteRequest(cmd *cobra.Command, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, opts executeRequestOptions) (ExecuteRequest, error) {
	if err := opts.Validate(); err != nil {
		return ExecuteRequest{}, err
	}

	parsedRuntime, err := cmdFlags.parsedRuntimeMode()
	if err != nil {
		return ExecuteRequest{}, err
	}

	envInheritMode, err := invowkfile.ParseEnvInheritMode(stringFlagValue(cmd, "ivk-env-inherit-mode"))
	if err != nil {
		return ExecuteRequest{}, err
	}

	verbose, interactive, verboseSet, interactiveSet := explicitUIFlags(cmd, rootFlags)
	envVarFlags := stringArrayFlagValue(cmd, "ivk-env-var")
	return ExecuteRequest{
		Name:            opts.Name,
		Args:            opts.Args,
		Runtime:         parsedRuntime,
		Interactive:     interactive,
		InteractiveSet:  interactiveSet,
		Verbose:         verbose,
		VerboseSet:      verboseSet,
		FromSource:      opts.FromSource,
		ForceRebuild:    cmdFlags.forceRebuild,
		ContainerName:   invowkfile.ContainerName(cmdFlags.containerName),        //goplint:ignore -- CLI flag boundary conversion
		Workdir:         invowkfile.WorkDir(stringFlagValue(cmd, "ivk-workdir")), //goplint:ignore -- CLI flag value, may be empty
		EnvFiles:        toDotenvFilePaths(stringArrayFlagValue(cmd, "ivk-env-file")),
		EnvVars:         parseEnvVarFlags(envVarFlags),
		ConfigPath:      types.FilesystemPath(rootFlags.configPath), //goplint:ignore -- CLI flag value, may be empty
		FlagValues:      flagValuesFromCobra(cmd, opts.FlagDefs),
		FlagDefs:        opts.FlagDefs,
		ArgDefs:         opts.ArgDefs,
		EnvInheritMode:  envInheritMode,
		EnvInheritAllow: toEnvVarNames(stringArrayFlagValue(cmd, "ivk-env-inherit-allow")),
		EnvInheritDeny:  toEnvVarNames(stringArrayFlagValue(cmd, "ivk-env-inherit-deny")),
		DryRun:          cmdFlags.dryRun,
		ResolvedCommand: opts.ResolvedCommand,
	}, nil
}

//goplint:ignore -- command flag values are free-form Cobra strings keyed by typed flag names.
func flagValuesFromCobra(cmd *cobra.Command, defs []invowkfile.Flag) map[invowkfile.FlagName]string {
	if len(defs) == 0 {
		return nil
	}
	flagValues := make(map[invowkfile.FlagName]string, len(defs))
	for _, flag := range defs {
		val, ok := flagValueFromCobra(cmd, flag)
		if ok {
			flagValues[flag.Name] = val
		}
	}
	if len(flagValues) == 0 {
		return nil
	}
	return flagValues
}

//goplint:ignore -- Cobra returns user flag values as primitive strings at the CLI adapter boundary.
func flagValueFromCobra(cmd *cobra.Command, flag invowkfile.Flag) (string, bool) {
	name := string(flag.Name)
	switch flag.GetType() {
	case invowkfile.FlagTypeBool:
		boolVal, err := cmd.Flags().GetBool(name)
		if err != nil {
			return "", false
		}
		return strconv.FormatBool(boolVal), true
	case invowkfile.FlagTypeInt:
		intVal, err := cmd.Flags().GetInt(name)
		if err != nil {
			return "", false
		}
		return strconv.Itoa(intVal), true
	case invowkfile.FlagTypeFloat:
		floatVal, err := cmd.Flags().GetFloat64(name)
		if err != nil {
			return "", false
		}
		return fmt.Sprintf("%g", floatVal), true
	case invowkfile.FlagTypeString:
		stringVal, err := cmd.Flags().GetString(name)
		if err != nil {
			return "", false
		}
		return stringVal, true
	default:
		return "", false
	}
}

//goplint:ignore -- Cobra flag lookup uses string flag identifiers and returns raw flag text.
func stringFlagValue(cmd *cobra.Command, name string) string {
	value, err := cmd.Flags().GetString(name)
	if err != nil {
		return ""
	}
	return value
}

//goplint:ignore -- Cobra flag lookup uses string flag identifiers and returns raw flag text.
func stringArrayFlagValue(cmd *cobra.Command, name string) []string {
	value, err := cmd.Flags().GetStringArray(name)
	if err != nil {
		return nil
	}
	return value
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
		resolveReq, err := buildCommandExecuteRequest(cmd, rootFlags, cmdFlags, executeRequestOptions{
			Name:       args[0],
			Args:       args[1:],
			FromSource: filter.SourceID,
		})
		if err != nil {
			return err
		}

		cmdInfo, resolvedReq, diags, err := app.Commands.ResolveFromSource(ctx, resolveReq)
		app.Diagnostics.Render(ctx, diags, app.stderr)
		if err != nil {
			return renderServiceErrorIfPresent(app, renderAndWrapServiceError(err, resolveReq))
		}

		req, err := buildCommandExecuteRequest(cmd, rootFlags, cmdFlags, executeRequestOptions{
			Name:            resolvedReq.Name,
			Args:            resolvedReq.Args,
			FromSource:      filter.SourceID,
			ResolvedCommand: cmdInfo,
			FlagDefs:        commandFlagDefs(cmdInfo),
			ArgDefs:         commandArgDefs(cmdInfo),
		})
		if err != nil {
			return err
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
		return renderServiceErrorIfPresent(app, renderAndWrapServiceError(err, req))
	}

	childFlags := *cmdFlags
	childFlags.fromSource = string(filter.SourceID)
	return runWatchMode(cmd, app, rootFlags, &childFlags, append([]string{resolvedReq.Name}, resolvedReq.Args...))
}

func renderServiceErrorIfPresent(app *App, err error) error {
	if svcErr, ok := errors.AsType[*ServiceError](err); ok {
		renderServiceError(app.stderr, svcErr)
	}
	return err
}

func commandFlagDefs(cmdInfo *discovery.CommandInfo) []invowkfile.Flag {
	if cmdInfo == nil || cmdInfo.Command == nil {
		return nil
	}
	return cmdInfo.Command.Flags
}

func commandArgDefs(cmdInfo *discovery.CommandInfo) []invowkfile.Argument {
	if cmdInfo == nil || cmdInfo.Command == nil {
		return nil
	}
	return cmdInfo.Command.Args
}

// checkAmbiguousCommand asks the command service to resolve the full candidate
// path and maps command-service ambiguity into the CLI rendering type.
func checkAmbiguousCommand(ctx context.Context, app *App, args []string) error {
	if len(args) == 0 || app.Commands == nil {
		return nil
	}

	req := ExecuteRequest{
		Name:       strings.Join(args, " "),
		ConfigPath: types.FilesystemPath(configPathFromContext(ctx)), //goplint:ignore -- config path already normalized at CLI boundary
	}
	_, _, diags, err := app.Commands.ResolveCommand(ctx, req)
	app.Diagnostics.Render(ctx, diags, app.stderr)
	if err == nil {
		return nil
	}
	if classified, ok := errors.AsType[*commandsvc.ClassifiedError](err); ok {
		if ambigErr, ambigOK := errors.AsType[*commandsvc.AmbiguousCommandError](classified.Err); ambigOK {
			return &AmbiguousCommandError{CommandName: ambigErr.CommandName, Sources: ambigErr.Sources}
		}
		return nil
	}
	if ambigErr, ok := errors.AsType[*commandsvc.AmbiguousCommandError](err); ok {
		return &AmbiguousCommandError{CommandName: ambigErr.CommandName, Sources: ambigErr.Sources}
	}
	return nil
}
