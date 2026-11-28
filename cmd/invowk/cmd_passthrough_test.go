// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type (
	passthroughStubDiscovery struct {
		set *discovery.DiscoveredCommandSet
	}

	passthroughRecordingService struct {
		set      *discovery.DiscoveredCommandSet
		requests []ExecuteRequest
	}
)

func (d *passthroughStubDiscovery) DiscoverCommandSet(context.Context) (discovery.CommandSetResult, error) {
	return discovery.CommandSetResult{Set: d.set}, nil
}

func (d *passthroughStubDiscovery) DiscoverAndValidateCommandSet(context.Context) (discovery.CommandSetResult, error) {
	return discovery.CommandSetResult{Set: d.set}, nil
}

func (d *passthroughStubDiscovery) DiscoverModules(context.Context) (discovery.ModuleListResult, error) {
	return discovery.ModuleListResult{}, nil
}

func (d *passthroughStubDiscovery) GetCommand(_ context.Context, name string) (discovery.LookupResult, error) {
	if d.set == nil {
		return discovery.LookupResult{}, nil
	}
	return discovery.LookupResult{Command: d.set.ByName[invowkfile.CommandName(name)]}, nil
}

func (s *passthroughRecordingService) Execute(_ context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	s.requests = append(s.requests, req)
	return ExecuteResult{}, nil, nil
}

func (s *passthroughRecordingService) ResolveCommand(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	return nil, req, nil, nil
}

func (s *passthroughRecordingService) ResolveWatchPlan(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, commandsvc.WatchPlan, []discovery.Diagnostic, error) {
	return nil, req, commandsvc.WatchPlan{}, nil, nil
}

func (s *passthroughRecordingService) ResolveFromSource(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	if s.set != nil {
		for _, info := range s.set.BySource[req.FromSource] {
			if string(info.SimpleName) == req.Name || string(info.Name) == req.Name {
				resolved := req
				resolved.Name = string(info.Name)
				resolved.ResolvedCommand = info
				return info, resolved, nil, nil
			}
		}
	}
	return nil, req, nil, nil
}

func newPassthroughTestSet(t *testing.T) *discovery.DiscoveredCommandSet {
	t.Helper()

	tmpDir := t.TempDir()
	cstMod := invowkmod.ModuleID("io.example.cst")
	set := discovery.NewDiscoveredCommandSet()

	wrap := testCommandInfo(t, "wrap", "echo wrap")
	wrap.Command.Passthrough = true
	set.Add(wrap)

	normal := testCommandInfo(t, "normal", "echo normal")
	normal.Command.Flags = []invowkfile.Flag{{Name: "flag", Description: "A flag", Type: invowkfile.FlagTypeBool}}
	set.Add(normal)

	btrfsSub := testCommandInfo(t, "btrfs sub", "echo btrfs")
	btrfsSub.Command.Passthrough = true
	set.Add(btrfsSub)

	invDeploy := testCommandInfo(t, "deploy", "echo inv deploy")
	invDeploy.FilePath = types.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))
	set.Add(invDeploy)

	invDeployDev := testCommandInfo(t, "deploy dev", "echo inv deploy dev")
	invDeployDev.FilePath = types.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))
	set.Add(invDeployDev)

	cstDeploy := testCommandInfo(t, "deploy", "echo cst deploy")
	cstDeploy.Name = "cst deploy"
	cstDeploy.SimpleName = "deploy"
	cstDeploy.Source = discovery.SourceModule
	cstDeploy.SourceID = "cst"
	cstDeploy.ModuleID = &cstMod
	cstDeploy.FilePath = types.FilesystemPath(filepath.Join(tmpDir, "cst.invowkmod", "invowkfile.cue"))
	cstDeploy.Command.Passthrough = true
	set.Add(cstDeploy)

	cstDoctor := testCommandInfo(t, "doctor", "echo cst doctor")
	cstDoctor.Name = "cst doctor"
	cstDoctor.SimpleName = "doctor"
	cstDoctor.Source = discovery.SourceModule
	cstDoctor.SourceID = "cst"
	cstDoctor.ModuleID = &cstMod
	cstDoctor.FilePath = types.FilesystemPath(filepath.Join(tmpDir, "cst.invowkmod", "invowkfile.cue"))
	cstDoctor.Command.Passthrough = true
	set.Add(cstDoctor)

	set.Analyze()
	return set
}

