// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/sshserver"
	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// runtimeRegistryResult bundles the runtime registry with its cleanup function,
// non-fatal initialization diagnostics, and any container runtime init error
// for fail-fast dispatch.
type runtimeRegistryResult struct {
	Registry         *runtime.Registry
	Cleanup          func()
	Diagnostics      []discovery.Diagnostic
	ContainerInitErr error
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

// runDisambiguatedCommand executes a command from a specific source.
// It validates that the source exists and that the command is available in that source.
// This is used when @source prefix or --ivk-from flag is provided.
//
// For subcommands (e.g., "deploy staging"), this function attempts to match the longest
// possible command name by progressively joining args. For example, with args ["deploy", "staging"],
// it first tries "deploy staging", then falls back to "deploy" if no match is found.
// Remaining tokens after the match are passed as positional arguments.
func runDisambiguatedCommand(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, filter *SourceFilter, args []string) error {
	ctx := contextWithConfigPath(cmd.Context(), rootFlags.configPath)
	cmd.SetContext(ctx)

	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	commandSetResult, err := app.Discovery.DiscoverCommandSet(ctx)
	if err != nil {
		return err
	}
	app.Diagnostics.Render(ctx, commandSetResult.Diagnostics, app.stderr)

	commandSet := commandSetResult.Set
	// Validate that the requested source exists before attempting command lookup.
	sourceExists := false
	availableSources := make([]string, 0, len(commandSet.SourceOrder))
	for _, sourceID := range commandSet.SourceOrder {
		availableSources = append(availableSources, sourceID)
		if sourceID == filter.SourceID {
			sourceExists = true
		}
	}

	if !sourceExists {
		err := &SourceNotFoundError{Source: filter.SourceID, AvailableSources: availableSources}
		fmt.Fprint(app.stderr, RenderSourceNotFoundError(err))
		return err
	}

	cmdsInSource := commandSet.BySource[filter.SourceID]
	var targetCmd *discovery.CommandInfo
	matchLen := 0
	// Greedy lookup: try longest command prefix first to support nested commands
	// such as `deploy staging` without treating `staging` as an argument.
	for i := len(args); i > 0; i-- {
		candidateName := strings.Join(args[:i], " ")
		for _, discovered := range cmdsInSource {
			if discovered.SimpleName == candidateName {
				targetCmd = discovered
				matchLen = i
				break
			}
		}
		if targetCmd != nil {
			break
		}
	}

	cmdArgs := []string(nil)
	if matchLen < len(args) {
		// Remaining positional tokens are passed as command arguments.
		cmdArgs = args[matchLen:]
	}

	displayCmdName := strings.Join(args, " ")
	if targetCmd == nil {
		fmt.Fprintf(app.stderr, "\n%s Command '%s' not found in source '%s'\n\n", ErrorStyle.Render("✗"), displayCmdName, filter.SourceID)
		if len(cmdsInSource) > 0 {
			fmt.Fprintf(app.stderr, "Available commands in %s:\n", formatSourceDisplayName(filter.SourceID))
			for _, discovered := range cmdsInSource {
				fmt.Fprintf(app.stderr, "  %s\n", discovered.SimpleName)
			}
			fmt.Fprintln(app.stderr)
		}
		return fmt.Errorf("command '%s' not found in source '%s'", displayCmdName, filter.SourceID)
	}

	// Watch mode intercepts before normal execution.
	if cmdFlags.watch {
		return runWatchMode(cmd, app, rootFlags, cmdFlags, append([]string{targetCmd.Name}, cmdArgs...))
	}

	parsedRuntime, err := cmdFlags.parsedRuntimeMode()
	if err != nil {
		return err
	}

	verbose, interactive := resolveUIFlags(ctx, app, cmd, rootFlags)
	// Delegate final execution to CommandService with explicit per-request flags.
	req := ExecuteRequest{
		Name:         targetCmd.Name,
		Args:         cmdArgs,
		Runtime:      parsedRuntime,
		Interactive:  interactive,
		Verbose:      verbose,
		FromSource:   cmdFlags.fromSource,
		ForceRebuild: cmdFlags.forceRebuild,
		DryRun:       cmdFlags.dryRun,
		ConfigPath:   rootFlags.configPath,
	}

	result, diags, err := app.Commands.Execute(ctx, req)
	app.Diagnostics.Render(ctx, diags, app.stderr)
	if err != nil {
		var svcErr *ServiceError
		if errors.As(err, &svcErr) {
			renderServiceError(app.stderr, svcErr)
		}
		return err
	}
	if result.ExitCode != 0 {
		return &ExitError{Code: result.ExitCode}
	}

	return nil
}

// checkAmbiguousCommand checks if a command name (including nested subcommands) is
// ambiguous across sources. It mirrors Cobra's longest-match resolution for nested
// command names and returns an AmbiguousCommandError when the resolved name exists
// in multiple sources, requiring explicit disambiguation via @source or --ivk-from.
func checkAmbiguousCommand(ctx context.Context, app *App, rootFlags *rootFlagValues, args []string) error {
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
	cmdName := ""
	// Mirror Cobra longest-match behavior for nested command names.
	for i := len(args); i > 0; i-- {
		candidateName := strings.Join(args[:i], " ")
		if _, exists := commandSet.BySimpleName[candidateName]; exists {
			cmdName = candidateName
			break
		}
	}

	if cmdName == "" {
		// Unknown command path: let normal Cobra command resolution handle errors.
		cmdName = args[0]
	}

	if !commandSet.AmbiguousNames[cmdName] {
		return nil
	}

	sources := make([]string, 0)
	for _, sourceID := range commandSet.SourceOrder {
		cmdsInSource := commandSet.BySource[sourceID]
		for _, discovered := range cmdsInSource {
			if discovered.SimpleName == cmdName {
				sources = append(sources, sourceID)
				break
			}
		}
	}

	return &AmbiguousCommandError{CommandName: cmdName, Sources: sources}
}

// createRuntimeRegistry creates and populates the runtime registry.
// Native and virtual runtimes are always registered because they execute in-process.
// The container runtime is conditionally registered based on engine availability
// (Docker or Podman). When an SSH server is active for host access, it is forwarded
// to the container runtime so containers can reach back into the host.
//
// INVARIANT: This function creates exactly one ContainerRuntime instance per call.
// The ContainerRuntime.runMu mutex provides intra-process serialization as a fallback
// when flock-based cross-process locking is unavailable (non-Linux platforms).
// Creating multiple ContainerRuntime instances would give each its own mutex,
// defeating the serialization and reintroducing the ping_group_range race.
// See TestCreateRuntimeRegistry_SingleContainerInstance for the enforcement test.
//
// The returned result includes the runtime registry, cleanup function, and
// non-fatal diagnostics produced during runtime initialization.
func createRuntimeRegistry(cfg *config.Config, sshServer *sshserver.Server) runtimeRegistryResult {
	built := runtime.BuildRegistry(runtime.BuildRegistryOptions{
		Config:    cfg,
		SSHServer: sshServer,
	})

	result := runtimeRegistryResult{
		Registry:         built.Registry,
		Cleanup:          built.Cleanup,
		ContainerInitErr: built.ContainerInitErr,
	}

	for _, diag := range built.Diagnostics {
		result.Diagnostics = append(result.Diagnostics, discovery.Diagnostic{
			Severity: discovery.SeverityWarning,
			Code:     discovery.DiagnosticCode(diag.Code),
			Message:  diag.Message,
			Cause:    diag.Cause,
		})
	}

	return result
}

// bridgeTUIRequests bridges TUI component requests from the HTTP-based TUI server
// to the Bubble Tea event loop. It runs as a goroutine that reads from the server's
// request channel until closed, converting each HTTP request into a tea.Msg for
// the interactive model to handle.
func bridgeTUIRequests(server *tuiserver.Server, program *tea.Program) {
	for req := range server.RequestChannel() {
		program.Send(tui.TUIComponentMsg{
			Component:  tui.ComponentType(req.Component),
			Options:    req.Options,
			ResponseCh: req.ResponseCh,
		})
	}
}
