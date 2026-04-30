// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/testutil/invowkfiletest"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	stubCommandDiscovery struct {
		lookup        discovery.LookupResult
		lookupErr     error
		commandSet    discovery.CommandSetResult
		commandSetErr error
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
	return s.commandSet, s.commandSetErr
}

func (s *stubCommandDiscovery) GetCommand(context.Context, string) (discovery.LookupResult, error) {
	return s.lookup, s.lookupErr
}

func TestServiceDiscoverCommand(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	configFallback := func(context.Context, config.Loader, string) (*config.Config, []discovery.Diagnostic) {
		return cfg, nil
	}
	service := &Service{
		config:         &staticCommandsvcConfigProvider{cfg: cfg},
		discovery:      &stubCommandDiscovery{},
		configFallback: configFallback,
	}

	cmdInfo := commandsvcTestCommandInfo(t, "build")
	foundCfg, foundCmd, _, diags, err := service.discoverCommand(t.Context(), Request{
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
	_, _, _, _, err = service.discoverCommand(t.Context(), Request{Name: "missing"})
	if err == nil {
		t.Fatal("expected missing-command error")
	}
	var classified *ClassifiedError
	if !errors.As(err, &classified) {
		t.Fatalf("errors.As(*ClassifiedError) = false for %T", err)
	}
	if classified.Kind != ErrorKindCommandNotFound {
		t.Fatalf("classified.Kind = %v, want %v", classified.Kind, ErrorKindCommandNotFound)
	}

	diag, diagErr := discovery.NewDiagnostic(discovery.SeverityWarning, discovery.CodeConfigLoadFailed, "warn")
	if diagErr != nil {
		t.Fatalf("NewDiagnostic(): %v", diagErr)
	}
	service.discovery = &stubCommandDiscovery{
		lookup:    discovery.LookupResult{Diagnostics: []discovery.Diagnostic{diag}},
		lookupErr: errors.New("lookup failed"),
	}
	_, _, _, diags, err = service.discoverCommand(t.Context(), Request{Name: "build"})
	if err == nil || len(diags) != 1 {
		t.Fatalf("discoverCommand(lookupErr) err=%v diags=%v", err, diags)
	}
}

func TestServiceResolveCommandRejectsAmbiguousLongestMatch(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	first := commandsvcTestCommandInfo(t, "deploy staging")
	first.Name = "deploy staging"
	first.SimpleName = "deploy staging"
	first.SourceID = discovery.SourceIDInvowkfile
	second := commandsvcTestCommandInfo(t, "deploy staging")
	second.Name = "tools deploy staging"
	second.SimpleName = "deploy staging"
	second.SourceID = "tools"

	commandSet := discovery.NewDiscoveredCommandSet()
	commandSet.Add(first)
	commandSet.Add(second)
	commandSet.Analyze()

	service := &Service{
		config:    &staticCommandsvcConfigProvider{cfg: cfg},
		discovery: &stubCommandDiscovery{commandSet: discovery.CommandSetResult{Set: commandSet}},
		configFallback: func(context.Context, config.Loader, string) (*config.Config, []discovery.Diagnostic) {
			return cfg, nil
		},
	}

	_, _, _, err := service.ResolveCommand(t.Context(), Request{
		Name: "deploy",
		Args: []string{"staging"},
	})
	if err == nil {
		t.Fatal("ResolveCommand() returned nil error, want ambiguous command")
	}
	var ambiguous *AmbiguousCommandError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("errors.As(*AmbiguousCommandError) = false for %T", err)
	}
	if ambiguous.CommandName != "deploy staging" {
		t.Fatalf("CommandName = %q, want deploy staging", ambiguous.CommandName)
	}
}

func TestServiceResolveFromSourceRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	service := &Service{}
	_, _, _, err := service.ResolveFromSource(t.Context(), Request{
		FromSource: discovery.SourceID("invalid source"),
	})
	if err == nil {
		t.Fatal("ResolveFromSource() returned nil error, want validation error")
	}
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ResolveFromSource() error = %v, want ErrInvalidRequest", err)
	}
}

