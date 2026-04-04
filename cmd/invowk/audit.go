// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/audit"
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
		Findings        []auditJSONFinding `json:"findings"`
		CompoundThreats []auditJSONFinding `json:"compound_threats,omitempty"`
		Summary         auditJSONSummary   `json:"summary"`
	}

	auditJSONFinding struct {
		Severity       string   `json:"severity"`
		Category       string   `json:"category"`
		SurfaceID      string   `json:"surface_id,omitempty"`
		FilePath       string   `json:"file_path,omitempty"`
		Line           int      `json:"line,omitempty"`
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Recommendation string   `json:"recommendation"`
		EscalatedFrom  []string `json:"escalated_from,omitempty"`
	}

	auditJSONSummary struct {
		Total              int   `json:"total"`
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
)

// newAuditCommand creates the top-level `invowk audit` command.
func newAuditCommand(app *App) *cobra.Command {
	var (
		format        string
		minSeverity   string
		includeGlobal bool

		// LLM flags.
		enableLLM      bool
		llmProvider    string
		llmURL         string
		llmModel       string
		llmAPIKey      string
		llmTimeout     time.Duration
		llmConcurrency int
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
Use --llm for manual configuration via --llm-url and --llm-api-key.

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

			// Validate mutual exclusion of --llm and --llm-provider.
			if enableLLM && llmProvider != "" {
				return &ExitError{
					Code: auditExitError,
					Err:  errors.New("--llm and --llm-provider are mutually exclusive"),
				}
			}

			// Validate provider name at the CLI boundary.
			if llmProvider != "" {
				validProviders := audit.ValidProviders()
				if !slices.Contains(validProviders, llmProvider) {
					return &ExitError{
						Code: auditExitError,
						Err:  fmt.Errorf("invalid --llm-provider %q; valid: %s", llmProvider, strings.Join(validProviders, ", ")),
					}
				}
			}

			// Resolve LLM flags with env var fallbacks.
			llmOpts := resolveLLMFlags(cmd, audit.LLMClientConfig{
				BaseURL:     llmURL,
				Model:       llmModel,
				APIKey:      llmAPIKey,
				Timeout:     llmTimeout,
				Concurrency: llmConcurrency,
			})

			return runAudit(cmd, app, auditPath, format, minSeverity, includeGlobal, enableLLM, llmProvider, llmOpts)
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text, json")
	cmd.Flags().StringVar(&minSeverity, "severity", "low", "minimum severity: info, low, medium, high, critical")
	cmd.Flags().BoolVar(&includeGlobal, "include-global", false, "include ~/.invowk/cmds/ in scan")

	// LLM flags.
	cmd.Flags().StringVar(&llmProvider, "llm-provider", "", "auto-detect or use specific provider: auto, claude, codex, gemini, ollama")
	cmd.Flags().BoolVar(&enableLLM, "llm", false, "enable LLM with manual --llm-url/--llm-api-key config")
	cmd.Flags().StringVar(&llmURL, "llm-url", audit.DefaultLLMBaseURL, "OpenAI-compatible API base URL")
	cmd.Flags().StringVar(&llmModel, "llm-model", audit.DefaultLLMModel, "LLM model name")
	cmd.Flags().StringVar(&llmAPIKey, "llm-api-key", "", "API key (empty for local servers)")
	cmd.Flags().DurationVar(&llmTimeout, "llm-timeout", audit.DefaultLLMTimeout, "per-request timeout")
	cmd.Flags().IntVar(&llmConcurrency, "llm-concurrency", audit.DefaultLLMConcurrency, "max parallel LLM requests")

	return cmd
}

func runAudit(
	cmd *cobra.Command, app *App,
	auditPath, format, minSeverity string,
	includeGlobal, enableLLM bool,
	llmProvider string,
	llmOpts audit.LLMClientConfig,
) error {
	ctx := cmd.Context()
	w := cmd.OutOrStdout()

	// Parse minimum severity.
	minSev, err := audit.ParseSeverity(minSeverity)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid severity: %v\n", err)
		return &ExitError{Code: auditExitError, Err: err}
	}

	// Create scanner with optional LLM checker.
	var scannerOpts []audit.ScannerOption

	switch {
	case llmProvider != "":
		// Auto-detect or use specific provider.
		checker, checkerErr := buildProviderChecker(ctx, cmd, llmProvider, llmOpts)
		if checkerErr != nil {
			return checkerErr
		}
		scannerOpts = append(scannerOpts, audit.WithChecker(checker))

	case enableLLM:
		// Manual --llm configuration.
		checker, checkerErr := buildManualChecker(ctx, cmd, llmOpts)
		if checkerErr != nil {
			return checkerErr
		}
		scannerOpts = append(scannerOpts, audit.WithChecker(checker))
	}
	scanner := audit.NewScanner(app.Config, scannerOpts...)

	// Run scan.
	report, scanErr := scanner.Scan(ctx, types.FilesystemPath(auditPath), includeGlobal) //goplint:ignore -- CLI arg path from Cobra, validated by filepath.Abs in BuildScanContext
	if scanErr != nil && report == nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "scan error: %v\n", scanErr)
		return &ExitError{Code: auditExitError, Err: scanErr}
	}

	// If scanner returned partial results with errors, warn on stderr.
	if scanErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: some checkers failed: %v\n", scanErr)
	}

	// Render output.
	switch strings.ToLower(format) {
	case "json":
		if err := renderAuditJSON(w, report, minSev); err != nil {
			return &ExitError{Code: auditExitError, Err: err}
		}
	case "text":
		renderAuditText(w, report, auditPath, minSev)
	default:
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: unknown output format %q (must be \"text\" or \"json\")\n", format)
		return &ExitError{Code: auditExitError}
	}

	// Determine exit code based on filtered findings.
	if report.HasFindings(minSev) {
		return &ExitError{Code: auditExitFindings}
	}

	return nil
}

