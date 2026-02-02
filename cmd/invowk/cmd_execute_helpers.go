// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/runtime"
	"invowk-cli/internal/sshserver"
	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"

	tea "github.com/charmbracelet/bubbletea"
)

// parseEnvVarFlags parses an array of KEY=VALUE strings into a map.
// Invalid entries (without '=') are silently ignored.
func parseEnvVarFlags(envVarFlags []string) map[string]string {
	if len(envVarFlags) == 0 {
		return nil
	}
	result := make(map[string]string, len(envVarFlags))
	for _, kv := range envVarFlags {
		idx := strings.Index(kv, "=")
		if idx > 0 {
			key := kv[:idx]
			value := kv[idx+1:]
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// runDisambiguatedCommand executes a command from a specific source.
// It validates that the source exists and that the command is available in that source.
// This is used when @source prefix or --from flag is provided.
//
// For subcommands (e.g., "deploy staging"), this function attempts to match the longest
// possible command name by progressively joining args. For example, with args ["deploy", "staging"],
// it first tries "deploy staging", then falls back to "deploy" if no match is found.
func runDisambiguatedCommand(filter *SourceFilter, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	// Get the command set for source validation and command lookup
	commandSet, err := disc.DiscoverCommandSet()
	if err != nil {
		return err
	}

	// Validate that the source exists
	sourceExists := false
	var availableSources []string
	for _, sourceID := range commandSet.SourceOrder {
		availableSources = append(availableSources, sourceID)
		if sourceID == filter.SourceID {
			sourceExists = true
		}
	}

	if !sourceExists {
		// Render helpful error with available sources
		err := &SourceNotFoundError{
			Source:           filter.SourceID,
			AvailableSources: availableSources,
		}
		fmt.Fprint(os.Stderr, RenderSourceNotFoundError(err))
		return err
	}

	// Find the command in the specified source.
	// For subcommands, try to match the longest possible command name.
	// e.g., for args ["deploy", "staging", "arg1"], try:
	//   1. "deploy staging" (if it exists as a command)
	//   2. "deploy" (fall back if "deploy staging" doesn't exist)
	cmdsInSource := commandSet.BySource[filter.SourceID]
	var targetCmd *discovery.CommandInfo
	var matchLen int

	// Try matching progressively longer command names (greedy match)
	for i := len(args); i > 0; i-- {
		candidateName := strings.Join(args[:i], " ")
		for _, cmd := range cmdsInSource {
			if cmd.SimpleName == candidateName {
				targetCmd = cmd
				matchLen = i
				break
			}
		}
		if targetCmd != nil {
			break
		}
	}

	// Determine remaining args after the matched command name
	var cmdArgs []string
	if matchLen < len(args) {
		cmdArgs = args[matchLen:]
	}

	// For error reporting, use the first arg as the "requested" command name
	displayCmdName := args[0]
	if len(args) > 1 {
		displayCmdName = strings.Join(args, " ")
	}

	if targetCmd == nil {
		// Command not found in specified source
		fmt.Fprintf(os.Stderr, "\n%s Command '%s' not found in source '%s'\n\n",
			ErrorStyle.Render("✗"), displayCmdName, filter.SourceID)

		// Show what commands ARE available in that source
		if len(cmdsInSource) > 0 {
			fmt.Fprintf(os.Stderr, "Available commands in %s:\n", formatSourceDisplayName(filter.SourceID))
			for _, cmd := range cmdsInSource {
				fmt.Fprintf(os.Stderr, "  %s\n", cmd.SimpleName)
			}
			fmt.Fprintln(os.Stderr)
		}
		return fmt.Errorf("command '%s' not found in source '%s'", displayCmdName, filter.SourceID)
	}

	// Execute the command using its full Name (which includes any module prefix)
	return runCommandWithFlags(targetCmd.Name, cmdArgs, nil, nil, nil, nil, nil, "", "", nil, nil)
}

// checkAmbiguousCommand checks if a command (including subcommands) is ambiguous.
// It takes the full args slice and tries to find the longest matching command name,
// then checks if that command exists in multiple sources.
//
// For example, with args ["deploy", "staging"], it checks:
//  1. Is "deploy staging" ambiguous?
//  2. If not a known command, is "deploy" ambiguous?
//
// This function is called before Cobra's normal command matching when no explicit source is specified.
func checkAmbiguousCommand(args []string) error {
	if len(args) == 0 {
		return nil
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	commandSet, err := disc.DiscoverCommandSet()
	if err != nil {
		// If we can't discover commands, let the normal flow handle it.
		// Intentionally returning nil here to allow Cobra to attempt command matching,
		// which will produce an appropriate error if the command doesn't exist.
		return nil //nolint:nilerr // Intentional: fall through to normal flow on discovery errors
	}

	// Try to find the longest matching command name.
	// This handles subcommands like "deploy staging" which would be passed as ["deploy", "staging"].
	var cmdName string
	for i := len(args); i > 0; i-- {
		candidateName := strings.Join(args[:i], " ")
		// Check if this candidate is a known command name
		if _, exists := commandSet.BySimpleName[candidateName]; exists {
			cmdName = candidateName
			break
		}
	}

	// If no matching command found, fall back to just the first arg
	// (let Cobra handle "unknown command" errors)
	if cmdName == "" {
		cmdName = args[0]
	}

	// Check if this command name is ambiguous
	if !commandSet.AmbiguousNames[cmdName] {
		return nil
	}

	// Collect the sources where this command exists
	var sources []string
	for _, sourceID := range commandSet.SourceOrder {
		cmdsInSource := commandSet.BySource[sourceID]
		for _, cmd := range cmdsInSource {
			if cmd.SimpleName == cmdName {
				sources = append(sources, sourceID)
				break
			}
		}
	}

	return &AmbiguousCommandError{
		CommandName: cmdName,
		Sources:     sources,
	}
}

// createRuntimeRegistry creates and populates the runtime registry
func createRuntimeRegistry(cfg *config.Config) *runtime.Registry {
	registry := runtime.NewRegistry()

	// Register native runtime
	registry.Register(runtime.RuntimeTypeNative, runtime.NewNativeRuntime())

	// Register virtual runtime
	registry.Register(runtime.RuntimeTypeVirtual, runtime.NewVirtualRuntime(cfg.VirtualShell.EnableUrootUtils))

	// Register container runtime (may fail if no engine available)
	containerRT, err := runtime.NewContainerRuntime(cfg)
	if err == nil {
		// Set the SSH server if it's running
		sshServerMu.Lock()
		if sshServerInstance != nil && sshServerInstance.IsRunning() {
			containerRT.SetSSHServer(sshServerInstance)
		}
		sshServerMu.Unlock()
		registry.Register(runtime.RuntimeTypeContainer, containerRT)
	}

	return registry
}

// sshServerInstance and sshServerMu are declared in cmd.go
// These variables are package-level and shared across files

// ensureSSHServer starts the SSH server if not already running
func ensureSSHServer() (*sshserver.Server, error) {
	sshServerMu.Lock()
	defer sshServerMu.Unlock()

	if sshServerInstance != nil && sshServerInstance.IsRunning() {
		return sshServerInstance, nil
	}

	srv := sshserver.New(sshserver.DefaultConfig())

	// Start blocks until the server is ready to accept connections or fails
	if err := srv.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start SSH server: %w", err)
	}

	sshServerInstance = srv
	return srv, nil
}

// stopSSHServer stops the SSH server if running
func stopSSHServer() {
	sshServerMu.Lock()
	defer sshServerMu.Unlock()

	if sshServerInstance != nil {
		_ = sshServerInstance.Stop()
		sshServerInstance = nil
	}
}

// bridgeTUIRequests reads TUI component requests from the server's channel
// and sends them to the parent Bubbletea program for rendering as overlays.
// It runs in a goroutine and terminates when the server stops.
func bridgeTUIRequests(server *tuiserver.Server, program *tea.Program) {
	for req := range server.RequestChannel() {
		// Send the request to the interactive model for rendering
		program.Send(tui.TUIComponentMsg{
			Component:  tui.ComponentType(req.Component),
			Options:    req.Options,
			ResponseCh: req.ResponseCh,
		})
	}
}
