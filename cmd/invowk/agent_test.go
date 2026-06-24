// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestValidateAgentCmdCreateModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dryRun    bool
		printOnly bool
		verify    bool
		wantErr   string
	}{
		{name: "write mode"},
		{name: "dry run", dryRun: true},
		{name: "print", printOnly: true},
		{name: "verify write", verify: true},
		{name: "dry run and print", dryRun: true, printOnly: true, wantErr: "--dry-run and --print"},
		{name: "verify and dry run", dryRun: true, verify: true, wantErr: "--verify requires writing"},
		{name: "verify and print", printOnly: true, verify: true, wantErr: "--verify requires writing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAgentCmdCreateModes(tt.dryRun, tt.printOnly, tt.verify)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateAgentCmdCreateModes() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateAgentCmdCreateModes() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestAgentCommandTreeIncludesAuthoringCRUD(t *testing.T) {
	t.Parallel()

	app, err := NewApp(Dependencies{})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	agent := newAgentCommand(app, &rootFlagValues{})

	for _, path := range []string{
		"cmd prompt",
		"cmd create",
		"cmd change",
		"cmd remove",
		"mod prompt",
		"mod create",
		"mod change",
		"mod remove",
	} {
		if !hasCommandPath(agent, strings.Fields(path)) {
			t.Fatalf("agent command tree missing %q", path)
		}
	}
}

func TestValidateCommandAuthoringInputRequiresDescription(t *testing.T) {
	t.Parallel()

	err := validateCommandAuthoringInput(invowkfile.CommandName("legacy description only"), "", "")
	if err == nil || !strings.Contains(err.Error(), "command description is required") {
		t.Fatalf("validateCommandAuthoringInput() error = %v, want required description", err)
	}
}

func hasCommandPath(cmd *cobra.Command, path []string) bool {
	if len(path) == 0 {
		return true
	}
	for _, child := range cmd.Commands() {
		if child.Name() == path[0] {
			return hasCommandPath(child, path[1:])
		}
	}
	return false
}
