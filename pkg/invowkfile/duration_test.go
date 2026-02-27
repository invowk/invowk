// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestDurationStringValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     DurationString
		wantValid bool
	}{
		{name: "empty is valid", value: "", wantValid: true},
		{name: "30s is valid", value: "30s", wantValid: true},
		{name: "5m is valid", value: "5m", wantValid: true},
		{name: "1h30m is valid", value: "1h30m", wantValid: true},
		{name: "500ms is valid", value: "500ms", wantValid: true},
		{name: "1ns is valid", value: "1ns", wantValid: true},
		{name: "invalid string", value: "invalid", wantValid: false},
		{name: "number without unit", value: "30", wantValid: false},
		{name: "zero duration", value: "0s", wantValid: false},
		{name: "negative duration", value: "-5m", wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("DurationString(%q).Validate() error = %v, want valid=%v", tt.value, err, tt.wantValid)
			}
			if tt.wantValid {
				if err != nil {
					t.Errorf("DurationString(%q).Validate() returned error for valid value: %v", tt.value, err)
				}
			} else {
				if err == nil {
					t.Error("DurationString.Validate() returned nil for invalid value")
				}
				if !errors.Is(err, ErrInvalidDurationString) {
					t.Errorf("error does not wrap ErrInvalidDurationString: %v", err)
				}
			}
		})
	}
}

func TestDurationStringString(t *testing.T) {
	t.Parallel()

	d := DurationString("30s")
	if got := d.String(); got != "30s" {
		t.Errorf("DurationString.String() = %q, want %q", got, "30s")
	}
}
