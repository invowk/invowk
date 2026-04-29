// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/invowk/invowk/pkg/types"
)

const (
	invalidSeverityErrMsg = "invalid severity"

	severityInfoStr     = "info"
	severityLowStr      = "low"
	severityMediumStr   = "medium"
	severityHighStr     = "high"
	severityCriticalStr = "critical"

	invalidCategoryErrMsg = "invalid category"

	// Category constants are declared here for grouping with Severity constants;
	// the Category type is defined below in the type() block.

	// CategoryIntegrity covers lock file hash mismatches, version issues, and tamper detection.
	CategoryIntegrity Category = "integrity"
	// CategoryPathTraversal covers script path escapes and absolute path usage in modules.
	CategoryPathTraversal Category = "path-traversal"
	// CategoryExfiltration covers network access, DNS exfiltration, and credential extraction.
	CategoryExfiltration Category = "exfiltration"
	// CategoryExecution covers remote code execution, reverse shells, and obfuscation.
	CategoryExecution Category = "execution"
	// CategoryTrust covers module trust boundaries, global modules, and dependency chains.
	CategoryTrust Category = "trust"
	// CategoryObfuscation covers encoded content, eval patterns, and deliberate evasion.
	CategoryObfuscation Category = "obfuscation"

	// SurfaceKindRootInvowkfile identifies findings from a root standalone invowkfile.
	SurfaceKindRootInvowkfile SurfaceKind = "root_invowkfile"
	// SurfaceKindLocalModule identifies findings from a local module.
	SurfaceKindLocalModule SurfaceKind = "local_module"
	// SurfaceKindVendoredModule identifies findings from a vendored module.
	SurfaceKindVendoredModule SurfaceKind = "vendored_module"
	// SurfaceKindGlobalModule identifies findings from a global user commands module.
	SurfaceKindGlobalModule SurfaceKind = "global_module"
)

// ErrInvalidCategory is the sentinel error for unrecognized category values.
var ErrInvalidCategory = errors.New(invalidCategoryErrMsg)

type (
	// Category classifies the type of security concern a finding represents.
	Category string

	// FindingCode is a stable machine-readable finding identifier.
	FindingCode string

	// SurfaceKind classifies the trust boundary that produced a finding.
	SurfaceKind string

	// DiagnosticCode is a stable machine-readable scan diagnostic identifier.
	DiagnosticCode string

	// DiagnosticSeverity is the risk level for a scan diagnostic.
	DiagnosticSeverity string

	// DiagnosticMessage is the human-readable diagnostic message.
	DiagnosticMessage string

	// InvalidCategoryError is returned when a Category value is not recognized.
	InvalidCategoryError struct {
		Value Category
	}

	// Finding represents a single security issue discovered by a checker.
	Finding struct {
		// Code is the stable machine-readable identifier for this finding.
		Code FindingCode `json:"code,omitempty"`
		// Severity is the risk level of this finding.
		Severity Severity `json:"severity"`
		// Category classifies the type of security concern.
		Category Category `json:"category"`
		// SurfaceID identifies the attack surface (e.g., "SC-01") when applicable.
		// Empty string when the finding does not map to a known attack surface.
		SurfaceID string `json:"surface_id,omitempty"`
		// SurfaceKind identifies the trust boundary that produced the finding.
		SurfaceKind SurfaceKind `json:"surface_kind,omitempty"`
		// CheckerName is the name of the checker that produced this finding.
		CheckerName string `json:"checker_name"`
		// FilePath is the file where the issue was detected.
		FilePath types.FilesystemPath `json:"file_path"`
		// Line is the line number within FilePath (0 means unknown/not applicable).
		Line int `json:"line,omitempty"`
		// Title is a short one-line description of the finding.
		Title string `json:"title"`
		// Description provides detailed explanation of the issue.
		Description string `json:"description"`
		// Recommendation suggests how to fix or mitigate the issue.
		Recommendation string `json:"recommendation"`
		// EscalatedFrom lists titles of findings that were combined to produce
		// this compound finding. Empty for non-correlated findings.
		EscalatedFrom []string `json:"escalated_from,omitempty"`
		// EscalatedFromCodes lists finding codes combined into this compound finding.
		EscalatedFromCodes []FindingCode `json:"escalated_from_codes,omitempty"`
	}

	// Diagnostic is a non-fatal scan warning with structured identity.
	Diagnostic struct {
		severity DiagnosticSeverity
		code     DiagnosticCode
		message  DiagnosticMessage
		path     types.FilesystemPath
	}

	diagnosticOption func(*Diagnostic)

	diagnosticJSON struct {
		Severity DiagnosticSeverity   `json:"severity"`
		Code     DiagnosticCode       `json:"code"`
		Message  DiagnosticMessage    `json:"message"`
		Path     types.FilesystemPath `json:"path,omitempty"`
	}

	// Report aggregates findings from all checkers with summary statistics.
	Report struct {
		// Findings contains all individual checker findings.
		Findings []Finding `json:"findings"`
		// Correlated contains compound findings from the correlator.
		Correlated []Finding `json:"compound_threats,omitempty"`
		// Diagnostics contains non-fatal warnings from context building
		// (e.g., modules that failed to load, parse errors, discovery issues).
		Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
		// ScanDuration is how long the scan took.
		ScanDuration time.Duration `json:"-"`
		// ModuleCount is the number of modules scanned.
		ModuleCount int `json:"modules_scanned"`
		// InvowkfileCount is the number of standalone invowkfiles scanned.
		InvowkfileCount int `json:"invowkfiles_scanned"`
		// ScriptCount is the number of scripts analyzed.
		ScriptCount int `json:"scripts_scanned"`
	}
)

