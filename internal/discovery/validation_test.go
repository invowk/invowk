// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	// testCmdDeploy is a test constant for the command name "deploy"
	testCmdDeploy = "deploy"
)

func TestValidateCommandTree_NoConflict(t *testing.T) {
	// Leaf commands with args are valid
	commands := []*CommandInfo{
		{
			Name: "deploy",
			Command: &invowkfile.Command{
				Args: []invowkfile.Argument{
					{Name: "env", Description: "Environment to deploy to", Required: true},
				},
			},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name: "build",
			Command: &invowkfile.Command{
				Args: []invowkfile.Argument{
					{Name: "target", Description: "Build target"},
				},
			},
			FilePath: "/test/invowkfile.cue",
		},
	}

	err := ValidateCommandTree(commands)
	if err != nil {
		t.Errorf("ValidateCommandTree() returned error for valid commands: %v", err)
	}
}

func TestValidateCommandTree_NoConflict_NestedLeaves(t *testing.T) {
	// Parent without args, leaf children with args is valid
	commands := []*CommandInfo{
		{
			Name:     "deploy",
			Command:  &invowkfile.Command{},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name: "deploy staging",
			Command: &invowkfile.Command{
				Args: []invowkfile.Argument{
					{Name: "version", Description: "Version to deploy"},
				},
			},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name: "deploy production",
			Command: &invowkfile.Command{
				Args: []invowkfile.Argument{
					{Name: "version", Description: "Version to deploy"},
				},
			},
			FilePath: "/test/invowkfile.cue",
		},
	}

	err := ValidateCommandTree(commands)
	if err != nil {
		t.Errorf("ValidateCommandTree() returned error for valid nested commands: %v", err)
	}
}

func TestValidateCommandTree_Conflict(t *testing.T) {
	// Parent with args that also has children is a conflict
	commands := []*CommandInfo{
		{
			Name: "deploy",
			Command: &invowkfile.Command{
				Args: []invowkfile.Argument{
					{Name: "env", Description: "Environment", Required: true},
				},
			},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name:     "deploy staging",
			Command:  &invowkfile.Command{},
			FilePath: "/test/invowkfile.cue",
		},
	}

	err := ValidateCommandTree(commands)
	if err == nil {
		t.Fatal("ValidateCommandTree() should have returned an error for conflicting commands")
	}

	// Check that it's the right error type
	conflictErr, ok := errors.AsType[*ArgsSubcommandConflictError](err)
	if !ok {
		t.Fatalf("Expected ArgsSubcommandConflictError, got %T", err)
	}

	if conflictErr.CommandName != testCmdDeploy {
		t.Errorf("Expected CommandName %q, got %q", testCmdDeploy, conflictErr.CommandName)
	}

	if len(conflictErr.Args) != 1 || conflictErr.Args[0].Name != "env" {
		t.Errorf("Expected Args to contain 'env', got %v", conflictErr.Args)
	}

	if len(conflictErr.Subcommands) != 1 || conflictErr.Subcommands[0] != "deploy staging" {
		t.Errorf("Expected Subcommands to contain 'deploy staging', got %v", conflictErr.Subcommands)
	}
}

func TestValidateCommandTree_Conflict_MultipleChildren(t *testing.T) {
	// Parent with args that has multiple children
	commands := []*CommandInfo{
		{
			Name: "deploy",
			Command: &invowkfile.Command{
				Args: []invowkfile.Argument{
					{Name: "env", Description: "Environment"},
				},
			},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name:     "deploy staging",
			Command:  &invowkfile.Command{},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name:     "deploy production",
			Command:  &invowkfile.Command{},
			FilePath: "/test/invowkfile.cue",
		},
	}

	err := ValidateCommandTree(commands)
	if err == nil {
		t.Fatal("ValidateCommandTree() should have returned an error")
	}

	conflictErr, ok := errors.AsType[*ArgsSubcommandConflictError](err)
	if !ok {
		t.Fatalf("Expected ArgsSubcommandConflictError, got %T", err)
	}

	if len(conflictErr.Subcommands) != 2 {
		t.Errorf("Expected 2 subcommands, got %d", len(conflictErr.Subcommands))
	}
}

