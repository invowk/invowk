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
#   INVOWK_REQUIRE_SIGNATURE - Set to 1, true or yes to fail when cosign is not
#                     installed, instead of warning and installing without
#                     verifying the signature of checksums.txt. 0, false, no
#                     or unset keep the warning; any other value is an error.
#
# Requirements:
#   - curl or wget
#   - sha256sum (Linux) or shasum (macOS)
#   - tar, gzip
#   - cosign (optional, recommended): verifies the Sigstore signature of
#     checksums.txt before the script trusts it. Releases with a Sigstore
#     bundle (v0.5.0 on) need cosign v2.4.2 or later.
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

# The only signer accepted for checksums.txt: the release workflow, run by a
# tag push, at the tag of the version being installed. Other workflows of the
# repository can also obtain an OIDC token, so the identity is pinned to the
# workflow file and the tag, never to the repository alone.
SIGNER_IDENTITY_PREFIX="https://github.com/${GITHUB_REPO}/.github/workflows/release.yml@refs/tags/"
SIGNER_OIDC_ISSUER="https://token.actions.githubusercontent.com"
SIGNER_WORKFLOW_TRIGGER="push"

# The oldest cosign that verifies every published checksums.txt.sigstore.json.
# From v0.7.0 on the bundles use the Sigstore bundle v0.3 format, which
# cosign v2.4.1 and earlier read as an old cosign bundle and reject with
# "bundle does not contain cert for verification". Checked against the bundles
# of v0.5.0 to v0.17.0 with cosign v2.2.4, v2.4.0, v2.4.1 (fail from v0.7.0)
# and v2.4.2, v2.4.3, v2.5.0, v3.0.1, v3.1.3 (all verify). The check applies to
# every bundle, including the old-format ones of v0.5.0 and v0.6.0; detached
# signatures (before v0.5.0) verify with older cosign and skip it.
COSIGN_MIN_BUNDLE_VERSION="2.4.2"

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

log()  { printf '%b%s%b\n' "${CYAN}" "$*" "${RESET}" >&2; }
ok()   { printf '%b%s%b\n' "${GREEN}" "$*" "${RESET}" >&2; }
warn() { printf '%b%s%b\n' "${YELLOW}WARNING: " "$*" "${RESET}" >&2; }
err()  { printf '%b%s%b\n' "${RED}ERROR: " "$*" "${RESET}" >&2; }
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
                curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" "${_url}"
            else
                curl -fsSL "${_url}"
            fi
            ;;
        wget)
            if [ -n "${GITHUB_TOKEN:-}" ]; then
                wget -qO- --header="Authorization: Bearer ${GITHUB_TOKEN}" "${_url}"
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
                curl -fsSL -H "Authorization: Bearer ${GITHUB_TOKEN}" -o "${_dest}" "${_url}"
            else
                curl -fsSL -o "${_dest}" "${_url}"
            fi
            ;;
        wget)
            if [ -n "${GITHUB_TOKEN:-}" ]; then
                wget -qO "${_dest}" --header="Authorization: Bearer ${GITHUB_TOKEN}" "${_url}"
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
# Captures the full output first to avoid pipe masking the hash tool's exit code.
# POSIX sh does not support set -o pipefail, so in a pipeline like
# sha256sum "$file" | cut -d' ' -f1, a sha256sum failure would be masked by
# cut's exit code 0.
sha256_file() {
    _file="$1"
    case "${SHA256_CMD}" in
        sha256sum)
            _output=$(sha256sum "${_file}") || die "sha256sum failed for ${_file}"
            ;;
        shasum)
            _output=$(shasum -a 256 "${_file}") || die "shasum failed for ${_file}"
            ;;
        *) die "Internal error: unsupported SHA256 command: ${SHA256_CMD}" ;;
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
# Signature verification
# ---------------------------------------------------------------------------

# Print the certificate identity that must have signed a version's checksums.txt.
signer_identity() {
    printf '%s%s' "${SIGNER_IDENTITY_PREFIX}" "$1"
}

