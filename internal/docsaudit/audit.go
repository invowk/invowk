// SPDX-License-Identifier: MPL-2.0

// Package docsaudit provides documentation audit orchestration.
package docsaudit

import (
	"context"
	"fmt"
	"time"
)

// Audit orchestrates the documentation audit based on options.
func Audit(ctx context.Context, opts Options) (*AuditReport, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	catalog, err := DiscoverSources(ctx, opts)
	if err != nil {
		return nil, err
	}

	var surfaces []UserFacingSurface
	if opts.RootCmd != nil {
		cliSurfaces, err := DiscoverCLISurfaces(opts.RootCmd)
		if err != nil {
			return nil, err
		}
		surfaces = append(surfaces, cliSurfaces...)
	}

	configSurfaces, err := DiscoverConfigSurfaces()
	if err != nil {
		return nil, err
	}
	surfaces = append(surfaces, configSurfaces...)

	moduleSurfaces, err := DiscoverModuleSurfaces(opts)
	if err != nil {
		return nil, err
	}
	surfaces = append(surfaces, moduleSurfaces...)

	updatedSurfaces, findings, err := MatchDocumentation(ctx, catalog, surfaces)
	if err != nil {
		return nil, err
	}

	findings = ApplySeverity(findings)
	findings = ApplyRecommendations(findings)

	examples, err := ExtractExamples(ctx, catalog)
	if err != nil {
		return nil, err
	}

	examples, err = ValidateExamples(ctx, examples, updatedSurfaces)
	if err != nil {
		return nil, err
	}
	examples = MarkExamplesOutsideCanonical(examples, opts.CanonicalExamplesPath)

	metrics := ComputeMetrics(updatedSurfaces, findings)

	report := &AuditReport{
		GeneratedAt: time.Now().UTC(),
		Scope: AuditScope{
			DocSources:            catalog.Sources,
			Exclusions:            defaultExclusions(opts),
			Assumptions:           defaultAssumptions(opts),
			CanonicalExamplesPath: opts.CanonicalExamplesPath,
		},
		Metrics:  metrics,
		Sources:  catalog.Sources,
		Surfaces: updatedSurfaces,
		Findings: findings,
		Examples: examples,
	}

	return report, nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled: %w", ctx.Err())
	default:
		return nil
	}
}

func defaultAssumptions(opts Options) []string {
	assumptions := []string{
		"Audit targets the current repository content at execution time.",
		"User-facing surfaces include CLI commands, flags, configuration fields, and module definitions.",
		"Examples and sample modules are treated as documentation sources.",
	}
	if opts.ExcludePkgAPIs {
		assumptions = append(assumptions, "Public Go packages under pkg/ are out of scope unless explicitly documented.")
	}
	return assumptions
}

func defaultExclusions(_ Options) []string {
	return []string{
		"External blog posts, issue trackers, and roadmap discussions.",
		"Non-repository documentation not referenced by in-repo sources.",
	}
}
