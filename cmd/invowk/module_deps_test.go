// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestExtractModuleRequirementsFromMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		meta     *invowkfile.Invowkmod
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
			meta: &invowkfile.Invowkmod{
				Module:  "test",
				Version: "1.0.0",
			},
			wantLen: 0,
		},
		{
			name: "nil requires field returns empty slice",
			meta: &invowkfile.Invowkmod{
				Module:   "test",
				Version:  "1.0.0",
				Requires: nil,
			},
			wantLen: 0,
		},
		{
			name: "single requirement without alias",
			meta: &invowkfile.Invowkmod{
				Module:  "test",
				Version: "1.0.0",
				Requires: []invowkfile.ModuleRequirement{
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
			meta: &invowkfile.Invowkmod{
				Module:  "myapp",
				Version: "2.0.0",
				Requires: []invowkfile.ModuleRequirement{
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

			got := extractModuleRequirementsFromMetadata(tt.meta)

			if len(got) != tt.wantLen {
				t.Fatalf("extractModuleRequirementsFromMetadata() returned %d refs, want %d", len(got), tt.wantLen)
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
