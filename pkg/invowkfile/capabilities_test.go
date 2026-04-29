// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestCapabilityError_Error(t *testing.T) {
	t.Parallel()

	err := &CapabilityError{
		Capability: CapabilityInternet,
		Message:    "no connection available",
	}

	expected := `capability "internet" not available: no connection available`
	if err.Error() != expected {
		t.Errorf("CapabilityError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestValidCapabilityNames(t *testing.T) {
	t.Parallel()

	names := ValidCapabilityNames()

	if len(names) != 4 {
		t.Errorf("ValidCapabilityNames() returned %d names, want 4", len(names))
	}

	// Check that expected capabilities are in the list
	found := make(map[CapabilityName]bool)
	for _, name := range names {
		found[name] = true
	}

	if !found[CapabilityLocalAreaNetwork] {
		t.Error("ValidCapabilityNames() should include CapabilityLocalAreaNetwork")
	}

	if !found[CapabilityInternet] {
		t.Error("ValidCapabilityNames() should include CapabilityInternet")
	}

	if !found[CapabilityContainers] {
		t.Error("ValidCapabilityNames() should include CapabilityContainers")
	}

	if !found[CapabilityTTY] {
		t.Error("ValidCapabilityNames() should include CapabilityTTY")
	}
}

func TestCapabilityName_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    CapabilityName
		want    bool
		wantErr bool
	}{
		{CapabilityLocalAreaNetwork, true, false},
		{CapabilityInternet, true, false},
		{CapabilityContainers, true, false},
		{CapabilityTTY, true, false},
		{"", false, true},
		{"unknown", false, true},
		{"LOCAL-AREA-NETWORK", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			t.Parallel()
			err := tt.name.Validate()
			if (err == nil) != tt.want {
				t.Errorf("CapabilityName(%q).Validate() error = %v, want valid=%v", tt.name, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CapabilityName(%q).Validate() returned nil, want error", tt.name)
				}
				if !errors.Is(err, ErrInvalidCapabilityName) {
					t.Errorf("error should wrap ErrInvalidCapabilityName, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("CapabilityName(%q).Validate() returned unexpected error: %v", tt.name, err)
			}
		})
	}
}

func TestCapabilityName_Validate_ErrorType(t *testing.T) {
	t.Parallel()

	err := CapabilityName("bogus").Validate()
	if err == nil {
		t.Fatal("expected error for invalid capability name")
	}

	var invalidErr *InvalidCapabilityNameError
	if !errors.As(err, &invalidErr) {
		t.Errorf("error should be *InvalidCapabilityNameError, got %T", err)
	}
	if invalidErr.Value != "bogus" {
		t.Errorf("InvalidCapabilityNameError.Value = %q, want %q", invalidErr.Value, "bogus")
	}
}

func TestCapabilityName_String(t *testing.T) {
	t.Parallel()

	if got := CapabilityLocalAreaNetwork.String(); got != "local-area-network" {
		t.Errorf("CapabilityLocalAreaNetwork.String() = %q, want %q", got, "local-area-network")
	}
	if got := CapabilityName("").String(); got != "" {
		t.Errorf("CapabilityName(\"\").String() = %q, want %q", got, "")
	}
}
