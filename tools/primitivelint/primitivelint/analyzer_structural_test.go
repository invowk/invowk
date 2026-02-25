// SPDX-License-Identifier: MPL-2.0

package primitivelint

import (
	"testing"
)

func TestCapitalizeFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"single lowercase", "x", "X"},
		{"single uppercase", "X", "X"},
		{"camelCase", "shellArgs", "ShellArgs"},
		{"already capitalized", "Shell", "Shell"},
		{"all lowercase", "timeout", "Timeout"},
		{"unicode", "über", "Über"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := capitalizeFirst(tt.input)
			if got != tt.want {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
