// SPDX-License-Identifier: MPL-2.0

package pathmatrix

import (
	goruntime "runtime"
	"sort"
	"testing"
)

// Canonical volume-mount input strings exposed for use in ExtraVectors and
// custom assertions. Volume mounts have the shape `host:container[:options]`
// where the host portion is platform-native and the container portion is
// always forward-slash. The most subtle bug class lives in how the host
// portion is converted before string concatenation — v0.10.0 bug #4 was
// exactly the case where a Windows backslash host path landed in the
// `host:container` string verbatim, producing an unparseable spec.
const (
	// VMUnixHostUnixContainer is a Unix host path mounted at a Unix
	// container path.
	VMUnixHostUnixContainer = "/host:/container"

	// VMWindowsBackslashHostUnix is a Windows backslash host path mounted
	// at a Unix container path. This is the v0.10.0 bug #4 vector — a
	// validator that doesn't ToSlash the host portion will reject this
	// spec or accept it incorrectly.
	VMWindowsBackslashHostUnix = `C:\host:/container`

	// VMWindowsForwardSlashHostUnix is the same Windows host path written
	// with forward slashes — Windows accepts both forms.
	VMWindowsForwardSlashHostUnix = "C:/host:/container"

	// VMNamedVolumeUnix is a named Docker volume bound to a Unix
	// container path (no host filesystem path involved).
	VMNamedVolumeUnix = "myvol:/container"

	// VMRelativeHostUnix is a relative host path. Most engines reject
	// these.
	VMRelativeHostUnix = "./host:/container"

	// VMHostWithColonInPath is a Windows host path containing a colon
	// embedded in the path itself (uncommon but possible) — exercises
	// the validator's parsing of multiple colons.
	VMHostWithColonInPath = `C:\path\with:colon:/container`

	// VMEmptyHost is a malformed spec with no host portion.
	VMEmptyHost = ":/container"

	// VMEmptyContainer is a malformed spec with no container portion.
	VMEmptyContainer = "/host:"
)

type (
	// VolumeMountVectors holds one Outcome per canonical volume-mount
	// vector plus optional per-platform overrides and additional
	// surface-specific vectors. Like [Expectations] for paths, all base
	// fields must be set; the matrix Fatalfs with a list of missing
	// vectors at setup.
	VolumeMountVectors struct {
		ExtraVectors                map[string]VectorCase
		OnWindows                   *VolumeMountPlatformOverride
		OnLinux                     *VolumeMountPlatformOverride
		OnDarwin                    *VolumeMountPlatformOverride
		UnixHostUnixContainer       Outcome
		WindowsBackslashHostUnix    Outcome
		WindowsForwardSlashHostUnix Outcome
		NamedVolumeUnix             Outcome
		RelativeHostUnix            Outcome
		HostWithColonInPath         Outcome
		EmptyHost                   Outcome
		EmptyContainer              Outcome
	}

	// VolumeMountPlatformOverride supersedes the corresponding
	// VolumeMountVectors field when goruntime.GOOS matches the parent
	// override (OnWindows/OnLinux/OnDarwin). A nil pointer field means
	// "inherit".
	VolumeMountPlatformOverride struct {
		UnixHostUnixContainer       *Outcome
		WindowsBackslashHostUnix    *Outcome
		WindowsForwardSlashHostUnix *Outcome
		NamedVolumeUnix             *Outcome
		RelativeHostUnix            *Outcome
		HostWithColonInPath         *Outcome
		EmptyHost                   *Outcome
		EmptyContainer              *Outcome
	}
)

// VolumeMount runs the canonical volume-mount matrix against a validator
// that takes a `host:container[:options]` spec string and returns an
// error. Use this for compound volume-mount validators such as
// `(*VolumeMountSpec).Validate` or `ValidateVolumeMount`.
//
// The eight canonical vectors target the bug class behind v0.10.0 #4: a
// Windows backslash host path embedded in a `host:container` string. A
// validator that doesn't `filepath.ToSlash` the host portion (or doesn't
// reject the backslash form outright) will fail this matrix on the
// `windows_backslash_host_unix` vector.
func VolumeMount(t *testing.T, validate func(spec string) error, expect VolumeMountVectors) {
	t.Helper()
	if missing := missingVMBaseVectors(expect); len(missing) > 0 {
		t.Fatalf("pathmatrix.VolumeMount: missing base-vector expectations: %v", missing)
	}
	if validate == nil {
		t.Fatal("pathmatrix.VolumeMount: validate func is nil")
	}
	resolve := func(input string) (string, error) { return "", validate(input) }
	t.Run("matrix", func(matrix *testing.T) {
		matrix.Helper()
		vectors := vmCanonicalVectors(expect)
		for i := range vectors {
			vec := vectors[i]
			matrix.Run(vec.name, func(sub *testing.T) {
				sub.Parallel()
				runOneVector(sub, "" /* baseDir unused */, resolve, vec.input, vec.outcome, validatorMode)
			})
		}
		if len(expect.ExtraVectors) == 0 {
			return
		}
		extraNames := make([]string, 0, len(expect.ExtraVectors))
		for name := range expect.ExtraVectors {
			extraNames = append(extraNames, name)
		}
		sort.Strings(extraNames)
		for _, name := range extraNames {
			extra := expect.ExtraVectors[name]
			matrix.Run("extra/"+name, func(sub *testing.T) {
				sub.Parallel()
				runOneVector(sub, "" /* baseDir unused */, resolve, extra.Input, extra.Expect, validatorMode)
			})
		}
	})
}

