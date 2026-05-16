// SPDX-License-Identifier: MPL-2.0

package containerplan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestResolvePersistentTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		req        PersistentRequest
		wantMode   PersistentMode
		wantName   invowkfile.ContainerName
		wantSource PersistentNameSource
		wantCreate bool
	}{
		{
			name:     "ephemeral when not requested",
			req:      PersistentRequest{},
			wantMode: PersistentModeEphemeral,
		},
		{
			name:       "CLI name wins",
			req:        mustPersistentRequest(t, "", "", "", "cli-dev", &invowkfile.RuntimePersistentConfig{Name: "cfg-dev", CreateIfMissing: true}),
			wantMode:   PersistentModePersistent,
			wantName:   "cli-dev",
			wantSource: PersistentNameSourceCLI,
			wantCreate: true,
		},
		{
			name:       "config name wins over derived name",
			req:        mustPersistentRequest(t, "", "", "", "", &invowkfile.RuntimePersistentConfig{Name: "cfg-dev"}),
			wantMode:   PersistentModePersistent,
			wantName:   "cfg-dev",
			wantSource: PersistentNameSourceConfig,
		},
		{
			name: "derive name when only create-if-missing is set",
			req: mustPersistentRequest(t,
				"",
				"build assets",
				invowkfile.FilesystemPath(filepath.Join("workspace", "invowkfile.cue")),
				"",
				&invowkfile.RuntimePersistentConfig{CreateIfMissing: true},
			),
			wantMode:   PersistentModePersistent,
			wantSource: PersistentNameSourceDerived,
			wantCreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ResolvePersistentTarget(tt.req)
			if got.Mode() != tt.wantMode {
				t.Fatalf("Mode = %q, want %q", got.Mode(), tt.wantMode)
			}
			if tt.wantName != "" && got.Name() != tt.wantName {
				t.Fatalf("Name = %q, want %q", got.Name(), tt.wantName)
			}
			if got.NameSource() != tt.wantSource {
				t.Fatalf("NameSource = %q, want %q", got.NameSource(), tt.wantSource)
			}
			if got.CreateIfMissing() != tt.wantCreate {
				t.Fatalf("CreateIfMissing = %v, want %v", got.CreateIfMissing(), tt.wantCreate)
			}
			if got.Requested() != (tt.wantMode == PersistentModePersistent) {
				t.Fatalf("Requested() = %v for mode %q", got.Requested(), got.Mode())
			}
			if got.Requested() && got.Name() == "" {
				t.Fatal("persistent plan has empty name")
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}

func TestDerivePersistentName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace CommandNamespace
		wantSlug  string
	}{
		{
			name:      "root command namespace",
			namespace: "build assets",
			wantSlug:  "invowk-build-assets-",
		},
		{
			name:      "module command namespace",
			namespace: "io.invowk.tools build",
			wantSlug:  "invowk-io.invowk.tools-build-",
		},
		{
			name:      "punctuation is normalized",
			namespace: "Build:Release/All",
			wantSlug:  "invowk-build-release-all-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := mustPersistentRequest(t,
				tt.namespace,
				"",
				invowkfile.FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue")),
				"",
				nil,
			)
			name := DerivePersistentName(req)
			if !strings.HasPrefix(string(name), tt.wantSlug) {
				t.Fatalf("derived name = %q, want prefix %q", name, tt.wantSlug)
			}
			if err := name.Validate(); err != nil {
				t.Fatalf("derived name Validate() = %v", err)
			}
			again := DerivePersistentName(req)
			if again != name {
				t.Fatalf("derived name is not deterministic: %q then %q", name, again)
			}
		})
	}
}

func mustPersistentRequest(
	t *testing.T,
	commandFullName, commandName CommandNamespace,
	invowkfilePath invowkfile.FilesystemPath,
	containerNameOverride invowkfile.ContainerName,
	cfg *invowkfile.RuntimePersistentConfig,
) PersistentRequest {
	t.Helper()
	opts := []PersistentRequestOption{
		WithContainerNameOverride(containerNameOverride),
		WithConfig(cfg),
	}
	if commandFullName != "" {
		opts = append(opts, WithCommandFullName(&commandFullName))
	}
	if commandName != "" {
		opts = append(opts, WithCommandName(&commandName))
	}
	if invowkfilePath != "" {
		opts = append(opts, WithInvowkfilePath(&invowkfilePath))
	}
	req, err := NewPersistentRequest(opts...)
	if err != nil {
		t.Fatalf("NewPersistentRequest() error = %v", err)
	}
	return req
}
