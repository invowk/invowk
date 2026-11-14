// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/pkg/types"
)

// usageGuardTree is a synthetic Cobra tree wired by installUsageGuards.
type usageGuardTree struct {
	root           *cobra.Command
	stdout, stderr *bytes.Buffer
	arbitraryRuns  *[]string
}

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	plain := errors.New("boom")
	svcErr := newServiceError(plain, 0, "")

	tests := []struct {
		name string
		err  error
		want types.ExitCode
	}{
		{name: "nil", err: nil, want: 0},
		{name: "ExitError 0", err: &ExitError{Code: 0}, want: 0},
		{name: "ExitError 1", err: &ExitError{Code: 1}, want: 1},
		{name: "ExitError 2", err: &ExitError{Code: 2}, want: 2},
		{name: "ExitError 42", err: &ExitError{Code: 42}, want: 42},
		{name: "UsageError", err: &UsageError{Err: plain}, want: ExitCodeUsage},
		{name: "UsageError wrapping ServiceError", err: newUsageError(svcErr), want: ExitCodeUsage},
		{name: "ExitError wrapping UsageError keeps explicit code", err: &ExitError{Code: 7, Err: &UsageError{Err: plain}}, want: 7},
		{name: "wrapped UsageError", err: fmt.Errorf("ctx: %w", newUsageError(plain)), want: ExitCodeUsage},
		{name: "wrapped plain error", err: fmt.Errorf("ctx: %w", plain), want: 1},
		{name: "plain error", err: plain, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := exitCodeForError(tt.err); got != tt.want {
				t.Fatalf("exitCodeForError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestNewUsageError(t *testing.T) {
	t.Parallel()

	if got := newUsageError(nil); got != nil {
		t.Fatalf("newUsageError(nil) = %v, want nil", got)
	}

	plain := errors.New("unknown flag: --bogus")
	wrapped := newUsageError(plain)
	usageErr, ok := errors.AsType[*UsageError](wrapped)
	if !ok {
		t.Fatalf("newUsageError(plain) = %T, want *UsageError", wrapped)
	}
	if !errors.Is(wrapped, plain) || !errors.Is(usageErr.Err, plain) {
		t.Fatal("UsageError must unwrap to the original error")
	}
	if wrapped.Error() != plain.Error() {
		t.Fatalf("Error() = %q, want unchanged %q", wrapped.Error(), plain.Error())
	}

	exitErr := &ExitError{Code: 42, Err: plain}
	for name, err := range map[string]error{"ExitError": exitErr, "wrapped ExitError": fmt.Errorf("ctx: %w", exitErr)} {
		got := newUsageError(err)
		if _, isUsage := errors.AsType[*UsageError](got); isUsage || !errors.Is(got, exitErr) {
			t.Fatalf("newUsageError(%s) = %#v, want it returned unchanged", name, got)
		}
		if code := exitCodeForError(got); code != 42 {
			t.Fatalf("exitCodeForError(newUsageError(%s)) = %d, want 42", name, code)
		}
	}

	again, ok := errors.AsType[*UsageError](newUsageError(wrapped))
	if !ok {
		t.Fatal("newUsageError(UsageError) lost the *UsageError")
	}
	if _, nested := errors.AsType[*UsageError](again.Err); nested {
		t.Fatal("newUsageError(UsageError) must not double-wrap")
	}
}

// newUsageGuardTree builds a synthetic tree: a runnable help-fallback group
// (`config`), a non-runnable group (`module`), a leaf with MaximumNArgs(1)
// under it, and a parent with explicit ArbitraryArgs (`passthrough`).
func newUsageGuardTree(t *testing.T) usageGuardTree {
	t.Helper()

	var stdout, stderr bytes.Buffer
	var runs []string

	root := &cobra.Command{Use: "invowk", SilenceErrors: true, SilenceUsage: true}
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	config := &cobra.Command{Use: "config", Short: "Manage configuration", RunE: func(c *cobra.Command, _ []string) error { return c.Help() }}
	config.AddCommand(
		&cobra.Command{Use: "show", Short: "Show configuration", RunE: func(*cobra.Command, []string) error { return nil }},
		&cobra.Command{Use: "path", Short: "Show config path", RunE: func(*cobra.Command, []string) error { return nil }},
		&cobra.Command{Use: "secret", Short: "Hidden", Hidden: true, RunE: func(*cobra.Command, []string) error { return nil }},
	)

	module := &cobra.Command{Use: "module", Short: "Manage modules"}
	module.AddCommand(
		&cobra.Command{Use: "list", Short: "List modules", Args: cobra.MaximumNArgs(1), RunE: func(*cobra.Command, []string) error { return nil }},
		&cobra.Command{Use: "create", Short: "Create a module", RunE: func(*cobra.Command, []string) error { return nil }},
	)

	passthrough := &cobra.Command{
		Use:   "passthrough",
		Short: "Accepts any tokens",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			runs = append(runs, strings.Join(args, " "))
			return nil
		},
	}
	passthrough.AddCommand(&cobra.Command{Use: "child", RunE: func(*cobra.Command, []string) error { return nil }})

	root.AddCommand(config, module, passthrough)
	installUsageGuards(root)

	return usageGuardTree{root: root, stdout: &stdout, stderr: &stderr, arbitraryRuns: &runs}
}

func (tree usageGuardTree) execute(args ...string) error {
	tree.root.SetArgs(args)
	return tree.root.Execute() //nolint:wrapcheck // test harness returns Cobra's error unchanged so assertions see the exact message
}

func TestInstallUsageGuards(t *testing.T) {
	t.Parallel()

	t.Run("bare root shows help", func(t *testing.T) {
		t.Parallel()
		tree := newUsageGuardTree(t)
		if err := tree.execute(); err != nil {
			t.Fatalf("bare root error = %v, want nil", err)
		}
		if !strings.Contains(tree.stdout.String(), "Available Commands") {
			t.Fatalf("bare root stdout = %q, want help", tree.stdout.String())
		}
	})

	t.Run("unknown root command is a usage error with Cobra's message", func(t *testing.T) {
		t.Parallel()
		tree := newUsageGuardTree(t)
		err := tree.execute("bogus")
		if got := exitCodeForError(err); got != ExitCodeUsage {
			t.Fatalf("exit code = %d (%v), want %d", got, err, ExitCodeUsage)
		}
		if want := `unknown command "bogus" for "invowk"`; err.Error() != want {
			t.Fatalf("error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("near-miss root command gets a suggestion", func(t *testing.T) {
		t.Parallel()
		tree := newUsageGuardTree(t)
		err := tree.execute("confg")
		if err == nil || !strings.Contains(err.Error(), "Did you mean this?\n\tconfig\n") {
			t.Fatalf("error = %v, want a config suggestion", err)
		}
	})

	for _, group := range []string{"config", "module"} {
		t.Run("bare "+group+" shows help", func(t *testing.T) {
			t.Parallel()
			tree := newUsageGuardTree(t)
			if err := tree.execute(group); err != nil {
				t.Fatalf("bare %s error = %v, want nil", group, err)
			}
			if !strings.Contains(tree.stdout.String(), "Usage:") {
				t.Fatalf("bare %s stdout = %q, want help", group, tree.stdout.String())
			}
		})

		t.Run(group+" with a token is a usage error", func(t *testing.T) {
			t.Parallel()
			tree := newUsageGuardTree(t)
			err := tree.execute(group, "bogus")
			if _, ok := errors.AsType[*UsageError](err); !ok {
				t.Fatalf("error = %T (%v), want *UsageError", err, err)
			}
			if want := fmt.Sprintf("unknown command %q for %q", "bogus", "invowk "+group); err.Error() != want {
				t.Fatalf("error = %q, want %q", err.Error(), want)
			}
			if tree.stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", tree.stdout.String())
			}
			if !strings.Contains(tree.stderr.String(), "Available commands:") {
				t.Fatalf("stderr = %q, want the subcommand list", tree.stderr.String())
			}
		})
	}

	t.Run("near-miss under a group gets a suggestion", func(t *testing.T) {
		t.Parallel()
		tree := newUsageGuardTree(t)
		err := tree.execute("config", "shwo")
		if err == nil || !strings.Contains(err.Error(), "Did you mean this?\n\tshow\n") {
			t.Fatalf("error = %v, want a show suggestion", err)
		}
	})

	t.Run("unknown flag on a nested child is a usage error", func(t *testing.T) {
		t.Parallel()
		tree := newUsageGuardTree(t)
		err := tree.execute("module", "list", "--bogus")
		if got := exitCodeForError(err); got != ExitCodeUsage {
			t.Fatalf("exit code = %d (%v), want %d", got, err, ExitCodeUsage)
		}
		if !strings.Contains(err.Error(), "unknown flag: --bogus") {
			t.Fatalf("error = %q, want the pflag message unchanged", err.Error())
		}
	})

	t.Run("help flag still succeeds", func(t *testing.T) {
		t.Parallel()
		tree := newUsageGuardTree(t)
		if err := tree.execute("module", "list", "--help"); err != nil {
			t.Fatalf("--help error = %v, want nil", err)
		}
	})

	t.Run("explicit Args validator error becomes a usage error with its text", func(t *testing.T) {
		t.Parallel()
		tree := newUsageGuardTree(t)
		err := tree.execute("module", "list", "a", "b")
		if _, ok := errors.AsType[*UsageError](err); !ok {
			t.Fatalf("error = %T (%v), want *UsageError", err, err)
		}
		if want := cobra.MaximumNArgs(1)(&cobra.Command{}, []string{"a", "b"}).Error(); err.Error() != want {
			t.Fatalf("error = %q, want %q", err.Error(), want)
		}
	})

	t.Run("explicit ArbitraryArgs parent still accepts any tokens", func(t *testing.T) {
		t.Parallel()
		tree := newUsageGuardTree(t)
		if err := tree.execute("passthrough", "x", "y"); err != nil {
			t.Fatalf("passthrough error = %v, want nil", err)
		}
		if got := *tree.arbitraryRuns; len(got) != 1 || got[0] != "x y" {
			t.Fatalf("passthrough runs = %v, want [x y]", got)
		}
	})
}

func TestRejectUnmatchedGroupToken(t *testing.T) {
	t.Parallel()

	group := &cobra.Command{Use: "test"}
	group.AddCommand(
		&cobra.Command{Use: "unit", Short: "Unit tests", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "other", Short: "Other tests", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "hidden", Hidden: true, Run: func(*cobra.Command, []string) {}},
	)
	var stdout, stderr bytes.Buffer
	group.SetOut(&stdout)
	group.SetErr(&stderr)

	if err := rejectUnmatchedGroupToken(group, nil); err != nil {
		t.Fatalf("no args error = %v, want nil", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("no args wrote stderr %q", stderr.String())
	}

	err := rejectUnmatchedGroupToken(group, []string{"uint", "x"})
	if got := exitCodeForError(err); got != ExitCodeUsage {
		t.Fatalf("exit code = %d, want %d", got, ExitCodeUsage)
	}
	wantMsg := "unknown command \"uint\" for \"test\"\n\nDid you mean this?\n\tunit\n"
	if err.Error() != wantMsg {
		t.Fatalf("error = %q, want %q", err.Error(), wantMsg)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	wantList := "Available commands:\n  other  Other tests\n  unit   Unit tests\n\n"
	if stderr.String() != wantList {
		t.Fatalf("stderr = %q, want %q", stderr.String(), wantList)
	}
}
