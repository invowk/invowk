// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/runtime"
	"invowk-cli/internal/sshserver"
	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// parseEnvVarFlags parses an array of KEY=VALUE strings into a map.
func parseEnvVarFlags(envVarFlags []string) map[string]string {
	if len(envVarFlags) == 0 {
		return nil
	}

	result := make(map[string]string, len(envVarFlags))
	for _, kv := range envVarFlags {
		// Only KEY=VALUE forms are accepted; malformed items are ignored.
		idx := strings.Index(kv, "=")
		if idx > 0 {
			result[kv[:idx]] = kv[idx+1:]
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

// runDisambiguatedCommand executes a command from a specific source.
func runDisambiguatedCommand(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, filter *SourceFilter, args []string) error {
	ctx := cmd.Context()

	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	lookupCtx := contextWithConfigPath(ctx, rootFlags.configPath)
	commandSetResult, err := app.Discovery.DiscoverCommandSet(lookupCtx)
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

	verbose, interactive := resolveUIFlags(ctx, app, cmd, rootFlags)
	// Delegate final execution to CommandService with explicit per-request flags.
	req := ExecuteRequest{
		Name:         targetCmd.Name,
		Args:         cmdArgs,
		Runtime:      cmdFlags.runtimeOverride,
		Interactive:  interactive,
		Verbose:      verbose,
		FromSource:   cmdFlags.fromSource,
		ForceRebuild: cmdFlags.forceRebuild,
		ConfigPath:   rootFlags.configPath,
	}

	result, diags, err := app.Commands.Execute(ctx, req)
	app.Diagnostics.Render(ctx, diags, app.stderr)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return &ExitError{Code: result.ExitCode}
	}

	return nil
}

// checkAmbiguousCommand checks if a command (including subcommands) is ambiguous.
func checkAmbiguousCommand(ctx context.Context, app *App, rootFlags *rootFlagValues, args []string) error {
	if len(args) == 0 {
		return nil
	}

	lookupCtx := contextWithConfigPath(ctx, rootFlags.configPath)
	commandSetResult, discoverErr := app.Discovery.DiscoverCommandSet(lookupCtx)
	if discoverErr == nil {
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

	return nil
}

// createRuntimeRegistry creates and populates the runtime registry.
func createRuntimeRegistry(cfg *config.Config, sshServer *sshserver.Server) *runtime.Registry {
	registry := runtime.NewRegistry()
	// Native and virtual runtimes are always available in-process.
	registry.Register(runtime.RuntimeTypeNative, runtime.NewNativeRuntime())
	registry.Register(runtime.RuntimeTypeVirtual, runtime.NewVirtualRuntime(cfg.VirtualShell.EnableUrootUtils))

	// Container runtime registration is conditional on engine availability.
	containerRT, err := runtime.NewContainerRuntime(cfg)
	if err == nil {
		if sshServer != nil && sshServer.IsRunning() {
			containerRT.SetSSHServer(sshServer)
		}
		registry.Register(runtime.RuntimeTypeContainer, containerRT)
	}

	return registry
}

// bridgeTUIRequests reads TUI component requests from the server's channel.
func bridgeTUIRequests(server *tuiserver.Server, program *tea.Program) {
	for req := range server.RequestChannel() {
		program.Send(tui.TUIComponentMsg{
			Component:  tui.ComponentType(req.Component),
			Options:    req.Options,
			ResponseCh: req.ResponseCh,
		})
	}
}
