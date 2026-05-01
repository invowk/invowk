// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/watch"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	fakeWatchFactory struct {
		cfg watch.Config
		run func(context.Context, watch.Config) error
	}

	fakeWatchRunner struct {
		cfg watch.Config
		run func(context.Context, watch.Config) error
	}

	fakeWatchCommandService struct {
		cmdInfo      *discovery.CommandInfo
		executeErrs  []error
		exitCodes    []types.ExitCode
		executeCalls int
	}
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
func newWatchTestApp(t *testing.T, disc DiscoveryService) *App {
	t.Helper()

	app, err := NewApp(Dependencies{
		Config:    &staticConfigProvider{cfg: config.DefaultConfig()},
		Discovery: disc,
		Stdout:    io.Discard,
		Stderr:    io.Discard,
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	return app
}

// emptyCommandSet returns an initialized (but empty) CommandSetResult.
// Without this, checkAmbiguousCommand panics on nil map access.
func emptyCommandSet() discovery.CommandSetResult {
	return discovery.CommandSetResult{Set: discovery.NewDiscoveredCommandSet()}
}

func (f *fakeWatchFactory) Create(cfg watch.Config) (WatchRunner, error) {
	f.cfg = cfg
	return fakeWatchRunner{cfg: cfg, run: f.run}, nil
}

func (r fakeWatchRunner) Run(ctx context.Context) error {
	if r.run == nil {
		return nil
	}
	return r.run(ctx, r.cfg)
}

func (s *fakeWatchCommandService) Execute(context.Context, ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	s.executeCalls++
	result := ExecuteResult{}
	if len(s.exitCodes) > 0 {
		result.ExitCode = s.exitCodes[0]
		s.exitCodes = s.exitCodes[1:]
	}
	if len(s.executeErrs) == 0 {
		return result, nil, nil
	}
	err := s.executeErrs[0]
	s.executeErrs = s.executeErrs[1:]
	return result, nil, err
}

func (s *fakeWatchCommandService) ResolveCommand(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	req.Name = string(s.cmdInfo.Name)
	req.ResolvedCommand = s.cmdInfo
	return s.cmdInfo, req, nil, nil
}

func (s *fakeWatchCommandService) ResolveFromSource(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	return s.cmdInfo, req, nil, nil
}

func newResolvedWatchCommand(t *testing.T) *discovery.CommandInfo {
	t.Helper()

	return &discovery.CommandInfo{
		Name:       "build",
		SimpleName: "build",
		SourceID:   discovery.SourceIDInvowkfile,
		FilePath:   invowkfile.FilesystemPath(t.TempDir() + "/invowkfile.cue"),
		Command: &invowkfile.Command{
			Name: "build",
			Watch: &invowkfile.WatchConfig{
				Patterns: []invowkfile.GlobPattern{"**/*.go"},
			},
		},
	}
}

func TestRunWatchMode_NoCommand(t *testing.T) {
	t.Parallel()

	err := runWatchMode(
		newWatchTestCmd(t),
		newWatchTestApp(t, &stubDiscoveryService{}),
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
		newWatchTestApp(t, &stubDiscoveryService{}),
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
		newWatchTestApp(t, disc),
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"nonexistent"},
	)
	var cmdNotFound *ServiceError
	if !errors.As(err, &cmdNotFound) {
		t.Fatalf("error = %v (%T), want *ServiceError", err, err)
	}
}

func TestRunWatchMode_GetCommandError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("discovery exploded")
	disc := &stubDiscoveryService{
		lookupErr: wantErr,
	}

	err := runWatchMode(
		newWatchTestCmd(t),
		newWatchTestApp(t, disc),
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
		newWatchTestApp(t, disc),
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
		newWatchTestApp(t, disc),
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"deploy"},
	)
	var ambigErr *commandsvc.AmbiguousCommandError
	if !errors.As(err, &ambigErr) {
		t.Fatalf("error = %v (%T), want *commandsvc.AmbiguousCommandError", err, err)
	}
	if string(ambigErr.CommandName) != "deploy" {
		t.Fatalf("AmbiguousCommandError.CommandName = %q, want %q", ambigErr.CommandName, "deploy")
	}
}

