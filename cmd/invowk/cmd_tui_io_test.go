// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"strings"
	"testing"
)

func TestRunTUIFormatUsesCommandInputAndOutput(t *testing.T) {
	t.Parallel()

	cmd := newTUIFormatCommand()
	cmd.SetIn(strings.NewReader("hello from stdin"))
	var output strings.Builder
	cmd.SetOut(&output)
	if err := cmd.Flags().Set("type", "template"); err != nil {
		t.Fatalf("Set(type): %v", err)
	}

	if err := runTuiFormat(cmd, nil); err != nil {
		t.Fatalf("runTuiFormat(): %v", err)
	}
	if output.String() != "hello from stdin\n" {
		t.Fatalf("stdout = %q, want %q", output.String(), "hello from stdin\n")
	}
}
