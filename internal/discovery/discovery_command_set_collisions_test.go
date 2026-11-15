// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
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
	addCommandInfos(set, []*CommandInfo{
		{Name: "hello", SimpleName: "hello", SourceID: "invowkfile", Source: SourceCurrentDir},
		{Name: "deploy", SimpleName: "deploy", SourceID: "invowkfile", Source: SourceCurrentDir},
		{
			Name: "foo build", SimpleName: "build", SourceID: "foo",
			ModuleID: moduleIDPtr("io.invowk.foo"), Source: SourceModule,
		},
		{
			Name: "foo deploy", SimpleName: "deploy", SourceID: "foo",
			ModuleID: moduleIDPtr("io.invowk.foo"), Source: SourceModule,
		},
		{
			Name: "bar test", SimpleName: "test", SourceID: "bar",
			ModuleID: moduleIDPtr("io.invowk.bar"), Source: SourceModule,
		},
	})

	set.Analyze()

	requireCommandCount(t, set, 5)
	requireIndexedCommandCounts(t, "BySource", set.BySource, map[SourceID]int{
		"invowkfile": 2,
		"foo":        2,
		"bar":        1,
	})
	requireSourceOrder(t, set, []SourceID{"invowkfile", "bar", "foo"})
	requireAmbiguityState(t, set, map[invowkfile.CommandName]bool{
		"deploy": true,
		"hello":  false,
		"build":  false,
		"test":   false,
	})
}

func TestDiscoveredCommandSet_HierarchicalAmbiguity(t *testing.T) {
	t.Parallel()

	// Test T035: Hierarchical command ambiguity detection (User Story 4)
	// Tests that subcommands like "deploy staging" are tracked separately from "deploy"
	// and ambiguity is detected at the correct hierarchical level
	set := NewDiscoveredCommandSet()
	addCommandInfos(set, []*CommandInfo{
		{Name: "deploy", SimpleName: "deploy", SourceID: "invowkfile", Source: SourceCurrentDir},
		{Name: "deploy staging", SimpleName: "deploy staging", SourceID: "invowkfile", Source: SourceCurrentDir},
		{Name: "deploy local", SimpleName: "deploy local", SourceID: "invowkfile", Source: SourceCurrentDir},
		{
			Name: "foo deploy", SimpleName: "deploy", SourceID: "foo",
			ModuleID: moduleIDPtr("io.invowk.foo"), Source: SourceModule,
		},
		{
			Name: "bar deploy staging", SimpleName: "deploy staging", SourceID: "bar",
			ModuleID: moduleIDPtr("io.invowk.bar"), Source: SourceModule,
		},
		{
			Name: "bar deploy production", SimpleName: "deploy production", SourceID: "bar",
			ModuleID: moduleIDPtr("io.invowk.bar"), Source: SourceModule,
		},
	})

	set.Analyze()

	requireAmbiguityState(t, set, map[invowkfile.CommandName]bool{
		"deploy":            true,
		"deploy staging":    true,
		"deploy local":      false,
		"deploy production": false,
	})
	requireIndexedCommandCounts(t, "BySimpleName", set.BySimpleName, map[invowkfile.CommandName]int{
		"deploy":            2,
		"deploy staging":    2,
		"deploy local":      1,
		"deploy production": 1,
	})
}

func addCommandInfos(set *DiscoveredCommandSet, commands []*CommandInfo) {
	for _, command := range commands {
		set.Add(command)
	}
}

func requireCommandCount(t *testing.T, set *DiscoveredCommandSet, want int) {
	t.Helper()

	if len(set.Commands) != want {
		t.Errorf("len(Commands) = %d, want %d", len(set.Commands), want)
	}
}

func requireIndexedCommandCounts[K comparable](
	t *testing.T,
	indexName string,
	index map[K][]*CommandInfo,
	want map[K]int,
) {
	t.Helper()

	for key, wantCount := range want {
		if got := len(index[key]); got != wantCount {
			t.Errorf("len(%s[%v]) = %d, want %d", indexName, key, got, wantCount)
		}
	}
}

