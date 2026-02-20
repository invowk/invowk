#!/bin/sh
# SPDX-License-Identifier: MPL-2.0
#
# Tests for enhance-winget-manifest.sh and enhance_winget_fields.py.
# Exercises field injection, idempotency, partial enhancement, and pre-release
# skip behavior. No network calls required.

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
NestedInstallerType: portable
NestedInstallerFiles:
  - RelativeFilePath: invowk.exe
    PortableCommandAlias: invowk
ReleaseDate: "2026-02-18"
UpgradeBehavior: uninstallPrevious
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
UpgradeBehavior: uninstallPrevious
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
NestedInstallerType: portable
NestedInstallerFiles:
  - RelativeFilePath: invowk.exe
    PortableCommandAlias: invowk
UpgradeBehavior: uninstallPrevious
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
# Tests: enhance_winget_fields.py — reverse partial (Commands present, others missing)
# ---------------------------------------------------------------------------

_rev_partial_input="$TMPDIR_BASE/rev_partial_input.yaml"
_rev_partial_expected="$TMPDIR_BASE/rev_partial_expected.yaml"

cat > "$_rev_partial_input" << 'EOF'
PackageIdentifier: Invowk.Invowk
PackageVersion: 1.0.0
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

cat > "$_rev_partial_expected" << 'EOF'
PackageIdentifier: Invowk.Invowk
PackageVersion: 1.0.0
MinimumOSVersion: 10.0.17763.0
Platform:
  - Windows.Desktop
InstallerType: zip
NestedInstallerType: portable
NestedInstallerFiles:
  - RelativeFilePath: invowk.exe
    PortableCommandAlias: invowk
UpgradeBehavior: uninstallPrevious
Commands:
  - invowk
Installers:
  - Architecture: x64
    InstallerUrl: https://example.com/test.zip
    InstallerSha256: 0000000000000000000000000000000000000000000000000000000000000000
ManifestType: installer
ManifestVersion: 1.12.0
EOF

python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$_rev_partial_input"
assert_file_eq "reverse partial: Commands present, MinOS+Platform injected" "$_rev_partial_expected" "$_rev_partial_input"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — fully enhanced manifest (all fields present)
# ---------------------------------------------------------------------------

# Use the reference template (stripping the yaml-language-server comment lines).
_full_input="$TMPDIR_BASE/full_input.yaml"
grep -v '^# ' "$SCRIPT_DIR/winget/Invowk.Invowk.installer.yaml" | grep -v '^$' > "$_full_input"
_full_before=$(cat "$_full_input")
python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$_full_input"
_full_after=$(cat "$_full_input")
assert_eq "fully enhanced manifest: no changes made" "$_full_before" "$_full_after"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — usage error
# ---------------------------------------------------------------------------

assert_exit_code "missing argument exits with error" 1 \
    python3 "$SCRIPT_DIR/enhance_winget_fields.py"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — nonexistent file
# ---------------------------------------------------------------------------

assert_exit_code "nonexistent file exits with error" 1 \
    python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$TMPDIR_BASE/nonexistent.yaml"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — empty file
# ---------------------------------------------------------------------------

_empty_input="$TMPDIR_BASE/empty_input.yaml"
: > "$_empty_input"
assert_exit_code "empty file exits with error" 1 \
    python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$_empty_input"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — missing PackageVersion anchor
# ---------------------------------------------------------------------------

_no_pkgver="$TMPDIR_BASE/no_pkgver.yaml"
cat > "$_no_pkgver" << 'EOF'
PackageIdentifier: Invowk.Invowk
InstallerType: zip
Installers:
  - Architecture: x64
    InstallerUrl: https://example.com/test.zip
    InstallerSha256: 0000000000000000000000000000000000000000000000000000000000000000
ManifestType: installer
ManifestVersion: 1.12.0
EOF

assert_exit_code "missing PackageVersion anchor exits with error" 1 \
    python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$_no_pkgver"

# ---------------------------------------------------------------------------
# Tests: enhance_winget_fields.py — missing Installers anchor
# ---------------------------------------------------------------------------

_no_installers="$TMPDIR_BASE/no_installers.yaml"
cat > "$_no_installers" << 'EOF'
PackageIdentifier: Invowk.Invowk
PackageVersion: 1.0.0
InstallerType: zip
ManifestType: installer
ManifestVersion: 1.12.0
EOF

assert_exit_code "missing Installers anchor exits with error" 1 \
    python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$_no_installers"

# ---------------------------------------------------------------------------
# Tests: PR body checkbox transformation (sed patterns)
# ---------------------------------------------------------------------------

