// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRegistry_Lookup_FallbackBehavior(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	// Register only specific commands
	r.Register(newMockCommand("cat"))
	r.Register(newMockCommand("ls"))

	tests := []struct {
		name     string
		cmdName  string
		wantOK   bool
		scenario string
	}{
		{
			name:     "registered_command_found",
			cmdName:  "cat",
			wantOK:   true,
			scenario: "u-root handles registered command",
		},
		{
			name:     "unregistered_git_fallback",
			cmdName:  "git",
			wantOK:   false,
			scenario: "falls back to system for git",
		},
		{
			name:     "unregistered_curl_fallback",
			cmdName:  "curl",
			wantOK:   false,
			scenario: "falls back to system for curl",
		},
		{
			name:     "unregistered_docker_fallback",
			cmdName:  "docker",
			wantOK:   false,
			scenario: "falls back to system for docker",
		},
		{
			name:     "unregistered_node_fallback",
			cmdName:  "node",
			wantOK:   false,
			scenario: "falls back to system for node",
		},
		{
			name:     "unregistered_python_fallback",
			cmdName:  "python",
			wantOK:   false,
			scenario: "falls back to system for python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd, ok := r.Lookup(tt.cmdName)
			if ok != tt.wantOK {
				t.Errorf("Lookup(%q) ok = %v, want %v (%s)",
					tt.cmdName, ok, tt.wantOK, tt.scenario)
			}
			if tt.wantOK && cmd == nil {
				t.Errorf("Lookup(%q) returned nil command when found", tt.cmdName)
			}
			if !tt.wantOK && cmd != nil {
				t.Errorf("Lookup(%q) returned non-nil command when not found", tt.cmdName)
			}
		})
	}
}

// TestRegistry_Run_NoSilentFallback verifies that when a registered u-root
// command fails, the error is propagated directly without any attempt to fall
// back to a system binary. This is critical for User Story 3's error handling.
//
// [US3] T045: No silent fallback - u-root errors are propagated
func TestRegistry_Run_NoSilentFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cmdName     string
		errMsg      string
		wantErrPart string
	}{
		{
			name:        "cp_permission_denied",
			cmdName:     "cp",
			errMsg:      "[uroot] cp: /protected: permission denied",
			wantErrPart: "[uroot] cp:",
		},
		{
			name:        "rm_file_not_found",
			cmdName:     "rm",
			errMsg:      "[uroot] rm: /missing: no such file or directory",
			wantErrPart: "[uroot] rm:",
		},
		{
			name:        "cat_is_directory",
			cmdName:     "cat",
			errMsg:      "[uroot] cat: /dir: is a directory",
			wantErrPart: "[uroot] cat:",
		},
		{
			name:        "mkdir_exists",
			cmdName:     "mkdir",
			errMsg:      "[uroot] mkdir: /existing: file exists",
			wantErrPart: "[uroot] mkdir:",
		},
		{
			name:        "grep_invalid_regex",
			cmdName:     "grep",
			errMsg:      "[uroot] grep: invalid regex: missing closing bracket",
			wantErrPart: "[uroot] grep:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := NewRegistry()

			// Create command that returns a specific error
			expectedErr := errors.New(tt.errMsg)
			cmd := &mockCommand{
				name: tt.cmdName,
				runFn: func(_ context.Context, _ []string) error {
					return expectedErr
				},
			}
			r.Register(cmd)

			// Run the command
			ctx := t.Context()
			err := r.Run(ctx, tt.cmdName, []string{tt.cmdName, "arg1", "arg2"})

			// Verify error is propagated (not nil, not wrapped differently)
			if err == nil {
				t.Fatal("Run should return error when command fails, not silently succeed")
			}

			// Verify the original error is preserved
			if !errors.Is(err, expectedErr) {
				t.Errorf("Run should propagate original error.\nGot: %v\nWant: %v", err, expectedErr)
			}

			// Verify error contains proper prefix
			if !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("error should contain %q prefix: %v", tt.wantErrPart, err)
			}
		})
	}
}

// =============================================================================
// POSIX Combined Short Flag Preprocessing Tests
// =============================================================================
// These tests verify that Registry.Run() correctly preprocesses POSIX-style
// combined short flags (e.g., "-sf" → "-s", "-f") for custom implementations,
// while skipping upstream wrappers that handle this internally.

// TestRegistry_Run_CombinedFlagsSplit verifies that combined boolean flags
// are split into individual flags for custom implementations.
func TestRegistry_Run_CombinedFlagsSplit(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := newMockCommand("testcmd")
	r.Register(cmd)

	ctx := t.Context()
	err := r.Run(ctx, "testcmd", []string{"testcmd", "-abc", "file.txt"})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if !cmd.called {
		t.Fatal("command was not called")
	}

	// "-abc" should be split into "-a", "-b", "-c"
	// Expected: ["testcmd", "-a", "-b", "-c", "file.txt"]
	want := []string{"testcmd", "-a", "-b", "-c", "file.txt"}
	if len(cmd.args) != len(want) {
		t.Fatalf("args length = %d, want %d\ngot:  %v\nwant: %v", len(cmd.args), len(want), cmd.args, want)
	}
	for i, arg := range cmd.args {
		if arg != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, want[i])
		}
	}
}

