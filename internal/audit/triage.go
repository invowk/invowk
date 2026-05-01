// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"cmp"
	"errors"
	"slices"
)

const (
	// TriageDispositionConfirmed marks findings that should be reported.
	TriageDispositionConfirmed TriageDisposition = "confirmed"
	// TriageDispositionSuppressed marks by-design findings that should be listed separately.
	TriageDispositionSuppressed TriageDisposition = "suppressed"

	// TriageRuleR1 confirms module-surface findings.
	TriageRuleR1 TriageRule = "R1"
	// TriageRuleR2 confirms reverse-shell findings.
	TriageRuleR2 TriageRule = "R2"
	// TriageRuleR3 confirms remote-code-execution findings.
	TriageRuleR3 TriageRule = "R3"
	// TriageRuleR4 confirms content-hash mismatches.
	TriageRuleR4 TriageRule = "R4"
	// TriageRuleR5 confirms symlink escapes.
	TriageRuleR5 TriageRule = "R5"
	// TriageRuleR6 suppresses compound findings built only from suppressed constituents.
	TriageRuleR6 TriageRule = "R6"
	// TriageRuleR7 suppresses root default env-inheritance findings.
	TriageRuleR7 TriageRule = "R7"
	// TriageRuleR8 suppresses root sensitive-env access findings.
	TriageRuleR8 TriageRule = "R8"
	// TriageRuleR9 suppresses root DNS-exfil pattern findings.
	TriageRuleR9 TriageRule = "R9"
	// TriageRuleR10 suppresses root credential-exfil compound findings built from suppressed constituents.
	TriageRuleR10 TriageRule = "R10"
	// TriageRuleR11 suppresses root high-plus-other compound findings built from suppressed constituents.
	TriageRuleR11 TriageRule = "R11"
	// TriageRuleR12 confirms everything else.
	TriageRuleR12 TriageRule = "R12"
)

type (
	// TriageDisposition is the deterministic classification assigned to a finding.
	TriageDisposition string

	// TriageRule identifies the first R1-R12 policy rule that matched a finding.
	TriageRule string

	// TriageRationale describes why the matched policy rule produced the disposition.
	TriageRationale string

	findingTitle string

	// TriagedFinding pairs a finding with its deterministic triage classification.
	TriagedFinding struct {
		finding     Finding
		disposition TriageDisposition
		rule        TriageRule
		rationale   TriageRationale
	}

	// TriagedFindingOption configures NewTriagedFinding.
	TriagedFindingOption func(*TriagedFinding)

	// ReportTriage is the classified audit report split by finding kind.
	ReportTriage struct {
		confirmedFindings    []Finding
		confirmedCorrelated  []Finding
		suppressedFindings   []TriagedFinding
		suppressedCorrelated []TriagedFinding
	}
)

// String returns the string representation of the triage disposition.
func (d TriageDisposition) String() string { return string(d) }

// Validate returns nil when the disposition is recognized.
func (d TriageDisposition) Validate() error { return nil }

// String returns the string representation of the triage rule.
func (r TriageRule) String() string { return string(r) }

// Validate returns nil when the rule is recognized.
func (r TriageRule) Validate() error { return nil }

// String returns the string representation of the triage rationale.
func (r TriageRationale) String() string { return string(r) }

// Validate returns nil when the rationale is set by trusted audit policy code.
func (r TriageRationale) Validate() error { return nil }

func (t findingTitle) String() string { return string(t) }

func (t findingTitle) Validate() error { return nil }

// NewTriagedFinding constructs a triaged finding.
func NewTriagedFinding(opts ...TriagedFindingOption) (TriagedFinding, error) {
	result := TriagedFinding{}
	for _, opt := range opts {
		opt(&result)
	}
	if err := result.Validate(); err != nil {
		return TriagedFinding{}, err
	}
	return result, nil
}

// WithFinding sets the classified audit finding.
func WithFinding(finding Finding) TriagedFindingOption {
	return func(f *TriagedFinding) {
		f.finding = finding
	}
}

// WithDisposition sets the triage disposition.
func WithDisposition(disposition TriageDisposition) TriagedFindingOption {
	return func(f *TriagedFinding) {
		f.disposition = disposition
	}
}

// WithRule sets the triage rule.
func WithRule(rule TriageRule) TriagedFindingOption {
	return func(f *TriagedFinding) {
		f.rule = rule
	}
}

// WithRationale sets the triage rationale.
func WithRationale(rationale TriageRationale) TriagedFindingOption {
	return func(f *TriagedFinding) {
		f.rationale = rationale
	}
}

// Validate returns nil when triage metadata is internally consistent.
func (f TriagedFinding) Validate() error {
	return errors.Join(
		f.disposition.Validate(),
		f.rule.Validate(),
		f.rationale.Validate(),
	)
}

// Finding returns the classified audit finding.
func (f TriagedFinding) Finding() Finding { return f.finding }

// Disposition returns the deterministic triage disposition.
func (f TriagedFinding) Disposition() TriageDisposition { return f.disposition }

