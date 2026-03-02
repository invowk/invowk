// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestRegistryNewExecutionID tests that Registry.NewExecutionID generates unique, valid IDs.
func TestRegistryNewExecutionID(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	id1 := reg.NewExecutionID()
	id2 := reg.NewExecutionID()

	if id1 == "" {
		t.Error("Registry.NewExecutionID() returned empty string")
	}
	// IDs should be unique due to the monotonic counter.
	if id1 == id2 {
		t.Error("Registry.NewExecutionID() should generate unique IDs")
	}
	// Generated IDs must pass Validate.
	if err := id1.Validate(); err != nil {
		t.Errorf("Registry.NewExecutionID() generated invalid ID %q: %v", id1, err)
	}
	if err := id2.Validate(); err != nil {
		t.Errorf("Registry.NewExecutionID() generated invalid ID %q: %v", id2, err)
	}
}

// TestExecutionID_Validate tests the ExecutionID validation method.
func TestExecutionID_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      ExecutionID
		want    bool
		wantErr bool
	}{
		{"valid", ExecutionID("1234567890-1"), true, false},
		{"valid_large", ExecutionID("9999999999999-42"), true, false},
		{"empty", ExecutionID(""), false, true},
		{"no_counter", ExecutionID("1234567890"), false, true},
		{"letters", ExecutionID("abc-1"), false, true},
		{"wrong_separator", ExecutionID("123_456"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.id.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ExecutionID(%q).Validate() valid = %v, want %v", tt.id, err == nil, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ExecutionID(%q).Validate() returned nil, want error", tt.id)
				}
				if !errors.Is(err, ErrInvalidExecutionID) {
					t.Errorf("error should wrap ErrInvalidExecutionID, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("ExecutionID(%q).Validate() returned unexpected error: %v", tt.id, err)
			}
		})
	}
}

func TestRuntimeType_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		runtimeType RuntimeType
		want        bool
		wantErr     bool
	}{
		{RuntimeTypeNative, true, false},
		{RuntimeTypeVirtual, true, false},
		{RuntimeTypeContainer, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"NATIVE", false, true},
	}

	for _, tt := range tests {
		name := string(tt.runtimeType)
		if name == "" {
			name = "empty"
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tt.runtimeType.Validate()
			if (err == nil) != tt.want {
				t.Errorf("RuntimeType(%q).Validate() valid = %v, want %v", tt.runtimeType, err == nil, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RuntimeType(%q).Validate() returned nil, want error", tt.runtimeType)
				}
				if !errors.Is(err, ErrInvalidRuntimeType) {
					t.Errorf("error should wrap ErrInvalidRuntimeType, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("RuntimeType(%q).Validate() returned unexpected error: %v", tt.runtimeType, err)
			}
		})
	}
}

func TestEnvContext_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		envCtx        EnvContext
		want          bool
		wantErr       bool
		wantFieldErrs int
	}{
		{
			name:   "zero value is valid",
			envCtx: EnvContext{},
			want:   true,
		},
		{
			name: "valid with all fields",
			envCtx: EnvContext{
				InheritModeOverride:  invowkfile.EnvInheritAll,
				InheritAllowOverride: []invowkfile.EnvVarName{"HOME", "PATH"},
				InheritDenyOverride:  []invowkfile.EnvVarName{"SECRET"},
				Cwd:                  "/some/dir",
			},
			want: true,
		},
		{
			name: "invalid inherit mode",
			envCtx: EnvContext{
				InheritModeOverride: invowkfile.EnvInheritMode("bogus"),
			},
			want:          false,
			wantErr:       true,
			wantFieldErrs: 1,
		},
		{
			name: "invalid env var name in allow override",
			envCtx: EnvContext{
				InheritAllowOverride: []invowkfile.EnvVarName{"VALID", "123-invalid"},
			},
			want:          false,
			wantErr:       true,
			wantFieldErrs: 1,
		},
		{
			name: "invalid env var name in deny override",
			envCtx: EnvContext{
				InheritDenyOverride: []invowkfile.EnvVarName{"has space"},
			},
			want:          false,
			wantErr:       true,
			wantFieldErrs: 1,
		},
		{
			name: "invalid cwd (whitespace-only)",
			envCtx: EnvContext{
				Cwd: invowkfile.WorkDir("   "),
			},
			want:          false,
			wantErr:       true,
			wantFieldErrs: 1,
		},
		{
			name: "multiple errors",
			envCtx: EnvContext{
				InheritModeOverride:  invowkfile.EnvInheritMode("bad"),
				InheritAllowOverride: []invowkfile.EnvVarName{"123bad"},
				InheritDenyOverride:  []invowkfile.EnvVarName{"also bad"},
			},
			want:          false,
			wantErr:       true,
			wantFieldErrs: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.envCtx.Validate()
			if (err == nil) != tt.want {
				t.Errorf("EnvContext.Validate() valid = %v, want %v", err == nil, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("EnvContext.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidEnvContext) {
					t.Errorf("error should wrap ErrInvalidEnvContext, got: %v", err)
				}
				var envErr *InvalidEnvContextError
				if !errors.As(err, &envErr) {
					t.Fatalf("error should be *InvalidEnvContextError, got: %T", err)
				}
				if len(envErr.FieldErrors) != tt.wantFieldErrs {
					t.Errorf("InvalidEnvContextError.FieldErrors = %d, want %d", len(envErr.FieldErrors), tt.wantFieldErrs)
				}
			} else if err != nil {
				t.Errorf("EnvContext.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestExecutionID_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   ExecutionID
		want string
	}{
		{"valid ID", ExecutionID("1234567890-1"), "1234567890-1"},
		{"empty", ExecutionID(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.id.String(); got != tt.want {
				t.Errorf("ExecutionID(%q).String() = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestRuntimeType_String(t *testing.T) {
	t.Parallel()

	if got := RuntimeTypeNative.String(); got != "native" {
		t.Errorf("RuntimeTypeNative.String() = %q, want %q", got, "native")
	}
	if got := RuntimeType("").String(); got != "" {
		t.Errorf("RuntimeType(\"\").String() = %q, want %q", got, "")
	}
}
