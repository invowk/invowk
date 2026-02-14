// SPDX-License-Identifier: MPL-2.0

package cmd

import "testing"

func TestGetVersionString(t *testing.T) {
	// Not parallel: subtests mutate package-level Version/Commit/BuildDate vars.

	t.Run("ldflags version takes priority", func(t *testing.T) {
		// Save and restore package-level vars.
		origVersion, origCommit, origBuildDate := Version, Commit, BuildDate
		t.Cleanup(func() {
			Version, Commit, BuildDate = origVersion, origCommit, origBuildDate
		})

		Version = "v1.2.3"
		Commit = "abc1234"
		BuildDate = "2025-06-15T10:00:00Z"

		got := getVersionString()
		want := "v1.2.3 (commit: abc1234, built: 2025-06-15T10:00:00Z)"
		if got != want {
			t.Errorf("getVersionString() = %q, want %q", got, want)
		}
	})

	t.Run("fallback to dev when no build info", func(t *testing.T) {
		origVersion, origCommit, origBuildDate := Version, Commit, BuildDate
		t.Cleanup(func() {
			Version, Commit, BuildDate = origVersion, origCommit, origBuildDate
		})

		// In test binaries, debug.ReadBuildInfo() returns Main.Version == "(devel)",
		// so the function should fall through to the final fallback.
		Version = "dev"
		Commit = "unknown"
		BuildDate = "unknown"

		got := getVersionString()
		want := "dev (built from source)"
		if got != want {
			t.Errorf("getVersionString() = %q, want %q", got, want)
		}
	})

	// Note: The middle path (debug.ReadBuildInfo with a real module version) is
	// exercised by go-install binaries. It cannot be unit-tested because test
	// binaries always report Main.Version == "(devel)". The path is verified
	// manually via: go install ./... && $(go env GOBIN)/invowk --version
}
