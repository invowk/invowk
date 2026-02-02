// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchDocumentation(t *testing.T) {
	root := t.TempDir()
	docPath := filepath.Join(root, "README.md")
	content := strings.Join([]string{
		"invowk internal docs audit --out docs-audit.md",
		"invowk ghost",
	}, "\n")
	if err := os.WriteFile(docPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	catalog := &SourceCatalog{
		Sources: []DocumentationSource{{
			ID:       "README.md",
			Kind:     DocKindReadme,
			Location: docPath,
		}},
		Files: []string{docPath},
		FileToSource: map[string]DocumentationSource{
			docPath: {
				ID:       "README.md",
				Kind:     DocKindReadme,
				Location: docPath,
			},
		},
	}

	surfaces := []UserFacingSurface{
		{
			ID:   "cli:command:invowk internal docs audit",
			Type: SurfaceTypeCommand,
			Name: "invowk internal docs audit",
		},
		{
			ID:   "cli:flag:invowk internal docs audit:out",
			Type: SurfaceTypeFlag,
			Name: "invowk internal docs audit --out",
		},
	}

	updated, findings, err := MatchDocumentation(context.Background(), catalog, surfaces)
	if err != nil {
		t.Fatalf("MatchDocumentation: %v", err)
	}
	if len(updated) != len(surfaces) {
		t.Fatalf("expected %d surfaces, got %d", len(surfaces), len(updated))
	}
	if len(updated[0].DocumentationRefs) == 0 {
		t.Fatalf("expected documentation refs for command surface")
	}
	if len(updated[1].DocumentationRefs) == 0 {
		t.Fatalf("expected documentation refs for flag surface")
	}

	if len(findings) == 0 {
		t.Fatalf("expected docs-only findings")
	}
	found := false
	for _, finding := range findings {
		if strings.Contains(finding.Summary, "ghost") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finding for docs-only command")
	}
}
