// SPDX-License-Identifier: MPL-2.0

package platform

import (
	"errors"
	"slices"
	"testing"
)

func TestDetectSandboxFrom_NoSandbox(t *testing.T) {
	t.Parallel()

	result := detectSandboxFrom(
		func(string) string { return "" },
		func(string) error { return errors.New("not found") },
	)

	if result != SandboxNone {
		t.Errorf("expected SandboxNone, got %q", result)
	}
}

func TestDetectSandboxFrom_Flatpak(t *testing.T) {
	t.Parallel()

	result := detectSandboxFrom(
		func(string) string { return "" },
		func(string) error { return nil }, // /.flatpak-info exists
	)

	if result != SandboxFlatpak {
		t.Errorf("expected SandboxFlatpak, got %q", result)
	}
}

func TestDetectSandboxFrom_Snap(t *testing.T) {
	t.Parallel()

	result := detectSandboxFrom(
		func(key string) string {
			if key == "SNAP_NAME" {
				return "test-snap"
			}
			return ""
		},
		func(string) error { return errors.New("not found") },
	)

	if result != SandboxSnap {
		t.Errorf("expected SandboxSnap, got %q", result)
	}
}

func TestDetectSandboxFrom_FlatpakTakesPrecedence(t *testing.T) {
	t.Parallel()

	// Both Flatpak file exists and SNAP_NAME is set — Flatpak wins.
	result := detectSandboxFrom(
		func(key string) string {
			if key == "SNAP_NAME" {
				return "test-snap"
			}
			return ""
		},
		func(string) error { return nil }, // /.flatpak-info exists
	)

	if result != SandboxFlatpak {
		t.Errorf("expected SandboxFlatpak (takes precedence over Snap), got %q", result)
	}
}

func TestIsInSandbox(t *testing.T) {
	t.Parallel()

	// Verify consistency: IsInSandbox() should agree with DetectSandbox() != SandboxNone.
	inSandbox := IsInSandbox()
	if inSandbox != (DetectSandbox() != SandboxNone) {
		t.Error("IsInSandbox inconsistent with DetectSandbox")
	}
}

func TestSpawnCommandFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sandbox SandboxType
		want    string
	}{
		{name: "no sandbox", sandbox: SandboxNone, want: ""},
		{name: "flatpak", sandbox: SandboxFlatpak, want: "flatpak-spawn"},
		{name: "snap", sandbox: SandboxSnap, want: "snap"},
		{name: "unknown", sandbox: SandboxType("unknown"), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SpawnCommandFor(tt.sandbox)
			if got != tt.want {
				t.Errorf("SpawnCommandFor(%q) = %q, want %q", tt.sandbox, got, tt.want)
			}
		})
	}
}

func TestSpawnArgsFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sandbox SandboxType
		want    []string
	}{
		{name: "no sandbox", sandbox: SandboxNone, want: nil},
		{name: "flatpak", sandbox: SandboxFlatpak, want: []string{"--host"}},
		{name: "snap", sandbox: SandboxSnap, want: []string{"run", "--shell"}},
		{name: "unknown", sandbox: SandboxType("unknown"), want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SpawnArgsFor(tt.sandbox)
			if tt.want == nil {
				if got != nil {
					t.Errorf("SpawnArgsFor(%q) = %v, want nil", tt.sandbox, got)
				}
				return
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("SpawnArgsFor(%q) = %v, want %v", tt.sandbox, got, tt.want)
			}
		})
	}
}

func TestSandboxTypeConstants(t *testing.T) {
	t.Parallel()

	// Verify type constants are distinct.
	types := []SandboxType{SandboxNone, SandboxFlatpak, SandboxSnap}
	seen := make(map[SandboxType]bool)

	for _, st := range types {
		if seen[st] {
			t.Errorf("duplicate SandboxType constant: %q", st)
		}
		seen[st] = true
	}

	// Verify SandboxNone is empty string for boolean-like checks.
	if SandboxNone != "" {
		t.Errorf("SandboxNone should be empty string, got %q", SandboxNone)
	}
}

func TestSandboxType_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sandbox SandboxType
		want    bool
		wantErr bool
	}{
		{"none", SandboxNone, true, false},
		{"flatpak", SandboxFlatpak, true, false},
		{"snap", SandboxSnap, true, false},
		{"invalid", SandboxType("invalid"), false, true},
		{"unknown", SandboxType("unknown"), false, true},
		{"FLATPAK", SandboxType("FLATPAK"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.sandbox.IsValid()
			if isValid != tt.want {
				t.Errorf("SandboxType(%q).IsValid() = %v, want %v", tt.sandbox, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("SandboxType(%q).IsValid() returned no errors, want error", tt.sandbox)
				}
				if !errors.Is(errs[0], ErrInvalidSandboxType) {
					t.Errorf("error should wrap ErrInvalidSandboxType, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("SandboxType(%q).IsValid() returned unexpected errors: %v", tt.sandbox, errs)
			}
		})
	}
}
