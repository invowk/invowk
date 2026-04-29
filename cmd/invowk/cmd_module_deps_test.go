// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestModuleRefsFromRequirements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		meta     *invowkmod.Invowkmod
		wantLen  int
		wantRefs []invowkmod.ModuleRef
	}{
		{
			name:    "nil metadata returns empty slice",
			meta:    nil,
			wantLen: 0,
		},
		{
			name: "empty requires returns empty slice",
			meta: &invowkmod.Invowkmod{
				Module:  "test",
				Version: "1.0.0",
			},
			wantLen: 0,
		},
		{
			name: "nil requires field returns empty slice",
			meta: &invowkmod.Invowkmod{
				Module:   "test",
				Version:  "1.0.0",
				Requires: nil,
			},
			wantLen: 0,
		},
		{
			name: "single requirement without alias",
			meta: &invowkmod.Invowkmod{
				Module:  "test",
				Version: "1.0.0",
				Requires: []invowkmod.ModuleRequirement{
					{
						GitURL:  "https://github.com/example/tools.invowkmod.git",
						Version: "^1.0.0",
					},
				},
			},
			wantLen: 1,
			wantRefs: []invowkmod.ModuleRef{
				{
					GitURL:  "https://github.com/example/tools.invowkmod.git",
					Version: "^1.0.0",
				},
			},
		},
		{
			name: "multiple requirements with alias and path",
			meta: &invowkmod.Invowkmod{
				Module:  "myapp",
				Version: "2.0.0",
				Requires: []invowkmod.ModuleRequirement{
					{
						GitURL:  "https://github.com/org/utils.invowkmod.git",
						Version: "^1.2.0",
						Alias:   "myutils",
					},
					{
						GitURL:  "https://github.com/org/monorepo.git",
						Version: "~2.0.0",
						Path:    "modules/helpers",
					},
				},
			},
			wantLen: 2,
			wantRefs: []invowkmod.ModuleRef{
				{
					GitURL:  "https://github.com/org/utils.invowkmod.git",
					Version: "^1.2.0",
					Alias:   "myutils",
				},
				{
					GitURL:  "https://github.com/org/monorepo.git",
					Version: "~2.0.0",
					Path:    "modules/helpers",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got []invowkmod.ModuleRef
			if tt.meta != nil {
				got = invowkmod.ModuleRefsFromRequirements(tt.meta.Requires)
			}

			if len(got) != tt.wantLen {
				t.Fatalf("ModuleRefsFromRequirements() returned %d refs, want %d", len(got), tt.wantLen)
			}

			for i, want := range tt.wantRefs {
				if got[i].GitURL != want.GitURL {
					t.Errorf("ref[%d].GitURL = %q, want %q", i, got[i].GitURL, want.GitURL)
				}
				if got[i].Version != want.Version {
					t.Errorf("ref[%d].Version = %q, want %q", i, got[i].Version, want.Version)
				}
				if got[i].Alias != want.Alias {
					t.Errorf("ref[%d].Alias = %q, want %q", i, got[i].Alias, want.Alias)
				}
				if got[i].Path != want.Path {
					t.Errorf("ref[%d].Path = %q, want %q", i, got[i].Path, want.Path)
				}
			}
		})
	}
}
