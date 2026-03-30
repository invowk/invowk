// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	recordingCommandService struct {
		lastConfigPath string
	}

	recordingDiscoveryService struct {
		result         discovery.CommandSetResult
		lastConfigPath string
	}

	lookupDiscoveryService struct {
		lookup discovery.LookupResult
		calls  int
	}

	fixedConfigProvider struct {
		cfg *config.Config
		err error
	}
)

func (s *recordingCommandService) Execute(ctx context.Context, _ ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	s.lastConfigPath = configPathFromContext(ctx)
	return ExecuteResult{}, nil, nil
}

func (s *recordingDiscoveryService) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	s.lastConfigPath = configPathFromContext(ctx)
	return s.result, nil
}

func (s *recordingDiscoveryService) DiscoverAndValidateCommandSet(_ context.Context) (discovery.CommandSetResult, error) {
	return s.result, nil
}

func (s *recordingDiscoveryService) GetCommand(_ context.Context, _ string) (discovery.LookupResult, error) {
	return discovery.LookupResult{}, nil
}

func (s *lookupDiscoveryService) DiscoverCommandSet(_ context.Context) (discovery.CommandSetResult, error) {
	return discovery.CommandSetResult{}, nil
}

func (s *lookupDiscoveryService) DiscoverAndValidateCommandSet(_ context.Context) (discovery.CommandSetResult, error) {
	return discovery.CommandSetResult{}, nil
}

func (s *lookupDiscoveryService) GetCommand(_ context.Context, _ string) (discovery.LookupResult, error) {
	s.calls++
	return s.lookup, nil
}

func (p *fixedConfigProvider) Load(_ context.Context, _ config.LoadOptions) (*config.Config, error) {
	if p.err != nil {
		return nil, p.err
	}
	if p.cfg != nil {
		return p.cfg, nil
	}
	return config.DefaultConfig(), nil
}

func TestExecuteRequest_AttachesConfigPathToContext(t *testing.T) {
	t.Parallel()

	commands := &recordingCommandService{}
	app := &App{
		Commands:    commands,
		Diagnostics: &defaultDiagnosticRenderer{},
		stderr:      io.Discard,
	}

	customCuePath := filepath.Join(t.TempDir(), "custom.cue")
	req := ExecuteRequest{
		Name:       "build",
		ConfigPath: types.FilesystemPath(customCuePath),
	}
	cmd := &cobra.Command{}

	if err := executeRequest(cmd, app, req); err != nil {
		t.Fatalf("executeRequest() error = %v", err)
	}

	if commands.lastConfigPath != string(req.ConfigPath) {
		t.Fatalf("config path in context = %q, want %q", commands.lastConfigPath, req.ConfigPath)
	}
}

func TestRunDisambiguatedCommand_AttachesConfigPathToContext(t *testing.T) {
	t.Parallel()

	set := discovery.NewDiscoveredCommandSet()
	set.SourceOrder = []discovery.SourceID{"invowkfile"}
	set.BySource[discovery.SourceIDInvowkfile] = []*discovery.CommandInfo{
		{
			Name:       "build",
			SimpleName: "build",
		},
	}

	disc := &recordingDiscoveryService{result: discovery.CommandSetResult{Set: set}}
	commands := &recordingCommandService{}
	rootFlags := &rootFlagValues{configPath: filepath.Join(t.TempDir(), "custom.cue")}

	app := &App{
		Config:      &fixedConfigProvider{cfg: config.DefaultConfig()},
		Discovery:   disc,
		Commands:    commands,
		Diagnostics: &defaultDiagnosticRenderer{},
		stderr:      io.Discard,
	}

	err := runDisambiguatedCommand(
		&cobra.Command{},
		app,
		rootFlags,
		&cmdFlagValues{},
		&SourceFilter{SourceID: discovery.SourceIDInvowkfile},
		[]string{"build"},
	)
	if err != nil {
		t.Fatalf("runDisambiguatedCommand() error = %v", err)
	}

	if disc.lastConfigPath != rootFlags.configPath {
		t.Fatalf("discovery context config path = %q, want %q", disc.lastConfigPath, rootFlags.configPath)
	}

	if commands.lastConfigPath != rootFlags.configPath {
		t.Fatalf("execute context config path = %q, want %q", commands.lastConfigPath, rootFlags.configPath)
	}
}