# Simulate the PR template GoReleaser fetches from microsoft/winget-pkgs.
_checkbox_input="$TMPDIR_BASE/pr_body_input.txt"
_checkbox_expected="$TMPDIR_BASE/pr_body_expected.txt"

cat > "$_checkbox_input" << 'PREOF'
Checklist for Pull Requests
- [ ] Have you signed the [Contributor License Agreement](https://cla.opensource.microsoft.com/microsoft/winget-pkgs)?
- [ ] Is there a linked Issue?  If so, fill in the Issue number below.
   <!-- Example: Resolves #328283 -->
  - Resolves #[Issue Number]

Manifests
- [ ] Have you checked that there aren't other open [pull requests](https://github.com/microsoft/winget-pkgs/pulls) for the same manifest update/change?
- [ ] This PR only modifies one (1) manifest
- [ ] Have you [validated](https://github.com/microsoft/winget-pkgs/blob/master/doc/Authoring.md#validation) your manifest locally with `winget validate --manifest <path>`?
- [ ] Have you tested your manifest locally with `winget install --manifest <path>`?
- [ ] Does your manifest conform to the [1.10 schema](https://github.com/microsoft/winget-pkgs/tree/master/doc/manifest/schema/1.10.0)?

---

###### Automated with [GoReleaser](https://goreleaser.com)
PREOF

cat > "$_checkbox_expected" << 'PREOF'
Checklist for Pull Requests
- [x] Have you signed the [Contributor License Agreement](https://cla.opensource.microsoft.com/microsoft/winget-pkgs)?
- [ ] Is there a linked Issue?  If so, fill in the Issue number below.
   <!-- Example: Resolves #328283 -->

Manifests
- [x] Have you checked that there aren't other open [pull requests](https://github.com/microsoft/winget-pkgs/pulls) for the same manifest update/change?
- [x] This PR only modifies one (1) manifest
- [ ] Have you [validated](https://github.com/microsoft/winget-pkgs/blob/master/doc/Authoring.md#validation) your manifest locally with `winget validate --manifest <path>`?
- [ ] Have you tested your manifest locally with `winget install --manifest <path>`?
- [x] Does your manifest conform to the [1.10 schema](https://github.com/microsoft/winget-pkgs/tree/master/doc/manifest/schema/1.10.0)?

---

###### Automated with [GoReleaser](https://goreleaser.com)
PREOF

_checkbox_actual="$TMPDIR_BASE/pr_body_actual.txt"
sed \
  -e '/Contributor License Agreement/s/- \[ \]/- [x]/' \
  -e '/other open.*pull requests/s/- \[ \]/- [x]/' \
  -e '/only modifies one/s/- \[ \]/- [x]/' \
  -e '/conform to the.*schema/s/- \[ \]/- [x]/' \
  -e '/Resolves #\[Issue Number\]/d' \
  "$_checkbox_input" > "$_checkbox_actual"
assert_file_eq "PR body: 4 boxes checked, 3 unchecked, issue placeholder removed" "$_checkbox_expected" "$_checkbox_actual"

# Idempotency: running the same sed on already-checked output should produce no changes.
_checkbox_idem="$TMPDIR_BASE/pr_body_idem.txt"
sed \
  -e '/Contributor License Agreement/s/- \[ \]/- [x]/' \
  -e '/other open.*pull requests/s/- \[ \]/- [x]/' \
  -e '/only modifies one/s/- \[ \]/- [x]/' \
  -e '/conform to the.*schema/s/- \[ \]/- [x]/' \
  -e '/Resolves #\[Issue Number\]/d' \
  "$_checkbox_actual" > "$_checkbox_idem"
assert_file_eq "PR body: idempotent on already-checked body" "$_checkbox_expected" "$_checkbox_idem"

# ---------------------------------------------------------------------------
# Tests: enhance-winget-manifest.sh — pre-release skip
# ---------------------------------------------------------------------------

# The bash script should exit 0 for pre-release versions without calling gh.
_prerelease_output="$TMPDIR_BASE/prerelease_output.txt"
_prerelease_code=0
bash "$SCRIPT_DIR/enhance-winget-manifest.sh" "1.0.0-alpha.1" > "$_prerelease_output" 2>&1 || _prerelease_code=$?
assert_eq "pre-release alpha exits with code 0" "0" "$_prerelease_code"
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

assert_exit_code "pre-release beta exits successfully" 0 \
    bash "$SCRIPT_DIR/enhance-winget-manifest.sh" "1.0.0-beta.2"

assert_exit_code "pre-release rc exits successfully" 0 \
    bash "$SCRIPT_DIR/enhance-winget-manifest.sh" "2.0.0-rc.1"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
