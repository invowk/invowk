// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestSeqCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newSeqCommand()
	if got := cmd.Name(); got != "seq" {
		t.Errorf("Name() = %q, want %q", got, "seq")
	}
}

func TestSeqCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newSeqCommand()
	flags := cmd.SupportedFlags()

	flagNames := make(map[string]bool)
	for _, f := range flags {
		flagNames[f.Name] = true
	}

	if !flagNames["s"] {
		t.Error("SupportedFlags() should include -s flag")
	}
	if !flagNames["w"] {
		t.Error("SupportedFlags() should include -w flag")
	}
}

func TestSeqCommand_Run_LastOnly(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	err := cmd.Run(ctx, []string{"seq", "5"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	want := "1\n2\n3\n4\n5\n"
	if stdout.String() != want {
		t.Errorf("got %q, want %q", stdout.String(), want)
	}
}

func TestSeqCommand_Run_FirstAndLast(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	err := cmd.Run(ctx, []string{"seq", "3", "7"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	want := "3\n4\n5\n6\n7\n"
	if stdout.String() != want {
		t.Errorf("got %q, want %q", stdout.String(), want)
	}
}

func TestSeqCommand_Run_WithIncrement(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	err := cmd.Run(ctx, []string{"seq", "1", "2", "9"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	want := "1\n3\n5\n7\n9\n"
	if stdout.String() != want {
		t.Errorf("got %q, want %q", stdout.String(), want)
	}
}

func TestSeqCommand_Run_CustomSeparator(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	err := cmd.Run(ctx, []string{"seq", "-s", ", ", "3"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	want := "1, 2, 3\n"
	if stdout.String() != want {
		t.Errorf("got %q, want %q", stdout.String(), want)
	}
}

func TestSeqCommand_Run_Reverse(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	err := cmd.Run(ctx, []string{"seq", "5", "-1", "1"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	want := "5\n4\n3\n2\n1\n"
	if stdout.String() != want {
		t.Errorf("got %q, want %q", stdout.String(), want)
	}
}

func TestSeqCommand_Run_EqualWidth(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	err := cmd.Run(ctx, []string{"seq", "-w", "1", "10"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 10 {
		t.Fatalf("got %d lines, want 10", len(lines))
	}

	// First line should be zero-padded to width of "10"
	if lines[0] != "01" {
		t.Errorf("first line = %q, want %q", lines[0], "01")
	}
	if lines[9] != "10" {
		t.Errorf("last line = %q, want %q", lines[9], "10")
	}
}

func TestSeqCommand_Run_SingleValue(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	err := cmd.Run(ctx, []string{"seq", "1"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	want := "1\n"
	if stdout.String() != want {
		t.Errorf("got %q, want %q", stdout.String(), want)
	}
}

func TestSeqCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	err := cmd.Run(ctx, []string{"seq"})

	if err == nil {
		t.Fatal("Run() should return error for missing operand")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] seq:") {
		t.Errorf("error should have [uroot] seq: prefix, got: %v", err)
	}
}

func TestSeqCommand_Run_ZeroIncrement(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	err := cmd.Run(ctx, []string{"seq", "1", "0", "5"})

	if err == nil {
		t.Fatal("Run() should return error for zero increment")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] seq:") {
		t.Errorf("error should have [uroot] seq: prefix, got: %v", err)
	}
}

func TestSeqCommand_Run_InvalidNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "non-numeric last", args: []string{"seq", "abc"}},
		{name: "non-numeric first", args: []string{"seq", "xyz", "10"}},
		{name: "non-numeric increment", args: []string{"seq", "1", "xyz", "10"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			ctx := WithHandlerContext(t.Context(), &HandlerContext{
				Stdin:     strings.NewReader(""),
				Stdout:    &stdout,
				Stderr:    &stderr,
				Dir:       t.TempDir(),
				LookupEnv: os.LookupEnv,
			})

			cmd := newSeqCommand()
			err := cmd.Run(ctx, tt.args)

			if err == nil {
				t.Fatal("Run() should return error for invalid number argument")
			}

			if !strings.HasPrefix(err.Error(), "[uroot] seq:") {
				t.Errorf("error should have [uroot] seq: prefix, got: %v", err)
			}

			if !strings.Contains(err.Error(), "invalid floating point argument") {
				t.Errorf("error should mention 'invalid floating point argument', got: %v", err)
			}
		})
	}
}

func TestSeqCommand_Run_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	var stdout, stderr bytes.Buffer
	ctx = WithHandlerContext(ctx, &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	// Large range — should exit early due to cancelled context
	err := cmd.Run(ctx, []string{"seq", "1", "1000000"})

	if err == nil {
		t.Fatal("Run() should return error for cancelled context")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] seq:") {
		t.Errorf("error should have [uroot] seq: prefix, got: %v", err)
	}
}

func TestSeqCommand_Run_EmptyRange(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSeqCommand()
	// first > last with positive increment: no output
	err := cmd.Run(ctx, []string{"seq", "5", "1", "3"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for empty range, got %q", stdout.String())
	}
}