// TestRegistry_Run_SingleFlagsUnchanged verifies that single flags pass
// through unchanged (no mangling).
func TestRegistry_Run_SingleFlagsUnchanged(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := newMockCommand("testcmd")
	r.Register(cmd)

	ctx := t.Context()
	err := r.Run(ctx, "testcmd", []string{"testcmd", "-s", "-f", "target", "link"})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	want := []string{"testcmd", "-s", "-f", "target", "link"}
	if len(cmd.args) != len(want) {
		t.Fatalf("args length = %d, want %d\ngot:  %v\nwant: %v", len(cmd.args), len(want), cmd.args, want)
	}
	for i, arg := range cmd.args {
		if arg != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, want[i])
		}
	}
}

// TestRegistry_Run_MixedFlagsAndPositionalArgs verifies that combined flags
// mixed with positional arguments are handled correctly.
func TestRegistry_Run_MixedFlagsAndPositionalArgs(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := newMockCommand("grep")
	r.Register(cmd)

	ctx := t.Context()
	err := r.Run(ctx, "grep", []string{"grep", "-in", "pattern", "file.txt"})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// "-in" should be split to "-i", "-n"
	want := []string{"grep", "-i", "-n", "pattern", "file.txt"}
	if len(cmd.args) != len(want) {
		t.Fatalf("args length = %d, want %d\ngot:  %v\nwant: %v", len(cmd.args), len(want), cmd.args, want)
	}
	for i, arg := range cmd.args {
		if arg != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, want[i])
		}
	}
}

// TestRegistry_Run_NativePreprocessorSkipped verifies that commands
// implementing NativePreprocessor receive original unsplit args.
func TestRegistry_Run_NativePreprocessorSkipped(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := &nativePreprocessorMock{
		baseWrapper: baseWrapper{name: "nativecmd"},
	}
	r.Register(cmd)

	ctx := t.Context()
	err := r.Run(ctx, "nativecmd", []string{"nativecmd", "-abc", "file.txt"})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if !cmd.called {
		t.Fatal("command was not called")
	}

	// NativePreprocessor commands should receive original args — no splitting.
	want := []string{"nativecmd", "-abc", "file.txt"}
	if len(cmd.args) != len(want) {
		t.Fatalf("args length = %d, want %d\ngot:  %v\nwant: %v", len(cmd.args), len(want), cmd.args, want)
	}
	for i, arg := range cmd.args {
		if arg != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, arg, want[i])
		}
	}
}

// TestRegistry_Run_CommandNameOnlyArgs verifies that args with only the
// command name (no flags) are handled without error or mutation.
func TestRegistry_Run_CommandNameOnlyArgs(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := newMockCommand("testcmd")
	r.Register(cmd)

	ctx := t.Context()
	err := r.Run(ctx, "testcmd", []string{"testcmd"})
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Only command name — preprocessing is skipped (len(args) <= 1)
	want := []string{"testcmd"}
	if len(cmd.args) != len(want) {
		t.Fatalf("args length = %d, want %d", len(cmd.args), len(want))
	}
	if cmd.args[0] != "testcmd" {
		t.Errorf("args[0] = %q, want %q", cmd.args[0], "testcmd")
	}
}

// TestRegistry_Run_NativePreprocessorInterface verifies the marker interface
// type assertion works correctly for both custom and upstream-style commands.
func TestRegistry_Run_NativePreprocessorInterface(t *testing.T) {
	t.Parallel()

	custom := newMockCommand("custom")
	native := &nativePreprocessorMock{
		baseWrapper: baseWrapper{name: "native"},
	}

	// Custom commands should NOT implement NativePreprocessor
	if _, ok := Command(custom).(NativePreprocessor); ok {
		t.Error("mockCommand should not implement NativePreprocessor")
	}

	// Native wrappers should implement NativePreprocessor via baseWrapper
	if _, ok := Command(native).(NativePreprocessor); !ok {
		t.Error("nativePreprocessorMock should implement NativePreprocessor via baseWrapper")
	}
}

// TestRegistry_CommandError_NotSwallowed verifies that command errors are never
// swallowed or transformed into success. This is a defensive test ensuring the
// "no silent fallback" invariant holds even for edge cases.
//
// [US3] T045: Additional verification that errors are not swallowed
func TestRegistry_CommandError_NotSwallowed(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	// Counters to verify command was actually executed
	callCount := 0

	cmd := &mockCommand{
		name: "failcmd",
		runFn: func(_ context.Context, _ []string) error {
			callCount++
			return errors.New("[uroot] failcmd: intentional failure")
		},
	}
	r.Register(cmd)

	ctx := t.Context()

	// Run multiple times to ensure consistent behavior
	for i := range 3 {
		err := r.Run(ctx, "failcmd", []string{"failcmd"})

		if err == nil {
			t.Errorf("iteration %d: Run should return error, got nil", i)
		}
		if callCount != i+1 {
			t.Errorf("iteration %d: command should be called exactly once per Run, call count: %d", i, callCount)
		}
	}
}
