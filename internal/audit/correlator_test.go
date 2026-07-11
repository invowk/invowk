// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"slices"
	"testing"
)

// mustNewCorrelator creates a Correlator or fails the test.
func mustNewCorrelator(t *testing.T, rules []CorrelationRule) *Correlator {
	t.Helper()
	c, err := NewCorrelator(rules)
	if err != nil {
		t.Fatalf("NewCorrelator: %v", err)
	}
	return c
}

func TestCorrelator_NoFindings(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, DefaultRules())
	result := c.Correlate(nil)
	if len(result) != 0 {
		t.Errorf("Correlate(nil) len = %d, want 0", len(result))
	}
}

func TestCorrelator_SingleFindingNoCorrelation(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, DefaultRules())
	findings := []Finding{
		{Severity: SeverityMedium, Category: CategoryExfiltration, SurfaceID: "mod1"},
	}
	result := c.Correlate(findings)
	if len(result) != 0 {
		t.Errorf("Single finding should not trigger correlation, got %d findings", len(result))
	}
}

func TestCorrelator_ObfuscatedExfiltration(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, DefaultRules())
	findings := []Finding{
		{Code: codeScriptRemoteExecution, Severity: SeverityCritical, Category: CategoryExecution, SurfaceID: "mod1", CheckerName: scriptCheckerName, Title: "remote execution"},
		{Code: codeScriptBase64Decode, Severity: SeverityHigh, Category: CategoryObfuscation, SurfaceID: "mod1", CheckerName: scriptCheckerName, Title: "obfuscation"},
		{Code: codeNetworkAccessCommand, Severity: SeverityHigh, Category: CategoryExfiltration, SurfaceID: "mod1", CheckerName: networkCheckerName, Title: "network"},
	}
	result := c.Correlate(findings)

	// Should find the named "obfuscated-exfiltration" rule + "High+any" escalation.
	hasObfuscatedExfil := false
	for _, f := range result {
		if f.Title == "Obfuscated network access detected" {
			hasObfuscatedExfil = true
			if f.Severity != SeverityCritical {
				t.Errorf("obfuscated exfil severity = %v, want Critical", f.Severity)
			}
			if slices.Contains(f.EscalatedFrom, "remote execution") {
				t.Fatalf("EscalatedFrom = %v, want only rule-matching findings", f.EscalatedFrom)
			}
		}
	}
	if !hasObfuscatedExfil {
		t.Error("expected obfuscated-exfiltration compound finding")
	}
}

func TestCorrelator_ObfuscatedExfiltrationRequiresObfuscation(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, DefaultRules())
	findings := []Finding{
		{Code: codeScriptRemoteExecution, Severity: SeverityCritical, Category: CategoryExecution, SurfaceID: "mod1", CheckerName: scriptCheckerName, Title: "remote execution"},
		{Code: codeNetworkAccessCommand, Severity: SeverityHigh, Category: CategoryExfiltration, SurfaceID: "mod1", CheckerName: networkCheckerName, Title: "network"},
	}
	result := c.Correlate(findings)

	for _, f := range result {
		if f.Code == codeCorrelatorObfuscatedExfiltration {
			t.Fatalf("non-obfuscated script finding triggered obfuscated-exfiltration: %+v", f)
		}
	}
}

func TestCorrelator_TrustChainWeakness(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, DefaultRules())
	findings := []Finding{
		{Severity: SeverityMedium, Category: CategoryTrust, SurfaceID: "mod1", CheckerName: moduleMetadataCheckerName, Title: "Wide dependency fan-out"},
		{Severity: SeverityMedium, Category: CategoryIntegrity, SurfaceID: "mod1", CheckerName: lockFileCheckerName, Title: "Module content hash mismatch"},
	}
	result := c.Correlate(findings)

	hasTrustChain := false
	for _, f := range result {
		if f.Title == "Trust chain weakness — dependency graph with missing integrity" {
			hasTrustChain = true
			if f.Severity != SeverityHigh {
				t.Errorf("trust chain severity = %v, want High", f.Severity)
			}
		}
	}
	if !hasTrustChain {
		t.Error("expected trust-chain-weakness compound finding")
	}
}

