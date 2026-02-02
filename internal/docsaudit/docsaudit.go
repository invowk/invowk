// SPDX-License-Identifier: MPL-2.0
// Package docsaudit provides documentation auditing helpers.
package docsaudit

import "context"

// Result is the output of a documentation audit run.
type Result struct {
	Report     *AuditReport
	ReportPath string
	Summary    Summary
}

// Summary captures the report path and metrics for a run.
type Summary struct {
	ReportPath string
	Metrics    Metrics
}

// Run executes a documentation audit and writes the report.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if err := opts.Normalize(); err != nil {
		return nil, err
	}

	report, err := Audit(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err := WriteMarkdown(report, opts.OutputPath); err != nil {
		return nil, err
	}

	summary := Summary{
		ReportPath: opts.OutputPath,
		Metrics:    report.Metrics,
	}

	return &Result{
		Report:     report,
		ReportPath: opts.OutputPath,
		Summary:    summary,
	}, nil
}
