// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// testContentHash is a valid ContentHash used in test fixtures.
	testContentHash = ContentHash("sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	// testContentHash2 is a second valid ContentHash for multi-module test fixtures.
	testContentHash2 = ContentHash("sha256:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2")
)

func TestNewLockFile(t *testing.T) {
	t.Parallel()

	lf := NewLockFile()

	if lf.Version != "2.0" {
		t.Errorf("Version = %q, want %q", lf.Version, "2.0")
	}
	if lf.Modules == nil {
		t.Fatal("Modules map is nil, want initialized")
	}
	if len(lf.Modules) != 0 {
		t.Errorf("Modules has %d entries, want 0", len(lf.Modules))
	}
	if time.Since(lf.Generated) > time.Minute {
		t.Errorf("Generated is too old: %v", lf.Generated)
	}
}

func TestLockFile_AddModule(t *testing.T) {
	t.Parallel()

	//nolint:thelper // Case runners are passed directly to t.Run and begin with t.Parallel.
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "add_to_empty", run: func(t *testing.T) {
			t.Parallel()

			lf := NewLockFile()
			resolved := &ResolvedModule{
				ModuleRef: ModuleRef{
					GitURL:  "https://github.com/user/repo.git",
					Version: "^1.0.0",
				},
				ResolvedVersion: "1.2.0",
				GitCommit:       "abc123def456789012345678901234567890abcd",
				ModuleID:        "io.example.repo",
				Namespace:       "repo@1.2.0",
				ContentHash:     testContentHash,
			}
			lf.AddModule(resolved)

			key := ModuleRefKey("https://github.com/user/repo.git")
			if !lf.HasModule(key) {
				t.Fatal("module not found after AddModule")
			}
			mod, ok := lf.GetModule(key)
			if !ok {
				t.Fatal("GetModule returned false")
			}
			if mod.GitURL != "https://github.com/user/repo.git" {
				t.Errorf("GitURL = %q", mod.GitURL)
			}
			if mod.ResolvedVersion != "1.2.0" {
				t.Errorf("ResolvedVersion = %q", mod.ResolvedVersion)
			}
			if mod.GitCommit != "abc123def456789012345678901234567890abcd" {
				t.Errorf("GitCommit = %q", mod.GitCommit)
			}
			if mod.ModuleID != "io.example.repo" {
				t.Errorf("ModuleID = %q, want %q", mod.ModuleID, "io.example.repo")
			}
		}},

		{name: "with_optional_fields", run: func(t *testing.T) {
			t.Parallel()

			lf := NewLockFile()
			resolved := &ResolvedModule{
				ModuleRef: ModuleRef{
					GitURL:  "https://github.com/org/monorepo.git",
					Version: "^2.0.0",
					Alias:   "tools",
					Path:    "modules/tools",
				},
				ResolvedVersion: "2.1.0",
				GitCommit:       "def456789012345678901234567890abcdef0123",
				ModuleID:        "io.example.tools",
				Namespace:       "tools",
				ContentHash:     testContentHash,
			}
			lf.AddModule(resolved)

			key := ModuleRefKey("https://github.com/org/monorepo.git#modules/tools")
			mod, ok := lf.GetModule(key)
			if !ok {
				t.Fatal("module with subpath not found")
			}
			if mod.Alias != "tools" {
				t.Errorf("Alias = %q, want %q", mod.Alias, "tools")
			}
			if mod.Path != "modules/tools" {
				t.Errorf("Path = %q, want %q", mod.Path, "modules/tools")
			}
			if mod.Namespace != "tools" {
				t.Errorf("Namespace = %q, want %q", mod.Namespace, "tools")
			}
		}},

		{name: "overwrite_duplicate", run: func(t *testing.T) {
			t.Parallel()

			lf := NewLockFile()
			ref := ModuleRef{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
			}
			lf.AddModule(&ResolvedModule{
				ModuleRef:       ref,
				ResolvedVersion: "1.0.0",
				GitCommit:       "0000000000000000000000000000000000000001",
				Namespace:       "repo@1.0.0",
				ContentHash:     testContentHash,
			})
			lf.AddModule(&ResolvedModule{
				ModuleRef:       ref,
				ResolvedVersion: "1.1.0",
				GitCommit:       "0000000000000000000000000000000000000002",
				Namespace:       "repo@1.1.0",
				ContentHash:     testContentHash2,
			})

			if len(lf.Modules) != 1 {
				t.Fatalf("expected 1 module after overwrite, got %d", len(lf.Modules))
			}
			key := ref.Key()
			mod, _ := lf.GetModule(key)
			if mod.ResolvedVersion != "1.1.0" {
				t.Errorf("ResolvedVersion = %q, want %q (should be overwritten)", mod.ResolvedVersion, "1.1.0")
			}
		}},
	}
	//nolint:paralleltest // Each table case runner begins with t.Parallel.
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestLockFile_HasModule(t *testing.T) {
	t.Parallel()

	lf := NewLockFile()
	key := ModuleRefKey("https://github.com/user/repo.git")

	if lf.HasModule(key) {
		t.Error("empty lock file should not have module")
	}

	lf.Modules[key] = LockedModule{
		GitURL:          "https://github.com/user/repo.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "abc123def456789012345678901234567890abcd",
		Namespace:       "repo@1.0.0",
		ContentHash:     testContentHash,
	}

	if !lf.HasModule(key) {
		t.Error("module should be found after adding")
	}
	if lf.HasModule(ModuleRefKey("nonexistent")) {
		t.Error("nonexistent key should not be found")
	}
}

