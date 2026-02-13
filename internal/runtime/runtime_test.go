// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"invowk-cli/pkg/invowkfile"
)

// mockRuntime is a test runtime that records calls and returns configured results.
type mockRuntime struct {
	name          string
	available     bool
	validateErr   error
	executeResult *Result
}

func (m *mockRuntime) Name() string {
	return m.name
}

func (m *mockRuntime) Available() bool {
	return m.available
}

func (m *mockRuntime) Validate(_ *ExecutionContext) error {
	return m.validateErr
}

func (m *mockRuntime) Execute(_ *ExecutionContext) *Result {
	if m.executeResult != nil {
		return m.executeResult
	}
	return &Result{ExitCode: 0}
}

// TestNewExecutionContext verifies that NewExecutionContext initializes defaults correctly.
func TestNewExecutionContext(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}
	cmd := testCommandWithScript("test", "echo hello", invowkfile.RuntimeNative)

	ctx := NewExecutionContext(cmd, inv)

	// Check that defaults are set
	if ctx.Command != cmd {
		t.Error("NewExecutionContext() Command not set")
	}
	if ctx.Invowkfile != inv {
		t.Error("NewExecutionContext() Invowkfile not set")
	}
	if ctx.Context == nil {
		t.Error("NewExecutionContext() Context should be background context")
	}
	if ctx.IO.Stdout != os.Stdout {
		t.Error("NewExecutionContext() IO.Stdout should default to os.Stdout")
	}
	if ctx.IO.Stderr != os.Stderr {
		t.Error("NewExecutionContext() IO.Stderr should default to os.Stderr")
	}
	if ctx.IO.Stdin != os.Stdin {
		t.Error("NewExecutionContext() IO.Stdin should default to os.Stdin")
	}
	if ctx.Env.ExtraEnv == nil {
		t.Error("NewExecutionContext() Env.ExtraEnv should be initialized")
	}
	if ctx.SelectedRuntime != invowkfile.RuntimeNative {
		t.Errorf("NewExecutionContext() SelectedRuntime = %q, want %q", ctx.SelectedRuntime, invowkfile.RuntimeNative)
	}
	if ctx.SelectedImpl == nil {
		t.Error("NewExecutionContext() SelectedImpl should be set")
	}
	if ctx.ExecutionID == "" {
		t.Error("NewExecutionContext() ExecutionID should be generated")
	}
}

// TestNewExecutionContext_VirtualRuntime tests context creation for virtual runtime.
func TestNewExecutionContext_VirtualRuntime(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}
	cmd := testCommandWithScript("test", "echo hello", invowkfile.RuntimeVirtual)

	ctx := NewExecutionContext(cmd, inv)

	if ctx.SelectedRuntime != invowkfile.RuntimeVirtual {
		t.Errorf("NewExecutionContext() SelectedRuntime = %q, want %q", ctx.SelectedRuntime, invowkfile.RuntimeVirtual)
	}
}

