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

	ctx := t.Context()
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

	ctx := t.Context()
	err := r.Run(ctx, "nonexistent", []string{"nonexistent"})

	if err == nil {
		t.Fatal("Run should return error for unregistered command")
	}
	if !errors.Is(err, ErrCommandNotFound) {
		t.Errorf("error should wrap ErrCommandNotFound, got: %v", err)
	}
	if !strings.Contains(err.Error(), "[uroot]") {
		t.Errorf("error should contain [uroot] prefix: %v", err)
	}
}

func TestRegistry_Run_CommandError(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	expectedErr := errors.New("[uroot] test: something went wrong")
	cmd := &mockCommand{
		name: "test",
		runFn: func(_ context.Context, _ []string) error {
			return expectedErr
		},
	}
	r.Register(cmd)

	ctx := t.Context()
	err := r.Run(ctx, "test", []string{"test"})

	if !errors.Is(err, expectedErr) {
		t.Errorf("Run should propagate command error, got: %v", err)
	}
}

func TestRegistry_Run_TarPathValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "archive and create operands",
			args: []string{"tar", "-cf", "archive.tar", "src"},
			want: []string{"tar", "-cf", "/checked/archive.tar", "/checked/src"},
		},
		{
			name: "inline archive flag value",
			args: []string{"tar", "-cfarchive.tar", "src"},
			want: []string{"tar", "-cf/checked/archive.tar", "/checked/src"},
		},
		{
			name: "list archive does not rewrite member filters",
			args: []string{"tar", "-tf", "archive.tar", "member.txt"},
			want: []string{"tar", "-tf", "/checked/archive.tar", "member.txt"},
		},
		{
			name: "long file flag",
			args: []string{"tar", "--file", "archive.tar"},
			want: []string{"tar", "--file", "/checked/archive.tar"},
		},
		{
			name: "inline long file flag",
			args: []string{"tar", "--file=archive.tar"},
			want: []string{"tar", "--file=/checked/archive.tar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := NewRegistry()
			cmd := &nativePreprocessorMock{baseWrapper: baseWrapper{name: "tar"}}
			r.Register(cmd)
			ctx := WithHandlerContext(t.Context(), &HandlerContext{
				Dir: "/work",
				ValidatePath: func(_, path string) (string, error) {
					return "/checked/" + path, nil
				},
			})

			if err := r.Run(ctx, "tar", tt.args); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if strings.Join(cmd.args, "\x00") != strings.Join(tt.want, "\x00") {
				t.Fatalf("tar args = %v, want %v", cmd.args, tt.want)
			}
		})
	}
}

func TestRegistry_Run_TarPathValidationRejectsArchive(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := &nativePreprocessorMock{baseWrapper: baseWrapper{name: "tar"}}
	r.Register(cmd)
	deniedErr := errors.New("denied")
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Dir: "/work",
		ValidatePath: func(_, path string) (string, error) {
			if path == "archive.tar" {
				return "", deniedErr
			}
			return path, nil
		},
	})

	err := r.Run(ctx, "tar", []string{"tar", "-tf", "archive.tar"})
	if err == nil {
		t.Fatal("Run() error = nil, want tar archive validation failure")
	}
	if cmd.called {
		t.Fatal("tar command was called after validation failure")
	}
	if !errors.Is(err, deniedErr) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, deniedErr)
	}
	if !strings.Contains(err.Error(), "[uroot] tar: denied") {
		t.Fatalf("Run() error = %v, want wrapped tar validation error", err)
	}
}

func TestRegistry_Run_ShasumSkipsAlgorithmFlagValues(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cmd := &nativePreprocessorMock{baseWrapper: baseWrapper{name: "shasum"}}
	r.Register(cmd)
	ctx := WithHandlerContext(t.Context(), &HandlerContext{
		Dir: "/work",
		ValidatePath: func(_, path string) (string, error) {
			return "/checked/" + path, nil
		},
	})

	err := r.Run(ctx, "shasum", []string{"shasum", "-a", "256", "hello.txt"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{"shasum", "-a", "256", "/checked/hello.txt"}
	if strings.Join(cmd.args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("shasum args = %v, want %v", cmd.args, want)
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

// TestBuildDefaultRegistry_ReturnsFreshInstances verifies that each call to
// BuildDefaultRegistry returns an independent registry. Mutating one must not
// affect the other.
func TestBuildDefaultRegistry_ReturnsFreshInstances(t *testing.T) {
	t.Parallel()

	r1 := BuildDefaultRegistry()
	r2 := BuildDefaultRegistry()

	// Both should have the same set of commands.
	names1 := r1.Names()
	names2 := r2.Names()
	if len(names1) != len(names2) {
		t.Fatalf("registries have different command counts: %d vs %d", len(names1), len(names2))
	}

	// Mutate r1 by registering an extra command — r2 must be unaffected.
	r1.Register(newMockCommand("__test_extra__"))

	if _, ok := r1.Lookup("__test_extra__"); !ok {
		t.Fatal("r1 should contain the extra command after mutation")
	}
	if _, ok := r2.Lookup("__test_extra__"); ok {
		t.Fatal("r2 should NOT contain the extra command — registries must be independent")
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

	ctx := WithHandlerContext(t.Context(), hc)
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
