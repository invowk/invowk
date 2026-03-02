// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

// --- ImplementationMatch ---

func TestImplementationMatch_Validate_ZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value ImplementationMatch has empty Platform and Runtime — should fail.
	m := ImplementationMatch{}
	if err := m.Validate(); err == nil {
		t.Fatal("ImplementationMatch{}.Validate() should fail (empty Platform and Runtime)")
	}
}

func TestImplementationMatch_Validate_Valid(t *testing.T) {
	t.Parallel()
	m := ImplementationMatch{
		Platform: PlatformLinux,
		Runtime:  RuntimeNative,
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("valid ImplementationMatch.Validate() returned error: %v", err)
	}
}

func TestImplementationMatch_Validate_InvalidPlatform(t *testing.T) {
	t.Parallel()
	m := ImplementationMatch{
		Platform: "beos",
		Runtime:  RuntimeNative,
	}
	err := m.Validate()
	if err == nil {
		t.Fatal("ImplementationMatch with invalid platform should fail")
	}
	if !errors.Is(err, ErrInvalidImplementationMatch) {
		t.Errorf("error should wrap ErrInvalidImplementationMatch, got: %v", err)
	}
}

func TestImplementationMatch_Validate_InvalidRuntime(t *testing.T) {
	t.Parallel()
	m := ImplementationMatch{
		Platform: PlatformLinux,
		Runtime:  "bogus",
	}
	err := m.Validate()
	if err == nil {
		t.Fatal("ImplementationMatch with invalid runtime should fail")
	}
	if !errors.Is(err, ErrInvalidImplementationMatch) {
		t.Errorf("error should wrap ErrInvalidImplementationMatch, got: %v", err)
	}
}

func TestImplementationMatch_Validate_BothInvalid(t *testing.T) {
	t.Parallel()
	m := ImplementationMatch{
		Platform: "beos",
		Runtime:  "bogus",
	}
	err := m.Validate()
	if err == nil {
		t.Fatal("ImplementationMatch with both invalid fields should fail")
	}
	var imErr *InvalidImplementationMatchError
	if !errors.As(err, &imErr) {
		t.Fatalf("error should be *InvalidImplementationMatchError, got: %T", err)
	}
	if len(imErr.FieldErrors) != 2 {
		t.Errorf("expected 2 field errors, got %d", len(imErr.FieldErrors))
	}
}

func TestInvalidImplementationMatchError_ErrorMessage(t *testing.T) {
	t.Parallel()
	e := &InvalidImplementationMatchError{FieldErrors: []error{errors.New("x")}}
	got := e.Error()
	want := "invalid implementation match: 1 field error(s)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestInvalidImplementationMatchError_Unwrap(t *testing.T) {
	t.Parallel()
	e := &InvalidImplementationMatchError{}
	if !errors.Is(e, ErrInvalidImplementationMatch) {
		t.Error("Unwrap() should return ErrInvalidImplementationMatch")
	}
}

// --- Implementation ---

func TestImplementation_Validate_ZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value Implementation has no runtimes or platforms — should pass
	// since Script is zero-valid and all slices are empty.
	impl := Implementation{}
	if err := impl.Validate(); err != nil {
		t.Fatalf("Implementation{}.Validate() should pass (all zero-valid), got: %v", err)
	}
}

func TestImplementation_Validate_Valid(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		Script:    "echo hello",
		Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
		Platforms: AllPlatformConfigs(),
		Timeout:   "30s",
	}
	if err := impl.Validate(); err != nil {
		t.Fatalf("valid Implementation.Validate() returned error: %v", err)
	}
}

func TestImplementation_Validate_ValidWithEnvAndDeps(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		Script:    "echo hello",
		Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
		Platforms: AllPlatformConfigs(),
		Env: &EnvConfig{
			Vars: map[EnvVarName]string{"MY_VAR": "value"},
		},
		WorkDir: "src",
		DependsOn: &DependsOn{
			Tools: []ToolDependency{{Alternatives: []BinaryName{"gcc"}}},
		},
	}
	if err := impl.Validate(); err != nil {
		t.Fatalf("valid Implementation with Env/DependsOn.Validate() returned error: %v", err)
	}
}