func TestLockFile_GetModule(t *testing.T) {
	t.Parallel()

	lf := NewLockFile()
	key := ModuleRefKey("https://github.com/user/repo.git")
	lf.Modules[key] = LockedModule{
		GitURL:          "https://github.com/user/repo.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "abc123def456789012345678901234567890abcd",
		Namespace:       "repo@1.0.0",
		ContentHash:     testContentHash,
	}

	mod, ok := lf.GetModule(key)
	if !ok {
		t.Fatal("GetModule returned false for existing key")
	}
	if mod.GitURL != "https://github.com/user/repo.git" {
		t.Errorf("GitURL = %q", mod.GitURL)
	}

	_, ok = lf.GetModule(ModuleRefKey("nonexistent"))
	if ok {
		t.Error("GetModule returned true for nonexistent key")
	}
}

func TestLoadLockFile(t *testing.T) {
	t.Parallel()

	t.Run("nonexistent_returns_fresh", func(t *testing.T) {
		t.Parallel()

		lf, err := LoadLockFile(filepath.Join(t.TempDir(), "missing.cue"))
		if err != nil {
			t.Fatalf("LoadLockFile() error: %v", err)
		}
		if lf.Version != "2.0" {
			t.Errorf("Version = %q, want fresh lock file", lf.Version)
		}
		if len(lf.Modules) != 0 {
			t.Errorf("expected 0 modules, got %d", len(lf.Modules))
		}
	})

	t.Run("valid_round_trip", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, LockFileName)

		lf := &LockFile{
			Version:   "2.0",
			Generated: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			Modules: map[ModuleRefKey]LockedModule{
				"https://github.com/user/repo.git": {
					GitURL:          "https://github.com/user/repo.git",
					Version:         "^1.0.0",
					ResolvedVersion: "1.2.0",
					GitCommit:       "abc123def456789012345678901234567890abcd",
					Namespace:       "repo@1.2.0",
					CommandSourceID: "repo",
					ModuleID:        "repo",
					ContentHash:     testContentHash,
				},
			},
		}
		if err := lf.Save(path); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		loaded, err := LoadLockFile(path)
		if err != nil {
			t.Fatalf("LoadLockFile() error: %v", err)
		}
		if loaded.Version != "2.0" {
			t.Errorf("Version = %q", loaded.Version)
		}
		if len(loaded.Modules) != 1 {
			t.Fatalf("expected 1 module, got %d", len(loaded.Modules))
		}
		mod := loaded.Modules[ModuleRefKey("https://github.com/user/repo.git")]
		if mod.ResolvedVersion != "1.2.0" {
			t.Errorf("ResolvedVersion = %q", mod.ResolvedVersion)
		}
	})
}

