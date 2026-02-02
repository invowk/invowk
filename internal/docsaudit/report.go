// SPDX-License-Identifier: MPL-2.0

// Package docsaudit provides documentation audit reporting.
package docsaudit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var mismatchTypeOrder = []MismatchType{
	MismatchTypeMissing,
	MismatchTypeOutdated,
	MismatchTypeIncorrect,
	MismatchTypeInconsistent,
}

var severityOrder = []Severity{
	SeverityCritical,
	SeverityHigh,
	SeverityMedium,
	SeverityLow,
}

// WriteMarkdown writes the audit report to the specified path.
func WriteMarkdown(report *AuditReport, path string) error {
	if report == nil {
		return errors.New("report is nil")
	}

	if path == "" {
		return errors.New("report path is empty")
	}

	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("ensure report dir: %w", err)
	}

	generatedAt := report.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}

	var sb strings.Builder
	sb.WriteString("# Documentation/API Audit Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", generatedAt.UTC().Format(time.RFC3339)))

	writeScopeSection(&sb, report)
	writeMetricsSection(&sb, report.Metrics)
	writeSurfacesSection(&sb, report.Surfaces)
	writeFindingsSection(&sb, report.Findings)
	writeExamplesSection(&sb, report.Examples)
	writeExamplesOutsideSection(&sb, report.Examples)

	if err := writeFile(path, sb.String()); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	return nil
}

// WriteSummaryJSON writes the summary JSON to the provided writer.
func WriteSummaryJSON(w io.Writer, summary Summary) error {
	if w == nil {
		return errors.New("writer is nil")
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		return fmt.Errorf("encode summary: %w", err)
	}

	return nil
}

func writeScopeSection(sb *strings.Builder, report *AuditReport) {
	sb.WriteString("## Scope\n\n")
	sb.WriteString("### Documentation Sources\n\n")

	sources := append([]DocumentationSource(nil), report.Scope.DocSources...)
	if len(sources) == 0 {
		sources = append([]DocumentationSource(nil), report.Sources...)
	}

	sort.Slice(sources, func(i, j int) bool {
		if sources[i].ID != sources[j].ID {
			return sources[i].ID < sources[j].ID
		}
		return sources[i].Location < sources[j].Location
	})

	if len(sources) == 0 {
		sb.WriteString("- None\n\n")
	} else {
		for _, source := range sources {
			line := formatSourceLine(source)
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### Canonical Examples Path\n\n")
	if report.Scope.CanonicalExamplesPath == "" {
		sb.WriteString("- Not set\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("- %s\n\n", report.Scope.CanonicalExamplesPath))
	}

	sb.WriteString("### Assumptions\n\n")
	writeStringList(sb, report.Scope.Assumptions, "None")

	sb.WriteString("### Exclusions\n\n")
	writeStringList(sb, report.Scope.Exclusions, "None")
}

func writeMetricsSection(sb *strings.Builder, metrics Metrics) {
	sb.WriteString("## Metrics\n\n")
	sb.WriteString(fmt.Sprintf("- Total surfaces: %d\n", metrics.TotalSurfaces))
	sb.WriteString(fmt.Sprintf("- Coverage percentage: %s\n\n", formatCoverage(metrics.CoveragePercentage)))

	sb.WriteString("### Counts by Mismatch Type\n\n")
	sb.WriteString("| Mismatch Type | Count |\n")
	sb.WriteString("|---|---|\n")
	for _, mismatchType := range mismatchTypeOrder {
		count := metrics.CountsByMismatchType[mismatchType]
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", mismatchType, count))
	}
	sb.WriteString("\n")

	sb.WriteString("### Counts by Severity\n\n")
	sb.WriteString("| Severity | Count |\n")
	sb.WriteString("|---|---|\n")
	for _, severity := range severityOrder {
		count := metrics.CountsBySeverity[severity]
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", severity, count))
	}
	sb.WriteString("\n")
}

func writeSurfacesSection(sb *strings.Builder, surfaces []UserFacingSurface) {
	sb.WriteString("## Surfaces Coverage\n\n")

	sorted := append([]UserFacingSurface(nil), surfaces...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Type != sorted[j].Type {
			return sorted[i].Type < sorted[j].Type
		}
		if sorted[i].Name != sorted[j].Name {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].ID < sorted[j].ID
	})

	if len(sorted) == 0 {
		sb.WriteString("No surfaces detected.\n\n")
		return
	}

	sb.WriteString("| Type | Name | Docs |\n")
	sb.WriteString("|---|---|---|\n")
	for _, surface := range sorted {
		docs := formatDocReferences(surface.DocumentationRefs)
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
			escapeTableValue(string(surface.Type)),
			escapeTableValue(surface.Name),
			escapeTableValue(docs),
		))
	}
	sb.WriteString("\n")
}