func TestCorrelator_TrustChainWeaknessIgnoresUnrelatedCheckerFindings(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, DefaultRules())
	findings := []Finding{
		{Severity: SeverityMedium, Category: CategoryTrust, SurfaceID: "mod1", CheckerName: moduleMetadataCheckerName, Title: "Module ID similar to another module"},
		{Severity: SeverityLow, Category: CategoryIntegrity, SurfaceID: "mod1", CheckerName: lockFileCheckerName, Title: "Orphaned lock file entry"},
	}
	result := c.Correlate(findings)

	for _, f := range result {
		if f.Code == "correlator-trust-chain-weakness" {
			t.Fatalf("unrelated module metadata and lockfile findings triggered trust-chain correlation: %+v", f)
		}
	}
}

func TestCorrelator_MediumMediumEscalation(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, nil) // No named rules, only escalation.
	findings := []Finding{
		{Severity: SeverityMedium, Category: CategoryExfiltration, SurfaceID: "mod1"},
		{Severity: SeverityMedium, Category: CategoryExfiltration, SurfaceID: "mod1"},
	}
	result := c.Correlate(findings)

	hasEscalation := false
	for _, f := range result {
		if f.Title == "Multiple medium-severity findings compound risk" {
			hasEscalation = true
			if f.Severity != SeverityHigh {
				t.Errorf("escalation severity = %v, want High", f.Severity)
			}
		}
	}
	if !hasEscalation {
		t.Error("expected Medium+Medium → High escalation")
	}
}

func TestCorrelator_HighPlusAnyEscalation(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, nil)
	findings := []Finding{
		{Code: codeScriptRemoteExecution, Severity: SeverityHigh, Category: CategoryExecution, SurfaceID: "mod1", Title: "remote execution"},
		{Code: codeNetworkAccessCommand, Severity: SeverityLow, Category: CategoryTrust, SurfaceID: "mod1", Title: "network access"},
	}
	result := c.Correlate(findings)

	hasEscalation := false
	for _, f := range result {
		if f.Title == "High-severity finding combined with other issues" {
			hasEscalation = true
			if f.Severity != SeverityCritical {
				t.Errorf("escalation severity = %v, want Critical", f.Severity)
			}
			if !slices.Contains(f.EscalatedFromCodes, codeScriptRemoteExecution) || !slices.Contains(f.EscalatedFromCodes, codeNetworkAccessCommand) {
				t.Fatalf("EscalatedFromCodes = %v, want constituent stable codes", f.EscalatedFromCodes)
			}
		}
	}
	if !hasEscalation {
		t.Error("expected High+any → Critical escalation")
	}
}

func TestCorrelator_ThreeCategoriesEscalation(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, nil)
	findings := []Finding{
		{Severity: SeverityMedium, Category: CategoryExfiltration, SurfaceID: "mod1"},
		{Severity: SeverityLow, Category: CategoryTrust, SurfaceID: "mod1"},
		{Severity: SeverityLow, Category: CategoryObfuscation, SurfaceID: "mod1"},
	}
	result := c.Correlate(findings)

	hasEscalation := false
	for _, f := range result {
		if f.Title == "Multiple security concern categories detected" {
			hasEscalation = true
			if f.Severity != SeverityCritical {
				t.Errorf("escalation severity = %v, want Critical", f.Severity)
			}
		}
	}
	if !hasEscalation {
		t.Error("expected 3+ categories → Critical escalation")
	}
}

