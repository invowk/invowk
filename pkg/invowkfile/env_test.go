// SPDX-License-Identifier: MPL-2.0

package invowkfile

import "testing"

func TestValidateEnvVarName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "HOME", false},
		{"valid with underscore", "MY_VAR", false},
		{"valid leading underscore", "_PRIVATE", false},
		{"valid alphanumeric", "VAR123", false},
		{"valid single char", "X", false},
		{"invalid empty", "", true},
		{"invalid whitespace only", "   ", true},
		{"invalid starts with number", "1VAR", true},
		{"invalid hyphen", "MY-VAR", true},
		{"invalid dot", "MY.VAR", true},
		{"invalid space in name", "MY VAR", true},
		{"invalid special char", "MY$VAR", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateEnvVarName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEnvVarName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
