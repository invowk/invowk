// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type (
	configAwareDynamicDiscovery struct {
		configPath string
		result     discovery.CommandSetResult
		seenPaths  []string
	}

	recordingDynamicCommandService struct {
		requests []ExecuteRequest
	}
)

func TestShouldRegisterDiscoveredCommands(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "empty args",
			args: nil,
			want: false,
		},
		{
			name: "root version flag",
			args: []string{"--version"},
			want: false,
		},
		{
			name: "direct cmd invocation",
			args: []string{"cmd", "build"},
			want: true,
		},
		{
			name: "cmd with long config flag",
			args: []string{"--ivk-config", filepath.Join(tmpDir, "config.cue"), "cmd", "build"},
			want: true,
		},
		{
			name: "cmd with short config flag",
			args: []string{"-c", filepath.Join(tmpDir, "config.cue"), "cmd", "build"},
			want: true,
		},
		{
			name: "cmd with root bool flags",
			args: []string{"--ivk-verbose", "--ivk-interactive", "cmd", "build"},
			want: true,
		},
		{
			name: "shell completion for cmd",
			args: []string{"__complete", "cmd", "bu"},
			want: true,
		},
		{
			name: "shell completion no-desc for cmd",
			args: []string{"__completeNoDesc", "cmd", "bu"},
			want: true,
		},
		{
			name: "non-cmd command",
			args: []string{"init"},
			want: false,
		},
		{
			name: "arg terminator before cmd",
			args: []string{"--", "cmd", "build"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldRegisterDiscoveredCommands(tt.args)
			if got != tt.want {
				t.Fatalf("shouldRegisterDiscoveredCommands(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestExplicitConfigPathRegistersDynamicCommandsBeforeCobraParse(t *testing.T) { //nolint:paralleltest // test mutates process-wide os.Args for Cobra parsing.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom-config.cue")
	commandSet := dynamicConfigCommandSet(t, tmpDir)
	discoverySvc := &configAwareDynamicDiscovery{
		configPath: configPath,
		result:     discovery.CommandSetResult{Set: commandSet},
	}
	commandSvc := &recordingDynamicCommandService{}
	app, err := NewApp(Dependencies{
		Config:    &staticConfigProvider{cfg: config.DefaultConfig()},
		Discovery: discoverySvc,
		Commands:  commandSvc,
		Stdout:    io.Discard,
		Stderr:    io.Discard,
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	originalArgs := os.Args
	os.Args = []string{"invowk", "--ivk-config", configPath, "cmd", "custom-command", "--flavor", "vanilla"}
	t.Cleanup(func() {
		os.Args = originalArgs
	})
	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs(os.Args[1:])

	if err := rootCmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if len(discoverySvc.seenPaths) == 0 || discoverySvc.seenPaths[0] != configPath {
		t.Fatalf("registration config paths = %v, want first %q", discoverySvc.seenPaths, configPath)
	}
	if len(commandSvc.requests) != 1 {
		t.Fatalf("Execute requests = %d, want 1", len(commandSvc.requests))
	}
	req := commandSvc.requests[0]
	if req.Name != "custom custom-command" {
		t.Fatalf("request name = %q, want source-qualified command", req.Name)
	}
	if got := req.FlagValues["flavor"]; got != "vanilla" {
		t.Fatalf("request flag flavor = %q, want vanilla", got)
	}
	if req.ConfigPath != types.FilesystemPath(configPath) {
		t.Fatalf("request config path = %q, want %q", req.ConfigPath, configPath)
	}
}

func (d *configAwareDynamicDiscovery) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	return d.resultForContext(ctx), nil
}

func (d *configAwareDynamicDiscovery) DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	return d.resultForContext(ctx), nil
}

func (d *configAwareDynamicDiscovery) DiscoverModules(context.Context) (discovery.ModuleListResult, error) {
	return discovery.ModuleListResult{}, nil
}

func (d *configAwareDynamicDiscovery) GetCommand(context.Context, string) (discovery.LookupResult, error) {
	return discovery.LookupResult{}, nil
}

func (d *configAwareDynamicDiscovery) resultForContext(ctx context.Context) discovery.CommandSetResult {
	configPath := configPathFromContext(ctx)
	d.seenPaths = append(d.seenPaths, configPath)
	if configPath == d.configPath {
		return d.result
	}
	return discovery.CommandSetResult{Set: discovery.NewDiscoveredCommandSet()}
}

func (s *recordingDynamicCommandService) Execute(_ context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	s.requests = append(s.requests, req)
	return ExecuteResult{}, nil, nil
}

func (s *recordingDynamicCommandService) ResolveCommand(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	return nil, req, nil, nil
}

func (s *recordingDynamicCommandService) ResolveWatchPlan(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, commandsvc.WatchPlan, []discovery.Diagnostic, error) {
	return nil, req, commandsvc.WatchPlan{}, nil, nil
}

func (s *recordingDynamicCommandService) ResolveFromSource(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	return nil, req, nil, nil
}

func dynamicConfigCommandSet(t *testing.T, tmpDir string) *discovery.DiscoveredCommandSet {
	t.Helper()

	moduleID := invowkmod.ModuleID("io.example.custom")
	cmdInfo := &discovery.CommandInfo{
		Name:        "custom custom-command",
		Description: "Custom command from explicit config",
		Source:      discovery.SourceModule,
		FilePath:    types.FilesystemPath(filepath.Join(tmpDir, "custom.invowkmod", "invowkfile.cue")),
		Command: &invowkfile.Command{
			Name:        "custom-command",
			Description: "Custom command from explicit config",
			Flags: []invowkfile.Flag{{
				Name:        "flavor",
				Description: "Flavor value",
				Type:        invowkfile.FlagTypeString,
			}},
		},
		SimpleName: "custom-command",
		SourceID:   "custom",
		ModuleID:   &moduleID,
	}
	commandSet := discovery.NewDiscoveredCommandSet()
	commandSet.Add(cmdInfo)
	commandSet.Analyze()
	return commandSet
}
