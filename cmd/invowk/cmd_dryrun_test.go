// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	appexec "github.com/invowk/invowk/internal/app/execute"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestIsArgEnvVar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		want bool
	}{
		{"", false},
		{"ARG", false},
		{"ARG0", true},
		{"ARG1", true},
		{"ARG9", true},
		{"ARG10", true},
		{"ARG99", true},
		{"ARGC", true},
		{"ARGS", false},
		{"ARGNAME", false},
		{"ARG_1", false},
		{"ARG1NAME", false},
		{"MY_ARG1", false},
		{"arg1", false},
		{"INVOWK_ARG1", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			if got := isArgEnvVar(tt.key); got != tt.want {
				t.Errorf("isArgEnvVar(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestRenderDryRun_AllSections(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	req := ExecuteRequest{Name: "deploy"}
	cmdInfo := &discovery.CommandInfo{SourceID: "my-module.invowkmod"}
	cmd := &invowkfile.Command{}
	inv := &invowkfile.Invowkfile{}
	execCtx := runtime.NewExecutionContext(context.Background(), cmd, inv)
	execCtx.WorkDir = "/app"
	execCtx.Env.ExtraEnv = map[string]string{
		"INVOWK_CMD_NAME": "deploy",
		"ARG1":            "production",
		"DATABASE_URL":    "postgres://localhost/app",
	}
	resolved := appexec.RuntimeSelectionOf(
		invowkfile.RuntimeVirtual,
		&invowkfile.Implementation{
			Script:  "echo deploying",
			Timeout: "30s",
		},
	)

	renderDryRun(&buf, req, cmdInfo, execCtx, resolved)
	out := buf.String()

	// Verify all conditional sections appear.
	for _, token := range []string{
		"Dry Run",
		"Command:", "deploy",
		"Source:", "my-module.invowkmod",
		"Runtime:", "virtual",
		"WorkDir:", "/app",
		"Timeout:", "30s",
		"Script:",
		"echo deploying",
		"INVOWK_CMD_NAME=deploy",
		"ARG1=production",
		"DATABASE_URL=postgres://localhost/app",
		"dependency validation",
	} {
		if !strings.Contains(out, token) {
			t.Errorf("renderDryRun output missing %q:\n%s", token, out)
		}
	}
}

func TestRenderDryRun_NoImpl(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	req := ExecuteRequest{Name: "test"}
	cmdInfo := &discovery.CommandInfo{SourceID: "invowkfile"}
	cmd := &invowkfile.Command{}
	inv := &invowkfile.Invowkfile{}
	execCtx := runtime.NewExecutionContext(context.Background(), cmd, inv)
	resolved := appexec.RuntimeSelectionOf(invowkfile.RuntimeNative, nil)

	renderDryRun(&buf, req, cmdInfo, execCtx, resolved)
	out := buf.String()

	// Should still render metadata but no script/timeout/env sections.
	if !strings.Contains(out, "Command:") {
		t.Error("expected Command: header")
	}
	if strings.Contains(out, "Timeout:") {
		t.Error("Timeout should not appear when impl is nil")
	}
	if strings.Contains(out, "Script:") {
		t.Error("Script should not appear when impl is nil")
	}
}
