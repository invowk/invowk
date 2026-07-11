// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLockFile_toCUE(t *testing.T) {
	t.Parallel()

	t.Run("empty_modules", func(t *testing.T) {
		t.Parallel()

		lf := &LockFile{
			Version:   "2.0",
			Generated: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			Modules:   make(map[ModuleRefKey]LockedModule),
		}
		got := lf.toCUE()

		if !strings.Contains(got, "// invowkmod.lock.cue") {
			t.Error("missing header comment")
		}
		if !strings.Contains(got, "// DO NOT EDIT MANUALLY") {
			t.Error("missing DO NOT EDIT comment")
		}
		if !strings.Contains(got, `version: "2.0"`) {
			t.Error("missing version field")
		}
		if !strings.Contains(got, "modules: {}") {
			t.Errorf("expected empty modules block, got:\n%s", got)
		}
	})

	t.Run("single_module", func(t *testing.T) {
		t.Parallel()

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
					ContentHash:     testContentHash,
				},
			},
		}
		got := lf.toCUE()

		if !strings.Contains(got, `"https://github.com/user/repo.git"`) {
			t.Error("missing module key")
		}
		if !strings.Contains(got, `git_url:`) {
			t.Error("missing git_url field")
		}
		if !strings.Contains(got, `resolved_version:`) {
			t.Error("missing resolved_version field")
		}
		if !strings.Contains(got, `namespace:`) {
			t.Error("missing namespace field")
		}
		if !strings.Contains(got, `content_hash:`) {
			t.Error("missing content_hash field")
		}
		// Optional fields should NOT be rendered when empty
		if strings.Contains(got, "alias:") {
			t.Error("alias should not be rendered when empty")
		}
		if strings.Contains(got, "path:") {
			t.Error("path should not be rendered when empty")
		}
	})

	t.Run("module_with_optional_fields", func(t *testing.T) {
		t.Parallel()

		lf := &LockFile{
			Version:   "2.0",
			Generated: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			Modules: map[ModuleRefKey]LockedModule{
				"https://github.com/org/repo.git#sub": {
					GitURL:          "https://github.com/org/repo.git",
					Version:         "^2.0.0",
					ResolvedVersion: "2.1.0",
					GitCommit:       "abc123def456789012345678901234567890abcd",
					Alias:           "myalias",
					Path:            "sub",
					ModuleID:        "io.example.tools",
					Namespace:       "myalias",
					ContentHash:     testContentHash,
				},
			},
		}
		got := lf.toCUE()

		if !strings.Contains(got, `alias:`) {
			t.Error("alias should be rendered when non-empty")
		}
		if !strings.Contains(got, `path:`) {
			t.Error("path should be rendered when non-empty")
		}
		if !strings.Contains(got, `module_id:`) {
			t.Error("module_id should be rendered when non-empty")
		}
	})
}

