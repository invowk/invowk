#!/bin/sh
# SPDX-License-Identifier: MPL-2.0
#
# Install script for invowk — a dynamically extensible command runner.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.sh | sh
#
# Environment variables:
#   INVOWK_VERSION  - Specific version to install (e.g., v1.0.0). Default: latest stable.
#   INSTALL_DIR     - Installation directory. Default: ~/.local/bin.
#   GITHUB_TOKEN    - GitHub API token for higher rate limits (optional).
#
# Requirements:
#   - curl or wget
#   - sha256sum (Linux) or shasum (macOS)
#   - tar, gzip
#
# Supported platforms:
#   - Linux (amd64, arm64)
#   - macOS (amd64, arm64)

set -eu

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

GITHUB_REPO="invowk/invowk"
RELEASES_API_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
RELEASES_TAG_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/tags"
DOWNLOAD_BASE_URL="https://github.com/${GITHUB_REPO}/releases/download"
BINARY_NAME="invowk"

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

# Color output when stdout is a terminal.
setup_colors() {
    if [ -t 1 ] && [ -t 2 ]; then
        RED='\033[0;31m'
        GREEN='\033[0;32m'
        YELLOW='\033[0;33m'
        CYAN='\033[0;36m'
        BOLD='\033[1m'
        RESET='\033[0m'
    else
        RED='' GREEN='' YELLOW='' CYAN='' BOLD='' RESET=''
    fi
}

log()  { printf '%b\n' "${CYAN}$*${RESET}" >&2; }
ok()   { printf '%b\n' "${GREEN}$*${RESET}" >&2; }
warn() { printf '%b\n' "${YELLOW}WARNING: $*${RESET}" >&2; }
err()  { printf '%b\n' "${RED}ERROR: $*${RESET}" >&2; }
die()  { err "$@"; exit 1; }

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------

# Temp directory for staging downloads. Set during install flow, cleaned on exit.
TMPDIR_INSTALL=""

cleanup() {
    if [ -n "${TMPDIR_INSTALL}" ] && [ -d "${TMPDIR_INSTALL}" ]; then
        rm -rf "${TMPDIR_INSTALL}"
    fi
}

# ---------------------------------------------------------------------------
# Platform detection
# ---------------------------------------------------------------------------

# Detect OS and normalize to Go naming convention (linux, darwin).
detect_os() {
    _os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "${_os}" in
        linux)  echo "linux" ;;
        darwin) echo "darwin" ;;
        mingw*|msys*|cygwin*|windows*)
            die "Windows is not supported by this installer.
Please use the PowerShell installer or one of these alternatives:
  irm https://raw.githubusercontent.com/${GITHUB_REPO}/main/scripts/install.ps1 | iex
  go install github.com/${GITHUB_REPO}@latest
  Download from https://github.com/${GITHUB_REPO}/releases"
            ;;
        freebsd|openbsd|netbsd)
            die "${_os} is not supported. Supported platforms: Linux, macOS.
Consider building from source: go install github.com/${GITHUB_REPO}@latest"
            ;;
        *)
            die "Unsupported operating system: ${_os}"
            ;;
    esac
}

# Detect architecture and normalize to Go naming convention (amd64, arm64).
detect_arch() {
    _arch=$(uname -m)
    case "${_arch}" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        armv7*|armv6*)
            die "32-bit ARM is not supported. Supported architectures: amd64 (x86_64), arm64 (aarch64)."
            ;;
        i386|i686)
            die "32-bit x86 is not supported. Supported architectures: amd64 (x86_64), arm64 (aarch64)."
            ;;
        *)
            die "Unsupported architecture: ${_arch}"
            ;;
    esac
}

# ---------------------------------------------------------------------------
# Download tool detection
# ---------------------------------------------------------------------------

# Detect available download tool (curl or wget) and set DOWNLOAD_CMD.
detect_download_tool() {
    if command -v curl >/dev/null 2>&1; then
        DOWNLOAD_CMD="curl"
    elif command -v wget >/dev/null 2>&1; then
        DOWNLOAD_CMD="wget"
    else
        die "Neither curl nor wget found. Please install one of them and try again."
    fi
}

