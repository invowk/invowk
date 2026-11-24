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
_saved_xdg="${XDG_CONFIG_HOME:-}"

SHELL="/bin/zsh"
HOME="/tmp/test_home"
assert_eq "detect_shell_config zsh" "/tmp/test_home/.zshrc" "$(detect_shell_config)"

# Fish uses XDG_CONFIG_HOME — unset it so the test exercises the fallback path
# (${HOME}/.config). Ubuntu CI runners pre-set XDG_CONFIG_HOME=/home/runner/.config,
# which would override the test HOME and produce a wrong expected path.
unset XDG_CONFIG_HOME
SHELL="/bin/fish"
assert_eq "detect_shell_config fish" "/tmp/test_home/.config/fish/config.fish" "$(detect_shell_config)"

# Also test with XDG_CONFIG_HOME explicitly set.
XDG_CONFIG_HOME="/tmp/custom_xdg"
export XDG_CONFIG_HOME
assert_eq "detect_shell_config fish with XDG" "/tmp/custom_xdg/fish/config.fish" "$(detect_shell_config)"

SHELL="/bin/sh"
assert_eq "detect_shell_config sh" "/tmp/test_home/.profile" "$(detect_shell_config)"

# Restore.
SHELL="$_saved_shell"
HOME="$_saved_home"
if [ -n "$_saved_xdg" ]; then
    XDG_CONFIG_HOME="$_saved_xdg"
    export XDG_CONFIG_HOME
else
    unset XDG_CONFIG_HOME
fi

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
# Tests: signer_identity
# ---------------------------------------------------------------------------

assert_eq "signer_identity pins release.yml at the version tag" \
    "https://github.com/invowk/invowk/.github/workflows/release.yml@refs/tags/v1.2.3" \
    "$(signer_identity v1.2.3)"

# ---------------------------------------------------------------------------
# Tests: signature_required
# ---------------------------------------------------------------------------

# Prints "on", "off", or nothing when the value is rejected (die exits the
# subshell before it prints).
require_sig_state() {
    (
        INVOWK_REQUIRE_SIGNATURE="$1"
        if signature_required; then echo on; else echo off; fi
    ) 2>/dev/null || :
}

for _v in 1 true TRUE True yes YES Yes; do
    assert_eq "signature_required treats '$_v' as on" "on" "$(require_sig_state "$_v")"
done
for _v in "" 0 false FALSE no NO No; do
    assert_eq "signature_required treats '$_v' as off" "off" "$(require_sig_state "$_v")"
done
for _v in 2 on off y n required " 1" "true "; do
    assert_eq "signature_required rejects '$_v'" "" "$(require_sig_state "$_v")"
done
_unset_state=$(
    unset INVOWK_REQUIRE_SIGNATURE
    if signature_required; then echo on; else echo off; fi
)
assert_eq "signature_required treats unset as off" "off" "$_unset_state"

# ---------------------------------------------------------------------------
# Tests: version_at_least / parse_cosign_version
# ---------------------------------------------------------------------------

assert_exit_code "version_at_least accepts the same version" 0 version_at_least 2.4.2 2.4.2
assert_exit_code "version_at_least rejects an older patch" 1 version_at_least 2.4.1 2.4.2
assert_exit_code "version_at_least rejects an older minor" 1 version_at_least 2.3.9 2.4.2
assert_exit_code "version_at_least rejects an older major" 1 version_at_least 1.99.99 2.4.2
assert_exit_code "version_at_least accepts a newer patch" 0 version_at_least 2.4.3 2.4.2
assert_exit_code "version_at_least compares numerically" 0 version_at_least 2.10.0 2.4.2
assert_exit_code "version_at_least accepts a newer major" 0 version_at_least 3.0.0 2.4.2

_cosign_banner='  ______   ______
 cosign: A tool for Container Signing, Verification and Storage in an OCI registry.

