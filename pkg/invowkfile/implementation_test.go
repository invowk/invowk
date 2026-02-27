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

func TestPlatformRuntimeKey_Validate(t *testing.T) {
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
			err := tt.key.Validate()
			if (err == nil) != tt.want {
				t.Errorf("PlatformRuntimeKey{%q, %q}.Validate() error = %v, want valid=%v",
					tt.key.Platform, tt.key.Runtime, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PlatformRuntimeKey{%q, %q}.Validate() returned nil, want error",
						tt.key.Platform, tt.key.Runtime)
				}
			} else if err != nil {
				t.Errorf("PlatformRuntimeKey{%q, %q}.Validate() returned unexpected error: %v",
					tt.key.Platform, tt.key.Runtime, err)
			}
		})
	}
}

func TestPlatformRuntimeKey_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  PlatformRuntimeKey
		want string
	}{
		{"linux_native", PlatformRuntimeKey{Platform: PlatformLinux, Runtime: RuntimeNative}, "linux/native"},
		{"macos_virtual", PlatformRuntimeKey{Platform: PlatformMac, Runtime: RuntimeVirtual}, "macos/virtual"},
		{"windows_container", PlatformRuntimeKey{Platform: PlatformWindows, Runtime: RuntimeContainer}, "windows/container"},
		{"empty_both", PlatformRuntimeKey{}, "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.key.String()
			if got != tt.want {
				t.Errorf("PlatformRuntimeKey.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlatformRuntimeKey_Validate_BothInvalidAggregatesErrors(t *testing.T) {
	t.Parallel()

	key := PlatformRuntimeKey{Platform: "bogus-platform", Runtime: "bogus-runtime"}
	err := key.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// The joined error should contain both platform and runtime errors
	if !errors.Is(err, ErrInvalidPlatform) {
		t.Errorf("error should wrap ErrInvalidPlatform, got: %v", err)
	}
	if !errors.Is(err, ErrInvalidRuntimeMode) {
		t.Errorf("error should wrap ErrInvalidRuntimeMode, got: %v", err)
	}
}
