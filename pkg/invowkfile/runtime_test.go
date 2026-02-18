// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"strings"
	"testing"
)

func TestRuntimeMode_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode RuntimeMode
		want bool
	}{
		{RuntimeNative, true},
		{RuntimeVirtual, true},
		{RuntimeContainer, true},
		{"", false},
		{"invalid", false},
		{"NATIVE", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			t.Parallel()
			if got := tt.mode.IsValid(); got != tt.want {
				t.Errorf("RuntimeMode(%q).IsValid() = %v, want %v", tt.mode, got, tt.want)
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
		mode EnvInheritMode
		want bool
	}{
		{EnvInheritNone, true},
		{EnvInheritAllow, true},
		{EnvInheritAll, true},
		{"", false},
		{"invalid", false},
		{"NONE", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			t.Parallel()
			if got := tt.mode.IsValid(); got != tt.want {
				t.Errorf("EnvInheritMode(%q).IsValid() = %v, want %v", tt.mode, got, tt.want)
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