GitVersion:    v3.1.3
GitCommit:     unknown
GitTreeState:  clean'
assert_eq "parse_cosign_version reads the GitVersion line" "3.1.3" "$(parse_cosign_version "$_cosign_banner")"
assert_eq "parse_cosign_version accepts no v prefix" "2.4.2" "$(parse_cosign_version "GitVersion: 2.4.2")"
assert_eq "parse_cosign_version drops a prerelease suffix" "2.5.0" "$(parse_cosign_version "GitVersion:    v2.5.0-rc.1")"
assert_eq "parse_cosign_version prints nothing for a development build" "" "$(parse_cosign_version "GitVersion:    devel")"
assert_eq "parse_cosign_version prints nothing without a GitVersion line" "" "$(parse_cosign_version "cosign version 3.1.3")"

# ---------------------------------------------------------------------------
# Tests: cosign_output_is_network_error (outputs recorded from real cosign runs)
# ---------------------------------------------------------------------------

assert_exit_code "cosign_output_is_network_error detects a TUF fetch failure" 0 \
    cosign_output_is_network_error 'Error: setting trusted material: getting trusted root from TUF for new bundle verification: error getting live trusted root: failed to create TUF client failed to load metadata: tuf refresh failed: Get "https://tuf-repo-cdn.sigstore.dev/15.root.json": proxyconnect tcp: dial tcp 127.0.0.1:9: connect: connection refused'
assert_exit_code "cosign_output_is_network_error detects a DNS failure" 0 \
    cosign_output_is_network_error 'error during command execution: getting rekor public keys: Get "https://rekor.sigstore.dev/api/v1/log/publicKey": dial tcp: lookup rekor.sigstore.dev on 127.0.0.53:53: no such host'
assert_exit_code "cosign_output_is_network_error keeps a bad signature a bad signature" 1 \
    cosign_output_is_network_error 'Error: failed to verify signature: could not verify message: invalid signature when validating ASN.1 encoded signature'
assert_exit_code "cosign_output_is_network_error keeps a wrong identity a bad signature" 1 \
    cosign_output_is_network_error 'Error: failed to verify certificate identity: no matching CertificateIdentity found, last error: expected SAN value "https://github.com/invowk/invowk/.github/workflows/ci.yml@refs/tags/v0.17.0", got "https://github.com/invowk/invowk/.github/workflows/release.yml@refs/tags/v0.17.0"'

# ---------------------------------------------------------------------------
# Tests: resolve_version with INVOWK_VERSION
# ---------------------------------------------------------------------------

_rv_log=$(mktemp)

# run_resolve <INVOWK_VERSION>; prints the resolved version, or nothing when
# resolve_version fails. Downloads are recorded in $_rv_log, never made.
run_resolve() {
    : > "$_rv_log"
    (
        INVOWK_VERSION="$1"
        download() { printf '%s\n' "$1" >> "$_rv_log"; }
        resolve_version
    ) 2>/dev/null || :
}

assert_eq "resolve_version adds the v prefix" "v1.2.3" "$(run_resolve 1.2.3)"
assert_eq "resolve_version keeps a prerelease tag" "v0.1.0-alpha.1" "$(run_resolve v0.1.0-alpha.1)"
assert_eq "resolve_version checks the tag endpoint" \
    "https://api.github.com/repos/invowk/invowk/releases/tags/v0.1.0-alpha.1" "$(cat "$_rv_log")"
for _v in latest v1.2 1 "v" "../v1.0.0" "vx.y.z" "v1.0.0/../../evil/v1.0.0" "v1.0.0 x" 'v1.0.0;x'; do
    assert_eq "resolve_version rejects INVOWK_VERSION '$_v'" "" "$(run_resolve "$_v")"
    assert_eq "resolve_version downloads nothing for INVOWK_VERSION '$_v'" "" "$(cat "$_rv_log")"
done
rm -f "$_rv_log"

# ---------------------------------------------------------------------------
# Tests: verify_checksums_signature
# ---------------------------------------------------------------------------

_sig_tmp=$(mktemp -d)
_sig_bin="$_sig_tmp/bin"
mkdir -p "$_sig_bin"
printf 'checksums\n' > "$_sig_tmp/checksums.txt"

