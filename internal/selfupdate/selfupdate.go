// SPDX-License-Identifier: MPL-2.0

package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"
)

// maxBinaryBytes is the upper bound on extracted binary size (500 MB).
// Prevents decompression bombs when extracting the invowk binary from
// a release archive.
const maxBinaryBytes = 500 << 20

var (
	// ErrInvalidVersion indicates the provided version string is not valid semver.
	ErrInvalidVersion = errors.New("invalid semantic version")

	//nolint:gochecknoglobals // Test seam for os.Executable().
	osExecutable = os.Executable

	//nolint:gochecknoglobals // Test seam for filepath.EvalSymlinks().
	evalSymlinks = filepath.EvalSymlinks
)

type (
	// UpgradeCheck holds the result of a version comparison between the currently
	// running binary and the latest (or target) GitHub release. The InstallMethod
	// field determines whether the Updater can apply the upgrade directly or must
	// defer to an external package manager.
	UpgradeCheck struct {
		CurrentVersion   string        // Currently running version
		LatestVersion    string        // Latest stable release version
		TargetRelease    *Release      // Full release info (nil if up-to-date, managed, or pre-release ahead)
		InstallMethod    InstallMethod // How invowk was installed
		UpgradeAvailable bool          // True if upgrade available and applicable
		Message          string        // Human-readable status message
	}

	// Updater composes the GitHub client, install method detection, and checksum
	// verification into an end-to-end upgrade flow. It is the primary facade for
	// the selfupdate package.
	Updater struct {
		client         *GitHubClient
		currentVersion string
	}

	// UpdaterOption configures an Updater during construction.
	UpdaterOption func(*Updater)
)

// WithGitHubClient overrides the default GitHubClient used by the Updater.
func WithGitHubClient(c *GitHubClient) UpdaterOption {
	return func(u *Updater) {
		u.client = c
	}
}

// NewUpdater creates an Updater for the given currentVersion. If no
// WithGitHubClient option is provided, a default GitHubClient is created.
func NewUpdater(currentVersion string, opts ...UpdaterOption) *Updater {
	u := &Updater{
		currentVersion: currentVersion,
	}
	for _, opt := range opts {
		opt(u)
	}
	if u.client == nil {
		u.client = NewGitHubClient()
	}
	return u
}

// Check determines whether an upgrade is available by comparing the current
// version against the latest stable release (or a specific targetVersion).
//
// For managed installs (Homebrew, go install), Check returns immediately with
// guidance to use the appropriate package manager — no GitHub API call is made.
// For unmanaged installs, it fetches release metadata from the GitHub API and
// performs a semver comparison.
func (u *Updater) Check(ctx context.Context, targetVersion string) (*UpgradeCheck, error) {
	execPath, err := resolveExecPath()
	if err != nil {
		return nil, fmt.Errorf("resolving executable path: %w", err)
	}

	method := DetectInstallMethod(execPath)

	// Managed installs should use their respective package managers.
	// Return guidance immediately without hitting the GitHub API.
	if method == InstallMethodHomebrew || method == InstallMethodGoInstall {
		return &UpgradeCheck{
			CurrentVersion:   u.currentVersion,
			InstallMethod:    method,
			UpgradeAvailable: false,
			Message:          managedInstallMessage(method, execPath),
		}, nil
	}

	// Resolve the target release from the GitHub API.
	var release *Release
	if targetVersion != "" {
		tag, tagErr := normalizeVersion(targetVersion)
		if tagErr != nil {
			return nil, tagErr
		}
		r, fetchErr := u.client.GetReleaseByTag(ctx, tag)
		if fetchErr != nil {
			return nil, fmt.Errorf("fetching release %s: %w", tag, fetchErr)
		}
		release = r
	} else {
		releases, listErr := u.client.ListReleases(ctx)
		if listErr != nil {
			return nil, fmt.Errorf("listing releases: %w", listErr)
		}
		if len(releases) == 0 {
			return nil, fmt.Errorf("no stable releases found")
		}
		// ListReleases returns results sorted by semver descending; the first
		// entry is the highest stable version.
		release = &releases[0]
	}

	currentNorm, err := normalizeVersion(u.currentVersion)
	if err != nil {
		return nil, fmt.Errorf("current version: %w", err)
	}
	targetNorm, err := normalizeVersion(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("release version: %w", err)
	}

	// Pre-release ahead: the running binary is a pre-release that is at or
	// beyond the target stable version. This happens during development and
	// CI — the user is already running a build newer than the latest release.
	if semver.Prerelease(currentNorm) != "" && semver.Compare(currentNorm, targetNorm) >= 0 {
		return &UpgradeCheck{
			CurrentVersion: u.currentVersion,
			LatestVersion:  release.TagName,
			InstallMethod:  method,
			Message:        fmt.Sprintf("Running pre-release %s (ahead of %s).", u.currentVersion, release.TagName),
		}, nil
	}

	// Already up-to-date: current version is equal to or newer than the target.
	if semver.Compare(currentNorm, targetNorm) >= 0 {
		return &UpgradeCheck{
			CurrentVersion: u.currentVersion,
			LatestVersion:  release.TagName,
			InstallMethod:  method,
			Message:        "Already up to date.",
		}, nil
	}

	return &UpgradeCheck{
		CurrentVersion:   u.currentVersion,
		LatestVersion:    release.TagName,
		TargetRelease:    release,
		InstallMethod:    method,
		UpgradeAvailable: true,
		Message:          fmt.Sprintf("Upgrade available: %s -> %s", u.currentVersion, release.TagName),
	}, nil
}