// Rule returns the deterministic triage rule that matched.
func (f TriagedFinding) Rule() TriageRule { return f.rule }

// Rationale returns the deterministic triage rationale.
func (f TriagedFinding) Rationale() TriageRationale { return f.rationale }

// NewReportTriage constructs an empty report triage result.
func NewReportTriage() (ReportTriage, error) {
	result := ReportTriage{}
	if err := result.Validate(); err != nil {
		return ReportTriage{}, err
	}
	return result, nil
}

// Validate returns nil when all suppressed entries contain valid triage metadata.
func (t ReportTriage) Validate() error {
	var errs []error
	for i := range t.suppressedFindings {
		errs = append(errs, t.suppressedFindings[i].Validate())
	}
	for i := range t.suppressedCorrelated {
		errs = append(errs, t.suppressedCorrelated[i].Validate())
	}
	return errors.Join(errs...)
}

// ClassifyReportFindings applies the deterministic R1-R12 triage policy to an audit report.
func ClassifyReportFindings(report *Report) ReportTriage {
	if report == nil {
		return ReportTriage{}
	}

	suppressedCodes := make(map[FindingCode]bool)
	suppressedTitles := make(map[findingTitle]bool)
	triage := ReportTriage{}

	for i := range report.Findings {
		classified := classifyFinding(report.Findings[i], suppressedCodes, suppressedTitles)
		if classified.Disposition() == TriageDispositionSuppressed {
			triage.suppressedFindings = append(triage.suppressedFindings, classified)
			rememberSuppressedFinding(classified.Finding(), suppressedCodes, suppressedTitles)
			continue
		}
		triage.confirmedFindings = append(triage.confirmedFindings, classified.Finding())
	}

	for i := range report.Correlated {
		classified := classifyFinding(report.Correlated[i], suppressedCodes, suppressedTitles)
		if classified.Disposition() == TriageDispositionSuppressed {
			triage.suppressedCorrelated = append(triage.suppressedCorrelated, classified)
			rememberSuppressedFinding(classified.Finding(), suppressedCodes, suppressedTitles)
			continue
		}
		triage.confirmedCorrelated = append(triage.confirmedCorrelated, classified.Finding())
	}

	sortFindings(triage.confirmedFindings)
	sortFindings(triage.confirmedCorrelated)
	sortTriagedFindings(triage.suppressedFindings)
	sortTriagedFindings(triage.suppressedCorrelated)
	return triage
}

// HasConfirmedFindings returns true if any confirmed finding meets the severity threshold.
func (t ReportTriage) HasConfirmedFindings(minSev Severity) bool {
	for i := range t.confirmedFindings {
		if t.confirmedFindings[i].Severity >= minSev {
			return true
		}
	}
	for i := range t.confirmedCorrelated {
		if t.confirmedCorrelated[i].Severity >= minSev {
			return true
		}
	}
	return false
}

// ConfirmedFindingsBySeverity returns confirmed individual findings at or above the threshold.
func (t ReportTriage) ConfirmedFindingsBySeverity(minSev Severity) []Finding {
	return filterFindingsBySeverity(t.confirmedFindings, minSev)
}

// ConfirmedCompoundThreatsBySeverity returns confirmed compound findings at or above the threshold.
func (t ReportTriage) ConfirmedCompoundThreatsBySeverity(minSev Severity) []Finding {
	return filterFindingsBySeverity(t.confirmedCorrelated, minSev)
}

// SuppressedFindingsBySeverity returns suppressed individual findings at or above the threshold.
func (t ReportTriage) SuppressedFindingsBySeverity(minSev Severity) []TriagedFinding {
	return filterTriagedFindingsBySeverity(t.suppressedFindings, minSev)
}

// SuppressedCompoundThreatsBySeverity returns suppressed compound findings at or above the threshold.
func (t ReportTriage) SuppressedCompoundThreatsBySeverity(minSev Severity) []TriagedFinding {
	return filterTriagedFindingsBySeverity(t.suppressedCorrelated, minSev)
}

