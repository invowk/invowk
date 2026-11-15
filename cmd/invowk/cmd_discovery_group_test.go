// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	// groupTestCommandService is a CommandService fake for discovered group nodes:
	// ResolveCommand reports ambiguity for the names in ambiguous and not-found
	// otherwise; ResolveFromSource and Execute record their requests.
	groupTestCommandService struct {
		ambiguous   map[string][]discovery.SourceID
		resolved    []string
		fromSource  []ExecuteRequest
		executeReqs []ExecuteRequest
	}

	// groupTestHarness holds a minimal `invowk cmd` tree with one discovered
	// group node.
	groupTestHarness struct {
		root           *cobra.Command
		svc            *groupTestCommandService
		stdout, stderr *bytes.Buffer
	}
)

func (s *groupTestCommandService) Execute(_ context.Context, req ExecuteRequest) (ExecuteResult, []discovery.Diagnostic, error) {
	s.executeReqs = append(s.executeReqs, req)
	return ExecuteResult{}, nil, nil
}

func (s *groupTestCommandService) ResolveCommand(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	s.resolved = append(s.resolved, req.Name)
	if sources, ok := s.ambiguous[req.Name]; ok {
		return nil, req, nil, &commandsvc.ClassifiedError{
			Err:  &commandsvc.AmbiguousCommandError{CommandName: invowkfile.CommandName(req.Name), Sources: sources},
			Kind: commandsvc.ErrorKindCommandAmbiguous,
		}
	}
	return nil, req, nil, &commandsvc.ClassifiedError{
		Err:  errors.New("command '" + req.Name + "' not found"),
		Kind: commandsvc.ErrorKindCommandNotFound,
	}
}

func (s *groupTestCommandService) ResolveWatchPlan(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, commandsvc.WatchPlan, []discovery.Diagnostic, error) {
	return nil, req, commandsvc.WatchPlan{}, nil, nil
}

func (s *groupTestCommandService) ResolveFromSource(_ context.Context, req ExecuteRequest) (*discovery.CommandInfo, ExecuteRequest, []discovery.Diagnostic, error) {
	s.fromSource = append(s.fromSource, req)
	info := &discovery.CommandInfo{
		Name:       invowkfile.CommandName(req.Name),
		SimpleName: invowkfile.CommandName(req.Name),
		SourceID:   req.FromSource,
	}
	return info, req, nil, nil
}

// newGroupTestHarness registers one discovered group node `deploy` (with a
// unique child `dev`) under a minimal `invowk cmd` and installs the usage
// guards, mirroring the production wiring.
func newGroupTestHarness(t *testing.T, prefixAmbiguous bool, ambiguous map[string][]discovery.SourceID) groupTestHarness {
	t.Helper()

	var stdout, stderr bytes.Buffer
	svc := &groupTestCommandService{ambiguous: ambiguous}
	app := &App{Commands: svc, Diagnostics: &defaultDiagnosticRenderer{}, stdout: &stdout, stderr: &stderr}
	rootFlags := &rootFlagValues{}
	cmdFlags := &cmdFlagValues{}

	root := &cobra.Command{Use: "invowk", SilenceErrors: true, SilenceUsage: true}
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	cmdCmd := &cobra.Command{Use: "cmd", Args: cobra.ArbitraryArgs, RunE: func(*cobra.Command, []string) error { return nil }}
	cmdCmd.PersistentFlags().StringVarP(&cmdFlags.fromSource, "ivk-from", "f", "", "")
	group := buildDiscoveredGroupCommand(app, rootFlags, cmdFlags, "deploy", "deploy", prefixAmbiguous)
	group.AddCommand(&cobra.Command{Use: "dev", Short: "Deploy to dev", RunE: func(*cobra.Command, []string) error { return nil }})
	cmdCmd.AddCommand(group)
	root.AddCommand(cmdCmd)
	installUsageGuards(root)

	return groupTestHarness{root: root, svc: svc, stdout: &stdout, stderr: &stderr}
}

