// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/tools/goplint/goplint"
)

func TestExtractUpdateBaselinePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "double-dash form",
			args: []string{"--update-baseline=out.toml", "./..."},
			want: "out.toml",
		},
		{
			name: "single-dash form",
			args: []string{"-update-baseline=/tmp/baseline.toml"},
			want: "/tmp/baseline.toml",
		},
		{
			name: "mixed with other flags",
			args: []string{"-check-all", "--update-baseline=b.toml", "-config=e.toml", "./..."},
			want: "b.toml",
		},
		{
			name: "space separated value",
			args: []string{"--update-baseline", "out.toml", "./..."},
			want: "out.toml",
		},
		{
			name: "absent flag returns empty",
			args: []string{"-check-all", "-config=e.toml", "./..."},
			want: "",
		},
		{
			name: "empty args",
			args: []string{},
			want: "",
		},
		{
			name: "nil args",
			args: nil,
			want: "",
		},
		{
			name: "flag without equals returns empty",
			args: []string{"--update-baseline"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractUpdateBaselinePath(tt.args)
			if got != tt.want {
				t.Errorf("extractUpdateBaselinePath(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestBuildSubprocessArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "removes update-baseline and adds json",
			args: []string{"--update-baseline=out.toml", "-check-all", "./..."},
			want: []string{"-json", "-check-all", "./..."},
		},
		{
			name: "preserves existing json flag",
			args: []string{"-json", "--update-baseline=out.toml", "-check-all"},
			want: []string{"-json", "-check-all"},
		},
		{
			name: "removes single-dash update-baseline",
			args: []string{"-update-baseline=out.toml", "./..."},
			want: []string{"-json", "./..."},
		},
		{
			name: "preserves all other flags and patterns",
			args: []string{"-check-all", "-config=e.toml", "--update-baseline=b.toml", "./internal/...", "./pkg/..."},
			want: []string{"-json", "-check-all", "-config=e.toml", "./internal/...", "./pkg/..."},
		},
		{
			name: "removes space separated update-baseline and value",
			args: []string{"--update-baseline", "out.toml", "-check-all", "./..."},
			want: []string{"-json", "-check-all", "./..."},
		},
		{
			name: "empty args adds json only",
			args: []string{},
			want: []string{"-json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildSubprocessArgs(tt.args)
			if !slices.Equal(got, tt.want) {
				t.Errorf("buildSubprocessArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestHasFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		flag string
		want bool
	}{
		{name: "single-dash present", args: []string{"-global"}, flag: "global", want: true},
		{name: "double-dash present", args: []string{"--global"}, flag: "global", want: true},
		{name: "absent", args: []string{"-check-all"}, flag: "global", want: false},
		{name: "value form true", args: []string{"--global=true"}, flag: "global", want: true},
		{name: "value form false", args: []string{"--global=false"}, flag: "global", want: false},
		{name: "space value true", args: []string{"--global", "true"}, flag: "global", want: true},
		{name: "space value false", args: []string{"--global", "false"}, flag: "global", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := hasFlag(tt.args, tt.flag)
			if got != tt.want {
				t.Errorf("hasFlag(%v, %q) = %v, want %v", tt.args, tt.flag, got, tt.want)
			}
		})
	}
}

func TestHasFlagToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		flag string
		want bool
	}{
		{name: "double dash bare", args: []string{"--global"}, flag: "global", want: true},
		{name: "double dash equals", args: []string{"--global=false"}, flag: "global", want: true},
		{name: "single dash equals", args: []string{"-global=true"}, flag: "global", want: true},
		{name: "absent", args: []string{"-check-all"}, flag: "global", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := hasFlagToken(tt.args, tt.flag)
			if got != tt.want {
				t.Errorf("hasFlagToken(%v, %q) = %v, want %v", tt.args, tt.flag, got, tt.want)
			}
		})
	}
}

func TestBuildGlobalAuditArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "removes global and adds required flags",
			args: []string{"--global", "./..."},
			want: []string{"-audit-exceptions", "-json", "./..."},
		},
		{
			name: "preserves existing required flags",
			args: []string{"-json", "-audit-exceptions", "--global", "./..."},
			want: []string{"-json", "-audit-exceptions", "./..."},
		},
		{
			name: "keeps unrelated flags",
			args: []string{"--global", "-config=exceptions.toml", "./pkg/..."},
			want: []string{"-audit-exceptions", "-json", "-config=exceptions.toml", "./pkg/..."},
		},
		{
			name: "removes bool value token when explicit",
			args: []string{"--global", "true", "-config=exceptions.toml", "./pkg/..."},
			want: []string{"-audit-exceptions", "-json", "-config=exceptions.toml", "./pkg/..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildGlobalAuditArgs(tt.args)
			if !slices.Equal(got, tt.want) {
				t.Errorf("buildGlobalAuditArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestAggregateGlobalStalePatterns(t *testing.T) {
	t.Parallel()

	t.Run("deduplicates package coverage per pattern", func(t *testing.T) {
		t.Parallel()
		stream := append(
			makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
				"example.com/a": {
					"goplint": {
						{Category: goplint.CategoryStaleException, Message: `stale exception: pattern "dup.pattern" matched no diagnostics (reason: x)`},
						{Category: goplint.CategoryStaleException, Message: `stale exception: pattern "dup.pattern" matched no diagnostics (reason: x)`},
					},
				},
			}),
			makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
				"example.com/b": {
					"goplint": {},
				},
			})...,
		)

		patterns, totalPatterns, totalPackages, err := aggregateGlobalStalePatterns(stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if totalPackages != 2 {
			t.Fatalf("expected 2 packages, got %d", totalPackages)
		}
		if totalPatterns != 1 {
			t.Fatalf("expected 1 stale pattern, got %d", totalPatterns)
		}
		if len(patterns) != 0 {
			t.Fatalf("expected no globally stale patterns, got %v", patterns)
		}
	})

	t.Run("reports patterns stale in all packages", func(t *testing.T) {
		t.Parallel()
		stream := append(
			makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
				"example.com/a": {
					"goplint": {
						{Category: goplint.CategoryStaleException, Message: `stale exception: pattern "shared.pattern" matched no diagnostics (reason: x)`},
					},
				},
			}),
			makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
				"example.com/b": {
					"goplint": {
						{Category: goplint.CategoryStaleException, Message: `stale exception: pattern "shared.pattern" matched no diagnostics (reason: y)`},
					},
				},
			})...,
		)

		patterns, totalPatterns, totalPackages, err := aggregateGlobalStalePatterns(stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if totalPackages != 2 {
			t.Fatalf("expected 2 packages, got %d", totalPackages)
		}
		if totalPatterns != 1 {
			t.Fatalf("expected 1 stale pattern, got %d", totalPatterns)
		}
		if !slices.Equal(patterns, []string{"shared.pattern"}) {
			t.Fatalf("unexpected globally stale patterns: got %v", patterns)
		}
	})

	t.Run("no packages analyzed returns zero counts", func(t *testing.T) {
		t.Parallel()
		patterns, totalPatterns, totalPackages, err := aggregateGlobalStalePatterns(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(patterns) != 0 {
			t.Fatalf("expected no patterns, got %v", patterns)
		}
		if totalPatterns != 0 || totalPackages != 0 {
			t.Fatalf("expected zero counts, got patterns=%d packages=%d", totalPatterns, totalPackages)
		}
	})

	t.Run("malformed stream returns decode error", func(t *testing.T) {
		t.Parallel()
		_, _, _, err := aggregateGlobalStalePatterns([]byte("{invalid"))
		if err == nil {
			t.Fatal("expected decode error")
		}
	})
}

