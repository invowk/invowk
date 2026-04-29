// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// mockChecker is a test double for the Checker interface.
type mockChecker struct {
	name     string
	category Category
	findings []Finding
	err      error
}

func (m *mockChecker) Name() string       { return m.name }
func (m *mockChecker) Category() Category { return m.category }
func (m *mockChecker) Check(_ context.Context, _ *ScanContext) ([]Finding, error) {
	return m.findings, m.err
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
