// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestRuntimeMode_Validate(t *testing.T) {
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
			err := tt.mode.Validate()
			if (err == nil) != tt.want {
				t.Errorf("RuntimeMode(%q).Validate() error = %v, want valid=%v", tt.mode, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RuntimeMode(%q).Validate() returned nil, want error", tt.mode)
				}
				if !errors.Is(err, ErrInvalidRuntimeMode) {
					t.Errorf("error should wrap ErrInvalidRuntimeMode, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("RuntimeMode(%q).Validate() returned unexpected error: %v", tt.mode, err)
			}
		})
	}
}

func TestPlatformType_Validate(t *testing.T) {
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
			err := tt.platform.Validate()
			if (err == nil) != tt.want {
				t.Errorf("PlatformType(%q).Validate() error = %v, want valid=%v", tt.platform, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PlatformType(%q).Validate() returned nil, want error", tt.platform)
				}
				if !errors.Is(err, ErrInvalidPlatform) {
					t.Errorf("error should wrap ErrInvalidPlatform, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("PlatformType(%q).Validate() returned unexpected error: %v", tt.platform, err)
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

func TestEnvInheritMode_Validate(t *testing.T) {
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
			err := tt.mode.Validate()
			if (err == nil) != tt.want {
				t.Errorf("EnvInheritMode(%q).Validate() error = %v, want valid=%v", tt.mode, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("EnvInheritMode(%q).Validate() returned nil, want error", tt.mode)
				}
				if !errors.Is(err, ErrInvalidEnvInheritMode) {
					t.Errorf("error should wrap ErrInvalidEnvInheritMode, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("EnvInheritMode(%q).Validate() returned unexpected error: %v", tt.mode, err)
			}
		})
	}
}

func TestFlagType_Validate(t *testing.T) {
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
			err := tt.ft.Validate()
			if (err == nil) != tt.want {
				t.Errorf("FlagType(%q).Validate() error = %v, want valid=%v", tt.ft, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("FlagType(%q).Validate() returned nil, want error", tt.ft)
				}
				if !errors.Is(err, ErrInvalidFlagType) {
					t.Errorf("error should wrap ErrInvalidFlagType, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("FlagType(%q).Validate() returned unexpected error: %v", tt.ft, err)
			}
		})
	}
}

func TestArgumentType_Validate(t *testing.T) {
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
			err := tt.at.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ArgumentType(%q).Validate() error = %v, want valid=%v", tt.at, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ArgumentType(%q).Validate() returned nil, want error", tt.at)
				}
				if !errors.Is(err, ErrInvalidArgumentType) {
					t.Errorf("error should wrap ErrInvalidArgumentType, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("ArgumentType(%q).Validate() returned unexpected error: %v", tt.at, err)
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

func TestContainerImage_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		img     ContainerImage
		want    bool
		wantErr bool
	}{
		{"valid image", "debian:stable-slim", true, false},
		{"valid with registry", "registry.example.com/image:tag", true, false},
		{"zero value (empty)", "", true, false},
		{"invalid whitespace only", "   ", false, true},
		{"invalid tab only", "\t", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.img.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ContainerImage(%q).Validate() error = %v, want valid=%v", tt.img, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ContainerImage(%q).Validate() returned nil, want error", tt.img)
				}
				if !errors.Is(err, ErrInvalidContainerImage) {
					t.Errorf("error should wrap ErrInvalidContainerImage, got: %v", err)
				}
				var typedErr *InvalidContainerImageError
				if !errors.As(err, &typedErr) {
					t.Errorf("error should be *InvalidContainerImageError, got: %T", err)
				} else if typedErr.Value != tt.img {
					t.Errorf("InvalidContainerImageError.Value = %q, want %q", typedErr.Value, tt.img)
				}
			} else if err != nil {
				t.Errorf("ContainerImage(%q).Validate() returned unexpected error: %v", tt.img, err)
			}
		})
	}
}

func TestContainerImage_String(t *testing.T) {
	t.Parallel()

	if got := ContainerImage("debian:stable-slim").String(); got != "debian:stable-slim" {
		t.Errorf("ContainerImage(\"debian:stable-slim\").String() = %q, want %q", got, "debian:stable-slim")
	}
}

func TestRuntimeMode_String(t *testing.T) {
	t.Parallel()

	if got := RuntimeNative.String(); got != "native" {
		t.Errorf("RuntimeNative.String() = %q, want %q", got, "native")
	}
	if got := RuntimeMode("").String(); got != "" {
		t.Errorf("RuntimeMode(\"\").String() = %q, want %q", got, "")
	}
}

func TestEnvInheritMode_String(t *testing.T) {
	t.Parallel()

	if got := EnvInheritAll.String(); got != "all" {
		t.Errorf("EnvInheritAll.String() = %q, want %q", got, "all")
	}
	if got := EnvInheritMode("").String(); got != "" {
		t.Errorf("EnvInheritMode(\"\").String() = %q, want %q", got, "")
	}
}

func TestPlatformType_String(t *testing.T) {
	t.Parallel()

	if got := PlatformLinux.String(); got != "linux" {
		t.Errorf("PlatformLinux.String() = %q, want %q", got, "linux")
	}
	if got := PlatformType("").String(); got != "" {
		t.Errorf("PlatformType(\"\").String() = %q, want %q", got, "")
	}
}

func TestFlagType_String(t *testing.T) {
	t.Parallel()

	if got := FlagTypeString.String(); got != "string" {
		t.Errorf("FlagTypeString.String() = %q, want %q", got, "string")
	}
	if got := FlagType("").String(); got != "" {
		t.Errorf("FlagType(\"\").String() = %q, want %q", got, "")
	}
}

func TestArgumentType_String(t *testing.T) {
	t.Parallel()

	if got := ArgumentTypeInt.String(); got != "int" {
		t.Errorf("ArgumentTypeInt.String() = %q, want %q", got, "int")
	}
	if got := ArgumentType("").String(); got != "" {
		t.Errorf("ArgumentType(\"\").String() = %q, want %q", got, "")
	}
}
