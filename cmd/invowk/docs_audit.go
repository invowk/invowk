// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"invowk-cli/internal/docsaudit"

	"github.com/spf13/cobra"
)

var (
	docsAuditOutPath      string
	docsAuditOutputFormat string
	docsAuditJSON         bool
	docsAuditCmd          = &cobra.Command{
		Use:   "audit",
		Short: "Audit documentation against current features",
		Long: `Generate a documentation audit report and emit a summary with
coverage metrics, mismatch counts, and report path details.`,
		Hidden: true,
		RunE:   runDocsAudit,
	}
)

func init() {
	docsAuditCmd.Flags().StringVarP(&docsAuditOutPath, "out", "o", "", "write the audit report to this path")
	docsAuditCmd.Flags().StringVar(&docsAuditOutputFormat, "output", "human", "output JSON summary to stdout")
	docsAuditCmd.Flags().BoolVar(&docsAuditJSON, "json", false, "output JSON summary to stdout (alias for --output json)")
}

func runDocsAudit(cmd *cobra.Command, _ []string) error {
	outputFormat := docsaudit.OutputFormat(docsAuditOutputFormat)
	if docsAuditJSON {
		outputFormat = docsaudit.OutputFormatJSON
	}

	opts := docsaudit.Options{
		OutputPath:   docsAuditOutPath,
		OutputFormat: outputFormat,
		RootCmd:      rootCmd,
	}

	result, err := docsaudit.Run(cmd.Context(), opts)
	if err != nil {
		return err
	}

	if outputFormat == docsaudit.OutputFormatJSON {
		payload, err := json.MarshalIndent(result.Summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal summary: %w", err)
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(payload)); err != nil {
			return fmt.Errorf("write summary: %w", err)
		}
		return nil
	}

	metrics := result.Summary.Metrics
	reportPath := result.Summary.ReportPath
	if reportPath == "" {
		reportPath = result.ReportPath
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, TitleStyle.Render("Documentation Audit Summary"))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s %s\n", SubtitleStyle.Render("Report:"), CmdStyle.Render(reportPath))
	fmt.Fprintf(out, "%s %s\n", SubtitleStyle.Render("Total surfaces:"), SuccessStyle.Render(strconv.Itoa(metrics.TotalSurfaces)))
	fmt.Fprintf(out, "%s %s\n", SubtitleStyle.Render("Coverage:"), SuccessStyle.Render(formatCoverage(metrics.CoveragePercentage)))
	fmt.Fprintf(out, "%s %s\n", SubtitleStyle.Render("Mismatches by type:"), CmdStyle.Render(formatMismatchCounts(metrics.CountsByMismatchType, []docsaudit.MismatchType{docsaudit.MismatchTypeMissing, docsaudit.MismatchTypeOutdated, docsaudit.MismatchTypeIncorrect, docsaudit.MismatchTypeInconsistent})))
	fmt.Fprintf(out, "%s %s\n", SubtitleStyle.Render("Mismatches by severity:"), CmdStyle.Render(formatSeverityCounts(metrics.CountsBySeverity, []docsaudit.Severity{docsaudit.SeverityCritical, docsaudit.SeverityHigh, docsaudit.SeverityMedium, docsaudit.SeverityLow})))

	return nil
}

func formatCoverage(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func formatMismatchCounts(counts map[docsaudit.MismatchType]int, order []docsaudit.MismatchType) string {
	if len(counts) == 0 {
		return "none"
	}

	parts := make([]string, 0, len(counts))
	seen := make(map[docsaudit.MismatchType]struct{}, len(counts))

	for _, key := range order {
		if value, ok := counts[key]; ok {
			parts = append(parts, fmt.Sprintf("%s=%d", key, value))
			seen[key] = struct{}{}
		}
	}

	var extras []string
	for key, value := range counts {
		if _, ok := seen[key]; ok {
			continue
		}
		extras = append(extras, fmt.Sprintf("%s=%d", key, value))
	}
	sort.Strings(extras)
	parts = append(parts, extras...)

	return strings.Join(parts, ", ")
}

func formatSeverityCounts(counts map[docsaudit.Severity]int, order []docsaudit.Severity) string {
	if len(counts) == 0 {
		return "none"
	}

	parts := make([]string, 0, len(counts))
	seen := make(map[docsaudit.Severity]struct{}, len(counts))

	for _, key := range order {
		if value, ok := counts[key]; ok {
			parts = append(parts, fmt.Sprintf("%s=%d", key, value))
			seen[key] = struct{}{}
		}
	}

	var extras []string
	for key, value := range counts {
		if _, ok := seen[key]; ok {
			continue
		}
		extras = append(extras, fmt.Sprintf("%s=%d", key, value))
	}
	sort.Strings(extras)
	parts = append(parts, extras...)

	return strings.Join(parts, ", ")
}