func newPassthroughTestApp(t *testing.T, set *discovery.DiscoveredCommandSet) (app *App, svc *passthroughRecordingService, stdout, stderr *bytes.Buffer) {
	t.Helper()

	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	svc = &passthroughRecordingService{set: set}
	app, err := NewApp(Dependencies{
		Config:      &staticConfigProvider{cfg: config.DefaultConfig()},
		Discovery:   &passthroughStubDiscovery{set: set},
		Commands:    svc,
		Diagnostics: &defaultDiagnosticRenderer{},
		Stdout:      stdout,
		Stderr:      stderr,
		Getenv:      func(string) string { return "" },
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	return app, svc, stdout, stderr
}

func TestResolvePassthroughTarget(t *testing.T) {
	t.Parallel()

	set := newPassthroughTestSet(t)

	tests := []struct {
		name        string
		fromFlag    string
		positionals []string
		wantMatched bool
		wantPath    []string
		wantPost    []string
	}{
		{
			name:        "direct leaf",
			positionals: []string{"wrap", "--help", "-h", "-v", "--foo=bar", "--"},
			wantMatched: true,
			wantPath:    []string{"wrap"},
			wantPost:    []string{"--help", "-h", "-v", "--foo=bar", "--"},
		},
		{
			name:        "multi-word passthrough subcommand",
			positionals: []string{"btrfs", "sub", "--help", "--json"},
			wantMatched: true,
			wantPath:    []string{"btrfs", "sub"},
			wantPost:    []string{"--help", "--json"},
		},
		{
			name:        "at-source disambiguation",
			positionals: []string{"@cst", "deploy", "--help", "-x"},
			wantMatched: true,
			wantPath:    []string{"@cst", "deploy"},
			wantPost:    []string{"--help", "-x"},
		},
		{
			name:        "ivk-from disambiguation",
			fromFlag:    "cst",
			positionals: []string{"deploy", "--help", "--json"},
			wantMatched: true,
			wantPath:    []string{"deploy"},
			wantPost:    []string{"--help", "--json"},
		},
		{
			name:        "module-qualified source and command",
			positionals: []string{"cst", "doctor", "--help", "--json"},
			wantMatched: true,
			wantPath:    []string{"cst", "doctor"},
			wantPost:    []string{"--help", "--json"},
		},
		{
			name:        "ivk-from with module-qualified form",
			fromFlag:    "cst",
			positionals: []string{"cst", "doctor", "--help"},
			wantMatched: true,
			wantPath:    []string{"cst", "doctor"},
			wantPost:    []string{"--help"},
		},
		{
			name:        "ambiguous without source is not matched",
			positionals: []string{"deploy", "--help"},
			wantMatched: false,
		},
		{
			name:        "non-passthrough command is not matched",
			positionals: []string{"normal", "--flag"},
			wantMatched: false,
		},
		{
			name:        "non-passthrough source command is not matched",
			fromFlag:    "invowkfile",
			positionals: []string{"deploy", "--help"},
			wantMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmdPath, postArgs, matched := resolvePassthroughTarget(set, tt.fromFlag, tt.positionals)
			if matched != tt.wantMatched {
				t.Fatalf("matched = %v, want %v (cmdPath=%v post=%v)", matched, tt.wantMatched, cmdPath, postArgs)
			}
			if !tt.wantMatched {
				return
			}
			if !slices.Equal(cmdPath, tt.wantPath) {
				t.Fatalf("cmdPath = %v, want %v", cmdPath, tt.wantPath)
			}
			if !slices.Equal(postArgs, tt.wantPost) {
				t.Fatalf("postArgs = %v, want %v", postArgs, tt.wantPost)
			}
		})
	}
}

func TestPassthroughExecutionRouting(t *testing.T) {
	t.Parallel()

	set := newPassthroughTestSet(t)

	tests := []struct {
		name     string
		args     []string
		wantName string
		wantArgs []string
		wantEnv  string
		wantDir  string
		wantSrc  discovery.SourceID
		wantCode int
	}{
		{
			name:     "direct passthrough with promoted pre-command flags",
			args:     []string{"cmd", "--ivk-env-var", "A=1", "-w", "sub", "wrap", "--help", "-h", "-v", "--foo=bar", "--", "--ivk-env-var", "B=2"},
			wantName: "wrap",
			wantArgs: []string{"--help", "-h", "-v", "--foo=bar", "--", "--ivk-env-var", "B=2"},
			wantEnv:  "1",
			wantDir:  "sub",
		},
		{
			name:     "at-source disambiguated passthrough",
			args:     []string{"cmd", "--ivk-env-var", "A=1", "@cst", "deploy", "--help", "--json"},
			wantName: "cst deploy",
			wantArgs: []string{"--help", "--json"},
			wantEnv:  "1",
			wantSrc:  "cst",
		},
		{
			name:     "ivk-from disambiguated passthrough",
			args:     []string{"cmd", "--ivk-from", "cst", "deploy", "--help", "--json"},
			wantName: "cst deploy",
			wantArgs: []string{"--help", "--json"},
			wantSrc:  "cst",
		},
		{
			name:     "module-qualified passthrough",
			args:     []string{"cmd", "--ivk-env-var", "A=1", "cst", "doctor", "--help", "--json"},
			wantName: "cst",
			wantArgs: []string{"doctor", "--help", "--json"},
			wantEnv:  "1",
		},
		{
			name:     "post-command ivk-config does not hijack parent configPath",
			args:     []string{"cmd", "wrap", "--ivk-config", "child.cue", "--help"},
			wantName: "wrap",
			wantArgs: []string{"--ivk-config", "child.cue", "--help"},
		},
		{
			name:     "unknown pre-command flag rejected with usage exit code",
			args:     []string{"cmd", "--bogus=1", "wrap", "--help"},
			wantCode: int(ExitCodeUsage),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app, svc, _, _ := newPassthroughTestApp(t, set)
			rootCmd := NewRootCommand(app)
			err := executeWithFang(t.Context(), app, rootCmd, tt.args)
			if tt.wantCode != 0 {
				if err == nil {
					t.Fatal("executeWithFang() = nil, want usage error")
				}
				if got := int(exitCodeForError(err)); got != tt.wantCode {
					t.Fatalf("exitCodeForError(%v) = %d, want %d", err, got, tt.wantCode)
				}
				if len(svc.requests) != 0 {
					t.Fatalf("Execute should not run on invalid pre-command flag, got %d requests", len(svc.requests))
				}
				return
			}
			if err != nil {
				t.Fatalf("executeWithFang() error = %v", err)
			}
			if len(svc.requests) != 1 {
				t.Fatalf("Execute requests = %d, want 1", len(svc.requests))
			}
			req := svc.requests[0]
			if req.Name != tt.wantName {
				t.Fatalf("req.Name = %q, want %q", req.Name, tt.wantName)
			}
			if !slices.Equal(req.Args, tt.wantArgs) {
				t.Fatalf("req.Args = %v, want %v", req.Args, tt.wantArgs)
			}
			if tt.wantEnv != "" && req.EnvVars["A"] != tt.wantEnv {
				t.Fatalf("req.EnvVars[A] = %q, want %q", req.EnvVars["A"], tt.wantEnv)
			}
			if tt.wantDir != "" && string(req.Workdir) != tt.wantDir {
				t.Fatalf("req.Workdir = %q, want %q", req.Workdir, tt.wantDir)
			}
			if tt.wantSrc != "" && req.FromSource != tt.wantSrc {
				t.Fatalf("req.FromSource = %q, want %q", req.FromSource, tt.wantSrc)
			}
			if req.ConfigPath != "" {
				t.Fatalf("req.ConfigPath = %q, want empty", req.ConfigPath)
			}
		})
	}
}

