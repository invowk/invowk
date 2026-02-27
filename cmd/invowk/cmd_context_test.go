// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/spf13/cobra"

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

	req := ExecuteRequest{
		Name:       "build",
		ConfigPath: types.FilesystemPath("/tmp/custom.cue"),
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
	rootFlags := &rootFlagValues{configPath: "/tmp/custom.cue"}

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

	svc := &commandService{
		config: &fixedConfigProvider{err: errors.New("load failed")},
		discovery: &lookupDiscoveryService{
			lookup: discovery.LookupResult{
				Command: &discovery.CommandInfo{
					Name: "build",
					Command: &invowkfile.Command{
						Name: "build",
					},
				},
				Diagnostics: []discovery.Diagnostic{
					testMustDiagnostic(t, discovery.SeverityWarning, discovery.CodeCommandNotFound, "from discovery"),
				},
			},
		},
		stdout: io.Discard,
		stderr: io.Discard,
		ssh:    &sshServerController{},
	}

	req := ExecuteRequest{
		Name:       "build",
		ConfigPath: types.FilesystemPath("/tmp/custom.cue"),
	}
	ctx := contextWithConfigPath(t.Context(), string(req.ConfigPath))

	_, _, diags, err := svc.discoverCommand(ctx, req)
	if err != nil {
		t.Fatalf("discoverCommand() error = %v", err)
	}

	if len(diags) != 1 {
		t.Fatalf("discoverCommand() diagnostics count = %d, want 1; diagnostics=%#v", len(diags), diags)
	}

	if diags[0].Code() != discovery.CodeCommandNotFound {
		t.Fatalf("discoverCommand() diagnostic code = %q, want %q", diags[0].Code(), discovery.CodeCommandNotFound)
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