func writeFindingsSection(sb *strings.Builder, findings []Finding) {
	sb.WriteString("## Findings\n\n")

	sorted := append([]Finding(nil), findings...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ID != sorted[j].ID {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Summary < sorted[j].Summary
	})

	if len(sorted) == 0 {
		sb.WriteString("No findings.\n\n")
		return
	}

	for idx, finding := range sorted {
		header := finding.ID
		if header == "" {
			header = fmt.Sprintf("Finding %d", idx+1)
		}

		sb.WriteString(fmt.Sprintf("### %s\n\n", header))
		sb.WriteString(fmt.Sprintf("- Severity: %s\n", emptyDash(string(finding.Severity))))
		sb.WriteString(fmt.Sprintf("- Mismatch type: %s\n", emptyDash(string(finding.MismatchType))))
		sb.WriteString(fmt.Sprintf("- Location: %s\n", emptyDash(finding.SourceLocation)))
		if finding.Summary != "" {
			sb.WriteString(fmt.Sprintf("- Summary: %s\n", finding.Summary))
		}
		sb.WriteString(fmt.Sprintf("- Expected behavior: %s\n", emptyDash(finding.ExpectedBehavior)))
		sb.WriteString(fmt.Sprintf("- Recommendation: %s\n", emptyDash(finding.Recommendation)))
		if finding.RelatedSurfaceID != "" {
			sb.WriteString(fmt.Sprintf("- Related surface: %s\n", finding.RelatedSurfaceID))
		}
		sb.WriteString("\n")
	}
}

func writeExamplesSection(sb *strings.Builder, examples []Example) {
	sb.WriteString("## Examples\n\n")

	sorted := append([]Example(nil), examples...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ID != sorted[j].ID {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].SourceLocation < sorted[j].SourceLocation
	})

	if len(sorted) == 0 {
		sb.WriteString("No examples.\n\n")
		return
	}

	sb.WriteString("| ID | Status | Location | Reason |\n")
	sb.WriteString("|---|---|---|---|\n")
	for _, example := range sorted {
		reason := example.InvalidReason
		if reason == "" {
			if example.Status == ExampleStatusInvalid {
				reason = "missing reason"
			} else {
				reason = "-"
			}
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			escapeTableValue(example.ID),
			escapeTableValue(string(example.Status)),
			escapeTableValue(example.SourceLocation),
			escapeTableValue(reason),
		))
	}
	sb.WriteString("\n")
}

func writeExamplesOutsideSection(sb *strings.Builder, examples []Example) {
	sb.WriteString("## Examples Outside Canonical Location\n\n")

	var outside []string
	for _, example := range examples {
		if example.OutsideCanonical {
			outside = append(outside, example.ID)
		}
	}

	sort.Strings(outside)
	if len(outside) == 0 {
		sb.WriteString("None\n\n")
		return
	}

	for _, id := range outside {
		sb.WriteString(fmt.Sprintf("- %s\n", id))
	}
	sb.WriteString("\n")
}

func formatSourceLine(source DocumentationSource) string {
	label := source.ID
	if label == "" {
		label = source.Location
	}
	if label == "" {
		label = "(unknown source)"
	}

	line := label
	if source.Kind != "" {
		line = fmt.Sprintf("%s (%s)", line, source.Kind)
	}
	if source.Location != "" && source.Location != label {
		line = fmt.Sprintf("%s - %s", line, source.Location)
	}
	if source.ScopeNotes != "" {
		line = fmt.Sprintf("%s — %s", line, source.ScopeNotes)
	}
	return "- " + line
}

func writeStringList(sb *strings.Builder, items []string, emptyLabel string) {
	if len(items) == 0 {
		sb.WriteString(fmt.Sprintf("- %s\n\n", emptyLabel))
		return
	}

	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s\n", item))
	}
	sb.WriteString("\n")
}

func formatDocReferences(refs []DocReference) string {
	if len(refs) == 0 {
		return "undocumented"
	}

	sorted := append([]DocReference(nil), refs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].SourceID != sorted[j].SourceID {
			return sorted[i].SourceID < sorted[j].SourceID
		}
		if sorted[i].FilePath != sorted[j].FilePath {
			return sorted[i].FilePath < sorted[j].FilePath
		}
		return sorted[i].Line < sorted[j].Line
	})

	formatted := make([]string, 0, len(sorted))
	for _, ref := range sorted {
		formatted = append(formatted, formatDocReference(ref))
	}

	return strings.Join(formatted, "<br>")
}

func formatDocReference(ref DocReference) string {
	location := ref.FilePath
	if ref.Line > 0 {
		location = fmt.Sprintf("%s:%d", ref.FilePath, ref.Line)
	}

	if ref.SourceID != "" {
		if location == "" {
			return ref.SourceID
		}
		return fmt.Sprintf("%s: %s", ref.SourceID, location)
	}

	if location == "" {
		return "(unknown)"
	}

	return location
}

func formatCoverage(value float64) string {
	return fmt.Sprintf("%.2f%%", value)
}

func escapeTableValue(value string) string {
	if value == "" {
		return "-"
	}

	escaped := strings.ReplaceAll(value, "\r", "")
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")
	escaped = strings.ReplaceAll(escaped, "|", "\\|")
	return escaped
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func writeFile(path string, content string) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("ensure output dir: %w", err)
	}

	return os.WriteFile(path, []byte(content), 0o644)
}
