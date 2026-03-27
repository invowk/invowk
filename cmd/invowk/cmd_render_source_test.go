// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ---------------------------------------------------------------------------
// SourceNotFoundError / AmbiguousCommandError — Error() method tests
// ---------------------------------------------------------------------------

func TestSourceNotFoundError_Error(t *testing.T) {
	t.Parallel()

	err := &SourceNotFoundError{
		Source:           "mymodule",
		AvailableSources: []discovery.SourceID{discovery.SourceIDInvowkfile, "other"},
	}

	got := err.Error()
	if !strings.Contains(got, "mymodule") {
		t.Errorf("Error() = %q, want it to contain source name 'mymodule'", got)
	}
	if !strings.Contains(got, "not found") {
		t.Errorf("Error() = %q, want it to contain 'not found'", got)
	}
}

func TestAmbiguousCommandError_Error(t *testing.T) {
	t.Parallel()

	err := &AmbiguousCommandError{
		CommandName: "deploy",
		Sources:     []discovery.SourceID{discovery.SourceIDInvowkfile, "mymodule"},
	}

	got := err.Error()
	if !strings.Contains(got, "deploy") {
		t.Errorf("Error() = %q, want it to contain command name 'deploy'", got)
	}
	if !strings.Contains(got, "ambiguous") {
		t.Errorf("Error() = %q, want it to contain 'ambiguous'", got)
	}
}

// ---------------------------------------------------------------------------
// RenderSourceNotFoundError tests
// ---------------------------------------------------------------------------

func TestRenderSourceNotFoundError_WithAvailableSources(t *testing.T) {
	t.Parallel()

	err := &SourceNotFoundError{
		Source:           "nonexistent",
		AvailableSources: []discovery.SourceID{discovery.SourceIDInvowkfile, "mymodule"},
	}

	output := RenderSourceNotFoundError(err)

	for _, want := range []string{
		"Source not found",
		"'nonexistent'",
		"Available sources:",
		// formatSourceDisplayName adds ".invowkmod" suffix for module sources
		"mymodule.invowkmod",
		"@<source>",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("RenderSourceNotFoundError() missing %q in output:\n%s", want, output)
		}
	}
}

func TestRenderSourceNotFoundError_EmptySources(t *testing.T) {
	t.Parallel()

	err := &SourceNotFoundError{
		Source:           "missing",
		AvailableSources: nil,
	}

	output := RenderSourceNotFoundError(err)

	if !strings.Contains(output, "(none)") {
		t.Error("RenderSourceNotFoundError() should show '(none)' when no sources are available")
	}
	if !strings.Contains(output, "'missing'") {
		t.Error("RenderSourceNotFoundError() should contain the requested source name")
	}
}

func TestRenderSourceNotFoundError_InvowkfileSource(t *testing.T) {
	t.Parallel()

	err := &SourceNotFoundError{
		Source:           "wrong",
		AvailableSources: []discovery.SourceID{discovery.SourceIDInvowkfile},
	}

	output := RenderSourceNotFoundError(err)

	// The invowkfile source should be displayed as-is (no .invowkmod suffix)
	if !strings.Contains(output, string(discovery.SourceIDInvowkfile)) {
		t.Errorf("RenderSourceNotFoundError() should display invowkfile source ID, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// RenderAmbiguousCommandError tests
// ---------------------------------------------------------------------------

func TestRenderAmbiguousCommandError_MultipleSources(t *testing.T) {
	t.Parallel()

	err := &AmbiguousCommandError{
		CommandName: "deploy",
		Sources:     []discovery.SourceID{discovery.SourceIDInvowkfile, "mymodule"},
	}

	output := RenderAmbiguousCommandError(err)

	for _, want := range []string{
		"Ambiguous command",
		"'deploy'",
		"multiple sources",
		"@" + string(discovery.SourceIDInvowkfile),
		"@mymodule",
		"--ivk-from",
		"invowk cmd",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("RenderAmbiguousCommandError() missing %q in output:\n%s", want, output)
		}
	}
}

func TestRenderAmbiguousCommandError_ShowsExampleWithFirstSource(t *testing.T) {
	t.Parallel()

	err := &AmbiguousCommandError{
		CommandName: "build",
		Sources:     []discovery.SourceID{"alpha", "beta"},
	}

	output := RenderAmbiguousCommandError(err)

	// The example command should use the first source
	if !strings.Contains(output, "@alpha") {
		t.Error("RenderAmbiguousCommandError() example should use the first source")
	}
	if !strings.Contains(output, "build") {
		t.Error("RenderAmbiguousCommandError() example should include the command name")
	}
}

// ---------------------------------------------------------------------------
// formatSourceDisplayName tests
// ---------------------------------------------------------------------------

func TestFormatSourceDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		sourceID discovery.SourceID
		want     string
	}{
		{
			name:     "invowkfile source returned as-is",
			sourceID: discovery.SourceIDInvowkfile,
			want:     string(discovery.SourceIDInvowkfile),
		},
		{
			name:     "module source gets .invowkmod suffix",
			sourceID: "mymodule",
			want:     "mymodule.invowkmod",
		},
		{
			name:     "RDNS module source gets .invowkmod suffix",
			sourceID: "com.example.tools",
			want:     "com.example.tools.invowkmod",
		},
		{
			// Space-containing IDs are returned as-is because .invowkmod suffixing
			// is only for filesystem-based module names (no spaces in directory names).
			name:     "space-containing module ID returned as-is",
			sourceID: "my module",
			want:     "my module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatSourceDisplayName(tt.sourceID)
			if got != tt.want {
				t.Errorf("formatSourceDisplayName(%q) = %q, want %q", tt.sourceID, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildCobraArgsValidator tests
// ---------------------------------------------------------------------------

func TestBuildCobraArgsValidator_NoArgs(t *testing.T) {
	t.Parallel()

	validator := buildCobraArgsValidator(nil)
	dummyCmd := &cobra.Command{Use: "test-cmd"}

	// With no arg definitions, any number of arguments should be accepted
	if err := validator(dummyCmd, []string{"anything", "goes"}); err != nil {
		t.Errorf("buildCobraArgsValidator(nil) should accept any args, got error: %v", err)
	}
	if err := validator(dummyCmd, []string{}); err != nil {
		t.Errorf("buildCobraArgsValidator(nil) should accept empty args, got error: %v", err)
	}
}

func TestBuildCobraArgsValidator_WithArgs_Valid(t *testing.T) {
	t.Parallel()

	argDefs := []invowkfile.Argument{
		{Name: "env", Description: "Target environment", Required: true},
	}

	validator := buildCobraArgsValidator(argDefs)
	dummyCmd := &cobra.Command{Use: "deploy"}

	if err := validator(dummyCmd, []string{"production"}); err != nil {
		t.Errorf("buildCobraArgsValidator(argDefs) should accept valid args, got error: %v", err)
	}
}

func TestBuildCobraArgsValidator_WithArgs_MissingRequired(t *testing.T) {
	t.Parallel()

	argDefs := []invowkfile.Argument{
		{Name: "env", Description: "Target environment", Required: true},
	}

	validator := buildCobraArgsValidator(argDefs)
	dummyCmd := &cobra.Command{Use: "deploy"}

	if err := validator(dummyCmd, []string{}); err == nil {
		t.Error("buildCobraArgsValidator(argDefs) should reject missing required args")
	}
}