// TestResult_Success tests the Success() method for various combinations.
func TestResult_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result Result
		want   bool
	}{
		{
			name:   "exit code 0, no error",
			result: Result{ExitCode: 0, Error: nil},
			want:   true,
		},
		{
			name:   "exit code 0 with error",
			result: Result{ExitCode: 0, Error: errors.New("some error")},
			want:   false,
		},
		{
			name:   "non-zero exit code, no error",
			result: Result{ExitCode: 1, Error: nil},
			want:   false,
		},
		{
			name:   "non-zero exit code with error",
			result: Result{ExitCode: 127, Error: errors.New("command not found")},
			want:   false,
		},
		{
			name:   "negative exit code",
			result: Result{ExitCode: -1, Error: nil},
			want:   false,
		},
		{
			name:   "exit code 0 with output",
			result: Result{ExitCode: 0, Error: nil, Output: "hello", ErrOutput: ""},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.result.Success(); got != tt.want {
				t.Errorf("Result.Success() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNewRegistry verifies that NewRegistry creates an empty registry.
func TestNewRegistry(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	if reg == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if reg.runtimes == nil {
		t.Error("NewRegistry() runtimes map should be initialized")
	}
	if len(reg.runtimes) != 0 {
		t.Error("NewRegistry() should create empty registry")
	}
}

// TestRegistry_Register tests runtime registration.
func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	mock := &mockRuntime{name: "test", available: true}

	reg.Register(RuntimeTypeNative, mock)

	if len(reg.runtimes) != 1 {
		t.Errorf("Register() registry size = %d, want 1", len(reg.runtimes))
	}

	rt, ok := reg.runtimes[RuntimeTypeNative]
	if !ok {
		t.Error("Register() runtime not found in registry")
	}
	if rt != mock {
		t.Error("Register() registered runtime doesn't match")
	}
}

// TestRegistry_Register_Overwrite tests that registering the same type overwrites.
func TestRegistry_Register_Overwrite(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	mock1 := &mockRuntime{name: "first", available: true}
	mock2 := &mockRuntime{name: "second", available: true}

	reg.Register(RuntimeTypeNative, mock1)
	reg.Register(RuntimeTypeNative, mock2)

	if len(reg.runtimes) != 1 {
		t.Errorf("Register() registry size = %d, want 1", len(reg.runtimes))
	}

	rt := reg.runtimes[RuntimeTypeNative]
	if rt.Name() != "second" {
		t.Errorf("Register() should overwrite, got name = %q", rt.Name())
	}
}

// TestRegistry_Get tests retrieval of registered runtimes.
func TestRegistry_Get(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	mock := &mockRuntime{name: "native", available: true}
	reg.Register(RuntimeTypeNative, mock)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		rt, err := reg.Get(RuntimeTypeNative)
		if err != nil {
			t.Fatalf("Get() unexpected error: %v", err)
		}
		if rt != mock {
			t.Error("Get() returned wrong runtime")
		}
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		_, err := reg.Get(RuntimeTypeVirtual)
		if err == nil {
			t.Error("Get() expected error for unregistered runtime")
		}
	})
}

// TestRegistry_GetForContext tests context-based runtime retrieval.
func TestRegistry_GetForContext(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	reg := NewRegistry()
	mockNative := &mockRuntime{name: "native", available: true}
	mockVirtual := &mockRuntime{name: "virtual", available: true}
	reg.Register(RuntimeTypeNative, mockNative)
	reg.Register(RuntimeTypeVirtual, mockVirtual)

	tests := []struct {
		name     string
		runtime  invowkfile.RuntimeMode
		wantName string
		wantErr  bool
	}{
		{
			name:     "native runtime",
			runtime:  invowkfile.RuntimeNative,
			wantName: "native",
		},
		{
			name:     "virtual runtime",
			runtime:  invowkfile.RuntimeVirtual,
			wantName: "virtual",
		},
		{
			name:    "unregistered runtime",
			runtime: invowkfile.RuntimeContainer,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := testCommandWithScript("test", "echo test", tt.runtime)
			ctx := NewExecutionContext(cmd, inv)

			rt, err := reg.GetForContext(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("GetForContext() expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetForContext() unexpected error: %v", err)
			}
			if rt.Name() != tt.wantName {
				t.Errorf("GetForContext() runtime name = %q, want %q", rt.Name(), tt.wantName)
			}
		})
	}
}

// TestRegistry_Available tests listing of available runtimes.
func TestRegistry_Available(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	reg.Register(RuntimeTypeNative, &mockRuntime{name: "native", available: true})
	reg.Register(RuntimeTypeVirtual, &mockRuntime{name: "virtual", available: true})
	reg.Register(RuntimeTypeContainer, &mockRuntime{name: "container", available: false})

	available := reg.Available()

	// Should only include available runtimes
	if len(available) != 2 {
		t.Errorf("Available() returned %d runtimes, want 2", len(available))
	}

	// Sort for deterministic comparison
	slices.Sort(available)

	if available[0] != RuntimeTypeNative {
		t.Errorf("Available()[0] = %q, want %q", available[0], RuntimeTypeNative)
	}
	if available[1] != RuntimeTypeVirtual {
		t.Errorf("Available()[1] = %q, want %q", available[1], RuntimeTypeVirtual)
	}
}

