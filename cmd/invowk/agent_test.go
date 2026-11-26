// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/agentcmd"
	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	countingDefaultErrorConfigProvider struct {
		loads int
		err   error
	}

	countingDiscoveryService struct {
		*commandadapters.DiscoveryService
		discoverSetCalls      int
		discoverValidateCalls int
		getCommandCalls       int
	}
)

func TestValidateAgentCmdCreateModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dryRun    bool
		printOnly bool
		verify    bool
		wantErr   string
	}{
		{name: "write mode"},
		{name: "dry run", dryRun: true},
		{name: "print", printOnly: true},
		{name: "verify write", verify: true},
		{name: "dry run and print", dryRun: true, printOnly: true, wantErr: "--dry-run and --print"},
		{name: "verify and dry run", dryRun: true, verify: true, wantErr: "--verify requires writing"},
		{name: "verify and print", printOnly: true, verify: true, wantErr: "--verify requires writing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAgentCmdCreateModes(tt.dryRun, tt.printOnly, tt.verify)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateAgentCmdCreateModes() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateAgentCmdCreateModes() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestAgentCommandTreeIncludesAuthoringCRUD(t *testing.T) {
	t.Parallel()

	app, err := NewApp(Dependencies{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	agent := newAgentCommand(app, &rootFlagValues{})

	for _, path := range []string{
		"cmd prompt",
		"cmd create",
		"cmd change",
		"cmd remove",
		"mod prompt",
		"mod create",
		"mod change",
		"mod remove",
	} {
		if !hasCommandPath(agent, strings.Fields(path)) {
			t.Fatalf("agent command tree missing %q", path)
		}
	}
}

func TestValidateCommandAuthoringInputRequiresDescription(t *testing.T) {
	t.Parallel()

	err := validateCommandAuthoringInput(invowkfile.CommandName("legacy description only"), "", "")
	if err == nil || !strings.Contains(err.Error(), "command description is required") {
		t.Fatalf("validateCommandAuthoringInput() error = %v, want required description", err)
	}
}

func hasCommandPath(cmd *cobra.Command, path []string) bool {
	if len(path) == 0 {
		return true
	}
	for _, child := range cmd.Commands() {
		if child.Name() == path[0] {
			return hasCommandPath(child, path[1:])
		}
	}
	return false
}

func TestVerifySourceForWrittenCommand(t *testing.T) {
	t.Parallel()

	const cmdCUE = `cmds: [{name: "lint", implementations: [{script: {content: "echo lint"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`
	writeFile := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	commandSet := func(t *testing.T, base string) *discovery.DiscoveredCommandSet {
		t.Helper()
		d := discovery.New(config.DefaultConfig(), discovery.WithBaseDir(types.FilesystemPath(base)), discovery.WithCommandsDir(""))
		result, err := d.DiscoverCommandSet(t.Context())
		if err != nil {
			t.Fatalf("DiscoverCommandSet() error: %v", err)
		}
		return result.Set
	}

	moduleDir := filepath.Join(t.TempDir(), "tools.invowkmod")
	writeFile(t, filepath.Join(moduleDir, "invowkmod.cue"), "module: \"tools\"\nversion: \"1.0.0\"\n")
	writeFile(t, filepath.Join(moduleDir, agentInvowkfileName), cmdCUE)
	plainDir := filepath.Join(t.TempDir(), "project")
	writeFile(t, filepath.Join(plainDir, agentInvowkfileName), cmdCUE)

	tests := []struct {
		name string
		base string
		cmd  invowkfile.CommandName
		want discovery.SourceID
	}{
		{name: "module directory", base: moduleDir, cmd: "lint", want: "tools"},
		{name: "plain directory", base: plainDir, cmd: "lint", want: discovery.SourceIDInvowkfile},
		{name: "name not written", base: moduleDir, cmd: "missing", want: discovery.SourceIDInvowkfile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := types.FilesystemPath(filepath.Join(tt.base, agentInvowkfileName))
			if got := verifySourceForWrittenCommand(commandSet(t, tt.base), target, tt.cmd); got != tt.want {
				t.Fatalf("verifySourceForWrittenCommand() = %q, want %q", got, tt.want)
			}
		})
	}
	if got := verifySourceForWrittenCommand(nil, types.FilesystemPath(filepath.Join(moduleDir, agentInvowkfileName)), "lint"); got != discovery.SourceIDInvowkfile {
		t.Fatalf("nil set: got %q, want %q", got, discovery.SourceIDInvowkfile)
	}
}

func (p *countingDefaultErrorConfigProvider) Load(context.Context, config.LoadOptions) (*config.Config, error) {
	p.loads++
	return nil, p.err
}

func (p *countingDefaultErrorConfigProvider) LoadWithSource(ctx context.Context, opts config.LoadOptions) (config.LoadResult, error) {
	cfg, err := p.Load(ctx, opts)
	if err != nil {
		return config.LoadResult{}, err
	}
	return config.LoadResult{Config: cfg}, nil
}

func (s *countingDiscoveryService) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	s.discoverSetCalls++
	return s.DiscoveryService.DiscoverCommandSet(ctx)
}

func (s *countingDiscoveryService) DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	s.discoverValidateCalls++
	return s.DiscoveryService.DiscoverAndValidateCommandSet(ctx)
}