func TestParseLockFileCUE(t *testing.T) {
	t.Parallel()

	t.Run("valid_single_module", func(t *testing.T) {
		t.Parallel()

		content := `// invowkmod.lock.cue
// DO NOT EDIT MANUALLY

version: "2.0"
generated: "2025-01-15T10:30:00Z"

modules: {
	"https://github.com/user/repo.git": {
		git_url:          "https://github.com/user/repo.git"
		version:          "^1.0.0"
		resolved_version: "1.2.0"
		git_commit:       "abc123def456789012345678901234567890abcd"
		namespace:        "repo@1.2.0"
		command_source_id: "repo"
		module_id:        "repo"
		content_hash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
}`
		lf, err := parseLockFile(content)
		if err != nil {
			t.Fatalf("parseLockFile() error: %v", err)
		}
		if lf.Version != "2.0" {
			t.Errorf("Version = %q, want %q", lf.Version, "2.0")
		}
		if len(lf.Modules) != 1 {
			t.Fatalf("expected 1 module, got %d", len(lf.Modules))
		}

		key := ModuleRefKey("https://github.com/user/repo.git")
		mod, ok := lf.Modules[key]
		if !ok {
			t.Fatal("module not found by key")
		}
		if mod.ResolvedVersion != "1.2.0" {
			t.Errorf("ResolvedVersion = %q", mod.ResolvedVersion)
		}
		if mod.GitCommit != "abc123def456789012345678901234567890abcd" {
			t.Errorf("GitCommit = %q", mod.GitCommit)
		}
	})

	t.Run("with_optional_fields", func(t *testing.T) {
		t.Parallel()

		content := `version: "2.0"
generated: "2025-01-15T10:30:00Z"

modules: {
	"https://github.com/org/repo.git#sub": {
		git_url:          "https://github.com/org/repo.git"
		version:          "^2.0.0"
		resolved_version: "2.1.0"
		git_commit:       "def456789012345678901234567890abcdef0123"
		alias:            "tools"
		path:             "sub"
		module_id:        "io.example.tools"
		namespace:        "tools"
		command_source_id: "tools"
		content_hash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
}`
		lf, err := parseLockFile(content)
		if err != nil {
			t.Fatalf("parseLockFile() error: %v", err)
		}

		key := ModuleRefKey("https://github.com/org/repo.git#sub")
		mod, ok := lf.Modules[key]
		if !ok {
			t.Fatal("module with subpath not found")
		}
		if mod.Alias != "tools" {
			t.Errorf("Alias = %q", mod.Alias)
		}
		if mod.Path != "sub" {
			t.Errorf("Path = %q", mod.Path)
		}
		if mod.ModuleID != "io.example.tools" {
			t.Errorf("ModuleID = %q", mod.ModuleID)
		}
	})

	t.Run("empty_modules", func(t *testing.T) {
		t.Parallel()

		content := `version: "2.0"
generated: "2025-01-15T10:30:00Z"

modules: {}`
		lf, err := parseLockFile(content)
		if err != nil {
			t.Fatalf("parseLockFile() error: %v", err)
		}
		if len(lf.Modules) != 0 {
			t.Errorf("expected 0 modules, got %d", len(lf.Modules))
		}
	})

	t.Run("comments_ignored", func(t *testing.T) {
		t.Parallel()

		content := `// This is a comment
version: "2.0"
// Another comment
generated: "2025-01-15T10:30:00Z"

modules: {}`
		lf, err := parseLockFile(content)
		if err != nil {
			t.Fatalf("parseLockFile() error: %v", err)
		}
		if lf.Version != "2.0" {
			t.Errorf("Version = %q", lf.Version)
		}
	})

	t.Run("version_field_collision_guard", func(t *testing.T) {
		t.Parallel()

		// The module-level "version:" field must NOT overwrite the top-level "version:".
		content := `version: "2.0"
generated: "2025-01-15T10:30:00Z"

modules: {
	"https://github.com/user/repo.git": {
		git_url:          "https://github.com/user/repo.git"
		version:          "^1.0.0"
		resolved_version: "1.2.0"
		git_commit:       "abc123def456789012345678901234567890abcd"
		namespace:        "repo@1.2.0"
		command_source_id: "repo"
		module_id:        "repo"
		content_hash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
}`
		lf, err := parseLockFile(content)
		if err != nil {
			t.Fatalf("parseLockFile() error: %v", err)
		}
		if lf.Version != "2.0" {
			t.Errorf("top-level Version = %q, want %q (module version field leaked)", lf.Version, "2.0")
		}
		mod := lf.Modules[ModuleRefKey("https://github.com/user/repo.git")]
		if mod.Version != "^1.0.0" {
			t.Errorf("module Version = %q, want %q", mod.Version, "^1.0.0")
		}
	})

	t.Run("invalid_version_empty", func(t *testing.T) {
		t.Parallel()

		content := `version: ""
generated: "2025-01-15T10:30:00Z"
modules: {}`
		_, err := parseLockFile(content)
		if err == nil {
			t.Fatal("expected error for empty version, got nil")
		}
	})

	t.Run("unknown_version_rejected", func(t *testing.T) {
		t.Parallel()

		content := `version: "99.0"
generated: "2025-01-15T10:30:00Z"
modules: {}`
		_, err := parseLockFile(content)
		if err == nil {
			t.Fatal("expected error for unknown version, got nil")
		}
		if !errors.Is(err, ErrUnknownLockFileVersion) {
			t.Errorf("expected ErrUnknownLockFileVersion, got: %v", err)
		}
	})

	t.Run("malformed_cue_rejected", func(t *testing.T) {
		t.Parallel()

		content := `version: "2.0"
generated: "2025-01-15T10:30:00Z"
modules: {
	"https://github.com/user/repo.git": {
		git_url: "https://github.com/user/repo.git"
`
		_, err := parseLockFile(content)
		if err == nil {
			t.Fatal("expected error for malformed lock file CUE, got nil")
		}
		if !strings.Contains(err.Error(), "parse lock file CUE") {
			t.Fatalf("error = %v, want CUE parse failure", err)
		}
	})

	t.Run("invalid_generated_timestamp_rejected", func(t *testing.T) {
		t.Parallel()

		content := `version: "2.0"
generated: "not-rfc3339"
modules: {}`
		_, err := parseLockFile(content)
		if err == nil {
			t.Fatal("expected error for invalid generated timestamp, got nil")
		}
		if !strings.Contains(err.Error(), "lock file generated") {
			t.Fatalf("error = %v, want generated timestamp failure", err)
		}
	})

	t.Run("v1_lock_file_accepted", func(t *testing.T) {
		t.Parallel()

		content := `version: "1.0"
generated: "2025-01-15T10:30:00Z"

modules: {
	"https://github.com/user/repo.git": {
		git_url:          "https://github.com/user/repo.git"
		version:          "^1.0.0"
		resolved_version: "1.2.0"
		git_commit:       "abc123def456789012345678901234567890abcd"
		namespace:        "repo@1.2.0"
		content_hash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
}`
		lf, err := parseLockFile(content)
		if err != nil {
			t.Fatalf("parseLockFile() error: %v", err)
		}
		if lf.Version != "1.0" {
			t.Errorf("Version = %q, want %q", lf.Version, "1.0")
		}
		if len(lf.Modules) != 1 {
			t.Errorf("expected 1 module, got %d", len(lf.Modules))
		}
	})
}

func TestParseStringValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"quoted_value", `version: "1.0"`, "1.0"},
		{"no_colon", "nocolon", ""},
		{"extra_spaces", `  key:   "value"  `, "value"},
		{"empty_value", `key: ""`, ""},
		{"unquoted_value", "key: bare", "bare"},
		{"unterminated_quote", `key: "bare`, "bare"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseStringValue(tt.input); got != tt.want {
				t.Errorf("parseStringValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseModuleKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  ModuleRefKey
	}{
		{"standard", `"https://github.com/user/repo.git": {`, ModuleRefKey("https://github.com/user/repo.git")},
		{"with_subpath", `"https://github.com/org/repo.git#sub": {`, ModuleRefKey("https://github.com/org/repo.git#sub")},
		{"no_quotes", `noquotes: {`, ModuleRefKey("")},
		{"empty", "", ModuleRefKey("")},
		{"only_quotes", `""`, ModuleRefKey("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseModuleKey(tt.input); got != tt.want {
				t.Errorf("parseModuleKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseLockFile_V1PreservesVersionState(t *testing.T) {
	t.Parallel()

	content := `version: "1.0"
generated: "2025-01-15T10:30:00Z"

modules: {
	"https://github.com/user/repo.git": {
		git_url:          "https://github.com/user/repo.git"
		version:          "^1.0.0"
		resolved_version: "1.2.0"
		git_commit:       "abc123def456789012345678901234567890abcd"
		namespace:        "repo@1.2.0"
		content_hash:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	}
}`
	lf, err := parseLockFile(content)
	if err != nil {
		t.Fatalf("parseLockFile() error: %v", err)
	}
	if lf.Version != "1.0" {
		t.Errorf("Version = %q, want %q", lf.Version, "1.0")
	}
}

func TestParseLockFileCUE_RoundTrip(t *testing.T) {
	t.Parallel()

	original := &LockFile{
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
			"https://github.com/org/mono.git#tools": {
				GitURL:          "https://github.com/org/mono.git",
				Version:         "~2.0.0",
				ResolvedVersion: "2.0.5",
				GitCommit:       "def456789012345678901234567890abcdef0123",
				Alias:           "tools",
				Path:            "tools",
				Namespace:       "tools",
				CommandSourceID: "tools",
				ModuleID:        "io.example.tools",
				ContentHash:     testContentHash2,
			},
		},
	}

	cue := original.toCUE()
	parsed, err := parseLockFile(cue)
	if err != nil {
		t.Fatalf("parseLockFile() error: %v", err)
	}

	if parsed.Version != original.Version {
		t.Errorf("Version = %q, want %q", parsed.Version, original.Version)
	}
	if len(parsed.Modules) != len(original.Modules) {
		t.Fatalf("module count = %d, want %d", len(parsed.Modules), len(original.Modules))
	}

	for key, origMod := range original.Modules {
		parsedMod, ok := parsed.Modules[key]
		if !ok {
			t.Errorf("module %q not found after round-trip", key)
			continue
		}
		if parsedMod.GitURL != origMod.GitURL {
			t.Errorf("[%s] GitURL = %q, want %q", key, parsedMod.GitURL, origMod.GitURL)
		}
		if parsedMod.Version != origMod.Version {
			t.Errorf("[%s] Version = %q, want %q", key, parsedMod.Version, origMod.Version)
		}
		if parsedMod.ResolvedVersion != origMod.ResolvedVersion {
			t.Errorf("[%s] ResolvedVersion = %q, want %q", key, parsedMod.ResolvedVersion, origMod.ResolvedVersion)
		}
		if parsedMod.GitCommit != origMod.GitCommit {
			t.Errorf("[%s] GitCommit = %q, want %q", key, parsedMod.GitCommit, origMod.GitCommit)
		}
		if parsedMod.Alias != origMod.Alias {
			t.Errorf("[%s] Alias = %q, want %q", key, parsedMod.Alias, origMod.Alias)
		}
		if parsedMod.Path != origMod.Path {
			t.Errorf("[%s] Path = %q, want %q", key, parsedMod.Path, origMod.Path)
		}
		if parsedMod.Namespace != origMod.Namespace {
			t.Errorf("[%s] Namespace = %q, want %q", key, parsedMod.Namespace, origMod.Namespace)
		}
		if parsedMod.ContentHash != origMod.ContentHash {
			t.Errorf("[%s] ContentHash = %q, want %q", key, parsedMod.ContentHash, origMod.ContentHash)
		}
	}
}
