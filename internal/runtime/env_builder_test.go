// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestEnvBuilder_InterfaceContract verifies that DefaultEnvBuilder and MockEnvBuilder
// both satisfy the EnvBuilder interface.
func TestEnvBuilder_InterfaceContract(t *testing.T) {
	t.Parallel()

	var _ EnvBuilder = &DefaultEnvBuilder{}
	var _ EnvBuilder = &MockEnvBuilder{}
}

// TestMockEnvBuilder_ReturnsConfiguredEnv verifies that MockEnvBuilder returns
// the configured environment map.
func TestMockEnvBuilder_ReturnsConfiguredEnv(t *testing.T) {
	t.Parallel()

	mock := &MockEnvBuilder{
		Env: map[string]string{
			"TEST_VAR": "test_value",
			"FOO":      "bar",
		},
	}

	env, err := mock.Build(nil, invowkfile.EnvInheritAll)
	if err != nil {
		t.Fatalf("MockEnvBuilder.Build() unexpected error: %v", err)
	}

	if got := env["TEST_VAR"]; got != "test_value" {
		t.Errorf("TEST_VAR = %q, want %q", got, "test_value")
	}
	if got := env["FOO"]; got != "bar" {
		t.Errorf("FOO = %q, want %q", got, "bar")
	}
}

// TestMockEnvBuilder_ReturnsCopy verifies that MockEnvBuilder returns a copy
// of the environment, not the original map (preventing mutation).
func TestMockEnvBuilder_ReturnsCopy(t *testing.T) {
	t.Parallel()

	original := map[string]string{"KEY": "value"}
	mock := &MockEnvBuilder{Env: original}

	env1, _ := mock.Build(nil, invowkfile.EnvInheritAll)
	env1["KEY"] = "mutated"

	env2, _ := mock.Build(nil, invowkfile.EnvInheritAll)
	if got := env2["KEY"]; got != "value" {
		t.Errorf("MockEnvBuilder.Build() should return a copy; got mutated value %q", got)
	}
}

// TestMockEnvBuilder_ReturnsError verifies that MockEnvBuilder returns
// the configured error.
func TestMockEnvBuilder_ReturnsError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("mock error")
	mock := &MockEnvBuilder{
		Err: expectedErr,
		Env: map[string]string{"KEY": "value"}, // Should be ignored when Err is set
	}

	env, err := mock.Build(nil, invowkfile.EnvInheritAll)
	if !errors.Is(err, expectedErr) {
		t.Errorf("MockEnvBuilder.Build() error = %v, want %v", err, expectedErr)
	}
	if env != nil {
		t.Errorf("MockEnvBuilder.Build() should return nil env when error is set")
	}
}

// TestMockEnvBuilder_NilEnvReturnsEmptyMap verifies that MockEnvBuilder returns
// an empty map when Env is nil (not nil).
func TestMockEnvBuilder_NilEnvReturnsEmptyMap(t *testing.T) {
	t.Parallel()

	mock := &MockEnvBuilder{Env: nil}

	env, err := mock.Build(nil, invowkfile.EnvInheritAll)
	if err != nil {
		t.Fatalf("MockEnvBuilder.Build() unexpected error: %v", err)
	}
	if env == nil {
		t.Error("MockEnvBuilder.Build() should return empty map, not nil")
	}
	if len(env) != 0 {
		t.Errorf("MockEnvBuilder.Build() should return empty map, got %v", env)
	}
}

// TestNewDefaultEnvBuilder verifies that NewDefaultEnvBuilder creates a valid builder.
func TestNewDefaultEnvBuilder(t *testing.T) {
	t.Parallel()

	builder := NewDefaultEnvBuilder()
	if builder == nil {
		t.Error("NewDefaultEnvBuilder() returned nil")
	}
}
