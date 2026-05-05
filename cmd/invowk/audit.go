// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/app/llmconfig"
	"github.com/invowk/invowk/internal/audit"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

const (
	auditExitClean    types.ExitCode = 0
	auditExitFindings types.ExitCode = 1
	auditExitError    types.ExitCode = 2

	// auditFindingDetailPadding is the left padding for finding detail lines,
	// aligned with the indented finding title.
	auditFindingDetailPadding = 9
)

var (
	auditTitleStyle = TitleStyle.MarginBottom(1)

	auditCriticalIcon = ErrorStyle.Render("▲")
	auditHighIcon     = ErrorStyle.Render("●")
	auditMediumIcon   = WarningStyle.Render("◆")
	auditLowIcon      = SubtitleStyle.Render("○")
	auditInfoIcon     = VerboseStyle.Render("ℹ")

	auditSeparatorStyle = SubtitleStyle

	auditFindingPathStyle = lipgloss.NewStyle().
				Foreground(ColorHighlight)
	auditFindingDetailStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				PaddingLeft(auditFindingDetailPadding)
)

type (
	// auditJSONOutput is the top-level JSON structure.
	auditJSONOutput struct {
		Findings                  []auditJSONFinding    `json:"findings"`
		CompoundThreats           []auditJSONFinding    `json:"compound_threats,omitempty"`
		SuppressedFindings        []auditJSONSuppressed `json:"suppressed_findings,omitempty"`
		SuppressedCompoundThreats []auditJSONSuppressed `json:"suppressed_compound_threats,omitempty"`
		Diagnostics               []auditJSONDiagnostic `json:"diagnostics,omitempty"`
		Summary                   auditJSONSummary      `json:"summary"`
	}

	//goplint:constant-only
	//
	// auditCheckerName is a JSON DTO string for checker provenance.
	auditCheckerName string

	//goplint:ignore -- CLI JSON DTO fields are wire-format primitives.
	auditJSONFinding struct {
		Code               audit.FindingCode   `json:"code"`
		Severity           string              `json:"severity"`
		Category           string              `json:"category"`
		SurfaceID          string              `json:"surface_id,omitempty"`
		SurfaceKind        audit.SurfaceKind   `json:"surface_kind,omitempty"`
		CheckerName        auditCheckerName    `json:"checker_name"`
		FilePath           string              `json:"file_path,omitempty"`
		Line               int                 `json:"line,omitempty"`
		Title              string              `json:"title"`
		Description        string              `json:"description"`
		Recommendation     string              `json:"recommendation"`
		EscalatedFrom      []string            `json:"escalated_from,omitempty"`
		EscalatedFromCodes []audit.FindingCode `json:"escalated_from_codes,omitempty"`
	}

	//goplint:ignore -- CLI JSON DTO fields are wire-format primitives.
	auditJSONDiagnostic struct {
		Severity audit.DiagnosticSeverity `json:"severity"`
		Code     audit.DiagnosticCode     `json:"code"`
		Message  audit.DiagnosticMessage  `json:"message"`
		Path     types.FilesystemPath     `json:"path,omitempty"`
	}

	//goplint:ignore -- CLI JSON DTO fields are wire-format primitives.
	auditJSONSuppressed struct {
		Finding     auditJSONFinding        `json:"finding"`
		Disposition audit.TriageDisposition `json:"disposition"`
		Rule        audit.TriageRule        `json:"rule"`
		Rationale   audit.TriageRationale   `json:"rationale"`
	}

	auditJSONSummary struct {
		Total              int   `json:"total"`
		Suppressed         int   `json:"suppressed"`
		Critical           int   `json:"critical"`
		High               int   `json:"high"`
		Medium             int   `json:"medium"`
		Low                int   `json:"low"`
		Info               int   `json:"info"`
		ModulesScanned     int   `json:"modules_scanned"`
		InvowkfilesScanned int   `json:"invowkfiles_scanned"`
		ScriptsScanned     int   `json:"scripts_scanned"`
		DurationMS         int64 `json:"duration_ms"`
	}

	auditRunOptions struct {
		path          string //goplint:ignore -- transient CLI argument validated by BuildScanContext.
		format        string //goplint:ignore -- transient CLI flag validated by runAudit.
		minSeverity   string //goplint:ignore -- transient CLI flag validated by audit.ParseSeverity.
		configPath    string //goplint:ignore -- root CLI flag validated by config provider.
		includeGlobal bool
		llm           *llmconfig.Resolved
	}
)

