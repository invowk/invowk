// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"runtime/debug"
	"strings"
	"sync/atomic"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/invowk/invowk/pkg/cueutil"
)

const (
	defaultDevBuildVersion        = "dev"
	develBuildInfoVersion         = "(devel)"
	fieldNotAllowedToken          = "field not allowed"
	fieldNotAllowedUpgradeHint    = "hint: this file may require a newer version of invowk"
	unsupportedInvowkVersionTitle = "unsupported invowk version"
)

var (
	// ErrUnsupportedInvowkVersion is the sentinel error wrapped by UnsupportedInvowkVersionError.
	ErrUnsupportedInvowkVersion = errors.New(unsupportedInvowkVersionTitle)

	// buildVersionOverride stores the ldflags-injected CLI version set by cmd/invowk.
	// Using atomic.Value keeps concurrent reads/overrides race-free.
	buildVersionOverride atomic.Value

	// releaseSemverPattern validates normalized release/pre-release versions with
	// at least MAJOR.MINOR.PATCH so commit-hash ldflags strings are not misread
	// as major-only semantic versions.
	releaseSemverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)
)

type (
	// BuildVersionInfo describes the resolved Invowk binary version used for
	// invowk_version constraint checks and CLI version display.
	BuildVersionInfo struct {
		// Raw is the raw version string from ldflags or debug.ReadBuildInfo(),
		// or "(devel)" when no release version is present.
		Raw string //goplint:ignore -- display/diagnostic build metadata string.
		// Normalized is the release version without a leading "v" or "+metadata"
		// suffix; empty when IsDev is true.
		Normalized string //goplint:ignore -- normalized version string from build metadata.
		// Parsed is the parsed semantic version when IsDev is false.
		Parsed *Version
		// IsDev is true when neither ldflags nor build info supplies a release version.
		IsDev bool
	}

	// UnsupportedInvowkVersionError is returned when an invowkmod.cue or
	// invowkfile.cue declares an invowk_version constraint that the running
	// Invowk binary does not satisfy.
	UnsupportedInvowkVersionError struct {
		FilePath       string //goplint:ignore -- parser display path context (may be empty or synthetic).
		Required       SemVerConstraint
		CurrentVersion string //goplint:ignore -- normalized running build version text.
	}
)

// Error implements the error interface for UnsupportedInvowkVersionError.
func (e *UnsupportedInvowkVersionError) Error() string {
	if e.FilePath == "" {
		return fmt.Sprintf("requires invowk version %q, but running version is %q; please upgrade invowk",
			e.Required, e.CurrentVersion)
	}
	return fmt.Sprintf("%s: requires invowk version %q, but running version is %q; please upgrade invowk",
		e.FilePath, e.Required, e.CurrentVersion)
}

// Unwrap returns ErrUnsupportedInvowkVersion for errors.Is() compatibility.
func (e *UnsupportedInvowkVersionError) Unwrap() error {
	return ErrUnsupportedInvowkVersion
}

// SetBuildVersion records the ldflags-injected Invowk version so parser
// pre-pass checks in pkg/invowkmod and pkg/invowkfile can evaluate invowk_version
// constraints against the running binary.
//
//goplint:ignore -- accepts raw ldflags version string at process startup boundary.
func SetBuildVersion(version string) {
	buildVersionOverride.Store(version)
}

// OverrideBuildVersionForTest temporarily overrides the active build version
// and returns a restore function for deterministic testing.
//
//goplint:ignore -- test hook accepts raw build version string.
func OverrideBuildVersionForTest(version string) func() {
	prev := currentBuildVersionRaw()
	buildVersionOverride.Store(version)
	return func() {
		buildVersionOverride.Store(prev)
	}
}

// CurrentBuildVersion resolves the active Invowk binary version from the
// configured ldflags version and debug.ReadBuildInfo().
func CurrentBuildVersion() BuildVersionInfo {
	return ResolveBuildVersion(currentBuildVersionRaw(), debug.ReadBuildInfo)
}