// TestRegistry_Available_Empty tests Available() with no registered runtimes.
func TestRegistry_Available_Empty(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	available := reg.Available()

	if len(available) != 0 {
		t.Errorf("Available() returned %d runtimes, want 0", len(available))
	}
}

// TestRegistry_Execute tests the Execute method.
func TestRegistry_Execute(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	tests := []struct {
		name         string
		setupReg     func(*Registry)
		runtime      invowkfile.RuntimeMode
		wantExitCode int
		wantErr      bool
	}{
		{
			name: "successful execution",
			setupReg: func(reg *Registry) {
				reg.Register(RuntimeTypeNative, &mockRuntime{
					name:          "native",
					available:     true,
					executeResult: &Result{ExitCode: 0},
				})
			},
			runtime:      invowkfile.RuntimeNative,
			wantExitCode: 0,
		},
		{
			name: "unregistered runtime",
			setupReg: func(_ *Registry) {
				// Don't register anything
			},
			runtime:      invowkfile.RuntimeNative,
			wantExitCode: 1,
			wantErr:      true,
		},
		{
			name: "unavailable runtime",
			setupReg: func(reg *Registry) {
				reg.Register(RuntimeTypeNative, &mockRuntime{
					name:      "native",
					available: false,
				})
			},
			runtime:      invowkfile.RuntimeNative,
			wantExitCode: 1,
			wantErr:      true,
		},
		{
			name: "validation error",
			setupReg: func(reg *Registry) {
				reg.Register(RuntimeTypeNative, &mockRuntime{
					name:        "native",
					available:   true,
					validateErr: errors.New("validation failed"),
				})
			},
			runtime:      invowkfile.RuntimeNative,
			wantExitCode: 1,
			wantErr:      true,
		},
		{
			name: "non-zero exit code from runtime",
			setupReg: func(reg *Registry) {
				reg.Register(RuntimeTypeNative, &mockRuntime{
					name:          "native",
					available:     true,
					executeResult: &Result{ExitCode: 42},
				})
			},
			runtime:      invowkfile.RuntimeNative,
			wantExitCode: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := NewRegistry()
			tt.setupReg(reg)

			cmd := testCommandWithScript("test", "echo test", tt.runtime)
			ctx := NewExecutionContext(cmd, inv)
			ctx.IO.Stdout = &bytes.Buffer{}
			ctx.IO.Stderr = &bytes.Buffer{}

			result := reg.Execute(ctx)

			if result.ExitCode != tt.wantExitCode {
				t.Errorf("Execute() exit code = %d, want %d", result.ExitCode, tt.wantExitCode)
			}
			if tt.wantErr && result.Error == nil {
				t.Error("Execute() expected error, got nil")
			}
			if !tt.wantErr && result.Error != nil {
				t.Errorf("Execute() unexpected error: %v", result.Error)
			}
		})
	}
}