func renderAuditText(w io.Writer, report *audit.Report, scanPath string, minSev audit.Severity) {
	fmt.Fprintln(w, auditTitleStyle.Render("Security Audit — "+scanPath))
	fmt.Fprintf(w, "Scanned: %d module(s), %d invowkfile(s), %d script(s) (%s)\n\n",
		report.ModuleCount, report.InvowkfileCount, report.ScriptCount,
		formatDuration(report.ScanDuration))

	findings := report.FilterBySeverity(minSev)

	if len(findings) == 0 {
		fmt.Fprintf(w, "%s No findings at or above %s severity\n", SuccessStyle.Render("✓"), minSev)
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

	// Compound threats section (filtered by minimum severity).
	filteredCorrelated := report.FilterCorrelatedBySeverity(minSev)
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

	// Summary line.
	counts := report.CountBySeverity()
	fmt.Fprintf(w, "Summary: %d critical, %d high, %d medium, %d low, %d info\n",
		counts[audit.SeverityCritical], counts[audit.SeverityHigh],
		counts[audit.SeverityMedium], counts[audit.SeverityLow],
		counts[audit.SeverityInfo])
}

func renderAuditJSON(w io.Writer, report *audit.Report, minSev audit.Severity) error {
	filtered := report.FilterBySeverity(minSev)

	// Apply the same severity filter to correlated findings so the JSON
	// total is consistent with the displayed findings.
	filteredCorrelated := report.FilterCorrelatedBySeverity(minSev)

	// Count only filtered findings so the severity breakdown matches the
	// findings and compound_threats arrays in the output (M16 fix).
	counts := make(map[audit.Severity]int)
	for i := range filtered {
		counts[filtered[i].Severity]++
	}
	for i := range filteredCorrelated {
		counts[filteredCorrelated[i].Severity]++
	}

	output := auditJSONOutput{
		Findings:        convertFindings(filtered),
		CompoundThreats: convertFindings(filteredCorrelated),
		Summary: auditJSONSummary{
			Total:              len(filtered) + len(filteredCorrelated),
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

func convertFindings(findings []audit.Finding) []auditJSONFinding {
	result := make([]auditJSONFinding, 0, len(findings))
	for i := range findings {
		result = append(result, auditJSONFinding{
			Severity:       findings[i].Severity.String(),
			Category:       findings[i].Category.String(),
			SurfaceID:      findings[i].SurfaceID,
			FilePath:       string(findings[i].FilePath),
			Line:           findings[i].Line,
			Title:          findings[i].Title,
			Description:    findings[i].Description,
			Recommendation: findings[i].Recommendation,
			EscalatedFrom:  findings[i].EscalatedFrom,
		})
	}
	return result
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

// resolveLLMFlags applies env var fallbacks for LLM flags. Cobra flags take
// precedence; env vars only apply when the flag was not explicitly set.
func resolveLLMFlags(cmd *cobra.Command, cfg audit.LLMClientConfig) audit.LLMClientConfig {
	if !cmd.Flags().Changed("llm-url") {
		if envURL := os.Getenv("INVOWK_LLM_URL"); envURL != "" {
			cfg.BaseURL = envURL
		}
	}
	if !cmd.Flags().Changed("llm-model") {
		if envModel := os.Getenv("INVOWK_LLM_MODEL"); envModel != "" {
			cfg.Model = envModel
		}
	}
	if !cmd.Flags().Changed("llm-api-key") {
		if envKey := os.Getenv("INVOWK_LLM_API_KEY"); envKey != "" {
			cfg.APIKey = envKey
		}
	}
	if !cmd.Flags().Changed("llm-timeout") {
		if envTimeout := os.Getenv("INVOWK_LLM_TIMEOUT"); envTimeout != "" {
			if d, err := time.ParseDuration(envTimeout); err == nil {
				cfg.Timeout = d
			}
		}
	}
	if !cmd.Flags().Changed("llm-concurrency") {
		if envConc := os.Getenv("INVOWK_LLM_CONCURRENCY"); envConc != "" {
			if n, err := strconv.Atoi(envConc); err == nil {
				cfg.Concurrency = n
			}
		}
	}

	return cfg
}

// buildProviderChecker creates an LLM checker via auto-detection or a named provider.
func buildProviderChecker(ctx context.Context, cmd *cobra.Command, provider string, llmOpts audit.LLMClientConfig) (*audit.LLMChecker, *ExitError) {
	modelOverride := ""
	if cmd.Flags().Changed("llm-model") {
		modelOverride = llmOpts.Model
	}

	timeout := llmOpts.Timeout
	if timeout == 0 {
		timeout = audit.DefaultLLMTimeout
	}

	result, err := audit.DetectProvider(ctx, provider, modelOverride, timeout)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}

	// Verify model availability when the provider supports it.
	// HTTP-based completers (Ollama, cloud env vars) support model listing;
	// CLI-based completers (claude, codex, gemini) do not.
	if verifier, ok := result.Completer().(audit.ModelVerifier); ok {
		if verifyErr := verifier.VerifyModel(ctx); verifyErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", verifyErr)
			return nil, &ExitError{Code: auditExitError, Err: verifyErr}
		}
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Using LLM provider: %s (model: %s)\n", result.Name(), result.Model())

	concurrency := llmOpts.Concurrency
	if concurrency == 0 {
		concurrency = audit.DefaultLLMConcurrency
	}

	return audit.NewLLMChecker(result.Completer(), concurrency), nil
}

// buildManualChecker creates an LLM checker from manual --llm configuration.
func buildManualChecker(ctx context.Context, cmd *cobra.Command, llmOpts audit.LLMClientConfig) (*audit.LLMChecker, *ExitError) {
	llmClient, llmErr := audit.NewLLMClient(llmOpts)
	if llmErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "LLM configuration error: %v\n", llmErr)
		return nil, &ExitError{Code: auditExitError, Err: llmErr}
	}

	if verifyErr := llmClient.VerifyModel(ctx); verifyErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", verifyErr)
		return nil, &ExitError{Code: auditExitError, Err: verifyErr}
	}

	return audit.NewLLMChecker(llmClient, llmOpts.Concurrency), nil
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
