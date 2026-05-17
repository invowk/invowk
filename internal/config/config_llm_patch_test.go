// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestLoadRejectsEmptyLLMAPIConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cue  string
	}{
		{
			name: "empty api",
			cue:  `llm: {api: {}}`,
		},
		{
			name: "provider plus empty api",
			cue:  `llm: {provider: "codex", api: {}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfgDir := t.TempDir()
			cfgPath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)
			if err := os.WriteFile(cfgPath, []byte(tt.cue), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, _, err := loadWithOptions(t.Context(), LoadOptions{ConfigDirPath: types.FilesystemPath(cfgDir)})
			if err == nil {
				t.Fatal("loadWithOptions() succeeded, want empty llm.api validation error")
			}
			if !errors.Is(err, ErrInvalidLLMConfig) {
				t.Fatalf("loadWithOptions() error = %v, want ErrInvalidLLMConfig", err)
			}
			if !errors.Is(err, ErrInvalidLLMAPIConfig) {
				t.Fatalf("loadWithOptions() error = %v, want ErrInvalidLLMAPIConfig", err)
			}
			var apiErr *InvalidLLMAPIConfigError
			if !errors.As(err, &apiErr) {
				t.Fatalf("loadWithOptions() error = %T, want *InvalidLLMAPIConfigError", err)
			}
			var fieldDetails []string
			for _, fieldErr := range apiErr.FieldErrors {
				fieldDetails = append(fieldDetails, fieldErr.Error())
			}
			if !strings.Contains(strings.Join(fieldDetails, "\n"), "llm.api must set at least one") {
				t.Fatalf("LLM API field errors = %v, want empty llm.api message", apiErr.FieldErrors)
			}
		})
	}
}