func (n auditCheckerName) String() string { return string(n) }

func (n auditCheckerName) Validate() error { return nil }

func (o auditJSONOutput) Validate() error {
	for i := range o.Findings {
		if err := o.Findings[i].Validate(); err != nil {
			return err
		}
	}
	for i := range o.CompoundThreats {
		if err := o.CompoundThreats[i].Validate(); err != nil {
			return err
		}
	}
	for i := range o.SuppressedFindings {
		if err := o.SuppressedFindings[i].Validate(); err != nil {
			return err
		}
	}
	for i := range o.SuppressedCompoundThreats {
		if err := o.SuppressedCompoundThreats[i].Validate(); err != nil {
			return err
		}
	}
	for i := range o.Diagnostics {
		if err := o.Diagnostics[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (f auditJSONFinding) Validate() error {
	var errs []error
	errs = append(errs,
		f.Code.Validate(),
		f.CheckerName.Validate(),
		f.SurfaceKind.Validate(),
	)
	for _, code := range f.EscalatedFromCodes {
		errs = append(errs, code.Validate())
	}
	return errors.Join(errs...)
}

func (s auditJSONSuppressed) Validate() error {
	return errors.Join(
		s.Finding.Validate(),
		s.Disposition.Validate(),
		s.Rule.Validate(),
		s.Rationale.Validate(),
	)
}

func (d auditJSONDiagnostic) Validate() error {
	var errs []error
	errs = append(errs,
		d.Severity.Validate(),
		d.Code.Validate(),
		d.Message.Validate(),
	)
	if d.Path != "" {
		errs = append(errs, d.Path.Validate())
	}
	return errors.Join(errs...)
}

func (opts auditRunOptions) Validate() error {
	if opts.llm != nil {
		return opts.llm.Validate()
	}
	return nil
}

// newAuditCommand creates the top-level `invowk audit` command.
func newAuditCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	var (
		format        string
		minSeverity   string
		includeGlobal bool
		llmFlags      llmFlagValues
	)

	cmd := &cobra.Command{
		Use:   "audit [path]",
		Short: "Scan for security risks",
		Long: `Analyze invowkfiles and modules for supply-chain vulnerabilities, script
injection, path traversal, suspicious patterns, and lock file integrity issues.

The audit scans standalone invowkfiles, local modules, vendored dependencies,
and optionally global modules in ~/.invowk/cmds/.

Use --llm-provider to enable LLM-powered security analysis. Providers:
auto (detect best available), claude, codex, gemini, ollama.
Use --llm to enable LLM analysis using configured or OpenAI-compatible API settings.

Exit codes:
  0  No findings at or above the severity threshold
  1  Findings detected
  2  Scan error`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			auditPath := "."
			if len(args) > 0 {
				auditPath = args[0]
			}

			if err := validateLLMFlagSelection(llmFlags); err != nil {
				return &ExitError{
					Code: auditExitError,
					Err:  err,
				}
			}

			var llm *llmconfig.Resolved
			if llmFlags.enable || llmFlags.provider != "" {
				resolved, llmErr := resolveLLMForCommand(
					cmd.Context(),
					cmd,
					app.Config,
					types.FilesystemPath(rootFlags.configPath), //goplint:ignore -- root flag value is validated by config provider.
					llmFlags,
					false,
				)
				if llmErr != nil {
					return llmErr
				}
				llm = resolved
			}

			return runAudit(cmd, app, auditRunOptions{
				path:          auditPath,
				format:        format,
				minSeverity:   minSeverity,
				configPath:    rootFlags.configPath,
				includeGlobal: includeGlobal,
				llm:           llm,
			})
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text, json")
	cmd.Flags().StringVar(&minSeverity, "severity", "low", "minimum severity: info, low, medium, high, critical")
	cmd.Flags().BoolVar(&includeGlobal, "include-global", false, "include ~/.invowk/cmds/ in scan")

	bindLLMFlags(cmd, &llmFlags)

	return cmd
}

func runAudit(cmd *cobra.Command, app *App, opts auditRunOptions) error {
	ctx := cmd.Context()
	w := cmd.OutOrStdout()

	if err := opts.Validate(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid LLM configuration: %v\n", err)
		return &ExitError{Code: auditExitError, Err: err}
	}

	// Parse minimum severity.
	minSev, err := audit.ParseSeverity(opts.minSeverity)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid severity: %v\n", err)
		return &ExitError{Code: auditExitError, Err: err}
	}

	// Create scanner with optional LLM checker.
	scannerOpts := []audit.ScannerOption{
		audit.WithConfigLoadOptions(config.LoadOptions{
			ConfigFilePath: types.FilesystemPath(opts.configPath), //goplint:ignore -- root flag value is validated by config provider.
		}),
	}

	if opts.llm != nil && opts.llm.Mode != llmconfig.ModeNone {
		result, completerErr := buildLLMCompleter(ctx, cmd, opts.llm)
		if completerErr != nil {
			return completerErr
		}
		scannerOpts = append(scannerOpts, audit.WithChecker(audit.NewLLMChecker(result.completer, result.concurrency)))
	}
	scanner, err := audit.NewScanner(app.Config, scannerOpts...)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid scanner configuration: %v\n", err)
		return &ExitError{Code: auditExitError, Err: err}
	}

	// Run scan.
	report, scanErr := scanner.Scan(ctx, types.FilesystemPath(opts.path), opts.includeGlobal) //goplint:ignore -- CLI arg path from Cobra, validated by filepath.Abs in BuildScanContext
	if scanErr != nil && report == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "scan error: %v\n", scanErr)
		return &ExitError{Code: auditExitError, Err: scanErr}
	}
	if scanErr != nil && scanErrorContainsChecker(scanErr, audit.LLMCheckerName) {
		fmt.Fprintf(cmd.ErrOrStderr(), "scan error: %v\n", scanErr)
		return &ExitError{Code: auditExitError, Err: scanErr}
	}

	// If scanner returned partial results with errors, warn on stderr.
	if scanErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: some checkers failed: %v\n", scanErr)
	}

	// Render output.
	switch strings.ToLower(opts.format) {
	case "json":
		if err := renderAuditJSON(w, report, minSev); err != nil {
			return &ExitError{Code: auditExitError, Err: err}
		}
	case "text":
		renderAuditText(w, report, opts.path, minSev)
	default:
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: unknown output format %q (must be \"text\" or \"json\")\n", opts.format)
		return &ExitError{Code: auditExitError}
	}

	// Determine exit code based on confirmed findings after deterministic triage.
	if audit.ClassifyReportFindings(report).HasConfirmedFindings(minSev) {
		return &ExitError{Code: auditExitFindings}
	}

	return nil
}

