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

// mkExecDep builds a CommandInfo with execute: true deps on the given dependency names.
// Reduces boilerplate in ValidateExecutionDAG tests.
func mkExecDep(name string, deps ...string) *CommandInfo {
	if len(deps) == 0 {
		return &CommandInfo{Name: name, Command: &invowkfile.Command{}}
	}
	var cmds []invowkfile.CommandDependency
	for _, dep := range deps {
		cmds = append(cmds, invowkfile.CommandDependency{Alternatives: []string{dep}, Execute: true})
	}
	return &CommandInfo{
		Name: name,
		Command: &invowkfile.Command{
			DependsOn: &invowkfile.DependsOn{Commands: cmds},
		},
	}
}

func TestValidateCommandTree_NoConflict(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// =============================================================================
// ValidateExecutionDAG tests
// =============================================================================

func TestValidateExecutionDAG_AcyclicGraph(t *testing.T) {
	t.Parallel()

	// A depends on B (execute:true), B has no deps → valid DAG.
	commands := []*CommandInfo{mkExecDep("build", "lint"), mkExecDep("lint")}

	if err := ValidateExecutionDAG(commands); err != nil {
		t.Errorf("ValidateExecutionDAG() returned error for valid DAG: %v", err)
	}
}

func TestValidateExecutionDAG_SimpleCycle(t *testing.T) {
	t.Parallel()

	// A depends on B (execute:true), B depends on A (execute:true) → cycle.
	commands := []*CommandInfo{mkExecDep("a", "b"), mkExecDep("b", "a")}

	err := ValidateExecutionDAG(commands)
	if err == nil {
		t.Fatal("ValidateExecutionDAG() should have returned error for cyclic graph")
	}

	// Verify the error wraps a CycleError from the dag package.
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestValidateExecutionDAG_TransitiveCycle(t *testing.T) {
	t.Parallel()

	// A -> B -> C -> A (all execute:true) → transitive cycle.
	commands := []*CommandInfo{mkExecDep("a", "b"), mkExecDep("b", "c"), mkExecDep("c", "a")}

	err := ValidateExecutionDAG(commands)
	if err == nil {
		t.Fatal("ValidateExecutionDAG() should detect transitive cycle")
	}
}

func TestValidateExecutionDAG_NonExecuteDepsIgnored(t *testing.T) {
	t.Parallel()

	// A depends on B with execute:false → should NOT form a graph edge.
	// B depends on A with execute:false → also no edge. No cycle.
	// mkExecDep cannot be used here because we need execute: false.
	commands := []*CommandInfo{
		{
			Name: "a",
			Command: &invowkfile.Command{
				DependsOn: &invowkfile.DependsOn{
					Commands: []invowkfile.CommandDependency{
						{Alternatives: []string{"b"}, Execute: false},
					},
				},
			},
		},
		{
			Name: "b",
			Command: &invowkfile.Command{
				DependsOn: &invowkfile.DependsOn{
					Commands: []invowkfile.CommandDependency{
						{Alternatives: []string{"a"}, Execute: false},
					},
				},
			},
		},
	}

	if err := ValidateExecutionDAG(commands); err != nil {
		t.Errorf("ValidateExecutionDAG() should ignore non-execute deps, got: %v", err)
	}
}

func TestValidateExecutionDAG_RequiredArgsBlocked(t *testing.T) {
	t.Parallel()

	// A has execute dep on B, but B has required args → validation error.
	cmdB := mkExecDep("b")
	cmdB.Command.Args = []invowkfile.Argument{{Name: "target", Required: true}}
	commands := []*CommandInfo{mkExecDep("a", "b"), cmdB}

	err := ValidateExecutionDAG(commands)
	if err == nil {
		t.Fatal("ValidateExecutionDAG() should reject execute dep on command with required args")
	}
	var reqErr *RequiredInputsError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected *RequiredInputsError, got %T: %v", err, err)
	}
	if reqErr.ParentName != "a" || reqErr.TargetName != "b" {
		t.Errorf("wrong parent/target: got %s/%s", reqErr.ParentName, reqErr.TargetName)
	}
	if len(reqErr.RequiredArgs) != 1 || reqErr.RequiredArgs[0] != "target" {
		t.Errorf("expected RequiredArgs=[target], got %v", reqErr.RequiredArgs)
	}
	if !strings.Contains(err.Error(), "required argument") {
		t.Errorf("error should mention required argument, got: %v", err)
	}
}

func TestValidateExecutionDAG_RequiredFlagsBlocked(t *testing.T) {
	t.Parallel()

	// A has execute dep on B, but B has required flags → validation error.
	cmdB := mkExecDep("b")
	cmdB.Command.Flags = []invowkfile.Flag{{Name: "output", Required: true}}
	commands := []*CommandInfo{mkExecDep("a", "b"), cmdB}

	err := ValidateExecutionDAG(commands)
	if err == nil {
		t.Fatal("ValidateExecutionDAG() should reject execute dep on command with required flags")
	}
	var reqErr *RequiredInputsError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected *RequiredInputsError, got %T: %v", err, err)
	}
	if reqErr.ParentName != "a" || reqErr.TargetName != "b" {
		t.Errorf("wrong parent/target: got %s/%s", reqErr.ParentName, reqErr.TargetName)
	}
	if len(reqErr.RequiredFlags) != 1 || reqErr.RequiredFlags[0] != "output" {
		t.Errorf("expected RequiredFlags=[output], got %v", reqErr.RequiredFlags)
	}
	if !strings.Contains(err.Error(), "required flag") {
		t.Errorf("error should mention required flag, got: %v", err)
	}
}

