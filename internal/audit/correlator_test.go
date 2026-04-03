// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"testing"
)

func TestCorrelator_NoFindings(t *testing.T) {
	t.Parallel()

	c := NewCorrelator(DefaultRules())
	result := c.Correlate(nil)
	if len(result) != 0 {
		t.Errorf("Correlate(nil) len = %d, want 0", len(result))
	}
}

func TestCorrelator_SingleFindingNoCorrelation(t *testing.T) {
	t.Parallel()

	c := NewCorrelator(DefaultRules())
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

	c := NewCorrelator(DefaultRules())
	findings := []Finding{
		{Severity: SeverityHigh, Category: CategoryObfuscation, SurfaceID: "mod1", CheckerName: scriptCheckerName, Title: "obfuscation"},
		{Severity: SeverityHigh, Category: CategoryExfiltration, SurfaceID: "mod1", CheckerName: networkCheckerName, Title: "network"},
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
		}
	}
	if !hasObfuscatedExfil {
		t.Error("expected obfuscated-exfiltration compound finding")
	}
}

func TestCorrelator_TrustChainWeakness(t *testing.T) {
	t.Parallel()

	c := NewCorrelator(DefaultRules())
	findings := []Finding{
		{Severity: SeverityMedium, Category: CategoryTrust, SurfaceID: "mod1", CheckerName: moduleMetadataCheckerName, Title: "deep deps"},
		{Severity: SeverityMedium, Category: CategoryIntegrity, SurfaceID: "mod1", CheckerName: lockFileCheckerName, Title: "missing hash"},
	}
	result := c.Correlate(findings)

	hasTrustChain := false
	for _, f := range result {
		if f.Title == "Trust chain weakness — deep deps with missing integrity" {
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

func TestCorrelator_MediumMediumEscalation(t *testing.T) {
	t.Parallel()

	c := NewCorrelator(nil) // No named rules, only escalation.
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

	c := NewCorrelator(nil)
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

	c := NewCorrelator(nil)
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

	c := NewCorrelator(DefaultRules())
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

	c := NewCorrelator(nil)
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