# Report whether INVOWK_REQUIRE_SIGNATURE asks for a verified signature.
# 1, true and yes (any case) mean on; 0, false, no, empty and unset mean off.
# Any other value is fatal, so a typo never turns the requirement off.
# Uses only shell built-ins: it runs before cosign or other tools are looked up.
signature_required() {
    case "${INVOWK_REQUIRE_SIGNATURE:-}" in
        1|[Tt][Rr][Uu][Ee]|[Yy][Ee][Ss]) return 0 ;;
        ""|0|[Ff][Aa][Ll][Ss][Ee]|[Nn][Oo]) return 1 ;;
        *) die "Invalid INVOWK_REQUIRE_SIGNATURE value: ${INVOWK_REQUIRE_SIGNATURE}
Use 1, true or yes to require a verified signature, or 0, false or no (or leave it unset) to allow installing without cosign." ;;
    esac
}

# Report whether version X.Y.Z ($1) is at least version X.Y.Z ($2).
# Both arguments are plain dotted numbers; parse_cosign_version produces them.
version_at_least() {
    _va_have="$1"
    _va_min="$2"
    for _va_i in 1 2 3; do
        _va_h=${_va_have%%.*}
        _va_m=${_va_min%%.*}
        if [ "${_va_h}" -gt "${_va_m}" ]; then
            return 0
        fi
        if [ "${_va_h}" -lt "${_va_m}" ]; then
            return 1
        fi
        _va_have=${_va_have#*.}
        _va_min=${_va_min#*.}
    done
    return 0
}

# Print the X.Y.Z version from the output of `cosign version` ($1), or nothing
# when it has no GitVersion line with a release version (a development build).
parse_cosign_version() {
    printf '%s\n' "$1" |
        sed -n 's/^GitVersion:[[:space:]]*v\{0,1\}\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\).*$/\1/p' |
        head -n 1
}

# Fail unless the cosign on PATH can verify a Sigstore bundle. An older cosign
# rejects a valid bundle, which would read as a bad signature; this tells the
# user to upgrade instead. A version that cannot be read only warns: the
# verification that follows still has to pass.
check_cosign_bundle_support() {
    if ! _cv_output=$(cosign version 2>&1); then
        die "cosign is on PATH, but 'cosign version' failed, so the signature of checksums.txt cannot be verified.
This is a problem with the cosign installation, not with the release.
cosign output:
${_cv_output}

Repair or upgrade cosign (v${COSIGN_MIN_BUNDLE_VERSION} or later), or remove it from PATH."
    fi
    _cv_version=$(parse_cosign_version "${_cv_output}")
    if [ -z "${_cv_version}" ]; then
        warn "Could not read the cosign version; this release needs cosign v${COSIGN_MIN_BUNDLE_VERSION} or later. Verifying anyway."
        return 0
    fi
    if ! version_at_least "${_cv_version}" "${COSIGN_MIN_BUNDLE_VERSION}"; then
        die "cosign ${_cv_version} is older than the minimum ${COSIGN_MIN_BUNDLE_VERSION} needed to verify this release; upgrade cosign or remove it from PATH.
Without cosign on PATH, the installer checks only the SHA256 checksum of the archive (and fails if INVOWK_REQUIRE_SIGNATURE is on)."
    fi
}

# Report whether cosign output ($1) shows that cosign could not reach the
# Sigstore services (TUF metadata, Rekor) rather than a signature that does
# not verify. Both outcomes are fatal; only the message differs.
cosign_output_is_network_error() {
    printf '%s\n' "$1" |
        grep -Eiq '(^|[^a-z])tuf([^a-z]|$)|dial tcp|proxyconnect|no such host|connection refused|connection reset|i/o timeout|tls handshake timeout|deadline exceeded|network is unreachable'
}

# Verify the Cosign signature of checksums.txt before it is trusted.
# Arguments: version, path of the downloaded checksums.txt.
# Without cosign on PATH, warn and return, unless INVOWK_REQUIRE_SIGNATURE is on.
# With cosign, any failure is fatal: a missing signature, a download error, a
# cosign too old for the bundle, cosign unable to reach Sigstore, or a signature
# that does not verify against the pinned identity, issuer and trigger.
verify_checksums_signature() {
    _sig_version="$1"
    _sig_checksums="$2"

    _sig_required=0
    if signature_required; then
        _sig_required=1
    fi

    if ! command -v cosign >/dev/null 2>&1; then
        if [ "${_sig_required}" = "1" ]; then
            die "cosign not found, and INVOWK_REQUIRE_SIGNATURE=${INVOWK_REQUIRE_SIGNATURE} requires the signature of checksums.txt to be verified.
Install cosign v${COSIGN_MIN_BUNDLE_VERSION} or later (https://docs.sigstore.dev/cosign/system_config/installation/) and try again."
        fi
        warn "cosign not found: the signature of checksums.txt was NOT verified; only the SHA256 checksum of the archive will be checked.
Install cosign v${COSIGN_MIN_BUNDLE_VERSION} or later (https://docs.sigstore.dev/cosign/system_config/installation/) to verify it,
or set INVOWK_REQUIRE_SIGNATURE=1 to make a missing cosign an error."
        return 0
    fi

    _sig_identity=$(signer_identity "${_sig_version}")
    _sig_url="${DOWNLOAD_BASE_URL}/${_sig_version}/checksums.txt"

    log "Verifying the signature of checksums.txt..."
    # Releases from v0.5.0 on publish a Sigstore bundle; earlier ones published
    # a detached signature and certificate.
    # A release with a bundle is verified with the bundle only: a bundle that
    # fails never falls back to a detached signature.
    if download_to_file "${_sig_url}.sigstore.json" "${_sig_checksums}.sigstore.json" 2>/dev/null; then
        check_cosign_bundle_support
        set -- --bundle "${_sig_checksums}.sigstore.json"
    elif download_to_file "${_sig_url}.sig" "${_sig_checksums}.sig" 2>/dev/null &&
        download_to_file "${_sig_url}.pem" "${_sig_checksums}.pem" 2>/dev/null; then
        set -- --signature "${_sig_checksums}.sig" --certificate "${_sig_checksums}.pem"
    else
        die "Could not download the signature of checksums.txt for ${_sig_version}.

cosign is installed, so the installer will not continue without verifying it.
Check your network connection and try again.
If this persists, report at https://github.com/${GITHUB_REPO}/issues"
    fi

    if ! _sig_output=$(cosign verify-blob "$@" \
        --certificate-identity "${_sig_identity}" \
        --certificate-oidc-issuer "${SIGNER_OIDC_ISSUER}" \
        --certificate-github-workflow-trigger "${SIGNER_WORKFLOW_TRIGGER}" \
        "${_sig_checksums}" 2>&1); then
        if cosign_output_is_network_error "${_sig_output}"; then
            die "cosign could not verify the signature of checksums.txt for ${_sig_version}: it could not reach the Sigstore services (TUF metadata or Rekor).
This points to a network, proxy or cosign problem, not to a bad signature,
but the installer does not continue without verifying the signature.
cosign output:
${_sig_output}

Check your network connection and proxy settings, and try again."
        fi
        die "Signature verification of checksums.txt failed for ${_sig_version}.

Expected signer: ${_sig_identity}
Expected issuer: ${SIGNER_OIDC_ISSUER}
cosign output:
${_sig_output}

Do not install this release. Report at https://github.com/${GITHUB_REPO}/issues"
    fi
    ok "Signature of checksums.txt verified."
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
        # The version goes into download URLs and the pinned signer identity,
        # so check its shape before it is used anywhere.
        if ! validate_version_format "${INVOWK_VERSION}"; then
            die "Invalid INVOWK_VERSION: ${INVOWK_VERSION}
Use a release tag such as v1.0.0 or v0.1.0-alpha.1 (the v prefix is optional).
Check available versions at: https://github.com/${GITHUB_REPO}/releases"
        fi
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
        if ! validate_version_format "${_version}"; then
            die "Unexpected version format from GitHub API: ${_version}
This may indicate an API change. Report at: https://github.com/${GITHUB_REPO}/issues"
        fi
        log "Latest stable version: ${BOLD}${_version}${RESET}"
        echo "${_version}"
    fi
}

# ---------------------------------------------------------------------------
# Version validation
# ---------------------------------------------------------------------------

# Validate that a version string looks like a semver tag (vN.N.N with optional suffix).
# Uses shell glob matching, which is approximate — [0-9]* matches a digit followed by
# any characters. This catches obvious malformations (missing dots, no leading digit)
# without requiring a full regex engine.
validate_version_format() {
    _ver="$1"
    case "${_ver}" in
        v[0-9]*.[0-9]*.[0-9]*)
            return 0 ;; # Approximate semver format
        *)
            return 1 ;;
    esac
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
    # Fish follows XDG conventions; bash config depends on login vs interactive shell.
    case "${_shell_name}" in
        zsh)  echo "${HOME}/.zshrc" ;;
        bash)
            if [ -f "${HOME}/.bashrc" ]; then
                echo "${HOME}/.bashrc"
            else
                echo "${HOME}/.bash_profile"
            fi
            ;;
        fish) echo "${XDG_CONFIG_HOME:-${HOME}/.config}/fish/config.fish" ;;
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

    # The checksum file and its signature are downloaded to a mktemp directory
    # (mode 0700), which mitigates TOCTOU race conditions between download and
    # verification.
    log "Downloading checksums..."
    download_to_file "${_checksums_url}" "${_checksums_path}" || die "Failed to download checksums.txt.

