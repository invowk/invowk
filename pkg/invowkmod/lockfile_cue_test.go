// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLockFile_toCUE(t *testing.T) {
	t.Parallel()

	generated := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name      string
		lockFile  *LockFile
		required  []string
		forbidden []string
	}{
		{
			name: "empty_modules",
			lockFile: &LockFile{
				Version:   "2.0",
				Generated: generated,
				Modules:   make(map[ModuleRefKey]LockedModule),
			},
			required: []string{"// invowkmod.lock.cue", "// DO NOT EDIT MANUALLY", `version: "2.0"`, "modules: {}"},
		},
		{
			name: "single_module",
			lockFile: &LockFile{
				Version:   "2.0",
				Generated: generated,
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
			},
			required:  []string{`"https://github.com/user/repo.git"`, "git_url:", "resolved_version:", "namespace:", "content_hash:"},
			forbidden: []string{"alias:", "path:"},
		},
		{
			name: "module_with_optional_fields",
			lockFile: &LockFile{
				Version:   "2.0",
				Generated: generated,
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
			},
			required: []string{"alias:", "path:", "module_id:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireStringFragments(t, tt.lockFile.toCUE(), tt.required, tt.forbidden)
		})
	}
}

func TestParseLockFileCUE(t *testing.T) {
	t.Parallel()

	t.Run("valid_single_module", testParseLockFileCUEValidSingleModule)
	t.Run("with_optional_fields", testParseLockFileCUEWithOptionalFields)
	t.Run("empty_modules", testParseLockFileCUEEmptyModules)
	t.Run("comments_ignored", testParseLockFileCUECommentsIgnored)
	t.Run("version_field_collision_guard", testParseLockFileCUEVersionFieldCollision)
	t.Run("invalid_version_empty", testParseLockFileCUEInvalidVersionEmpty)
	t.Run("unknown_version_rejected", testParseLockFileCUEUnknownVersion)
	t.Run("malformed_cue_rejected", testParseLockFileCUEMalformed)
	t.Run("invalid_generated_timestamp_rejected", testParseLockFileCUEInvalidTimestamp)
	t.Run("v1_lock_file_accepted", testParseLockFileCUEV1Accepted)
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
	if !reflect.DeepEqual(parsed, original) {
		t.Errorf("round-trip lock file mismatch:\n got: %#v\nwant: %#v", parsed, original)
	}
}

func testParseLockFileCUEValidSingleModule(t *testing.T) {
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
	lf := parseLockFileForTest(t, content)
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
}

func testParseLockFileCUEWithOptionalFields(t *testing.T) {
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
	lf := parseLockFileForTest(t, content)
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
}

func testParseLockFileCUEEmptyModules(t *testing.T) {
	t.Parallel()

	content := `version: "2.0"
generated: "2025-01-15T10:30:00Z"

modules: {}`
	lf := parseLockFileForTest(t, content)
	if len(lf.Modules) != 0 {
		t.Errorf("expected 0 modules, got %d", len(lf.Modules))
	}
}

func testParseLockFileCUECommentsIgnored(t *testing.T) {
	t.Parallel()

	content := `// This is a comment
version: "2.0"
// Another comment
generated: "2025-01-15T10:30:00Z"

modules: {}`
	lf := parseLockFileForTest(t, content)
	if lf.Version != "2.0" {
		t.Errorf("Version = %q", lf.Version)
	}
}

func testParseLockFileCUEVersionFieldCollision(t *testing.T) {
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
	lf := parseLockFileForTest(t, content)
	if lf.Version != "2.0" {
		t.Errorf("top-level Version = %q, want %q (module version field leaked)", lf.Version, "2.0")
	}
	mod := lf.Modules[ModuleRefKey("https://github.com/user/repo.git")]
	if mod.Version != "^1.0.0" {
		t.Errorf("module Version = %q, want %q", mod.Version, "^1.0.0")
	}
}

func testParseLockFileCUEInvalidVersionEmpty(t *testing.T) {
	t.Parallel()

	content := `version: ""
generated: "2025-01-15T10:30:00Z"
modules: {}`
	_, err := parseLockFile(content)
	if err == nil {
		t.Fatal("expected error for empty version, got nil")
	}
}

func testParseLockFileCUEUnknownVersion(t *testing.T) {
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
}

func testParseLockFileCUEMalformed(t *testing.T) {
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
}

func testParseLockFileCUEInvalidTimestamp(t *testing.T) {
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
}

func testParseLockFileCUEV1Accepted(t *testing.T) {
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
	lf := parseLockFileForTest(t, content)
	if lf.Version != "1.0" {
		t.Errorf("Version = %q, want %q", lf.Version, "1.0")
	}
	if len(lf.Modules) != 1 {
		t.Errorf("expected 1 module, got %d", len(lf.Modules))
	}
}

func parseLockFileForTest(t *testing.T, content string) *LockFile {
	t.Helper()

	lockFile, err := parseLockFile(content)
	if err != nil {
		t.Fatalf("parseLockFile() error: %v", err)
	}
	return lockFile
}

func requireStringFragments(t *testing.T, got string, required, forbidden []string) {
	t.Helper()

	for _, fragment := range required {
		if !strings.Contains(got, fragment) {
			t.Errorf("toCUE() output missing %q:\n%s", fragment, got)
		}
	}
	for _, fragment := range forbidden {
		if strings.Contains(got, fragment) {
			t.Errorf("toCUE() output unexpectedly contains %q:\n%s", fragment, got)
		}
	}
}
