// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"io"
	"slices"
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

// allDiscoverableMock returns a mock where every command is discoverable.
func allDiscoverableMock() *lookupDiscoveryServiceFunc {
	return &lookupDiscoveryServiceFunc{
		getCommand: func(_ context.Context, name string) (discovery.LookupResult, error) {
			return discovery.LookupResult{Command: &discovery.CommandInfo{Name: name}}, nil
		},
	}
}

func TestRenderDryRun_AllSections(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	req := ExecuteRequest{Name: "deploy"}
	cmdInfo := &discovery.CommandInfo{SourceID: "my-module.invowkmod"}
	cmd := &invowkfile.Command{}
	inv := &invowkfile.Invowkfile{}
	execCtx := runtime.NewExecutionContext(cmd, inv)
	execCtx.WorkDir = "/app"
	execCtx.Env.ExtraEnv = map[string]string{
		"INVOWK_CMD_NAME": "deploy",
		"ARG1":            "production",
		"DATABASE_URL":    "postgres://localhost/app",
	}
	resolved := appexec.RuntimeSelection{
		Mode: invowkfile.RuntimeVirtual,
		Impl: &invowkfile.Implementation{
			Script:  "echo deploying",
			Timeout: "30s",
		},
	}

	renderDryRun(&buf, req, cmdInfo, execCtx, resolved, []string{"build", "lint"})
	out := buf.String()

	// Verify all conditional sections appear.
	for _, token := range []string{
		"Dry Run",
		"Command:", "deploy",
		"Source:", "my-module.invowkmod",
		"Runtime:", "virtual",
		"WorkDir:", "/app",
		"Timeout:", "30s",
		"Exec deps:", "build, lint",
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
	execCtx := runtime.NewExecutionContext(cmd, inv)
	resolved := appexec.RuntimeSelection{
		Mode: invowkfile.RuntimeNative,
		Impl: nil,
	}

	renderDryRun(&buf, req, cmdInfo, execCtx, resolved, nil)
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

func TestCollectExecDepNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmdInfo  *discovery.CommandInfo
		implDeps *invowkfile.DependsOn
		// discoverable lists which command names exist; nil means all discoverable.
		discoverable map[string]bool
		want         []string
		wantErr      bool
	}{
		{
			name: "nil deps returns nil",
			cmdInfo: &discovery.CommandInfo{
				Command:    &invowkfile.Command{},
				Invowkfile: &invowkfile.Invowkfile{},
			},
			want: nil,
		},
		{
			name: "single alternative",
			cmdInfo: &discovery.CommandInfo{
				Command: &invowkfile.Command{
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []string{"build"}, Execute: true},
						},
					},
				},
				Invowkfile: &invowkfile.Invowkfile{},
			},
			want: []string{"build"},
		},
		{
			name: "multi alternative resolves to first discoverable",
			cmdInfo: &discovery.CommandInfo{
				Command: &invowkfile.Command{
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []string{"build-debug", "build-release"}, Execute: true},
						},
					},
				},
				Invowkfile: &invowkfile.Invowkfile{},
			},
			discoverable: map[string]bool{"build-release": true},
			want:         []string{"build-release"},
		},
		{
			name: "dedup across merge levels",
			cmdInfo: &discovery.CommandInfo{
				Command: &invowkfile.Command{
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []string{"build"}, Execute: true},
						},
					},
				},
				Invowkfile: &invowkfile.Invowkfile{
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []string{"build"}, Execute: true},
						},
					},
				},
			},
			want: []string{"build"},
		},
		{
			name: "dedup after OR resolution across levels",
			cmdInfo: &discovery.CommandInfo{
				Command: &invowkfile.Command{
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							// Root declares ["missing", "build"] — resolves to "build".
							{Alternatives: []string{"missing", "build"}, Execute: true},
						},
					},
				},
				Invowkfile: &invowkfile.Invowkfile{
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							// Invowkfile declares ["build"] — also resolves to "build".
							{Alternatives: []string{"build"}, Execute: true},
						},
					},
				},
			},
			// "missing" is not discoverable, so first dep resolves to "build".
			// Second dep also resolves to "build". Dedup keeps only one.
			discoverable: map[string]bool{"build": true},
			want:         []string{"build"},
		},
		{
			name: "empty alternatives skipped",
			cmdInfo: &discovery.CommandInfo{
				Command: &invowkfile.Command{
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []string{}, Execute: true},
							{Alternatives: []string{"test"}, Execute: true},
						},
					},
				},
				Invowkfile: &invowkfile.Invowkfile{},
			},
			want: []string{"test"},
		},
		{
			name: "non-execute deps excluded",
			cmdInfo: &discovery.CommandInfo{
				Command: &invowkfile.Command{
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []string{"lint"}, Execute: false},
							{Alternatives: []string{"build"}, Execute: true},
						},
					},
				},
				Invowkfile: &invowkfile.Invowkfile{},
			},
			want: []string{"build"},
		},
		{
			name: "multi alt with none discoverable returns error",
			cmdInfo: &discovery.CommandInfo{
				Command: &invowkfile.Command{
					DependsOn: &invowkfile.DependsOn{
						Commands: []invowkfile.CommandDependency{
							{Alternatives: []string{"missing-a", "missing-b"}, Execute: true},
						},
					},
				},
				Invowkfile: &invowkfile.Invowkfile{},
			},
			discoverable: map[string]bool{},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var disc DiscoveryService
			if tt.discoverable != nil {
				disc = &lookupDiscoveryServiceFunc{
					getCommand: func(_ context.Context, name string) (discovery.LookupResult, error) {
						if tt.discoverable[name] {
							return discovery.LookupResult{Command: &discovery.CommandInfo{Name: name}}, nil
						}
						return discovery.LookupResult{}, nil
					},
				}
			} else {
				disc = allDiscoverableMock()
			}

			svc := &commandService{
				stdout:    io.Discard,
				stderr:    io.Discard,
				ssh:       &sshServerController{},
				discovery: disc,
			}

			cmd := tt.cmdInfo.Command
			inv := tt.cmdInfo.Invowkfile
			execCtx := runtime.NewExecutionContext(cmd, inv)
			if tt.implDeps != nil {
				execCtx.SelectedImpl = &invowkfile.Implementation{DependsOn: tt.implDeps}
			}

			got, err := svc.collectExecDepNames(context.Background(), tt.cmdInfo, execCtx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("collectExecDepNames() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !slices.Equal(got, tt.want) {
				t.Errorf("collectExecDepNames() = %v, want %v", got, tt.want)
			}
		})
	}
}
