// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

// --- PlatformConfig ---

func TestPlatformConfig_Validate_ZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value PlatformConfig has empty Name — should fail.
	pc := PlatformConfig{}
	if err := pc.Validate(); err == nil {
		t.Fatal("PlatformConfig{}.Validate() should fail (empty Name)")
	}
}

func TestPlatformConfig_Validate_Valid(t *testing.T) {
	t.Parallel()
	pc := PlatformConfig{Name: PlatformLinux}
	if err := pc.Validate(); err != nil {
		t.Fatalf("valid PlatformConfig.Validate() returned error: %v", err)
	}
}

func TestPlatformConfig_Validate_AllPlatforms(t *testing.T) {
	t.Parallel()
	for _, p := range AllPlatformNames() {
		pc := PlatformConfig{Name: p}
		if err := pc.Validate(); err != nil {
			t.Errorf("PlatformConfig{Name: %q}.Validate() returned error: %v", p, err)
		}
	}
}

func TestPlatformConfig_Validate_InvalidName(t *testing.T) {
	t.Parallel()
	pc := PlatformConfig{Name: "beos"}
	err := pc.Validate()
	if err == nil {
		t.Fatal("PlatformConfig with invalid name should fail")
	}
	if !errors.Is(err, ErrInvalidPlatformConfig) {
		t.Errorf("error should wrap ErrInvalidPlatformConfig, got: %v", err)
	}
	var pcErr *InvalidPlatformConfigError
	if !errors.As(err, &pcErr) {
		t.Errorf("error should be *InvalidPlatformConfigError, got: %T", err)
	} else if len(pcErr.FieldErrors) == 0 {
		t.Error("InvalidPlatformConfigError.FieldErrors should not be empty")
	}
}

func TestInvalidPlatformConfigError_ErrorMessage(t *testing.T) {
	t.Parallel()
	e := &InvalidPlatformConfigError{FieldErrors: []error{errors.New("x")}}
	got := e.Error()
	want := "invalid platform config: 1 field error(s)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestInvalidPlatformConfigError_Unwrap(t *testing.T) {
	t.Parallel()
	e := &InvalidPlatformConfigError{}
	if !errors.Is(e, ErrInvalidPlatformConfig) {
		t.Error("Unwrap() should return ErrInvalidPlatformConfig")
	}
}

// --- RuntimeConfig ---

func TestRuntimeConfig_Validate_ZeroValue(t *testing.T) {
	t.Parallel()
	// Zero-value RuntimeConfig has empty Name — should fail.
	rc := RuntimeConfig{}
	if err := rc.Validate(); err == nil {
		t.Fatal("RuntimeConfig{}.Validate() should fail (empty Name)")
	}
}

func TestRuntimeConfig_Validate_ValidMinimal(t *testing.T) {
	t.Parallel()
	rc := RuntimeConfig{Name: RuntimeNative}
	if err := rc.Validate(); err != nil {
		t.Fatalf("minimal RuntimeConfig.Validate() returned error: %v", err)
	}
}

func TestRuntimeConfig_Validate_ValidContainer(t *testing.T) {
	t.Parallel()
	rc := RuntimeConfig{
		Name:           RuntimeContainer,
		Image:          "debian:stable-slim",
		EnvInheritMode: EnvInheritAll,
		Volumes:        []VolumeMountSpec{"./data:/data"},
		Ports:          []PortMappingSpec{"8080:80"},
	}
	if err := rc.Validate(); err != nil {
		t.Fatalf("valid container RuntimeConfig.Validate() returned error: %v", err)
	}
}

func TestRuntimeConfig_Validate_InvalidName(t *testing.T) {
	t.Parallel()
	rc := RuntimeConfig{Name: "bogus"}
	err := rc.Validate()
	if err == nil {
		t.Fatal("RuntimeConfig with invalid name should fail")
	}
	if !errors.Is(err, ErrInvalidRuntimeConfig) {
		t.Errorf("error should wrap ErrInvalidRuntimeConfig, got: %v", err)
	}
}