# Fake cosign. `cosign version` prints FAKE_COSIGN_VERSION_OUTPUT (default: a
# current release) and exits with FAKE_COSIGN_VERSION_RC. Any other call
# records its arguments one per line in FAKE_COSIGN_ARGS, appends a line to
# FAKE_COSIGN_CALLS, prints FAKE_COSIGN_OUTPUT and exits with FAKE_COSIGN_RC.
cat > "$_sig_bin/cosign" <<'FAKE'
#!/bin/sh
if [ "$1" = version ]; then
    printf '%s\n' "${FAKE_COSIGN_VERSION_OUTPUT-GitVersion:    v3.1.3}"
    exit "${FAKE_COSIGN_VERSION_RC:-0}"
fi
printf '%s\n' "$@" > "$FAKE_COSIGN_ARGS"
printf '%s\n' "$1" >> "$FAKE_COSIGN_CALLS"
if [ -n "${FAKE_COSIGN_OUTPUT:-}" ]; then
    printf '%s\n' "$FAKE_COSIGN_OUTPUT" >&2
fi
exit "${FAKE_COSIGN_RC:-0}"
FAKE
chmod +x "$_sig_bin/cosign"
FAKE_COSIGN_ARGS="$_sig_tmp/cosign-args"
FAKE_COSIGN_CALLS="$_sig_tmp/cosign-calls"
_sig_out="$_sig_tmp/out"
export FAKE_COSIGN_ARGS FAKE_COSIGN_CALLS

# Fake downloads: FAKE_SIG_ASSETS lists the signature suffixes that exist.
download_to_file() {
    case " ${FAKE_SIG_ASSETS:-} " in
        *" ${1##*/checksums.txt} "*) printf 'sig\n' > "$2" ;;
        *) return 22 ;;
    esac
}

# A PATH without cosign (the function needs no other external command when
# cosign is missing), and one with the fake first.
_no_cosign_path="$_sig_tmp/empty"
mkdir -p "$_no_cosign_path"
_fake_cosign_path="$_sig_bin:$PATH"

# run_sig_case <require_signature> <path> <assets> <cosign_rc>; prints the exit
# code. The combined output goes to $_sig_out. FAKE_COSIGN_VERSION_OUTPUT,
# FAKE_COSIGN_VERSION_RC and FAKE_COSIGN_OUTPUT reach the fake when exported.
run_sig_case() {
    rm -f "$FAKE_COSIGN_ARGS" "$FAKE_COSIGN_CALLS"
    _case_rc=0
    # PATH and the settings change in the subshell only.
    # shellcheck disable=SC2030
    (
        INVOWK_REQUIRE_SIGNATURE="$1"
        PATH="$2"
        FAKE_SIG_ASSETS="$3"
        FAKE_COSIGN_RC="$4"
        export INVOWK_REQUIRE_SIGNATURE FAKE_COSIGN_RC
        verify_checksums_signature v1.2.3 "$_sig_tmp/checksums.txt"
    ) >"$_sig_out" 2>&1 || _case_rc=$?
    printf '%s' "$_case_rc"
}

# assert_output_has <desc> <text>: the last run_sig_case printed <text>.
assert_output_has() {
    if grep -Fq -- "$2" "$_sig_out"; then
        PASS=$((PASS + 1))
    else
        FAIL=$((FAIL + 1))
        printf 'FAIL: %s\n  missing: %s\n  output:\n%s\n' "$1" "$2" "$(cat "$_sig_out")"
    fi
}

# assert_output_lacks <desc> <text>: the last run_sig_case did not print <text>.
assert_output_lacks() {
    if grep -Fq -- "$2" "$_sig_out"; then
        FAIL=$((FAIL + 1))
        printf 'FAIL: %s\n  unexpected: %s\n  output:\n%s\n' "$1" "$2" "$(cat "$_sig_out")"
    else
        PASS=$((PASS + 1))
    fi
}

