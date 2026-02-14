// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

type (
	// mockCommand is a test implementation of Command (custom implementation).
	mockCommand struct {
		name   string
		flags  []FlagInfo
		runFn  func(ctx context.Context, args []string) error
		called bool
		args   []string
	}

	// nativePreprocessorMock simulates an upstream wrapper that handles POSIX
	// flag preprocessing internally. It embeds baseWrapper to inherit the
	// NativePreprocessor marker interface.
	nativePreprocessorMock struct {
		baseWrapper
		runFn  func(ctx context.Context, args []string) error
		called bool
		args   []string
	}
)

func (m *mockCommand) Name() string { return m.name }

func (m *mockCommand) SupportedFlags() []FlagInfo { return m.flags }

func (m *mockCommand) Run(ctx context.Context, args []string) error {
	m.called = true
	m.args = args
	if m.runFn != nil {
		return m.runFn(ctx, args)
	}
	return nil
}

func newMockCommand(name string) *mockCommand {
	return &mockCommand{name: name}
}

func (m *nativePreprocessorMock) Run(ctx context.Context, args []string) error {
	m.called = true
	m.args = args
	if m.runFn != nil {
		return m.runFn(ctx, args)
	}
	return nil
}

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.commands == nil {
		t.Fatal("NewRegistry did not initialize commands map")
	}
	if len(r.commands) != 0 {
		t.Errorf("NewRegistry should create empty registry, got %d commands", len(r.commands))
	}
}

func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := newMockCommand("test")

	r.Register(cmd)

	if len(r.commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(r.commands))
	}
	if _, ok := r.commands["test"]; !ok {
		t.Error("command 'test' not found in registry")
	}
}

func TestRegistry_Register_Multiple(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.Register(newMockCommand("cat"))
	r.Register(newMockCommand("ls"))
	r.Register(newMockCommand("cp"))

	if len(r.commands) != 3 {
		t.Errorf("expected 3 commands, got %d", len(r.commands))
	}
}

func TestRegistry_Register_PanicOnDuplicate(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.Register(newMockCommand("test"))

	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()

	r.Register(newMockCommand("test"))
}

func TestRegistry_Register_PanicOnEmptyName(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	defer func() {
		if recover() == nil {
			t.Error("expected panic on empty name registration")
		}
	}()

	r.Register(newMockCommand(""))
}

func TestRegistry_Lookup(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := newMockCommand("test")
	r.Register(cmd)

	found, ok := r.Lookup("test")
	if !ok {
		t.Error("Lookup should find registered command")
	}
	if found != cmd {
		t.Error("Lookup returned wrong command")
	}
}

func TestRegistry_Lookup_NotFound(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	_, ok := r.Lookup("nonexistent")
	if ok {
		t.Error("Lookup should return false for unregistered command")
	}
}

