// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestLoadResultValidateMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("rejects invalid config payload", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultConfig()
		cfg.ContainerEngine = "bad-engine"
		err := (LoadResult{Config: cfg}).Validate()
		if !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("LoadResult.Validate() error = %v, want ErrInvalidConfig", err)
		}
		if !errors.Is(err, ErrInvalidContainerEngine) {
			t.Fatalf("LoadResult.Validate() error = %v, want ErrInvalidContainerEngine", err)
		}
	})

	t.Run("rejects invalid source path payload", func(t *testing.T) {
		t.Parallel()

		err := (LoadResult{
			Config:     DefaultConfig(),
			SourcePath: types.FilesystemPath(" \t "),
		}).Validate()
		if !errors.Is(err, types.ErrInvalidFilesystemPath) {
			t.Fatalf("LoadResult.Validate() error = %v, want ErrInvalidFilesystemPath", err)
		}
	})
}

func TestProviderLoadWithSourcePropagatesLoadErrorMutation(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "missing.cue")
	_, err := NewProvider().LoadWithSource(t.Context(), LoadOptions{
		ConfigFilePath: types.FilesystemPath(missingPath),
	})
	if !errors.Is(err, ErrConfigFileNotFound) {
		t.Fatalf("LoadWithSource() error = %v, want ErrConfigFileNotFound", err)
	}
}
