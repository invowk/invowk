// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
)

const (
	depsMutationCommand         invowkfile.CommandName     = "build"
	depsMutationSource          discovery.SourceID         = "tools"
	depsMutationSimple          invowkfile.CommandName     = "lint"
	depsMutationModuleID        invowkmod.ModuleID         = "io.example.tools"
	depsMutationCallerID        invowkmod.ModuleID         = "io.example.caller"
	depsMutationGitURL          invowkmod.GitURL           = "https://github.com/example/tools.git"
	depsMutationVersion         invowkmod.SemVerConstraint = "^1.0.0"
	depsMutationResolvedVersion                            = "1.2.3"
	depsMutationGitCommit                                  = "0123456789abcdef0123456789abcdef01234567"
	depsMutationContentHash                                = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type (
	depsMutationContextKey struct{}

	panicCommandSetProvider struct {
		t *testing.T
	}

	contextRecordingCommandSetProvider struct {
		want   context.Context
		called bool
	}

	recordingRuntimeProbe struct {
		tools         []invowkfile.BinaryName
		filepaths     []invowkfile.FilepathDependency
		envVars       []invowkfile.EnvVarCheck
		capabilities  []invowkfile.CapabilityName
		commands      []invowkfile.CommandName
		checks        []invowkfile.CustomCheck
		envErr        error
		toolErr       error
		filepathErr   error
		capabilityErr error
		checkErr      error
	}
)

func (p panicCommandSetProvider) DiscoverCommandSet(context.Context) (discovery.CommandSetResult, error) {
	p.t.Helper()
	p.t.Fatal("DiscoverCommandSet should not be called")
	return discovery.CommandSetResult{}, nil
}

func (p *contextRecordingCommandSetProvider) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	p.called = true
	if ctx != p.want {
		return discovery.CommandSetResult{}, errors.New("unexpected context")
	}
	return discovery.CommandSetResult{
		Set: &discovery.DiscoveredCommandSet{
			Commands: []*discovery.CommandInfo{{Name: depsMutationCommand}},
		},
	}, nil
}

func (p *recordingRuntimeProbe) CheckTool(tool invowkfile.BinaryName) error {
	p.tools = append(p.tools, tool)
	return p.toolErr
}

func (p *recordingRuntimeProbe) CheckFilepath(fp invowkfile.FilepathDependency) error {
	p.filepaths = append(p.filepaths, fp)
	return p.filepathErr
}

func (p *recordingRuntimeProbe) CheckEnvVar(envVar invowkfile.EnvVarCheck) error {
	p.envVars = append(p.envVars, envVar)
	return p.envErr
}

func (p *recordingRuntimeProbe) CheckCapability(capability invowkfile.CapabilityName) error {
	p.capabilities = append(p.capabilities, capability)
	return p.capabilityErr
}

func (p *recordingRuntimeProbe) CheckCommand(command invowkfile.CommandName) error {
	p.commands = append(p.commands, command)
	return nil
}

func (p *recordingRuntimeProbe) RunCustomCheck(check invowkfile.CustomCheck) (CustomCheckResult, error) {
	p.checks = append(p.checks, check)
	return CustomCheckResult{}, p.checkErr
}

func TestCommandResolutionMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "nil and empty command deps do not discover", run: testCommandResolutionEmptyDeps},
		{name: "resolved command records matched discovery name and original alternatives", run: testCommandResolutionMatchedCommand},
		{name: "missing and forbidden commands both report structured failures", run: testCommandResolutionStructuredFailures},
		{name: "discover uses execution context value", run: testDiscoverAvailableCommandsUsesContext},
		{name: "discovery failure preserves sentinel and cause", run: testDiscoverAvailableCommandsErrorWrap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestCommandScopeMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "lock provider fallback only loads for module paths", run: testCommandScopeLockFallback},
		{name: "direct requirement matching requires command identity source and lock", run: testCommandScopeDirectRequirementMatching},
		{name: "scope uses command info module override and every global source", run: testCommandScopeBuildsCompleteIdentity},
		{name: "accessible command reports allowed forbidden and root decisions", run: testCommandScopeAccessibleCommandDecisions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestCommandCandidateMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "source candidates de-duplicate prioritized lookups and fallback scan", run: testSourceCommandCandidates},
		{name: "prioritized lookups respect qualified and bare lookup order", run: testPrioritizedCommandLookups},
		{name: "source and simple-name helpers classify command identity", run: testCommandInfoIdentityHelpers},
		{name: "current command source falls back from source id to metadata", run: testCurrentCommandSourceIDFallback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testCommandResolutionEmptyDeps(t *testing.T) {
	t.Helper()

	resolved, err := resolveCommandDependenciesWithLockProvider(panicCommandSetProvider{t: t}, nil, nil, ExecutionContext{}, nil)
	if err != nil {
		t.Fatalf("nil deps error = %v, want nil", err)
	}
	if resolved != nil {
		t.Fatalf("nil deps resolved = %v, want nil", resolved)
	}

	resolved, err = resolveCommandDependenciesWithLockProvider(
		panicCommandSetProvider{t: t},
		&invowkfile.DependsOn{},
		nil,
		ExecutionContext{},
		nil,
	)
	if err != nil {
		t.Fatalf("empty deps error = %v, want nil", err)
	}
	if resolved != nil {
		t.Fatalf("empty deps resolved = %v, want nil", resolved)
	}
}

func testCommandResolutionMatchedCommand(t *testing.T) {
	t.Helper()

	available := &discovery.DiscoveredCommandSet{
		Commands: []*discovery.CommandInfo{{Name: depsMutationCommand}},
	}
	resolved, err := resolveCommandDependenciesWithLockProvider(
		&stubCommandSetProvider{result: discovery.CommandSetResult{Set: available}},
		&invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{
			Alternatives: []invowkfile.CommandDependencyRef{invowkfile.CommandDependencyRef(depsMutationCommand)},
		}}},
		&discovery.CommandInfo{Invowkfile: &invowkfile.Invowkfile{}},
		ExecutionContext{CommandName: depsMutationCommand},
		nil,
	)
	if err != nil {
		t.Fatalf("resolveCommandDependenciesWithLockProvider() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("resolved length = %d, want 1", len(resolved))
	}
	if resolved[0].Command == nil || *resolved[0].Command != depsMutationCommand {
		t.Fatalf("resolved command = %v, want %q", resolved[0].Command, depsMutationCommand)
	}
	if len(resolved[0].Alternatives) != 1 || resolved[0].Alternatives[0] != invowkfile.CommandDependencyRef(depsMutationCommand) {
		t.Fatalf("resolved alternatives = %v, want original ref", resolved[0].Alternatives)
	}
}

func testCommandResolutionStructuredFailures(t *testing.T) {
	t.Helper()

	blockedID := invowkmod.ModuleID("io.example.blocked")
	cmd := depsMutationHostCommand(&invowkfile.DependsOn{
		Commands: []invowkfile.CommandDependency{
			{Alternatives: []invowkfile.CommandDependencyRef{"missing"}},
			{Alternatives: []invowkfile.CommandDependencyRef{"@blocked lint"}},
		},
	})
	callerMeta := mustModuleMetadata(t, &invowkfile.Invowkmod{
		Module:  depsMutationCallerID,
		Version: "1.0.0",
	})
	callerInfo := depsMutationCommandInfo(cmd, &invowkfile.Invowkfile{Metadata: callerMeta})
	callerInfo.SourceID = "caller"
	available := &discovery.DiscoveredCommandSet{
		Commands: []*discovery.CommandInfo{{
			Name:       "blocked lint",
			SimpleName: depsMutationSimple,
			SourceID:   "blocked",
			ModuleID:   &blockedID,
		}},
	}

	err := CheckCommandDependenciesExistWithLockProvider(
		&stubCommandSetProvider{result: discovery.CommandSetResult{Set: available}},
		cmd.DependsOn,
		callerInfo,
		testDependencyExecutionContext(t, cmd, invowkfile.RuntimeNative),
		nil,
	)
	depErr := requireDependencyError(t, err)
	if len(depErr.MissingCommands) != 1 || len(depErr.ForbiddenCommands) != 1 {
		t.Fatalf("missing=%v forbidden=%v, want one of each", depErr.MissingCommands, depErr.ForbiddenCommands)
	}
	requireDependencyFailureKinds(t, depErr.Failures(), DependencyFailureCommand, DependencyFailureForbiddenCommand)
}