# Download a URL to stdout.
download() {
    _url="$1"
    case "${DOWNLOAD_CMD}" in
        curl)
            if [ -n "${GITHUB_TOKEN:-}" ]; then
                curl -fsSL -H "Authorization: token ${GITHUB_TOKEN}" "${_url}"
            else
                curl -fsSL "${_url}"
            fi
            ;;
        wget)
            if [ -n "${GITHUB_TOKEN:-}" ]; then
                wget -qO- --header="Authorization: token ${GITHUB_TOKEN}" "${_url}"
            else
                wget -qO- "${_url}"
            fi
            ;;
        *) die "Internal error: unsupported download command: ${DOWNLOAD_CMD}" ;;
    esac
}

# Download a URL to a file.
download_to_file() {
    _url="$1"
    _dest="$2"
    case "${DOWNLOAD_CMD}" in
        curl)
            if [ -n "${GITHUB_TOKEN:-}" ]; then
                curl -fsSL -H "Authorization: token ${GITHUB_TOKEN}" -o "${_dest}" "${_url}"
            else
                curl -fsSL -o "${_dest}" "${_url}"
            fi
            ;;
        wget)
            if [ -n "${GITHUB_TOKEN:-}" ]; then
                wget -qO "${_dest}" --header="Authorization: token ${GITHUB_TOKEN}" "${_url}"
            else
                wget -qO "${_dest}" "${_url}"
            fi
            ;;
        *) die "Internal error: unsupported download command: ${DOWNLOAD_CMD}" ;;
    esac
}

# ---------------------------------------------------------------------------
# Checksum verification
# ---------------------------------------------------------------------------

# Detect available SHA256 tool and set SHA256_CMD.
detect_sha256_tool() {
    if command -v sha256sum >/dev/null 2>&1; then
        SHA256_CMD="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
        SHA256_CMD="shasum"
    else
        die "Neither sha256sum nor shasum found. Cannot verify download integrity."
    fi
}

# Compute SHA256 hash of a file and print the hex digest.
# Captures the full output first to avoid pipe masking the hash tool's exit code
# (POSIX sh does not support pipefail).
sha256_file() {
    _file="$1"
    case "${SHA256_CMD}" in
        sha256sum)
            _output=$(sha256sum "${_file}") || die "sha256sum failed for ${_file}"
            ;;
        shasum)
            _output=$(shasum -a 256 "${_file}") || die "shasum failed for ${_file}"
            ;;
    esac
    _hash=$(printf '%s' "${_output}" | cut -d' ' -f1)
    if [ -z "${_hash}" ]; then
        die "Failed to extract hash from ${SHA256_CMD} output for ${_file}"
    fi
    printf '%s' "${_hash}"
}

# Verify a file's SHA256 hash against an expected value.
verify_checksum() {
    _file="$1"
    _expected="$2"
    _actual=$(sha256_file "${_file}")
    if [ "${_actual}" != "${_expected}" ]; then
        die "Checksum verification failed for $(basename "${_file}")

Expected: ${_expected}
Got:      ${_actual}

The download may be corrupted. Please try again.
If this persists, report at https://github.com/${GITHUB_REPO}/issues"
    fi
}

# ---------------------------------------------------------------------------
# Version resolution
# ---------------------------------------------------------------------------

# Resolve the target version. Uses INVOWK_VERSION if set, otherwise queries
# the GitHub API for the latest stable release.
resolve_version() {
    if [ -n "${INVOWK_VERSION:-}" ]; then
        # User-specified version — validate format and verify it exists.
        case "${INVOWK_VERSION}" in
            v*) ;; # Already has v prefix
            *)  INVOWK_VERSION="v${INVOWK_VERSION}" ;; # Add v prefix
        esac
        log "Using specified version: ${BOLD}${INVOWK_VERSION}${RESET}"

        # Verify the release exists by checking the tag endpoint.
        if ! download "${RELEASES_TAG_URL}/${INVOWK_VERSION}" >/dev/null 2>&1; then
            die "Version ${INVOWK_VERSION} not found.
Check available versions at: https://github.com/${GITHUB_REPO}/releases"
        fi
        echo "${INVOWK_VERSION}"
    else
        # Query the latest stable release via the GitHub API.
        log "Fetching latest stable version..."
        _response=$(download "${RELEASES_API_URL}") || die "Failed to fetch latest release from GitHub.

