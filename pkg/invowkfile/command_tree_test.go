// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
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
	if len(conflict.Args) != 1 || conflict.Args[0].Name != "target" {
		t.Fatalf("Args = %#v, want target argument", conflict.Args)
	}
	if len(conflict.Subcommands) != 1 || conflict.Subcommands[0] != "build release" {
		t.Fatalf("Subcommands = %#v, want build release", conflict.Subcommands)
	}
	if conflict.FilePath != "invowkfile.cue" {
		t.Fatalf("FilePath = %q, want invowkfile.cue", conflict.FilePath)
	}
	if got := conflict.Error(); !strings.Contains(got, "in invowkfile.cue") {
		t.Fatalf("Error() = %q, want file path", got)
	}
}

func TestValidateCommandTreeAllowsParentCommandWithoutArgs(t *testing.T) {
	t.Parallel()

	err := ValidateCommandTree([]CommandTreeEntry{
		{
			Name:    "deploy",
			Command: &Command{Name: "deploy"},
		},
		{
			Name:    "deploy prod",
			Command: &Command{Name: "deploy prod"},
		},
	})
	if err != nil {
		t.Fatalf("ValidateCommandTree() = %v, want nil", err)
	}
}

func TestValidateCommandTreeAllowsArgsOnNestedLeafCommand(t *testing.T) {
	t.Parallel()

	err := ValidateCommandTree([]CommandTreeEntry{
		{
			Name: "deploy prod",
			Command: &Command{
				Name: "deploy prod",
				Args: []Argument{{Name: "target"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("ValidateCommandTree() = %v, want nil", err)
	}
}

func TestValidateCommandTreeDoesNotInventRootParent(t *testing.T) {
	t.Parallel()

	// ValidateCommandTree only checks tree invariants. Field validity is covered
	// by CommandTreeEntry.Validate, so an invalid empty command name must not be
	// treated as a synthetic parent for every top-level command.
	err := ValidateCommandTree([]CommandTreeEntry{
		{
			Name: "",
			Command: &Command{
				Name: "",
				Args: []Argument{{Name: "target"}},
			},
		},
		{
			Name:    "deploy",
			Command: &Command{Name: "deploy"},
		},
	})
	if err != nil {
		t.Fatalf("ValidateCommandTree() = %v, want nil", err)
	}
}
