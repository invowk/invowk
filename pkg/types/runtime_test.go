// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
)

func TestRuntimeModeValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     RuntimeMode
		wantValid bool
	}{
		{name: "native", value: RuntimeNative, wantValid: true},
		{name: "virtual", value: RuntimeVirtual, wantValid: true},
		{name: "container", value: RuntimeContainer, wantValid: true},
		{name: "empty", value: "", wantValid: false},
		{name: "unknown", value: "magical", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("RuntimeMode(%q).Validate() error = %v, wantValid %v", tt.value, err, tt.wantValid)
			}
			if tt.wantValid {
				return
			}
			if !errors.Is(err, ErrInvalidRuntimeMode) {
				t.Errorf("error does not wrap ErrInvalidRuntimeMode: %v", err)
			}
		})
	}
}

func TestRuntimeModeString(t *testing.T) {
	t.Parallel()

	if got := RuntimeVirtual.String(); got != "virtual" {
		t.Errorf("RuntimeVirtual.String() = %q, want virtual", got)
	}
}
