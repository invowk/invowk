// SPDX-License-Identifier: MPL-2.0

package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

// createTestArchive builds a tar.gz archive containing a fake invowk binary
// nested inside a directory (e.g., invowk_1.0.0_linux_amd64/invowk), matching
// a nested archive layout.
func createTestArchive(t *testing.T, binaryContent []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// GoReleaser wraps archives in a directory named after the release.
	dirName := fmt.Sprintf("invowk_1.0.0_%s_%s", runtime.GOOS, runtime.GOARCH)
	hdr := &tar.Header{
		Name:     dirName + "/invowk",
		Mode:     0o755,
		Size:     int64(len(binaryContent)),
		Typeflag: tar.TypeReg,
	}

	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("writing tar header: %v", err)
	}
	if _, err := tw.Write(binaryContent); err != nil {
		t.Fatalf("writing tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("closing tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}

	return buf.Bytes()
}

// sha256Hex computes the lowercase hex-encoded SHA256 digest of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// newTestServer creates an httptest server that handles GitHub Releases API
// endpoints. It accepts a release list for the /releases endpoint and optional
// file handlers keyed by URL path suffix for asset downloads.
func newTestServer(t *testing.T, releases []githubRelease, files map[string][]byte) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle releases list.
		if strings.HasSuffix(r.URL.Path, "/releases") {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(releases); err != nil {
				t.Errorf("encoding releases: %v", err)
			}
			return
		}

		// Handle release by tag: /repos/invowk/invowk/releases/tags/{tag}
		if strings.Contains(r.URL.Path, "/releases/tags/") {
			tag := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			for _, rel := range releases {
				if rel.TagName == tag {
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(rel); err != nil {
						t.Errorf("encoding release: %v", err)
					}
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Not Found"}`)
			return
		}

		// Handle file downloads.
		for path, data := range files {
			if strings.HasSuffix(r.URL.Path, path) {
				w.Header().Set("Content-Type", "application/octet-stream")
				if _, err := w.Write(data); err != nil {
					t.Errorf("writing file response: %v", err)
				}
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"message":"Not Found","path":%q}`, r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	return srv
}

// overrideExecSeams saves and restores the osExecutable and evalSymlinks test
// seams, setting them to return the given path. The caller does not need to
// call t.Cleanup — it is registered automatically.
func overrideExecSeams(t *testing.T, path string) {
	t.Helper()

	origExec := osExecutable
	origSymlinks := evalSymlinks
	t.Cleanup(func() {
		osExecutable = origExec
		evalSymlinks = origSymlinks
	})

	osExecutable = func() (string, error) { return path, nil }
	evalSymlinks = func(p string) (string, error) { return p, nil }
}

// --- Tests ---

func TestUpdater_Check_UpgradeAvailable(t *testing.T) {
	// Not parallel: overrides package-level test seams (osExecutable, evalSymlinks,
	// installMethodHint, readBuildInfo).

	// Clear detection seams so the install method falls through to Unknown.
	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	overrideExecSeams(t, "/usr/local/bin/invowk")

	archiveName := fmt.Sprintf("invowk_1.1.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	releases := []githubRelease{
		{
			TagName:    "v1.1.0",
			Name:       "v1.1.0",
			Prerelease: false,
			Draft:      false,
			HTMLURL:    "https://github.com/invowk/invowk/releases/tag/v1.1.0",
			CreatedAt:  "2026-01-01T00:00:00Z",
			Assets: []githubAsset{
				{
					Name:               archiveName,
					BrowserDownloadURL: "http://example.com/download/" + archiveName,
					Size:               1000,
					ContentType:        "application/gzip",
				},
				{
					Name:               "checksums.txt",
					BrowserDownloadURL: "http://example.com/download/checksums.txt",
					Size:               200,
					ContentType:        "text/plain",
				},
			},
		},
	}

	srv := newTestServer(t, releases, nil)

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v1.0.0", WithGitHubClient(client))

	result, err := updater.Check(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.UpgradeAvailable {
		t.Error("expected UpgradeAvailable to be true")
	}
	if result.LatestVersion != "v1.1.0" {
		t.Errorf("expected LatestVersion %q, got %q", "v1.1.0", result.LatestVersion)
	}
	if result.TargetRelease == nil {
		t.Fatal("expected TargetRelease to be non-nil")
	}
	if result.TargetRelease.TagName != "v1.1.0" {
		t.Errorf("expected TargetRelease.TagName %q, got %q", "v1.1.0", result.TargetRelease.TagName)
	}
	if result.CurrentVersion != "v1.0.0" {
		t.Errorf("expected CurrentVersion %q, got %q", "v1.0.0", result.CurrentVersion)
	}
}

func TestUpdater_Check_UpToDate(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	overrideExecSeams(t, "/usr/local/bin/invowk")

	releases := []githubRelease{
		{
			TagName:    "v1.0.0",
			Name:       "v1.0.0",
			Prerelease: false,
			Draft:      false,
			HTMLURL:    "https://github.com/invowk/invowk/releases/tag/v1.0.0",
			CreatedAt:  "2026-01-01T00:00:00Z",
		},
	}

	srv := newTestServer(t, releases, nil)

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v1.0.0", WithGitHubClient(client))

	result, err := updater.Check(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.UpgradeAvailable {
		t.Error("expected UpgradeAvailable to be false")
	}
	if !strings.Contains(result.Message, "Already up to date") {
		t.Errorf("expected message to contain 'Already up to date', got %q", result.Message)
	}
	if result.TargetRelease != nil {
		t.Errorf("expected TargetRelease to be nil, got %+v", result.TargetRelease)
	}
}

func TestUpdater_Check_PreReleaseAhead(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	overrideExecSeams(t, "/usr/local/bin/invowk")

	releases := []githubRelease{
		{
			TagName:    "v1.0.0",
			Name:       "v1.0.0",
			Prerelease: false,
			Draft:      false,
			HTMLURL:    "https://github.com/invowk/invowk/releases/tag/v1.0.0",
			CreatedAt:  "2026-01-01T00:00:00Z",
		},
	}

	srv := newTestServer(t, releases, nil)

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v1.1.0-alpha.1", WithGitHubClient(client))

	result, err := updater.Check(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.UpgradeAvailable {
		t.Error("expected UpgradeAvailable to be false for pre-release ahead")
	}
	if !strings.Contains(strings.ToLower(result.Message), "pre-release") {
		t.Errorf("expected message to mention 'pre-release', got %q", result.Message)
	}
}

func TestUpdater_Check_HomebrewDetected(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	homebrewPath := "/opt/homebrew/Cellar/invowk/1.0.0/bin/invowk"
	overrideExecSeams(t, homebrewPath)

	// Set up a server that fails if any request is made — Homebrew detection
	// must short-circuit before any HTTP calls.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server was hit; Homebrew detection should have short-circuited")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v1.0.0", WithGitHubClient(client))

	result, err := updater.Check(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.InstallMethod != InstallMethodHomebrew {
		t.Errorf("expected InstallMethodHomebrew, got %v", result.InstallMethod)
	}
	if !strings.Contains(result.Message, "brew upgrade") {
		t.Errorf("expected message to contain 'brew upgrade', got %q", result.Message)
	}
	if result.UpgradeAvailable {
		t.Error("expected UpgradeAvailable to be false for Homebrew install")
	}
}

func TestUpdater_Check_GoInstallDetected(t *testing.T) {
	// Not parallel: overrides package-level test seams and uses t.Setenv.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""

	// Set up a fake GOPATH and exec path inside its bin directory.
	gopath := t.TempDir()
	execPath := filepath.Join(gopath, "bin", "invowk")

	overrideExecSeams(t, execPath)
	t.Setenv("GOPATH", gopath)

	// Mock readBuildInfo to confirm the module path matches invowk.
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Path: "github.com/invowk/invowk",
		}, true
	}

	// Set up a server that fails if any request is made — GoInstall detection
	// must short-circuit before any HTTP calls.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("server was hit; GoInstall detection should have short-circuited")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v1.0.0", WithGitHubClient(client))

	result, err := updater.Check(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.InstallMethod != InstallMethodGoInstall {
		t.Errorf("expected InstallMethodGoInstall, got %v", result.InstallMethod)
	}
	if !strings.Contains(result.Message, "go install") {
		t.Errorf("expected message to contain 'go install', got %q", result.Message)
	}
	if result.UpgradeAvailable {
		t.Error("expected UpgradeAvailable to be false for GoInstall install")
	}
}

func TestUpdater_Apply_Success(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	// 1. Create a fake binary to act as the "current" invowk binary.
	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "invowk")
	originalContent := []byte("original-binary-content")
	if err := os.WriteFile(fakeBinary, originalContent, 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}

	overrideExecSeams(t, fakeBinary)

	// 2. Create a tar.gz archive with a new "binary" inside.
	newBinaryContent := []byte("#!/bin/sh\necho hello-upgraded")
	archiveData := createTestArchive(t, newBinaryContent)
	archiveHash := sha256Hex(archiveData)

	// 3. Build the checksums.txt content with the correct hash.
	archiveName := fmt.Sprintf("invowk_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	checksumsContent := fmt.Sprintf("%s  %s\n", archiveHash, archiveName)

	// 4. Set up httptest server serving both the checksums and the archive.
	files := map[string][]byte{
		"/download/checksums.txt":  []byte(checksumsContent),
		"/download/" + archiveName: archiveData,
	}

	srv := newTestServer(t, nil, files)

	// 5. Build a Release with assets pointing to the test server.
	release := &Release{
		TagName: "v1.0.0",
		Name:    "v1.0.0",
		Assets: []Asset{
			{
				Name:               archiveName,
				BrowserDownloadURL: srv.URL + "/download/" + archiveName,
				Size:               int64(len(archiveData)),
				ContentType:        "application/gzip",
			},
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: srv.URL + "/download/checksums.txt",
				Size:               int64(len(checksumsContent)),
				ContentType:        "text/plain",
			},
		},
	}

	// 6. Run Apply.
	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v0.9.0", WithGitHubClient(client))

	if err := updater.Apply(context.Background(), release); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// 7. Verify the binary was replaced with the new content.
	replaced, err := os.ReadFile(fakeBinary)
	if err != nil {
		t.Fatalf("reading replaced binary: %v", err)
	}

	if !bytes.Equal(replaced, newBinaryContent) {
		t.Errorf("binary content mismatch:\ngot:  %q\nwant: %q", replaced, newBinaryContent)
	}
}

func TestUpdater_Apply_ChecksumMismatch(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	// Create a fake binary.
	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "invowk")
	originalContent := []byte("original-binary-content")
	if err := os.WriteFile(fakeBinary, originalContent, 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}

	overrideExecSeams(t, fakeBinary)

	// Create an archive.
	archiveData := createTestArchive(t, []byte("new binary content"))
	archiveName := fmt.Sprintf("invowk_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	// Use a WRONG hash in checksums.txt.
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	checksumsContent := fmt.Sprintf("%s  %s\n", wrongHash, archiveName)

	files := map[string][]byte{
		"/download/checksums.txt":  []byte(checksumsContent),
		"/download/" + archiveName: archiveData,
	}

	srv := newTestServer(t, nil, files)

	release := &Release{
		TagName: "v1.0.0",
		Name:    "v1.0.0",
		Assets: []Asset{
			{
				Name:               archiveName,
				BrowserDownloadURL: srv.URL + "/download/" + archiveName,
				Size:               int64(len(archiveData)),
				ContentType:        "application/gzip",
			},
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: srv.URL + "/download/checksums.txt",
				Size:               int64(len(checksumsContent)),
				ContentType:        "text/plain",
			},
		},
	}

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v0.9.0", WithGitHubClient(client))

	err := updater.Apply(context.Background(), release)
	if err == nil {
		t.Fatal("expected error for checksum mismatch, got nil")
	}

	// Verify the error wraps ErrChecksumMismatch.
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("expected error to wrap ErrChecksumMismatch, got: %v", err)
	}

	// Verify the original binary was NOT replaced.
	content, readErr := os.ReadFile(fakeBinary)
	if readErr != nil {
		t.Fatalf("reading binary after failed apply: %v", readErr)
	}
	if !bytes.Equal(content, originalContent) {
		t.Error("original binary was modified despite checksum mismatch — rollback failed")
	}
}

func TestUpdater_Apply_PermissionError(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	// Create a fake binary in a directory, then make the directory read-only
	// so temp file creation fails.
	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(fakeBinary, []byte("binary"), 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}

	overrideExecSeams(t, fakeBinary)

	// Make the directory read-only to prevent temp file creation.
	if err := os.Chmod(tmpDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() {
		// Restore permissions so t.TempDir() cleanup can remove the directory.
		_ = os.Chmod(tmpDir, 0o755)
	})

	archiveName := fmt.Sprintf("invowk_1.0.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	archiveData := createTestArchive(t, []byte("new binary"))
	archiveHash := sha256Hex(archiveData)
	checksumsContent := fmt.Sprintf("%s  %s\n", archiveHash, archiveName)

	files := map[string][]byte{
		"/download/checksums.txt":  []byte(checksumsContent),
		"/download/" + archiveName: archiveData,
	}

	srv := newTestServer(t, nil, files)

	release := &Release{
		TagName: "v1.0.0",
		Name:    "v1.0.0",
		Assets: []Asset{
			{
				Name:               archiveName,
				BrowserDownloadURL: srv.URL + "/download/" + archiveName,
				Size:               int64(len(archiveData)),
				ContentType:        "application/gzip",
			},
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: srv.URL + "/download/checksums.txt",
				Size:               int64(len(checksumsContent)),
				ContentType:        "text/plain",
			},
		},
	}

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v0.9.0", WithGitHubClient(client))

	err := updater.Apply(context.Background(), release)
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}

	// The error should indicate a permission issue (either creating temp file
	// or writing to the directory).
	errMsg := err.Error()
	if !strings.Contains(errMsg, "permission denied") && !strings.Contains(errMsg, "read-only") {
		t.Logf("note: error message does not literally contain 'permission denied', but got: %v", err)
	}

	// The important invariant is that Apply returned an error, which it did.
}

func TestUpdater_Check_SpecificVersion(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	overrideExecSeams(t, "/usr/local/bin/invowk")

	archiveName := fmt.Sprintf("invowk_1.0.5_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	releases := []githubRelease{
		{
			TagName:    "v1.0.5",
			Name:       "v1.0.5",
			Prerelease: false,
			Draft:      false,
			HTMLURL:    "https://github.com/invowk/invowk/releases/tag/v1.0.5",
			CreatedAt:  "2026-01-15T00:00:00Z",
			Assets: []githubAsset{
				{
					Name:               archiveName,
					BrowserDownloadURL: "http://example.com/download/" + archiveName,
					Size:               1000,
					ContentType:        "application/gzip",
				},
				{
					Name:               "checksums.txt",
					BrowserDownloadURL: "http://example.com/download/checksums.txt",
					Size:               200,
					ContentType:        "text/plain",
				},
			},
		},
	}

	srv := newTestServer(t, releases, nil)

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v1.0.0", WithGitHubClient(client))

	result, err := updater.Check(context.Background(), "v1.0.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.UpgradeAvailable {
		t.Error("expected UpgradeAvailable to be true")
	}
	if result.LatestVersion != "v1.0.5" {
		t.Errorf("expected LatestVersion %q, got %q", "v1.0.5", result.LatestVersion)
	}
	if result.TargetRelease == nil {
		t.Fatal("expected TargetRelease to be non-nil")
	}
}

func TestUpdater_Check_VersionNormalization(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	overrideExecSeams(t, "/usr/local/bin/invowk")

	releases := []githubRelease{
		{
			TagName:    "v1.0.5",
			Name:       "v1.0.5",
			Prerelease: false,
			Draft:      false,
			CreatedAt:  "2026-01-15T00:00:00Z",
		},
	}

	srv := newTestServer(t, releases, nil)

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v1.0.0", WithGitHubClient(client))

	// Pass version WITHOUT "v" prefix — Check should normalize it.
	result, err := updater.Check(context.Background(), "1.0.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.UpgradeAvailable {
		t.Error("expected UpgradeAvailable to be true after normalization")
	}
	if result.LatestVersion != "v1.0.5" {
		t.Errorf("expected LatestVersion %q, got %q", "v1.0.5", result.LatestVersion)
	}
}

func TestNewUpdater_DefaultClient(t *testing.T) {
	t.Parallel()

	updater := NewUpdater("v1.0.0")

	if updater.client == nil {
		t.Fatal("expected default client to be created, got nil")
	}
	if updater.currentVersion != "v1.0.0" {
		t.Errorf("expected currentVersion %q, got %q", "v1.0.0", updater.currentVersion)
	}
}

func TestUpdater_Check_InvalidVersion(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	overrideExecSeams(t, "/usr/local/bin/invowk")

	releases := []githubRelease{
		{TagName: "v1.0.0", Name: "v1.0.0", Draft: false, Prerelease: false},
	}
	srv := newTestServer(t, releases, nil)

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v1.0.0", WithGitHubClient(client))

	// Pass an invalid version string — should return ErrInvalidVersion.
	_, err := updater.Check(context.Background(), "banana")
	if err == nil {
		t.Fatal("expected error for invalid version, got nil")
	}
	if !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("expected ErrInvalidVersion, got: %v", err)
	}
}

func TestUpdater_Apply_NilRelease(t *testing.T) {
	t.Parallel()

	updater := NewUpdater("v1.0.0")

	err := updater.Apply(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil release, got nil")
	}
	if !strings.Contains(err.Error(), "release must not be nil") {
		t.Errorf("expected 'release must not be nil' error, got: %v", err)
	}
}

func TestUpdater_Check_NoStableReleases(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	overrideExecSeams(t, "/usr/local/bin/invowk")

	// Server returns only drafts and prereleases — no stable releases.
	releases := []githubRelease{
		{TagName: "v2.0.0-alpha.1", Name: "Alpha", Draft: false, Prerelease: true},
		{TagName: "v2.0.0", Name: "Draft", Draft: true, Prerelease: false},
	}
	srv := newTestServer(t, releases, nil)

	client := NewGitHubClient(WithBaseURL(srv.URL))
	updater := NewUpdater("v1.0.0", WithGitHubClient(client))

	_, err := updater.Check(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for no stable releases, got nil")
	}
	if !strings.Contains(err.Error(), "no stable releases found") {
		t.Errorf("expected 'no stable releases found' error, got: %v", err)
	}
}

func TestUpdater_Apply_MissingPlatformAsset(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	savedHint := installMethodHint
	savedReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		installMethodHint = savedHint
		readBuildInfo = savedReadBuildInfo
	})
	installMethodHint = ""
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }

	tmpDir := t.TempDir()
	fakeBinary := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(fakeBinary, []byte("binary"), 0o755); err != nil {
		t.Fatalf("writing fake binary: %v", err)
	}
	overrideExecSeams(t, fakeBinary)

	// Release has assets for a different platform — no match for current OS/arch.
	release := &Release{
		TagName: "v1.0.0",
		Name:    "v1.0.0",
		Assets: []Asset{
			{
				Name:               "invowk_1.0.0_freebsd_riscv64.tar.gz",
				BrowserDownloadURL: "http://example.com/download/invowk_1.0.0_freebsd_riscv64.tar.gz",
			},
			{
				Name:               "checksums.txt",
				BrowserDownloadURL: "http://example.com/download/checksums.txt",
			},
		},
	}

	client := NewGitHubClient()
	updater := NewUpdater("v0.9.0", WithGitHubClient(client))

	err := updater.Apply(context.Background(), release)
	if err == nil {
		t.Fatal("expected error for missing platform asset, got nil")
	}
	if !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("expected ErrAssetNotFound, got: %v", err)
	}
}

// createFlatTestArchive builds a tar.gz archive with the invowk binary at the
// root level (no enclosing directory), matching a flat archive layout.
func createFlatTestArchive(t *testing.T, binaryContent []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     "invowk",
		Mode:     0o755,
		Size:     int64(len(binaryContent)),
		Typeflag: tar.TypeReg,
	}

	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("writing tar header: %v", err)
	}
	if _, err := tw.Write(binaryContent); err != nil {
		t.Fatalf("writing tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("closing tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}

	return buf.Bytes()
}

func TestExtractBinaryFromArchive_FlatArchive(t *testing.T) {
	t.Parallel()

	binaryContent := []byte("#!/bin/sh\necho flat-binary")
	archiveData := createFlatTestArchive(t, binaryContent)

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatalf("writing archive: %v", err)
	}

	extractedPath, err := extractBinaryFromArchive(archivePath, tmpDir)
	if err != nil {
		t.Fatalf("extracting binary: %v", err)
	}
	defer func() { _ = os.Remove(extractedPath) }()

	got, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}
	if !bytes.Equal(got, binaryContent) {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", got, binaryContent)
	}
}

func TestExtractBinaryFromArchive_NoBinaryFound(t *testing.T) {
	t.Parallel()

	// Create an archive with a different filename — no "invowk" entry.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:     "not-invowk",
		Mode:     0o755,
		Size:     5,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("writing tar header: %v", err)
	}
	if _, err := tw.Write([]byte("hello")); err != nil {
		t.Fatalf("writing tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("closing tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("writing archive: %v", err)
	}

	_, err := extractBinaryFromArchive(archivePath, tmpDir)
	if err == nil {
		t.Fatal("expected error when binary not found in archive, got nil")
	}
	if !strings.Contains(err.Error(), "not found in archive") {
		t.Errorf("expected 'not found in archive' error, got: %v", err)
	}
}

func TestExtractBinaryFromArchive_NestedArchive(t *testing.T) {
	t.Parallel()

	binaryContent := []byte("#!/bin/sh\necho nested-binary")
	archiveData := createTestArchive(t, binaryContent)

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		t.Fatalf("writing archive: %v", err)
	}

	extractedPath, err := extractBinaryFromArchive(archivePath, tmpDir)
	if err != nil {
		t.Fatalf("extracting binary: %v", err)
	}
	defer func() { _ = os.Remove(extractedPath) }()

	got, err := os.ReadFile(extractedPath)
	if err != nil {
		t.Fatalf("reading extracted binary: %v", err)
	}
	if !bytes.Equal(got, binaryContent) {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", got, binaryContent)
	}
}

func TestResolveExecPath_OsExecutableError(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	origExec := osExecutable
	origSymlinks := evalSymlinks
	t.Cleanup(func() {
		osExecutable = origExec
		evalSymlinks = origSymlinks
	})

	osExecutable = func() (string, error) {
		return "", fmt.Errorf("injected os.Executable error")
	}
	evalSymlinks = func(p string) (string, error) { return p, nil }

	_, err := resolveExecPath()
	if err == nil {
		t.Fatal("expected error from os.Executable failure, got nil")
	}
	if !strings.Contains(err.Error(), "determining executable path") {
		t.Errorf("expected 'determining executable path' context, got: %v", err)
	}
}

func TestResolveExecPath_EvalSymlinksError(t *testing.T) {
	// Not parallel: overrides package-level test seams.

	origExec := osExecutable
	origSymlinks := evalSymlinks
	t.Cleanup(func() {
		osExecutable = origExec
		evalSymlinks = origSymlinks
	})

	osExecutable = func() (string, error) { return "/usr/local/bin/invowk", nil }
	evalSymlinks = func(_ string) (string, error) {
		return "", fmt.Errorf("injected EvalSymlinks error")
	}

	_, err := resolveExecPath()
	if err == nil {
		t.Fatal("expected error from EvalSymlinks failure, got nil")
	}
	if !strings.Contains(err.Error(), "resolving symlinks") {
		t.Errorf("expected 'resolving symlinks' context, got: %v", err)
	}
}
