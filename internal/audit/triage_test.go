// SPDX-License-Identifier: MPL-2.0

package audit

import "testing"

func TestClassifyReportFindingsAppliesDeterministicPolicy(t *testing.T) {
	t.Parallel()

	report := &Report{
		Findings: []Finding{
			triageFinding(codeEnvInheritDefaultAll, SurfaceKindRootInvowkfile, "Command uses default env inheritance (all host variables)"),
			triageFinding(codeEnvSensitiveVar, SurfaceKindRootInvowkfile, "Script accesses sensitive environment variable"),
			triageFinding(codeNetworkDNSExfiltration, SurfaceKindRootInvowkfile, "Possible DNS exfiltration pattern"),
			triageFinding(codeNetworkAccessCommand, SurfaceKindLocalModule, "Script uses network access command"),
			triageFinding(codeNetworkReverseShell, SurfaceKindRootInvowkfile, "Reverse shell pattern detected"),
			triageFinding(codeScriptRemoteExecution, SurfaceKindRootInvowkfile, "Script downloads and executes remote code"),
			triageFinding(codeLockfileContentHashMismatch, SurfaceKindRootInvowkfile, "Module content hash mismatch"),
			triageFinding(codeSymlinkEscapesRoot, SurfaceKindRootInvowkfile, "Symlink points outside module boundary"),
			triageFinding(codeNetworkEncodedURL, SurfaceKindRootInvowkfile, "Script contains encoded URL"),
		},
		Correlated: []Finding{
			{
				Code:               codeCorrelatorCredentialExfiltration,
				Severity:           SeverityCritical,
				Category:           CategoryExfiltration,
				SurfaceKind:        SurfaceKindRootInvowkfile,
				CheckerName:        "correlator",
				Title:              "Potential credential exfiltration",
				EscalatedFromCodes: []FindingCode{codeEnvInheritDefaultAll, codeEnvSensitiveVar},
			},
			{
				Code:               codeCorrelatorHighPlusOther,
				Severity:           SeverityCritical,
				Category:           CategoryTrust,
				SurfaceKind:        SurfaceKindRootInvowkfile,
				CheckerName:        "correlator",
				Title:              "High-severity finding combined with other issues",
				EscalatedFromCodes: []FindingCode{codeEnvInheritDefaultAll, codeNetworkDNSExfiltration},
			},
			{
				Code:               codeCorrelatorMultipleCategories,
				Severity:           SeverityCritical,
				Category:           CategoryTrust,
				SurfaceKind:        SurfaceKindRootInvowkfile,
				CheckerName:        "correlator",
				Title:              "Multiple security concern categories detected",
				EscalatedFromCodes: []FindingCode{codeEnvInheritDefaultAll, codeNetworkDNSExfiltration},
			},
			{
				Code:               codeCorrelatorObfuscatedExfiltration,
				Severity:           SeverityCritical,
				Category:           CategoryExfiltration,
				SurfaceKind:        SurfaceKindRootInvowkfile,
				CheckerName:        "correlator",
				Title:              "Obfuscated network access detected",
				EscalatedFromCodes: []FindingCode{codeNetworkEncodedURL, codeScriptRemoteExecution},
			},
		},
	}

	triage := ClassifyReportFindings(report)

	assertTriageRule(t, triage.suppressedFindings, codeEnvInheritDefaultAll, TriageRuleR7)
	assertTriageRule(t, triage.suppressedFindings, codeEnvSensitiveVar, TriageRuleR8)
	assertTriageRule(t, triage.suppressedFindings, codeNetworkDNSExfiltration, TriageRuleR9)
	assertTriageRule(t, triage.suppressedCorrelated, codeCorrelatorCredentialExfiltration, TriageRuleR10)
	assertTriageRule(t, triage.suppressedCorrelated, codeCorrelatorHighPlusOther, TriageRuleR11)
	assertTriageRule(t, triage.suppressedCorrelated, codeCorrelatorMultipleCategories, TriageRuleR6)

	assertConfirmedFinding(t, triage.confirmedFindings, codeNetworkAccessCommand)
	assertConfirmedFinding(t, triage.confirmedFindings, codeNetworkReverseShell)
	assertConfirmedFinding(t, triage.confirmedFindings, codeScriptRemoteExecution)
	assertConfirmedFinding(t, triage.confirmedFindings, codeLockfileContentHashMismatch)
	assertConfirmedFinding(t, triage.confirmedFindings, codeSymlinkEscapesRoot)
	assertConfirmedFinding(t, triage.confirmedFindings, codeNetworkEncodedURL)
	assertConfirmedFinding(t, triage.confirmedCorrelated, codeCorrelatorObfuscatedExfiltration)
}

func TestReportTriageHasConfirmedFindingsUsesConfirmedBucketsOnly(t *testing.T) {
	t.Parallel()

	report := &Report{
		Findings: []Finding{
			triageFinding(codeEnvInheritDefaultAll, SurfaceKindRootInvowkfile, "Command uses default env inheritance (all host variables)"),
		},
	}

	triage := ClassifyReportFindings(report)
	if triage.HasConfirmedFindings(SeverityLow) {
		t.Fatal("HasConfirmedFindings() = true for suppressed-only report")
	}

	triage.confirmedFindings = append(triage.confirmedFindings, triageFinding(codeNetworkAccessCommand, SurfaceKindRootInvowkfile, "Script uses network access command"))
	if !triage.HasConfirmedFindings(SeverityLow) {
		t.Fatal("HasConfirmedFindings() = false after confirmed finding was added")
	}
}

func triageFinding(code FindingCode, surfaceKind SurfaceKind, title string) Finding {
	return Finding{
		Code:        code,
		Severity:    SeverityHigh,
		Category:    CategoryExfiltration,
		SurfaceKind: surfaceKind,
		CheckerName: "test",
		Title:       title,
	}
}

func assertTriageRule(t *testing.T, findings []TriagedFinding, code FindingCode, rule TriageRule) {
	t.Helper()
	for i := range findings {
		if findings[i].Finding().CodeOrDefault() == code {
			if findings[i].Disposition() != TriageDispositionSuppressed {
				t.Fatalf("%s disposition = %s, want suppressed", code, findings[i].Disposition())
			}
			if findings[i].Rule() != rule {
				t.Fatalf("%s rule = %s, want %s", code, findings[i].Rule(), rule)
			}
			return
		}
	}
	t.Fatalf("suppressed finding %s not found", code)
}

func assertConfirmedFinding(t *testing.T, findings []Finding, code FindingCode) {
	t.Helper()
	for i := range findings {
		if findings[i].CodeOrDefault() == code {
			return
		}
	}
	t.Fatalf("confirmed finding %s not found", code)
}