//goplint:ignore -- CLI needs a small predicate over joined scanner errors.
func scanErrorContainsChecker(err error, checkerName string) bool {
	if err == nil {
		return false
	}

	pending := []error{err}
	for len(pending) > 0 {
		last := len(pending) - 1
		current := pending[last]
		pending = pending[:last]

		if failed, ok := errors.AsType[*audit.CheckerFailedError](current); ok && failed.CheckerName == checkerName {
			return true
		}
		if joined, ok := current.(interface{ Unwrap() []error }); ok {
			pending = append(pending, joined.Unwrap()...)
		}
	}

	return false
}

func renderAuditText(w io.Writer, report *audit.Report, scanPath string, minSev audit.Severity) {
	fmt.Fprintln(w, auditTitleStyle.Render("Security Audit — "+scanPath))
	fmt.Fprintf(w, "Scanned: %d module(s), %d invowkfile(s), %d script(s) (%s)\n\n",
		report.ModuleCount, report.InvowkfileCount, report.ScriptCount,
		formatDuration(report.ScanDuration))

	triage := audit.ClassifyReportFindings(report)
	findings := triage.ConfirmedFindingsBySeverity(minSev)
	filteredCorrelated := triage.ConfirmedCompoundThreatsBySeverity(minSev)
	renderAuditDiagnostics(w, report.Diagnostics)

	if len(findings) == 0 && len(filteredCorrelated) == 0 {
		fmt.Fprintf(w, "%s No confirmed findings at or above %s severity\n", SuccessStyle.Render("✓"), minSev)
		renderSuppressedAuditFindings(w, triage, minSev)
		return
	}

	// Group findings by severity.
	grouped := groupBySeverity(findings)
	severityOrder := []audit.Severity{
		audit.SeverityCritical, audit.SeverityHigh, audit.SeverityMedium,
		audit.SeverityLow, audit.SeverityInfo,
	}

	for _, sev := range severityOrder {
		group := grouped[sev]
		if len(group) == 0 {
			continue
		}

		icon := severityIcon(sev)
		label := strings.ToUpper(sev.String())
		fmt.Fprintf(w, "%s %s (%d)\n", icon, label, len(group))

		for i := range group {
			fmt.Fprintf(w, "  %s\n", group[i].Title)
			if group[i].FilePath != "" {
				pathStr := auditFindingPathStyle.Render(string(group[i].FilePath))
				if group[i].Line > 0 {
					pathStr = auditFindingPathStyle.Render(fmt.Sprintf("%s:%d", group[i].FilePath, group[i].Line))
				}
				fmt.Fprintln(w, auditFindingDetailStyle.Render("File: "+pathStr))
			}
			if group[i].Description != "" {
				fmt.Fprintln(w, auditFindingDetailStyle.Render(group[i].Description))
			}
			if group[i].Recommendation != "" {
				fmt.Fprintln(w, auditFindingDetailStyle.Render("Fix: "+group[i].Recommendation))
			}
			fmt.Fprintln(w)
		}
	}

	if len(filteredCorrelated) > 0 {
		fmt.Fprintln(w, auditSeparatorStyle.Render("═══ Compound Threats ═══"))
		for i := range filteredCorrelated {
			icon := severityIcon(filteredCorrelated[i].Severity)
			fmt.Fprintf(w, "  %s %s\n", icon, filteredCorrelated[i].Title)
			if filteredCorrelated[i].Description != "" {
				fmt.Fprintln(w, auditFindingDetailStyle.Render(filteredCorrelated[i].Description))
			}
			if len(filteredCorrelated[i].EscalatedFrom) > 0 {
				fmt.Fprintln(w, auditFindingDetailStyle.Render("Escalated from: "+strings.Join(filteredCorrelated[i].EscalatedFrom, ", ")))
			}
			fmt.Fprintln(w)
		}
	}
	renderSuppressedAuditFindings(w, triage, minSev)

	// Summary line.
	counts := countFindingsBySeverity(findings, filteredCorrelated)
	fmt.Fprintf(w, "Summary: %d critical, %d high, %d medium, %d low, %d info\n",
		counts[audit.SeverityCritical], counts[audit.SeverityHigh],
		counts[audit.SeverityMedium], counts[audit.SeverityLow],
		counts[audit.SeverityInfo])
}

