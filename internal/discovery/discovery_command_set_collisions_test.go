// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"testing"
)

// moduleIDPtr is a test helper that creates a *invowkmod.ModuleID from a string.
func TestDiscoveredCommandSet_Add(t *testing.T) {
	t.Parallel()

	// Test T006: Add method
	set := NewDiscoveredCommandSet()

	cmd1 := &CommandInfo{
		Name:       "hello",
		SimpleName: "hello",
		SourceID:   "invowkfile",
	}
	cmd2 := &CommandInfo{
		Name:       "build",
		SimpleName: "build",
		SourceID:   "foo",
	}
	cmd3 := &CommandInfo{
		Name:       "deploy",
		SimpleName: "deploy",
		SourceID:   "foo",
	}

	set.Add(cmd1)
	set.Add(cmd2)
	set.Add(cmd3)

	// Check Commands slice
	if len(set.Commands) != 3 {
		t.Errorf("len(Commands) = %d, want 3", len(set.Commands))
	}

	// Check BySimpleName index
	if len(set.BySimpleName["hello"]) != 1 {
		t.Errorf("len(BySimpleName[hello]) = %d, want 1", len(set.BySimpleName["hello"]))
	}
	if len(set.BySimpleName["build"]) != 1 {
		t.Errorf("len(BySimpleName[build]) = %d, want 1", len(set.BySimpleName["build"]))
	}

	// Check BySource index
	if len(set.BySource["invowkfile"]) != 1 {
		t.Errorf("len(BySource[invowkfile]) = %d, want 1", len(set.BySource["invowkfile"]))
	}
	if len(set.BySource["foo"]) != 2 {
		t.Errorf("len(BySource[foo]) = %d, want 2", len(set.BySource["foo"]))
	}

	// Check SourceOrder
	if len(set.SourceOrder) != 2 {
		t.Errorf("len(SourceOrder) = %d, want 2", len(set.SourceOrder))
	}
}

func TestDiscoveredCommandSet_Analyze_NoConflicts(t *testing.T) {
	t.Parallel()

	// Test T007: Analyze method with no conflicts
	set := NewDiscoveredCommandSet()

	set.Add(&CommandInfo{SimpleName: "hello", SourceID: "invowkfile"})
	set.Add(&CommandInfo{SimpleName: "build", SourceID: "foo"})
	set.Add(&CommandInfo{SimpleName: "test", SourceID: "bar"})

	set.Analyze()

	// No ambiguous names
	if len(set.AmbiguousNames) != 0 {
		t.Errorf("len(AmbiguousNames) = %d, want 0", len(set.AmbiguousNames))
	}

	// All commands should not be ambiguous
	for _, cmd := range set.Commands {
		if cmd.IsAmbiguous {
			t.Errorf("Command %s should not be ambiguous", cmd.SimpleName)
		}
	}

	// SourceOrder should be sorted: invowkfile first, then alphabetically
	if set.SourceOrder[0] != "invowkfile" {
		t.Errorf("SourceOrder[0] = %s, want invowkfile", set.SourceOrder[0])
	}
	if set.SourceOrder[1] != "bar" {
		t.Errorf("SourceOrder[1] = %s, want bar", set.SourceOrder[1])
	}
	if set.SourceOrder[2] != "foo" {
		t.Errorf("SourceOrder[2] = %s, want foo", set.SourceOrder[2])
	}
}

func TestDiscoveredCommandSet_Analyze_WithConflicts(t *testing.T) {
	t.Parallel()

	// Test T007: Analyze method with conflicts
	set := NewDiscoveredCommandSet()

	// "deploy" exists in both invowkfile and foo
	set.Add(&CommandInfo{SimpleName: "hello", SourceID: "invowkfile"})
	set.Add(&CommandInfo{SimpleName: "deploy", SourceID: "invowkfile"})
	set.Add(&CommandInfo{SimpleName: "deploy", SourceID: "foo"})
	set.Add(&CommandInfo{SimpleName: "build", SourceID: "foo"})

	set.Analyze()

	// "deploy" should be ambiguous
	if !set.AmbiguousNames["deploy"] {
		t.Error("'deploy' should be marked as ambiguous")
	}

	// "hello" and "build" should not be ambiguous
	if set.AmbiguousNames["hello"] {
		t.Error("'hello' should not be marked as ambiguous")
	}
	if set.AmbiguousNames["build"] {
		t.Error("'build' should not be marked as ambiguous")
	}

	// Check IsAmbiguous flag on commands
	for _, cmd := range set.Commands {
		if cmd.SimpleName == "deploy" {
			if !cmd.IsAmbiguous {
				t.Errorf("Command 'deploy' from %s should be marked as ambiguous", cmd.SourceID)
			}
		} else {
			if cmd.IsAmbiguous {
				t.Errorf("Command '%s' should not be marked as ambiguous", cmd.SimpleName)
			}
		}
	}
}