func (h groupTestHarness) execute(args ...string) error {
	h.root.SetArgs(args)
	return h.root.Execute() //nolint:wrapcheck // test harness returns Cobra's error unchanged so assertions see the exact message
}

func TestDiscoveredGroup(t *testing.T) {
	t.Parallel()

	bothSources := []discovery.SourceID{discovery.SourceIDInvowkfile, "m"}

	t.Run("bare group shows help without resolving", func(t *testing.T) {
		t.Parallel()
		h := newGroupTestHarness(t, false, nil)
		if err := h.execute("cmd", "deploy"); err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
		if !strings.Contains(h.stdout.String(), "Usage:") {
			t.Fatalf("stdout = %q, want help", h.stdout.String())
		}
		if len(h.svc.resolved) != 0 {
			t.Fatalf("resolved = %v, want no resolution for a bare group", h.svc.resolved)
		}
	})

	t.Run("bare group whose prefix is an ambiguous command reports ambiguity", func(t *testing.T) {
		t.Parallel()
		h := newGroupTestHarness(t, true, map[string][]discovery.SourceID{"deploy": bothSources})
		err := h.execute("cmd", "deploy")
		if got := exitCodeForError(err); got != ExitCodeUsage {
			t.Fatalf("exit code = %d (%v), want %d", got, err, ExitCodeUsage)
		}
		if !strings.Contains(h.stderr.String(), "Ambiguous command") {
			t.Fatalf("stderr = %q, want the ambiguous-command message", h.stderr.String())
		}
	})

	t.Run("ivk-from routes the full path to the source", func(t *testing.T) {
		t.Parallel()
		h := newGroupTestHarness(t, false, map[string][]discovery.SourceID{"deploy prod": bothSources})
		if err := h.execute("cmd", "deploy", "prod", "--ivk-from", "invowkfile"); err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
		if len(h.svc.fromSource) != 1 || h.svc.fromSource[0].Name != "deploy" || !slices.Equal(h.svc.fromSource[0].Args, []string{"prod"}) {
			t.Fatalf("ResolveFromSource requests = %+v, want deploy [prod]", h.svc.fromSource)
		}
		if h.svc.fromSource[0].FromSource != discovery.SourceIDInvowkfile {
			t.Fatalf("FromSource = %q, want invowkfile", h.svc.fromSource[0].FromSource)
		}
		if len(h.svc.executeReqs) != 1 {
			t.Fatalf("Execute calls = %d, want 1", len(h.svc.executeReqs))
		}
	})

	t.Run("ambiguous full path through an unambiguous group", func(t *testing.T) {
		t.Parallel()
		h := newGroupTestHarness(t, false, map[string][]discovery.SourceID{"deploy prod": bothSources})
		err := h.execute("cmd", "deploy", "prod")
		if got := exitCodeForError(err); got != ExitCodeUsage {
			t.Fatalf("exit code = %d (%v), want %d", got, err, ExitCodeUsage)
		}
		if _, ok := errors.AsType[*AmbiguousCommandError](err); !ok {
			t.Fatalf("error = %T, want *AmbiguousCommandError in the chain", err)
		}
		if !slices.Equal(h.svc.resolved, []string{"deploy prod"}) {
			t.Fatalf("resolved = %v, want [deploy prod]", h.svc.resolved)
		}
		if !strings.Contains(h.stderr.String(), "Ambiguous command") {
			t.Fatalf("stderr = %q, want the ambiguous-command message", h.stderr.String())
		}
		if h.stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", h.stdout.String())
		}
	})

	t.Run("unmatched token is rejected with the subcommand list", func(t *testing.T) {
		t.Parallel()
		h := newGroupTestHarness(t, false, nil)
		err := h.execute("cmd", "deploy", "dve")
		if got := exitCodeForError(err); got != ExitCodeUsage {
			t.Fatalf("exit code = %d (%v), want %d", got, err, ExitCodeUsage)
		}
		want := "unknown command \"dve\" for \"invowk cmd deploy\"\n\nDid you mean this?\n\tdev\n"
		if err.Error() != want {
			t.Fatalf("error = %q, want %q", err.Error(), want)
		}
		if !strings.Contains(h.stderr.String(), "Available commands:\n  dev  Deploy to dev\n") {
			t.Fatalf("stderr = %q, want the subcommand list", h.stderr.String())
		}
		if h.stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", h.stdout.String())
		}
	})
}