# The cosign arguments expected for v1.2.3, after the signature arguments.
_pinned_args="--certificate-identity
https://github.com/invowk/invowk/.github/workflows/release.yml@refs/tags/v1.2.3
--certificate-oidc-issuer
https://token.actions.githubusercontent.com
--certificate-github-workflow-trigger
push
$_sig_tmp/checksums.txt"
_bundle_args="verify-blob
--bundle
$_sig_tmp/checksums.txt.sigstore.json
$_pinned_args"
_legacy_args="verify-blob
--signature
$_sig_tmp/checksums.txt.sig
--certificate
$_sig_tmp/checksums.txt.pem
$_pinned_args"

# Without cosign.
assert_eq "verify_checksums_signature warns and continues without cosign" "0" \
    "$(run_sig_case "" "$_no_cosign_path" ".sigstore.json" 0)"
assert_output_has "verify_checksums_signature names the cosign minimum when cosign is missing" \
    "Install cosign v2.4.2 or later"
for _v in 0 false No; do
    assert_eq "verify_checksums_signature continues without cosign when INVOWK_REQUIRE_SIGNATURE=$_v" "0" \
        "$(run_sig_case "$_v" "$_no_cosign_path" ".sigstore.json" 0)"
done
for _v in 1 true YES; do
    assert_eq "verify_checksums_signature fails without cosign when INVOWK_REQUIRE_SIGNATURE=$_v" "1" \
        "$(run_sig_case "$_v" "$_no_cosign_path" ".sigstore.json" 0)"
done
assert_eq "verify_checksums_signature rejects an invalid INVOWK_REQUIRE_SIGNATURE without cosign" "1" \
    "$(run_sig_case "maybe" "$_no_cosign_path" ".sigstore.json" 0)"
assert_output_has "verify_checksums_signature names the invalid INVOWK_REQUIRE_SIGNATURE" \
    "Invalid INVOWK_REQUIRE_SIGNATURE value: maybe"
assert_eq "verify_checksums_signature rejects an invalid INVOWK_REQUIRE_SIGNATURE with cosign" "1" \
    "$(run_sig_case "on" "$_fake_cosign_path" ".sigstore.json" 0)"
assert_exit_code "verify_checksums_signature does not run cosign for an invalid INVOWK_REQUIRE_SIGNATURE" 1 \
    test -e "$FAKE_COSIGN_ARGS"

# A bundle.
assert_eq "verify_checksums_signature passes a verified bundle" "0" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json" 0)"
assert_eq "verify_checksums_signature pins identity, issuer and trigger" \
    "$_bundle_args" "$(cat "$FAKE_COSIGN_ARGS")"

FAKE_COSIGN_OUTPUT='Error: failed to verify signature: could not verify message: invalid signature when validating ASN.1 encoded signature'
export FAKE_COSIGN_OUTPUT
assert_eq "verify_checksums_signature fails closed when cosign rejects" "1" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json" 1)"
assert_output_has "verify_checksums_signature reports a rejected signature as a bad release" \
    "Do not install this release."
assert_output_has "verify_checksums_signature shows the cosign output" "invalid signature when validating"

FAKE_COSIGN_OUTPUT='Error: tuf refresh failed: Get "https://tuf-repo-cdn.sigstore.dev/11.root.json": proxyconnect tcp: dial tcp 127.0.0.1:9: connect: connection refused'
assert_eq "verify_checksums_signature fails closed when cosign cannot reach Sigstore" "1" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json" 1)"
assert_output_has "verify_checksums_signature reports a network failure as one" \
    "could not reach the Sigstore services"
assert_output_lacks "verify_checksums_signature does not call a network failure a bad signature" \
    "Signature verification of checksums.txt failed"
assert_output_has "verify_checksums_signature still says not to install on a network failure" \
    "Do not install this release"

# A forged release signed by an attacker workflow whose path contains "tuf" stays a bad signature.
FAKE_COSIGN_OUTPUT='Error: failed to verify certificate identity: no matching CertificateIdentity found, last error: expected SAN value "https://github.com/invowk/invowk/.github/workflows/release.yml@refs/tags/v0.17.0", got "https://github.com/attacker/fork/.github/workflows/tuf.yml@refs/tags/v0.17.0"'
assert_eq "verify_checksums_signature fails closed on an attacker SAN that mentions tuf" "1" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json" 1)"
assert_output_has "verify_checksums_signature reports an attacker SAN that mentions tuf as a bad release" \
    "Signature verification of checksums.txt failed"