func TestDiscoveredCommandSet_Analyze_SameNameSameSource(t *testing.T) {
	t.Parallel()

	// Test: Multiple commands with same name from same source are NOT ambiguous
	// (This could happen with command overloading or errors)
	set := NewDiscoveredCommandSet()

	set.Add(&CommandInfo{SimpleName: "build", SourceID: "foo"})
	set.Add(&CommandInfo{SimpleName: "build", SourceID: "foo"}) // Same name, same source

	set.Analyze()

	// Should NOT be marked as ambiguous since they're from the same source
	if set.AmbiguousNames["build"] {
		t.Error("Commands with same name from same source should not be marked as ambiguous")
	}
}

func TestDiscoveredCommandSet_MultiSourceAggregation(t *testing.T) {
	t.Parallel()

	// Test T011: Multi-source aggregation for User Story 1
	// Simulates commands from invowkfile + two modules

	set := NewDiscoveredCommandSet()

	// Commands from invowkfile
	set.Add(&CommandInfo{
		Name:       "hello",
		SimpleName: "hello",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})
	set.Add(&CommandInfo{
		Name:       "deploy",
		SimpleName: "deploy",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})

	// Commands from foo module
	set.Add(&CommandInfo{
		Name:       "foo build",
		SimpleName: "build",
		SourceID:   "foo",
		ModuleID:   moduleIDPtr("io.invowk.foo"),
		Source:     SourceModule,
	})
	set.Add(&CommandInfo{
		Name:       "foo deploy",
		SimpleName: "deploy",
		SourceID:   "foo",
		ModuleID:   moduleIDPtr("io.invowk.foo"),
		Source:     SourceModule,
	})

	// Commands from bar module
	set.Add(&CommandInfo{
		Name:       "bar test",
		SimpleName: "test",
		SourceID:   "bar",
		ModuleID:   moduleIDPtr("io.invowk.bar"),
		Source:     SourceModule,
	})

	set.Analyze()

	// Test aggregation counts
	if len(set.Commands) != 5 {
		t.Errorf("len(Commands) = %d, want 5", len(set.Commands))
	}

	// Test source grouping
	if len(set.BySource["invowkfile"]) != 2 {
		t.Errorf("len(BySource[invowkfile]) = %d, want 2", len(set.BySource["invowkfile"]))
	}
	if len(set.BySource["foo"]) != 2 {
		t.Errorf("len(BySource[foo]) = %d, want 2", len(set.BySource["foo"]))
	}
	if len(set.BySource["bar"]) != 1 {
		t.Errorf("len(BySource[bar]) = %d, want 1", len(set.BySource["bar"]))
	}

	// Test source order (invowkfile first, then alphabetically)
	if set.SourceOrder[0] != "invowkfile" {
		t.Errorf("SourceOrder[0] = %s, want invowkfile", set.SourceOrder[0])
	}
	if set.SourceOrder[1] != "bar" {
		t.Errorf("SourceOrder[1] = %s, want bar", set.SourceOrder[1])
	}
	if set.SourceOrder[2] != "foo" {
		t.Errorf("SourceOrder[2] = %s, want foo", set.SourceOrder[2])
	}

	// Test ambiguity detection - "deploy" is in both invowkfile and foo
	if !set.AmbiguousNames["deploy"] {
		t.Error("'deploy' should be marked as ambiguous")
	}
	// "hello", "build", "test" are unique
	if set.AmbiguousNames["hello"] {
		t.Error("'hello' should not be marked as ambiguous")
	}
	if set.AmbiguousNames["build"] {
		t.Error("'build' should not be marked as ambiguous")
	}
	if set.AmbiguousNames["test"] {
		t.Error("'test' should not be marked as ambiguous")
	}

	// Test IsAmbiguous flag on individual commands
	for _, cmd := range set.Commands {
		if cmd.SimpleName == "deploy" {
			if !cmd.IsAmbiguous {
				t.Errorf("Command 'deploy' from %s should be marked as ambiguous", cmd.SourceID)
			}
		} else {
			if cmd.IsAmbiguous {
				t.Errorf("Command '%s' from %s should not be marked as ambiguous", cmd.SimpleName, cmd.SourceID)
			}
		}
	}
}

