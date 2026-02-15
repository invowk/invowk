#!/bin/sh
# SPDX-License-Identifier: MPL-2.0
#
# Unit tests for install.sh pure functions.
# Usage: sh scripts/test_install.sh

set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INVOWK_INSTALL_TESTING=1
export INVOWK_INSTALL_TESTING

# Source install.sh without executing main.
# shellcheck source=install.sh
. "$SCRIPT_DIR/install.sh"

# Initialize colors (needed because setup_colors is called in main, not at top level).
setup_colors

PASS=0
FAIL=0

assert_eq() {
    _desc="$1"
    _expected="$2"
    _actual="$3"
    if [ "$_expected" = "$_actual" ]; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
        printf 'FAIL: %s\n  expected: %s\n  actual:   %s\n' "$_desc" "$_expected" "$_actual"
    fi
}

assert_exit_code() {
    _desc="$1"
    _expected_code="$2"
    shift 2
    _actual_code=0
    "$@" >/dev/null 2>&1 || _actual_code=$?
    if [ "$_expected_code" = "$_actual_code" ]; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
        printf 'FAIL: %s\n  expected exit code: %s\n  actual exit code:   %s\n' \
            "$_desc" "$_expected_code" "$_actual_code"
    fi
}

# ---------------------------------------------------------------------------
# Tests: asset_filename
# ---------------------------------------------------------------------------

assert_eq "asset_filename linux amd64" \
    "invowk_1.0.0_linux_amd64.tar.gz" \
    "$(asset_filename v1.0.0 linux amd64)"

assert_eq "asset_filename darwin arm64" \
    "invowk_1.0.0_darwin_arm64.tar.gz" \
    "$(asset_filename v1.0.0 darwin arm64)"

assert_eq "asset_filename strips v prefix" \
    "invowk_2.3.4_linux_arm64.tar.gz" \
    "$(asset_filename v2.3.4 linux arm64)"

assert_eq "asset_filename prerelease version" \
    "invowk_1.0.0-alpha.1_linux_amd64.tar.gz" \
    "$(asset_filename v1.0.0-alpha.1 linux amd64)"

# ---------------------------------------------------------------------------
# Tests: is_in_path
# ---------------------------------------------------------------------------

assert_exit_code "is_in_path finds existing entry" 0 \
    is_in_path "/usr/bin"

assert_exit_code "is_in_path rejects missing entry" 1 \
    is_in_path "/nonexistent/path/unlikely/to/exist"

# Test with a controlled PATH to avoid false positives.
_saved_path="$PATH"
PATH="/a:/b:/c"
assert_exit_code "is_in_path matches exact dir" 0 is_in_path "/b"
assert_exit_code "is_in_path rejects substring" 1 is_in_path "/b/sub"
assert_exit_code "is_in_path rejects partial match" 1 is_in_path "/"
PATH="$_saved_path"

# ---------------------------------------------------------------------------
# Tests: detect_shell_config
# ---------------------------------------------------------------------------

_saved_shell="${SHELL:-}"
_saved_home="$HOME"

SHELL="/bin/zsh"
HOME="/tmp/test_home"
assert_eq "detect_shell_config zsh" "/tmp/test_home/.zshrc" "$(detect_shell_config)"

SHELL="/bin/fish"
assert_eq "detect_shell_config fish" "/tmp/test_home/.config/fish/config.fish" "$(detect_shell_config)"

SHELL="/bin/sh"
assert_eq "detect_shell_config sh" "/tmp/test_home/.profile" "$(detect_shell_config)"

# Restore.
SHELL="$_saved_shell"
HOME="$_saved_home"

# ---------------------------------------------------------------------------
# Tests: sha256_file / verify_checksum
# ---------------------------------------------------------------------------

# Initialize SHA256_CMD (normally done in main's detect_sha256_tool).
detect_sha256_tool

_tmpfile=$(mktemp)
printf 'hello\n' > "$_tmpfile"
# SHA256("hello\n") = 5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
_expected_hash="5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"

_actual_hash=$(sha256_file "$_tmpfile")
assert_eq "sha256_file computes correct hash" "$_expected_hash" "$_actual_hash"

assert_exit_code "verify_checksum passes with correct hash" 0 \
    verify_checksum "$_tmpfile" "$_expected_hash"

# verify_checksum calls die() -> exit 1 on mismatch, which terminates the
# process. Run in a subshell so exit only kills the subshell, allowing the
# parent to capture the exit code.
_checksum_exit=0
(verify_checksum "$_tmpfile" "0000000000000000000000000000000000000000000000000000000000000000") >/dev/null 2>&1 || _checksum_exit=$?
assert_eq "verify_checksum fails with incorrect hash (exit code)" "1" "$_checksum_exit"

rm -f "$_tmpfile"

# ---------------------------------------------------------------------------
# Tests: detect_os (host-dependent — just verify it produces a known value)
# ---------------------------------------------------------------------------

_os=$(detect_os)
case "$_os" in
    linux|darwin)
        PASS=$((PASS + 1))
        ;;
    *)
        FAIL=$((FAIL + 1))
        printf 'FAIL: detect_os returned unexpected value: %s\n' "$_os"
        ;;
esac

# ---------------------------------------------------------------------------
# Tests: detect_arch (host-dependent — just verify it produces a known value)
# ---------------------------------------------------------------------------

_arch=$(detect_arch)
case "$_arch" in
    amd64|arm64)
        PASS=$((PASS + 1))
        ;;
    *)
        FAIL=$((FAIL + 1))
        printf 'FAIL: detect_arch returned unexpected value: %s\n' "$_arch"
        ;;
esac

# ---------------------------------------------------------------------------
# Tests: validate_version_format
# ---------------------------------------------------------------------------

assert_exit_code "validate_version_format accepts v1.0.0" 0 \
    validate_version_format "v1.0.0"

assert_exit_code "validate_version_format accepts v0.1.0-alpha.1" 0 \
    validate_version_format "v0.1.0-alpha.1"

assert_exit_code "validate_version_format accepts v2.3.4-beta.5+build.123" 0 \
    validate_version_format "v2.3.4-beta.5+build.123"

assert_exit_code "validate_version_format rejects missing v prefix" 1 \
    validate_version_format "1.0.0"

assert_exit_code "validate_version_format rejects single segment" 1 \
    validate_version_format "v1"

assert_exit_code "validate_version_format rejects two segments" 1 \
    validate_version_format "v1.0"

assert_exit_code "validate_version_format rejects empty string" 1 \
    validate_version_format ""

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
