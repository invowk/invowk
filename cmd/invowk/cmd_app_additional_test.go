// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	errorConfigProvider struct {
		err error
	}

	stubDiscoveryService struct {
		commandSet      discovery.CommandSetResult
		commandSetErr   error
		validatedSet    discovery.CommandSetResult
		validatedSetErr error
		modules         discovery.ModuleListResult
		modulesErr      error
		lookup          discovery.LookupResult
		lookupErr       error
	}
)

func (p *errorConfigProvider) Load(context.Context, config.LoadOptions) (*config.Config, error) {
	return nil, p.err
}

func (p *errorConfigProvider) LoadWithSource(ctx context.Context, opts config.LoadOptions) (config.LoadResult, error) {
	cfg, err := p.Load(ctx, opts)
	if err != nil {
		return config.LoadResult{}, err
	}
	return config.LoadResult{Config: cfg}, nil
}

func (s *stubDiscoveryService) DiscoverCommandSet(context.Context) (discovery.CommandSetResult, error) {
	return s.commandSet, s.commandSetErr
}

func (s *stubDiscoveryService) DiscoverAndValidateCommandSet(context.Context) (discovery.CommandSetResult, error) {
	return s.validatedSet, s.validatedSetErr
}

func (s *stubDiscoveryService) DiscoverModules(context.Context) (discovery.ModuleListResult, error) {
	return s.modules, s.modulesErr
}

func (s *stubDiscoveryService) GetCommand(context.Context, string) (discovery.LookupResult, error) {
	return s.lookup, s.lookupErr
}