func TestImplementation_Validate_InvalidScript(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		Script: "   ", // whitespace-only
	}
	err := impl.Validate()
	if err == nil {
		t.Fatal("Implementation with whitespace-only script should fail")
	}
	if !errors.Is(err, ErrInvalidImplementation) {
		t.Errorf("error should wrap ErrInvalidImplementation, got: %v", err)
	}
}

func TestImplementation_Validate_InvalidRuntime(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		Runtimes: []RuntimeConfig{{Name: "bogus"}},
	}
	err := impl.Validate()
	if err == nil {
		t.Fatal("Implementation with invalid runtime should fail")
	}
	if !errors.Is(err, ErrInvalidImplementation) {
		t.Errorf("error should wrap ErrInvalidImplementation, got: %v", err)
	}
}

func TestImplementation_Validate_InvalidPlatform(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		Platforms: []PlatformConfig{{Name: "beos"}},
	}
	err := impl.Validate()
	if err == nil {
		t.Fatal("Implementation with invalid platform should fail")
	}
	if !errors.Is(err, ErrInvalidImplementation) {
		t.Errorf("error should wrap ErrInvalidImplementation, got: %v", err)
	}
}

func TestImplementation_Validate_InvalidEnv(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		Env: &EnvConfig{
			Vars: map[EnvVarName]string{"123-BAD": "value"}, // invalid env var name
		},
	}
	err := impl.Validate()
	if err == nil {
		t.Fatal("Implementation with invalid env should fail")
	}
	if !errors.Is(err, ErrInvalidImplementation) {
		t.Errorf("error should wrap ErrInvalidImplementation, got: %v", err)
	}
}

func TestImplementation_Validate_InvalidWorkDir(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		WorkDir: "   ", // whitespace-only
	}
	err := impl.Validate()
	if err == nil {
		t.Fatal("Implementation with whitespace-only WorkDir should fail")
	}
	if !errors.Is(err, ErrInvalidImplementation) {
		t.Errorf("error should wrap ErrInvalidImplementation, got: %v", err)
	}
}

func TestImplementation_Validate_InvalidDependsOn(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		DependsOn: &DependsOn{
			Tools: []ToolDependency{{Alternatives: []BinaryName{""}}},
		},
	}
	err := impl.Validate()
	if err == nil {
		t.Fatal("Implementation with invalid DependsOn should fail")
	}
	if !errors.Is(err, ErrInvalidImplementation) {
		t.Errorf("error should wrap ErrInvalidImplementation, got: %v", err)
	}
}

func TestImplementation_Validate_InvalidTimeout(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		Timeout: "not-a-duration",
	}
	err := impl.Validate()
	if err == nil {
		t.Fatal("Implementation with invalid timeout should fail")
	}
	if !errors.Is(err, ErrInvalidImplementation) {
		t.Errorf("error should wrap ErrInvalidImplementation, got: %v", err)
	}
}

func TestImplementation_Validate_MultipleErrors(t *testing.T) {
	t.Parallel()
	impl := Implementation{
		Script:    "   ",                            // invalid
		Runtimes:  []RuntimeConfig{{Name: "bogus"}}, // invalid
		Platforms: []PlatformConfig{{Name: "beos"}}, // invalid
		WorkDir:   "   ",                            // invalid
		Timeout:   "not-a-duration",                 // invalid
	}
	err := impl.Validate()
	if err == nil {
		t.Fatal("Implementation with multiple invalid fields should fail")
	}
	var implErr *InvalidImplementationError
	if !errors.As(err, &implErr) {
		t.Fatalf("error should be *InvalidImplementationError, got: %T", err)
	}
	if len(implErr.FieldErrors) < 4 {
		t.Errorf("expected at least 4 field errors, got %d", len(implErr.FieldErrors))
	}
}

func TestInvalidImplementationError_ErrorMessage(t *testing.T) {
	t.Parallel()
	e := &InvalidImplementationError{FieldErrors: []error{errors.New("a"), errors.New("b")}}
	got := e.Error()
	want := "invalid implementation: 2 field error(s)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestInvalidImplementationError_Unwrap(t *testing.T) {
	t.Parallel()
	e := &InvalidImplementationError{}
	if !errors.Is(e, ErrInvalidImplementation) {
		t.Error("Unwrap() should return ErrInvalidImplementation")
	}
}
