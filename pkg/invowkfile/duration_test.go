// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
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
		{name: "exact max runes is valid", value: DurationString(strings.Repeat("1h", 16)), wantValid: true},
		{name: "invalid string", value: "invalid", wantValid: false},
		{name: "number without unit", value: "30", wantValid: false},
		{name: "zero duration", value: "0s", wantValid: false},
		{name: "negative duration", value: "-5m", wantValid: false},
		{name: "over max runes", value: DurationString(strings.Repeat("1h", 17)), wantValid: false},
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

func TestDurationStringMutationErrorPayloads(t *testing.T) {
	t.Parallel()

	tooLong := DurationString(strings.Repeat("1h", 17))
	err := tooLong.Validate()
	var invalid *InvalidDurationStringError
	if !errors.As(err, &invalid) {
		t.Fatalf("Validate() error = %T, want *InvalidDurationStringError", err)
	}
	if invalid.Value != tooLong {
		t.Fatalf("InvalidDurationStringError.Value = %q, want %q", invalid.Value, tooLong)
	}
	if invalid.Reason != "must be at most 32 runes" {
		t.Fatalf("InvalidDurationStringError.Reason = %q, want max-runes reason", invalid.Reason)
	}

	zeroErr := DurationString("0s").Validate()
	if !errors.As(zeroErr, &invalid) {
		t.Fatalf("zero Validate() error = %T, want *InvalidDurationStringError", zeroErr)
	}
	if invalid.Value != "0s" || invalid.Reason != "must be a positive duration" {
		t.Fatalf("zero duration error = %+v, want value and positive-duration reason", invalid)
	}
}

func TestParseDurationMutationErrorContracts(t *testing.T) {
	t.Parallel()

	got, err := parseDuration("timeout", DurationString("0s"))
	if got != 0 {
		t.Fatalf("parseDuration() = %v, want zero duration on validation error", got)
	}
	if !errors.Is(err, ErrInvalidDurationString) {
		t.Fatalf("parseDuration() error = %v, want ErrInvalidDurationString", err)
	}
	var invalid *InvalidDurationStringError
	if !errors.As(err, &invalid) {
		t.Fatalf("parseDuration() error = %T, want wrapped *InvalidDurationStringError", err)
	}
	if invalid.Value != "0s" || invalid.Reason != "must be a positive duration" {
		t.Fatalf("wrapped duration error = %+v, want original payload", invalid)
	}
	if !strings.Contains(err.Error(), `invalid timeout "0s"`) {
		t.Fatalf("parseDuration() error = %q, want field name and value", err.Error())
	}
}

func TestDurationStringString(t *testing.T) {
	t.Parallel()

	d := DurationString("30s")
	if got := d.String(); got != "30s" {
		t.Errorf("DurationString.String() = %q, want %q", got, "30s")
	}
}
