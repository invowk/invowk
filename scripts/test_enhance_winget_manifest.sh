#!/bin/sh
# SPDX-License-Identifier: MPL-2.0
#
# Unit tests for enhance-winget-manifest.sh and enhance_winget_fields.py.
# Usage: sh scripts/test_enhance_winget_manifest.sh

set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TMPDIR_BASE="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_BASE"' EXIT

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
        printf 'FAIL: %s\n  expected:\n%s\n  actual:\n%s\n' "$_desc" "$_expected" "$_actual"
    fi
}

assert_file_eq() {
    _desc="$1"
    _expected_file="$2"
    _actual_file="$3"
    _expected_content=$(cat "$_expected_file")
    _actual_content=$(cat "$_actual_file")
    assert_eq "$_desc" "$_expected_content" "$_actual_content"
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
# Tests: enhance_winget_fields.py — normal GoReleaser-generated manifest
# ---------------------------------------------------------------------------

# Simulate what GoReleaser v2.13+ generates for a zip archive.
_input="$TMPDIR_BASE/goreleaser_input.yaml"
_expected="$TMPDIR_BASE/goreleaser_expected.yaml"

cat > "$_input" << 'EOF'
PackageIdentifier: Invowk.Invowk
PackageVersion: 1.0.0
InstallerLocale: en-US
InstallerType: zip
ReleaseDate: "2026-02-18"
Installers:
  - Architecture: x64
    NestedInstallerType: portable
    NestedInstallerFiles:
      - RelativeFilePath: invowk.exe
        PortableCommandAlias: invowk
    InstallerUrl: https://github.com/invowk/invowk/releases/download/v1.0.0/invowk_1.0.0_windows_amd64.zip
    InstallerSha256: abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890
    UpgradeBehavior: uninstallPrevious
ManifestType: installer
ManifestVersion: 1.12.0
EOF

cat > "$_expected" << 'EOF'
PackageIdentifier: Invowk.Invowk
PackageVersion: 1.0.0
MinimumOSVersion: 10.0.17763.0
Platform:
  - Windows.Desktop
InstallerLocale: en-US
InstallerType: zip
ReleaseDate: "2026-02-18"
Commands:
  - invowk
Installers:
  - Architecture: x64
    NestedInstallerType: portable
    NestedInstallerFiles:
      - RelativeFilePath: invowk.exe
        PortableCommandAlias: invowk
    InstallerUrl: https://github.com/invowk/invowk/releases/download/v1.0.0/invowk_1.0.0_windows_amd64.zip
    InstallerSha256: abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890
    UpgradeBehavior: uninstallPrevious
ManifestType: installer
ManifestVersion: 1.12.0
EOF

python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$_input"
assert_file_eq "GoReleaser manifest: fields injected correctly" "$_expected" "$_input"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — idempotency
# ---------------------------------------------------------------------------

# Run the same script again on the already-enhanced output.
_before_second_run=$(cat "$_input")
python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$_input"
_after_second_run=$(cat "$_input")
assert_eq "idempotency: second run produces no changes" "$_before_second_run" "$_after_second_run"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — manual manifest (root-level nested fields)
# ---------------------------------------------------------------------------

# Our manual v0.2.0 manifest has NestedInstallerType at root level (not
# inside each InstallerItem). The script should handle both layouts.
_manual_input="$TMPDIR_BASE/manual_input.yaml"
_manual_expected="$TMPDIR_BASE/manual_expected.yaml"

cat > "$_manual_input" << 'EOF'
PackageIdentifier: Invowk.Invowk
PackageVersion: 0.2.0
InstallerType: zip
NestedInstallerType: portable
NestedInstallerFiles:
  - RelativeFilePath: invowk.exe
    PortableCommandAlias: invowk
Installers:
  - Architecture: x64
    InstallerUrl: https://github.com/invowk/invowk/releases/download/v0.2.0/invowk_0.2.0_windows_amd64.zip
    InstallerSha256: 7b41621a769ead828130b6aedcc779b9bd7f6cb973ec2da6be88125b146a335e
    ReleaseDate: 2026-02-15
ManifestType: installer
ManifestVersion: 1.10.0
EOF

cat > "$_manual_expected" << 'EOF'
PackageIdentifier: Invowk.Invowk
PackageVersion: 0.2.0
MinimumOSVersion: 10.0.17763.0
Platform:
  - Windows.Desktop
InstallerType: zip
NestedInstallerType: portable
NestedInstallerFiles:
  - RelativeFilePath: invowk.exe
    PortableCommandAlias: invowk
Commands:
  - invowk
Installers:
  - Architecture: x64
    InstallerUrl: https://github.com/invowk/invowk/releases/download/v0.2.0/invowk_0.2.0_windows_amd64.zip
    InstallerSha256: 7b41621a769ead828130b6aedcc779b9bd7f6cb973ec2da6be88125b146a335e
    ReleaseDate: 2026-02-15
ManifestType: installer
ManifestVersion: 1.10.0
EOF

python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$_manual_input"
assert_file_eq "manual manifest: fields injected correctly" "$_manual_expected" "$_manual_input"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — partial enhancement (some fields exist)
# ---------------------------------------------------------------------------

# Manifest that already has MinimumOSVersion but not Platform or Commands.
_partial_input="$TMPDIR_BASE/partial_input.yaml"
_partial_expected="$TMPDIR_BASE/partial_expected.yaml"

cat > "$_partial_input" << 'EOF'
PackageIdentifier: Invowk.Invowk
PackageVersion: 1.0.0
MinimumOSVersion: 10.0.17763.0
InstallerType: zip
Installers:
  - Architecture: x64
    InstallerUrl: https://example.com/test.zip
    InstallerSha256: 0000000000000000000000000000000000000000000000000000000000000000
ManifestType: installer
ManifestVersion: 1.12.0
EOF

cat > "$_partial_expected" << 'EOF'
PackageIdentifier: Invowk.Invowk
PackageVersion: 1.0.0
Platform:
  - Windows.Desktop
MinimumOSVersion: 10.0.17763.0
InstallerType: zip
Commands:
  - invowk
Installers:
  - Architecture: x64
    InstallerUrl: https://example.com/test.zip
    InstallerSha256: 0000000000000000000000000000000000000000000000000000000000000000
ManifestType: installer
ManifestVersion: 1.12.0
EOF

python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$_partial_input"
assert_file_eq "partial manifest: only missing fields added" "$_partial_expected" "$_partial_input"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — usage error
# ---------------------------------------------------------------------------

assert_exit_code "missing argument exits with error" 1 \
    python3 "$SCRIPT_DIR/enhance_winget_fields.py"

# ---------------------------------------------------------------------------
# Tests: enhance-winget-manifest.sh — pre-release skip
# ---------------------------------------------------------------------------

# The bash script should exit 0 for pre-release versions without calling gh.
_prerelease_output="$TMPDIR_BASE/prerelease_output.txt"
bash "$SCRIPT_DIR/enhance-winget-manifest.sh" "1.0.0-alpha.1" > "$_prerelease_output" 2>&1 || true
_prerelease_msg=$(cat "$_prerelease_output")
case "$_prerelease_msg" in
    *"Skipping"*"pre-release"*)
        PASS=$((PASS + 1))
        ;;
    *)
        FAIL=$((FAIL + 1))
        printf 'FAIL: pre-release skip message not found\n  output: %s\n' "$_prerelease_msg"
        ;;
esac

assert_exit_code "pre-release version exits successfully" 0 \
    bash "$SCRIPT_DIR/enhance-winget-manifest.sh" "1.0.0-beta.2"

assert_exit_code "pre-release rc exits successfully" 0 \
    bash "$SCRIPT_DIR/enhance-winget-manifest.sh" "2.0.0-rc.1"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