// newRegistrationTestCommand builds a discovered command from sourceID.
func newRegistrationTestCommand(t *testing.T, name string, sourceID discovery.SourceID) *discovery.CommandInfo {
	t.Helper()
	info := testCommandInfo(t, name, "echo "+name)
	info.SourceID = sourceID
	if sourceID != discovery.SourceIDInvowkfile {
		info.Name = invowkfile.CommandName(string(sourceID) + " " + name)
	}
	return info
}

func registerTestCommands(t *testing.T, infos ...*discovery.CommandInfo) (*cobra.Command, *discovery.DiscoveredCommandSet) {
	t.Helper()
	set := discovery.NewDiscoveredCommandSet()
	for _, info := range infos {
		set.Add(info)
	}
	set.Analyze()

	app := &App{
		Discovery:   &stubDiscoveryService{validatedSet: discovery.CommandSetResult{Set: set}},
		Diagnostics: &defaultDiagnosticRenderer{},
		stdout:      &bytes.Buffer{},
		stderr:      &bytes.Buffer{},
	}
	cmdCmd := &cobra.Command{Use: "cmd"}
	registerDiscoveredCommands(t.Context(), app, &rootFlagValues{}, &cmdFlagValues{}, cmdCmd)
	return cmdCmd, set
}

func findSubcommand(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	t.Fatalf("%q has no subcommand %q", parent.Name(), name)
	return nil
}

func TestRegisterDiscoveredCommands(t *testing.T) {
	t.Parallel()

	orders := map[string][]string{
		"module child first": {"m", "invowkfile"},
		"invowkfile first":   {"invowkfile", "m"},
	}
	for name, order := range orders {
		t.Run("prefix leaf across sources, "+name, func(t *testing.T) {
			t.Parallel()

			bySource := map[string]*discovery.CommandInfo{
				"invowkfile": newRegistrationTestCommand(t, "w2", discovery.SourceIDInvowkfile),
				"m":          newRegistrationTestCommand(t, "w2 sub", "m"),
			}
			infos := []*discovery.CommandInfo{bySource[order[0]], bySource[order[1]]}
			before := slices.Clone(infos)

			cmdCmd, set := registerTestCommands(t, infos...)

			if !slices.Equal(set.Commands, before) {
				t.Fatal("registration reordered commandSet.Commands")
			}
			w2 := findSubcommand(t, cmdCmd, "w2")
			if w2.RunE == nil || w2.Short != "test command" {
				t.Fatalf("w2 = %q (RunE set: %v), want the runnable invowkfile leaf", w2.Short, w2.RunE != nil)
			}
			if err := w2.ValidateArgs([]string{"x"}); err != nil {
				t.Fatalf("w2.ValidateArgs([x]) = %v, want nil (leaf accepts extra tokens)", err)
			}
			findSubcommand(t, w2, "sub")
		})
	}
}

func TestDiscoveredLeafForwardsUnmatchedToken(t *testing.T) {
	t.Parallel()

	cmdCmd, _ := registerTestCommands(t,
		newRegistrationTestCommand(t, "w", discovery.SourceIDInvowkfile),
		newRegistrationTestCommand(t, "w sub", discovery.SourceIDInvowkfile),
	)
	root := &cobra.Command{Use: "invowk"}
	root.AddCommand(cmdCmd)
	installUsageGuards(root)

	leaf, args, err := root.Find([]string{"cmd", "w", "typo"})
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if leaf.Name() != "w" || !slices.Equal(args, []string{"typo"}) {
		t.Fatalf("Find() = %q %v, want leaf w with [typo]", leaf.Name(), args)
	}
	if err := leaf.ValidateArgs(args); err != nil {
		t.Fatalf("ValidateArgs(%v) = %v, want nil: typo is forwarded as $1", args, err)
	}
}