unset FAKE_COSIGN_OUTPUT

# The cosign version check (bundles only).
FAKE_COSIGN_VERSION_OUTPUT="GitVersion:    v2.4.1"
export FAKE_COSIGN_VERSION_OUTPUT
assert_eq "verify_checksums_signature fails with a cosign older than the minimum" "1" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json" 0)"
assert_output_has "verify_checksums_signature names the cosign minimum" \
    "cosign 2.4.1 is older than the minimum 2.4.2 needed to verify this release; upgrade cosign to v2.4.2 or later and try again"
assert_output_lacks "verify_checksums_signature does not call an old cosign a bad release" \
    "Do not install this release."
assert_exit_code "verify_checksums_signature does not verify with a cosign older than the minimum" 1 \
    test -e "$FAKE_COSIGN_ARGS"

assert_eq "verify_checksums_signature accepts an old cosign for a legacy signature" "0" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sig .pem" 0)"

for _ver in "GitVersion:    v2.4.2" "GitVersion: 2.10.0" "GitVersion:    v3.0.1"; do
    FAKE_COSIGN_VERSION_OUTPUT="$_ver"
    assert_eq "verify_checksums_signature accepts cosign '$_ver'" "0" \
        "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json" 0)"
done

FAKE_COSIGN_VERSION_OUTPUT="GitVersion:    devel"
assert_eq "verify_checksums_signature verifies with an unreadable cosign version" "0" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json" 0)"
assert_output_has "verify_checksums_signature warns about an unreadable cosign version" \
    "Could not read the cosign version"
assert_eq "verify_checksums_signature still fails closed with an unreadable cosign version" "1" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json" 1)"
unset FAKE_COSIGN_VERSION_OUTPUT

FAKE_COSIGN_VERSION_RC=1
export FAKE_COSIGN_VERSION_RC
assert_eq "verify_checksums_signature fails when 'cosign version' fails" "1" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json" 0)"
assert_output_has "verify_checksums_signature reports a broken cosign as a tool problem" \
    "'cosign version' failed"
assert_exit_code "verify_checksums_signature does not verify with a broken cosign" 1 \
    test -e "$FAKE_COSIGN_ARGS"
unset FAKE_COSIGN_VERSION_RC

# A legacy signature and certificate.
assert_eq "verify_checksums_signature falls back to a legacy signature" "0" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sig .pem" 0)"
assert_eq "verify_checksums_signature pins identity, issuer and trigger for a legacy signature" \
    "$_legacy_args" "$(cat "$FAKE_COSIGN_ARGS")"

# Both a bundle and a legacy pair: the bundle wins, and its failure is final.
assert_eq "verify_checksums_signature prefers the bundle to a legacy signature" "0" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json .sig .pem" 0)"
assert_eq "verify_checksums_signature verifies only the bundle when both exist" \
    "$_bundle_args" "$(cat "$FAKE_COSIGN_ARGS")"
assert_eq "verify_checksums_signature fails closed when the bundle fails and a legacy pair exists" "1" \
    "$(run_sig_case "" "$_fake_cosign_path" ".sigstore.json .sig .pem" 1)"
assert_eq "verify_checksums_signature does not fall back to the legacy pair after a bundle failure" \
    "verify-blob" "$(cat "$FAKE_COSIGN_CALLS")"
assert_eq "verify_checksums_signature ran the failing verification with the bundle" \
    "$_bundle_args" "$(cat "$FAKE_COSIGN_ARGS")"

# No signature at all.
assert_eq "verify_checksums_signature fails closed without a signature" "1" \
    "$(run_sig_case "" "$_fake_cosign_path" "" 0)"
assert_exit_code "verify_checksums_signature does not run cosign without a signature" 1 \
    test -e "$FAKE_COSIGN_ARGS"

rm -rf "$_sig_tmp"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
