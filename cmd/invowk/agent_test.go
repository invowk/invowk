// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
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

func TestVerifySourceForWrittenCommand(t *testing.T) {
	t.Parallel()

	const cmdCUE = `cmds: [{name: "lint", implementations: [{script: {content: "echo lint"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]}]`
	writeFile := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	commandSet := func(t *testing.T, base string) *discovery.DiscoveredCommandSet {
		t.Helper()
		d := discovery.New(config.DefaultConfig(), discovery.WithBaseDir(types.FilesystemPath(base)), discovery.WithCommandsDir(""))
		result, err := d.DiscoverCommandSet(t.Context())
		if err != nil {
			t.Fatalf("DiscoverCommandSet() error: %v", err)
		}
		return result.Set
	}

	moduleDir := filepath.Join(t.TempDir(), "tools.invowkmod")
	writeFile(t, filepath.Join(moduleDir, "invowkmod.cue"), "module: \"tools\"\nversion: \"1.0.0\"\n")
	writeFile(t, filepath.Join(moduleDir, agentInvowkfileName), cmdCUE)
	plainDir := filepath.Join(t.TempDir(), "project")
	writeFile(t, filepath.Join(plainDir, agentInvowkfileName), cmdCUE)

	tests := []struct {
		name string
		base string
		cmd  invowkfile.CommandName
		want discovery.SourceID
	}{
		{name: "module directory", base: moduleDir, cmd: "lint", want: "tools"},
		{name: "plain directory", base: plainDir, cmd: "lint", want: discovery.SourceIDInvowkfile},
		{name: "name not written", base: moduleDir, cmd: "missing", want: discovery.SourceIDInvowkfile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := types.FilesystemPath(filepath.Join(tt.base, agentInvowkfileName))
			if got := verifySourceForWrittenCommand(commandSet(t, tt.base), target, tt.cmd); got != tt.want {
				t.Fatalf("verifySourceForWrittenCommand() = %q, want %q", got, tt.want)
			}
		})
	}
	if got := verifySourceForWrittenCommand(nil, types.FilesystemPath(filepath.Join(moduleDir, agentInvowkfileName)), "lint"); got != discovery.SourceIDInvowkfile {
		t.Fatalf("nil set: got %q, want %q", got, discovery.SourceIDInvowkfile)
	}
}
