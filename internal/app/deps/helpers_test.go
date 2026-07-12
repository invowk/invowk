// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestCollectToolErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		tools          []invowkfile.ToolDependency
		checkMode      string
		wantErrors     int
		wantSubstring  string
		wantCheckCalls int
	}{
		{name: "no tools returns nil", checkMode: "fail"},
		{name: "single alternative found", tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"go"}}}, checkMode: "pass", wantCheckCalls: 1},
		{name: "single alternative missing", tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"missing-tool"}}}, checkMode: "fail", wantErrors: 1, wantSubstring: "not found", wantCheckCalls: 1},
		{name: "multi-alternative all missing", tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"podman", "docker"}}}, checkMode: "fail", wantErrors: 1, wantSubstring: "none of [podman, docker] found", wantCheckCalls: 2},
		{name: "multi-alternative first found", tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"podman", "docker"}}}, checkMode: "pass", wantCheckCalls: 1},
		{name: "multiple tools with mixed results", tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"go"}}, {Alternatives: []invowkfile.BinaryName{"missing1", "missing2"}}}, checkMode: "go-only", wantErrors: 1, wantSubstring: "none of [missing1, missing2] found", wantCheckCalls: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			callCount := 0
			result := CollectToolErrors(tt.tools, func(name invowkfile.BinaryName) error {
				callCount++
				if tt.checkMode == "pass" || (tt.checkMode == "go-only" && name == "go") {
					return nil
				}
				return errors.New("  • " + string(name) + " - not found")
			})
			if len(result) != tt.wantErrors {
				t.Fatalf("CollectToolErrors() returned %d errors, want %d", len(result), tt.wantErrors)
			}
			if tt.wantSubstring != "" && !strings.Contains(result[0].String(), tt.wantSubstring) {
				t.Errorf("error = %q, want containing %q", result[0], tt.wantSubstring)
			}
			if callCount != tt.wantCheckCalls {
				t.Errorf("check calls = %d, want %d", callCount, tt.wantCheckCalls)
			}
		})
	}
}
