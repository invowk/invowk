// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
	"time"
)

func TestParseTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout DurationString
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "empty string returns zero",
			timeout: "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "30 seconds",
			timeout: "30s",
			want:    30 * time.Second,
			wantErr: false,
		},
		{
			name:    "5 minutes",
			timeout: "5m",
			want:    5 * time.Minute,
			wantErr: false,
		},
		{
			name:    "1 hour 30 minutes",
			timeout: "1h30m",
			want:    90 * time.Minute,
			wantErr: false,
		},
		{
			name:    "500 milliseconds",
			timeout: "500ms",
			want:    500 * time.Millisecond,
			wantErr: false,
		},
		{
			name:    "invalid string returns error",
			timeout: "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "number without unit returns error",
			timeout: "30",
			want:    0,
			wantErr: true,
		},
		{
			name:    "zero duration returns error",
			timeout: "0s",
			want:    0,
			wantErr: true,
		},
		{
			name:    "negative duration returns error",
			timeout: "-5m",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			impl := &Implementation{Timeout: tt.timeout}
			got, err := impl.ParseTimeout()

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlatformRuntimeKey_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     PlatformRuntimeKey
		want    bool
		wantErr bool
	}{
		{"both valid linux/native", PlatformRuntimeKey{Platform: PlatformLinux, Runtime: RuntimeNative}, true, false},
		{"both valid macos/container", PlatformRuntimeKey{Platform: PlatformMac, Runtime: RuntimeContainer}, true, false},
		{"invalid platform", PlatformRuntimeKey{Platform: "bogus", Runtime: RuntimeNative}, false, true},
		{"invalid runtime", PlatformRuntimeKey{Platform: PlatformLinux, Runtime: "bogus"}, false, true},
		{"both invalid", PlatformRuntimeKey{Platform: "bogus", Runtime: "bogus"}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.key.IsValid()
			if isValid != tt.want {
				t.Errorf("PlatformRuntimeKey{%q, %q}.IsValid() = %v, want %v",
					tt.key.Platform, tt.key.Runtime, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("PlatformRuntimeKey{%q, %q}.IsValid() returned no errors, want error",
						tt.key.Platform, tt.key.Runtime)
				}
			} else if len(errs) > 0 {
				t.Errorf("PlatformRuntimeKey{%q, %q}.IsValid() returned unexpected errors: %v",
					tt.key.Platform, tt.key.Runtime, errs)
			}
		})
	}
}

func TestPlatformRuntimeKey_IsValid_BothInvalidAggregatesErrors(t *testing.T) {
	t.Parallel()

	key := PlatformRuntimeKey{Platform: "bogus-platform", Runtime: "bogus-runtime"}
	isValid, errs := key.IsValid()
	if isValid {
		t.Fatal("expected invalid, got valid")
	}
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors (one per field), got %d: %v", len(errs), errs)
	}
	if !errors.Is(errs[0], ErrInvalidPlatform) {
		t.Errorf("first error should wrap ErrInvalidPlatform, got: %v", errs[0])
	}
	if !errors.Is(errs[1], ErrInvalidRuntimeMode) {
		t.Errorf("second error should wrap ErrInvalidRuntimeMode, got: %v", errs[1])
	}
}
