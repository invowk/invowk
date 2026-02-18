#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
#
# Enhance GoReleaser-generated WinGet installer manifest with fields that
# GoReleaser doesn't natively support: Commands, MinimumOSVersion, Platform.
#
# Usage: GH_TOKEN=<token> bash scripts/enhance-winget-manifest.sh <version>
#
# This script is designed to run as a CI step immediately after GoReleaser
# creates the WinGet PR. It fetches the installer manifest from the fork,
# injects the missing fields, and pushes the update back.

set -euo pipefail

for cmd in gh jq python3 base64; do
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "ERROR: Required command '${cmd}' not found."
    echo "  This script requires: gh, jq, python3, base64"
    exit 1
  }
done

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VERSION="${1:?Usage: enhance-winget-manifest.sh <version>}"

# Skip pre-releases — GoReleaser uses skip_upload: auto, so no WinGet PR is
# created for pre-release tags.
if [[ "$VERSION" == *-* ]]; then
  echo "Skipping WinGet manifest enhancement for pre-release: $VERSION"
  exit 0
fi

REPO="invowk/winget-pkgs"
BRANCH="invowk-${VERSION}"
FILE_PATH="manifests/i/Invowk/Invowk/${VERSION}/Invowk.Invowk.installer.yaml"
TMPFILE="$(mktemp)"
trap 'rm -f "$TMPFILE"' EXIT

echo "Enhancing WinGet installer manifest for version ${VERSION}..."
echo "  Repo:   ${REPO}"
echo "  Branch: ${BRANCH}"
echo "  File:   ${FILE_PATH}"

# Fetch the current file from the fork. If the file doesn't exist (HTTP 404),
# exit gracefully — the WinGet PR may not have been created. All other errors
# (auth, rate-limit, network) fail loudly.
if ! RESPONSE=$(gh api "repos/${REPO}/contents/${FILE_PATH}?ref=${BRANCH}" 2>&1); then
  if echo "$RESPONSE" | grep -q '"Not Found"'; then
    echo "Warning: WinGet manifest not found at ${FILE_PATH} on branch ${BRANCH}."
    echo "The WinGet PR may not have been created. Skipping enhancement."
    exit 0
  fi
  echo "ERROR: Failed to fetch WinGet manifest."
  echo "  Repo:     ${REPO}"
  echo "  Branch:   ${BRANCH}"
  echo "  Response: ${RESPONSE}"
  exit 1
fi

FILE_SHA=$(echo "$RESPONSE" | jq -r '.sha // empty')
CONTENT_B64=$(echo "$RESPONSE" | jq -r '.content // empty')

if [ -z "$FILE_SHA" ] || [ -z "$CONTENT_B64" ]; then
  echo "ERROR: Unexpected API response — missing 'sha' or 'content' field."
  echo "  Response (first 500 chars): $(echo "$RESPONSE" | head -c 500)"
  exit 1
fi

echo "$CONTENT_B64" | base64 -d > "$TMPFILE" || {
  echo "ERROR: Failed to decode base64 content from API response."
  exit 1
}

echo "  SHA:    ${FILE_SHA}"
echo "  Original manifest:"
cat "$TMPFILE"
echo ""

# Inject missing fields using the companion Python script.
# This preserves GoReleaser's exact formatting while inserting fields at the
# correct positions in the manifest. The Python script is idempotent (safe to
# re-run on an already-enhanced manifest without producing changes).
python3 "$SCRIPT_DIR/enhance_winget_fields.py" "$TMPFILE"

echo "  Enhanced manifest:"
cat "$TMPFILE"
echo ""

# Push the enhanced manifest back to the fork branch.
CONTENT=$(base64 "$TMPFILE" | tr -d '\n')
gh api -X PUT "repos/${REPO}/contents/${FILE_PATH}" \
  -f message="Add Commands, MinimumOSVersion, Platform to installer manifest" \
  -f branch="${BRANCH}" \
  -f sha="${FILE_SHA}" \
  -f content="${CONTENT}" > /dev/null || {
  echo "ERROR: Failed to push enhanced manifest to ${REPO}."
  echo "  Branch: ${BRANCH}"
  echo "  File:   ${FILE_PATH}"
  echo "  This may be a SHA conflict (race condition). Re-running the workflow may help."
  exit 1
}

echo "WinGet installer manifest enhanced successfully."