func renderSuppressedAuditFindings(w io.Writer, triage audit.ReportTriage, minSev audit.Severity) {
	suppressed := triage.SuppressedFindingsBySeverity(minSev)
	suppressedCompound := triage.SuppressedCompoundThreatsBySeverity(minSev)
	if len(suppressed) == 0 && len(suppressedCompound) == 0 {
		return
	}

	fmt.Fprintln(w, auditSeparatorStyle.Render("═══ Suppressed By Design ═══"))
	for i := range suppressed {
		fmt.Fprintf(w, "  %s [%s] %s\n", auditInfoIcon, suppressed[i].Rule(), suppressed[i].Finding().Title)
		fmt.Fprintln(w, auditFindingDetailStyle.Render(suppressed[i].Rationale().String()))
	}
	for i := range suppressedCompound {
		fmt.Fprintf(w, "  %s [%s] %s\n", auditInfoIcon, suppressedCompound[i].Rule(), suppressedCompound[i].Finding().Title)
		fmt.Fprintln(w, auditFindingDetailStyle.Render(suppressedCompound[i].Rationale().String()))
	}
	fmt.Fprintln(w)
}

func renderAuditDiagnostics(w io.Writer, diagnostics []audit.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}

	fmt.Fprintf(w, "%s Diagnostics (%d)\n", WarningStyle.Render("!"), len(diagnostics))
	for i := range diagnostics {
		fmt.Fprintf(w, "  %s: %s\n", diagnostics[i].Code(), diagnostics[i].Message())
		if diagnostics[i].Path() != "" {
			pathStr := auditFindingPathStyle.Render(string(diagnostics[i].Path()))
			fmt.Fprintln(w, auditFindingDetailStyle.Render("Path: "+pathStr))
		}
	}
	fmt.Fprintln(w)
}