func classifyFinding(finding Finding, suppressedCodes map[FindingCode]bool, suppressedTitles map[findingTitle]bool) TriagedFinding {
	if findingIsModuleSurface(finding) {
		return confirmedFinding(finding, TriageRuleR1, "module findings are supply-chain findings")
	}
	if finding.CodeOrDefault() == codeNetworkReverseShell {
		return confirmedFinding(finding, TriageRuleR2, "reverse-shell patterns are always reported")
	}
	if finding.CodeOrDefault() == codeScriptRemoteExecution {
		return confirmedFinding(finding, TriageRuleR3, "download-and-execute patterns are always reported")
	}
	if finding.CodeOrDefault() == codeLockfileContentHashMismatch {
		return confirmedFinding(finding, TriageRuleR4, "content-hash mismatches are tamper indicators")
	}
	if finding.CodeOrDefault() == codeSymlinkEscapesRoot {
		return confirmedFinding(finding, TriageRuleR5, "symlink escapes cross the module boundary")
	}

	if findingIsRootInvowkfile(finding) {
		if finding.CodeOrDefault() == codeEnvInheritDefaultAll || finding.Title == "Command uses default env inheritance (all host variables)" {
			return suppressedFinding(finding, TriageRuleR7, "root invowkfiles are user-controlled and default env inheritance is expected")
		}
		if finding.CodeOrDefault() == codeEnvSensitiveVar || finding.Title == "Script accesses sensitive environment variable" {
			return suppressedFinding(finding, TriageRuleR8, "root invowkfiles may intentionally forward credentials")
		}
		if finding.CodeOrDefault() == codeNetworkDNSExfiltration || finding.Title == "Possible DNS exfiltration pattern" {
			return suppressedFinding(finding, TriageRuleR9, "root invowkfile container commands may legitimately use DNS")
		}
		if compoundConstituentsSuppressed(finding, suppressedCodes, suppressedTitles) {
			switch {
			case finding.CodeOrDefault() == codeCorrelatorCredentialExfiltration || finding.Title == "Potential credential exfiltration":
				return suppressedFinding(finding, TriageRuleR10, "credential-exfiltration compound contains only suppressed constituents")
			case finding.CodeOrDefault() == codeCorrelatorHighPlusOther || finding.Title == "High-severity finding combined with other issues":
				return suppressedFinding(finding, TriageRuleR11, "generic escalation contains only suppressed constituents")
			default:
				return suppressedFinding(finding, TriageRuleR6, "compound finding contains only suppressed constituents")
			}
		}
	}

	return confirmedFinding(finding, TriageRuleR12, "default policy is to report")
}

func confirmedFinding(finding Finding, rule TriageRule, rationale TriageRationale) TriagedFinding {
	return TriagedFinding{
		finding:     finding,
		disposition: TriageDispositionConfirmed,
		rule:        rule,
		rationale:   rationale,
	}
}

func suppressedFinding(finding Finding, rule TriageRule, rationale TriageRationale) TriagedFinding {
	return TriagedFinding{
		finding:     finding,
		disposition: TriageDispositionSuppressed,
		rule:        rule,
		rationale:   rationale,
	}
}

func findingIsModuleSurface(finding Finding) bool {
	switch finding.SurfaceKind {
	case SurfaceKindLocalModule, SurfaceKindVendoredModule, SurfaceKindGlobalModule:
		return true
	case "", SurfaceKindRootInvowkfile:
		return false
	default:
		return false
	}
}

func findingIsRootInvowkfile(finding Finding) bool {
	return finding.SurfaceKind == "" || finding.SurfaceKind == SurfaceKindRootInvowkfile
}

func compoundConstituentsSuppressed(finding Finding, suppressedCodes map[FindingCode]bool, suppressedTitles map[findingTitle]bool) bool {
	if len(finding.EscalatedFromCodes) > 0 {
		for _, code := range finding.EscalatedFromCodes {
			if !suppressedCodes[code] {
				return false
			}
		}
		return true
	}
	if len(finding.EscalatedFrom) == 0 {
		return false
	}
	for _, title := range finding.EscalatedFrom {
		key := findingTitle(title) //goplint:ignore -- correlator titles are produced by trusted checker output.
		if !suppressedTitles[key] {
			return false
		}
	}
	return true
}

func rememberSuppressedFinding(finding Finding, suppressedCodes map[FindingCode]bool, suppressedTitles map[findingTitle]bool) {
	suppressedCodes[finding.CodeOrDefault()] = true
	if finding.Title != "" {
		key := findingTitle(finding.Title) //goplint:ignore -- finding titles are produced by trusted checker output.
		suppressedTitles[key] = true
	}
}

func filterFindingsBySeverity(findings []Finding, minSev Severity) []Finding {
	filtered := slices.Clone(findings)
	filtered = slices.DeleteFunc(filtered, func(finding Finding) bool {
		return finding.Severity < minSev
	})
	sortFindings(filtered)
	return filtered
}

func filterTriagedFindingsBySeverity(findings []TriagedFinding, minSev Severity) []TriagedFinding {
	filtered := slices.Clone(findings)
	filtered = slices.DeleteFunc(filtered, func(finding TriagedFinding) bool {
		return finding.finding.Severity < minSev
	})
	sortTriagedFindings(filtered)
	return filtered
}

func sortTriagedFindings(findings []TriagedFinding) {
	slices.SortFunc(findings, func(a, b TriagedFinding) int {
		if c := cmp.Compare(b.finding.Severity, a.finding.Severity); c != 0 {
			return c
		}
		if c := cmp.Compare(string(a.finding.FilePath), string(b.finding.FilePath)); c != 0 {
			return c
		}
		if c := cmp.Compare(a.finding.CheckerName, b.finding.CheckerName); c != 0 {
			return c
		}
		if c := cmp.Compare(a.finding.Title, b.finding.Title); c != 0 {
			return c
		}
		return cmp.Compare(a.rule, b.rule)
	})
}
