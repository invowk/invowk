// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"strings"
	"testing"
)

func TestValidateAgentCmdCreateModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dryRun    bool
		printOnly bool
		verify    bool
		wantErr   string
	}{
		{name: "write mode"},
		{name: "dry run", dryRun: true},
		{name: "print", printOnly: true},
		{name: "verify write", verify: true},
		{name: "dry run and print", dryRun: true, printOnly: true, wantErr: "--dry-run and --print"},
		{name: "verify and dry run", dryRun: true, verify: true, wantErr: "--verify requires writing"},
		{name: "verify and print", printOnly: true, verify: true, wantErr: "--verify requires writing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAgentCmdCreateModes(tt.dryRun, tt.printOnly, tt.verify)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateAgentCmdCreateModes() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateAgentCmdCreateModes() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
