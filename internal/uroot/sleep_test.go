// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSleepCommand_Name(t *testing.T) {
	t.Parallel()

	cmd := newSleepCommand()
	if got := cmd.Name(); got != "sleep" {
		t.Errorf("Name() = %q, want %q", got, "sleep")
	}
}

func TestSleepCommand_SupportedFlags(t *testing.T) {
	t.Parallel()

	cmd := newSleepCommand()
	flags := cmd.SupportedFlags()
	if len(flags) != 0 {
		t.Errorf("SupportedFlags() returned %d flags, want 0", len(flags))
	}
}

func TestSleepCommand_Run_ShortDuration(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSleepCommand()
	start := time.Now()
	err := cmd.Run(ctx, []string{"sleep", "0.001"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// Should complete within a reasonable time (well under 1 second)
	if elapsed > 2*time.Second {
		t.Errorf("sleep 0.001 took %v, expected much less", elapsed)
	}
}

func TestSleepCommand_Run_WithSuffix(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSleepCommand()
	// 1ms expressed as seconds with suffix
	err := cmd.Run(ctx, []string{"sleep", "0.001s"})
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
}

func TestSleepCommand_Run_ContextCancellation(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	ctx = WithHandlerContext(ctx, &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	// Cancel immediately to ensure sleep returns promptly
	cancel()

	cmd := newSleepCommand()
	start := time.Now()
	err := cmd.Run(ctx, []string{"sleep", "3600"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Run() should return error when context is cancelled")
	}

	// Should return very quickly due to cancellation
	if elapsed > 2*time.Second {
		t.Errorf("cancelled sleep took %v, expected much less", elapsed)
	}
}

func TestSleepCommand_Run_NoArgs(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSleepCommand()
	err := cmd.Run(ctx, []string{"sleep"})

	if err == nil {
		t.Fatal("Run() should return error for missing operand")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] sleep:") {
		t.Errorf("error should have [uroot] sleep: prefix, got: %v", err)
	}
}

func TestSleepCommand_Run_InvalidDuration(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	ctx := WithHandlerContext(context.Background(), &HandlerContext{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Dir:       t.TempDir(),
		LookupEnv: os.LookupEnv,
	})

	cmd := newSleepCommand()
	err := cmd.Run(ctx, []string{"sleep", "notanumber"})

	if err == nil {
		t.Fatal("Run() should return error for invalid duration")
	}

	if !strings.HasPrefix(err.Error(), "[uroot] sleep:") {
		t.Errorf("error should have [uroot] sleep: prefix, got: %v", err)
	}
}

func TestParseSleepDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "plain seconds", input: "5", want: 5 * time.Second},
		{name: "seconds suffix", input: "2s", want: 2 * time.Second},
		{name: "minutes suffix", input: "1m", want: 1 * time.Minute},
		{name: "hours suffix", input: "1h", want: 1 * time.Hour},
		{name: "fractional seconds", input: "0.5", want: 500 * time.Millisecond},
		{name: "fractional with suffix", input: "0.5s", want: 500 * time.Millisecond},
		{name: "empty string", input: "", wantErr: true},
		{name: "invalid", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseSleepDuration(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseSleepDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
