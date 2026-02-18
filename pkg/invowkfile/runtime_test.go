// SPDX-License-Identifier: MPL-2.0

package invowkfile

import "testing"

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
		})
	}
}
