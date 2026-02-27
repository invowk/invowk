// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"testing"
)

func TestEngineNotAvailableError_Error(t *testing.T) {
	t.Parallel()

	err := &EngineNotAvailableError{
		Engine: "podman",
		Reason: "not installed",
	}

	expected := "container engine 'podman' is not available: not installed"
	if err.Error() != expected {
		t.Errorf("EngineNotAvailableError.Error() = %s, want %s", err.Error(), expected)
	}
}

func TestEngineNotAvailableError_UnwrapsToSentinel(t *testing.T) {
	t.Parallel()

	err := &EngineNotAvailableError{
		Engine: "docker",
		Reason: "not installed",
	}

	if !errors.Is(err, ErrNoEngineAvailable) {
		t.Error("EngineNotAvailableError should unwrap to ErrNoEngineAvailable")
	}
}

func TestErrNoEngineAvailable_Sentinel(t *testing.T) {
	t.Parallel()

	if ErrNoEngineAvailable == nil {
		t.Fatal("ErrNoEngineAvailable should not be nil")
	}
	if ErrNoEngineAvailable.Error() != "no container engine available" {
		t.Errorf("ErrNoEngineAvailable.Error() = %q, want %q", ErrNoEngineAvailable.Error(), "no container engine available")
	}
}

func TestDockerEngine_AvailableWithNoPath(t *testing.T) {
	t.Parallel()

	// Engine created with no binary path should not be available
	engine := &DockerEngine{BaseCLIEngine: NewBaseCLIEngine("")}
	if engine.Available() {
		t.Error("DockerEngine with empty path should not be available")
	}
}

func TestPodmanEngine_AvailableWithNoPath(t *testing.T) {
	t.Parallel()

	// Engine created with no binary path should not be available
	engine := &PodmanEngine{BaseCLIEngine: NewBaseCLIEngine("")}
	if engine.Available() {
		t.Error("PodmanEngine with empty path should not be available")
	}
}

func TestEngineType_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		et      EngineType
		want    bool
		wantErr bool
	}{
		{EngineTypePodman, true, false},
		{EngineTypeDocker, true, false},
		{"", false, true},
		{"unknown", false, true},
		{"PODMAN", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.et), func(t *testing.T) {
			t.Parallel()
			err := tt.et.Validate()
			if (err == nil) != tt.want {
				t.Errorf("EngineType(%q).Validate() error = %v, want valid=%v", tt.et, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("EngineType(%q).Validate() returned nil, want error", tt.et)
				}
				if !errors.Is(err, ErrInvalidEngineType) {
					t.Errorf("error should wrap ErrInvalidEngineType, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("EngineType(%q).Validate() returned unexpected error: %v", tt.et, err)
			}
		})
	}
}

func TestNewEngine_UnknownType(t *testing.T) {
	t.Parallel()

	_, err := NewEngine("unknown")
	if err == nil {
		t.Error("NewEngine with unknown type should return error")
	}
	if !errors.Is(err, ErrInvalidEngineType) {
		t.Errorf("NewEngine with unknown type should return ErrInvalidEngineType, got: %v", err)
	}
}

func TestNewEngine_Podman(t *testing.T) {
	t.Parallel()

	// This test verifies the logic, not actual availability
	engine, err := NewEngine(EngineTypePodman)
	// If neither podman nor docker is available, we should get an error
	if err != nil {
		if _, ok := errors.AsType[*EngineNotAvailableError](err); !ok {
			t.Errorf("expected EngineNotAvailableError, got %T", err)
		}
		return
	}

	// If we got an engine, it should be either podman or docker (fallback)
	if engine.Name() != "podman" && engine.Name() != "docker" {
		t.Errorf("expected podman or docker engine, got %s", engine.Name())
	}
}

func TestNewEngine_Docker(t *testing.T) {
	t.Parallel()

	// This test verifies the logic, not actual availability
	engine, err := NewEngine(EngineTypeDocker)
	// If neither docker nor podman is available, we should get an error
	if err != nil {
		if _, ok := errors.AsType[*EngineNotAvailableError](err); !ok {
			t.Errorf("expected EngineNotAvailableError, got %T", err)
		}
		return
	}

	// If we got an engine, it should be either docker or podman (fallback)
	if engine.Name() != "docker" && engine.Name() != "podman" {
		t.Errorf("expected docker or podman engine, got %s", engine.Name())
	}
}

func TestAutoDetectEngine(t *testing.T) {
	t.Parallel()

	engine, err := AutoDetectEngine()
	// If no engine is available, we should get an error
	if err != nil {
		if _, ok := errors.AsType[*EngineNotAvailableError](err); !ok {
			t.Errorf("expected EngineNotAvailableError, got %T: %v", err, err)
		}
		return
	}

	// If we got an engine, it should be either podman or docker
	if engine.Name() != "podman" && engine.Name() != "docker" {
		t.Errorf("expected podman or docker engine, got %s", engine.Name())
	}
}

