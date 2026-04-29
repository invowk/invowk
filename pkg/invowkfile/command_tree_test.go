// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestValidateCommandTreeRejectsArgsOnParentCommand(t *testing.T) {
	t.Parallel()

	err := ValidateCommandTree([]CommandTreeEntry{
		{
			Name: "build",
			Command: &Command{
				Name: "build",
				Args: []Argument{{Name: "target"}},
			},
			FilePath: "invowkfile.cue",
		},
		{
			Name:    "build release",
			Command: &Command{Name: "build release"},
		},
	})
	if err == nil {
		t.Fatal("ValidateCommandTree() error = nil, want conflict")
	}
	var conflict *ArgsSubcommandConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("errors.As(%T) = false", &conflict)
	}
	if conflict.CommandName != "build" {
		t.Fatalf("CommandName = %q, want build", conflict.CommandName)
	}
}