// missingVMBaseVectors returns the names of base-VolumeMountVectors fields
// that are the zero Outcome. Used to fail-fast at matrix setup with a
// clear list.
func missingVMBaseVectors(v VolumeMountVectors) []string {
	checks := []struct {
		name string
		kind outcomeKind
	}{
		{"UnixHostUnixContainer", v.UnixHostUnixContainer.kind},
		{"WindowsBackslashHostUnix", v.WindowsBackslashHostUnix.kind},
		{"WindowsForwardSlashHostUnix", v.WindowsForwardSlashHostUnix.kind},
		{"NamedVolumeUnix", v.NamedVolumeUnix.kind},
		{"RelativeHostUnix", v.RelativeHostUnix.kind},
		{"HostWithColonInPath", v.HostWithColonInPath.kind},
		{"EmptyHost", v.EmptyHost.kind},
		{"EmptyContainer", v.EmptyContainer.kind},
	}
	var missing []string
	for _, c := range checks {
		if c.kind == outcomeUnset {
			missing = append(missing, c.name)
		}
	}
	return missing
}

// activeVMOverride returns the platform override matching goruntime.GOOS,
// or nil if none is configured for the current platform.
func activeVMOverride(v VolumeMountVectors) *VolumeMountPlatformOverride {
	switch goruntime.GOOS {
	case "windows":
		return v.OnWindows
	case "linux":
		return v.OnLinux
	case "darwin":
		return v.OnDarwin
	}
	return nil
}

// vmCanonicalVectors enumerates the eight base volume-mount vectors in
// stable order. Used by VolumeMount to drive subtests.
func vmCanonicalVectors(v VolumeMountVectors) []vectorEntry {
	override := activeVMOverride(v)
	pick := func(base Outcome, getOverride func(*VolumeMountPlatformOverride) *Outcome) Outcome {
		if override == nil {
			return base
		}
		return resolveOutcome(base, getOverride(override))
	}
	return []vectorEntry{
		{
			name:    "unix_host_unix_container",
			input:   VMUnixHostUnixContainer,
			outcome: pick(v.UnixHostUnixContainer, func(p *VolumeMountPlatformOverride) *Outcome { return p.UnixHostUnixContainer }),
		},
		{
			name:    "windows_backslash_host_unix",
			input:   VMWindowsBackslashHostUnix,
			outcome: pick(v.WindowsBackslashHostUnix, func(p *VolumeMountPlatformOverride) *Outcome { return p.WindowsBackslashHostUnix }),
		},
		{
			name:    "windows_forward_slash_host_unix",
			input:   VMWindowsForwardSlashHostUnix,
			outcome: pick(v.WindowsForwardSlashHostUnix, func(p *VolumeMountPlatformOverride) *Outcome { return p.WindowsForwardSlashHostUnix }),
		},
		{
			name:    "named_volume_unix",
			input:   VMNamedVolumeUnix,
			outcome: pick(v.NamedVolumeUnix, func(p *VolumeMountPlatformOverride) *Outcome { return p.NamedVolumeUnix }),
		},
		{
			name:    "relative_host_unix",
			input:   VMRelativeHostUnix,
			outcome: pick(v.RelativeHostUnix, func(p *VolumeMountPlatformOverride) *Outcome { return p.RelativeHostUnix }),
		},
		{
			name:    "host_with_colon_in_path",
			input:   VMHostWithColonInPath,
			outcome: pick(v.HostWithColonInPath, func(p *VolumeMountPlatformOverride) *Outcome { return p.HostWithColonInPath }),
		},
		{
			name:    "empty_host",
			input:   VMEmptyHost,
			outcome: pick(v.EmptyHost, func(p *VolumeMountPlatformOverride) *Outcome { return p.EmptyHost }),
		},
		{
			name:    "empty_container",
			input:   VMEmptyContainer,
			outcome: pick(v.EmptyContainer, func(p *VolumeMountPlatformOverride) *Outcome { return p.EmptyContainer }),
		},
	}
}
