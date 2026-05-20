// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestAllowedPathNameValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   AllowedPathName
		wantErr bool
	}{
		{name: "uppercase", value: "DB_ROOT"},
		{name: "underscore prefix", value: "_CACHE"},
		{name: "lowercase rejected", value: "db_root", wantErr: true},
		{name: "digit prefix rejected", value: "1_CACHE", wantErr: true},
		{name: "punctuation rejected", value: "DB-ROOT", wantErr: true},
		{name: "space rejected", value: "DB ROOT", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidAllowedPathName) {
					t.Fatalf("Validate() error = %v, want ErrInvalidAllowedPathName", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestParseAllowedPathsAcceptsCommonAndPlatformValues(t *testing.T) {
	t.Parallel()

	data := []byte(`
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo ok"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}, {name: "macos"}]
		allowed_paths: {
			DB_ROOT: "./db"
			CACHE_ROOT: {
				linux: "/var/cache/invowk-test"
				macos: "/Users/Shared/invowk-test-cache"
			}
		}
	}]
}]`)

	inv, err := ParseBytes(data, "invowkfile.cue")
	if err != nil {
		t.Fatalf("ParseBytes() error = %v, want nil", err)
	}
	paths := inv.Commands[0].Implementations[0].AllowedPaths
	if got, ok, err := paths.PathForPlatform("DB_ROOT", PlatformLinux); err != nil || !ok || got != "./db" {
		t.Fatalf("DB_ROOT linux = %q, %v, %v; want ./db, true, nil", got, ok, err)
	}
	if got, ok, err := paths.PathForPlatform("CACHE_ROOT", PlatformMac); err != nil || !ok || got != "/Users/Shared/invowk-test-cache" {
		t.Fatalf("CACHE_ROOT macos = %q, %v, %v; want macOS mapping", got, ok, err)
	}
}

func TestParseAllowedPathsRejectsMissingPlatformMapping(t *testing.T) {
	t.Parallel()

	data := []byte(`
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo ok"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{name: "linux"}, {name: "windows"}]
		allowed_paths: {
			DB_ROOT: {
				linux: "/var/lib/app"
			}
		}
	}]
}]`)

	_, err := ParseBytes(data, "invowkfile.cue")
	if err == nil {
		t.Fatal("ParseBytes() error = nil, want missing windows mapping error")
	}
	if !strings.Contains(err.Error(), "missing \"windows\" mapping") {
		t.Fatalf("ParseBytes() error = %v, want missing windows mapping", err)
	}
}

func TestGenerateCUEAllowedPathsRoundTrip(t *testing.T) {
	t.Parallel()

	original := &Invowkfile{
		Commands: []Command{{
			Name: "test",
			Implementations: []Implementation{{
				Script:   ImplementationScript{Content: "echo ok"},
				Runtimes: []RuntimeConfig{{Name: RuntimeVirtualLua}},
				Platforms: []PlatformConfig{
					{Name: PlatformLinux},
					{Name: PlatformWindows},
				},
				AllowedPaths: AllowedPaths{
					"DB_ROOT": "./db",
					"CACHE_ROOT": map[string]any{
						"linux":   "/var/cache/invowk-test",
						"windows": "C:/ProgramData/InvowkTest/cache",
					},
				},
			}},
		}},
	}

	generated := GenerateCUE(original)
	if !strings.Contains(generated, "allowed_paths:") {
		t.Fatalf("GenerateCUE() missing allowed_paths:\n%s", generated)
	}
	parsed, err := ParseBytes([]byte(generated), "generated.cue")
	if err != nil {
		t.Fatalf("ParseBytes(GenerateCUE()) error = %v\n%s", err, generated)
	}
	if got := parsed.Commands[0].Implementations[0].AllowedPaths; len(got) != 2 {
		t.Fatalf("parsed allowed_paths = %v, want 2 entries", got)
	}
}