func renderAuditJSON(w io.Writer, report *audit.Report, minSev audit.Severity) error {
	triage := audit.ClassifyReportFindings(report)
	filtered := triage.ConfirmedFindingsBySeverity(minSev)

	// Apply the same severity filter to correlated findings so the JSON
	// total is consistent with the displayed findings.
	filteredCorrelated := triage.ConfirmedCompoundThreatsBySeverity(minSev)
	suppressed := triage.SuppressedFindingsBySeverity(minSev)
	suppressedCorrelated := triage.SuppressedCompoundThreatsBySeverity(minSev)

	// Count only filtered findings so the severity breakdown matches the
	// findings and compound_threats arrays in the output (M16 fix).
	counts := countFindingsBySeverity(filtered, filteredCorrelated)

	output := auditJSONOutput{
		Findings:                  convertFindings(filtered),
		CompoundThreats:           convertFindings(filteredCorrelated),
		SuppressedFindings:        convertSuppressedFindings(suppressed),
		SuppressedCompoundThreats: convertSuppressedFindings(suppressedCorrelated),
		Diagnostics:               convertDiagnostics(report.Diagnostics),
		Summary: auditJSONSummary{
			Total:              len(filtered) + len(filteredCorrelated),
			Suppressed:         len(suppressed) + len(suppressedCorrelated),
			Critical:           counts[audit.SeverityCritical],
			High:               counts[audit.SeverityHigh],
			Medium:             counts[audit.SeverityMedium],
			Low:                counts[audit.SeverityLow],
			Info:               counts[audit.SeverityInfo],
			ModulesScanned:     report.ModuleCount,
			InvowkfilesScanned: report.InvowkfileCount,
			ScriptsScanned:     report.ScriptCount,
			DurationMS:         report.DurationMillis(),
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(output); err != nil {
		return fmt.Errorf("encoding audit JSON: %w", err)
	}
	return nil
}

//goplint:ignore -- CLI summary counts are map values for JSON/text rendering.
func countFindingsBySeverity(findings, correlated []audit.Finding) map[audit.Severity]int {
	counts := make(map[audit.Severity]int)
	for i := range findings {
		counts[findings[i].Severity]++
	}
	for i := range correlated {
		counts[correlated[i].Severity]++
	}
	return counts
}

func convertFindings(findings []audit.Finding) []auditJSONFinding {
	result := make([]auditJSONFinding, 0, len(findings))
	for i := range findings {
		result = append(result, auditJSONFinding{
			Code:               findings[i].CodeOrDefault(),
			Severity:           findings[i].Severity.String(),
			Category:           findings[i].Category.String(),
			SurfaceID:          findings[i].SurfaceID,
			SurfaceKind:        findings[i].SurfaceKind,
			CheckerName:        auditCheckerName(findings[i].CheckerName), //goplint:ignore -- checker names are internal scanner identifiers.
			FilePath:           string(findings[i].FilePath),
			Line:               findings[i].Line,
			Title:              findings[i].Title,
			Description:        findings[i].Description,
			Recommendation:     findings[i].Recommendation,
			EscalatedFrom:      findings[i].EscalatedFrom,
			EscalatedFromCodes: findingCodesToStrings(findings[i].EscalatedFromCodes),
		})
	}
	return result
}

func convertSuppressedFindings(findings []audit.TriagedFinding) []auditJSONSuppressed {
	result := make([]auditJSONSuppressed, 0, len(findings))
	for i := range findings {
		result = append(result, auditJSONSuppressed{
			Finding:     convertFindings([]audit.Finding{findings[i].Finding()})[0],
			Disposition: findings[i].Disposition(),
			Rule:        findings[i].Rule(),
			Rationale:   findings[i].Rationale(),
		})
	}
	return result
}

func convertDiagnostics(diagnostics []audit.Diagnostic) []auditJSONDiagnostic {
	result := make([]auditJSONDiagnostic, 0, len(diagnostics))
	for i := range diagnostics {
		result = append(result, auditJSONDiagnostic{
			Severity: diagnostics[i].Severity(),
			Code:     diagnostics[i].Code(),
			Message:  diagnostics[i].Message(),
			Path:     diagnostics[i].Path(),
		})
	}
	return result
}

func findingCodesToStrings(codes []audit.FindingCode) []audit.FindingCode {
	if len(codes) == 0 {
		return nil
	}
	return append([]audit.FindingCode(nil), codes...)
}

func groupBySeverity(findings []audit.Finding) map[audit.Severity][]audit.Finding {
	grouped := make(map[audit.Severity][]audit.Finding)
	for i := range findings {
		grouped[findings[i].Severity] = append(grouped[findings[i].Severity], findings[i])
	}
	return grouped
}

func severityIcon(s audit.Severity) string {
	switch s {
	case audit.SeverityCritical:
		return auditCriticalIcon
	case audit.SeverityHigh:
		return auditHighIcon
	case audit.SeverityMedium:
		return auditMediumIcon
	case audit.SeverityLow:
		return auditLowIcon
	case audit.SeverityInfo:
		return auditInfoIcon
	default:
		return " "
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%d\u00b5s", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
