#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Unit tests for release.sh and release-notes.sh. These tests avoid network and
# release mutations; successful release.sh coverage uses DRY_RUN=1.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TMPDIR_BASE="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_BASE"' EXIT

# shellcheck source=release-notes.sh
source "$SCRIPT_DIR/release-notes.sh"

PASS=0
FAIL=0
LAST_OUTPUT=""
LAST_STATUS=0

record_pass() {
    PASS=$((PASS + 1))
}

record_fail() {
    local desc="$1"
    local message="$2"

    FAIL=$((FAIL + 1))
    printf 'FAIL: %s\n%s\n' "$desc" "$message"
}

run_capture() {
    set +e
    LAST_OUTPUT="$("$@" 2>&1)"
    LAST_STATUS=$?
    set -e
}

assert_status_contains() {
    local desc="$1"
    local expected_status="$2"
    local expected_text="$3"
    shift 3

    run_capture "$@"
    if [[ "$LAST_STATUS" -ne "$expected_status" ]]; then
        record_fail "$desc" "  expected exit code: $expected_status
  actual exit code:   $LAST_STATUS
  output:
$LAST_OUTPUT"
        return
    fi

    if [[ "$LAST_OUTPUT" != *"$expected_text"* ]]; then
        record_fail "$desc" "  expected output to contain: $expected_text
  actual output:
$LAST_OUTPUT"
        return
    fi

    record_pass
}

assert_output_not_contains() {
    local desc="$1"
    local unexpected_text="$2"

    if [[ "$LAST_OUTPUT" == *"$unexpected_text"* ]]; then
        record_fail "$desc" "  unexpected output text: $unexpected_text
  actual output:
$LAST_OUTPUT"
        return
    fi

    record_pass
}

assert_file_eq() {
    local desc="$1"
    local expected_file="$2"
    local actual_file="$3"
    local expected_content
    local actual_content

    expected_content="$(cat "$expected_file")"
    actual_content="$(cat "$actual_file")"
    if [[ "$expected_content" != "$actual_content" ]]; then
        record_fail "$desc" "  expected:
$expected_content
  actual:
$actual_content"
        return
    fi

    record_pass
}

# ---------------------------------------------------------------------------
# Tests: release notes validation through release.sh
# ---------------------------------------------------------------------------

assert_status_contains "missing RELEASE_NOTES_FILE fails before bump computation" 1 \
    "RELEASE_NOTES_FILE=<path> is required" \
    bash "$REPO_ROOT/scripts/release.sh" bump patch
assert_output_not_contains "missing RELEASE_NOTES_FILE does not compute next version" \
    "Latest stable tag"

assert_status_contains "missing release notes path is reported" 1 \
    "Release notes file does not exist: $TMPDIR_BASE/missing.md" \
    bash "$REPO_ROOT/scripts/release.sh" tag v999.999.991 "$TMPDIR_BASE/missing.md" "" 1

assert_status_contains "directory release notes path is rejected" 1 \
    "regular markdown file" \
    bash "$REPO_ROOT/scripts/release.sh" tag v999.999.992 "$TMPDIR_BASE" "" 1

_txt_notes="$TMPDIR_BASE/release-notes.txt"
printf '# Release\n' > "$_txt_notes"
assert_status_contains "non-markdown release notes path is rejected" 1 \
    "markdown file (.md or .markdown)" \
    bash "$REPO_ROOT/scripts/release.sh" tag v999.999.993 "$_txt_notes" "" 1

_empty_notes="$TMPDIR_BASE/empty.md"
: > "$_empty_notes"
assert_status_contains "empty release notes file is rejected" 1 \
    "Release notes file cannot be empty" \
    bash "$REPO_ROOT/scripts/release.sh" tag v999.999.994 "$_empty_notes" "" 1

_valid_notes="$TMPDIR_BASE/release-notes.md"
cat > "$_valid_notes" <<'EOF'
# Release v999.999.995

- Curated release note.
EOF

assert_status_contains "tag dry-run succeeds with release notes file" 0 \
    "[DRY RUN] Would create signed tag 'v999.999.995' with release notes" \
    bash "$REPO_ROOT/scripts/release.sh" tag v999.999.995 "$_valid_notes" "" 1
assert_output_not_contains "tag dry-run does not push" "Pushing tag to origin"

assert_status_contains "bump dry-run succeeds with release notes file" 0 \
    "[DRY RUN] Would create signed tag" \
    bash "$REPO_ROOT/scripts/release.sh" bump patch "$_valid_notes" "" "" "" 1

# ---------------------------------------------------------------------------
# Tests: tag-bound release notes extraction
# ---------------------------------------------------------------------------

_tag_repo="$TMPDIR_BASE/tag-repo"
mkdir -p "$_tag_repo"
git -C "$_tag_repo" init -q
git -C "$_tag_repo" config user.email test@example.com
git -C "$_tag_repo" config user.name Test
printf 'content\n' > "$_tag_repo/file.txt"
git -C "$_tag_repo" add file.txt
git -C "$_tag_repo" commit -qm init

_multiline_notes="$TMPDIR_BASE/multiline-notes.md"
cat > "$_multiline_notes" <<'EOF'
# Release v1.2.3

- item one
- item two

```bash
echo "hello from release notes"
```
EOF

git -C "$_tag_repo" tag -a --cleanup=verbatim v1.2.3 -F "$_multiline_notes"

_extracted_notes="$TMPDIR_BASE/extracted-notes.md"
(
    cd "$_tag_repo"
    extract_release_notes_from_tag v1.2.3 "$_extracted_notes"
)
assert_file_eq "extracts multiline markdown tag notes exactly" \
    "$_multiline_notes" "$_extracted_notes"

git -C "$_tag_repo" tag v1.2.4
assert_status_contains "lightweight tag release notes extraction fails" 1 \
    "must be an annotated tag containing release notes" \
    bash -c 'source "$1"; cd "$2"; extract_release_notes_from_tag v1.2.4 "$3"' \
    bash "$SCRIPT_DIR/release-notes.sh" "$_tag_repo" "$TMPDIR_BASE/lightweight.md"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