func TestAuditExceptionsGlobalExitBehavior(t *testing.T) {
	originalRunCommand := runCommand
	t.Cleanup(func() {
		runCommand = originalRunCommand
	})

	t.Run("fails when globally stale patterns are found", func(t *testing.T) {
		stream := append(
			makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
				"example.com/a": {
					"goplint": {
						{Category: goplint.CategoryStaleException, Message: `stale exception: pattern "shared.pattern" matched no diagnostics (reason: x)`},
					},
				},
			}),
			makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
				"example.com/b": {
					"goplint": {
						{Category: goplint.CategoryStaleException, Message: `stale exception: pattern "shared.pattern" matched no diagnostics (reason: y)`},
					},
				},
			})...,
		)

		runCommand = func(cmd *exec.Cmd) error {
			buf, ok := cmd.Stdout.(*bytes.Buffer)
			if !ok {
				t.Fatalf("expected *bytes.Buffer stdout, got %T", cmd.Stdout)
			}
			_, err := buf.Write(stream)
			return err
		}

		err := auditExceptionsGlobal([]string{"--global", "./..."})
		if err == nil {
			t.Fatal("expected non-nil error when globally stale patterns exist")
		}
	})

	t.Run("succeeds when no globally stale patterns are found", func(t *testing.T) {
		stream := append(
			makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
				"example.com/a": {
					"goplint": {
						{Category: goplint.CategoryStaleException, Message: `stale exception: pattern "only.in.a" matched no diagnostics (reason: x)`},
					},
				},
			}),
			makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
				"example.com/b": {
					"goplint": {},
				},
			})...,
		)

		runCommand = func(cmd *exec.Cmd) error {
			buf, ok := cmd.Stdout.(*bytes.Buffer)
			if !ok {
				t.Fatalf("expected *bytes.Buffer stdout, got %T", cmd.Stdout)
			}
			_, err := buf.Write(stream)
			return err
		}

		if err := auditExceptionsGlobal([]string{"--global", "./..."}); err != nil {
			t.Fatalf("expected nil error when no globally stale patterns exist, got %v", err)
		}
	})
}