func TestDiscoverCommand_DoesNotDuplicateConfigDiagnostics(t *testing.T) {
	t.Parallel()

	// The service's discoverCommand delegates config loading to the configFallback
	// callback. Discovery diagnostics come from the DiscoveryService. This test
	// verifies that config-load diagnostics (produced by the fallback) are not
	// duplicated with discovery diagnostics when the service returns both.
	svc := commandsvc.New(
		&fixedConfigProvider{err: errors.New("load failed")},
		&lookupDiscoveryService{
			lookup: discovery.LookupResult{
				Command: &discovery.CommandInfo{
					Name: "build",
					Command: &invowkfile.Command{
						Name:            "build",
						Implementations: buildMinimalImpl(),
					},
					Invowkfile: &invowkfile.Invowkfile{},
				},
				Diagnostics: []discovery.Diagnostic{
					testMustDiagnostic(t, discovery.SeverityWarning, discovery.CodeCommandNotFound, "from discovery"),
				},
			},
		},
		io.Discard,
		io.Discard,
		func() map[string]string { return nil },
		testConfigFallback,
	)

	customCuePath2 := filepath.Join(t.TempDir(), "custom.cue")
	req := commandsvc.Request{
		Name:       "build",
		ConfigPath: types.FilesystemPath(customCuePath2),
	}
	ctx := contextWithConfigPath(t.Context(), string(req.ConfigPath))

	// Execute the full pipeline; the service returns diagnostics from discovery only.
	// Config diagnostics are emitted separately by the configFallback callback
	// and should not be mixed into the discovery diagnostics.
	_, diags, err := svc.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// The discovery returns 1 diagnostic; verify no duplication.
	if len(diags) != 1 {
		t.Fatalf("Execute() diagnostics count = %d, want 1; diagnostics=%#v", len(diags), diags)
	}

	if diags[0].Code() != discovery.CodeCommandNotFound {
		t.Fatalf("Execute() diagnostic code = %q, want %q", diags[0].Code(), discovery.CodeCommandNotFound)
	}
}

func TestDiscoverCommand_ResolvedCommandSkipsLookup(t *testing.T) {
	t.Parallel()

	disc := &lookupDiscoveryService{}
	svc := commandsvc.New(
		&fixedConfigProvider{cfg: config.DefaultConfig()},
		disc,
		io.Discard,
		io.Discard,
		func() map[string]string { return nil },
		testConfigFallback,
	)

	resolved := &discovery.CommandInfo{
		Name:       "build",
		SimpleName: "build",
		SourceID:   discovery.SourceIDInvowkfile,
		Command: &invowkfile.Command{
			Name:            "build",
			Implementations: buildMinimalImpl(),
		},
		Invowkfile: &invowkfile.Invowkfile{},
	}

	req := commandsvc.Request{
		Name:            "build",
		DryRun:          true,
		ResolvedCommand: resolved,
	}
	ctx := contextWithConfigPath(t.Context(), "")

	if _, _, err := svc.Execute(ctx, req); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if disc.calls != 0 {
		t.Fatalf("GetCommand() calls = %d, want 0 when ResolvedCommand is provided", disc.calls)
	}
}

// buildMinimalImpl returns a minimal implementation set for all platforms
// to satisfy runtime resolution in tests.
func buildMinimalImpl() []invowkfile.Implementation {
	return []invowkfile.Implementation{
		{
			Script:    "echo test",
			Platforms: invowkfile.AllPlatformConfigs(),
			Runtimes: []invowkfile.RuntimeConfig{
				{Name: invowkfile.RuntimeNative},
			},
		},
	}
}

// testMustDiagnostic creates a discovery.Diagnostic and fails the test if construction fails.
func testMustDiagnostic(t *testing.T, severity discovery.Severity, code discovery.DiagnosticCode, message string) discovery.Diagnostic {
	t.Helper()
	d, err := discovery.NewDiagnostic(severity, code, message)
	if err != nil {
		t.Fatalf("NewDiagnostic(%q, %q, %q) unexpected error: %v", severity, code, message, err)
	}
	return d
}
