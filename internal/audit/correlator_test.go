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
		{Severity: SeverityHigh, Category: CategoryExecution, SurfaceID: "mod1"},
		{Severity: SeverityLow, Category: CategoryTrust, SurfaceID: "mod1"},
	}
	result := c.Correlate(findings)

	hasEscalation := false
	for _, f := range result {
		if f.Title == "High-severity finding combined with other issues" {
			hasEscalation = true
			if f.Severity != SeverityCritical {
				t.Errorf("escalation severity = %v, want Critical", f.Severity)
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
