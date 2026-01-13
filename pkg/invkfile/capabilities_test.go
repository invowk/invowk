package invkfile

import (
	"testing"
)

func TestCheckCapability_LocalAreaNetwork(t *testing.T) {
	// This test assumes the test machine has network connectivity
	// In a CI environment without network, this test may fail
	err := CheckCapability(CapabilityLocalAreaNetwork)

	// We don't assert success/failure since it depends on the test environment
	// Instead, we just verify the function runs without panicking
	// and returns a proper error type if it fails
	if err != nil {
		if _, ok := err.(*CapabilityError); !ok {
			t.Errorf("CheckCapability(CapabilityLocalAreaNetwork) returned wrong error type: %T", err)
		}
	}
}

func TestCheckCapability_Internet(t *testing.T) {
	// This test assumes the test machine has internet connectivity
	// In a CI environment without internet, this test may fail
	err := CheckCapability(CapabilityInternet)

	// We don't assert success/failure since it depends on the test environment
	// Instead, we just verify the function runs without panicking
	// and returns a proper error type if it fails
	if err != nil {
		if _, ok := err.(*CapabilityError); !ok {
			t.Errorf("CheckCapability(CapabilityInternet) returned wrong error type: %T", err)
		}
	}
}

func TestCheckCapability_Unknown(t *testing.T) {
	err := CheckCapability(CapabilityName("unknown-capability"))
	if err == nil {
		t.Error("CheckCapability should return error for unknown capability")
	}

	capErr, ok := err.(*CapabilityError)
	if !ok {
		t.Fatalf("CheckCapability should return *CapabilityError, got: %T", err)
	}

	if capErr.Capability != "unknown-capability" {
		t.Errorf("CapabilityError.Capability = %q, want %q", capErr.Capability, "unknown-capability")
	}

	if capErr.Message != "unknown capability" {
		t.Errorf("CapabilityError.Message = %q, want %q", capErr.Message, "unknown capability")
	}
}

func TestCapabilityError_Error(t *testing.T) {
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
	names := ValidCapabilityNames()

	if len(names) != 2 {
		t.Errorf("ValidCapabilityNames() returned %d names, want 2", len(names))
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
}

func TestIsValidCapabilityName(t *testing.T) {
	tests := []struct {
		name     CapabilityName
		expected bool
	}{
		{CapabilityLocalAreaNetwork, true},
		{CapabilityInternet, true},
		{CapabilityName("unknown"), false},
		{CapabilityName(""), false},
		{CapabilityName("local-area-network"), true}, // explicit string match
		{CapabilityName("internet"), true},           // explicit string match
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			result := IsValidCapabilityName(tt.name)
			if result != tt.expected {
				t.Errorf("IsValidCapabilityName(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestCheckLocalAreaNetwork_ReturnsCapabilityError(t *testing.T) {
	// Call the internal function to test its behavior
	// This will succeed or fail based on the environment
	err := checkLocalAreaNetwork()
	if err != nil {
		capErr, ok := err.(*CapabilityError)
		if !ok {
			t.Errorf("checkLocalAreaNetwork() returned wrong error type: %T", err)
		}
		if capErr.Capability != CapabilityLocalAreaNetwork {
			t.Errorf("CapabilityError.Capability = %q, want %q", capErr.Capability, CapabilityLocalAreaNetwork)
		}
	}
}

func TestCheckInternet_ReturnsCapabilityError(t *testing.T) {
	// Call the internal function to test its behavior
	// This will succeed or fail based on the environment
	err := checkInternet()
	if err != nil {
		capErr, ok := err.(*CapabilityError)
		if !ok {
			t.Errorf("checkInternet() returned wrong error type: %T", err)
		}
		if capErr.Capability != CapabilityInternet {
			t.Errorf("CapabilityError.Capability = %q, want %q", capErr.Capability, CapabilityInternet)
		}
	}
}