// Apply downloads, verifies, and atomically replaces the current binary with
// the version from the given release. The replacement uses os.Rename, which
// requires the temp file to reside on the same filesystem as the target — all
// temporary files are created in the same directory as the running binary.
func (u *Updater) Apply(ctx context.Context, release *Release) error {
	if release == nil {
		return errors.New("release must not be nil")
	}

	execPath, err := resolveExecPath()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	// Windows with unknown install method cannot do in-place replacement
	// because the OS locks the running binary. Guide the user to alternative
	// upgrade methods.
	method := DetectInstallMethod(execPath)
	if runtime.GOOS == "windows" && method == InstallMethodUnknown {
		return fmt.Errorf(
			"automatic upgrade is not supported on Windows for manual installations; " +
				"download the new version from the GitHub releases page or use: go install github.com/invowk/invowk@latest")
	}

	// Build the expected archive filename. GoReleaser strips the "v" prefix
	// from the version in filenames (e.g., invowk_1.0.0_linux_amd64.tar.gz).
	version := strings.TrimPrefix(release.TagName, "v")
	archiveName := fmt.Sprintf("invowk_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	checksumsName := "checksums.txt"

	archiveAsset, err := findAsset(release.Assets, archiveName)
	if err != nil {
		return fmt.Errorf("finding archive asset: %w", err)
	}

	checksumsAsset, err := findAsset(release.Assets, checksumsName)
	if err != nil {
		return fmt.Errorf("finding checksums asset: %w", err)
	}

	// Download and parse checksums.txt to obtain the expected hash for the
	// archive before downloading the (much larger) archive itself.
	checksumsBody, err := u.client.DownloadAsset(ctx, checksumsAsset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	defer func() { _ = checksumsBody.Close() }() // read-only HTTP response body

	entries, err := ParseChecksums(checksumsBody)
	if err != nil {
		return fmt.Errorf("parsing checksums: %w", err)
	}

	expectedHash, err := FindChecksum(entries, archiveName)
	if err != nil {
		return fmt.Errorf("finding checksum for %s: %w", archiveName, err)
	}

	// Download the archive to a temp file in the same directory as the target
	// binary so that the final os.Rename is an atomic same-filesystem move.
	targetDir := filepath.Dir(execPath)

	archivePath, err := downloadToTempFile(ctx, u.client, archiveAsset.BrowserDownloadURL, targetDir)
	if err != nil {
		return fmt.Errorf("downloading archive: %w", err)
	}
	defer func() { _ = os.Remove(archivePath) }()

	// Verify the downloaded archive against the expected SHA256 hash.
	err = VerifyFile(archivePath, expectedHash)
	if err != nil {
		return fmt.Errorf("verifying archive checksum: %w", err)
	}

	// Extract the invowk binary from the tar.gz archive into a temp file.
	tempBinaryPath, err := extractBinaryFromArchive(archivePath, targetDir)
	if err != nil {
		return fmt.Errorf("extracting binary from archive: %w", err)
	}

	// Track whether the rename succeeded so the deferred cleanup knows
	// whether to remove the temp binary.
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tempBinaryPath)
		}
	}()

	// Preserve the original binary's file permissions.
	info, err := os.Stat(execPath)
	if err != nil {
		return fmt.Errorf("reading original binary permissions: %w", err)
	}

	err = os.Chmod(tempBinaryPath, info.Mode())
	if err != nil {
		return fmt.Errorf("setting binary permissions: %w", err)
	}

	// Atomic replacement via rename. This requires the temp file to be on
	// the same filesystem as the target, which is guaranteed because both
	// reside in targetDir.
	err = os.Rename(tempBinaryPath, execPath)
	if err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}
	renamed = true

	return nil
}

