// SPDX-License-Identifier: MPL-2.0

package cmd

import "testing"

func TestIsArgEnvVar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		want bool
	}{
		{"", false},
		{"ARG", false},
		{"ARG0", true},
		{"ARG1", true},
		{"ARG9", true},
		{"ARG10", true},
		{"ARG99", true},
		{"ARGC", true},
		{"ARGS", false},
		{"ARGNAME", false},
		{"ARG_1", false},
		{"ARG1NAME", false},
		{"MY_ARG1", false},
		{"arg1", false},
		{"INVOWK_ARG1", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			if got := isArgEnvVar(tt.key); got != tt.want {
				t.Errorf("isArgEnvVar(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
