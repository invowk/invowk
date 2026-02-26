// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTrCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newTrCommand()
	if got := cmd.Name(); got != "tr" {
		t.Errorf("Name() = %q, want %q", got, "tr")
	}
}

func TestTrCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newTrCommand()
	flags := cmd.SupportedFlags()

	// Should include common flags
	expectedFlags := map[string]bool{"d": false, "s": false, "c": false}
	for _, f := range flags {
		if _, exists := expectedFlags[f.Name]; exists {
			expectedFlags[f.Name] = true
		}
	}

	for name, found := range expectedFlags {
		if !found {
			t.Errorf("SupportedFlags() should include -%s flag", name)
		}
	}
}

func TestTrCommand_Run_BasicTranslate(t *testing.T) {
	t.Parallel()

	stdinContent := "hello world\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	// Replace 'e' with 'a' and 'o' with 'u'
	err := cmd.Run(ctx, []string{"tr", "eo", "au"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output != "hallu wurld\n" {
		t.Errorf("output = %q, want %q", output, "hallu wurld\n")
	}
}

func TestTrCommand_Run_Lowercase(t *testing.T) {
	t.Parallel()

	stdinContent := "HELLO WORLD\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	err := cmd.Run(ctx, []string{"tr", "A-Z", "a-z"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output != "hello world\n" {
		t.Errorf("output = %q, want %q", output, "hello world\n")
	}
}

func TestTrCommand_Run_Uppercase(t *testing.T) {
	t.Parallel()

	stdinContent := "hello world\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	err := cmd.Run(ctx, []string{"tr", "a-z", "A-Z"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output != "HELLO WORLD\n" {
		t.Errorf("output = %q, want %q", output, "HELLO WORLD\n")
	}
}

func TestTrCommand_Run_Delete(t *testing.T) {
	t.Parallel()

	stdinContent := "hello123world456\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	// Delete digits
	err := cmd.Run(ctx, []string{"tr", "-d", "0-9"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output != "helloworld\n" {
		t.Errorf("output = %q, want %q", output, "helloworld\n")
	}
}

func TestTrCommand_Run_Squeeze(t *testing.T) {
	t.Parallel()

	stdinContent := "helllllo   wooorld\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	// Squeeze repeated characters
	err := cmd.Run(ctx, []string{"tr", "-s", "lo "})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output != "helo world\n" {
		t.Errorf("output = %q, want %q", output, "helo world\n")
	}
}

func TestTrCommand_Run_Complement(t *testing.T) {
	t.Parallel()

	stdinContent := "hello123world\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	// Delete everything except digits and newline
	err := cmd.Run(ctx, []string{"tr", "-c", "-d", "0-9\n"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output != "123\n" {
		t.Errorf("output = %q, want %q", output, "123\n")
	}
}

func TestTrCommand_Run_ROT13(t *testing.T) {
	t.Parallel()

	stdinContent := "hello\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	err := cmd.Run(ctx, []string{"tr", "a-zA-Z", "n-za-mN-ZA-M"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output != "uryyb\n" {
		t.Errorf("output = %q, want %q", output, "uryyb\n")
	}
}

func TestTrCommand_Run_EmptyInput(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	err := cmd.Run(ctx, []string{"tr", "a", "b"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty for empty input, got %q", stdout.String())
	}
}

func TestTrCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader("test\n"),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	// tr requires SET1 argument
	err := cmd.Run(ctx, []string{"tr"})

	if err == nil {
		t.Fatal("Run() should return error when no arguments provided")
	}

	// Error should have [uroot] prefix
	if !strings.HasPrefix(err.Error(), "[uroot] tr:") {
		t.Errorf("error should have [uroot] tr: prefix, got: %v", err)
	}
}

func TestTrCommand_Run_DeleteWithoutSet(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader("test\n"),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	// -d requires SET1
	err := cmd.Run(ctx, []string{"tr", "-d"})

	if err == nil {
		t.Fatal("Run() should return error when -d without set")
	}
}

func TestTrCommand_Run_SpecialChars(t *testing.T) {
	t.Parallel()

	stdinContent := "hello\tworld\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	// Replace tab with space
	err := cmd.Run(ctx, []string{"tr", "\t", " "})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output != "hello world\n" {
		t.Errorf("output = %q, want %q", output, "hello world\n")
	}
}

func TestTrCommand_Run_NewlineToSpace(t *testing.T) {
	t.Parallel()

	stdinContent := "line1\nline2\nline3\n"

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Stdin:     strings.NewReader(stdinContent),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newTrCommand()
	err := cmd.Run(ctx, []string{"tr", "\n", " "})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if output != "line1 line2 line3 " {
		t.Errorf("output = %q, want %q", output, "line1 line2 line3 ")
	}
}