// ResolveBuildVersion resolves the Invowk binary version with precedence:
// ldflags version > debug.ReadBuildInfo().Main.Version > true (devel) fallback.
// Leading "v" prefixes and "+build" metadata are normalized for SemVer matching.
//
//goplint:ignore -- inspects raw ldflags and runtime/debug version strings.
func ResolveBuildVersion(ldflagsVersion string, readBuildInfo func() (*debug.BuildInfo, bool)) BuildVersionInfo {
	if ldflagsVersion != "" && ldflagsVersion != defaultDevBuildVersion {
		if normalized, parsed, ok := normalizeReleaseVersion(ldflagsVersion); ok {
			return BuildVersionInfo{
				Raw:        ldflagsVersion,
				Normalized: normalized,
				Parsed:     parsed,
			}
		}
		return BuildVersionInfo{
			Raw:   ldflagsVersion,
			IsDev: true,
		}
	}

	if readBuildInfo == nil {
		readBuildInfo = debug.ReadBuildInfo
	}
	if info, ok := readBuildInfo(); ok && info != nil && info.Main.Version != "" && info.Main.Version != develBuildInfoVersion {
		if normalized, parsed, parsedOK := normalizeReleaseVersion(info.Main.Version); parsedOK {
			return BuildVersionInfo{
				Raw:        info.Main.Version,
				Normalized: normalized,
				Parsed:     parsed,
			}
		}
		return BuildVersionInfo{
			Raw:   info.Main.Version,
			IsDev: true,
		}
	}

	return BuildVersionInfo{
		Raw:   develBuildInfoVersion,
		IsDev: true,
	}
}

// CheckInvowkVersionCUE runs a lightweight schema-less CUE pre-pass on data
// before strict close({...}) schema decoding so files with invowk_version
// constraints fail cleanly with UnsupportedInvowkVersionError even when they
// also contain schema fields introduced by newer versions of invowk.
//
//goplint:ignore -- CUE parser pre-pass operates on raw file bytes and filename display text.
func CheckInvowkVersionCUE(data []byte, path string) error {
	return checkInvowkVersionCUEWithVersion(data, path, CurrentBuildVersion())
}

// AnnotateFieldNotAllowedHint appends an upgrade hint whenever strict CUE
// schema validation fails with a "field not allowed" error.
func AnnotateFieldNotAllowedHint(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), fieldNotAllowedToken) {
		return fmt.Errorf("%w\n%s", err, fieldNotAllowedUpgradeHint)
	}
	return err
}

//goplint:ignore -- internal helper returning raw build version string.
func currentBuildVersionRaw() string {
	if v, ok := buildVersionOverride.Load().(string); ok && v != "" {
		return v
	}
	return defaultDevBuildVersion
}

//goplint:ignore -- normalizes raw build version strings for SemVer comparison.
func normalizeReleaseVersion(raw string) (string, *Version, bool) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "v")
	core, _, _ := strings.Cut(trimmed, "+")
	if !releaseSemverPattern.MatchString(core) {
		return "", nil, false
	}
	parsed, err := ParseVersion(core)
	if err != nil {
		return "", nil, false
	}
	return core, parsed, true
}

//goplint:ignore -- CUE pre-pass helper operates on raw file bytes and filename display text.
func checkInvowkVersionCUEWithVersion(data []byte, path string, buildVer BuildVersionInfo) error {
	if err := cueutil.CheckFileSize(data, cueutil.DefaultMaxFileSize, path); err != nil {
		return err
	}

	ctx := cuecontext.New()
	val := ctx.CompileBytes(data, cue.Filename(path))
	if val.Err() != nil {
		return cueutil.FormatError(val.Err(), path)
	}

	field := val.LookupPath(cue.MakePath(cue.Str("invowk_version")))
	if !field.Exists() {
		return nil
	}
	if field.Err() != nil {
		return cueutil.FormatError(field.Err(), path)
	}

	constraintStr, err := field.String()
	if err != nil {
		return cueutil.FormatError(err, path)
	}

	constraint := SemVerConstraint(constraintStr)
	if valErr := constraint.Validate(); valErr != nil {
		return fmt.Errorf("%s: invowk_version: %w", displayFilePath(path), valErr)
	}
	if valErr := ValidateDeclaredSemVerConstraint(constraint); valErr != nil {
		return fmt.Errorf("%s: invowk_version: %w", displayFilePath(path), valErr)
	}

	if buildVer.IsDev {
		slog.Warn("skipping invowk_version constraint check in development build",
			"constraint", constraintStr,
			"build_version", buildVer.Raw,
			"path", displayFilePath(path),
		)
		return nil
	}

	resolver := &SemverResolver{}
	parsedConstraint, err := resolver.ParseConstraint(constraintStr)
	if err != nil {
		return fmt.Errorf("%s: invowk_version: %w", displayFilePath(path), err)
	}
	if !parsedConstraint.Matches(buildVer.Parsed) {
		return &UnsupportedInvowkVersionError{
			FilePath:       path,
			Required:       constraint,
			CurrentVersion: buildVer.Normalized,
		}
	}
	return nil
}

//goplint:ignore -- fallback display name for empty parser path.
func displayFilePath(path string) string {
	if path == "" {
		return "<input>"
	}
	return path
}
