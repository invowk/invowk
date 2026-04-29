// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestHostCapabilityChecker_Check(t *testing.T) {
	t.Parallel()

	checker := newHostCapabilityChecker()

	t.Run("local area network", func(t *testing.T) {
		t.Parallel()
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		assertCapabilityErrorType(t, checker.Check(invowkfile.CapabilityLocalAreaNetwork))
	})

	t.Run("internet", func(t *testing.T) {
		t.Parallel()
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		assertCapabilityErrorType(t, checker.Check(invowkfile.CapabilityInternet))
	})

	t.Run("containers", func(t *testing.T) {
		t.Parallel()
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		assertCapabilityErrorType(t, checker.Check(invowkfile.CapabilityContainers))
	})

	t.Run("tty", func(t *testing.T) {
		t.Parallel()
		assertCapabilityErrorType(t, checker.Check(invowkfile.CapabilityTTY))
	})

	t.Run("unknown", func(t *testing.T) {
		t.Parallel()

		err := checker.Check(invowkfile.CapabilityName("unknown-capability"))
		var capErr *invowkfile.CapabilityError
		if !errors.As(err, &capErr) {
			t.Fatalf("errors.As(*CapabilityError) = false for %T", err)
		}
		if capErr.Capability != "unknown-capability" {
			t.Errorf("CapabilityError.Capability = %q, want %q", capErr.Capability, "unknown-capability")
		}
		if capErr.Message != "unknown capability" {
			t.Errorf("CapabilityError.Message = %q, want %q", capErr.Message, "unknown capability")
		}
	})
}

func TestCheckLocalAreaNetwork_ReturnsCapabilityError(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	err := checkLocalAreaNetwork()
	if err != nil {
		var capErr *invowkfile.CapabilityError
		if !errors.As(err, &capErr) {
			t.Errorf("errors.As(*CapabilityError) = false for %T", err)
		}
		if capErr != nil && capErr.Capability != invowkfile.CapabilityLocalAreaNetwork {
			t.Errorf("CapabilityError.Capability = %q, want %q", capErr.Capability, invowkfile.CapabilityLocalAreaNetwork)
		}
	}
}

func TestCheckInternet_ReturnsCapabilityError(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	err := checkInternet()
	if err != nil {
		var capErr *invowkfile.CapabilityError
		if !errors.As(err, &capErr) {
			t.Errorf("errors.As(*CapabilityError) = false for %T", err)
		}
		if capErr != nil && capErr.Capability != invowkfile.CapabilityInternet {
			t.Errorf("CapabilityError.Capability = %q, want %q", capErr.Capability, invowkfile.CapabilityInternet)
		}
	}
}

func assertCapabilityErrorType(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}
	var capErr *invowkfile.CapabilityError
	if !errors.As(err, &capErr) {
		t.Errorf("errors.As(*CapabilityError) = false for %T", err)
	}
}
