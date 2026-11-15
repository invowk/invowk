// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"testing"

	"github.com/invowk/invowk/internal/discovery"
)

// ---------------------------------------------------------------------------
// Source filter tests
// ---------------------------------------------------------------------------

func TestNormalizeSourceName(t *testing.T) {
	t.Parallel()

	// Test T009: normalizeSourceName helper
	tests := []struct {
		input    string
		expected discovery.SourceID
	}{
		// Module names
		{"foo", "foo"},
		{"foo.invowkmod", "foo"},
		{"@foo", "foo"},
		{"@foo.invowkmod", "foo"},
		{"bar", "bar"},
		{"bar.invowkmod", "bar"},

		// Invowkfile variants
		{"invowkfile", "invowkfile"},
		{"invowkfile.cue", "invowkfile"},
		{"@invowkfile", "invowkfile"},
		{"@invowkfile.cue", "invowkfile"},

		// RDNS module names
		{"com.example.mytools", "com.example.mytools"},
		{"com.example.mytools.invowkmod", "com.example.mytools"},
		{"@com.example.mytools.invowkmod", "com.example.mytools"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			result := normalizeSourceName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSourceName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseSourceFilter_FromFlag(t *testing.T) {
	t.Parallel()

	// Test T008: ParseSourceFilter with --ivk-from flag
	args := []string{"deploy", "arg1"}

	filter, remaining, err := ParseSourceFilter(args, "foo")
	if err != nil {
		t.Fatalf("ParseSourceFilter() error: %v", err)
	}

	if filter == nil {
		t.Fatal("filter should not be nil when --ivk-from is specified")
	}
	if filter.SourceID != "foo" {
		t.Errorf("SourceID = %q, want %q", filter.SourceID, "foo")
	}

	// Args should be unchanged
	if len(remaining) != 2 || remaining[0] != "deploy" {
		t.Errorf("remaining args = %v, want [deploy arg1]", remaining)
	}
}

func TestParseSourceFilter_AtPrefix(t *testing.T) {
	t.Parallel()

	// Test T008: ParseSourceFilter with @source prefix
	args := []string{"@foo", "deploy", "arg1"}

	filter, remaining, err := ParseSourceFilter(args, "")
	if err != nil {
		t.Fatalf("ParseSourceFilter() error: %v", err)
	}

	if filter == nil {
		t.Fatal("filter should not be nil when @source is specified")
	}
	if filter.SourceID != "foo" {
		t.Errorf("SourceID = %q, want %q", filter.SourceID, "foo")
	}

	// @source should be consumed from args
	if len(remaining) != 2 || remaining[0] != "deploy" {
		t.Errorf("remaining args = %v, want [deploy arg1]", remaining)
	}
}

func TestParseSourceFilter_NoFilter(t *testing.T) {
	t.Parallel()

	// Test T008: ParseSourceFilter with no filter
	args := []string{"deploy", "arg1"}

	filter, remaining, err := ParseSourceFilter(args, "")
	if err != nil {
		t.Fatalf("ParseSourceFilter() error: %v", err)
	}

	if filter != nil {
		t.Error("filter should be nil when no filter is specified")
	}

	// Args should be unchanged
	if len(remaining) != 2 || remaining[0] != "deploy" {
		t.Errorf("remaining args = %v, want [deploy arg1]", remaining)
	}
}

func TestParseSourceFilter_FromFlagTakesPrecedence(t *testing.T) {
	t.Parallel()

	// Test T008: --ivk-from flag takes precedence over @prefix
	args := []string{"@bar", "deploy"}

	filter, remaining, err := ParseSourceFilter(args, "foo")
	if err != nil {
		t.Fatalf("ParseSourceFilter() error: %v", err)
	}

	if filter == nil {
		t.Fatal("filter should not be nil")
	}
	// --ivk-from foo should take precedence over @bar
	if filter.SourceID != "foo" {
		t.Errorf("SourceID = %q, want %q (--ivk-from takes precedence)", filter.SourceID, "foo")
	}

	// @bar should NOT be consumed since --ivk-from was used
	if len(remaining) != 2 || remaining[0] != "@bar" {
		t.Errorf("remaining args = %v, want [@bar deploy]", remaining)
	}
}

func TestParseSourceFilter_EmptyArgs(t *testing.T) {
	t.Parallel()

	// Test T008: ParseSourceFilter with empty args
	filter, remaining, err := ParseSourceFilter([]string{}, "")
	if err != nil {
		t.Fatalf("ParseSourceFilter() error: %v", err)
	}

	if filter != nil {
		t.Error("filter should be nil for empty args")
	}
	if len(remaining) != 0 {
		t.Errorf("remaining should be empty, got %v", remaining)
	}
}

func TestParseSourceFilter_MalformedSourceIsUsageError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		fromFlag string
	}{
		{name: "path after @", args: []string{"@./m.invowkmod", "w2"}},
		{name: "bare @", args: []string{"@", "w2"}},
		{name: "path in --ivk-from", args: []string{"w2"}, fromFlag: "./m.invowkmod"},
		{name: "leading digit in --ivk-from", args: []string{"hello"}, fromFlag: "1bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			filter, _, err := ParseSourceFilter(tt.args, tt.fromFlag)
			if filter != nil {
				t.Fatalf("ParseSourceFilter() filter = %+v, want nil", filter)
			}
			if got := exitCodeForError(err); got != ExitCodeUsage {
				t.Fatalf("exit code = %d (error: %v), want %d", got, err, ExitCodeUsage)
			}
		})
	}
}

func TestParseSourceFilter_NormalizesValidForms(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"@m", "m", "m.invowkmod", "@m.invowkmod"} {
		filter, _, err := ParseSourceFilter([]string{"w2"}, raw)
		if err != nil {
			t.Fatalf("ParseSourceFilter(--ivk-from %q) error = %v", raw, err)
		}
		if filter.SourceID != "m" {
			t.Fatalf("ParseSourceFilter(--ivk-from %q) = %q, want %q", raw, filter.SourceID, "m")
		}
	}
}