// TestEnvToSlice tests the map to slice conversion.
func TestEnvToSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "empty map",
			env:  map[string]string{},
			want: []string{},
		},
		{
			name: "single entry",
			env:  map[string]string{"FOO": "bar"},
			want: []string{"FOO=bar"},
		},
		{
			name: "multiple entries",
			env: map[string]string{
				"FOO":  "bar",
				"PATH": "/usr/bin",
			},
			want: []string{"FOO=bar", "PATH=/usr/bin"},
		},
		{
			name: "empty value",
			env:  map[string]string{"EMPTY": ""},
			want: []string{"EMPTY="},
		},
		{
			name: "value with equals",
			env:  map[string]string{"URL": "https://example.com?foo=bar"},
			want: []string{"URL=https://example.com?foo=bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := EnvToSlice(tt.env)

			if len(got) != len(tt.want) {
				t.Errorf("EnvToSlice() length = %d, want %d", len(got), len(tt.want))
				return
			}

			// Sort both for comparison since map iteration order is non-deterministic
			slices.Sort(got)
			slices.Sort(tt.want)

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("EnvToSlice()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestFindEnvSeparator tests the helper function for finding '=' in env strings.
func TestFindEnvSeparator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "simple key=value",
			input: "FOO=bar",
			want:  3,
		},
		{
			name:  "no separator",
			input: "FOOBAR",
			want:  -1,
		},
		{
			name:  "empty string",
			input: "",
			want:  -1,
		},
		{
			name:  "separator at start",
			input: "=value",
			want:  0,
		},
		{
			name:  "multiple separators",
			input: "FOO=bar=baz",
			want:  3,
		},
		{
			name:  "empty value",
			input: "FOO=",
			want:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := findEnvSeparator(tt.input)
			if got != tt.want {
				t.Errorf("findEnvSeparator(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestNewExecutionID tests that execution IDs are generated.
func TestNewExecutionID(t *testing.T) {
	t.Parallel()

	id1 := newExecutionID()
	id2 := newExecutionID()

	if id1 == "" {
		t.Error("newExecutionID() returned empty string")
	}
	// IDs should be unique (assuming tests don't run at exact same nanosecond)
	if id1 == id2 {
		t.Error("newExecutionID() should generate unique IDs")
	}
}

// TestExecutionContext_CustomOverrides tests setting custom overrides on context.
func TestExecutionContext_CustomOverrides(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}
	cmd := testCommandWithScript("test", "echo test", invowkfile.RuntimeNative)

	ctx := NewExecutionContext(cmd, inv)

	// Set custom overrides
	ctx.Context = context.Background()
	ctx.IO.Stdout = &bytes.Buffer{}
	ctx.IO.Stderr = &bytes.Buffer{}
	ctx.Env.ExtraEnv["CUSTOM"] = "value"
	ctx.WorkDir = "/custom/dir"
	ctx.Verbose = true
	ctx.PositionalArgs = []string{"arg1", "arg2"}
	ctx.Env.RuntimeEnvFiles = []string{".env"}
	ctx.Env.RuntimeEnvVars = map[string]string{"VAR": "val"}
	ctx.TUI.ServerURL = "http://localhost:8080"
	ctx.TUI.ServerToken = "token123"

	// Verify overrides are set
	if ctx.WorkDir != "/custom/dir" {
		t.Errorf("WorkDir = %q, want %q", ctx.WorkDir, "/custom/dir")
	}
	if !ctx.Verbose {
		t.Error("Verbose should be true")
	}
	if len(ctx.PositionalArgs) != 2 {
		t.Errorf("PositionalArgs length = %d, want 2", len(ctx.PositionalArgs))
	}
	if ctx.Env.ExtraEnv["CUSTOM"] != "value" {
		t.Error("Env.ExtraEnv not set correctly")
	}
	if ctx.TUI.ServerURL != "http://localhost:8080" {
		t.Error("TUI.ServerURL not set correctly")
	}
	if ctx.TUI.ServerToken != "token123" {
		t.Error("TUI.ServerToken not set correctly")
	}
}

// TestErrRuntimeNotAvailable_Sentinel verifies the sentinel error value.
func TestErrRuntimeNotAvailable_Sentinel(t *testing.T) {
	t.Parallel()

	if ErrRuntimeNotAvailable == nil {
		t.Fatal("ErrRuntimeNotAvailable should not be nil")
	}
	if ErrRuntimeNotAvailable.Error() != "runtime not available" {
		t.Errorf("ErrRuntimeNotAvailable.Error() = %q, want %q", ErrRuntimeNotAvailable.Error(), "runtime not available")
	}
}

// TestRegistry_Execute_UnavailableRuntimeWraps verifies that executing on an unavailable
// runtime produces an error wrapping ErrRuntimeNotAvailable.
func TestRegistry_Execute_UnavailableRuntimeWraps(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	reg := NewRegistry()
	reg.Register(RuntimeTypeNative, &mockRuntime{
		name:      "native",
		available: false,
	})

	cmd := testCommandWithScript("test", "echo test", invowkfile.RuntimeNative)
	ctx := NewExecutionContext(cmd, inv)
	ctx.IO.Stdout = &bytes.Buffer{}
	ctx.IO.Stderr = &bytes.Buffer{}

	result := reg.Execute(ctx)

	if result.Error == nil {
		t.Fatal("Execute() expected error for unavailable runtime")
	}
	if !errors.Is(result.Error, ErrRuntimeNotAvailable) {
		t.Errorf("Execute() error should wrap ErrRuntimeNotAvailable, got: %v", result.Error)
	}
}