Check your network connection and try again.
If behind a firewall, you can specify a version directly:
  INVOWK_VERSION=v1.0.0 sh -c '\$(curl -fsSL https://raw.githubusercontent.com/${GITHUB_REPO}/main/scripts/install.sh)'"

        # Extract tag_name from JSON response using grep+sed instead of jq to
        # minimize runtime dependencies — this script must work on minimal systems.
        _version=$(printf '%s' "${_response}" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
        if [ -z "${_version}" ]; then
            # Check for API error message (e.g., rate limiting returns HTTP 200 with error body).
            _api_message=$(printf '%s' "${_response}" | grep '"message"' | head -1 | sed 's/.*"message"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
            if [ -n "${_api_message}" ]; then
                die "GitHub API error: ${_api_message}

This often happens due to API rate limiting for unauthenticated requests.
Try again in a few minutes, or specify a version directly:
  INVOWK_VERSION=v1.0.0 sh -c '\$(curl -fsSL https://raw.githubusercontent.com/${GITHUB_REPO}/main/scripts/install.sh)'"
            fi
            die "Could not determine latest version from GitHub API response."
        fi
        # Validate the extracted version looks like a semver tag.
        case "${_version}" in
            v[0-9]*)
                ;; # Valid version format
            *)
                die "Unexpected version format from GitHub API: ${_version}
This may indicate an API change. Report at: https://github.com/${GITHUB_REPO}/issues"
                ;;
        esac
        log "Latest stable version: ${BOLD}${_version}${RESET}"
        echo "${_version}"
    fi
}

# ---------------------------------------------------------------------------
# Installation
# ---------------------------------------------------------------------------

# Construct the asset filename from version, OS, and architecture.
# GoReleaser convention: version in filename strips the 'v' prefix.
asset_filename() {
    _version="$1"
    _os="$2"
    _arch="$3"
    _version_no_v=$(echo "${_version}" | sed 's/^v//')
    echo "${BINARY_NAME}_${_version_no_v}_${_os}_${_arch}.tar.gz"
}

# Check if a directory is in the user's PATH.
is_in_path() {
    _dir="$1"
    case ":${PATH}:" in
        *":${_dir}:"*) return 0 ;;
        *)             return 1 ;;
    esac
}

# Detect the user's shell configuration file for PATH instructions.
detect_shell_config() {
    _shell_name=$(basename "${SHELL:-/bin/sh}")
    case "${_shell_name}" in
        zsh)  echo "${HOME}/.zshrc" ;;
        bash)
            if [ -f "${HOME}/.bashrc" ]; then
                echo "${HOME}/.bashrc"
            else
                echo "${HOME}/.bash_profile"
            fi
            ;;
        fish) echo "${HOME}/.config/fish/config.fish" ;;
        *)    echo "${HOME}/.profile" ;;
    esac
}

# Perform the installation: download, verify, extract, install.
install() {
    _version="$1"
    _os="$2"
    _arch="$3"
    _install_dir="$4"

    _asset=$(asset_filename "${_version}" "${_os}" "${_arch}")
    _download_url="${DOWNLOAD_BASE_URL}/${_version}/${_asset}"
    _checksums_url="${DOWNLOAD_BASE_URL}/${_version}/checksums.txt"

    # Create temp directory for staging.
    TMPDIR_INSTALL=$(mktemp -d) || die "Failed to create temporary directory."

    _archive_path="${TMPDIR_INSTALL}/${_asset}"
    _checksums_path="${TMPDIR_INSTALL}/checksums.txt"

    # Download the archive and checksums.
    log "Downloading ${BOLD}${BINARY_NAME} ${_version}${RESET} for ${_os}/${_arch}..."
    download_to_file "${_download_url}" "${_archive_path}" || die "Failed to download ${_asset}.

The release asset may not exist for your platform (${_os}/${_arch}).
Check available assets at: https://github.com/${GITHUB_REPO}/releases/tag/${_version}"

    # Note: checksums.txt is not signature-verified by this script because cosign
    # is not a standard system tool. For supply-chain verification, see:
    # https://github.com/invowk/invowk#verifying-signatures
    #
    # The checksum file is downloaded to a mktemp directory (mode 0700), which
    # mitigates TOCTOU race conditions between download and verification.
    log "Downloading checksums..."
    download_to_file "${_checksums_url}" "${_checksums_path}" || die "Failed to download checksums.txt.

Cannot verify download integrity. Please try again."

    # Extract expected checksum for our asset from checksums.txt.
    # -F: fixed-string match (asset names contain dots, which are regex wildcards).
    _expected_hash=$(grep -F "${_asset}" "${_checksums_path}" | cut -d' ' -f1)
    if [ -z "${_expected_hash}" ]; then
        die "Asset ${_asset} not found in checksums.txt.
This may indicate a GoReleaser configuration issue.
Report at: https://github.com/${GITHUB_REPO}/issues"
    fi

    # Verify the archive checksum.
    log "Verifying checksum..."
    verify_checksum "${_archive_path}" "${_expected_hash}"
    ok "Checksum verified."

    # Extract the binary from the archive using a two-phase strategy:
    # 1. Try extracting the binary by exact name (flat archive layout).
    #    Stderr is captured (not suppressed) because some tar implementations
    #    print a "not found in archive" message when the member doesn't exist at
    #    the top level, which is expected for nested layouts.
    # 2. Fall back to extracting the entire archive (nested directory layout),
    #    then locate the binary below.
    # If both attempts fail, the captured stderr from the first attempt is
    # included in the error message for debugging.
    log "Extracting binary..."
    _tar_stderr=$(tar -xzf "${_archive_path}" -C "${TMPDIR_INSTALL}" "${BINARY_NAME}" 2>&1) ||
        tar -xzf "${_archive_path}" -C "${TMPDIR_INSTALL}" || die "Failed to extract ${_asset}.${_tar_stderr:+
First attempt error: ${_tar_stderr}}"

    if [ ! -f "${TMPDIR_INSTALL}/${BINARY_NAME}" ]; then
        die "Binary '${BINARY_NAME}' not found in archive ${_asset}."
    fi

    # Ensure the binary is executable.
    chmod +x "${TMPDIR_INSTALL}/${BINARY_NAME}"

    # Create install directory if it doesn't exist.
    mkdir -p "${_install_dir}" || die "Failed to create install directory: ${_install_dir}