func TestRegistry_Names(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.Register(newMockCommand("cat"))
	r.Register(newMockCommand("ls"))
	r.Register(newMockCommand("cp"))

	names := r.Names()

	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	// Names should be sorted
	expected := []string{"cat", "cp", "ls"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Names()[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestRegistry_Names_Empty(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	names := r.Names()

	if len(names) != 0 {
		t.Errorf("expected empty names, got %d", len(names))
	}
}

func TestRegistry_Run(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := newMockCommand("echo")
	r.Register(cmd)

	ctx := context.Background()
	err := r.Run(ctx, "echo", []string{"echo", "hello", "world"})
	if err != nil {
		t.Errorf("Run returned unexpected error: %v", err)
	}
	if !cmd.called {
		t.Error("command was not called")
	}
	if len(cmd.args) != 3 || cmd.args[0] != "echo" || cmd.args[1] != "hello" || cmd.args[2] != "world" {
		t.Errorf("command received wrong args: %v", cmd.args)
	}
}

func TestRegistry_Run_NotFound(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	ctx := context.Background()
	err := r.Run(ctx, "nonexistent", []string{"nonexistent"})

	if err == nil {
		t.Error("Run should return error for unregistered command")
	}
	if !strings.Contains(err.Error(), "[uroot]") {
		t.Errorf("error should contain [uroot] prefix: %v", err)
	}
	if !strings.Contains(err.Error(), "command not found") {
		t.Errorf("error should indicate command not found: %v", err)
	}
}

func TestRegistry_Run_CommandError(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	expectedErr := errors.New("[uroot] test: something went wrong")
	cmd := &mockCommand{
		name: "test",
		runFn: func(ctx context.Context, args []string) error {
			return expectedErr
		},
	}
	r.Register(cmd)

	ctx := context.Background()
	err := r.Run(ctx, "test", []string{"test"})

	if !errors.Is(err, expectedErr) {
		t.Errorf("Run should propagate command error, got: %v", err)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	// Register some commands
	for i := range 10 {
		r.Register(newMockCommand(fmt.Sprintf("cmd%d", i)))
	}

	// Concurrent reads
	done := make(chan bool)
	for range 10 {
		go func() {
			for range 100 {
				r.Lookup("cmd5")
				r.Names()
			}
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}

func TestBuildDefaultRegistry(t *testing.T) {
	t.Parallel()

	r := BuildDefaultRegistry()
	if r == nil {
		t.Fatal("BuildDefaultRegistry returned nil")
	}

	// Verify all 28 commands are registered
	names := r.Names()
	if len(names) != 28 {
		t.Errorf("BuildDefaultRegistry registered %d commands, want 28", len(names))
	}
}

func TestHandlerContext_WithContext(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("input")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	hc := &HandlerContext{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Dir:    "/test/dir",
		LookupEnv: func(name string) (string, bool) {
			if name == "HOME" {
				return "/home/test", true
			}
			return "", false
		},
	}

	ctx := WithHandlerContext(context.Background(), hc)
	retrieved := GetHandlerContext(ctx)

	if retrieved != hc {
		t.Error("GetHandlerContext should return the same HandlerContext")
	}
	if retrieved.Dir != "/test/dir" {
		t.Errorf("Dir = %q, want %q", retrieved.Dir, "/test/dir")
	}

	// Test LookupEnv
	val, ok := retrieved.LookupEnv("HOME")
	if !ok || val != "/home/test" {
		t.Errorf("LookupEnv(HOME) = (%q, %v), want (%q, true)", val, ok, "/home/test")
	}

	val, ok = retrieved.LookupEnv("NONEXISTENT")
	if ok {
		t.Errorf("LookupEnv(NONEXISTENT) should return false, got (%q, %v)", val, ok)
	}
}

func TestHandlerContext_ReadWrite(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("test input")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	hc := &HandlerContext{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}

	// Test reading from stdin
	data, err := io.ReadAll(hc.Stdin)
	if err != nil {
		t.Fatalf("reading stdin: %v", err)
	}
	if string(data) != "test input" {
		t.Errorf("stdin data = %q, want %q", string(data), "test input")
	}

	// Test writing to stdout
	_, err = hc.Stdout.Write([]byte("stdout output"))
	if err != nil {
		t.Fatalf("writing stdout: %v", err)
	}
	if stdout.String() != "stdout output" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "stdout output")
	}

	// Test writing to stderr
	_, err = hc.Stderr.Write([]byte("stderr output"))
	if err != nil {
		t.Fatalf("writing stderr: %v", err)
	}
	if stderr.String() != "stderr output" {
		t.Errorf("stderr = %q, want %q", stderr.String(), "stderr output")
	}
}

func TestFlagInfo(t *testing.T) {
	t.Parallel()

	flags := []FlagInfo{
		{Name: "recursive", ShortName: "r", Description: "Copy recursively", TakesValue: false},
		{Name: "n", ShortName: "", Description: "Number of lines", TakesValue: true},
	}

	if flags[0].Name != "recursive" {
		t.Errorf("flags[0].Name = %q, want %q", flags[0].Name, "recursive")
	}
	if flags[0].ShortName != "r" {
		t.Errorf("flags[0].ShortName = %q, want %q", flags[0].ShortName, "r")
	}
	if flags[0].TakesValue {
		t.Error("flags[0].TakesValue should be false")
	}
	if !flags[1].TakesValue {
		t.Error("flags[1].TakesValue should be true")
	}
}

// =============================================================================
// User Story 3: Gradual Adoption with Fallback Tests
// =============================================================================
// These tests verify the fallback behavior for the virtual runtime's u-root
// integration. They ensure:
// 1. Unregistered commands trigger fallback to system binaries (T044)
// 2. Registered command errors are propagated, NOT silently falling back (T045)

// TestRegistry_Lookup_FallbackBehavior verifies that Lookup returns (nil, false)
// for unregistered commands, which signals the virtual runtime to fall back to
// system binaries. This is the foundation of User Story 3's gradual adoption.
//
// [US3] T044: Fallback behavior - unregistered commands use system
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
				runFn: func(ctx context.Context, args []string) error {
					return expectedErr
				},
			}
			r.Register(cmd)

			// Run the command
			ctx := context.Background()
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

	ctx := context.Background()
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

	ctx := context.Background()
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

	ctx := context.Background()
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

	ctx := context.Background()
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

	ctx := context.Background()
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
		runFn: func(ctx context.Context, args []string) error {
			callCount++
			return fmt.Errorf("[uroot] failcmd: intentional failure")
		},
	}
	r.Register(cmd)

	ctx := context.Background()

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