func TestInspectLockFile(t *testing.T) {
	t.Parallel()

	//nolint:thelper // Case runners are passed directly to t.Run and begin with t.Parallel.
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "missing_preserves_absence", run: func(t *testing.T) {
			t.Parallel()

			snapshot := InspectLockFile(types.FilesystemPath(filepath.Join(t.TempDir(), LockFileName)))
			if snapshot.Present {
				t.Fatal("Present = true, want false")
			}
			if snapshot.LockFile != nil || snapshot.StatErr != nil || snapshot.ParseErr != nil {
				t.Fatalf("snapshot = %#v, want empty absence state", snapshot)
			}
		}},

		{name: "valid_preserves_size_and_lock", run: func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), LockFileName)
			lock := NewLockFile()
			if err := lock.Save(path); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			snapshot := InspectLockFile(types.FilesystemPath(path))
			if !snapshot.Present {
				t.Fatal("Present = false, want true")
			}
			if snapshot.Size == 0 {
				t.Fatal("Size = 0, want captured file size")
			}
			if snapshot.LockFile == nil {
				t.Fatal("LockFile = nil, want parsed lock file")
			}
			if snapshot.StatErr != nil || snapshot.ParseErr != nil {
				t.Fatalf("unexpected snapshot errors: stat=%v parse=%v", snapshot.StatErr, snapshot.ParseErr)
			}
		}},

		{name: "parse_error_preserves_presence", run: func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), LockFileName)
			if err := os.WriteFile(path, []byte("version: 2.0"), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			snapshot := InspectLockFile(types.FilesystemPath(path))
			if !snapshot.Present {
				t.Fatal("Present = false, want true")
			}
			if snapshot.ParseErr == nil {
				t.Fatal("ParseErr = nil, want parse error")
			}
			if snapshot.LockFile != nil {
				t.Fatal("LockFile != nil, want nil on parse error")
			}
		}},
	}
	//nolint:paralleltest // Each table case runner begins with t.Parallel.
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestLockFile_Save(t *testing.T) {
	t.Parallel()

	//nolint:thelper // Case runners are passed directly to t.Run and begin with t.Parallel.
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "creates_parent_dirs", run: func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "nested", "deep", LockFileName)

			lf := NewLockFile()
			if err := lf.Save(path); err != nil {
				t.Fatalf("Save() error: %v", err)
			}

			if _, err := os.Stat(path); err != nil {
				t.Fatalf("file not created: %v", err)
			}
		}},

		{name: "content_matches", run: func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, LockFileName)

			lf := &LockFile{
				Version:   "2.0",
				Generated: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
				Modules:   make(map[ModuleRefKey]LockedModule),
			}
			if err := lf.Save(path); err != nil {
				t.Fatalf("Save() error: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error: %v", err)
			}
			content := string(data)
			if !strings.Contains(content, `version: "2.0"`) {
				t.Error("saved file missing version field")
			}
			if !strings.Contains(content, "modules: {}") {
				t.Error("saved file missing empty modules block")
			}
		}},

		{name: "rejects_v2_entry_missing_split_identity", run: func(t *testing.T) {
			t.Parallel()

			lf := NewLockFile()
			lf.Modules["https://github.com/user/repo.git"] = LockedModule{
				GitURL:          "https://github.com/user/repo.git",
				Version:         "^1.0.0",
				ResolvedVersion: "1.2.0",
				GitCommit:       "abc123def456789012345678901234567890abcd",
				Namespace:       "repo@1.2.0",
				ContentHash:     testContentHash,
			}

			err := lf.Save(filepath.Join(t.TempDir(), LockFileName))
			if err == nil {
				t.Fatal("Save() error = nil, want missing v2 split identity error")
			}
			if !errors.Is(err, ErrInvalidLockedModule) {
				t.Fatalf("Save() error = %v, want ErrInvalidLockedModule", err)
			}
			var lockedErr *InvalidLockedModuleError
			if !errors.As(err, &lockedErr) {
				t.Fatalf("Save() error = %T, want *InvalidLockedModuleError", err)
			}
			var fieldDetails []string
			for _, fieldErr := range lockedErr.FieldErrors {
				fieldDetails = append(fieldDetails, fieldErr.Error())
			}
			detail := strings.Join(fieldDetails, "\n")
			if !strings.Contains(detail, "command_source_id") || !strings.Contains(detail, "module_id") {
				t.Fatalf("Save() field errors = %v, want command_source_id and module_id details", lockedErr.FieldErrors)
			}
		}},
	}
	//nolint:paralleltest // Each table case runner begins with t.Parallel.
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
