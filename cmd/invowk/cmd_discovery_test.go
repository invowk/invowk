// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestGroupByCategory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cmds           []*discovery.CommandInfo
		wantGroups     int
		wantFirstEmpty bool     // first group has empty category
		wantCategories []string // non-empty category names in order
	}{
		{
			name:           "all uncategorized",
			cmds:           makeCmds("a", "", "b", "", "c", ""),
			wantGroups:     1,
			wantFirstEmpty: true,
			wantCategories: nil,
		},
		{
			name:           "single category",
			cmds:           makeCmds("a", "build", "b", "build"),
			wantGroups:     1,
			wantFirstEmpty: false,
			wantCategories: []string{"build"},
		},
		{
			name:           "uncategorized first then alphabetical",
			cmds:           makeCmds("x", "", "a", "deploy", "b", "build"),
			wantGroups:     3,
			wantFirstEmpty: true,
			wantCategories: []string{"build", "deploy"},
		},
		{
			name:           "categories alphabetically sorted",
			cmds:           makeCmds("a", "z-cat", "b", "a-cat", "c", "m-cat"),
			wantGroups:     3,
			wantFirstEmpty: false,
			wantCategories: []string{"a-cat", "m-cat", "z-cat"},
		},
		{
			name:       "empty input",
			cmds:       nil,
			wantGroups: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			groups := groupByCategory(tt.cmds)

			if len(groups) != tt.wantGroups {
				t.Fatalf("got %d groups, want %d", len(groups), tt.wantGroups)
			}
			if tt.wantGroups == 0 {
				return
			}

			if tt.wantFirstEmpty && groups[0].category != "" {
				t.Errorf("expected first group to be uncategorized, got %q", groups[0].category)
			}

			// Collect non-empty categories.
			var cats []string
			for _, g := range groups {
				if g.category != "" {
					cats = append(cats, string(g.category))
				}
			}
			if len(cats) != len(tt.wantCategories) {
				t.Fatalf("got categories %v, want %v", cats, tt.wantCategories)
			}
			for i, cat := range cats {
				if cat != tt.wantCategories[i] {
					t.Errorf("category[%d] = %q, want %q", i, cat, tt.wantCategories[i])
				}
			}
		})
	}
}

// makeCmds creates a slice of CommandInfo from alternating (name, category) pairs.
func makeCmds(pairs ...string) []*discovery.CommandInfo {
	var result []*discovery.CommandInfo
	for i := 0; i < len(pairs); i += 2 {
		result = append(result, &discovery.CommandInfo{
			Name:       invowkfile.CommandName(pairs[i]),
			SimpleName: invowkfile.CommandName(pairs[i]),
			Command: &invowkfile.Command{
				Name:     invowkfile.CommandName(pairs[i]),
				Category: invowkfile.CommandCategory(pairs[i+1]),
			},
		})
	}
	return result
}