func (s *countingDiscoveryService) GetCommand(ctx context.Context, name string) (discovery.LookupResult, error) {
	s.getCommandCalls++
	return s.DiscoveryService.GetCommand(ctx, name)
}

func TestVerifyGeneratedCommand_SharesDiscoveryCache(t *testing.T) { //nolint:paralleltest // subtests mutate process cwd via t.Chdir.
	tests := []struct {
		name       string
		dirName    string
		moduleName string
		wantSource string
	}{
		{name: "plain directory", dirName: "project", wantSource: "invowkfile"},
		{name: "module directory", dirName: "tools.invowkmod", moduleName: "tools", wantSource: "tools"},
	}

	//nolint:paralleltest // subtests mutate process cwd via t.Chdir.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runVerifyGeneratedCommandCacheCase(t, tt.dirName, tt.moduleName, tt.wantSource)
		})
	}
}

func runVerifyGeneratedCommandCacheCase(t *testing.T, dirName, moduleName, wantSource string) {
	t.Helper()
	const cmdCUE = `cmds: [{name: "lint", implementations: [{script: {content: "echo ok"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`

	dir := filepath.Join(t.TempDir(), dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if moduleName != "" {
		if err := os.WriteFile(filepath.Join(dir, "invowkmod.cue"), []byte("module: \""+moduleName+"\"\nversion: \"1.0.0\"\n"), 0o644); err != nil {
			t.Fatalf("write invowkmod.cue: %v", err)
		}
	}
	targetPath := filepath.Join(dir, agentInvowkfileName)
	if err := os.WriteFile(targetPath, []byte(cmdCUE), 0o644); err != nil {
		t.Fatalf("write %s: %v", targetPath, err)
	}

	// ensureCurrentInvowkfileTarget resolves "." against process cwd.
	t.Chdir(dir)

	cfgProvider := &countingDefaultErrorConfigProvider{err: errors.New("broken default config")}
	basePath := types.FilesystemPath(dir)
	if err := basePath.Validate(); err != nil {
		t.Fatalf("validate dir path: %v", err)
	}
	innerDisc, err := commandadapters.NewDiscoveryServiceWithDirs(cfgProvider, basePath, "")
	if err != nil {
		t.Fatalf("NewDiscoveryServiceWithDirs() error = %v", err)
	}
	discSvc := &countingDiscoveryService{DiscoveryService: innerDisc}

	var stdout, stderr bytes.Buffer
	app, err := NewApp(Dependencies{
		Config:    cfgProvider,
		Discovery: discSvc,
		Stdout:    &stdout,
		Stderr:    &stderr,
		Getenv:    func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	cmd := &cobra.Command{Use: "create"}
	cmd.SetContext(t.Context())
	result := &agentcmd.CreateResult{
		CommandName: "lint",
		TargetPath:  targetPath,
	}

	if err := verifyGeneratedCommand(cmd, app, &rootFlagValues{}, result); err != nil {
		t.Fatalf("verifyGeneratedCommand() error = %v\nstderr:\n%s", err, stderr.String())
	}

	assertVerifyCacheTelemetry(t, cfgProvider, discSvc, stderr.String(), stdout.String(), wantSource)
}

func assertVerifyCacheTelemetry(
	t *testing.T,
	cfgProvider *countingDefaultErrorConfigProvider,
	discSvc *countingDiscoveryService,
	stderrText, stdoutText, wantSource string,
) {
	t.Helper()
	if cfgProvider.loads != 1 {
		t.Fatalf("Config.Load count = %d, want 1 (cache must be shared with executeRequest)", cfgProvider.loads)
	}
	// verifyGeneratedCommand calls DiscoverCommandSet once for source resolution,
	// then ResolveFromSource inside executeRequest calls DiscoverCommandSet again
	// and hits the shared request cache (zero additional Config.Load or GetCommand calls).
	if discSvc.discoverSetCalls != 2 || discSvc.discoverValidateCalls != 0 || discSvc.getCommandCalls != 0 {
		t.Fatalf("discovery calls = (set=%d, validate=%d, get=%d), want (2, 0, 0)",
			discSvc.discoverSetCalls, discSvc.discoverValidateCalls, discSvc.getCommandCalls)
	}
	if got := strings.Count(stderrText, "broken default config"); got != 1 {
		t.Fatalf("fallback config warning count = %d, want 1\nstderr:\n%s", got, stderrText)
	}
	if !strings.Contains(stdoutText, "lint") || !strings.Contains(stdoutText, wantSource) {
		t.Fatalf("dry-run stdout missing command %q or source %q:\n%s", "lint", wantSource, stdoutText)
	}
}
