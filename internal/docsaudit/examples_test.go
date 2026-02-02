// SPDX-License-Identifier: MPL-2.0

package docsaudit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractAndValidateExamples(t *testing.T) {
	root := t.TempDir()
	mdPath := filepath.Join(root, "README.md")
	mdContent := strings.Join([]string{
		"```bash",
		"invowk internal docs audit",
		"```",
	}, "\n")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	cuePath := filepath.Join(root, "example.cue")
	if err := os.WriteFile(cuePath, []byte("foo: \"bar\"\n"), 0o644); err != nil {
		t.Fatalf("write cue: %v", err)
	}

	catalog := &SourceCatalog{
		Files: []string{mdPath, cuePath},
		FileToSource: map[string]DocumentationSource{
			mdPath:  {ID: "README.md", Location: mdPath},
			cuePath: {ID: "example.cue", Location: cuePath},
		},
	}

	examples, err := ExtractExamples(context.Background(), catalog)
	if err != nil {
		t.Fatalf("ExtractExamples: %v", err)
	}
	if len(examples) != 2 {
		t.Fatalf("expected 2 examples, got %d", len(examples))
	}

	surfaces := []UserFacingSurface{{Type: SurfaceTypeCommand, Name: "invowk internal docs audit"}}
	validated, err := ValidateExamples(context.Background(), examples, surfaces)
	if err != nil {
		t.Fatalf("ValidateExamples: %v", err)
	}

	for _, example := range validated {
		if example.Status != ExampleStatusValid {
			t.Fatalf("expected example to be valid: %s", example.SourceLocation)
		}
	}
}

func TestValidateExamplesUnknownCommand(t *testing.T) {
	root := t.TempDir()
	mdPath := filepath.Join(root, "README.md")
	mdContent := strings.Join([]string{
		"```bash",
		"invowk ghost",
		"```",
	}, "\n")
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	catalog := &SourceCatalog{Files: []string{mdPath}}
	examples, err := ExtractExamples(context.Background(), catalog)
	if err != nil {
		t.Fatalf("ExtractExamples: %v", err)
	}

	validated, err := ValidateExamples(context.Background(), examples, nil)
	if err != nil {
		t.Fatalf("ValidateExamples: %v", err)
	}
	if len(validated) != 1 {
		t.Fatalf("expected 1 example, got %d", len(validated))
	}
	if validated[0].Status != ExampleStatusInvalid {
		t.Fatalf("expected example to be invalid")
	}
}