// String returns the string representation of the Category.
func (c Category) String() string { return string(c) }

// String returns the string representation of the FindingCode.
func (c FindingCode) String() string { return string(c) }

// Validate returns nil because finding codes are generated by trusted checkers.
func (c FindingCode) Validate() error { return nil }

// String returns the string representation of the SurfaceKind.
func (k SurfaceKind) String() string { return string(k) }

// Validate returns nil if SurfaceKind is empty or one of the known trust boundaries.
func (k SurfaceKind) Validate() error {
	switch k {
	case "", SurfaceKindRootInvowkfile, SurfaceKindLocalModule, SurfaceKindVendoredModule, SurfaceKindGlobalModule:
		return nil
	default:
		return fmt.Errorf("invalid surface kind %q", k)
	}
}

// String returns the string representation of the DiagnosticCode.
func (c DiagnosticCode) String() string { return string(c) }

// Validate returns nil because diagnostic codes are package-defined constants.
func (c DiagnosticCode) Validate() error { return nil }

// String returns the string representation of the DiagnosticSeverity.
func (s DiagnosticSeverity) String() string { return string(s) }

// Validate returns nil if the diagnostic severity is non-empty.
func (s DiagnosticSeverity) Validate() error {
	if strings.TrimSpace(string(s)) == "" {
		return errors.New("empty diagnostic severity")
	}
	return nil
}

// String returns the string representation of the DiagnosticMessage.
func (m DiagnosticMessage) String() string { return string(m) }

// Validate returns nil if the diagnostic message is non-empty.
func (m DiagnosticMessage) Validate() error {
	if strings.TrimSpace(string(m)) == "" {
		return errors.New("empty diagnostic message")
	}
	return nil
}

// WithDiagnosticPath sets the diagnostic path.
func withDiagnosticPath(path types.FilesystemPath) diagnosticOption {
	return func(d *Diagnostic) {
		d.path = path
	}
}

// NewDiagnostic constructs a structured audit diagnostic.
func NewDiagnostic(severity DiagnosticSeverity, code DiagnosticCode, message DiagnosticMessage, opts ...diagnosticOption) (Diagnostic, error) {
	d := Diagnostic{severity: severity, code: code, message: message}
	for _, opt := range opts {
		opt(&d)
	}
	if err := d.Validate(); err != nil {
		return Diagnostic{}, err
	}
	return d, nil
}