// resolveExecPath returns the absolute, symlink-resolved path to the currently
// running binary.
func resolveExecPath() (string, error) {
	p, err := osExecutable()
	if err != nil {
		return "", fmt.Errorf("determining executable path: %w", err)
	}

	resolved, err := evalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks for %s: %w", p, err)
	}

	return resolved, nil
}

// findAsset scans the release assets for one with the given name. Returns
// ErrAssetNotFound (from checksum.go) if no match is found.
func findAsset(assets []Asset, name string) (*Asset, error) {
	for i := range assets {
		if assets[i].Name == name {
			return &assets[i], nil
		}
	}
	return nil, fmt.Errorf("asset %q not found in release: %w", name, ErrAssetNotFound)
}

// managedInstallMessage returns a human-readable message advising the user to
// upgrade via their package manager, formatted per the CLI contract.
func managedInstallMessage(method InstallMethod, execPath string) string {
	switch method {
	case InstallMethodHomebrew:
		return fmt.Sprintf("Detected Homebrew installation at %s\n\nTo upgrade, run:\n  brew upgrade invowk", execPath)
	case InstallMethodGoInstall:
		return fmt.Sprintf("Detected go install at %s\n\nTo upgrade, run:\n  go install github.com/invowk/invowk@latest", execPath)
	case InstallMethodScript, InstallMethodUnknown:
		return ""
	}
	return ""
}

// normalizeVersion ensures the version string has a "v" prefix as required by
// the semver package, and validates that the result is a well-formed semantic
// version. Returns ErrInvalidVersion if the input cannot be normalized to
// valid semver.
func normalizeVersion(v string) (string, error) {
	norm := v
	if !strings.HasPrefix(norm, "v") {
		norm = "v" + norm
	}
	if !semver.IsValid(norm) {
		return "", fmt.Errorf("%w: %q", ErrInvalidVersion, v)
	}
	return norm, nil
}

// downloadToTempFile downloads the asset at url into a temporary file in dir
// and returns the path to the temp file. The caller is responsible for removing
// the file when done.
func downloadToTempFile(ctx context.Context, client *GitHubClient, url, dir string) (_ string, err error) {
	body, err := client.DownloadAsset(ctx, url)
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }() // read-only HTTP response body

	tmp, err := os.CreateTemp(dir, "invowk-download-*")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer func() {
		if closeErr := tmp.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if _, err := io.Copy(tmp, body); err != nil {
		// Best-effort removal of partially written temp file.
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("writing to temp file: %w", err)
	}

	return tmp.Name(), nil
}

// extractBinaryFromArchive opens the tar.gz archive at archivePath and extracts
// the invowk binary into a temp file in targetDir. It matches by the base
// filename rather than the full entry path, so both flat archives (invowk at
// the root) and nested layouts (e.g., invowk_1.0.0_linux_amd64/invowk) are
// handled transparently.
func extractBinaryFromArchive(archivePath, targetDir string) (_ string, err error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("opening archive: %w", err)
	}
	defer func() {
		// Read-only file handle; close errors are exotic.
		_ = f.Close()
	}()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("creating gzip reader: %w", err)
	}
	defer func() {
		// Gzip reader wraps the underlying file; close errors are not
		// actionable here since we only read from it.
		_ = gz.Close()
	}()

	// Determine the expected binary name based on the platform.
	binaryName := "invowk"
	if runtime.GOOS == "windows" {
		binaryName = "invowk.exe"
	}

	tr := tar.NewReader(gz)
	for {
		hdr, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return "", fmt.Errorf("reading tar entry: %w", nextErr)
		}

		// Match by base name to handle both flat and nested archive layouts.
		if filepath.Base(hdr.Name) != binaryName {
			continue
		}

		// Found the binary entry — extract it to a temp file.
		// Limit the reader to maxBinaryBytes to prevent decompression bombs.
		tmp, createErr := os.CreateTemp(targetDir, "invowk-upgrade-*")
		if createErr != nil {
			return "", fmt.Errorf("creating temp file for binary: %w", createErr)
		}

		// Use a closure to handle the temp file lifecycle and report any
		// copy or close errors back to the caller.
		if copyErr := func() (copyErr error) {
			defer func() {
				if closeErr := tmp.Close(); closeErr != nil && copyErr == nil {
					copyErr = closeErr
				}
			}()
			if _, copyErr = io.Copy(tmp, io.LimitReader(tr, maxBinaryBytes)); copyErr != nil {
				return fmt.Errorf("extracting binary: %w", copyErr)
			}
			return nil
		}(); copyErr != nil {
			// Best-effort removal of partially written temp file.
			_ = os.Remove(tmp.Name())
			return "", copyErr
		}

		return tmp.Name(), nil
	}

	return "", fmt.Errorf("binary %q not found in archive %s", binaryName, archivePath)
}
