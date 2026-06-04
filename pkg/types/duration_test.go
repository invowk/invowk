// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
	"time"
)

func TestOptionalPositiveDurationString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    OptionalPositiveDurationString
		want     time.Duration
		wantErr  bool
		sentinel error
		reason   string
	}{
		{name: "empty default", value: "", want: 0},
		{name: "one nanosecond is positive", value: "1ns", want: time.Nanosecond},
		{name: "positive", value: "2m30s", want: 2*time.Minute + 30*time.Second},
		{
			name:     "malformed",
			value:    "soon",
			wantErr:  true,
			sentinel: ErrInvalidOptionalPositiveDurationString,
			reason:   "time: invalid duration \"soon\"",
		},
		{
			name:     "zero",
			value:    "0s",
			wantErr:  true,
			sentinel: ErrInvalidOptionalPositiveDurationString,
			reason:   "must be a positive duration",
		},
		{
			name:     "negative",
			value:    "-1s",
			wantErr:  true,
			sentinel: ErrInvalidOptionalPositiveDurationString,
			reason:   "must be a positive duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.value.Duration()
			if tt.wantErr {
				if got != 0 {
					t.Fatalf("Duration() = %v with error, want 0", got)
				}
				if err == nil {
					t.Fatal("Duration() error = nil, want error")
				}
				if !errors.Is(err, tt.sentinel) {
					t.Fatalf("Duration() error = %v, want sentinel %v", err, tt.sentinel)
				}
				assertInvalidDurationDetails(t, err, tt.value, tt.reason)
				if validateErr := tt.value.Validate(); !errors.Is(validateErr, tt.sentinel) {
					t.Fatalf("Validate() error = %v, want sentinel %v", validateErr, tt.sentinel)
				}
				return
			}
			if err != nil {
				t.Fatalf("Duration() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("Duration() = %v, want %v", got, tt.want)
			}
			if tt.value.String() != string(tt.value) {
				t.Fatalf("String() = %q, want %q", tt.value.String(), string(tt.value))
			}
			if err := tt.value.Validate(); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func assertInvalidDurationDetails(
	t *testing.T,
	err error,
	wantValue OptionalPositiveDurationString,
	wantReason string,
) {
	t.Helper()

	var invalid *InvalidOptionalPositiveDurationStringError
	if !errors.As(err, &invalid) {
		t.Fatalf("Duration() error type = %T, want *InvalidOptionalPositiveDurationStringError", err)
	}
	if invalid.Value != wantValue {
		t.Fatalf("InvalidOptionalPositiveDurationStringError.Value = %q, want %q", invalid.Value, wantValue)
	}
	if invalid.Reason != wantReason {
		t.Fatalf("InvalidOptionalPositiveDurationStringError.Reason = %q, want %q", invalid.Reason, wantReason)
	}

	wantError := "invalid optional positive duration string \"" + string(wantValue) + "\": " + wantReason
	if got := invalid.Error(); got != wantError {
		t.Fatalf("InvalidOptionalPositiveDurationStringError.Error() = %q, want %q", got, wantError)
	}
}
