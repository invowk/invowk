// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// mockChecker is a test double for the Checker interface.
	mockChecker struct {
		name     string
		category Category
		findings []Finding
		err      error
	}

	failingConfigProvider struct {
		called bool
	}
)

func (m *mockChecker) Name() string       { return m.name }
func (m *mockChecker) Category() Category { return m.category }
func (m *mockChecker) Check(_ context.Context, _ *ScanContext) ([]Finding, error) {
	return m.findings, m.err
}

func (p *failingConfigProvider) Load(_ context.Context, _ config.LoadOptions) (*config.Config, error) {
	p.called = true
	return nil, errors.New("unexpected config load")
}

func (p *failingConfigProvider) LoadWithSource(ctx context.Context, opts config.LoadOptions) (config.LoadResult, error) {
	cfg, err := p.Load(ctx, opts)
	if err != nil {
		return config.LoadResult{}, err
	}
	return config.LoadResult{Config: cfg}, nil
}

func TestScanner_RunCheckersCollectsFindings(t *testing.T) {
	t.Parallel()

	s := &Scanner{
		checkers: []Checker{
			&mockChecker{
				name:     "checker1",
				category: CategoryExecution,
				findings: []Finding{{Severity: SeverityHigh, Title: "finding1"}},
			},
			&mockChecker{
				name:     "checker2",
				category: CategoryTrust,
				findings: []Finding{{Severity: SeverityLow, Title: "finding2"}},
			},
		},
		correlator: mustNewCorrelator(t, nil),
	}

	sc := &ScanContext{
		invowkfiles: []*ScannedInvowkfile{{
			Path:       "test.cue",
			SurfaceID:  "test",
			Invowkfile: &invowkfile.Invowkfile{},
		}},
	}

	findings, errs := s.runCheckers(t.Context(), sc)
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(findings) != 2 {
		t.Errorf("findings len = %d, want 2", len(findings))
	}
}

func TestScanner_DirectCueTargetSkipsConfigLoad(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "invowkfile.cue")
	if err := os.WriteFile(target, []byte(`cmds: [{
	name: "build"
	implementations: [{
		script: "echo build"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}]
	}]
}]
`), 0o644); err != nil {
		t.Fatalf("write invowkfile: %v", err)
	}

	provider := &failingConfigProvider{}
	scanner := NewScanner(provider, WithCheckers(nil), WithCorrelator(nil))
	report, err := scanner.Scan(t.Context(), types.FilesystemPath(target), false)
	if err != nil {
		t.Fatalf("Scan() = %v", err)
	}
	if provider.called {
		t.Fatal("Scan() loaded config for an explicit invowkfile target")
	}
	if report.InvowkfileCount != 1 {
		t.Fatalf("InvowkfileCount = %d, want 1", report.InvowkfileCount)
	}
}

func TestEnsureFindingCodesFillsLegacyFindings(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{
			Severity:    SeverityCritical,
			Category:    CategoryExecution,
			CheckerName: networkCheckerName,
			Title:       "Reverse shell pattern detected",
		},
		{
			Code:        "explicit-code",
			Severity:    SeverityHigh,
			Category:    CategoryIntegrity,
			CheckerName: lockFileCheckerName,
			Title:       "Module content hash mismatch",
		},
	}

	ensureFindingCodes(findings)

	if findings[0].Code != "network-execution-reverse-shell-pattern-detected" {
		t.Fatalf("derived code = %q", findings[0].Code)
	}
	if findings[1].Code != "explicit-code" {
		t.Fatalf("explicit code overwritten: %q", findings[1].Code)
	}
}

func TestScanner_RunCheckersPartialOnError(t *testing.T) {
	t.Parallel()

	s := &Scanner{
		checkers: []Checker{
			&mockChecker{
				name:     "good",
				category: CategoryExecution,
				findings: []Finding{{Severity: SeverityHigh, Title: "good-finding"}},
			},
			&mockChecker{
				name: "bad",
				err:  errors.New("checker error"),
			},
		},
		correlator: mustNewCorrelator(t, nil),
	}

	sc := &ScanContext{
		invowkfiles: []*ScannedInvowkfile{{
			Path:       "test.cue",
			SurfaceID:  "test",
			Invowkfile: &invowkfile.Invowkfile{},
		}},
	}

	findings, errs := s.runCheckers(t.Context(), sc)
	if len(errs) != 1 {
		t.Errorf("errors len = %d, want 1", len(errs))
	}
	if !errors.Is(errs[0], ErrCheckerFailed) {
		t.Errorf("error should wrap ErrCheckerFailed, got %v", errs[0])
	}
	// Good checker's findings should still be present.
	if len(findings) != 1 {
		t.Errorf("findings len = %d, want 1 (partial results)", len(findings))
	}
}

func TestScanner_RunCheckersContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately.

	s := &Scanner{
		checkers: []Checker{
			&mockChecker{name: "never-runs", category: CategoryExecution},
		},
		correlator: mustNewCorrelator(t, nil),
	}

	sc := &ScanContext{}
	_, errs := s.runCheckers(ctx, sc)
	if len(errs) == 0 {
		t.Error("expected context cancellation error")
	}
}