func testDiscoverAvailableCommandsUsesContext(t *testing.T) {
	t.Helper()

	wantCtx := context.WithValue(t.Context(), depsMutationContextKey{}, "sentinel")
	provider := &contextRecordingCommandSetProvider{want: wantCtx}
	available, err := discoverAvailableCommands(provider, ExecutionContext{Context: wantCtx})
	if err != nil {
		t.Fatalf("discoverAvailableCommands() error = %v", err)
	}
	if !provider.called {
		t.Fatal("provider was not called")
	}
	if available[depsMutationCommand] == nil {
		t.Fatalf("available commands = %v, want %q", available, depsMutationCommand)
	}
}

func testDiscoverAvailableCommandsErrorWrap(t *testing.T) {
	t.Helper()

	cause := errors.New("discovery failed")
	_, err := discoverAvailableCommands(&stubCommandSetProvider{err: cause}, ExecutionContext{Context: t.Context()})
	if !errors.Is(err, ErrDependencyDiscoveryFailed) {
		t.Fatalf("discoverAvailableCommands() error = %v, want ErrDependencyDiscoveryFailed", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("discoverAvailableCommands() error = %v, want cause %v", err, cause)
	}
}

func testCommandScopeLockFallback(t *testing.T) {
	t.Helper()

	providerErr := &CommandScopeLockError{Path: "invowkmod.lock.cue", Err: errors.New("corrupt")}
	provider := staticCommandScopeLockProvider{err: providerErr}

	lock, err := commandScopeLock(provider, nil)
	if err != nil {
		t.Fatalf("nil inv lock error = %v, want nil", err)
	}
	if lock == nil {
		t.Fatal("nil inv lock = nil, want empty lock")
	}

	lock, err = commandScopeLock(provider, &invowkfile.Invowkfile{})
	if err != nil {
		t.Fatalf("empty module path lock error = %v, want nil", err)
	}
	if lock == nil {
		t.Fatal("empty module path lock = nil, want empty lock")
	}

	lock, err = commandScopeLock(nil, &invowkfile.Invowkfile{ModulePath: "module.invowkmod"})
	if err != nil {
		t.Fatalf("nil provider with module path error = %v, want nil", err)
	}
	if lock == nil {
		t.Fatal("nil provider with module path lock = nil, want empty lock")
	}

	_, err = commandScopeLock(provider, &invowkfile.Invowkfile{ModulePath: "module.invowkmod"})
	if !errors.Is(err, ErrCommandScopeLockLoadFailed) {
		t.Fatalf("module path lock error = %v, want ErrCommandScopeLockLoadFailed", err)
	}
}

func testCommandScopeDirectRequirementMatching(t *testing.T) {
	t.Helper()

	req, lock := depsMutationRequirementAndLock()
	if !invowkmod.IsDeclaredLockedCommandSource(
		[]invowkmod.ModuleRequirement{req},
		lock,
		depsMutationModuleID,
		invowkmod.ModuleSourceID(depsMutationSource),
	) {
		t.Fatal("IsDeclaredLockedCommandSource() = false, want true for matching lock identity")
	}
	if invowkmod.IsDeclaredLockedCommandSource(
		[]invowkmod.ModuleRequirement{req},
		nil,
		depsMutationModuleID,
		invowkmod.ModuleSourceID(depsMutationSource),
	) {
		t.Fatal("IsDeclaredLockedCommandSource() = true with nil lock")
	}
	if invowkmod.IsDeclaredLockedCommandSource(
		[]invowkmod.ModuleRequirement{req},
		lock,
		"",
		invowkmod.ModuleSourceID(depsMutationSource),
	) {
		t.Fatal("IsDeclaredLockedCommandSource() = true with empty module ID")
	}
	if invowkmod.IsDeclaredLockedCommandSource(
		[]invowkmod.ModuleRequirement{req},
		lock,
		depsMutationModuleID,
		"",
	) {
		t.Fatal("IsDeclaredLockedCommandSource() = true with empty source ID")
	}
	otherID := invowkmod.ModuleID("io.example.other")
	if invowkmod.IsDeclaredLockedCommandSource(
		[]invowkmod.ModuleRequirement{req},
		lock,
		otherID,
		invowkmod.ModuleSourceID(depsMutationSource),
	) {
		t.Fatal("IsDeclaredLockedCommandSource() = true for mismatched module ID")
	}
}

func testCommandScopeBuildsCompleteIdentity(t *testing.T) {
	t.Helper()

	metadataID := invowkmod.ModuleID("io.example.metadata")
	overrideID := invowkmod.ModuleID("io.example.override")
	globalOneID := invowkmod.ModuleID("io.example.globalone")
	globalTwoID := invowkmod.ModuleID("io.example.globaltwo")
	meta := mustModuleMetadata(t, &invowkfile.Invowkmod{Module: metadataID, Version: "1.0.0"})
	cmdInfo := &discovery.CommandInfo{
		SourceID:   "caller",
		ModuleID:   &overrideID,
		Invowkfile: &invowkfile.Invowkfile{Metadata: meta},
	}
	globalOne := scopedCommandInfo("global-one lint", "global-one", &globalOneID)
	globalOne.IsGlobalModule = true
	globalTwo := scopedCommandInfo("global-two lint", "global-two", &globalTwoID)
	globalTwo.IsGlobalModule = true
	available := map[invowkfile.CommandName]*discovery.CommandInfo{
		globalOne.Name: globalOne,
		globalTwo.Name: globalTwo,
	}

	scope := buildCommandScope(cmdInfo, available, &invowkmod.LockFile{})
	if scope == nil {
		t.Fatal("buildCommandScope() = nil, want module scope")
	}
	if scope.ModuleID != overrideID {
		t.Fatalf("scope.ModuleID = %q, want command info override %q", scope.ModuleID, overrideID)
	}
	requireAccessibleCommandDecision(t, available, scope, "@global-two lint", globalTwo, 0)
}

func testCommandScopeAccessibleCommandDecisions(t *testing.T) {
	t.Helper()

	allowedID := depsMutationModuleID
	blockedID := invowkmod.ModuleID("io.example.blocked")
	allowed := scopedCommandInfo("tools lint", depsMutationSource, &allowedID)
	blocked := scopedCommandInfo("blocked lint", "blocked", &blockedID)
	available := map[invowkfile.CommandName]*discovery.CommandInfo{
		allowed.Name: allowed,
		blocked.Name: blocked,
	}
	scope := invowkmod.NewCommandScope(depsMutationCallerID)
	scope.AddDirectDependency(depsMutationModuleID, invowkmod.ModuleSourceID(depsMutationSource))

	requireAccessibleCommandDecision(t, available, scope, "@tools lint", allowed, 0)
	matched, forbidden, found := findAccessibleCommand(available, "", commandDependencyAlternativesForTest(t, "@blocked lint"), scope)
	if !found || matched != nil || len(forbidden) != 1 {
		t.Fatalf("blocked lookup matched=%v forbidden=%v found=%v, want one forbidden", matched, forbidden, found)
	}
	if !strings.Contains(forbidden[0].String(), "module 'blocked' is not accessible") {
		t.Fatalf("forbidden detail = %q, want inaccessible blocked module", forbidden[0])
	}

	decision := commandScopeDecision(nil, &discovery.CommandInfo{Name: depsMutationCommand})
	if !decision.Allowed || decision.TargetCommand != invowkmod.CommandReference(depsMutationCommand) {
		t.Fatalf("root decision = %+v, want allowed target command", decision)
	}
}

func testSourceCommandCandidates(t *testing.T) {
	t.Helper()

	shared := &discovery.CommandInfo{Name: "tools lint", SimpleName: depsMutationSimple, SourceID: depsMutationSource}
	fallback := &discovery.CommandInfo{Name: "not-indexed", SimpleName: depsMutationSimple, SourceID: depsMutationSource}
	available := map[invowkfile.CommandName]*discovery.CommandInfo{
		"tools lint": shared,
		"lint":       shared,
		"fallback":   fallback,
	}

	got := sourceCommandCandidates(available, invowkmod.ModuleSourceID(depsMutationSource), depsMutationSimple)
	if len(got) != 2 || got[0] != shared || got[1] != fallback {
		t.Fatalf("sourceCommandCandidates() = %v, want shared then fallback", got)
	}
}

func testPrioritizedCommandLookups(t *testing.T) {
	t.Helper()

	qualified := &discovery.CommandInfo{Name: "tools lint", SimpleName: depsMutationSimple, SourceID: depsMutationSource}
	bare := &discovery.CommandInfo{Name: depsMutationSimple}
	available := map[invowkfile.CommandName]*discovery.CommandInfo{
		"tools lint":       qualified,
		depsMutationSimple: bare,
	}

	got := prioritizedCommandLookups(available, invowkmod.ModuleSourceID(depsMutationSource), depsMutationSimple)
	if len(got) != 2 || got[0] != qualified || got[1] != bare {
		t.Fatalf("prioritizedCommandLookups() = %v, want qualified then bare", got)
	}

	got = prioritizedCommandLookups(available, "", depsMutationSimple)
	if len(got) != 1 || got[0] != bare {
		t.Fatalf("prioritizedCommandLookups() without source = %v, want bare only", got)
	}
}

func testCommandInfoIdentityHelpers(t *testing.T) {
	t.Helper()

	if commandInfoSourceID(nil) != "" {
		t.Fatal("commandInfoSourceID(nil) should be empty")
	}
	if commandInfoSourceID(&discovery.CommandInfo{}) != "" {
		t.Fatal("commandInfoSourceID(empty) should be empty")
	}
	cmd := &discovery.CommandInfo{Name: "tools lint", SourceID: depsMutationSource}
	if commandInfoSourceID(cmd) != invowkmod.ModuleSourceID(depsMutationSource) {
		t.Fatalf("commandInfoSourceID() = %q, want %q", commandInfoSourceID(cmd), depsMutationSource)
	}
	if commandInfoSimpleName(nil, invowkmod.ModuleSourceID(depsMutationSource)) != "" {
		t.Fatal("commandInfoSimpleName(nil) should be empty")
	}
	if got := commandInfoSimpleName(&discovery.CommandInfo{Name: "tools lint", SimpleName: "fmt"}, invowkmod.ModuleSourceID(depsMutationSource)); got != "fmt" {
		t.Fatalf("explicit SimpleName = %q, want fmt", got)
	}
	if got := commandInfoSimpleName(cmd, invowkmod.ModuleSourceID(depsMutationSource)); got != depsMutationSimple {
		t.Fatalf("derived SimpleName = %q, want %q", got, depsMutationSimple)
	}
	if got := commandInfoSimpleName(cmd, "other"); got != cmd.Name {
		t.Fatalf("nonmatching prefix SimpleName = %q, want full command name", got)
	}
	if got := commandInfoSimpleName(&discovery.CommandInfo{Name: " lint"}, ""); got != " lint" {
		t.Fatalf("empty source SimpleName = %q, want leading-space command name", got)
	}
	if commandMatchesSourceAndName(cmd, "other", depsMutationSimple) {
		t.Fatal("commandMatchesSourceAndName() = true for mismatched explicit source")
	}
	if commandMatchesSourceAndName(cmd, "", depsMutationSimple) {
		t.Fatal("commandMatchesSourceAndName() = true for sourced command with empty source")
	}
}

func testCurrentCommandSourceIDFallback(t *testing.T) {
	t.Helper()

	if currentCommandSourceID(nil) != "" {
		t.Fatal("currentCommandSourceID(nil) should be empty")
	}
	if got := currentCommandSourceID(&discovery.CommandInfo{SourceID: depsMutationSource}); got != invowkmod.ModuleSourceID(depsMutationSource) {
		t.Fatalf("current explicit source = %q, want %q", got, depsMutationSource)
	}
	if currentCommandSourceID(&discovery.CommandInfo{}) != "" {
		t.Fatal("currentCommandSourceID(empty command info) should be empty")
	}
	meta := mustModuleMetadata(t, &invowkfile.Invowkmod{Module: depsMutationModuleID, Version: "1.0.0"})
	if got := currentCommandSourceID(&discovery.CommandInfo{Invowkfile: &invowkfile.Invowkfile{Metadata: meta}}); got != invowkmod.ModuleSourceID(depsMutationModuleID) {
		t.Fatalf("current metadata source = %q, want %q", got, depsMutationModuleID)
	}
}

func scopedCommandInfo(name invowkfile.CommandName, source discovery.SourceID, moduleID *invowkmod.ModuleID) *discovery.CommandInfo {
	return &discovery.CommandInfo{
		Name:       name,
		SimpleName: depsMutationSimple,
		SourceID:   source,
		ModuleID:   moduleID,
	}
}

func requireAccessibleCommandDecision(
	t *testing.T,
	available map[invowkfile.CommandName]*discovery.CommandInfo,
	scope *invowkmod.CommandScope,
	ref invowkfile.CommandDependencyRef,
	want *discovery.CommandInfo,
	wantForbidden int,
) {
	t.Helper()

	matched, forbidden, found := findAccessibleCommand(available, "", commandDependencyAlternativesForTest(t, ref), scope)
	if !found || matched != want || len(forbidden) != wantForbidden {
		t.Fatalf("lookup %q matched=%v forbidden=%v found=%v, want matched=%v forbidden=%d", ref, matched, forbidden, found, want, wantForbidden)
	}
}

func TestMissingCommandMessageMutationContracts(t *testing.T) {
	t.Parallel()

	available := map[invowkfile.CommandName]*discovery.CommandInfo{
		"tools fmt": {
			Name:       "tools fmt",
			SimpleName: "fmt",
			SourceID:   depsMutationSource,
		},
	}

	tests := []struct {
		name        string
		alts        []commandDependencyAlternative
		current     invowkmod.ModuleSourceID
		inContainer bool
		want        string
	}{
		{
			name: "unknown qualified source",
			alts: commandDependencyAlternativesForTest(t, "@missing lint"),
			want: `@missing lint - source "missing" not found`,
		},
		{
			name:        "missing command in known source in container",
			alts:        commandDependencyAlternativesForTest(t, "@tools lint"),
			inContainer: true,
			want:        `@tools lint - command "lint" not found in source "tools" in container`,
		},
		{
			name:    "bare command missing in current source",
			alts:    commandDependencyAlternativesForTest(t, "lint"),
			current: invowkmod.ModuleSourceID(depsMutationSource),
			want:    `lint - command not found in source "tools"`,
		},
		{
			name:        "multiple alternatives in container",
			alts:        commandDependencyAlternativesForTest(t, "lint", "@tools fmt"),
			inContainer: true,
			want:        "none of [lint, @tools fmt] found in container",
		},
		{
			name: "multiple alternatives are not rendered as a qualified single miss",
			alts: commandDependencyAlternativesForTest(t, "@missing lint", "build"),
			want: "none of [@missing lint, build] found",
		},
		{
			name:    "multiple alternatives are not rendered as a current source single miss",
			alts:    commandDependencyAlternativesForTest(t, "lint", "build"),
			current: invowkmod.ModuleSourceID(depsMutationSource),
			want:    "none of [lint, build] found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatMissingDiscoveredCommandDependency(available, tt.current, tt.alts, tt.inContainer)
			if got.String() != tt.want {
				t.Fatalf("formatMissingDiscoveredCommandDependency() = %q, want %q", got, tt.want)
			}
		})
	}
}