func TestPortProtocol_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pp      PortProtocol
		want    bool
		wantErr bool
	}{
		{"tcp", PortProtocolTCP, true, false},
		{"udp", PortProtocolUDP, true, false},
		{"zero value (empty)", PortProtocol(""), true, false},
		{"invalid protocol", PortProtocol("sctp"), false, true},
		{"uppercase TCP", PortProtocol("TCP"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.pp.Validate()
			if (err == nil) != tt.want {
				t.Errorf("PortProtocol(%q).Validate() error = %v, want valid=%v", tt.pp, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PortProtocol(%q).Validate() returned nil, want error", tt.pp)
				}
				if !errors.Is(err, ErrInvalidPortProtocol) {
					t.Errorf("error should wrap ErrInvalidPortProtocol, got: %v", err)
				}
				var typedErr *InvalidPortProtocolError
				if !errors.As(err, &typedErr) {
					t.Errorf("error should be *InvalidPortProtocolError, got: %T", err)
				} else if typedErr.Value != tt.pp {
					t.Errorf("InvalidPortProtocolError.Value = %q, want %q", typedErr.Value, tt.pp)
				}
			} else if err != nil {
				t.Errorf("PortProtocol(%q).Validate() returned unexpected error: %v", tt.pp, err)
			}
		})
	}
}

func TestPortProtocol_String(t *testing.T) {
	t.Parallel()

	if got := PortProtocolTCP.String(); got != "tcp" {
		t.Errorf("PortProtocolTCP.String() = %q, want %q", got, "tcp")
	}
	if got := PortProtocolUDP.String(); got != "udp" {
		t.Errorf("PortProtocolUDP.String() = %q, want %q", got, "udp")
	}
}

func TestSELinuxLabel_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sl      SELinuxLabel
		want    bool
		wantErr bool
	}{
		{"none (empty)", SELinuxLabelNone, true, false},
		{"shared (z)", SELinuxLabelShared, true, false},
		{"private (Z)", SELinuxLabelPrivate, true, false},
		{"invalid label", SELinuxLabel("x"), false, true},
		{"lowercase z valid", SELinuxLabel("z"), true, false},
		{"uppercase Z valid", SELinuxLabel("Z"), true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.sl.Validate()
			if (err == nil) != tt.want {
				t.Errorf("SELinuxLabel(%q).Validate() error = %v, want valid=%v", tt.sl, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SELinuxLabel(%q).Validate() returned nil, want error", tt.sl)
				}
				if !errors.Is(err, ErrInvalidSELinuxLabel) {
					t.Errorf("error should wrap ErrInvalidSELinuxLabel, got: %v", err)
				}
				var typedErr *InvalidSELinuxLabelError
				if !errors.As(err, &typedErr) {
					t.Errorf("error should be *InvalidSELinuxLabelError, got: %T", err)
				} else if typedErr.Value != tt.sl {
					t.Errorf("InvalidSELinuxLabelError.Value = %q, want %q", typedErr.Value, tt.sl)
				}
			} else if err != nil {
				t.Errorf("SELinuxLabel(%q).Validate() returned unexpected error: %v", tt.sl, err)
			}
		})
	}
}

func TestSELinuxLabel_String(t *testing.T) {
	t.Parallel()

	if got := SELinuxLabelNone.String(); got != "" {
		t.Errorf("SELinuxLabelNone.String() = %q, want %q", got, "")
	}
	if got := SELinuxLabelShared.String(); got != "z" {
		t.Errorf("SELinuxLabelShared.String() = %q, want %q", got, "z")
	}
	if got := SELinuxLabelPrivate.String(); got != "Z" {
		t.Errorf("SELinuxLabelPrivate.String() = %q, want %q", got, "Z")
	}
}

// Integration tests - only run if container engine is available
func TestDockerEngine_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine := NewDockerEngine()
	if !engine.Available() {
		t.Skip("Docker is not available, skipping integration tests")
	}

	ctx := t.Context()

	t.Run("Version", func(t *testing.T) {
		version, err := engine.Version(ctx)
		if err != nil {
			t.Errorf("Version() returned error: %v", err)
		}
		if version == "" {
			t.Error("Version() returned empty string")
		}
		t.Logf("Docker version: %s", version)
	})

	t.Run("ImageExists_NonExistent", func(t *testing.T) {
		exists, err := engine.ImageExists(ctx, "invowk-test-nonexistent-image:latest")
		if err != nil {
			t.Errorf("ImageExists() returned error: %v", err)
		}
		if exists {
			t.Error("ImageExists() returned true for non-existent image")
		}
	})
}

func TestEngineType_String(t *testing.T) {
	t.Parallel()

	if got := EngineTypePodman.String(); got != "podman" {
		t.Errorf("EngineTypePodman.String() = %q, want %q", got, "podman")
	}
	if got := EngineType("").String(); got != "" {
		t.Errorf("EngineType(\"\").String() = %q, want %q", got, "")
	}
}

func TestPodmanEngine_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine := NewPodmanEngine()
	if !engine.Available() {
		t.Skip("Podman is not available, skipping integration tests")
	}

	ctx := t.Context()

	t.Run("Version", func(t *testing.T) {
		version, err := engine.Version(ctx)
		if err != nil {
			t.Errorf("Version() returned error: %v", err)
		}
		if version == "" {
			t.Error("Version() returned empty string")
		}
		t.Logf("Podman version: %s", version)
	})

	t.Run("ImageExists_NonExistent", func(t *testing.T) {
		exists, err := engine.ImageExists(ctx, "invowk-test-nonexistent-image:latest")
		if err != nil {
			t.Errorf("ImageExists() returned error: %v", err)
		}
		if exists {
			t.Error("ImageExists() returned true for non-existent image")
		}
	})
}