func TestCLICommandAdapterExecute_DryRun(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app, err := NewApp(Dependencies{
		Config:    &staticConfigProvider{cfg: config.DefaultConfig()},
		Discovery: &stubDiscoveryService{lookup: discovery.LookupResult{Command: testCommandInfo(t, "build", "echo hello")}},
		Stdout:    &stdout,
		Stderr:    &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	result, diags, err := app.Commands.Execute(t.Context(), ExecuteRequest{
		Name:   "build",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Commands.Execute() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("result.ExitCode = %d, want 0", result.ExitCode)
	}
	if len(diags) != 0 {
		t.Fatalf("len(diags) = %d, want 0", len(diags))
	}

	out := stdout.String()
	for _, token := range []string{"Dry Run", "Command:", "build", "Runtime:", "virtual"} {
		if !strings.Contains(out, token) {
			t.Fatalf("dry-run output missing %q:\n%s", token, out)
		}
	}
}

func TestCLICommandAdapterExecute_CommandNotFoundWrapsServiceError(t *testing.T) {
	t.Parallel()

	app, err := NewApp(Dependencies{
		Config:    &staticConfigProvider{cfg: config.DefaultConfig()},
		Discovery: &stubDiscoveryService{lookup: discovery.LookupResult{}},
		Stdout:    &bytes.Buffer{},
		Stderr:    &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	_, _, execErr := app.Commands.Execute(t.Context(), ExecuteRequest{Name: "missing"})
	if execErr == nil {
		t.Fatal("expected error for missing command")
	}

	var svcErr *ServiceError
	if !errors.As(execErr, &svcErr) {
		t.Fatalf("errors.As(*ServiceError) = false for %T", execErr)
	}
	if svcErr.IssueID != issue.CommandNotFoundId {
		t.Fatalf("svcErr.IssueID = %v, want %v", svcErr.IssueID, issue.CommandNotFoundId)
	}
}

func TestLoadConfigWithFallback(t *testing.T) {
	t.Parallel()

	customPath := filepath.Join(t.TempDir(), "custom.cue")
	explicitCfg, explicitDiags := commandsvc.LoadConfigWithFallback(
		t.Context(),
		&errorConfigProvider{err: errors.New("parse failure")},
		customPath,
	)
	if explicitCfg == nil {
		t.Fatal("explicitCfg = nil")
	}
	if len(explicitDiags) != 1 {
		t.Fatalf("len(explicitDiags) = %d, want 1", len(explicitDiags))
	}
	if explicitDiags[0].Severity() != commandsvc.DiagnosticSeverityError {
		t.Fatalf("explicit severity = %s, want error", explicitDiags[0].Severity())
	}
	if string(explicitDiags[0].Path()) != customPath {
		t.Fatalf("explicit path = %q, want %q", explicitDiags[0].Path(), customPath)
	}

	defaultCfg, defaultDiags := commandsvc.LoadConfigWithFallback(
		t.Context(),
		&errorConfigProvider{err: errors.New("syntax error")},
		"",
	)
	if defaultCfg == nil {
		t.Fatal("defaultCfg = nil")
	}
	if len(defaultDiags) != 1 || defaultDiags[0].Severity() != commandsvc.DiagnosticSeverityError {
		t.Fatalf("defaultDiags = %v", defaultDiags)
	}

	warnCfg, warnDiags := commandsvc.LoadConfigWithFallback(
		t.Context(),
		&errorConfigProvider{err: errors.New("wrapped: " + types.FilesystemPath("").String())},
		"",
	)
	if warnCfg == nil {
		t.Fatal("warnCfg = nil")
	}
	_ = warnDiags

	notExistCfg, notExistDiags := commandsvc.LoadConfigWithFallback(
		t.Context(),
		&errorConfigProvider{err: types.ErrUserCancelled},
		"",
	)
	if notExistCfg == nil {
		t.Fatal("notExistCfg = nil")
	}
	_ = notExistDiags
}

func TestLoadConfigWithFallback_NotExistWarning(t *testing.T) {
	t.Parallel()

	cfg, diags := commandsvc.LoadConfigWithFallback(t.Context(), &errorConfigProvider{err: errors.New("wrapper: " + context.Canceled.Error())}, "")
	if cfg == nil {
		t.Fatal("cfg = nil")
	}
	_ = diags

	cfg, diags = commandsvc.LoadConfigWithFallback(t.Context(), &errorConfigProvider{err: errors.New("wrapper")}, "")
	if cfg == nil {
		t.Fatal("cfg = nil")
	}
	if len(diags) != 1 {
		t.Fatalf("len(diags) = %d, want 1", len(diags))
	}
}

func TestLoadConfigWithFallback_DefaultNotExistWarning(t *testing.T) {
	t.Parallel()

	cfg, diags := commandsvc.LoadConfigWithFallback(t.Context(), &errorConfigProvider{err: context.Canceled}, "")
	if cfg == nil {
		t.Fatal("cfg = nil")
	}
	if len(diags) != 1 {
		t.Fatalf("len(diags) = %d, want 1", len(diags))
	}
}

func TestContextHelpersAndCacheValidation(t *testing.T) {
	t.Parallel()

	invowkPath := filepath.Join(t.TempDir(), "invowk.cue")
	ctx := contextWithConfigPath(t.Context(), invowkPath)
	if got := configPathFromContext(ctx); got != invowkPath {
		t.Fatalf("configPathFromContext() = %q, want %q", got, invowkPath)
	}
	if contextWithDiscoveryRequestCache(ctx) != ctx {
		t.Fatal("contextWithDiscoveryRequestCache() should reuse existing cache")
	}
}

func TestLookupFromCommandSetAndDiagnosticRenderer(t *testing.T) {
	t.Parallel()

	set := discovery.NewDiscoveredCommandSet()
	cmdInfo := testCommandInfo(t, "build", "echo hello")
	set.Add(cmdInfo)

	result, err := lookupFromCommandSet(discovery.CommandSetResult{Set: set}, "build")
	if err != nil {
		t.Fatalf("lookupFromCommandSet(build) error = %v", err)
	}
	if result.Command != cmdInfo {
		t.Fatal("lookupFromCommandSet(build) returned wrong command")
	}

	missing, err := lookupFromCommandSet(discovery.CommandSetResult{Set: set}, "missing")
	if err != nil {
		t.Fatalf("lookupFromCommandSet(missing) error = %v", err)
	}
	if missing.Command != nil || len(missing.Diagnostics) != 1 {
		t.Fatalf("missing lookup = %#v", missing)
	}

	if _, lookupErr := lookupFromCommandSet(discovery.CommandSetResult{Set: set}, "   "); lookupErr == nil {
		t.Fatal("expected invalid command name error")
	}

	warnDiag, err := discovery.NewDiagnostic(discovery.SeverityWarning, discovery.CodeConfigLoadFailed, "warn")
	if err != nil {
		t.Fatalf("NewDiagnostic(warn): %v", err)
	}
	diagPath := types.FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue"))
	pathDiag, err := discovery.NewDiagnosticWithPath(discovery.SeverityError, discovery.CodeCommandNotFound, "missing", diagPath)
	if err != nil {
		t.Fatalf("NewDiagnosticWithPath(error): %v", err)
	}

	var stderr bytes.Buffer
	(&defaultDiagnosticRenderer{}).Render(t.Context(), []discovery.Diagnostic{warnDiag, pathDiag}, &stderr)
	rendered := stderr.String()
	for _, token := range []string{"warn", "missing", string(diagPath)} {
		if !strings.Contains(rendered, token) {
			t.Fatalf("rendered diagnostics missing %q:\n%s", token, rendered)
		}
	}
}

func TestRunModuleListRendersDiscoveryDiagnostics(t *testing.T) {
	t.Parallel()

	diag := testMustDiagnostic(t, discovery.SeverityWarning, discovery.CodeModuleLoadSkipped, "bad module skipped")
	var stderr bytes.Buffer
	app, err := NewApp(Dependencies{
		Config: &staticConfigProvider{cfg: config.DefaultConfig()},
		Discovery: &stubDiscoveryService{
			modules: discovery.ModuleListResult{Diagnostics: []discovery.Diagnostic{diag}},
		},
		Diagnostics: &defaultDiagnosticRenderer{},
		Stderr:      &stderr,
	})
	if err != nil {
		t.Fatalf("NewApp() unexpected error: %v", err)
	}

	if err := runModuleList(t.Context(), app); err != nil {
		t.Fatalf("runModuleList() error = %v", err)
	}
	if !strings.Contains(stderr.String(), "bad module skipped") {
		t.Fatalf("stderr = %q, want discovery diagnostic", stderr.String())
	}
}

func TestRenderAndWrapServiceError(t *testing.T) {
	t.Parallel()

	depErr := &deps.DependencyError{CommandName: "build"}
	wrapped := renderAndWrapServiceError(depErr, ExecuteRequest{Name: "build"})
	var svcErr *ServiceError
	if !errors.As(wrapped, &svcErr) {
		t.Fatalf("dep branch did not return ServiceError: %T", wrapped)
	}
	if svcErr.IssueID != issue.DependenciesNotSatisfiedId {
		t.Fatalf("dep IssueID = %v", svcErr.IssueID)
	}

	argErr := &deps.ArgumentValidationError{Type: deps.ArgErrMissingRequired, CommandName: "build", MinArgs: 1}
	wrapped = renderAndWrapServiceError(argErr, ExecuteRequest{Name: "build"})
	if !errors.As(wrapped, &svcErr) || svcErr.IssueID != issue.InvalidArgumentId {
		t.Fatalf("arg branch returned %T issue %v", wrapped, svcErr.IssueID)
	}

	flagErr := &deps.FlagValidationError{
		CommandName: "build",
		Failures:    []deps.DependencyMessage{"required flag '--name' was not provided"},
	}
	wrapped = renderAndWrapServiceError(flagErr, ExecuteRequest{Name: "build"})
	if !errors.As(wrapped, &svcErr) || svcErr.IssueID != issue.InvalidArgumentId || !strings.Contains(svcErr.StyledMessage, "Invalid flag") {
		t.Fatalf("flag branch returned %#v", svcErr)
	}

	platformErr := &commandsvc.UnsupportedPlatformError{
		CommandName: "build",
		Current:     invowkfile.PlatformLinux,
		Supported:   []invowkfile.Platform{invowkfile.PlatformMac},
	}
	wrapped = renderAndWrapServiceError(platformErr, ExecuteRequest{Name: "build"})
	if !errors.As(wrapped, &svcErr) || svcErr.IssueID != issue.HostNotSupportedId || !strings.Contains(svcErr.StyledMessage, "Host not supported") {
		t.Fatalf("platform branch returned %#v", svcErr)
	}

	runtimeErr := &commandsvc.RuntimeNotAllowedError{
		CommandName: "build",
		Runtime:     invowkfile.RuntimeContainer,
		Platform:    invowkfile.PlatformLinux,
		Allowed:     []invowkfile.RuntimeMode{invowkfile.RuntimeVirtual},
	}
	wrapped = renderAndWrapServiceError(runtimeErr, ExecuteRequest{Name: "build", Runtime: invowkfile.RuntimeContainer})
	if !errors.As(wrapped, &svcErr) || svcErr.IssueID != issue.InvalidRuntimeModeId {
		t.Fatalf("runtime branch returned %T issue %v", wrapped, svcErr.IssueID)
	}

	classified := &commandsvc.ClassifiedError{
		Err:     context.DeadlineExceeded,
		Kind:    commandsvc.ErrorKindScriptExecutionFailed,
		Message: commandsvc.HintTimedOut,
	}
	wrapped = renderAndWrapServiceError(classified, ExecuteRequest{Name: "build"})
	if !errors.As(wrapped, &svcErr) || !strings.Contains(svcErr.StyledMessage, "timed out") {
		t.Fatalf("classified branch returned %#v", svcErr)
	}

	sourceErr := &commandsvc.ClassifiedError{
		Err: &commandsvc.SourceNotFoundError{
			Source:           "missing",
			AvailableSources: []discovery.SourceID{"invowkfile", "tools"},
		},
		Kind: commandsvc.ErrorKindCommandNotFound,
	}
	wrapped = renderAndWrapServiceError(sourceErr, ExecuteRequest{Name: "build"})
	if !errors.As(wrapped, &svcErr) || svcErr.IssueID != issue.CommandNotFoundId || !strings.Contains(svcErr.StyledMessage, "Available sources") {
		t.Fatalf("source-not-found branch returned %#v", svcErr)
	}

	plain := errors.New("plain error")
	got := renderAndWrapServiceError(plain, ExecuteRequest{})
	if !errors.Is(got, plain) {
		t.Fatal("plain error should pass through unchanged")
	}
	if errors.As(got, &svcErr) {
		t.Fatal("plain error should not be wrapped as ServiceError")
	}
}

func testCommandInfo(t *testing.T, name, script string) *discovery.CommandInfo {
	t.Helper()
	cmd := &invowkfile.Command{
		Name:        invowkfile.CommandName(name),
		Description: "test command",
		Implementations: []invowkfile.Implementation{{
			Script:    invowkfile.ScriptContent(script),
			Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
			Platforms: invowkfile.AllPlatformConfigs(),
		}},
	}
	inv := &invowkfile.Invowkfile{FilePath: types.FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue"))}
	return &discovery.CommandInfo{
		Name:        cmd.Name,
		Description: cmd.Description,
		Source:      discovery.SourceCurrentDir,
		FilePath:    inv.FilePath,
		Command:     cmd,
		Invowkfile:  inv,
		SimpleName:  cmd.Name,
		SourceID:    discovery.SourceIDInvowkfile,
	}
}