Try running with a different INSTALL_DIR or check permissions."

    # Install via mv. When source and destination are on the same filesystem,
    # mv is atomic (rename syscall). For cross-filesystem moves, mv falls back
    # to copy+delete, which is NOT atomic but is the best available option.
    mv "${TMPDIR_INSTALL}/${BINARY_NAME}" "${_install_dir}/${BINARY_NAME}" || die "Failed to install binary to ${_install_dir}/${BINARY_NAME}

If permission denied, try:
  INSTALL_DIR=~/.local/bin sh -c '\$(curl -fsSL https://raw.githubusercontent.com/${GITHUB_REPO}/main/scripts/install.sh)'
  or: sudo INSTALL_DIR=/usr/local/bin sh -c '\$(curl -fsSL https://raw.githubusercontent.com/${GITHUB_REPO}/main/scripts/install.sh)'"

    ok "Successfully installed ${BOLD}${BINARY_NAME} ${_version}${RESET} to ${_install_dir}/${BINARY_NAME}"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

# main() wrapping prevents partial execution when the script is piped from
# curl/wget. The shell must receive the entire function definition before it
# can call main, so a truncated download produces a parse error rather than
# executing partial code.
main() {
    setup_colors

    log "Installing ${BOLD}${BINARY_NAME}${RESET}..."
    log ""

    # Detect platform and tools.
    _os=$(detect_os)
    _arch=$(detect_arch)
    detect_download_tool
    detect_sha256_tool

    # Resolve installation directory.
    _install_dir="${INSTALL_DIR:-${HOME}/.local/bin}"

    # Resolve target version.
    _version=$(resolve_version)

    # Perform installation.
    install "${_version}" "${_os}" "${_arch}" "${_install_dir}"

    # PATH check and setup instructions.
    if ! is_in_path "${_install_dir}"; then
        _shell_config=$(detect_shell_config)
        warn "${_install_dir} is not in your PATH."
        log ""
        log "Add it to your PATH by running:"
        log ""
        log "  ${BOLD}export PATH=\"${_install_dir}:\$PATH\"${RESET}"
        log ""
        log "To make this permanent, add the above line to ${BOLD}${_shell_config}${RESET}"
        log ""
    fi

    # Verify installation.
    if command -v "${BINARY_NAME}" >/dev/null 2>&1; then
        _installed_version=$("${BINARY_NAME}" --version 2>&1 || true)
        ok ""
        ok "Verify: ${BOLD}${BINARY_NAME} --version${RESET} -> ${_installed_version}"
    else
        log ""
        log "Run '${BOLD}${BINARY_NAME} --version${RESET}' to verify the installation."
    fi
}

# Allow sourcing for testing without executing main.
if [ "${INVOWK_INSTALL_TESTING:-}" != "1" ]; then
    trap cleanup EXIT
    main "$@"
fi
