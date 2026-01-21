// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"invowk-cli/pkg/invkfile"
	"strings"
	"testing"
)

func TestValidateCommandTree_NoConflict(t *testing.T) {
	// Leaf commands with args are valid
	commands := []*CommandInfo{
		{
			Name: "deploy",
			Command: &invkfile.Command{
				Args: []invkfile.Argument{
					{Name: "env", Description: "Environment to deploy to", Required: true},
				},
			},
			FilePath: "/test/invkfile.cue",
		},
		{
			Name: "build",
			Command: &invkfile.Command{
				Args: []invkfile.Argument{
					{Name: "target", Description: "Build target"},
				},
			},
			FilePath: "/test/invkfile.cue",
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
			Command:  &invkfile.Command{},
			FilePath: "/test/invkfile.cue",
		},
		{
			Name: "deploy staging",
			Command: &invkfile.Command{
				Args: []invkfile.Argument{
					{Name: "version", Description: "Version to deploy"},
				},
			},
			FilePath: "/test/invkfile.cue",
		},
		{
			Name: "deploy production",
			Command: &invkfile.Command{
				Args: []invkfile.Argument{
					{Name: "version", Description: "Version to deploy"},
				},
			},
			FilePath: "/test/invkfile.cue",
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
			Command: &invkfile.Command{
				Args: []invkfile.Argument{
					{Name: "env", Description: "Environment", Required: true},
				},
			},
			FilePath: "/test/invkfile.cue",
		},
		{
			Name:     "deploy staging",
			Command:  &invkfile.Command{},
			FilePath: "/test/invkfile.cue",
		},
	}

	err := ValidateCommandTree(commands)
	if err == nil {
		t.Fatal("ValidateCommandTree() should have returned an error for conflicting commands")
	}

	// Check that it's the right error type
	conflictErr, ok := err.(*ArgsSubcommandConflictError)
	if !ok {
		t.Fatalf("Expected ArgsSubcommandConflictError, got %T", err)
	}

	if conflictErr.CommandName != "deploy" {
		t.Errorf("Expected CommandName 'deploy', got %q", conflictErr.CommandName)
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
			Command: &invkfile.Command{
				Args: []invkfile.Argument{
					{Name: "env", Description: "Environment"},
				},
			},
			FilePath: "/test/invkfile.cue",
		},
		{
			Name:     "deploy staging",
			Command:  &invkfile.Command{},
			FilePath: "/test/invkfile.cue",
		},
		{
			Name:     "deploy production",
			Command:  &invkfile.Command{},
			FilePath: "/test/invkfile.cue",
		},
	}

	err := ValidateCommandTree(commands)
	if err == nil {
		t.Fatal("ValidateCommandTree() should have returned an error")
	}

	conflictErr, ok := err.(*ArgsSubcommandConflictError)
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
			Command: &invkfile.Command{
				Args: []invkfile.Argument{
					{Name: "connection", Description: "DB connection string"},
				},
			},
			FilePath: "/test/invkfile.cue",
		},
		{
			Name:     "db migrate",
			Command:  &invkfile.Command{},
			FilePath: "/test/invkfile.cue",
		},
		{
			Name:     "db migrate up",
			Command:  &invkfile.Command{},
			FilePath: "/test/invkfile.cue",
		},
		{
			Name:     "db migrate down",
			Command:  &invkfile.Command{},
			FilePath: "/test/invkfile.cue",
		},
	}

	err := ValidateCommandTree(commands)
	if err == nil {
		t.Fatal("ValidateCommandTree() should have returned an error for deep nested conflict")
	}

	conflictErr, ok := err.(*ArgsSubcommandConflictError)
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
			Command:  &invkfile.Command{},
			FilePath: "/test/invkfile.cue",
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
		Args: []invkfile.Argument{
			{Name: "env"},
			{Name: "version"},
		},
		Subcommands: []string{"deploy staging", "deploy production"},
		FilePath:    "/test/invkfile.cue",
	}

	errStr := err.Error()

	// Check that all parts are present
	if !strings.Contains(errStr, "command 'deploy' has both args and subcommands") {
		t.Errorf("Error message missing header, got: %s", errStr)
	}

	if !strings.Contains(errStr, "in /test/invkfile.cue") {
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
		Args: []invkfile.Argument{
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
		Args: []invkfile.Argument{
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
