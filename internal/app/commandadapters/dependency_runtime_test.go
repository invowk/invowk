// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestCapabilityCheckScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		capability   invowkfile.CapabilityName
		wantNonEmpty bool
	}{
		{"internet", invowkfile.CapabilityInternet, true},
		{"containers", invowkfile.CapabilityContainers, true},
		{"lan", invowkfile.CapabilityLocalAreaNetwork, true},
		{"tty", invowkfile.CapabilityTTY, true},
		{"unknown", invowkfile.CapabilityName("bogus"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := capabilityCheckScript(tt.capability)
			if tt.wantNonEmpty && got == "" {
				t.Fatalf("capabilityCheckScript(%q) = empty, want non-empty script", tt.capability)
			}
			if !tt.wantNonEmpty && got != "" {
				t.Fatalf("capabilityCheckScript(%q) = %q, want empty string", tt.capability, got)
			}
		})
	}
}