func TestServiceDiscoverCommandFromSourceAdjustsArgs(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	configFallback := func(context.Context, config.Loader, string) (*config.Config, []discovery.Diagnostic) {
		return cfg, nil
	}
	cmdInfo := commandsvcTestCommandInfo(t, "deploy staging")
	cmdInfo.Name = "tools deploy staging"
	cmdInfo.SimpleName = "deploy staging"
	cmdInfo.SourceID = "tools"

	commandSet := discovery.NewDiscoveredCommandSet()
	commandSet.Add(cmdInfo)
	commandSet.Analyze()

	service := &Service{
		config:         &staticCommandsvcConfigProvider{cfg: cfg},
		discovery:      &stubCommandDiscovery{commandSet: discovery.CommandSetResult{Set: commandSet}},
		configFallback: configFallback,
	}

	_, foundCmd, resolvedReq, _, err := service.discoverCommand(t.Context(), Request{
		Name:       "deploy",
		Args:       []string{"staging", "prod"},
		FromSource: "tools",
	})
	if err != nil {
		t.Fatalf("discoverCommand(from source) error = %v", err)
	}
	if foundCmd != cmdInfo {
		t.Fatalf("foundCmd = %v, want %v", foundCmd, cmdInfo)
	}
	if resolvedReq.Name != "tools deploy staging" {
		t.Fatalf("resolvedReq.Name = %q, want tools deploy staging", resolvedReq.Name)
	}
	if len(resolvedReq.Args) != 1 || resolvedReq.Args[0] != "prod" {
		t.Fatalf("resolvedReq.Args = %v, want [prod]", resolvedReq.Args)
	}
}

func TestServiceDiscoverCommandFromSourceReturnsTypedSourceNotFound(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	configFallback := func(context.Context, config.Loader, string) (*config.Config, []discovery.Diagnostic) {
		return cfg, nil
	}
	cmdInfo := commandsvcTestCommandInfo(t, "deploy")
	cmdInfo.SourceID = "tools"

	commandSet := discovery.NewDiscoveredCommandSet()
	commandSet.Add(cmdInfo)
	commandSet.Analyze()

	service := &Service{
		config:         &staticCommandsvcConfigProvider{cfg: cfg},
		discovery:      &stubCommandDiscovery{commandSet: discovery.CommandSetResult{Set: commandSet}},
		configFallback: configFallback,
	}

	_, _, _, _, err := service.discoverCommand(t.Context(), Request{
		Name:       "deploy",
		FromSource: "missing",
	})
	if err == nil {
		t.Fatal("discoverCommand(from missing source) error = nil, want error")
	}

	var classified *ClassifiedError
	if !errors.As(err, &classified) {
		t.Fatalf("errors.As(*ClassifiedError) = false for %T", err)
	}
	var sourceErr *SourceNotFoundError
	if !errors.As(err, &sourceErr) {
		t.Fatalf("errors.As(*SourceNotFoundError) = false for %T", err)
	}
	if sourceErr.Source != "missing" {
		t.Fatalf("Source = %q, want missing", sourceErr.Source)
	}
	if len(sourceErr.AvailableSources) != 1 || sourceErr.AvailableSources[0] != "tools" {
		t.Fatalf("AvailableSources = %v, want [tools]", sourceErr.AvailableSources)
	}
	if classified.Kind != ErrorKindCommandNotFound {
		t.Fatalf("Kind = %q, want command-not-found", classified.Kind)
	}
}

func TestResolveDefinitionsAndLoadConfig(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	calls := 0
	service := &Service{
		config: &staticCommandsvcConfigProvider{cfg: cfg},
		configFallback: func(context.Context, config.Loader, string) (*config.Config, []discovery.Diagnostic) {
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
		hostAccess:      hostAccess,
		registryFactory: defaultRuntimeRegistryFactory{},
		interactive:     defaultInteractiveExecutor{},
		userEnvFunc:     func() map[string]string { return map[string]string{} },
		configFallback: func(context.Context, config.Loader, string) (*config.Config, []discovery.Diagnostic) {
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

func (p *staticCommandsvcConfigProvider) LoadWithSource(ctx context.Context, opts config.LoadOptions) (config.LoadResult, error) {
	cfg, err := p.Load(ctx, opts)
	if err != nil {
		return config.LoadResult{}, err
	}
	return config.LoadResult{Config: cfg}, nil
}
