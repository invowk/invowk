// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/issue"
)

func TestFormatActionableError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *issue.ActionableError
		verbose  bool
		contains []string
		excludes []string
	}{
		{
			name: "simple error non-verbose",
			err: issue.NewErrorContext().
				WithOperation("load config").
				Build(),
			contains: []string{"failed to load config"},
		},
		{
			name: "error with suggestions",
			err: issue.NewErrorContext().
				WithOperation("load invowkfile").
				WithResource("./invowkfile.cue").
				WithSuggestions("Run 'invowk init'", "Check file permissions").
				Build(),
			contains: []string{
				"failed to load invowkfile",
				"./invowkfile.cue",
				"• Run 'invowk init'",
				"• Check file permissions",
			},
		},
		{
			name: "error chain in verbose mode",
			err: issue.NewErrorContext().
				WithOperation("parse config").
				Wrap(errors.New("syntax error")).
				Build(),
			verbose: true,
			contains: []string{
				"failed to parse config",
				"Error chain:",
				"1. syntax error",
			},
		},
		{
			name: "no error chain in non-verbose",
			err: issue.NewErrorContext().
				WithOperation("parse config").
				Wrap(errors.New("syntax error")).
				Build(),
			contains: []string{"failed to parse config: syntax error"},
			excludes: []string{"Error chain:"},
		},
		{
			name: "nested error chain verbose",
			err: issue.NewErrorContext().
				WithOperation("execute command").
				Wrap(issue.NewErrorContext().
					WithOperation("load script").
					Wrap(errors.New("file not found")).
					Build()).
				Build(),
			verbose: true,
			contains: []string{
				"Error chain:",
				"1. failed to load script: file not found",
				"2. file not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatActionableError(tt.err, tt.verbose).String()

			for _, expected := range tt.contains {
				if !strings.Contains(got, expected) {
					t.Errorf("formatActionableError() missing %q\ngot:\n%s", expected, got)
				}
			}

			for _, excluded := range tt.excludes {
				if strings.Contains(got, excluded) {
					t.Errorf("formatActionableError() should not contain %q\ngot:\n%s", excluded, got)
				}
			}
		})
	}
}

func TestIssueCatalogMarkdown(t *testing.T) {
	t.Parallel()

	catalogEntry := issue.Get(issue.InvowkfileNotFoundId)
	if catalogEntry == nil {
		t.Fatal("Get(InvowkfileNotFoundId) returned nil")
	}

	got := issueCatalogMarkdown(catalogEntry).String()
	if got == "" {
		t.Fatal("issueCatalogMarkdown() returned empty string")
	}
	if !strings.Contains(got, "invowkfile") {
		t.Errorf("issueCatalogMarkdown() should contain issue content, got: %s", got)
	}
}
