// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateModuleRef(t *testing.T) {
	t.Parallel()

	resolver, err := NewResolver(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	tests := []struct {
		name    string
		req     ModuleRef
		wantErr bool
		wantMsg string
	}{
		{
			name: "https URL",
			req: ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
			},
		},
		{
			name: "git@ URL",
			req: ModuleRef{
				GitURL:  "git@github.com:user/repo.git",
				Version: "^1.0.0",
			},
		},
		{
			name: "ssh URL",
			req: ModuleRef{
				GitURL:  "ssh://git@github.com/user/repo.git",
				Version: "^1.0.0",
			},
		},
		{
			name: "invalid scheme",
			req: ModuleRef{
				GitURL:  "http://github.com/user/repo.git",
				Version: "^1.0.0",
			},
			wantErr: true,
			wantMsg: "git_url must start with https://, git@, or ssh://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := resolver.validateModuleRef(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("validateModuleRef() expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantMsg) {
					t.Fatalf("validateModuleRef() error = %q, want containing %q", err.Error(), tt.wantMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("validateModuleRef() unexpected error: %v", err)
			}
		})
	}
}

func TestExtractRequiresFromInvowkmod(t *testing.T) {
	t.Parallel()

	reqs := []ModuleRequirement{
		{
			GitURL:  "https://github.com/org/tools.invowkmod.git",
			Version: "^1.2.0",
			Alias:   "tools",
			Path:    "modules/tools",
		},
		{
			GitURL:  "ssh://git@github.com/org/dep.invowkmod.git",
			Version: "~2.0.0",
		},
	}

	got := extractRequiresFromInvowkmod(reqs)
	if len(got) != len(reqs) {
		t.Fatalf("extractRequiresFromInvowkmod() returned %d entries, want %d", len(got), len(reqs))
	}

	for i := range reqs {
		if got[i].GitURL != reqs[i].GitURL {
			t.Errorf("entry[%d].GitURL = %q, want %q", i, got[i].GitURL, reqs[i].GitURL)
		}
		if got[i].Version != reqs[i].Version {
			t.Errorf("entry[%d].Version = %q, want %q", i, got[i].Version, reqs[i].Version)
		}
		if got[i].Alias != reqs[i].Alias {
			t.Errorf("entry[%d].Alias = %q, want %q", i, got[i].Alias, reqs[i].Alias)
		}
		if got[i].Path != reqs[i].Path {
			t.Errorf("entry[%d].Path = %q, want %q", i, got[i].Path, reqs[i].Path)
		}
	}
}

func TestLoadTransitiveDeps(t *testing.T) {
	t.Parallel()

	resolver, err := NewResolver(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	cachePath := t.TempDir()
	content := `module: "dep"
version: "1.0.0"
requires: [
	{
		git_url: "https://github.com/example/nested.invowkmod.git"
		version: "^2.0.0"
		alias: "nested"
		path: "modules/nested"
	},
]`
	if writeErr := os.WriteFile(filepath.Join(cachePath, "invowkmod.cue"), []byte(content), 0o644); writeErr != nil {
		t.Fatalf("failed to write invowkmod.cue: %v", writeErr)
	}

	reqs, moduleID, err := resolver.loadTransitiveDeps(cachePath)
	if err != nil {
		t.Fatalf("loadTransitiveDeps() error = %v", err)
	}
	if moduleID != "dep" {
		t.Fatalf("moduleID = %q, want %q", moduleID, "dep")
	}
	if len(reqs) != 1 {
		t.Fatalf("loadTransitiveDeps() returned %d requirements, want 1", len(reqs))
	}
	if reqs[0].GitURL != "https://github.com/example/nested.invowkmod.git" {
		t.Errorf("reqs[0].GitURL = %q", reqs[0].GitURL)
	}
	if reqs[0].Version != "^2.0.0" {
		t.Errorf("reqs[0].Version = %q", reqs[0].Version)
	}
	if reqs[0].Alias != "nested" {
		t.Errorf("reqs[0].Alias = %q", reqs[0].Alias)
	}
	if reqs[0].Path != "modules/nested" {
		t.Errorf("reqs[0].Path = %q", reqs[0].Path)
	}
}
