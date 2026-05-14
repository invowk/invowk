// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/commandsvc"
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
	plan := commandsvc.DryRunPlan{
		CommandName: "deploy",
		SourceID:    "my-module.invowkmod",
		Runtime:     invowkfile.RuntimeVirtual,
		Platform:    invowkfile.PlatformLinux,
		WorkDir:     "/app",
		Timeout:     "30s",
		Script:      "echo deploying",
		Env: map[string]string{
			"INVOWK_CMD_NAME": "deploy",
			"ARG1":            "production",
			"DATABASE_URL":    "postgres://localhost/app",
		},
		DependencyValidationSkipped: true,
	}

	renderDryRun(&buf, plan)
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
	plan := commandsvc.DryRunPlan{
		CommandName: "test",
		SourceID:    "invowkfile",
		Runtime:     invowkfile.RuntimeNative,
		Platform:    invowkfile.PlatformLinux,
	}

	renderDryRun(&buf, plan)
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

func TestRenderDryRun_PersistentContainer(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plan := commandsvc.DryRunPlan{
		CommandName:                        "test",
		SourceID:                           "invowkfile",
		Runtime:                            invowkfile.RuntimeContainer,
		Platform:                           invowkfile.PlatformLinux,
		Timeout:                            "30s",
		Script:                             "echo persistent",
		PersistentContainerMode:            "persistent",
		PersistentContainerName:            "existing-dev",
		PersistentContainerNameSource:      "cli",
		PersistentContainerCreateIfMissing: false,
		DependencyValidationSkipped:        true,
	}

	renderDryRun(&buf, plan)
	out := buf.String()

	for _, token := range []string{
		"Container:",
		"persistent",
		"ContainerName:",
		"existing-dev",
		"ContainerNameSource:",
		"cli",
		"CreateIfMissing:",
		"false",
	} {
		if !strings.Contains(out, token) {
			t.Fatalf("renderDryRun output missing %q:\n%s", token, out)
		}
	}
}

func TestRenderDryRun_EphemeralContainer(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	plan := commandsvc.DryRunPlan{
		CommandName:             "test",
		SourceID:                "invowkfile",
		Runtime:                 invowkfile.RuntimeContainer,
		Platform:                invowkfile.PlatformLinux,
		Script:                  "echo ephemeral",
		PersistentContainerMode: "ephemeral",
	}

	renderDryRun(&buf, plan)
	out := buf.String()

	for _, token := range []string{
		"Container:",
		"ephemeral",
	} {
		if !strings.Contains(out, token) {
			t.Fatalf("renderDryRun output missing %q:\n%s", token, out)
		}
	}
	for _, token := range []string{
		"ContainerName:",
		"ContainerNameSource:",
		"CreateIfMissing:",
	} {
		if strings.Contains(out, token) {
			t.Fatalf("renderDryRun output contains persistent-only field %q:\n%s", token, out)
		}
	}
}
