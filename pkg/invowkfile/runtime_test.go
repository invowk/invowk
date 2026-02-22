// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestRuntimeMode_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode    RuntimeMode
		want    bool
		wantErr bool
	}{
		{RuntimeNative, true, false},
		{RuntimeVirtual, true, false},
		{RuntimeContainer, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"NATIVE", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.mode.IsValid()
			if isValid != tt.want {
				t.Errorf("RuntimeMode(%q).IsValid() = %v, want %v", tt.mode, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("RuntimeMode(%q).IsValid() returned no errors, want error", tt.mode)
				}
				if !errors.Is(errs[0], ErrInvalidRuntimeMode) {
					t.Errorf("error should wrap ErrInvalidRuntimeMode, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("RuntimeMode(%q).IsValid() returned unexpected errors: %v", tt.mode, errs)
			}
		})
	}
}

func TestPlatformType_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		platform PlatformType
		want     bool
		wantErr  bool
	}{
		{PlatformLinux, true, false},
		{PlatformMac, true, false},
		{PlatformWindows, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"darwin", false, true},  // Go uses "darwin", invowk uses "macos"
		{"LINUX", false, true},   // case-sensitive
		{"MacOS", false, true},   // case-sensitive
		{"Windows", false, true}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.platform), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.platform.IsValid()
			if isValid != tt.want {
				t.Errorf("PlatformType(%q).IsValid() = %v, want %v", tt.platform, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("PlatformType(%q).IsValid() returned no errors, want error", tt.platform)
				}
				if !errors.Is(errs[0], ErrInvalidPlatform) {
					t.Errorf("error should wrap ErrInvalidPlatform, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("PlatformType(%q).IsValid() returned unexpected errors: %v", tt.platform, errs)
			}
		})
	}
}

func TestParseRuntimeMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    RuntimeMode
		wantErr bool
	}{
		{"", "", false},
		{"native", RuntimeNative, false},
		{"virtual", RuntimeVirtual, false},
		{"container", RuntimeContainer, false},
		{"invalid", "", true},
		{"NATIVE", "", true},
		{"Native", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRuntimeMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRuntimeMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseRuntimeMode(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), "native, virtual, container") {
					t.Errorf("error message should list valid modes, got: %v", err)
				}
			}
		})
	}
}

func TestEnvInheritMode_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode    EnvInheritMode
		want    bool
		wantErr bool
	}{
		{EnvInheritNone, true, false},
		{EnvInheritAllow, true, false},
		{EnvInheritAll, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"NONE", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.mode.IsValid()
			if isValid != tt.want {
				t.Errorf("EnvInheritMode(%q).IsValid() = %v, want %v", tt.mode, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("EnvInheritMode(%q).IsValid() returned no errors, want error", tt.mode)
				}
				if !errors.Is(errs[0], ErrInvalidEnvInheritMode) {
					t.Errorf("error should wrap ErrInvalidEnvInheritMode, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("EnvInheritMode(%q).IsValid() returned unexpected errors: %v", tt.mode, errs)
			}
		})
	}
}

func TestFlagType_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ft      FlagType
		want    bool
		wantErr bool
	}{
		{FlagTypeString, true, false},
		{FlagTypeBool, true, false},
		{FlagTypeInt, true, false},
		{FlagTypeFloat, true, false},
		{"", true, false}, // zero value is valid (treated as "string" by GetType)
		{"invalid", false, true},
		{"STRING", false, true},
		{"number", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.ft), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.ft.IsValid()
			if isValid != tt.want {
				t.Errorf("FlagType(%q).IsValid() = %v, want %v", tt.ft, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("FlagType(%q).IsValid() returned no errors, want error", tt.ft)
				}
				if !errors.Is(errs[0], ErrInvalidFlagType) {
					t.Errorf("error should wrap ErrInvalidFlagType, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("FlagType(%q).IsValid() returned unexpected errors: %v", tt.ft, errs)
			}
		})
	}
}

func TestArgumentType_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		at      ArgumentType
		want    bool
		wantErr bool
	}{
		{ArgumentTypeString, true, false},
		{ArgumentTypeInt, true, false},
		{ArgumentTypeFloat, true, false},
		{"", true, false},     // zero value is valid (treated as "string" by GetType)
		{"bool", false, true}, // bool is valid for FlagType but NOT for ArgumentType
		{"invalid", false, true},
		{"STRING", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.at), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.at.IsValid()
			if isValid != tt.want {
				t.Errorf("ArgumentType(%q).IsValid() = %v, want %v", tt.at, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ArgumentType(%q).IsValid() returned no errors, want error", tt.at)
				}
				if !errors.Is(errs[0], ErrInvalidArgumentType) {
					t.Errorf("error should wrap ErrInvalidArgumentType, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ArgumentType(%q).IsValid() returned unexpected errors: %v", tt.at, errs)
			}
		})
	}
}

func TestParseEnvInheritMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    EnvInheritMode
		wantErr bool
	}{
		{"", "", false},
		{"none", EnvInheritNone, false},
		{"allow", EnvInheritAllow, false},
		{"all", EnvInheritAll, false},
		{"invalid", "", true},
		{"NONE", "", true},
		{"None", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := ParseEnvInheritMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseEnvInheritMode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseEnvInheritMode(%q) = %v, want %v", tt.input, got, tt.want)
			}
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), "none, allow, all") {
					t.Errorf("error message should list valid modes, got: %v", err)
				}
			}
		})
	}
}