Cannot verify download integrity. Please try again."

    # checksums.txt is trusted only after its signature verifies (when cosign
    # is available; see verify_checksums_signature).
    verify_checksums_signature "${_version}" "${_checksums_path}"

    # Extract expected checksum for our asset from checksums.txt.
    # Uses awk for exact field match — grep -F does substring matching which can
    # match multiple lines (e.g., both the .tar.gz and .tar.gz.sig entries).
    _expected_hash=$(awk -v asset="${_asset}" '$2 == asset { print $1; exit }' "${_checksums_path}")
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
        # Full extraction may have placed the binary in a subdirectory
        # (e.g., invowk_1.0.0_linux_amd64/invowk). Search recursively.
        _found=$(find "${TMPDIR_INSTALL}" -name "${BINARY_NAME}" -type f | head -1)
        if [ -z "${_found}" ]; then
            die "Binary '${BINARY_NAME}' not found in archive ${_asset}."
        fi
        mv "${_found}" "${TMPDIR_INSTALL}/${BINARY_NAME}" || die "Failed to relocate binary from nested archive path: ${_found}"
    fi

    # Ensure the binary is executable.
    chmod +x "${TMPDIR_INSTALL}/${BINARY_NAME}" || die "Failed to set executable permission on ${BINARY_NAME}"

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

    # Reject an invalid INVOWK_REQUIRE_SIGNATURE before anything is downloaded.
    signature_required || :

    # Detect platform and tools.
    _os=$(detect_os)
    _arch=$(detect_arch)
    detect_download_tool
    detect_sha256_tool

    # Resolve installation directory.
    _install_dir="${INSTALL_DIR:-${HOME}/.local/bin}"

    # Resolve target version.
    _version=$(resolve_version)
    if [ -z "${_version}" ]; then
        die "Failed to resolve version. Set INVOWK_VERSION explicitly or check network connectivity."
    fi

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
        if _installed_version=$("${BINARY_NAME}" --version 2>&1); then
            ok ""
            ok "Verify: ${BOLD}${BINARY_NAME} --version${RESET} -> ${_installed_version}"
        else
            warn "Binary installed but '${BINARY_NAME} --version' returned a non-zero exit code."
        fi
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
