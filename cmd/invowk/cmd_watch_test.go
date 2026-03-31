// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"io"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// newWatchTestCmd creates a minimal *cobra.Command with a config-path-enhanced context
// suitable for calling runWatchMode.
func newWatchTestCmd(t *testing.T) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{}
	cmd.SetContext(contextWithConfigPath(t.Context(), ""))
	return cmd
}

// newWatchTestApp creates an App with the given discovery service and discarded I/O.
func newWatchTestApp(disc DiscoveryService) *App {
	return &App{
		Discovery:   disc,
		Diagnostics: &defaultDiagnosticRenderer{},
		stdout:      io.Discard,
		stderr:      io.Discard,
	}
}

// emptyCommandSet returns an initialized (but empty) CommandSetResult.
// Without this, checkAmbiguousCommand panics on nil map access.
func emptyCommandSet() discovery.CommandSetResult {
	return discovery.CommandSetResult{Set: discovery.NewDiscoveredCommandSet()}
}

func TestRunWatchMode_NoCommand(t *testing.T) {
	t.Parallel()

	err := runWatchMode(
		newWatchTestCmd(t),
		newWatchTestApp(&stubDiscoveryService{}),
		&rootFlagValues{},
		&cmdFlagValues{},
		nil, // no args
	)
	if !errors.Is(err, errNoCommandSpecified) {
		t.Fatalf("error = %v, want errNoCommandSpecified", err)
	}
}

func TestRunWatchMode_DryRunConflict(t *testing.T) {
	t.Parallel()

	err := runWatchMode(
		newWatchTestCmd(t),
		newWatchTestApp(&stubDiscoveryService{}),
		&rootFlagValues{},
		&cmdFlagValues{dryRun: true},
		[]string{"build"},
	)
	if !errors.Is(err, errWatchDryRunConflict) {
		t.Fatalf("error = %v, want errWatchDryRunConflict", err)
	}
}

func TestRunWatchMode_CommandNotFound(t *testing.T) {
	t.Parallel()

	disc := &stubDiscoveryService{
		commandSet: emptyCommandSet(),
	}

	err := runWatchMode(
		newWatchTestCmd(t),
		newWatchTestApp(disc),
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"nonexistent"},
	)
	var cmdNotFound *WatchCommandNotFoundError
	if !errors.As(err, &cmdNotFound) {
		t.Fatalf("error = %v (%T), want *WatchCommandNotFoundError", err, err)
	}
	if cmdNotFound.Name != "nonexistent" {
		t.Fatalf("WatchCommandNotFoundError.Name = %q, want %q", cmdNotFound.Name, "nonexistent")
	}
}

func TestRunWatchMode_GetCommandError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("discovery exploded")
	disc := &stubDiscoveryService{
		commandSet: emptyCommandSet(),
		lookupErr:  wantErr,
	}

	err := runWatchMode(
		newWatchTestCmd(t),
		newWatchTestApp(disc),
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"build"},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestRunWatchMode_InvalidDebounce(t *testing.T) {
	t.Parallel()

	disc := &stubDiscoveryService{
		commandSet: emptyCommandSet(),
		lookup: discovery.LookupResult{
			Command: &discovery.CommandInfo{
				Name: "build",
				Command: &invowkfile.Command{
					Name: "build",
					Watch: &invowkfile.WatchConfig{
						Patterns: []invowkfile.GlobPattern{"**/*"},
						Debounce: "not-a-duration",
					},
					Implementations: []invowkfile.Implementation{{
						Script:    "echo hello",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
						Platforms: invowkfile.AllPlatformConfigs(),
					}},
				},
				SourceID: discovery.SourceIDInvowkfile,
			},
		},
	}

	err := runWatchMode(
		newWatchTestCmd(t),
		newWatchTestApp(disc),
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"build"},
	)
	if !errors.Is(err, errInvalidWatchDebounce) {
		t.Fatalf("error = %v, want errInvalidWatchDebounce", err)
	}
}

func TestRunWatchMode_AmbiguousCommand(t *testing.T) {
	t.Parallel()

	// Build a command set where "deploy" exists in two sources.
	set := discovery.NewDiscoveredCommandSet()
	set.BySimpleName["deploy"] = []*discovery.CommandInfo{
		{SimpleName: "deploy", SourceID: discovery.SourceIDInvowkfile},
		{SimpleName: "deploy", SourceID: "mymodule"},
	}
	set.AmbiguousNames["deploy"] = true
	set.BySource[discovery.SourceIDInvowkfile] = []*discovery.CommandInfo{
		{SimpleName: "deploy", SourceID: discovery.SourceIDInvowkfile},
	}
	set.BySource["mymodule"] = []*discovery.CommandInfo{
		{SimpleName: "deploy", SourceID: "mymodule"},
	}
	set.SourceOrder = []discovery.SourceID{discovery.SourceIDInvowkfile, "mymodule"}

	disc := &stubDiscoveryService{
		commandSet: discovery.CommandSetResult{Set: set},
	}

	err := runWatchMode(
		newWatchTestCmd(t),
		newWatchTestApp(disc),
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"deploy"},
	)
	var ambigErr *AmbiguousCommandError
	if !errors.As(err, &ambigErr) {
		t.Fatalf("error = %v (%T), want *AmbiguousCommandError", err, err)
	}
	if string(ambigErr.CommandName) != "deploy" {
		t.Fatalf("AmbiguousCommandError.CommandName = %q, want %q", ambigErr.CommandName, "deploy")
	}
}