func TestValidateExecutionDAG_ImplementationLevelDeps(t *testing.T) {
	t.Parallel()

	// Execute dep defined at implementation level should also be validated.
	// A's implementation has execute dep on B, B's implementation has execute dep on A → cycle.
	commands := []*CommandInfo{
		{
			Name: "a",
			Command: &invowkfile.Command{
				Implementations: []invowkfile.Implementation{
					{
						DependsOn: &invowkfile.DependsOn{
							Commands: []invowkfile.CommandDependency{
								{Alternatives: []string{"b"}, Execute: true},
							},
						},
					},
				},
			},
		},
		{
			Name: "b",
			Command: &invowkfile.Command{
				Implementations: []invowkfile.Implementation{
					{
						DependsOn: &invowkfile.DependsOn{
							Commands: []invowkfile.CommandDependency{
								{Alternatives: []string{"a"}, Execute: true},
							},
						},
					},
				},
			},
		},
	}

	err := ValidateExecutionDAG(commands)
	if err == nil {
		t.Fatal("ValidateExecutionDAG() should detect cycle from implementation-level deps")
	}
}

func TestValidateExecutionDAG_RootLevelDeps(t *testing.T) {
	t.Parallel()

	// Root-level execute dep creates a cycle: root depends on B, B depends on A.
	// Since root deps apply to all commands, A inherits root's dep on B.
	inv := &invowkfile.Invowkfile{
		DependsOn: &invowkfile.DependsOn{
			Commands: []invowkfile.CommandDependency{
				{Alternatives: []string{"b"}, Execute: true},
			},
		},
	}

	commands := []*CommandInfo{
		{
			Name:       "a",
			Command:    &invowkfile.Command{},
			Invowkfile: inv,
		},
		{
			Name: "b",
			Command: &invowkfile.Command{
				DependsOn: &invowkfile.DependsOn{
					Commands: []invowkfile.CommandDependency{
						{Alternatives: []string{"a"}, Execute: true},
					},
				},
			},
			Invowkfile: inv,
		},
	}

	err := ValidateExecutionDAG(commands)
	if err == nil {
		t.Fatal("ValidateExecutionDAG() should detect cycle involving root-level deps")
	}
}

func TestValidateExecutionDAG_EmptyAndNil(t *testing.T) {
	t.Parallel()

	if err := ValidateExecutionDAG(nil); err != nil {
		t.Errorf("ValidateExecutionDAG(nil) should return nil, got: %v", err)
	}
	if err := ValidateExecutionDAG([]*CommandInfo{}); err != nil {
		t.Errorf("ValidateExecutionDAG([]) should return nil, got: %v", err)
	}

	// Nil entries should be skipped gracefully.
	commands := []*CommandInfo{nil, {Name: "ok", Command: &invowkfile.Command{}}}
	if err := ValidateExecutionDAG(commands); err != nil {
		t.Errorf("ValidateExecutionDAG() should handle nil entries, got: %v", err)
	}
}

func TestValidateExecutionDAG_AllAlternativesChecked(t *testing.T) {
	t.Parallel()

	// dep has alternatives ["b", "c"]. c depends on a → cycle via second alternative.
	commands := []*CommandInfo{
		{
			Name: "a",
			Command: &invowkfile.Command{
				DependsOn: &invowkfile.DependsOn{
					Commands: []invowkfile.CommandDependency{
						{Alternatives: []string{"b", "c"}, Execute: true},
					},
				},
			},
		},
		{
			Name:    "b",
			Command: &invowkfile.Command{},
		},
		{
			Name: "c",
			Command: &invowkfile.Command{
				DependsOn: &invowkfile.DependsOn{
					Commands: []invowkfile.CommandDependency{
						{Alternatives: []string{"a"}, Execute: true},
					},
				},
			},
		},
	}

	err := ValidateExecutionDAG(commands)
	if err == nil {
		t.Fatal("ValidateExecutionDAG() should check ALL alternatives for cycles, not just the first")
	}
}

func TestValidateExecutionDAG_SelfReference(t *testing.T) {
	t.Parallel()

	// A command that lists itself as an execute dependency → self-loop cycle.
	commands := []*CommandInfo{mkExecDep("self", "self")}

	err := ValidateExecutionDAG(commands)
	if err == nil {
		t.Fatal("ValidateExecutionDAG() should detect self-referencing dependency")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error should mention cycle, got: %v", err)
	}
}

func TestValidateExecutionDAG_OptionalArgsAllowed(t *testing.T) {
	t.Parallel()

	// Execute dep targets a command with optional args and flags — should be allowed.
	// Only required args/flags should be rejected.
	child := mkExecDep("child")
	child.Command.Args = []invowkfile.Argument{{Name: "target", Description: "optional target"}}
	child.Command.Flags = []invowkfile.Flag{{Name: "verbose", Description: "enable verbose output"}}
	commands := []*CommandInfo{mkExecDep("parent", "child"), child}

	if err := ValidateExecutionDAG(commands); err != nil {
		t.Errorf("ValidateExecutionDAG() should allow optional args/flags, got: %v", err)
	}
}

func TestArgsSubcommandConflictError_Error_SingleArg(t *testing.T) {
	t.Parallel()

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