func TestValidateCommandTree_DeepNesting(t *testing.T) {
	// Deep nesting: grandparent with args has grandchildren via child
	commands := []*CommandInfo{
		{
			Name: "db",
			Command: &invowkfile.Command{
				Args: []invowkfile.Argument{
					{Name: "connection", Description: "DB connection string"},
				},
			},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name:     "db migrate",
			Command:  &invowkfile.Command{},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name:     "db migrate up",
			Command:  &invowkfile.Command{},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name:     "db migrate down",
			Command:  &invowkfile.Command{},
			FilePath: "/test/invowkfile.cue",
		},
	}

	err := ValidateCommandTree(commands)
	if err == nil {
		t.Fatal("ValidateCommandTree() should have returned an error for deep nested conflict")
	}

	conflictErr, ok := errors.AsType[*ArgsSubcommandConflictError](err)
	if !ok {
		t.Fatalf("Expected ArgsSubcommandConflictError, got %T", err)
	}

	if conflictErr.CommandName != "db" {
		t.Errorf("Expected CommandName 'db', got %q", conflictErr.CommandName)
	}
}

func TestValidateCommandTree_EmptyCommands(t *testing.T) {
	// Empty input returns nil
	err := ValidateCommandTree(nil)
	if err != nil {
		t.Errorf("ValidateCommandTree(nil) should return nil, got %v", err)
	}

	err = ValidateCommandTree([]*CommandInfo{})
	if err != nil {
		t.Errorf("ValidateCommandTree([]) should return nil, got %v", err)
	}
}

func TestValidateCommandTree_NilCommands(t *testing.T) {
	// Handles nil entries gracefully
	commands := []*CommandInfo{
		nil,
		{
			Name:     "test",
			Command:  &invowkfile.Command{},
			FilePath: "/test/invowkfile.cue",
		},
		{
			Name:    "test2",
			Command: nil, // nil Command field
		},
	}

	err := ValidateCommandTree(commands)
	if err != nil {
		t.Errorf("ValidateCommandTree() should handle nil entries gracefully, got %v", err)
	}
}

func TestArgsSubcommandConflictError_Error(t *testing.T) {
	err := &ArgsSubcommandConflictError{
		CommandName: "deploy",
		Args: []invowkfile.Argument{
			{Name: "env"},
			{Name: "version"},
		},
		Subcommands: []string{"deploy staging", "deploy production"},
		FilePath:    "/test/invowkfile.cue",
	}

	errStr := err.Error()

	// Check that all parts are present
	if !strings.Contains(errStr, "command 'deploy' has both args and subcommands") {
		t.Errorf("Error message missing header, got: %s", errStr)
	}

	if !strings.Contains(errStr, "in /test/invowkfile.cue") {
		t.Errorf("Error message missing file path, got: %s", errStr)
	}

	if !strings.Contains(errStr, "defined args: env, version") {
		t.Errorf("Error message missing args, got: %s", errStr)
	}

	if !strings.Contains(errStr, "subcommands: deploy staging, deploy production") {
		t.Errorf("Error message missing subcommands, got: %s", errStr)
	}
}

func TestArgsSubcommandConflictError_Error_NoFilePath(t *testing.T) {
	err := &ArgsSubcommandConflictError{
		CommandName: "deploy",
		Args: []invowkfile.Argument{
			{Name: "env"},
		},
		Subcommands: []string{"deploy staging"},
		FilePath:    "", // No file path
	}

	errStr := err.Error()

	// Should not contain "in " prefix for empty file path
	if strings.Contains(errStr, " in ") {
		t.Errorf("Error message should not contain file path prefix when empty, got: %s", errStr)
	}
}

func TestArgsSubcommandConflictError_Error_SingleArg(t *testing.T) {
	err := &ArgsSubcommandConflictError{
		CommandName: "deploy",
		Args: []invowkfile.Argument{
			{Name: "env"},
		},
		Subcommands: []string{"deploy staging"},
	}

	errStr := err.Error()

	// Single arg should not have comma
	if !strings.Contains(errStr, "defined args: env") {
		t.Errorf("Error message should show single arg without comma, got: %s", errStr)
	}
}
