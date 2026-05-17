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
	}{
		{name: "empty default", value: "", want: 0},
		{name: "positive", value: "2m30s", want: 2*time.Minute + 30*time.Second},
		{name: "malformed", value: "soon", wantErr: true, sentinel: ErrInvalidOptionalPositiveDurationString},
		{name: "zero", value: "0s", wantErr: true, sentinel: ErrInvalidOptionalPositiveDurationString},
		{name: "negative", value: "-1s", wantErr: true, sentinel: ErrInvalidOptionalPositiveDurationString},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.value.Duration()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Duration() error = nil, want error")
				}
				if !errors.Is(err, tt.sentinel) {
					t.Fatalf("Duration() error = %v, want sentinel %v", err, tt.sentinel)
				}
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
			if err := tt.value.Validate(); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}