func TestFilterAndEnsureFlags(t *testing.T) {
	t.Parallel()

	t.Run("skip and prepend required flags", func(t *testing.T) {
		t.Parallel()
		got := filterAndEnsureFlags(
			[]string{"--drop", "-x", "./..."},
			func(trimmed string) bool { return trimmed == "drop" },
			[]string{"-a", "-b"},
		)
		want := []string{"-b", "-a", "-x", "./..."}
		if !slices.Equal(got, want) {
			t.Errorf("filterAndEnsureFlags() = %v, want %v", got, want)
		}
	})

	t.Run("does not duplicate existing required flags", func(t *testing.T) {
		t.Parallel()
		got := filterAndEnsureFlags(
			[]string{"-a", "-x"},
			func(string) bool { return false },
			[]string{"-a"},
		)
		want := []string{"-a", "-x"}
		if !slices.Equal(got, want) {
			t.Errorf("filterAndEnsureFlags() = %v, want %v", got, want)
		}
	})
}

func TestExtractPatternFromStaleMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "valid format",
			message: `stale exception: pattern "pkg.Type.Field" matched no diagnostics (reason: test)`,
			want:    "pkg.Type.Field",
		},
		{
			name:    "invalid format",
			message: "something else",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractPatternFromStaleMessage(tt.message)
			if got != tt.want {
				t.Errorf("extractPatternFromStaleMessage(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

func TestExtractPatternFromStaleDiagnostic(t *testing.T) {
	t.Parallel()

	t.Run("prefers URL metadata", func(t *testing.T) {
		t.Parallel()
		diag := analysisDiagnostic{
			Category: goplint.CategoryStaleException,
			Message:  `stale exception: pattern "wrong" matched no diagnostics (reason: test)`,
			URL: goplint.DiagnosticURLForFindingWithMeta(
				goplint.StableFindingID(goplint.CategoryStaleException, "pkg.Type.Field"),
				map[string]string{"pattern": "pkg.Type.Field"},
			),
		}
		got := extractPatternFromStaleDiagnostic(diag)
		if got != "pkg.Type.Field" {
			t.Fatalf("extractPatternFromStaleDiagnostic() = %q, want %q", got, "pkg.Type.Field")
		}
	})

	t.Run("falls back to message parsing", func(t *testing.T) {
		t.Parallel()
		diag := analysisDiagnostic{
			Category: goplint.CategoryStaleException,
			Message:  `stale exception: pattern "pkg.Type.Legacy" matched no diagnostics (reason: legacy)`,
		}
		got := extractPatternFromStaleDiagnostic(diag)
		if got != "pkg.Type.Legacy" {
			t.Fatalf("extractPatternFromStaleDiagnostic() = %q, want %q", got, "pkg.Type.Legacy")
		}
	})
}

func TestTolerateAnalyzerExit(t *testing.T) {
	t.Parallel()

	t.Run("nil error accepted", func(t *testing.T) {
		t.Parallel()
		if err := tolerateAnalyzerExit(nil, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("exit error with json output accepted", func(t *testing.T) {
		t.Parallel()
		exitErr := makeExitError(t)
		if err := tolerateAnalyzerExit(exitErr, 10); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("exit error with empty output rejected", func(t *testing.T) {
		t.Parallel()
		exitErr := makeExitError(t)
		if err := tolerateAnalyzerExit(exitErr, 0); err == nil {
			t.Fatal("expected error for empty stdout")
		}
	})
}

func TestGenerateBaseline(t *testing.T) {
	t.Run("happy path writes baseline file", func(t *testing.T) {
		originalRunCommand := runCommand
		t.Cleanup(func() {
			runCommand = originalRunCommand
		})

		stream := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{Category: goplint.CategoryPrimitive, Posn: "pkg/a.go:10:2", Message: "struct field pkg.A.B uses primitive type string"},
					{Category: goplint.CategoryUnusedValidateResult, Posn: "pkg/a.go:20:2", Message: "Validate() result discarded — error return is unused"},
					{Category: goplint.CategoryUnusedValidateResult, Posn: "pkg/a.go:30:2", Message: "Validate() result discarded — error return is unused"},
					{Category: goplint.CategoryStaleException, Posn: "pkg/a.go:1:1", Message: `stale exception: pattern "x" matched no diagnostics (reason: y)`},
				},
			},
		})

		runCommand = func(cmd *exec.Cmd) error {
			buf, ok := cmd.Stdout.(*bytes.Buffer)
			if !ok {
				t.Fatalf("expected *bytes.Buffer stdout, got %T", cmd.Stdout)
			}
			_, err := buf.Write(stream)
			return err
		}

		outPath := filepath.Join(t.TempDir(), "baseline.toml")
		if err := generateBaseline(outPath, []string{"--update-baseline", outPath, "./..."}); err != nil {
			t.Fatalf("generateBaseline() error = %v", err)
		}

		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("reading baseline output: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "[primitive]") {
			t.Fatal("expected primitive section in baseline output")
		}
		if !strings.Contains(content, "[unused-validate-result]") {
			t.Fatal("expected unused-validate-result section in baseline output")
		}
		if strings.Contains(content, "[stale-exception]") {
			t.Fatal("did not expect stale-exception section in baseline output")
		}
		if got := strings.Count(content, "Validate() result discarded — error return is unused"); got != 2 {
			t.Fatalf("expected 2 validate-usage entries, got %d", got)
		}
	})

	t.Run("malformed JSON stream returns parse error", func(t *testing.T) {
		originalRunCommand := runCommand
		t.Cleanup(func() {
			runCommand = originalRunCommand
		})
		runCommand = func(cmd *exec.Cmd) error {
			buf, ok := cmd.Stdout.(*bytes.Buffer)
			if !ok {
				t.Fatalf("expected *bytes.Buffer stdout, got %T", cmd.Stdout)
			}
			_, err := buf.Write([]byte("{invalid"))
			return err
		}
		outPath := filepath.Join(t.TempDir(), "baseline.toml")
		err := generateBaseline(outPath, []string{"--update-baseline", outPath, "./..."})
		if err == nil {
			t.Fatal("expected parse error from malformed JSON stream")
		}
		if !strings.Contains(err.Error(), "parsing analysis output") {
			t.Fatalf("expected parsing analysis output error, got %v", err)
		}
	})

	t.Run("non-exit command error is returned", func(t *testing.T) {
		originalRunCommand := runCommand
		t.Cleanup(func() {
			runCommand = originalRunCommand
		})

		expectedErr := errors.New("boom")
		runCommand = func(*exec.Cmd) error { return expectedErr }

		outPath := filepath.Join(t.TempDir(), "baseline.toml")
		err := generateBaseline(outPath, []string{"--update-baseline", outPath, "./..."})
		if err == nil {
			t.Fatal("expected command error")
		}
		if !strings.Contains(err.Error(), "running analyzer subprocess") {
			t.Fatalf("expected wrapped subprocess error, got %v", err)
		}
	})
}

func TestParseAnalysisJSON(t *testing.T) {
	t.Parallel()

	t.Run("single package with findings", func(t *testing.T) {
		t.Parallel()
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{Category: "primitive", Message: "struct field pkg.Foo.Bar uses primitive type string"},
					{Category: "missing-validate", Message: "named type pkg.MyType has no Validate() method"},
				},
			},
		})

		findings, err := parseAnalysisJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(findings["primitive"]) != 1 {
			t.Errorf("expected 1 primitive finding, got %d", len(findings["primitive"]))
		}
		if len(findings["missing-validate"]) != 1 {
			t.Errorf("expected 1 missing-validate finding, got %d", len(findings["missing-validate"]))
		}
	})

	t.Run("deduplicates across packages", func(t *testing.T) {
		t.Parallel()
		// Simulate the same diagnostic appearing in both the package and its test variant.
		diag := analysisDiagnostic{
			Category: "primitive",
			Message:  "struct field pkg.Foo.Bar uses primitive type string",
		}
		pkg1 := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {"goplint": {diag}},
		})
		pkg2 := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg [example.com/pkg.test]": {"goplint": {diag}},
		})
		combined := append(pkg1, pkg2...)

		findings, err := parseAnalysisJSON(combined)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(findings["primitive"]) != 1 {
			t.Errorf("expected 1 deduplicated finding, got %d", len(findings["primitive"]))
		}
	})

	t.Run("uses finding ID from diagnostic URL", func(t *testing.T) {
		t.Parallel()
		const findingID = "gpl1_deadbeef"
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{
						Category: "primitive",
						Message:  "struct field pkg.Foo.Bar uses primitive type string",
						URL:      goplint.DiagnosticURLForFinding(findingID),
					},
				},
			},
		})

		findings, err := parseAnalysisJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings["primitive"]) != 1 {
			t.Fatalf("expected 1 primitive finding, got %d", len(findings["primitive"]))
		}
		if findings["primitive"][0].ID != findingID {
			t.Errorf("expected finding ID %q, got %q", findingID, findings["primitive"][0].ID)
		}
	})

	t.Run("handles CFA categories in baseline parsing", func(t *testing.T) {
		t.Parallel()
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{
						Category: goplint.CategoryUnvalidatedCast,
						Message:  "type conversion to pkg.CommandName from non-constant without Validate() check",
						URL:      goplint.DiagnosticURLForFinding("gpl1_cfa_unvalidated"),
					},
					{
						Category: goplint.CategoryUseBeforeValidate,
						Message:  "variable x of type pkg.CommandName used before Validate() in same block",
						URL:      goplint.DiagnosticURLForFinding("gpl1_cfa_ubv"),
					},
				},
			},
		})

		findings, err := parseAnalysisJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings[goplint.CategoryUnvalidatedCast]) != 1 {
			t.Fatalf("expected 1 %s finding, got %d", goplint.CategoryUnvalidatedCast, len(findings[goplint.CategoryUnvalidatedCast]))
		}
		if len(findings[goplint.CategoryUseBeforeValidate]) != 1 {
			t.Fatalf("expected 1 %s finding, got %d", goplint.CategoryUseBeforeValidate, len(findings[goplint.CategoryUseBeforeValidate]))
		}
	})

	t.Run("falls back to derived ID when URL is missing", func(t *testing.T) {
		t.Parallel()
		const (
			category = "primitive"
			message  = "struct field pkg.Foo.Bar uses primitive type string"
		)
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{Category: category, Message: message},
				},
			},
		})

		findings, err := parseAnalysisJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings[category]) != 1 {
			t.Fatalf("expected 1 %s finding, got %d", category, len(findings[category]))
		}
		wantID := goplint.FallbackFindingIDForDiagnostic(category, "example.com/pkg", message)
		if findings[category][0].ID != wantID {
			t.Errorf("expected fallback ID %q, got %q", wantID, findings[category][0].ID)
		}
	})

	t.Run("fallback ID uses position to keep repeated messages distinct", func(t *testing.T) {
		t.Parallel()
		const (
			category = goplint.CategoryUnusedValidateResult
			message  = "Validate() result discarded — error return is unused"
		)
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{Category: category, Posn: "pkg/file.go:10:2", Message: message},
					{Category: category, Posn: "pkg/file.go:20:2", Message: message},
				},
			},
		})

		findings, err := parseAnalysisJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings[category]) != 2 {
			t.Fatalf("expected 2 %s findings, got %d", category, len(findings[category]))
		}
	})

	t.Run("filters out stale-exception diagnostics", func(t *testing.T) {
		t.Parallel()
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{Category: "primitive", Message: "real finding"},
					{Category: goplint.CategoryStaleException, Message: "stale exception: pattern ..."},
				},
			},
		})

		findings, err := parseAnalysisJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(findings["primitive"]) != 1 {
			t.Errorf("expected 1 primitive finding, got %d", len(findings["primitive"]))
		}
		if len(findings[goplint.CategoryStaleException]) != 0 {
			t.Errorf("expected 0 stale-exception findings, got %d", len(findings[goplint.CategoryStaleException]))
		}
	})

	t.Run("skips entries with empty category or message", func(t *testing.T) {
		t.Parallel()
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{Category: "", Message: "orphaned message"},
					{Category: "primitive", Message: ""},
					{Category: "primitive", Message: "valid finding"},
				},
			},
		})

		findings, err := parseAnalysisJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(findings["primitive"]) != 1 {
			t.Errorf("expected 1 finding (empty category/message filtered), got %d", len(findings["primitive"]))
		}
	})

	t.Run("filters non-suppressible categories", func(t *testing.T) {
		t.Parallel()
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{Category: goplint.CategoryUnknownDirective, Message: "unknown directive key"},
					{Category: "primitive", Message: "valid finding"},
				},
			},
		})

		findings, err := parseAnalysisJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(findings[goplint.CategoryUnknownDirective]) != 0 {
			t.Errorf("expected unknown-directive to be excluded from baseline findings")
		}
		if len(findings["primitive"]) != 1 {
			t.Errorf("expected primitive finding to remain, got %d", len(findings["primitive"]))
		}
	})

	t.Run("unknown category returns error", func(t *testing.T) {
		t.Parallel()
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"goplint": {
					{Category: "totally-unknown-category", Message: "unexpected"},
				},
			},
		})
		_, err := parseAnalysisJSON(input)
		if err == nil {
			t.Fatal("expected error for unknown category")
		}
	})

	t.Run("empty input returns empty findings", func(t *testing.T) {
		t.Parallel()
		findings, err := parseAnalysisJSON([]byte{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(findings) != 0 {
			t.Errorf("expected 0 categories, got %d", len(findings))
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		t.Parallel()
		_, err := parseAnalysisJSON([]byte("{invalid json"))
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
	})

	t.Run("ignores non-goplint analyzer results", func(t *testing.T) {
		t.Parallel()
		input := makeAnalysisJSON(t, map[string]map[string][]analysisDiagnostic{
			"example.com/pkg": {
				"otherana": {
					{Category: "other", Message: "not our concern"},
				},
				"goplint": {
					{Category: "primitive", Message: "our finding"},
				},
			},
		})

		findings, err := parseAnalysisJSON(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(findings["other"]) != 0 {
			t.Errorf("expected 0 'other' findings, got %d", len(findings["other"]))
		}
		if len(findings["primitive"]) != 1 {
			t.Errorf("expected 1 primitive finding, got %d", len(findings["primitive"]))
		}
	})
}

func TestStableDiagnosticPosKey(t *testing.T) {
	t.Parallel()

	got := stableDiagnosticPosKey("example.com/pkg", "/tmp/work/pkg/file.go:10:2")
	want := "example.com/pkg:file.go:10:2"
	if got != want {
		t.Fatalf("stableDiagnosticPosKey() = %q, want %q", got, want)
	}
}

func TestCanonicalPackagePath(t *testing.T) {
	t.Parallel()

	got := canonicalPackagePath("example.com/pkg [example.com/pkg.test]")
	if got != "example.com/pkg" {
		t.Fatalf("canonicalPackagePath() = %q, want %q", got, "example.com/pkg")
	}
}

// makeAnalysisJSON serializes the go/analysis -json output format for testing.
func makeAnalysisJSON(t *testing.T, result analysisResult) []byte {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshaling test JSON: %v", err)
	}
	return data
}

func makeExitError(t *testing.T) error {
	t.Helper()
	return &exec.ExitError{}
}