// Validate returns nil if the diagnostic has valid identity and message fields.
func (d Diagnostic) Validate() error {
	var errs []error
	if err := d.severity.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := d.code.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := d.message.Validate(); err != nil {
		errs = append(errs, err)
	}
	if d.path != "" {
		if err := d.path.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// MarshalJSON renders Diagnostic as the public audit report DTO.
func (d Diagnostic) MarshalJSON() ([]byte, error) {
	return json.Marshal(diagnosticJSON{
		Severity: d.severity,
		Code:     d.code,
		Message:  d.message,
		Path:     d.path,
	})
}

// CodeOrDefault returns Code or derives a deterministic fallback for legacy findings.
func (f Finding) CodeOrDefault() FindingCode {
	if f.Code != "" {
		return f.Code
	}
	parts := []string{f.CheckerName, f.Category.String(), f.Title}
	return FindingCode(slugParts(parts...)) //goplint:ignore -- fallback code is generated from trusted checker output.
}

//goplint:ignore -- helper normalizes presentation text into fallback machine codes.
func slugParts(parts ...string) string {
	var b strings.Builder
	for _, part := range parts {
		for _, r := range strings.ToLower(part) {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				b.WriteRune(r)
			default:
				if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
					b.WriteByte('-')
				}
			}
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// Validate returns nil if the Category is one of the defined values.
func (c Category) Validate() error {
	switch c {
	case CategoryIntegrity, CategoryPathTraversal, CategoryExfiltration,
		CategoryExecution, CategoryTrust, CategoryObfuscation:
		return nil
	default:
		return &InvalidCategoryError{Value: c}
	}
}

// Error implements the error interface.
func (e *InvalidCategoryError) Error() string {
	return fmt.Sprintf("invalid category %q", e.Value)
}

// Unwrap returns ErrInvalidCategory for errors.Is() compatibility.
func (e *InvalidCategoryError) Unwrap() error { return ErrInvalidCategory }

// AllFindings returns all findings (individual + correlated) sorted by
// severity descending, then by file path ascending.
func (r *Report) AllFindings() []Finding {
	all := make([]Finding, 0, len(r.Findings)+len(r.Correlated))
	all = append(all, r.Findings...)
	all = append(all, r.Correlated...)
	slices.SortFunc(all, func(a, b Finding) int {
		if c := cmp.Compare(b.Severity, a.Severity); c != 0 { // descending severity
			return c
		}
		if c := cmp.Compare(string(a.FilePath), string(b.FilePath)); c != 0 { // ascending path
			return c
		}
		if c := cmp.Compare(a.CheckerName, b.CheckerName); c != 0 { // ascending checker
			return c
		}
		return cmp.Compare(a.Title, b.Title) // title as final tiebreaker for determinism
	})
	return all
}

// FilterBySeverity returns findings at or above the given minimum severity.
func (r *Report) FilterBySeverity(minSev Severity) []Finding {
	all := r.AllFindings()
	return slices.DeleteFunc(all, func(f Finding) bool {
		return f.Severity < minSev
	})
}

// CountBySeverity returns finding counts keyed by severity level.
// Includes both individual and correlated findings.
func (r *Report) CountBySeverity() map[Severity]int {
	counts := make(map[Severity]int)
	for i := range r.Findings {
		counts[r.Findings[i].Severity]++
	}
	for i := range r.Correlated {
		counts[r.Correlated[i].Severity]++
	}
	return counts
}

// MaxSeverity returns the highest severity across all findings.
// Returns SeverityInfo if there are no findings.
func (r *Report) MaxSeverity() Severity {
	maxSev := SeverityInfo
	for i := range r.Findings {
		if r.Findings[i].Severity > maxSev {
			maxSev = r.Findings[i].Severity
		}
	}
	for i := range r.Correlated {
		if r.Correlated[i].Severity > maxSev {
			maxSev = r.Correlated[i].Severity
		}
	}
	return maxSev
}

// HasFindings returns true if the report contains any findings at or above
// the given minimum severity.
func (r *Report) HasFindings(minSev Severity) bool {
	for i := range r.Findings {
		if r.Findings[i].Severity >= minSev {
			return true
		}
	}
	for i := range r.Correlated {
		if r.Correlated[i].Severity >= minSev {
			return true
		}
	}
	return false
}

// FilterCorrelatedBySeverity returns correlated findings at or above the given
// minimum severity. Used by render functions that need correlated findings
// separate from individual findings (e.g., JSON output with distinct arrays).
func (r *Report) FilterCorrelatedBySeverity(minSev Severity) []Finding {
	var filtered []Finding
	for i := range r.Correlated {
		if r.Correlated[i].Severity >= minSev {
			filtered = append(filtered, r.Correlated[i])
		}
	}
	return filtered
}

// DurationMillis returns the scan duration in milliseconds for JSON output.
func (r *Report) DurationMillis() int64 {
	return r.ScanDuration.Milliseconds()
}