func TestCorrelator_DifferentSurfaces(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, DefaultRules())
	findings := []Finding{
		{Severity: SeverityHigh, Category: CategoryObfuscation, SurfaceID: "mod1", CheckerName: scriptCheckerName},
		{Severity: SeverityHigh, Category: CategoryExfiltration, SurfaceID: "mod2", CheckerName: networkCheckerName},
	}
	// Different surfaces should not trigger the obfuscated-exfiltration rule.
	result := c.Correlate(findings)

	for _, f := range result {
		if f.Title == "Obfuscated network access detected" {
			t.Error("should not correlate across different surfaces")
		}
	}
}

func TestCorrelator_CriticalNotEscalated(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, nil)
	findings := []Finding{
		{Severity: SeverityCritical, Category: CategoryExecution, SurfaceID: "mod1"},
		{Severity: SeverityMedium, Category: CategoryTrust, SurfaceID: "mod1"},
	}
	result := c.Correlate(findings)

	// Already critical — should not add more escalation.
	for _, f := range result {
		if f.Title == "High-severity finding combined with other issues" ||
			f.Title == "Multiple medium-severity findings compound risk" {
			t.Errorf("should not escalate when already Critical, got: %s", f.Title)
		}
	}
}

func TestCorrelatorNamedRuleDeduplicatesEscalationInputs(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, DefaultRules())
	findings := []Finding{
		{Code: codeEnvSensitiveVar, Severity: SeverityMedium, Category: CategoryExfiltration, SurfaceID: "mod1", CheckerName: envCheckerName, Title: "sensitive environment"},
		{Code: codeEnvSensitiveVar, Severity: SeverityMedium, Category: CategoryExfiltration, SurfaceID: "mod1", CheckerName: envCheckerName, Title: "sensitive environment"},
		{Code: codeNetworkAccessCommand, Severity: SeverityMedium, Category: CategoryExfiltration, SurfaceID: "mod1", CheckerName: networkCheckerName, Title: "network access"},
		{Code: codeNetworkAccessCommand, Severity: SeverityMedium, Category: CategoryExfiltration, SurfaceID: "mod1", CheckerName: networkCheckerName, Title: "network access"},
	}

	result := c.Correlate(findings)
	for i := range result {
		if result[i].Code != codeCorrelatorCredentialExfiltration {
			continue
		}
		wantTitles := []string{"sensitive environment", "network access"}
		wantCodes := []FindingCode{codeEnvSensitiveVar, codeNetworkAccessCommand}
		if !slices.Equal(result[i].EscalatedFrom, wantTitles) {
			t.Fatalf("EscalatedFrom = %v, want %v", result[i].EscalatedFrom, wantTitles)
		}
		if !slices.Equal(result[i].EscalatedFromCodes, wantCodes) {
			t.Fatalf("EscalatedFromCodes = %v, want %v", result[i].EscalatedFromCodes, wantCodes)
		}
		return
	}
	t.Fatal("credential-exfiltration correlation not found")
}

func TestCorrelatorGenericEscalationDeduplicatesProvenanceIndependently(t *testing.T) {
	t.Parallel()

	c := mustNewCorrelator(t, nil)
	findings := []Finding{
		{Code: "shared-code", Severity: SeverityHigh, Category: CategoryExecution, SurfaceID: "mod1", Title: "first title"},
		{Code: "shared-code", Severity: SeverityLow, Category: CategoryExecution, SurfaceID: "mod1", Title: "first title"},
		{Code: "other-code", Severity: SeverityLow, Category: CategoryExecution, SurfaceID: "mod1", Title: "first title"},
	}

	result := c.Correlate(findings)
	if len(result) != 1 {
		t.Fatalf("Correlate() len = %d, want 1", len(result))
	}
	if !slices.Equal(result[0].EscalatedFrom, []string{"first title"}) {
		t.Fatalf("EscalatedFrom = %v, want one unique title", result[0].EscalatedFrom)
	}
	wantCodes := []FindingCode{"shared-code", "other-code"}
	if !slices.Equal(result[0].EscalatedFromCodes, wantCodes) {
		t.Fatalf("EscalatedFromCodes = %v, want %v", result[0].EscalatedFromCodes, wantCodes)
	}
}
