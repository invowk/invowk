// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// ScannerOption configures a Scanner instance via the functional options pattern.
	ScannerOption func(*Scanner)

	// Scanner orchestrates security analysis by building a ScanContext from the
	// target path, running all registered Checkers concurrently, and applying the
	// Correlator for compound threat detection.
	Scanner struct {
		checkers   []Checker
		correlator *Correlator
		config     config.Provider
	}
)

// NewScanner creates a Scanner with default checkers and correlator.
// Use options to customize which checkers run or to inject a custom correlator.
func NewScanner(cfg config.Provider, opts ...ScannerOption) *Scanner {
	s := &Scanner{
		checkers:   DefaultCheckers(),
		correlator: NewCorrelator(DefaultRules()),
		config:     cfg,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithChecker appends a checker to the scanner's default set.
// To replace all checkers, use WithCheckers.
func WithChecker(c Checker) ScannerOption {
	return func(s *Scanner) {
		s.checkers = append(s.checkers, c)
	}
}

// WithCheckers replaces all checkers with the provided set.
func WithCheckers(checkers []Checker) ScannerOption {
	return func(s *Scanner) {
		s.checkers = checkers
	}
}

// WithCorrelator replaces the default correlator.
func WithCorrelator(c *Correlator) ScannerOption {
	return func(s *Scanner) {
		s.correlator = c
	}
}

// Scan performs a full security analysis of the target path.
//
// Flow:
//  1. Load config for discovery
//  2. Build immutable ScanContext from the target path
//  3. Run all checkers concurrently with context cancellation
//  4. Apply correlator for compound threat detection
//  5. Return assembled Report
func (s *Scanner) Scan(ctx context.Context, path types.FilesystemPath, includeGlobal bool) (*Report, error) {
	start := time.Now()

	// Load config for discovery.
	cfg, err := s.config.Load(ctx, config.LoadOptions{})
	if err != nil {
		return nil, &ScanContextBuildError{
			Path: path,
			Err:  fmt.Errorf("loading config: %w", err),
		}
	}

	// Build immutable scan context.
	sc, err := BuildScanContext(path, cfg, includeGlobal)
	if err != nil {
		return nil, err
	}

	// Run checkers concurrently.
	findings, checkerErrors := s.runCheckers(ctx, sc)

	// Apply correlation.
	correlated := s.correlator.Correlate(findings)

	report := &Report{
		Findings:        findings,
		Correlated:      correlated,
		ScanDuration:    time.Since(start),
		ModuleCount:     len(sc.modules),
		InvowkfileCount: len(sc.invowkfiles),
		ScriptCount:     sc.ScriptCount(),
	}

	// If checkers failed, return partial results with the combined error.
	if len(checkerErrors) > 0 {
		return report, errors.Join(checkerErrors...)
	}

	return report, nil
}

// runCheckers dispatches all checkers concurrently and collects findings.
// A failing checker does not cancel other checkers — partial results are returned.
func (s *Scanner) runCheckers(ctx context.Context, sc *ScanContext) ([]Finding, []error) {
	type result struct {
		findings []Finding
		err      error
	}

	results := make([]result, len(s.checkers))
	var wg sync.WaitGroup

	for i, checker := range s.checkers {
		wg.Add(1)
		go func(idx int, c Checker) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				results[idx] = result{err: fmt.Errorf("checker cancelled: %w", ctx.Err())}
				return
			default:
			}

			findings, err := c.Check(ctx, sc)
			results[idx] = result{findings: findings, err: err}
		}(i, checker)
	}
	wg.Wait()

	var allFindings []Finding
	var errs []error

	for i, r := range results {
		if r.err != nil {
			errs = append(errs, &CheckerFailedError{
				CheckerName: s.checkers[i].Name(),
				Err:         r.err,
			})
		}
		allFindings = append(allFindings, r.findings...)
	}

	return allFindings, errs
}

// DefaultCheckers returns the full set of built-in security checkers.
func DefaultCheckers() []Checker {
	return []Checker{
		NewLockFileChecker(),
		NewScriptChecker(),
		NewNetworkChecker(),
		NewEnvChecker(),
		NewSymlinkChecker(),
		NewModuleMetadataChecker(),
	}
}
