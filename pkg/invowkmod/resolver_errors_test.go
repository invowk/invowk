// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"strings"
	"testing"
)

func TestMissingTransitiveDepDiagnostic_CUESnippet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		diag     MissingTransitiveDepDiagnostic
		contains []string
	}{
		{
			name: "simple dependency",
			diag: MissingTransitiveDepDiagnostic{
				RequiringModule: "io.example.moduleA",
				RequiringURL:    "https://github.com/example/moduleA.git",
				MissingRef: ModuleRef{
					GitURL:  "https://github.com/example/moduleC.git",
					Version: "^1.0.0",
				},
			},
			contains: []string{
				`git_url: "https://github.com/example/moduleC.git"`,
				`version: "^1.0.0"`,
			},
		},
		{
			name: "with alias",
			diag: MissingTransitiveDepDiagnostic{
				RequiringModule: "io.example.moduleA",
				RequiringURL:    "https://github.com/example/moduleA.git",
				MissingRef: ModuleRef{
					GitURL:  "https://github.com/example/moduleC.git",
					Version: "~2.0.0",
					Alias:   "myalias",
				},
			},
			contains: []string{
				`git_url: "https://github.com/example/moduleC.git"`,
				`version: "~2.0.0"`,
				`alias:   "myalias"`,
			},
		},
		{
			name: "with path",
			diag: MissingTransitiveDepDiagnostic{
				RequiringModule: "io.example.moduleA",
				RequiringURL:    "https://github.com/example/monorepo.git",
				MissingRef: ModuleRef{
					GitURL:  "https://github.com/example/monorepo.git",
					Version: "^1.5.0",
					Path:    "packages/utils",
				},
			},
			contains: []string{
				`git_url: "https://github.com/example/monorepo.git"`,
				`version: "^1.5.0"`,
				`path:    "packages/utils"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			snippet := tt.diag.CUESnippet()
			for _, want := range tt.contains {
				if !strings.Contains(snippet, want) {
					t.Errorf("CUESnippet() missing %q\ngot:\n%s", want, snippet)
				}
			}

			// Must be wrapped in braces.
			if !strings.Contains(snippet, "{") || !strings.Contains(snippet, "}") {
				t.Errorf("CUESnippet() missing braces:\n%s", snippet)
			}
		})
	}
}

func TestMissingTransitiveDepError_Error(t *testing.T) {
	t.Parallel()

	t.Run("single missing dep", func(t *testing.T) {
		t.Parallel()

		err := &MissingTransitiveDepError{
			Diagnostics: []MissingTransitiveDepDiagnostic{
				{
					RequiringModule: "io.example.B",
					RequiringURL:    "https://github.com/org/B.invowkmod",
					MissingRef: ModuleRef{
						GitURL:  "https://github.com/org/C.invowkmod",
						Version: "^1.0.0",
					},
				},
			},
		}

		msg := err.Error()
		if !strings.Contains(msg, "1 missing transitive dependency(ies)") {
			t.Errorf("expected count in message, got:\n%s", msg)
		}
		if !strings.Contains(msg, "io.example.B") {
			t.Errorf("expected requiring module in message, got:\n%s", msg)
		}
		if !strings.Contains(msg, "C.invowkmod") {
			t.Errorf("expected missing module URL in message, got:\n%s", msg)
		}
		if !strings.Contains(msg, "invowk module tidy") {
			t.Errorf("expected tidy hint in message, got:\n%s", msg)
		}
	})

	t.Run("multiple missing deps", func(t *testing.T) {
		t.Parallel()

		err := &MissingTransitiveDepError{
			Diagnostics: []MissingTransitiveDepDiagnostic{
				{
					RequiringModule: "io.example.B",
					RequiringURL:    "https://github.com/org/B.git",
					MissingRef: ModuleRef{
						GitURL:  "https://github.com/org/C.git",
						Version: "^1.0.0",
					},
				},
				{
					RequiringModule: "io.example.B",
					RequiringURL:    "https://github.com/org/B.git",
					MissingRef: ModuleRef{
						GitURL:  "https://github.com/org/D.git",
						Version: "^2.0.0",
					},
				},
			},
		}

		msg := err.Error()
		if !strings.Contains(msg, "2 missing transitive dependency(ies)") {
			t.Errorf("expected count=2 in message, got:\n%s", msg)
		}
		if !strings.Contains(msg, "C.git") || !strings.Contains(msg, "D.git") {
			t.Errorf("expected both missing URLs in message, got:\n%s", msg)
		}
	})

	t.Run("unwrap returns sentinel", func(t *testing.T) {
		t.Parallel()

		err := &MissingTransitiveDepError{}
		if !errors.Is(err, ErrMissingTransitiveDeps) {
			t.Error("expected errors.Is to match ErrMissingTransitiveDeps")
		}
	})
}
