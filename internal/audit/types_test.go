// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"encoding/json"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestSeverity_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sev  Severity
		want string
	}{
		{SeverityInfo, "info"},
		{SeverityLow, "low"},
		{SeverityMedium, "medium"},
		{SeverityHigh, "high"},
		{SeverityCritical, "critical"},
		{Severity(99), "severity(99)"},
	}

	for _, tt := range tests {
		if got := tt.sev.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.sev, got, tt.want)
		}
	}
}

func TestSeverity_Validate(t *testing.T) {
	t.Parallel()

	valid := []Severity{SeverityInfo, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical}
	for _, sev := range valid {
		if err := sev.Validate(); err != nil {
			t.Errorf("Severity(%d).Validate() = %v, want nil", sev, err)
		}
	}

	if err := Severity(99).Validate(); err == nil {
		t.Error("Severity(99).Validate() = nil, want error")
	}
}

func TestSeverity_Ordering(t *testing.T) {
	t.Parallel()

	if SeverityInfo >= SeverityLow {
		t.Error("SeverityInfo should be < SeverityLow")
	}
	if SeverityLow >= SeverityMedium {
		t.Error("SeverityLow should be < SeverityMedium")
	}
	if SeverityMedium >= SeverityHigh {
		t.Error("SeverityMedium should be < SeverityHigh")
	}
	if SeverityHigh >= SeverityCritical {
		t.Error("SeverityHigh should be < SeverityCritical")
	}
}

func TestParseSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    Severity
		wantErr bool
	}{
		{"info", SeverityInfo, false},
		{"low", SeverityLow, false},
		{"medium", SeverityMedium, false},
		{"high", SeverityHigh, false},
		{"critical", SeverityCritical, false},
		{"HIGH", SeverityHigh, false},
		{"  Medium  ", SeverityMedium, false},
		{"unknown", SeverityInfo, true},
		{"", SeverityInfo, true},
	}

	for _, tt := range tests {
		got, err := ParseSeverity(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseSeverity(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseSeverity(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSeverity_JSON(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(SeverityHigh)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"high"` {
		t.Errorf("Marshal = %s, want %q", data, "high")
	}

	var got Severity
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != SeverityHigh {
		t.Errorf("Unmarshal = %v, want %v", got, SeverityHigh)
	}
}

func TestCategory_Validate(t *testing.T) {
	t.Parallel()

	valid := []Category{
		CategoryIntegrity, CategoryPathTraversal, CategoryExfiltration,
		CategoryExecution, CategoryTrust, CategoryObfuscation,
	}
	for _, cat := range valid {
		if err := cat.Validate(); err != nil {
			t.Errorf("Category(%q).Validate() = %v, want nil", cat, err)
		}
	}

	if err := Category("unknown").Validate(); err == nil {
		t.Error("Category(unknown).Validate() = nil, want error")
	}
}

func TestReport_AllFindings(t *testing.T) {
	t.Parallel()

	report := &Report{
		Findings: []Finding{
			{Severity: SeverityLow, Title: "low1", FilePath: "b.go"},
			{Severity: SeverityHigh, Title: "high1", FilePath: "a.go"},
		},
		Correlated: []Finding{
			{Severity: SeverityCritical, Title: "crit1", FilePath: "c.go"},
		},
	}

	all := report.AllFindings()
	if len(all) != 3 {
		t.Fatalf("AllFindings() len = %d, want 3", len(all))
	}
	// Sorted by severity desc, then path asc.
	if all[0].Severity != SeverityCritical {
		t.Errorf("all[0].Severity = %v, want Critical", all[0].Severity)
	}
	if all[1].Severity != SeverityHigh {
		t.Errorf("all[1].Severity = %v, want High", all[1].Severity)
	}
	if all[2].Severity != SeverityLow {
		t.Errorf("all[2].Severity = %v, want Low", all[2].Severity)
	}
}

func TestReport_FilterBySeverity(t *testing.T) {
	t.Parallel()

	report := &Report{
		Findings: []Finding{
			{Severity: SeverityInfo, Title: "info1"},
			{Severity: SeverityLow, Title: "low1"},
			{Severity: SeverityMedium, Title: "med1"},
			{Severity: SeverityHigh, Title: "high1"},
			{Severity: SeverityCritical, Title: "crit1"},
		},
	}

	filtered := report.FilterBySeverity(SeverityMedium)
	if len(filtered) != 3 {
		t.Errorf("FilterBySeverity(Medium) len = %d, want 3", len(filtered))
	}
}

func TestReport_CountBySeverity(t *testing.T) {
	t.Parallel()

	report := &Report{
		Findings: []Finding{
			{Severity: SeverityHigh},
			{Severity: SeverityHigh},
			{Severity: SeverityLow},
		},
		Correlated: []Finding{
			{Severity: SeverityCritical},
		},
	}

	counts := report.CountBySeverity()
	if counts[SeverityHigh] != 2 {
		t.Errorf("counts[High] = %d, want 2", counts[SeverityHigh])
	}
	if counts[SeverityLow] != 1 {
		t.Errorf("counts[Low] = %d, want 1", counts[SeverityLow])
	}
	if counts[SeverityCritical] != 1 {
		t.Errorf("counts[Critical] = %d, want 1", counts[SeverityCritical])
	}
}

func TestReport_MaxSeverity(t *testing.T) {
	t.Parallel()

	empty := &Report{}
	if got := empty.MaxSeverity(); got != SeverityInfo {
		t.Errorf("empty.MaxSeverity() = %v, want Info", got)
	}

	report := &Report{
		Findings: []Finding{
			{Severity: SeverityLow},
			{Severity: SeverityHigh},
		},
	}
	if got := report.MaxSeverity(); got != SeverityHigh {
		t.Errorf("MaxSeverity() = %v, want High", got)
	}
}

func TestReport_HasFindings(t *testing.T) {
	t.Parallel()

	report := &Report{
		Findings: []Finding{
			{Severity: SeverityLow},
			{Severity: SeverityMedium},
		},
	}

	if !report.HasFindings(SeverityLow) {
		t.Error("HasFindings(Low) = false, want true")
	}
	if !report.HasFindings(SeverityMedium) {
		t.Error("HasFindings(Medium) = false, want true")
	}
	if report.HasFindings(SeverityHigh) {
		t.Error("HasFindings(High) = true, want false")
	}
}

func TestReport_EmptyFindings(t *testing.T) {
	t.Parallel()

	report := &Report{}
	if got := report.AllFindings(); len(got) != 0 {
		t.Errorf("AllFindings() len = %d, want 0", len(got))
	}
	if got := report.FilterBySeverity(SeverityInfo); len(got) != 0 {
		t.Errorf("FilterBySeverity() len = %d, want 0", len(got))
	}
	_ = report.CountBySeverity() // Should not panic.
	_ = report.MaxSeverity()
	_ = report.DurationMillis()
	_ = report.HasFindings(SeverityInfo)
}

func TestFinding_FilePath(t *testing.T) {
	t.Parallel()

	f := Finding{
		FilePath: types.FilesystemPath("/test/path.go"),
		Line:     42,
	}
	if f.FilePath != "/test/path.go" {
		t.Errorf("FilePath = %q, want /test/path.go", f.FilePath)
	}
	if f.Line != 42 {
		t.Errorf("Line = %d, want 42", f.Line)
	}
}