func requireSourceOrder(t *testing.T, set *DiscoveredCommandSet, want []SourceID) {
	t.Helper()

	if !slices.Equal(set.SourceOrder, want) {
		t.Errorf("SourceOrder = %v, want %v", set.SourceOrder, want)
	}
}

func requireAmbiguityState(
	t *testing.T,
	set *DiscoveredCommandSet,
	want map[invowkfile.CommandName]bool,
) {
	t.Helper()

	for name, wantAmbiguous := range want {
		if got := set.AmbiguousNames[name]; got != wantAmbiguous {
			t.Errorf("AmbiguousNames[%q] = %t, want %t", name, got, wantAmbiguous)
		}
	}
	for _, command := range set.Commands {
		wantAmbiguous, ok := want[command.SimpleName]
		if !ok {
			t.Fatalf("missing ambiguity expectation for command %q", command.SimpleName)
		}
		if command.IsAmbiguous != wantAmbiguous {
			t.Errorf("Command %q from %s IsAmbiguous = %t, want %t", command.SimpleName, command.SourceID, command.IsAmbiguous, wantAmbiguous)
		}
	}
}

func TestDiscoveredCommandSet_Analyze_SortsCommandsWithinSource(t *testing.T) {
	t.Parallel()

	set := NewDiscoveredCommandSet()
	for _, c := range []struct {
		name   invowkfile.CommandName
		source SourceID
	}{
		{"zeta", "invowkfile"},
		{"foo mid", "foo"},
		{"alpha", "invowkfile"},
		{"mid sub", "invowkfile"},
		{"foo alpha", "foo"},
		{"mid", "invowkfile"},
	} {
		set.Add(&CommandInfo{Name: c.name, SimpleName: c.name, SourceID: c.source})
	}

	set.Analyze()

	want := map[SourceID][]invowkfile.CommandName{
		"invowkfile": {"alpha", "mid", "mid sub", "zeta"},
		"foo":        {"foo alpha", "foo mid"},
	}
	for source, wantNames := range want {
		var got []invowkfile.CommandName
		for _, cmd := range set.BySource[source] {
			got = append(got, cmd.Name)
		}
		if !slices.Equal(got, wantNames) {
			t.Errorf("BySource[%s] = %v, want %v", source, got, wantNames)
		}
	}
	if !slices.Equal(set.SourceOrder, []SourceID{"invowkfile", "foo"}) {
		t.Errorf("SourceOrder = %v, want [invowkfile foo]", set.SourceOrder)
	}
}

// TestDiscoverCommandSet_ListsSourceCommandsAlphabetically discovers an
// invowkfile whose commands are declared out of order. FlattenCommands walks a
// map, so repeated discoveries would expose any order that is not sorted.
func TestDiscoverCommandSet_ListsSourceCommandsAlphabetically(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	impl := `implementations: [{script: {content: "echo test"}, runtimes: [{name: "virtual-sh"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]`
	var content strings.Builder
	content.WriteString("cmds: [\n")
	for _, name := range []string{"zeta", "mid", "alpha", "kilo", "mid sub", "bravo", "yankee"} {
		fmt.Fprintf(&content, "\t{name: %q, %s},\n", name, impl)
	}
	content.WriteString("]\n")
	if err := os.WriteFile(filepath.Join(baseDir, "invowkfile.cue"), []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write invowkfile.cue: %v", err)
	}

	want := []invowkfile.CommandName{"alpha", "bravo", "kilo", "mid", "mid sub", "yankee", "zeta"}
	for range 5 {
		d := New(config.DefaultConfig(), WithBaseDir(types.FilesystemPath(baseDir)), WithCommandsDir(""))
		result, err := d.DiscoverCommandSet(t.Context())
		if err != nil {
			t.Fatalf("DiscoverCommandSet() error = %v", err)
		}
		var got []invowkfile.CommandName
		for _, cmd := range result.Set.BySource[SourceIDInvowkfile] {
			got = append(got, cmd.Name)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("BySource[invowkfile] = %v, want %v", got, want)
		}
	}
}