func TestRunWatchMode_InjectedWatcherReexecutesOnChange(t *testing.T) {
	t.Parallel()

	commands := &fakeWatchCommandService{cmdInfo: newResolvedWatchCommand(t)}
	watchers := &fakeWatchFactory{
		run: func(ctx context.Context, cfg watch.Config) error {
			return cfg.OnChange(ctx, []string{"main.go"})
		},
	}
	app := &App{
		Config:      &staticConfigProvider{cfg: config.DefaultConfig()},
		Commands:    commands,
		Watchers:    watchers,
		Diagnostics: &defaultDiagnosticRenderer{},
		stdout:      io.Discard,
		stderr:      io.Discard,
	}

	err := runWatchMode(
		newWatchTestCmd(t),
		app,
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"build"},
	)
	if err != nil {
		t.Fatalf("runWatchMode() error = %v", err)
	}
	if commands.executeCalls != 2 {
		t.Fatalf("executeCalls = %d, want 2", commands.executeCalls)
	}
	if got := watchers.cfg.Patterns; len(got) != 1 || got[0] != "**/*.go" {
		t.Fatalf("watch Patterns = %v, want [**/*.go]", got)
	}
}

func TestRunWatchMode_NonZeroExitResetsInfrastructureFailureCounter(t *testing.T) {
	t.Parallel()

	infraErr := errors.New("config disappeared")
	commands := &fakeWatchCommandService{
		cmdInfo: newResolvedWatchCommand(t),
		executeErrs: []error{
			nil,
			infraErr,
			nil,
			infraErr,
			infraErr,
			nil,
		},
		exitCodes: []types.ExitCode{0, 0, 2, 0, 0, 0},
	}
	watchers := &fakeWatchFactory{
		run: func(ctx context.Context, cfg watch.Config) error {
			for range 5 {
				if err := cfg.OnChange(ctx, []string{"main.go"}); err != nil {
					return err
				}
			}
			return nil
		},
	}
	app := &App{
		Config:      &staticConfigProvider{cfg: config.DefaultConfig()},
		Commands:    commands,
		Watchers:    watchers,
		Diagnostics: &defaultDiagnosticRenderer{},
		stdout:      io.Discard,
		stderr:      io.Discard,
	}

	err := runWatchMode(
		newWatchTestCmd(t),
		app,
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"build"},
	)
	if err != nil {
		t.Fatalf("runWatchMode() error = %v", err)
	}
}

func TestRunWatchMode_ClearScreenOwnedByCLI(t *testing.T) {
	t.Parallel()

	commands := &fakeWatchCommandService{cmdInfo: newResolvedWatchCommand(t)}
	commands.cmdInfo.Command.Watch.ClearScreen = true

	var stdout strings.Builder
	watchers := &fakeWatchFactory{
		run: func(ctx context.Context, cfg watch.Config) error {
			return cfg.OnChange(ctx, []string{"main.go"})
		},
	}
	app := &App{
		Config:      &staticConfigProvider{cfg: config.DefaultConfig()},
		Commands:    commands,
		Watchers:    watchers,
		Diagnostics: &defaultDiagnosticRenderer{},
		stdout:      &stdout,
		stderr:      io.Discard,
	}

	err := runWatchMode(
		newWatchTestCmd(t),
		app,
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"build"},
	)
	if err != nil {
		t.Fatalf("runWatchMode() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "\033[2J\033[H") {
		t.Fatalf("stdout = %q, want ANSI clear sequence", stdout.String())
	}
}

func TestRunWatchMode_AbortsAfterConsecutiveInfrastructureFailures(t *testing.T) {
	t.Parallel()

	infraErr := errors.New("config disappeared")
	commands := &fakeWatchCommandService{
		cmdInfo: newResolvedWatchCommand(t),
		executeErrs: []error{
			nil,
			infraErr,
			infraErr,
			infraErr,
		},
	}
	watchers := &fakeWatchFactory{
		run: func(ctx context.Context, cfg watch.Config) error {
			for range 3 {
				if err := cfg.OnChange(ctx, []string{"main.go"}); err != nil {
					return err
				}
			}
			return nil
		},
	}
	app := &App{
		Config:      &staticConfigProvider{cfg: config.DefaultConfig()},
		Commands:    commands,
		Watchers:    watchers,
		Diagnostics: &defaultDiagnosticRenderer{},
		stdout:      io.Discard,
		stderr:      io.Discard,
	}

	err := runWatchMode(
		newWatchTestCmd(t),
		app,
		&rootFlagValues{},
		&cmdFlagValues{},
		[]string{"build"},
	)
	if err == nil || !strings.Contains(err.Error(), "aborting watch: 3 consecutive infrastructure failures") {
		t.Fatalf("runWatchMode() error = %v, want consecutive infrastructure failure", err)
	}
}