func TestPassthroughHelpAndListing(t *testing.T) {
	t.Parallel()

	set := newPassthroughTestSet(t)

	t.Run("listCommands annotates passthrough commands", func(t *testing.T) {
		t.Parallel()

		app, _, stdout, _ := newPassthroughTestApp(t, set)
		rootCmd := NewRootCommand(app)
		if err := executeWithFang(t.Context(), app, rootCmd, []string{"cmd"}); err != nil {
			t.Fatalf("executeWithFang(cmd) error = %v", err)
		}
		out := stdout.String()
		for line := range strings.SplitSeq(out, "\n") {
			if strings.Contains(line, "wrap") && !strings.Contains(line, "[passthrough]") {
				t.Fatalf("wrap listing line missing [passthrough]: %q", line)
			}
			if strings.Contains(line, "normal") && strings.Contains(line, "[passthrough]") {
				t.Fatalf("normal listing line should not contain [passthrough]: %q", line)
			}
		}
	})

	t.Run("invowk help cmd prints command help without executing", func(t *testing.T) {
		t.Parallel()

		app, svc, stdout, _ := newPassthroughTestApp(t, set)
		rootCmd := NewRootCommand(app)
		cmdCmd := findCmdSubcommand(rootCmd)
		if cmdCmd == nil {
			t.Fatal("cmd subcommand not found")
		}
		// Register discovered leaves explicitly since os.Args is the test runner's argv.
		registerDiscoveredCommands(t.Context(), app, &rootFlagValues{}, &cmdFlagValues{}, cmdCmd)

		if err := executeWithFang(t.Context(), app, rootCmd, []string{"help", "cmd", "wrap"}); err != nil {
			t.Fatalf("executeWithFang(help cmd wrap) error = %v", err)
		}
		if len(svc.requests) != 0 {
			t.Fatalf("invowk help cmd wrap executed command (%d requests)", len(svc.requests))
		}
		if !strings.Contains(stdout.String(), "Passthrough:") {
			t.Fatalf("help output missing Passthrough description:\n%s", stdout.String())
		}
	})

	t.Run("invowk cmd --help before command prints cmd help without executing", func(t *testing.T) {
		t.Parallel()

		app, svc, stdout, _ := newPassthroughTestApp(t, set)
		rootCmd := NewRootCommand(app)
		if err := executeWithFang(t.Context(), app, rootCmd, []string{"cmd", "--help", "wrap"}); err != nil {
			t.Fatalf("executeWithFang(cmd --help wrap) error = %v", err)
		}
		if len(svc.requests) != 0 {
			t.Fatalf("invowk cmd --help wrap executed command (%d requests)", len(svc.requests))
		}
		if !strings.Contains(stdout.String(), "Execute commands defined in invowkfiles") {
			t.Fatalf("cmd --help output unexpected:\n%s", stdout.String())
		}
	})
}
