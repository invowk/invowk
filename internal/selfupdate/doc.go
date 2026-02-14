// SPDX-License-Identifier: MPL-2.0

// Package selfupdate implements self-upgrade functionality for the invowk CLI.
// It provides GitHub Releases API integration, install method detection,
// SHA256 checksum verification, and atomic binary replacement.
//
// The package is organized into four concerns:
//   - github.go: HTTP client for the GitHub Releases API (list, get-by-tag, download)
//   - detect.go: Install method detection (Script, Homebrew, GoInstall, Unknown)
//   - checksum.go: SHA256 checksum parsing and file verification
//   - selfupdate.go: Updater type that composes the above for end-to-end upgrade flow
package selfupdate
