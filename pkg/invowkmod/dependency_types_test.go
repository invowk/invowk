// SPDX-License-Identifier: MPL-2.0

package invowkmod

import "testing"

func TestModuleRef_MatchesSourceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ref      ModuleRef
		sourceID string
		want     bool
	}{
		{
			name:     "alias wins",
			ref:      ModuleRef{GitURL: "https://github.com/example/tools.git", Alias: "ci-tools"},
			sourceID: "ci-tools",
			want:     true,
		},
		{
			name:     "monorepo path basename matches",
			ref:      ModuleRef{GitURL: "https://github.com/example/mono.git", Path: "modules/go-tools"},
			sourceID: "go-tools",
			want:     true,
		},
		{
			name:     "git repository basename matches",
			ref:      ModuleRef{GitURL: "git@github.com:example/build-tools.git"},
			sourceID: "build-tools",
			want:     true,
		},
		{
			name:     "module suffix stripped from git repository basename",
			ref:      ModuleRef{GitURL: "https://github.com/example/io.example.tools.invowkmod.git"},
			sourceID: "io.example.tools",
			want:     true,
		},
		{
			name:     "module suffix stripped from local path basename",
			ref:      ModuleRef{Path: "/tmp/modules/io.example.local.invowkmod"},
			sourceID: "io.example.local",
			want:     true,
		},
		{
			name:     "nonmatching source rejected",
			ref:      ModuleRef{GitURL: "https://github.com/example/tools.git"},
			sourceID: "other",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.ref.MatchesSourceID(ModuleSourceID(tt.sourceID)); got != tt.want {
				t.Fatalf("MatchesSourceID(%q) = %v, want %v", tt.sourceID, got, tt.want)
			}
		})
	}
}