func TestDiscoveredCommandSet_HierarchicalAmbiguity(t *testing.T) {
	t.Parallel()

	// Test T035: Hierarchical command ambiguity detection (User Story 4)
	// Tests that subcommands like "deploy staging" are tracked separately from "deploy"
	// and ambiguity is detected at the correct hierarchical level

	set := NewDiscoveredCommandSet()

	// Commands from invowkfile - parent "deploy" and subcommand "deploy staging"
	set.Add(&CommandInfo{
		Name:       "deploy",
		SimpleName: "deploy",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})
	set.Add(&CommandInfo{
		Name:       "deploy staging",
		SimpleName: "deploy staging",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})
	set.Add(&CommandInfo{
		Name:       "deploy local",
		SimpleName: "deploy local",
		SourceID:   "invowkfile",
		Source:     SourceCurrentDir,
	})

	// Commands from foo module - conflicting parent "deploy"
	set.Add(&CommandInfo{
		Name:       "foo deploy",
		SimpleName: "deploy",
		SourceID:   "foo",
		ModuleID:   moduleIDPtr("io.invowk.foo"),
		Source:     SourceModule,
	})

	// Commands from bar module - conflicting "deploy staging" but unique "deploy production"
	set.Add(&CommandInfo{
		Name:       "bar deploy staging",
		SimpleName: "deploy staging",
		SourceID:   "bar",
		ModuleID:   moduleIDPtr("io.invowk.bar"),
		Source:     SourceModule,
	})
	set.Add(&CommandInfo{
		Name:       "bar deploy production",
		SimpleName: "deploy production",
		SourceID:   "bar",
		ModuleID:   moduleIDPtr("io.invowk.bar"),
		Source:     SourceModule,
	})

	set.Analyze()

	// Test that "deploy" is ambiguous (invowkfile vs foo)
	if !set.AmbiguousNames["deploy"] {
		t.Error("'deploy' should be marked as ambiguous (exists in invowkfile and foo)")
	}

	// Test that "deploy staging" is ambiguous (invowkfile vs bar)
	if !set.AmbiguousNames["deploy staging"] {
		t.Error("'deploy staging' should be marked as ambiguous (exists in invowkfile and bar)")
	}

	// Test that "deploy local" is NOT ambiguous (only in invowkfile)
	if set.AmbiguousNames["deploy local"] {
		t.Error("'deploy local' should NOT be marked as ambiguous (only in invowkfile)")
	}

	// Test that "deploy production" is NOT ambiguous (only in bar)
	if set.AmbiguousNames["deploy production"] {
		t.Error("'deploy production' should NOT be marked as ambiguous (only in bar)")
	}

	// Verify IsAmbiguous flag on individual commands
	for _, cmd := range set.Commands {
		switch cmd.SimpleName {
		case "deploy":
			if !cmd.IsAmbiguous {
				t.Errorf("Command 'deploy' from %s should be marked as ambiguous", cmd.SourceID)
			}
		case "deploy staging":
			if !cmd.IsAmbiguous {
				t.Errorf("Command 'deploy staging' from %s should be marked as ambiguous", cmd.SourceID)
			}
		case "deploy local", "deploy production":
			if cmd.IsAmbiguous {
				t.Errorf("Command '%s' from %s should NOT be marked as ambiguous", cmd.SimpleName, cmd.SourceID)
			}
		}
	}

	// Verify correct command counts per SimpleName
	if len(set.BySimpleName["deploy"]) != 2 {
		t.Errorf("Expected 2 commands with SimpleName 'deploy', got %d", len(set.BySimpleName["deploy"]))
	}
	if len(set.BySimpleName["deploy staging"]) != 2 {
		t.Errorf("Expected 2 commands with SimpleName 'deploy staging', got %d", len(set.BySimpleName["deploy staging"]))
	}
	if len(set.BySimpleName["deploy local"]) != 1 {
		t.Errorf("Expected 1 command with SimpleName 'deploy local', got %d", len(set.BySimpleName["deploy local"]))
	}
	if len(set.BySimpleName["deploy production"]) != 1 {
		t.Errorf("Expected 1 command with SimpleName 'deploy production', got %d", len(set.BySimpleName["deploy production"]))
	}
}