func TestRuntimeConfig_Validate_InvalidEnvInheritMode(t *testing.T) {
	t.Parallel()
	rc := RuntimeConfig{
		Name:           RuntimeNative,
		EnvInheritMode: "bogus",
	}
	err := rc.Validate()
	if err == nil {
		t.Fatal("RuntimeConfig with invalid EnvInheritMode should fail")
	}
	if !errors.Is(err, ErrInvalidRuntimeConfig) {
		t.Errorf("error should wrap ErrInvalidRuntimeConfig, got: %v", err)
	}
}

func TestRuntimeConfig_Validate_InvalidImage(t *testing.T) {
	t.Parallel()
	rc := RuntimeConfig{
		Name:  RuntimeContainer,
		Image: "   ", // whitespace-only
	}
	err := rc.Validate()
	if err == nil {
		t.Fatal("RuntimeConfig with whitespace-only image should fail")
	}
	if !errors.Is(err, ErrInvalidRuntimeConfig) {
		t.Errorf("error should wrap ErrInvalidRuntimeConfig, got: %v", err)
	}
}

func TestRuntimeConfig_Validate_InvalidEnvInheritAllow(t *testing.T) {
	t.Parallel()
	rc := RuntimeConfig{
		Name:            RuntimeNative,
		EnvInheritAllow: []EnvVarName{"VALID", "123-INVALID"},
	}
	err := rc.Validate()
	if err == nil {
		t.Fatal("RuntimeConfig with invalid env var name should fail")
	}
	if !errors.Is(err, ErrInvalidRuntimeConfig) {
		t.Errorf("error should wrap ErrInvalidRuntimeConfig, got: %v", err)
	}
}

func TestRuntimeConfig_Validate_InvalidContainerfile(t *testing.T) {
	t.Parallel()
	rc := RuntimeConfig{
		Name:          RuntimeContainer,
		Containerfile: "   ", // whitespace-only
	}
	err := rc.Validate()
	if err == nil {
		t.Fatal("RuntimeConfig with whitespace-only containerfile should fail")
	}
	if !errors.Is(err, ErrInvalidRuntimeConfig) {
		t.Errorf("error should wrap ErrInvalidRuntimeConfig, got: %v", err)
	}
}

func TestRuntimeConfig_Validate_WithDependsOn(t *testing.T) {
	t.Parallel()
	rc := RuntimeConfig{
		Name: RuntimeContainer,
		DependsOn: &DependsOn{
			Tools: []ToolDependency{{Alternatives: []BinaryName{""}}}, // invalid empty binary
		},
	}
	err := rc.Validate()
	if err == nil {
		t.Fatal("RuntimeConfig with invalid DependsOn should fail")
	}
	if !errors.Is(err, ErrInvalidRuntimeConfig) {
		t.Errorf("error should wrap ErrInvalidRuntimeConfig, got: %v", err)
	}
}

func TestRuntimeConfig_Validate_MultipleErrors(t *testing.T) {
	t.Parallel()
	rc := RuntimeConfig{
		Name:           "bogus",
		EnvInheritMode: "bogus",
		Image:          "   ",
	}
	err := rc.Validate()
	if err == nil {
		t.Fatal("RuntimeConfig with multiple invalid fields should fail")
	}
	var rcErr *InvalidRuntimeConfigError
	if !errors.As(err, &rcErr) {
		t.Fatalf("error should be *InvalidRuntimeConfigError, got: %T", err)
	}
	if len(rcErr.FieldErrors) < 2 {
		t.Errorf("expected at least 2 field errors, got %d", len(rcErr.FieldErrors))
	}
}

func TestInvalidRuntimeConfigError_ErrorMessage(t *testing.T) {
	t.Parallel()
	e := &InvalidRuntimeConfigError{FieldErrors: []error{errors.New("a"), errors.New("b"), errors.New("c")}}
	got := e.Error()
	want := "invalid runtime config: 3 field error(s)"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestInvalidRuntimeConfigError_Unwrap(t *testing.T) {
	t.Parallel()
	e := &InvalidRuntimeConfigError{}
	if !errors.Is(e, ErrInvalidRuntimeConfig) {
		t.Error("Unwrap() should return ErrInvalidRuntimeConfig")
	}
}
