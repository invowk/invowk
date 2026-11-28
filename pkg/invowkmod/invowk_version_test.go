// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"bytes"
	"errors"
	"log/slog"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestResolveBuildVersion(t *testing.T) {
	t.Parallel()

	noBuildInfo := func() (*debug.BuildInfo, bool) { return nil, false }
	buildInfoWith := func(ver string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: ver}}, true
		}
	}

	tests := []struct {
		name           string
		ldflagsVersion string
		buildInfo      func() (*debug.BuildInfo, bool)
		wantIsDev      bool
		wantNorm       string
	}{
		{
			name:           "ldflags semver without v prefix",
			ldflagsVersion: "0.18.0",
			buildInfo:      buildInfoWith("v0.19.0"),
			wantIsDev:      false,
			wantNorm:       "0.18.0",
		},
		{
			name:           "ldflags semver with v prefix and build metadata",
			ldflagsVersion: "v0.18.0+build.42",
			buildInfo:      noBuildInfo,
			wantIsDev:      false,
			wantNorm:       "0.18.0",
		},
		{
			name:           "ldflags prerelease",
			ldflagsVersion: "v1.2.3-rc.1+meta",
			buildInfo:      noBuildInfo,
			wantIsDev:      false,
			wantNorm:       "1.2.3-rc.1",
		},
		{
			name:           "dev ldflags falls back to ReadBuildInfo release version",
			ldflagsVersion: "dev",
			buildInfo:      buildInfoWith("v0.19.2"),
			wantIsDev:      false,
			wantNorm:       "0.19.2",
		},
		{
			name:           "dev ldflags strips build metadata from ReadBuildInfo",
			ldflagsVersion: "dev",
			buildInfo:      buildInfoWith("v0.19.2+dirty"),
			wantIsDev:      false,
			wantNorm:       "0.19.2",
		},
		{
			name:           "dev ldflags with (devel) build info is true dev build",
			ldflagsVersion: "dev",
			buildInfo:      buildInfoWith("(devel)"),
			wantIsDev:      true,
			wantNorm:       "",
		},
		{
			name:           "empty ldflags with unavailable build info is true dev build",
			ldflagsVersion: "",
			buildInfo:      noBuildInfo,
			wantIsDev:      true,
			wantNorm:       "",
		},
		{
			name:           "non-semver commit hash ldflags falls back to dev",
			ldflagsVersion: "1234567",
			buildInfo:      noBuildInfo,
			wantIsDev:      true,
			wantNorm:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ResolveBuildVersion(tt.ldflagsVersion, tt.buildInfo)
			if got.IsDev != tt.wantIsDev {
				t.Fatalf("IsDev = %v, want %v (got=%+v)", got.IsDev, tt.wantIsDev, got)
			}
			if got.Normalized != tt.wantNorm {
				t.Fatalf("Normalized = %q, want %q", got.Normalized, tt.wantNorm)
			}
			if !tt.wantIsDev && got.Parsed == nil {
				t.Fatal("Parsed = nil for non-dev version")
			}
		})
	}
}

func TestParseInvowkmodBytes_InvowkVersionAndHint(t *testing.T) { //nolint:paralleltest // Mutates global buildVersionOverride and slog default logger.
	//nolint:paralleltest // subtests mutate global buildVersionOverride.
	t.Run("satisfied constraint passes", func(t *testing.T) {
		defer OverrideBuildVersionForTest("0.18.0")()

		data := []byte(`
module: "example.tools"
version: "1.0.0"
invowk_version: ">=0.18.0"
`)
		meta, err := ParseInvowkmodBytes(data, types.FilesystemPath("invowkmod.cue"))
		if err != nil {
			t.Fatalf("ParseInvowkmodBytes() error = %v", err)
		}
		if meta.InvowkVersion != ">=0.18.0" {
			t.Fatalf("InvowkVersion = %q, want >=0.18.0", meta.InvowkVersion)
		}
	})

	//nolint:paralleltest // subtests mutate global buildVersionOverride.
	t.Run("CUE expression in invowk_version is evaluated before strict schema", func(t *testing.T) {
		defer OverrideBuildVersionForTest("v0.18.0")()

		data := []byte(`
_min: "0.19.0"
module: "example.tools"
version: "1.0.0"
invowk_version: ">=\(_min)"
future_unknown_field: true
`)
		_, err := ParseInvowkmodBytes(data, types.FilesystemPath("invowkmod.cue"))
		if !errors.Is(err, ErrUnsupportedInvowkVersion) {
			t.Fatalf("error = %v, want ErrUnsupportedInvowkVersion", err)
		}
		if strings.Contains(err.Error(), "field not allowed") {
			t.Fatalf("error %q must fail on invowk_version before strict schema check", err)
		}
		var verErr *UnsupportedInvowkVersionError
		if !errors.As(err, &verErr) {
			t.Fatalf("error type = %T, want *UnsupportedInvowkVersionError", err)
		}
		if verErr.Required != ">=0.19.0" || verErr.CurrentVersion != "0.18.0" {
			t.Fatalf("UnsupportedInvowkVersionError = %+v, want Required=>=0.19.0 Current=0.18.0", verErr)
		}
	})

	//nolint:paralleltest // subtests mutate global buildVersionOverride.
	t.Run("non-string invowk_version rejected cleanly in pre-pass", func(t *testing.T) {
		defer OverrideBuildVersionForTest("0.18.0")()

		data := []byte(`
module: "example.tools"
version: "1.0.0"
invowk_version: 123
future_unknown_field: true
`)
		_, err := ParseInvowkmodBytes(data, types.FilesystemPath("invowkmod.cue"))
		if err == nil {
			t.Fatal("expected error for non-string invowk_version")
		}
		if strings.Contains(err.Error(), "field not allowed") {
			t.Fatalf("error %q should fail on non-string invowk_version before unknown-field check", err)
		}
	})

	//nolint:paralleltest // subtests mutate global buildVersionOverride.
	t.Run("field not allowed appends upgrade hint", func(t *testing.T) {
		defer OverrideBuildVersionForTest("0.18.0")()

		data := []byte(`
module: "example.tools"
version: "1.0.0"
bogusfield: true
`)
		_, err := ParseInvowkmodBytes(data, types.FilesystemPath("invowkmod.cue"))
		if err == nil {
			t.Fatal("expected schema error")
		}
		if !strings.Contains(err.Error(), "field not allowed") {
			t.Fatalf("error = %q, want field not allowed", err)
		}
		if !strings.Contains(err.Error(), "hint: this file may require a newer version of invowk") {
			t.Fatalf("error = %q, want upgrade hint", err)
		}
	})

	//nolint:paralleltest // subtests mutate global buildVersionOverride and slog default logger.
	t.Run("true dev build warns and continues", func(t *testing.T) {
		defer OverrideBuildVersionForTest("dev")()

		var logBuf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
		defer slog.SetDefault(prev)

		data := []byte(`
module: "example.tools"
version: "1.0.0"
invowk_version: ">=99.0.0"
`)
		meta, err := ParseInvowkmodBytes(data, types.FilesystemPath("invowkmod.cue"))
		if err != nil {
			t.Fatalf("ParseInvowkmodBytes() in dev build error = %v", err)
		}
		if meta.InvowkVersion != ">=99.0.0" {
			t.Fatalf("InvowkVersion = %q, want >=99.0.0", meta.InvowkVersion)
		}
		if !strings.Contains(logBuf.String(), "invowk_version") {
			t.Fatalf("expected slog warning mentioning invowk_version, got %q", logBuf.String())
		}
	})
}
