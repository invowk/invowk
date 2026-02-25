// SPDX-License-Identifier: MPL-2.0

// primitivelint reports bare primitive types (string, int, float64, etc.)
// in struct fields, function parameters, and return types where DDD Value
// Types should be used instead.
//
// Usage:
//
//	primitivelint [-config=exceptions.toml] [-json] ./...
//	primitivelint -audit-exceptions -config=exceptions.toml ./...
//	primitivelint -update-baseline=baseline.toml -check-all -config=exceptions.toml ./...
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/invowk/invowk/tools/primitivelint/primitivelint"
)

func main() {
	// Detect --update-baseline before singlechecker takes over flag parsing.
	// singlechecker.Main() calls os.Exit(), so we must intercept first.
	if outputPath := extractUpdateBaselinePath(os.Args[1:]); outputPath != "" {
		if err := generateBaseline(outputPath, os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "primitivelint: update-baseline: %v\n", err)
			os.Exit(1)
		}
		return
	}

	singlechecker.Main(primitivelint.Analyzer)
}

// extractUpdateBaselinePath scans CLI args for -update-baseline=PATH or
// --update-baseline=PATH and returns the path. Returns "" if not found.
func extractUpdateBaselinePath(args []string) string {
	for _, arg := range args {
		trimmed := strings.TrimLeft(arg, "-")
		if strings.HasPrefix(trimmed, "update-baseline=") {
			return strings.TrimPrefix(trimmed, "update-baseline=")
		}
	}
	return ""
}

// generateBaseline runs the analyzer as a subprocess with -json output,
// parses the diagnostics, and writes a sorted baseline TOML file.
//
// The subprocess approach is necessary because singlechecker.Main() calls
// os.Exit() after analysis — there is no post-analysis hook for cross-package
// aggregation within the framework.
func generateBaseline(outputPath string, originalArgs []string) error {
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	// Build subprocess args: remove -update-baseline, ensure -json is present.
	subArgs := buildSubprocessArgs(originalArgs)

	cmd := exec.Command(selfPath, subArgs...)
	cmd.Stderr = os.Stderr // let warnings/errors pass through

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// singlechecker exits non-zero when diagnostics are found.
	// We need the JSON output regardless, so ignore exit errors.
	_ = cmd.Run()

	// Parse the go/analysis JSON output.
	findings, err := parseAnalysisJSON(stdout.Bytes())
	if err != nil {
		return fmt.Errorf("parsing analysis output: %w", err)
	}

	if err := primitivelint.WriteBaseline(outputPath, findings); err != nil {
		return fmt.Errorf("writing baseline: %w", err)
	}

	total := 0
	for _, msgs := range findings {
		total += len(msgs)
	}
	fmt.Fprintf(os.Stderr, "Baseline written: %s (%d findings)\n", outputPath, total)

	return nil
}

// buildSubprocessArgs constructs args for the subprocess invocation by
// removing -update-baseline and ensuring -json is present.
func buildSubprocessArgs(args []string) []string {
	var result []string
	hasJSON := false

	for _, arg := range args {
		trimmed := strings.TrimLeft(arg, "-")

		// Skip the update-baseline flag itself.
		if strings.HasPrefix(trimmed, "update-baseline") {
			continue
		}

		if trimmed == "json" {
			hasJSON = true
		}

		result = append(result, arg)
	}

	if !hasJSON {
		// Prepend -json before package patterns (which come last).
		result = slices.Insert(result, 0, "-json")
	}

	return result
}

// analysisResult represents the go/analysis -json output structure.
// The JSON is a map from package path to per-analyzer results.
type analysisResult map[string]map[string][]analysisDiagnostic

// analysisDiagnostic is a single diagnostic in the -json output.
type analysisDiagnostic struct {
	Posn     string `json:"posn"`
	Message  string `json:"message"`
	Category string `json:"category"`
}

// parseAnalysisJSON parses the go/analysis -json output (one JSON object
// per package, concatenated) and returns findings grouped by category.
// Filters out stale-exception diagnostics and deduplicates.
func parseAnalysisJSON(data []byte) (map[string][]string, error) {
	// The -json output is a stream of JSON objects (one per package).
	// Each object maps package path → analyzer name → diagnostics array.
	decoder := json.NewDecoder(bytes.NewReader(data))

	// Deduplicate across packages (test variants can produce duplicates).
	seen := make(map[string]map[string]bool) // category → set of messages

	for decoder.More() {
		var result analysisResult
		if err := decoder.Decode(&result); err != nil {
			return nil, fmt.Errorf("decoding JSON object: %w", err)
		}

		for _, analyzers := range result {
			diags, ok := analyzers["primitivelint"]
			if !ok {
				continue
			}
			for _, d := range diags {
				// Skip stale-exception — config maintenance, not codebase findings.
				if d.Category == primitivelint.CategoryStaleException {
					continue
				}
				if d.Category == "" || d.Message == "" {
					continue
				}

				if seen[d.Category] == nil {
					seen[d.Category] = make(map[string]bool)
				}
				seen[d.Category][d.Message] = true
			}
		}
	}

	// Convert sets to sorted slices.
	findings := make(map[string][]string, len(seen))
	for cat, msgs := range seen {
		sorted := make([]string, 0, len(msgs))
		for msg := range msgs {
			sorted = append(sorted, msg)
		}
		slices.Sort(sorted)
		findings[cat] = sorted
	}

	return findings, nil
}
