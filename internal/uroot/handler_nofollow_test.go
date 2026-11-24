// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// These tests cover the second hook of design D2 (virtual-path-no-follow-
// operands): which hook each operand takes, and the refusals of a context that
// sets only one of them.

func TestHandlerContextResolveNoFollowPath(t *testing.T) {
	t.Parallel()
	follow := func(_, path string) (string, error) { return "/follow/" + path, nil }
	noFollow := func(_, path string) (string, error) { return "/nofollow/" + path, nil }

	t.Run("both hooks", func(t *testing.T) {
		t.Parallel()
		hc := &HandlerContext{Dir: "/work", ValidatePath: follow, ValidateNoFollowPath: noFollow}
		if got, err := hc.ResolveNoFollowPath("a"); err != nil || got != "/nofollow/a" {
			t.Errorf("ResolveNoFollowPath(a) = (%q, %v), want /nofollow/a", got, err)
		}
		if got, err := hc.ResolvePath("a"); err != nil || got != "/follow/a" {
			t.Errorf("ResolvePath(a) = (%q, %v), want /follow/a", got, err)
		}
	})
	t.Run("only ValidatePath", func(t *testing.T) {
		t.Parallel()
		hc := &HandlerContext{Dir: "/work", ValidatePath: follow}
		got, err := hc.ResolveNoFollowPath("a")
		if !errors.Is(err, errNoFollowValidatorMissing) || !strings.Contains(err.Error(), "a") {
			t.Errorf("ResolveNoFollowPath(a) = (%q, %v), want errNoFollowValidatorMissing naming a", got, err)
		}
	})
	t.Run("only ValidateNoFollowPath", func(t *testing.T) {
		t.Parallel()
		hc := &HandlerContext{Dir: "/work", ValidateNoFollowPath: noFollow}
		got, err := hc.ResolvePath("a")
		if !errors.Is(err, errFollowValidatorMissing) || !strings.Contains(err.Error(), "a") {
			t.Errorf("ResolvePath(a) = (%q, %v), want errFollowValidatorMissing naming a", got, err)
		}
		if got, err := hc.ResolveNoFollowPath("a"); err != nil || got != "/nofollow/a" {
			t.Errorf("ResolveNoFollowPath(a) = (%q, %v), want /nofollow/a", got, err)
		}
	})
	t.Run("neither hook", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		hc := &HandlerContext{Dir: dir}
		if got, err := hc.ResolveNoFollowPath("a"); err != nil || got != filepath.Join(dir, "a") {
			t.Errorf("ResolveNoFollowPath(a) = (%q, %v), want %s", got, err, filepath.Join(dir, "a"))
		}
		abs := filepath.Join(dir, "b")
		if got, err := hc.ResolveNoFollowPath(abs); err != nil || got != abs {
			t.Errorf("ResolveNoFollowPath(%s) = (%q, %v), want it unchanged", abs, got, err)
		}
	})
}

// TestUrootCommandOperandsOnlyNoFollowHook: a context with only the no-follow
// hook still goes through the operand collector, and an operand that needs
// the follow hook is refused instead of used unvalidated.
func TestUrootCommandOperandsOnlyNoFollowHook(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	validator := &operandTestValidator{root: root}
	var stdout, stderr bytes.Buffer
	hc := &HandlerContext{
		Stdin:                strings.NewReader(""),
		Stdout:               &stdout,
		Stderr:               &stderr,
		Dir:                  root,
		LookupEnv:            os.LookupEnv,
		ValidateNoFollowPath: validator.noFollow,
	}
	if _, ok := pathValidatingHandlerContext(WithHandlerContext(t.Context(), hc)); !ok {
		t.Fatal("a context with only ValidateNoFollowPath is not a validating context")
	}

	r, wrapper := newRecordingRegistry("rm")
	if err := r.Run(WithHandlerContext(t.Context(), hc), "rm", []string{"rm", "a", "-x"}); err != nil {
		t.Fatalf("rm a -x: %v", err)
	}
	received, called := wrapper.received()
	want := []string{"rm", "---", filepath.Join(root, "a"), filepath.Join(root, "-x")}
	if !called || !slices.Equal(received, want) {
		t.Errorf("rm a -x: received (%v) %q, want %q", called, received, want)
	}

	r, wrapper = newRecordingRegistry("cat")
	err := r.Run(WithHandlerContext(t.Context(), hc), "cat", []string{"cat", "a"})
	if !errors.Is(err, errFollowValidatorMissing) {
		t.Errorf("cat a = %v, want errFollowValidatorMissing", err)
	}
	if _, called := wrapper.received(); called {
		t.Error("cat ran with an operand no hook validated")
	}
}

// TestLnLinkNameTakesTheNoFollowHook: ln resolves its link name with the
// no-follow hook, a hard link's target with the follow hook, and stores a
// symbolic link's target as written.
func TestLnLinkNameTakesTheNoFollowHook(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		args  []string
		calls []operandTestCall
	}{
		{
			name:  "symbolic",
			args:  []string{"ln", "-s", "target.txt", "link"},
			calls: []operandTestCall{{path: "link", follow: false}},
		},
		{
			name:  "symbolic force",
			args:  []string{"ln", "-sf", "target.txt", "link"},
			calls: []operandTestCall{{path: "link", follow: false}},
		},
		{
			name:  "hard",
			args:  []string{"ln", "target.txt", "link"},
			calls: []operandTestCall{{path: "link", follow: false}, {path: "target.txt", follow: true}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "target.txt"), []byte("target"), 0o600); err != nil {
				t.Fatal(err)
			}
			validator := &operandTestValidator{root: root}
			var stdout, stderr bytes.Buffer
			ctx := WithHandlerContext(t.Context(), validator.handler(root, &stdout, &stderr))
			if err := BuildDefaultRegistry().Run(ctx, "ln", tt.args); err != nil {
				t.Fatalf("%q: %v (%s)", tt.args, err, stderr.String())
			}
			if got := validator.recorded(); !slices.Equal(got, tt.calls) {
				t.Errorf("%q: hook calls %v, want %v", tt.args, got, tt.calls)
			}
		})
	}

	t.Run("only ValidatePath refuses", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		validator := &operandTestValidator{root: root}
		hc := &HandlerContext{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Dir: root, ValidatePath: validator.follow}
		err := BuildDefaultRegistry().Run(WithHandlerContext(t.Context(), hc), "ln", []string{"ln", "-sf", "target.txt", "link"})
		if !errors.Is(err, errNoFollowValidatorMissing) {
			t.Errorf("ln -sf = %v, want errNoFollowValidatorMissing", err)
		}
		if _, statErr := os.Lstat(filepath.Join(root, "link")); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("ln created the link after the refusal (lstat %v)", statErr)
		}
	})
}
