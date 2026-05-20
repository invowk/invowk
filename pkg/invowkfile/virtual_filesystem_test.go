// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestVirtualFilesystemPathNameValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   VirtualFilesystemPathName
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
				if !errors.Is(err, ErrInvalidVirtualFilesystemPathName) {
					t.Fatalf("Validate() error = %v, want ErrInvalidVirtualFilesystemPathName", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestVirtualFilesystemAccessValidateAndDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     VirtualFilesystemAccess
		effective VirtualFilesystemAccess
		wantErr   bool
	}{
		{name: "zero defaults restricted", effective: VirtualFilesystemAccessRestricted},
		{name: "restricted", value: VirtualFilesystemAccessRestricted, effective: VirtualFilesystemAccessRestricted},
		{name: "full", value: VirtualFilesystemAccessFull, effective: VirtualFilesystemAccessFull},
		{name: "custom rejected", value: "custom", effective: "custom", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.value.Validate()
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidVirtualFilesystemAccess) {
					t.Fatalf("Validate() error = %v, want ErrInvalidVirtualFilesystemAccess", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
			if got := tt.value.Effective(); got != tt.effective {
				t.Fatalf("Effective() = %q, want %q", got, tt.effective)
			}
		})
	}
}

func TestParseVirtualFilesystemAcceptsRestrictedAndFull(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		filesystem string
		wantAccess VirtualFilesystemAccess
		wantPaths  int
	}{
		{
			name: "restricted with paths",
			filesystem: `{
				access: "restricted"
				paths: {
					DB_ROOT: "./db"
					CACHE_ROOT: "@cache/invowk-test"
				}
			}`,
			wantAccess: VirtualFilesystemAccessRestricted,
			wantPaths:  2,
		},
		{
			name:       "full without paths",
			filesystem: `{access: "full"}`,
			wantAccess: VirtualFilesystemAccessFull,
		},
		{
			name:       "omitted access defaults restricted",
			filesystem: `{paths: {DATA: "@data/invowk-test"}}`,
			wantAccess: VirtualFilesystemAccessRestricted,
			wantPaths:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := []byte(`
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo ok"}
		runtimes: [{name: "virtual-sh"}]
		platforms: [{
			name: "linux"
			virtual: {filesystem: ` + tt.filesystem + `}
		}]
	}]
}]`)

			inv, err := ParseBytes(data, "invowkfile.cue")
			if err != nil {
				t.Fatalf("ParseBytes() error = %v, want nil", err)
			}
			filesystem := inv.Commands[0].Implementations[0].Platforms[0].VirtualFilesystem()
			if got := filesystem.EffectiveAccess(); got != tt.wantAccess {
				t.Fatalf("EffectiveAccess() = %q, want %q", got, tt.wantAccess)
			}
			if got := len(filesystem.Paths); got != tt.wantPaths {
				t.Fatalf("paths len = %d, want %d", got, tt.wantPaths)
			}
		})
	}
}

func TestParseVirtualFilesystemRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "implementation-level allowed_paths rejected",
			body: `allowed_paths: {DB_ROOT: "./db"}`,
			want: "allowed_paths",
		},
		{
			name: "platform keyed path object rejected",
			body: `platforms: [{
				name: "linux"
				virtual: {filesystem: {paths: {DB_ROOT: {linux: "/var/lib/app"}}}}
			}]`,
			want: "DB_ROOT",
		},
		{
			name: "invalid access rejected",
			body: `platforms: [{
				name: "linux"
				virtual: {filesystem: {access: "custom"}}
			}]`,
			want: "custom",
		},
		{
			name: "invalid path name rejected",
			body: `platforms: [{
				name: "linux"
				virtual: {filesystem: {paths: {"db-root": "./db"}}}
			}]`,
			want: "db-root",
		},
		{
			name: "empty path value rejected",
			body: `platforms: [{
				name: "linux"
				virtual: {filesystem: {paths: {DB_ROOT: ""}}}
			}]`,
			want: "DB_ROOT",
		},
		{
			name: "binary policy under virtual rejected",
			body: `platforms: [{
				name: "linux"
				virtual: {allowed_binaries: ["git"]}
			}]`,
			want: "allowed_binaries",
		},
		{
			name: "binary policy under filesystem rejected",
			body: `platforms: [{
				name: "linux"
				virtual: {filesystem: {allowed_binaries: ["git"]}}
			}]`,
			want: "allowed_binaries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			platforms := `platforms: [{name: "linux"}]`
			body := tt.body
			if strings.Contains(body, "platforms:") {
				platforms = ""
			}
			data := []byte(`
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo ok"}
		runtimes: [{name: "virtual-sh"}]
		` + platforms + `
		` + body + `
	}]
}]`)

			_, err := ParseBytes(data, "invowkfile.cue")
			if err == nil {
				t.Fatal("ParseBytes() error = nil, want rejection")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseBytes() error = %v, want mention %q", err, tt.want)
			}
		})
	}
}

func TestGenerateCUEVirtualFilesystemRoundTrip(t *testing.T) {
	t.Parallel()

	original := &Invowkfile{
		Commands: []Command{{
			Name: "test",
			Implementations: []Implementation{{
				Script:   ImplementationScript{Content: "echo ok"},
				Runtimes: []RuntimeConfig{{Name: RuntimeVirtualLua}},
				Platforms: []PlatformConfig{{
					Name: PlatformLinux,
					Virtual: &PlatformVirtualConfig{Filesystem: &VirtualFilesystemConfig{
						Access: VirtualFilesystemAccessFull,
						Paths: VirtualFilesystemPaths{
							"DB_ROOT":    "./db",
							"CACHE_ROOT": "@cache/invowk-test",
						},
					}},
				}},
			}},
		}},
	}

	generated := GenerateCUE(original)
	for _, want := range []string{`virtual: {`, `filesystem: {`, `access: "full"`, `"DB_ROOT": "./db"`} {
		if !strings.Contains(generated, want) {
			t.Fatalf("GenerateCUE() missing %q:\n%s", want, generated)
		}
	}
	if strings.Contains(generated, "allowed_paths") {
		t.Fatalf("GenerateCUE() emitted removed field:\n%s", generated)
	}
	parsed, err := ParseBytes([]byte(generated), "generated.cue")
	if err != nil {
		t.Fatalf("ParseBytes(GenerateCUE()) error = %v\n%s", err, generated)
	}
	filesystem := parsed.Commands[0].Implementations[0].Platforms[0].VirtualFilesystem()
	if got := filesystem.EffectiveAccess(); got != VirtualFilesystemAccessFull {
		t.Fatalf("parsed access = %q, want full", got)
	}
	if got := len(filesystem.Paths); got != 2 {
		t.Fatalf("parsed paths = %v, want 2 entries", filesystem.Paths)
	}
}
