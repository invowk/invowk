// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

// executeWithUsageGuards runs args against a root holding sub, wired like the
// production tree, and returns the process exit status the error maps to.
func executeWithUsageGuards(t *testing.T, sub *cobra.Command, args ...string) (types.ExitCode, error) {
	t.Helper()
	root := &cobra.Command{Use: "invowk", SilenceErrors: true, SilenceUsage: true}
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.AddCommand(sub)
	installUsageGuards(root)
	root.SetArgs(args)
	err := root.Execute()
	return exitCodeForError(err), err
}

// TestAgentExitCodes covers one site per row of design.md Decision 8: argv-only
// errors exit with ExitCodeUsage, every other agent failure exits with 1.
func TestAgentExitCodes(t *testing.T) {
	t.Parallel()

	missing := func(t *testing.T, name string) string {
		t.Helper()
		return filepath.Join(t.TempDir(), name)
	}

	tests := []struct {
		name string
		args func(t *testing.T) []string
		want types.ExitCode
		// dials marks a row that opens a loopback TCP connection.
		dials bool
	}{
		{
			name: "unknown prompt format",
			args: func(*testing.T) []string { return []string{"agent", "cmd", "prompt", "--format", "yaml"} },
			want: ExitCodeUsage,
		},
		{
			name: "unknown module prompt operation",
			args: func(*testing.T) []string { return []string{"agent", "mod", "prompt", "bogus"} },
			want: ExitCodeUsage,
		},
		{
			name: "invalid command name",
			args: func(*testing.T) []string { return []string{"agent", "cmd", "remove", " "} },
			want: ExitCodeUsage,
		},
		{
			name: "mutually exclusive modes",
			args: func(*testing.T) []string {
				return []string{"agent", "cmd", "create", "hello", "describe it", "--dry-run", "--print"}
			},
			want: ExitCodeUsage,
		},
		{
			name: "missing module description",
			args: func(*testing.T) []string { return []string{"agent", "mod", "create", "io.example.tools"} },
			want: ExitCodeUsage,
		},
		{
			name: "invalid llm flag value",
			args: func(*testing.T) []string {
				return []string{"agent", "cmd", "create", "hello", "describe it", "--llm-concurrency", "-1"}
			},
			want: ExitCodeUsage,
		},
		{
			name: "arity error",
			args: func(*testing.T) []string { return []string{"agent", "cmd", "remove", "a", "b"} },
			want: ExitCodeUsage,
		},
		{
			name: "unknown agent subcommand",
			args: func(*testing.T) []string { return []string{"agent", "bogus"} },
			want: ExitCodeUsage,
		},
		{
			name: "remove a malformed module id",
			args: func(*testing.T) []string { return []string{"agent", "mod", "remove", "bad id"} },
			want: ExitCodeUsage,
		},
		{
			name: "change a malformed module id",
			args: func(*testing.T) []string { return []string{"agent", "mod", "change", "bad id", "describe it"} },
			want: ExitCodeUsage,
		},
		{
			name: "remove command from a missing invowkfile",
			args: func(t *testing.T) []string {
				t.Helper()
				return []string{"agent", "cmd", "remove", "hello", "--file", missing(t, "invowkfile.cue")}
			},
			want: agentExitFailure,
		},
		{
			name: "remove a missing module",
			args: func(t *testing.T) []string {
				t.Helper()
				return []string{"agent", "mod", "remove", missing(t, "io.example.none.invowkmod")}
			},
			want: agentExitFailure,
		},
		{
			name: "llm client cannot reach the configured API",
			args: func(t *testing.T) []string {
				t.Helper()
				return []string{
					"agent", "cmd", "create", "hello", "describe it",
					"--llm", "--llm-url", "http://127.0.0.1:1/v1", "--llm-model", "m", "--llm-timeout", "2s",
					"--file", missing(t, "invowkfile.cue"),
				}
			},
			want:  agentExitFailure,
			dials: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.dials && testing.Short() {
				t.Skip("skipping: dials a loopback LLM endpoint")
			}

			app, err := NewApp(Dependencies{
				Config: &staticConfigProvider{cfg: config.DefaultConfig()},
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
			})
			if err != nil {
				t.Fatalf("NewApp() error = %v", err)
			}
			got, runErr := executeWithUsageGuards(t, newAgentCommand(app, &rootFlagValues{}), tt.args(t)...)
			if got != tt.want {
				t.Fatalf("exit code = %d (error: %v), want %d", got, runErr, tt.want)
			}
		})
	}
}

// TestAgentModChangeRejectsMalformedIDBeforeProviderDetection verifies that a
// malformed module ID is rejected from argv alone: no LLM provider is resolved
// (no "Using LLM provider" line) before the usage error.
func TestAgentModChangeRejectsMalformedIDBeforeProviderDetection(t *testing.T) {
	t.Parallel()

	app, err := NewApp(Dependencies{
		Config: &staticConfigProvider{cfg: config.DefaultConfig()},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	var stderr bytes.Buffer
	root := &cobra.Command{Use: "invowk", SilenceErrors: true, SilenceUsage: true}
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.AddCommand(newAgentCommand(app, &rootFlagValues{}))
	installUsageGuards(root)
	root.SetArgs([]string{"agent", "mod", "change", "bad id", "describe it", "--llm-provider", "claude"})
	runErr := root.Execute()

	if got := exitCodeForError(runErr); got != ExitCodeUsage {
		t.Fatalf("exit code = %d (error: %v), want %d", got, runErr, ExitCodeUsage)
	}
	if strings.Contains(stderr.String(), "Using LLM provider") {
		t.Fatalf("stderr = %q, want no LLM provider detection before the ID check", stderr.String())
	}
}

// TestAuditLLMFlagExitCode verifies that `audit` keeps status 2 for an invalid
// LLM flag value handled by the helpers it shares with `agent`.
func TestAuditLLMFlagExitCode(t *testing.T) {
	t.Parallel()

	app, err := NewApp(Dependencies{
		Config: &staticConfigProvider{cfg: config.DefaultConfig()},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	got, runErr := executeWithUsageGuards(t, newAuditCommand(app, &rootFlagValues{}),
		"audit", t.TempDir(), "--llm-provider", "claude", "--llm-concurrency", "-1")
	if got != auditExitError {
		t.Fatalf("exit code = %d (error: %v), want %d", got, runErr, auditExitError)
	}
}

func TestLLMExitCodesValidate(t *testing.T) {
	t.Parallel()

	for name, codes := range map[string]llmExitCodes{"audit": auditLLMExitCodes, "agent": agentLLMExitCodes} {
		if err := codes.Validate(); err != nil {
			t.Fatalf("%s codes Validate() = %v, want nil", name, err)
		}
	}
	if err := (llmExitCodes{usage: 256, failure: 1}).Validate(); err == nil {
		t.Fatal("Validate() accepted an out-of-range usage code")
	}
	if err := (llmExitCodes{usage: 2, failure: -1}).Validate(); err == nil {
		t.Fatal("Validate() accepted an out-of-range failure code")
	}
}
