// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/internal/testutil/invowkfiletest"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	stubCommandDiscovery struct {
		lookup    discovery.LookupResult
		lookupErr error
	}

	staticCommandsvcConfigProvider struct {
		cfg *config.Config
	}

	recordingHostAccess struct {
		ensureCalls int
		running     bool
	}
)

func (s *stubCommandDiscovery) DiscoverCommandSet(context.Context) (discovery.CommandSetResult, error) {
	return discovery.CommandSetResult{}, nil
}

func (s *stubCommandDiscovery) GetCommand(context.Context, string) (discovery.LookupResult, error) {
	return s.lookup, s.lookupErr
}

func TestServiceDiscoverCommand(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	configFallback := func(context.Context, config.Provider, string) (*config.Config, []discovery.Diagnostic) {
		return cfg, nil
	}
	service := &Service{
		config:         &staticCommandsvcConfigProvider{cfg: cfg},
		discovery:      &stubCommandDiscovery{},
		stdout:         io.Discard,
		stderr:         io.Discard,
		configFallback: configFallback,
	}

	cmdInfo := commandsvcTestCommandInfo(t, "build")
	foundCfg, foundCmd, diags, err := service.discoverCommand(t.Context(), Request{
		Name:            "build",
		ResolvedCommand: cmdInfo,
	})
	if err != nil {
		t.Fatalf("discoverCommand(resolved) error = %v", err)
	}
	if foundCfg != cfg || foundCmd != cmdInfo || len(diags) != 0 {
		t.Fatalf("discoverCommand(resolved) = (%v, %v, %v)", foundCfg, foundCmd, diags)
	}

	service.discovery = &stubCommandDiscovery{lookup: discovery.LookupResult{}}
	_, _, _, err = service.discoverCommand(t.Context(), Request{Name: "missing"})
	if err == nil {
		t.Fatal("expected missing-command error")
	}
	var classified *ClassifiedError
	if !errors.As(err, &classified) {
		t.Fatalf("errors.As(*ClassifiedError) = false for %T", err)
	}
	if classified.IssueID != issue.CommandNotFoundId {
		t.Fatalf("classified.IssueID = %v, want %v", classified.IssueID, issue.CommandNotFoundId)
	}

	diag, diagErr := discovery.NewDiagnostic(discovery.SeverityWarning, discovery.CodeConfigLoadFailed, "warn")
	if diagErr != nil {
		t.Fatalf("NewDiagnostic(): %v", diagErr)
	}
	service.discovery = &stubCommandDiscovery{
		lookup:    discovery.LookupResult{Diagnostics: []discovery.Diagnostic{diag}},
		lookupErr: errors.New("lookup failed"),
	}
	_, _, diags, err = service.discoverCommand(t.Context(), Request{Name: "build"})
	if err == nil || len(diags) != 1 {
		t.Fatalf("discoverCommand(lookupErr) err=%v diags=%v", err, diags)
	}
}

func TestResolveDefinitionsAndLoadConfig(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	calls := 0
	service := &Service{
		config: &staticCommandsvcConfigProvider{cfg: cfg},
		configFallback: func(context.Context, config.Provider, string) (*config.Config, []discovery.Diagnostic) {
			calls++
			return cfg, nil
		},
	}

	flagName := invowkfile.FlagName("mode")
	cmdInfo := commandsvcTestCommandInfo(t, "build")
	cmdInfo.Command.Flags = []invowkfile.Flag{{
		Name:         flagName,
		Type:         invowkfile.FlagTypeString,
		DefaultValue: "safe",
	}}
	cmdInfo.Command.Args = []invowkfile.Argument{{Name: "target"}}

	defs := service.resolveDefinitions(Request{}, cmdInfo)
	if defs.flagValues[flagName] != "safe" {
		t.Fatalf("flagValues[%q] = %q, want safe", flagName, defs.flagValues[flagName])
	}
	if len(defs.argDefs) != 1 || defs.argDefs[0].Name != "target" {
		t.Fatalf("argDefs = %v", defs.argDefs)
	}

	loaded, diags := service.loadConfig(t.Context(), "")
	if loaded != cfg || len(diags) != 0 || calls != 1 {
		t.Fatalf("loadConfig() = (%v, %v), calls=%d", loaded, diags, calls)
	}
}

func TestServiceExecute_DryRunDoesNotStartHostAccess(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	hostAccess := &recordingHostAccess{}
	cmdInfo := commandsvcTestCommandInfo(t, "build")
	cmdInfo.Command.Implementations[0].Runtimes = []invowkfile.RuntimeConfig{{
		Name:          invowkfile.RuntimeContainer,
		Image:         "debian:stable-slim",
		EnableHostSSH: true,
	}}

	service := &Service{
		config:          &staticCommandsvcConfigProvider{cfg: cfg},
		discovery:       &stubCommandDiscovery{lookup: discovery.LookupResult{Command: cmdInfo}},
		stdout:          io.Discard,
		stderr:          io.Discard,
		hostAccess:      hostAccess,
		registryFactory: defaultRuntimeRegistryFactory{},
		interactive:     defaultInteractiveExecutor{},
		userEnvFunc:     func() map[string]string { return map[string]string{} },
		configFallback: func(context.Context, config.Provider, string) (*config.Config, []discovery.Diagnostic) {
			return cfg, nil
		},
	}

	result, diags, err := service.Execute(t.Context(), Request{
		Name:    "build",
		DryRun:  true,
		Runtime: invowkfile.RuntimeContainer,
	})
	if err != nil {
		t.Fatalf("Execute(dry-run) error = %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("Execute(dry-run) diagnostics = %v, want none", diags)
	}
	if result.DryRunData == nil {
		t.Fatal("Execute(dry-run) did not return DryRunData")
	}
	if hostAccess.ensureCalls != 0 {
		t.Fatalf("HostAccess.Ensure called %d times for dry-run, want 0", hostAccess.ensureCalls)
	}
	if hostAccess.running {
		t.Fatal("HostAccess left running after dry-run")
	}
}

func (h *recordingHostAccess) Ensure(context.Context) error {
	h.ensureCalls++
	h.running = true
	return nil
}

func (h *recordingHostAccess) Running() bool {
	return h.running
}

func (h *recordingHostAccess) Stop() {
	h.running = false
}

func commandsvcTestCommandInfo(t testing.TB, name string) *discovery.CommandInfo {
	t.Helper()
	cmd := invowkfiletest.NewTestCommand(name,
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithRuntime(invowkfile.RuntimeVirtual),
		invowkfiletest.WithAllPlatforms(),
	)
	inv := &invowkfile.Invowkfile{FilePath: types.FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue"))}
	return &discovery.CommandInfo{
		Name:       cmd.Name,
		Command:    cmd,
		Invowkfile: inv,
		SourceID:   discovery.SourceIDInvowkfile,
	}
}

func (p *staticCommandsvcConfigProvider) Load(context.Context, config.LoadOptions) (*config.Config, error) {
	return p.cfg, nil
}
